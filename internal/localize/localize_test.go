package localize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 48)...)

func testSetup(t *testing.T, handler http.Handler) (*Localizer, *store.SQLiteStore, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	st, err := store.OpenSQLiteWithAssetStore(":memory:", filepath.Join(t.TempDir(), "assets"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	cfg := config.Default().RemoteMedia
	cfg.AllowPrivateNetworks = true // httptest listens on loopback
	cfg.AllowedDomains = []string{parsed.Hostname()}
	cfg.BlockedDomains = []string{"tracker.example.com"}
	cfg.QuarantineDir = filepath.Join(t.TempDir(), "quarantine")
	return New(cfg, st), st, server
}

func pngHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	})
}

func TestLocalizeRewritesAllowedImage(t *testing.T) {
	localizer, st, server := testSetup(t, pngHandler())
	ctx := context.Background()
	body := "intro\n\n![pic](" + server.URL + "/a.png)\n\n![tracked](https://tracker.example.com/pixel.gif)\n"
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Localize me", Body: body})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	result, err := localizer.LocalizeDocument(ctx, Options{DocumentID: doc.ID})
	if err != nil {
		t.Fatalf("LocalizeDocument: %v", err)
	}
	if len(result.Localized) != 1 || len(result.Blocked) != 1 || len(result.Failed) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	entry := result.Localized[0]
	resourceURI, _ := entry["resource_uri"].(string)
	if !strings.HasPrefix(resourceURI, "resource://") {
		t.Fatalf("localized entry missing resource URI: %+v", entry)
	}
	if result.RevisionID == "" {
		t.Fatalf("rewrite must create a new revision")
	}

	updated, err := st.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if updated.CurrentRevisionID != result.RevisionID {
		t.Fatalf("revision mismatch: %s vs %s", updated.CurrentRevisionID, result.RevisionID)
	}
	if strings.Contains(updated.Body, server.URL) {
		t.Fatalf("remote URL must be rewritten: %s", updated.Body)
	}
	if !strings.Contains(updated.Body, resourceURI) {
		t.Fatalf("body must reference the resource URI: %s", updated.Body)
	}
	// The blocked URL stays untouched.
	if !strings.Contains(updated.Body, "tracker.example.com/pixel.gif") {
		t.Fatalf("blocked URL must remain: %s", updated.Body)
	}

	// The resource holds the exact bytes.
	resourceID, _ := entry["resource_id"].(string)
	_, reader, err := st.OpenResourceContent(ctx, resourceID)
	if err != nil {
		t.Fatalf("OpenResourceContent: %v", err)
	}
	defer reader.Close()
	data := make([]byte, len(pngBytes)+8)
	n, _ := reader.Read(data)
	sum := sha256.Sum256(data[:n])
	if hex.EncodeToString(sum[:]) != entry["sha256"] {
		t.Fatalf("admitted bytes do not match the recorded hash")
	}

	// Attached to the note.
	refs, err := st.ListDocumentResources(ctx, doc.ID)
	if err != nil || len(refs) != 1 || refs[0].RelationType != "embedded" {
		t.Fatalf("resource not attached as embedded: %+v err=%v", refs, err)
	}

	// Audit trail: quarantined then admitted.
	attempts, err := st.ListMediaAttempts(ctx, doc.ID, 10)
	if err != nil {
		t.Fatalf("ListMediaAttempts: %v", err)
	}
	statuses := map[string]bool{}
	for _, attempt := range attempts {
		statuses[attempt.Status] = true
	}
	if !statuses["quarantined"] || !statuses["admitted"] {
		t.Fatalf("audit trail incomplete: %+v", attempts)
	}
}

func TestLocalizeDryRunNeverFetches(t *testing.T) {
	requests := 0
	localizer, st, server := testSetup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Dry run",
		Body:  "![pic](" + server.URL + "/a.png)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	result, err := localizer.LocalizeDocument(ctx, Options{DocumentID: doc.ID, DryRun: true})
	if err != nil {
		t.Fatalf("LocalizeDocument: %v", err)
	}
	if requests != 0 {
		t.Fatalf("dry run must not fetch")
	}
	if result.RevisionID != "" {
		t.Fatalf("dry run must not create revisions")
	}
	if len(result.Localized) != 1 || result.Localized[0]["would_localize"] != true {
		t.Fatalf("dry run must report would-be localizations: %+v", result.Localized)
	}
	after, _ := st.GetDocument(ctx, doc.ID)
	if after.CurrentRevisionID != doc.CurrentRevisionID || after.Body != doc.Body {
		t.Fatalf("dry run must not modify the document")
	}
}

