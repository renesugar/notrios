package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/stablelink"
)

func stableLinkTestStore(t *testing.T) (*SQLiteStore, DatabaseIdentity) {
	t.Helper()
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	identity, err := st.GetDatabaseIdentity(context.Background())
	if err != nil {
		t.Fatalf("GetDatabaseIdentity: %v", err)
	}
	return st, identity
}

func TestStableDocumentURIUsesTheLogicalDatabaseIdentity(t *testing.T) {
	ctx := context.Background()
	st, identity := stableLinkTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Target", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	uri, err := st.StableDocumentURI(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("notrios://databases/%s/documents/%s", identity.DatabaseID, doc.ID)
	if uri != want {
		t.Fatalf("stable URI %q, want %q", uri, want)
	}
}

func TestResolveStableLinkAnswersForThisDatabaseOnly(t *testing.T) {
	ctx := context.Background()
	st, identity := stableLinkTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Live note", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := st.ResolveStableLink(ctx, stablelink.Format(identity.DatabaseID, doc.ID))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != StableLinkResolved || resolved.Title != "Live note" || resolved.DocumentURI == "" {
		t.Fatalf("live note: %+v", resolved)
	}

	stale, err := st.ResolveStableLink(ctx, stablelink.Format(identity.DatabaseID, "doc_missing"))
	if err != nil {
		t.Fatal(err)
	}
	if stale.Status != StableLinkStaleTarget || stale.Title != "" || stale.DocumentURI != "" {
		t.Fatalf("missing note: %+v", stale)
	}

	// A link naming another database must never be answered with a local
	// document, even when a document with exactly that ID exists here. IDs are
	// unique per database, not globally.
	foreign, err := st.ResolveStableLink(ctx, stablelink.Format("db_someone_else", doc.ID))
	if err != nil {
		t.Fatal(err)
	}
	if foreign.Status != StableLinkForeignDatabase {
		t.Fatalf("foreign database: %+v", foreign)
	}
	if foreign.Title != "" || foreign.DocumentURI != "" || foreign.NotebookID != "" {
		t.Fatalf("foreign link leaked local state: %+v", foreign)
	}
	if foreign.LocalDatabaseID != identity.DatabaseID {
		t.Fatalf("resolution should report which database answered: %+v", foreign)
	}
}

func TestResolveStableLinkReportsTrashedTargetsDistinctly(t *testing.T) {
	ctx := context.Background()
	st, identity := stableLinkTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Doomed", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	resolution, err := st.ResolveStableLink(ctx, stablelink.Format(identity.DatabaseID, doc.ID))
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Status != StableLinkTrashed || resolution.Title != "Doomed" {
		t.Fatalf("trashed note should be distinguishable from a purged one: %+v", resolution)
	}
}

func TestResolveStableLinkRejectsMalformedInput(t *testing.T) {
	ctx := context.Background()
	st, _ := stableLinkTestStore(t)
	for _, raw := range []string{
		"notrios://databases/db_a/documents/doc_b/extra",
		"notrios://databases/db_a/resources/res_b",
		"document://default/documents/doc_b",
		"https://example.com",
	} {
		if _, err := st.ResolveStableLink(ctx, raw); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%q: expected ErrInvalidInput, got %v", raw, err)
		}
	}
}

func TestStableLinksInNoteBodiesResolveLikeInternalLinks(t *testing.T) {
	ctx := context.Background()
	st, identity := stableLinkTestStore(t)
	target, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Target", Body: "target body"})
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(
		"[local](%s)\n[foreign](%s)\n[stale](%s)\n[broken](notrios://databases/db_a/documents/doc_b/extra)\n",
		stablelink.Format(identity.DatabaseID, target.ID),
		stablelink.Format("db_elsewhere", target.ID),
		stablelink.Format(identity.DatabaseID, "doc_gone"),
	)
	source, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Source", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	links, err := st.ListDocumentLinks(ctx, source.ID, "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	byText := map[string]DocumentLink{}
	for _, link := range links.Outgoing {
		byText[link.DisplayText] = link
	}
	if len(byText) != 4 {
		t.Fatalf("expected four outgoing links, got %d (%+v)", len(byText), links.Outgoing)
	}
	if got := byText["local"]; got.ResolutionStatus != "resolved" || got.TargetDocumentID != target.ID {
		t.Fatalf("local stable link: %+v", got)
	}
	if got := byText["foreign"]; got.ResolutionStatus != "external" || got.TargetDocumentID != "" {
		t.Fatalf("foreign stable link must not resolve locally: %+v", got)
	}
	if got := byText["stale"]; got.ResolutionStatus != "unresolved" || got.TargetDocumentID != "" {
		t.Fatalf("stale stable link: %+v", got)
	}
	if got := byText["broken"]; got.ResolutionStatus != "invalid" {
		t.Fatalf("malformed stable link: %+v", got)
	}
}

// A stable link is portable because it names the logical database rather than
// a path or a profile. Moving the SQLite file must therefore change nothing
// about how its own links resolve.
func TestStableLinksSurviveMovingTheDatabaseFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	original := filepath.Join(dir, "first", "notes.sqlite")
	st, err := OpenSQLite(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Target", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	uri := stablelink.Format(identity.DatabaseID, target.ID)
	source, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Source", Body: "[t](" + uri + ")"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	moved := filepath.Join(dir, "second", "renamed.sqlite")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(moved, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenSQLite(moved)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	movedIdentity, err := reopened.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if movedIdentity.DatabaseID != identity.DatabaseID {
		t.Fatalf("moving the file changed the logical database ID: %q -> %q", identity.DatabaseID, movedIdentity.DatabaseID)
	}
	resolution, err := reopened.ResolveStableLink(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Status != StableLinkResolved || resolution.Title != "Target" {
		t.Fatalf("link stopped resolving after the move: %+v", resolution)
	}
	links, err := reopened.ListDocumentLinks(ctx, source.ID, "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Outgoing) != 1 || links.Outgoing[0].TargetDocumentID != target.ID {
		t.Fatalf("stored link no longer points at the target: %+v", links.Outgoing)
	}
}
