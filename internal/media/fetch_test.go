package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

// pngBytes is a minimal payload whose first bytes sniff as image/png.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)

type fakeRecorder struct {
	mu       sync.Mutex
	attempts []Attempt
}

func (r *fakeRecorder) RecordMediaAttempt(_ context.Context, attempt Attempt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts = append(r.attempts, attempt)
	return nil
}

// testFetcher allows the loopback host httptest listens on: private networks
// are permitted and the test server's host is on the allowed list. Real
// deployments keep allow_private_networks=false; the SSRF dial check has its
// own unit test below.
func testFetcher(t *testing.T, serverURL string, recorder AttemptRecorder, mutate func(*config.RemoteMediaConfig)) *Fetcher {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	cfg := config.Default().RemoteMedia
	cfg.AllowPrivateNetworks = true
	cfg.AllowedDomains = []string{parsed.Hostname()}
	cfg.BlockedDomains = []string{"tracker.example.com"}
	cfg.QuarantineDir = filepath.Join(t.TempDir(), "quarantine")
	if mutate != nil {
		mutate(&cfg)
	}
	fetcher, err := NewFetcher(cfg, recorder)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	return fetcher
}

func TestQuarantineFetchesAllowedImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()
	recorder := &fakeRecorder{}
	fetcher := testFetcher(t, server.URL, recorder, nil)

	results := fetcher.Quarantine(context.Background(), QuarantineRequest{DocumentID: "doc_1", URLs: []string{server.URL + "/a.png"}})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %+v", results)
	}
	r := results[0]
	if r.Status != StatusQuarantined || r.Action != ActionAllow {
		t.Fatalf("unexpected result: %+v", r)
	}
	wantSHA := sha256.Sum256(pngBytes)
	if r.SHA256 != hex.EncodeToString(wantSHA[:]) {
		t.Fatalf("sha mismatch: %s", r.SHA256)
	}
	if r.ContentType != "image/png" || r.SizeBytes != int64(len(pngBytes)) {
		t.Fatalf("content facts wrong: %+v", r)
	}
	data, err := os.ReadFile(r.QuarantinePath)
	if err != nil || len(data) != len(pngBytes) {
		t.Fatalf("quarantined file unreadable: %v (len %d)", err, len(data))
	}
	if !strings.HasPrefix(filepath.Base(r.QuarantinePath), "sha256-") {
		t.Fatalf("quarantine file not hash-named: %s", r.QuarantinePath)
	}
	if len(recorder.attempts) != 1 || recorder.attempts[0].Status != StatusQuarantined || recorder.attempts[0].DocumentID != "doc_1" {
		t.Fatalf("attempt not recorded: %+v", recorder.attempts)
	}
}

func TestQuarantineRefusesBlockedWithoutNetworkTraffic(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()
	recorder := &fakeRecorder{}
	fetcher := testFetcher(t, server.URL, recorder, nil)

	results := fetcher.Quarantine(context.Background(), QuarantineRequest{
		URLs: []string{"https://tracker.example.com/pixel.gif"},
	})
	if results[0].Status != StatusRefused || results[0].Action != ActionBlock {
		t.Fatalf("blocked URL must be refused: %+v", results[0])
	}
	if requests != 0 {
		t.Fatalf("blocked URL must produce no network traffic")
	}
	if len(recorder.attempts) != 1 || recorder.attempts[0].Status != StatusRefused {
		t.Fatalf("refusal must be recorded: %+v", recorder.attempts)
	}
}

func TestQuarantineReviewRequiresExplicitOptIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()
	// The server host is NOT on the allowed list → default action "review".
	fetcher := testFetcher(t, server.URL, nil, func(cfg *config.RemoteMediaConfig) {
		cfg.AllowedDomains = nil
	})
	target := server.URL + "/maybe.png"

	refused := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{target}})
	if refused[0].Status != StatusRefused || refused[0].Action != ActionReview {
		t.Fatalf("review URL must be refused without opt-in: %+v", refused[0])
	}

	fetched := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{target}, AllowReview: true})
	if fetched[0].Status != StatusQuarantined {
		t.Fatalf("review URL must fetch with AllowReview: %+v", fetched[0])
	}
}

func TestQuarantineEnforcesSizeCapWhileStreaming(t *testing.T) {
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 4096)...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(big)
	}))
	defer server.Close()
	fetcher := testFetcher(t, server.URL, nil, func(cfg *config.RemoteMediaConfig) {
		cfg.MaxBytes = map[string]int64{"image": 1024}
	})

	results := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{server.URL + "/big.png"}})
	if results[0].Status != StatusRefused || !strings.Contains(results[0].Reason, "size cap") {
		t.Fatalf("oversized media must be refused: %+v", results[0])
	}
	entries, err := os.ReadDir(fetcher.quarantineDir)
	if err != nil {
		t.Fatalf("read quarantine dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("refused fetch must leave no quarantine file: %v", entries)
	}
}

func TestQuarantineRefusesNonMediaContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A lying image header must not defeat sniffing.
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("<html><body>not an image</body></html>"))
	}))
	defer server.Close()
	fetcher := testFetcher(t, server.URL, nil, nil)

	results := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{server.URL + "/fake.png"}})
	if results[0].Status != StatusRefused || !strings.Contains(results[0].Reason, "not localizable media") {
		t.Fatalf("non-media content must be refused: %+v", results[0])
	}
}

func TestQuarantineAllowsHeaderFallbackForInconclusiveSniffs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SVG sniffs as text/xml (inconclusive) — the image/svg+xml header
		// may then classify it.
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`))
	}))
	defer server.Close()
	fetcher := testFetcher(t, server.URL, nil, nil)

	results := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{server.URL + "/pic.svg"}})
	if results[0].Status != StatusQuarantined || results[0].ContentType != "image/svg+xml" {
		t.Fatalf("SVG with an inconclusive sniff must quarantine via the header type: %+v", results[0])
	}
}

func TestQuarantineChecksEveryRedirectHop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-blocked":
			http.Redirect(w, r, "https://tracker.example.com/x.png", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		default:
			_, _ = w.Write(pngBytes)
		}
	}))
	defer server.Close()
	fetcher := testFetcher(t, server.URL, nil, func(cfg *config.RemoteMediaConfig) {
		cfg.MaxRedirects = 3
	})

	blocked := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{server.URL + "/to-blocked"}})
	if blocked[0].Status != StatusRefused || !strings.Contains(blocked[0].Reason, "refused") {
		t.Fatalf("redirect to a blocked domain must refuse the fetch: %+v", blocked[0])
	}

	looping := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{server.URL + "/loop"}})
	if looping[0].Status != StatusRefused || !strings.Contains(looping[0].Reason, "redirect") {
		t.Fatalf("redirect loops must be refused: %+v", looping[0])
	}
}

func TestQuarantineRefusesNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()
	fetcher := testFetcher(t, server.URL, nil, nil)
	results := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{server.URL + "/gone.png"}})
	if results[0].Status != StatusRefused || !strings.Contains(results[0].Reason, "404") {
		t.Fatalf("non-200 must be refused: %+v", results[0])
	}
}

func TestCheckDialAddressBlocksPrivateResolutions(t *testing.T) {
	cfg := config.Default().RemoteMedia
	cfg.QuarantineDir = t.TempDir()
	fetcher, err := NewFetcher(cfg, nil)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	for _, address := range []string{"127.0.0.1:443", "10.0.0.8:80", "192.168.1.1:80", "169.254.0.9:80", "[::1]:443", "0.0.0.0:80"} {
		if err := fetcher.checkDialAddress(address); err == nil {
			t.Errorf("dial to %s must be refused (DNS rebinding protection)", address)
		}
	}
	if err := fetcher.checkDialAddress("93.184.216.34:443"); err != nil {
		t.Errorf("public address must dial: %v", err)
	}

	cfg.AllowPrivateNetworks = true
	permissive, err := NewFetcher(cfg, nil)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	if err := permissive.checkDialAddress("127.0.0.1:443"); err != nil {
		t.Errorf("allow_private_networks=true must permit loopback dials: %v", err)
	}
}

// The end-to-end proof that a hostname resolving to a private address is
// stopped at connect time even when the static policy allows the domain.
func TestQuarantineBlocksPrivateDialEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)

	cfg := config.Default().RemoteMedia
	cfg.AllowPrivateNetworks = false // dial checks active
	cfg.AllowedDomains = []string{"localtest.invalid"}
	cfg.QuarantineDir = t.TempDir()
	fetcher, err := NewFetcher(cfg, nil)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	// Make the allowed hostname resolve to the loopback test server.
	fetcher.client.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		address := net.JoinHostPort(parsed.Hostname(), parsed.Port())
		if err := fetcher.checkDialAddress(address); err != nil {
			return nil, err
		}
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}

	results := fetcher.Quarantine(context.Background(), QuarantineRequest{
		URLs: []string{fmt.Sprintf("http://localtest.invalid:%s/a.png", parsed.Port())},
	})
	if results[0].Status != StatusRefused || !strings.Contains(results[0].Reason, "private") {
		t.Fatalf("private resolution must be refused at connect time: %+v", results[0])
	}
}
