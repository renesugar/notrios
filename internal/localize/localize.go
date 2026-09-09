// Package localize converts a note's remote media into local
// content-addressed resources — the H4 step on top of the H2 scan and the
// H3 quarantine pipeline. One engine serves every entry point (REST, CLI,
// MCP, GUI, import-time): scan the body, quarantine-fetch what policy
// permits, check exact-hash rules, admit approved bytes to the asset store,
// attach resources, and rewrite the Markdown to resource:// URIs in a new
// revision guarded by a base-revision precondition. Dry runs report what
// would happen without fetching a single byte.
package localize

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/media"
	"github.com/renesugar/notrios/internal/store"
)

// Options selects the document and the run mode.
type Options struct {
	DocumentID string
	// BaseRevisionID guards the rewrite; empty uses the document's current
	// revision (still enforced atomically by the store on update).
	BaseRevisionID string
	// DryRun reports decisions and would-be rewrites without any fetching.
	DryRun bool
	// AllowReview also localizes URLs whose policy decision is "review"
	// (an explicit reviewer action).
	AllowReview bool
}

// ErrReadOnly marks notes that must not be rewritten (Help notebook, Trash).
var ErrReadOnly = fmt.Errorf("document is read-only")

// Localizer runs remote-media localization against one store.
type Localizer struct {
	store  store.Store
	cfg    config.RemoteMediaConfig
	policy *media.Policy

	fetchOnce sync.Once
	fetcher   *media.Fetcher
	fetchErr  error
}

// New builds a localizer. The quarantine fetcher (and its directory) is
// created lazily on the first non-dry run, so constructing a server never
// touches the filesystem.
func New(cfg config.RemoteMediaConfig, st store.Store) *Localizer {
	return &Localizer{store: st, cfg: cfg, policy: media.NewPolicy(cfg)}
}

type attemptRecorder struct{ st store.Store }

func (r attemptRecorder) RecordMediaAttempt(ctx context.Context, attempt media.Attempt) error {
	_, err := r.st.RecordMediaAttempt(ctx, store.MediaAttempt{
		DocumentID:     attempt.DocumentID,
		OriginalURL:    attempt.OriginalURL,
		FinalURL:       attempt.FinalURL,
		Decision:       attempt.Decision,
		Reason:         attempt.Reason,
		Status:         attempt.Status,
		ContentType:    attempt.ContentType,
		SizeBytes:      attempt.SizeBytes,
		SHA256:         attempt.SHA256,
		QuarantinePath: attempt.QuarantinePath,
	})
	return err
}

func (l *Localizer) getFetcher() (*media.Fetcher, error) {
	l.fetchOnce.Do(func() {
		l.fetcher, l.fetchErr = media.NewFetcher(l.cfg, attemptRecorder{l.store})
	})
	return l.fetcher, l.fetchErr
}

// LocalizeDocument runs the pipeline for one note.
func (l *Localizer) LocalizeDocument(ctx context.Context, opts Options) (api.RemoteMediaResult, error) {
	result := emptyResult()
	doc, err := l.store.GetDocument(ctx, opts.DocumentID)
	if err != nil {
		return result, err
	}
	if store.IsReadOnlyNotebook(doc.NotebookID) || !doc.DeletedAt.IsZero() {
		return result, fmt.Errorf("%w: %s", ErrReadOnly, doc.ID)
	}
	baseRevisionID := opts.BaseRevisionID
	if baseRevisionID == "" {
		baseRevisionID = doc.CurrentRevisionID
	}

	decisions := l.policy.ScanBody(doc.Body)
	var fetchable []media.Decision
	for _, decision := range decisions {
		switch {
		case decision.Action == media.ActionAllow,
			decision.Action == media.ActionReview && opts.AllowReview:
			fetchable = append(fetchable, decision)
		case decision.Action == media.ActionReview:
			result.Review = append(result.Review, map[string]any{"url": decision.URL, "reason": decision.Reason})
		default:
			result.Blocked = append(result.Blocked, map[string]any{"url": decision.URL, "reason": decision.Reason})
		}
	}

	if opts.DryRun {
		for _, decision := range fetchable {
			result.Localized = append(result.Localized, map[string]any{
				"url":            decision.URL,
				"media_class":    decision.MediaClass,
				"would_localize": true,
				"reason":         decision.Reason,
			})
		}
		return result, nil
	}
	if len(fetchable) == 0 {
		return result, nil
	}

	fetcher, err := l.getFetcher()
	if err != nil {
		return result, err
	}
	urls := make([]string, 0, len(fetchable))
	for _, decision := range fetchable {
		urls = append(urls, decision.URL)
	}
	fetched := fetcher.Quarantine(ctx, media.QuarantineRequest{
		DocumentID:  doc.ID,
		URLs:        urls,
		AllowReview: opts.AllowReview,
	})

	body := doc.Body
	rewrites := 0
	for _, fetch := range fetched {
		if fetch.Status != media.StatusQuarantined {
			result.Failed = append(result.Failed, map[string]any{"url": fetch.URL, "reason": fetch.Reason})
			continue
		}
		entry, ok := l.admit(ctx, doc, fetch, &result)
		if !ok {
			continue
		}
		replaced := strings.ReplaceAll(body, fetch.URL, entry["resource_uri"].(string))
		if replaced != body {
			body = replaced
			rewrites++
			entry["rewritten"] = true
		}
		result.Localized = append(result.Localized, entry)
	}

	if rewrites > 0 {
		updated, err := l.store.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID:             doc.ID,
			Title:          doc.Title,
			Body:           body,
			BodyMIMEType:   doc.BodyMIMEType,
			BaseRevisionID: baseRevisionID,
			Message:        fmt.Sprintf("Localize %d remote media URL(s)", rewrites),
		})
		if err != nil {
			return result, err
		}
		result.RevisionID = updated.CurrentRevisionID
	}
	return result, nil
}