func TestLocalizeReviewRequiresOptIn(t *testing.T) {
	localizer, st, server := testSetup(t, pngHandler())
	// Remove the allowed-domain entry so the URL falls to the review default.
	localizer.cfg.AllowedDomains = nil
	localizer.policy = nil // rebuilt below via New to keep fields consistent
	localizer = New(localizer.cfg, st)

	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Review note",
		Body:  "![maybe](" + server.URL + "/maybe.png)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	withoutOptIn, err := localizer.LocalizeDocument(ctx, Options{DocumentID: doc.ID})
	if err != nil {
		t.Fatalf("LocalizeDocument: %v", err)
	}
	if len(withoutOptIn.Review) != 1 || len(withoutOptIn.Localized) != 0 || withoutOptIn.RevisionID != "" {
		t.Fatalf("review URLs must not localize without opt-in: %+v", withoutOptIn)
	}

	withOptIn, err := localizer.LocalizeDocument(ctx, Options{DocumentID: doc.ID, AllowReview: true})
	if err != nil {
		t.Fatalf("LocalizeDocument(AllowReview): %v", err)
	}
	if len(withOptIn.Localized) != 1 || withOptIn.RevisionID == "" {
		t.Fatalf("review URLs must localize with opt-in: %+v", withOptIn)
	}
}

func TestLocalizeRefusesHashBlockedContent(t *testing.T) {
	localizer, st, server := testSetup(t, pngHandler())
	ctx := context.Background()
	sum := sha256.Sum256(pngBytes)
	if err := st.AddMediaHashRule(ctx, store.MediaHashRule{
		Algo: "sha256", Hash: hex.EncodeToString(sum[:]), Kind: "exact", Action: "block", Reason: "known bad content",
	}); err != nil {
		t.Fatalf("AddMediaHashRule: %v", err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Hash blocked",
		Body:  "![pic](" + server.URL + "/a.png)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	result, err := localizer.LocalizeDocument(ctx, Options{DocumentID: doc.ID})
	if err != nil {
		t.Fatalf("LocalizeDocument: %v", err)
	}
	if len(result.Blocked) != 1 || !strings.Contains(result.Blocked[0]["reason"].(string), "known bad content") {
		t.Fatalf("hash-blocked content must be refused: %+v", result)
	}
	if len(result.Localized) != 0 || result.RevisionID != "" {
		t.Fatalf("hash-blocked content must not rewrite: %+v", result)
	}
	after, _ := st.GetDocument(ctx, doc.ID)
	if after.Body != doc.Body {
		t.Fatalf("body must be unchanged")
	}
	refs, _ := st.ListDocumentResources(ctx, doc.ID)
	if len(refs) != 0 {
		t.Fatalf("blocked bytes must not reach the resource store: %+v", refs)
	}
}

func TestLocalizeEnforcesBaseRevisionPrecondition(t *testing.T) {
	localizer, st, server := testSetup(t, pngHandler())
	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Stale revision",
		Body:  "![pic](" + server.URL + "/a.png)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	_, err = localizer.LocalizeDocument(ctx, Options{DocumentID: doc.ID, BaseRevisionID: "rev_stale"})
	if err == nil {
		t.Fatalf("stale base revision must fail the rewrite")
	}
}

func TestLocalizeRefusesReadOnlyNotes(t *testing.T) {
	localizer, st, _ := testSetup(t, pngHandler())
	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Trash me", Body: "x"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if _, err := localizer.LocalizeDocument(ctx, Options{DocumentID: doc.ID}); err == nil {
		t.Fatalf("trashed notes must be refused")
	}
}

func TestLocalizeDeduplicatesByContent(t *testing.T) {
	localizer, st, server := testSetup(t, pngHandler())
	ctx := context.Background()
	docA, _ := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "A", Body: "![p](" + server.URL + "/a.png)\n"})
	docB, _ := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "B", Body: "![p](" + server.URL + "/b.png)\n"})

	resultA, err := localizer.LocalizeDocument(ctx, Options{DocumentID: docA.ID})
	if err != nil {
		t.Fatalf("LocalizeDocument A: %v", err)
	}
	resultB, err := localizer.LocalizeDocument(ctx, Options{DocumentID: docB.ID})
	if err != nil {
		t.Fatalf("LocalizeDocument B: %v", err)
	}
	// Same bytes → the content-addressed store holds one blob; the two
	// resources share its hash.
	if resultA.Localized[0]["sha256"] != resultB.Localized[0]["sha256"] {
		t.Fatalf("identical content must share the exact hash")
	}
}
