// Quarantine download pipeline (v0.3 task H3). Fetches policy-permitted
// remote media into the quarantine directory — never the asset store — with
// the protections SECURITY_AND_MEDIA_POLICY.md requires:
//
//   - the domain/scheme policy is re-applied to every redirect hop;
//   - resolved addresses are checked at connect time (SSRF/DNS-rebinding:
//     the static Evaluate cannot see what a hostname resolves to);
//   - proxies from the environment are deliberately ignored, since a proxy
//     would bypass the connect-time address checks;
//   - the per-class size cap is enforced while streaming;
//   - the content type is sniffed from the first bytes (headers can lie)
//     and must be localizable media (image, video/audio, or PDF);
//   - an exact SHA-256 is computed over the quarantined bytes.
//
// Every attempt — quarantined or refused — is reported and, when a recorder
// is attached, persisted to the media_policy_decisions table. Admission to
// the content-addressed store is task H4.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/renesugar/notrios/internal/config"
)

// Attempt statuses recorded for each URL.
const (
	StatusQuarantined = "quarantined"
	StatusRefused     = "refused"
)

// defaultMaxBytes caps media classes with no configured limit.
const defaultMaxBytes = 20 * 1024 * 1024

// FetchResult reports the outcome for one URL of a quarantine run.
type FetchResult struct {
	URL            string `json:"url"`
	FinalURL       string `json:"final_url,omitempty"`
	Status         string `json:"status"` // quarantined | refused
	Action         string `json:"action"` // policy action for the original URL
	Reason         string `json:"reason"`
	ContentType    string `json:"content_type,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	QuarantinePath string `json:"quarantine_path,omitempty"`
}

// Attempt is the persistence-facing view of one fetch attempt.
type Attempt struct {
	DocumentID     string
	OriginalURL    string
	FinalURL       string
	Decision       string
	Reason         string
	Status         string
	ContentType    string
	SizeBytes      int64
	SHA256         string
	QuarantinePath string
}

// AttemptRecorder persists attempts; implemented by the SQLite store
// (RecordMediaAttempt) via a thin adapter. A nil recorder skips persistence.
type AttemptRecorder interface {
	RecordMediaAttempt(ctx context.Context, attempt Attempt) error
}

// QuarantineRequest names the URLs to fetch for one document.
type QuarantineRequest struct {
	// DocumentID is recorded with each attempt; may be empty for ad-hoc runs.
	DocumentID string
	URLs       []string
	// AllowReview also fetches URLs whose policy decision is "review"
	// (an explicit reviewer action); "block" is never fetched.
	AllowReview bool
}

// Fetcher downloads permitted remote media into quarantine.
type Fetcher struct {
	policy        *Policy
	cfg           config.RemoteMediaConfig
	client        *http.Client
	quarantineDir string
	recorder      AttemptRecorder
}

// NewFetcher builds a quarantine fetcher for the configured policy. The
// quarantine directory is created if missing.
func NewFetcher(cfg config.RemoteMediaConfig, recorder AttemptRecorder) (*Fetcher, error) {
	dir := strings.TrimSpace(cfg.QuarantineDir)
	if dir == "" {
		return nil, fmt.Errorf("remote_media.quarantine_dir is not configured")
	}
	// Owner-only: this sits under the quarantine, which holds untrusted
	// downloaded bytes and the record of what a note tried to fetch.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create quarantine dir %q: %w", dir, err)
	}
	f := &Fetcher{
		policy:        NewPolicy(cfg),
		cfg:           cfg,
		quarantineDir: dir,
		recorder:      recorder,
	}

	dialer := &net.Dialer{
		Timeout: 15 * time.Second,
		// Control runs after DNS resolution with the concrete address about
		// to be dialed — the only reliable place to stop DNS-rebinding.
		Control: func(network, address string, _ syscall.RawConn) error {
			return f.checkDialAddress(address)
		},
	}
	timeout := time.Duration(cfg.FetchTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	f.client = &http.Client{
		Transport: &http.Transport{
			// No Proxy function: an environment proxy would dial on our
			// behalf and bypass the connect-time address checks.
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: timeout,
		},
		Timeout:       timeout,
		CheckRedirect: f.checkRedirect,
	}
	return f, nil
}

// Quarantine evaluates and fetches each URL, returning one result per URL in
// order. Blocked URLs (and review URLs unless AllowReview) are refused
// without any network traffic. Every result is recorded.
func (f *Fetcher) Quarantine(ctx context.Context, req QuarantineRequest) []FetchResult {
	results := make([]FetchResult, 0, len(req.URLs))
	for _, raw := range req.URLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		action, reason := f.policy.Evaluate(raw)
		var result FetchResult
		if !f.permits(action, req.AllowReview) {
			result = FetchResult{URL: raw, Status: StatusRefused, Action: action, Reason: reason}
		} else {
			result = f.fetchOne(ctx, raw, action, req.AllowReview)
		}
		f.record(ctx, req.DocumentID, result)
		results = append(results, result)
	}
	return results
}

func (f *Fetcher) permits(action string, allowReview bool) bool {
	return action == ActionAllow || (action == ActionReview && allowReview)
}

// checkRedirect re-applies the policy to every redirect hop and enforces the
// configured hop limit. via holds the preceding requests.
func (f *Fetcher) checkRedirect(req *http.Request, via []*http.Request) error {
	maxRedirects := f.cfg.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 5
	}
	if len(via) > maxRedirects {
		return fmt.Errorf("more than %d redirects", maxRedirects)
	}
	action, reason := f.policy.Evaluate(req.URL.String())
	// A redirect hop must satisfy the same permission the original URL was
	// fetched under; the hop's own review/block verdict refuses the fetch.
	allowReview := via[0].Context().Value(allowReviewKey{}) == true
	if !f.permits(action, allowReview) {
		return fmt.Errorf("redirect to %s refused: %s", req.URL.Redacted(), reason)
	}
	return nil
}

type allowReviewKey struct{}

// checkDialAddress rejects private, loopback, link-local, and unspecified
// resolved addresses unless the policy allows private networks.
func (f *Fetcher) checkDialAddress(address string) error {
	if f.cfg.AllowPrivateNetworks {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("unexpected dial address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("dial address %q is not an IP literal", address)
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("resolved address %s is private, loopback, or link-local", ip)
	}
	return nil
}

func refuse(url, action, reason string) FetchResult {
	return FetchResult{URL: url, Status: StatusRefused, Action: action, Reason: reason}
}

func (f *Fetcher) fetchOne(ctx context.Context, rawURL, action string, allowReview bool) FetchResult {
	ctx = context.WithValue(contextOrBackground(ctx), allowReviewKey{}, allowReview)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return refuse(rawURL, action, "invalid request URL: "+err.Error())
	}
	request.Header.Set("User-Agent", "notrios-media-quarantine")

	response, err := f.client.Do(request)
	if err != nil {
		return refuse(rawURL, action, "fetch failed: "+err.Error())
	}
	defer response.Body.Close()
	finalURL := response.Request.URL.String()
	if response.StatusCode != http.StatusOK {
		result := refuse(rawURL, action, fmt.Sprintf("unexpected HTTP status %d", response.StatusCode))
		result.FinalURL = finalURL
		return result
	}

	// Sniff the real content type from the first bytes; headers can lie.
	head := make([]byte, 512)
	n, err := io.ReadFull(response.Body, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		result := refuse(rawURL, action, "read failed: "+err.Error())
		result.FinalURL = finalURL
		return result
	}
	head = head[:n]
	contentType := strings.TrimSpace(strings.SplitN(http.DetectContentType(head), ";", 2)[0])
	class := classFromMIME(contentType)
	if class == "other" && sniffInconclusive(contentType) {
		// Formats sniffing doesn't know (e.g. SVG → text/xml) may fall back
		// to the header — but only when the sniff was inconclusive. A
		// positive detection (like text/html) always wins over a lying
		// image/* header.
		if headerType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type")); err == nil {
			if headerClass := classFromMIME(headerType); headerClass != "other" {
				contentType, class = headerType, headerClass
			}
		}
	}
	if class == "other" {
		result := refuse(rawURL, action, fmt.Sprintf("content type %q is not localizable media", contentType))
		result.FinalURL = finalURL
		return result
	}

	limit := f.maxBytesFor(class)
	if int64(len(head)) > limit {
		result := refuse(rawURL, action, fmt.Sprintf("exceeds the %s size cap (%d bytes)", class, limit))
		result.FinalURL = finalURL
		return result
	}

	file, err := os.CreateTemp(f.quarantineDir, ".fetch-*")
	if err != nil {
		result := refuse(rawURL, action, "quarantine write failed: "+err.Error())
		result.FinalURL = finalURL
		return result
	}
	tempPath := file.Name()
	cleanup := func() {
		file.Close()
		os.Remove(tempPath)
	}

	hasher := sha256.New()
	sink := io.MultiWriter(file, hasher)
	if _, err := sink.Write(head); err != nil {
		cleanup()
		result := refuse(rawURL, action, "quarantine write failed: "+err.Error())
		result.FinalURL = finalURL
		return result
	}
	remaining := limit - int64(len(head))
	copied, err := io.Copy(sink, io.LimitReader(response.Body, remaining+1))
	if err != nil {
		cleanup()
		result := refuse(rawURL, action, "read failed: "+err.Error())
		result.FinalURL = finalURL
		return result
	}
	if copied > remaining {
		cleanup()
		result := refuse(rawURL, action, fmt.Sprintf("exceeds the %s size cap (%d bytes)", class, limit))
		result.FinalURL = finalURL
		return result
	}
	if err := file.Close(); err != nil {
		os.Remove(tempPath)
		result := refuse(rawURL, action, "quarantine write failed: "+err.Error())
		result.FinalURL = finalURL
		return result
	}

	shaHex := hex.EncodeToString(hasher.Sum(nil))
	finalPath := filepath.Join(f.quarantineDir, "sha256-"+shaHex+extensionForMIME(contentType))
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		result := refuse(rawURL, action, "quarantine write failed: "+err.Error())
		result.FinalURL = finalURL
		return result
	}

	return FetchResult{
		URL:            rawURL,
		FinalURL:       finalURL,
		Status:         StatusQuarantined,
		Action:         action,
		Reason:         "fetched into quarantine",
		ContentType:    contentType,
		SizeBytes:      int64(len(head)) + copied,
		SHA256:         shaHex,
		QuarantinePath: finalPath,
	}
}

func (f *Fetcher) maxBytesFor(class string) int64 {
	if limit, ok := f.cfg.MaxBytes[class]; ok && limit > 0 {
		return limit
	}
	return defaultMaxBytes
}

func (f *Fetcher) record(ctx context.Context, documentID string, result FetchResult) {
	if f.recorder == nil {
		return
	}
	// Recording is auditing, not control flow: a failure must not turn a
	// completed quarantine into an error, so it is deliberately best-effort.
	_ = f.recorder.RecordMediaAttempt(ctx, Attempt{
		DocumentID:     documentID,
		OriginalURL:    result.URL,
		FinalURL:       result.FinalURL,
		Decision:       result.Action,
		Reason:         result.Reason,
		Status:         result.Status,
		ContentType:    result.ContentType,
		SizeBytes:      result.SizeBytes,
		SHA256:         result.SHA256,
		QuarantinePath: result.QuarantinePath,
	})
}

// sniffInconclusive reports whether DetectContentType failed to positively
// identify the payload (its generic fallbacks), as opposed to detecting a
// concrete non-media type like text/html.
func sniffInconclusive(sniffed string) bool {
	switch sniffed {
	case "application/octet-stream", "text/plain", "text/xml":
		return true
	}
	return false
}

func classFromMIME(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/"), strings.HasPrefix(mimeType, "audio/"):
		return "video"
	case mimeType == "application/pdf":
		return "pdf"
	}
	return "other"
}

func extensionForMIME(mimeType string) string {
	extensions, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(extensions) == 0 {
		return ".bin"
	}
	// Prefer common spellings over the first alphabetical match.
	preferred := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "application/pdf": ".pdf"}
	if ext, ok := preferred[strings.ToLower(mimeType)]; ok {
		return ext
	}
	return extensions[0]
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