// admit moves one quarantined file into the content-addressed store unless
// an exact-hash rule refuses it. The quarantine file is always removed:
// either its bytes were admitted or they are unwanted.
func (l *Localizer) admit(ctx context.Context, doc store.Document, fetch media.FetchResult, result *api.RemoteMediaResult) (map[string]any, bool) {
	defer os.Remove(fetch.QuarantinePath)

	if rule, err := l.store.FindMediaHashRule(ctx, "sha256", fetch.SHA256); err == nil {
		reason := fmt.Sprintf("content hash is %sed by rule", rule.Action)
		if rule.Reason != "" {
			reason += ": " + rule.Reason
		}
		l.recordAdmission(ctx, doc.ID, fetch, media.StatusRefused, reason)
		switch rule.Action {
		case "block":
			result.Blocked = append(result.Blocked, map[string]any{"url": fetch.URL, "reason": reason, "sha256": fetch.SHA256})
		default:
			result.Review = append(result.Review, map[string]any{"url": fetch.URL, "reason": reason, "sha256": fetch.SHA256})
		}
		return nil, false
	}

	file, err := os.Open(fetch.QuarantinePath)
	if err != nil {
		result.Failed = append(result.Failed, map[string]any{"url": fetch.URL, "reason": "quarantine read failed: " + err.Error()})
		return nil, false
	}
	defer file.Close()
	resource, err := l.store.CreateResource(ctx, store.CreateResourceRequest{
		CollectionID: doc.CollectionID,
		Filename:     filenameForURL(fetch.URL, fetch.QuarantinePath),
		MIMEType:     fetch.ContentType,
		Content:      file,
	})
	if err != nil {
		result.Failed = append(result.Failed, map[string]any{"url": fetch.URL, "reason": "resource admission failed: " + err.Error()})
		return nil, false
	}
	relation := "attachment"
	if strings.HasPrefix(fetch.ContentType, "image/") {
		relation = "embedded"
	}
	if _, err := l.store.AttachDocumentResource(ctx, store.AttachResourceRequest{
		DocumentID:   doc.ID,
		ResourceID:   resource.ID,
		RelationType: relation,
	}); err != nil {
		result.Failed = append(result.Failed, map[string]any{"url": fetch.URL, "reason": "resource attach failed: " + err.Error()})
		return nil, false
	}
	l.recordAdmission(ctx, doc.ID, fetch, "admitted", "admitted to the resource store as "+resource.ID)

	return map[string]any{
		"url":          fetch.URL,
		"final_url":    fetch.FinalURL,
		"resource_id":  resource.ID,
		"resource_uri": resource.URI,
		"sha256":       fetch.SHA256,
		"content_type": fetch.ContentType,
		"size_bytes":   fetch.SizeBytes,
	}, true
}

// recordAdmission adds the admission outcome to the audit trail (the fetch
// itself was already recorded by the quarantine pipeline). Best-effort.
func (l *Localizer) recordAdmission(ctx context.Context, documentID string, fetch media.FetchResult, status, reason string) {
	_, _ = l.store.RecordMediaAttempt(ctx, store.MediaAttempt{
		DocumentID:  documentID,
		OriginalURL: fetch.URL,
		FinalURL:    fetch.FinalURL,
		Decision:    fetch.Action,
		Reason:      reason,
		Status:      status,
		ContentType: fetch.ContentType,
		SizeBytes:   fetch.SizeBytes,
		SHA256:      fetch.SHA256,
	})
}

// filenameForURL derives a resource filename from the URL path, falling back
// to the hash-named quarantine file.
func filenameForURL(rawURL, quarantinePath string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		base := path.Base(parsed.Path)
		if base != "" && base != "." && base != "/" {
			return base
		}
	}
	return path.Base(quarantinePath)
}

func emptyResult() api.RemoteMediaResult {
	return api.RemoteMediaResult{
		Localized: []map[string]any{},
		Blocked:   []map[string]any{},
		Review:    []map[string]any{},
		Failed:    []map[string]any{},
	}
}
