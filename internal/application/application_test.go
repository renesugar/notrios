package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// The facade is tested against the real SQLite store rather than a mock. The
// contract worth proving is that facade results agree with what the store
// actually did, and a mock can only prove the facade agrees with itself.
func testStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	backing, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := backing.Bootstrap(context.Background()); err != nil {
		backing.Close()
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { _ = backing.Close() })
	return backing
}

func testApplication(t *testing.T) (*Application, *store.SQLiteStore) {
	t.Helper()
	backing := testStore(t)
	return New(backing), backing
}

func mustCreate(t *testing.T, app *Application, title, body string) Note {
	t.Helper()
	created, err := app.CreateNote(context.Background(), CreateNoteInput{
		CollectionID: "default",
		Title:        title,
		Body:         body,
		MIMEType:     "text/markdown",
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	return created
}

// A real store satisfies the narrow Repository seam with no adapter. If this
// stops compiling, the facade has grown a dependency the store cannot meet.
var _ Repository = (*store.SQLiteStore)(nil)
var _ Repository = (store.Store)(nil)

func TestCreateGetUpdateAgreesWithStore(t *testing.T) {
	app, backing := testApplication(t)
	ctx := context.Background()

	created := mustCreate(t, app, "Facade title", "alpha body")
	direct, err := backing.GetDocument(ctx, created.ID)
	if err != nil {
		t.Fatalf("direct read: %v", err)
	}
	if created.ID != direct.ID || created.RevisionID != direct.CurrentRevisionID {
		t.Fatalf("create disagrees with store: facade=%+v direct=%+v", created, direct)
	}
	if created.Trashed() {
		t.Error("a newly created note reports itself trashed")
	}

	got, err := app.GetNote(ctx, created.ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if got.Body != direct.Body || got.Title != direct.Title || got.MIMEType != direct.BodyMIMEType {
		t.Fatalf("get disagrees with store: facade=%+v direct=%+v", got, direct)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Error("timestamps were dropped in translation")
	}

	updated, err := app.UpdateNote(ctx, UpdateNoteInput{
		ID: got.ID, RevisionID: got.RevisionID,
		Title: "Updated", Body: "alpha updated", MIMEType: "text/markdown",
	})
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updated.RevisionID == got.RevisionID {
		t.Error("update did not advance the revision")
	}
	directUpdated, err := backing.GetDocument(ctx, got.ID)
	if err != nil {
		t.Fatalf("direct read after update: %v", err)
	}
	if updated.Body != directUpdated.Body || updated.RevisionID != directUpdated.CurrentRevisionID {
		t.Fatalf("update disagrees with store: facade=%+v direct=%+v", updated, directUpdated)
	}
}

func TestStaleRevisionIsConflict(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	created := mustCreate(t, app, "Conflict", "body")

	if _, err := app.UpdateNote(ctx, UpdateNoteInput{
		ID: created.ID, RevisionID: created.RevisionID, Title: "First", Body: "first", MIMEType: "text/markdown",
	}); err != nil {
		t.Fatalf("first update: %v", err)
	}

	_, err := app.UpdateNote(ctx, UpdateNoteInput{
		ID: created.ID, RevisionID: created.RevisionID, Title: "Second", Body: "second", MIMEType: "text/markdown",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision: got %v, want a conflict", err)
	}
	if KindOf(err) != KindConflict {
		t.Errorf("KindOf = %v, want %v", KindOf(err), KindConflict)
	}
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatal("conflict is not an *Error")
	}
	if typed.Op != "update_note" {
		t.Errorf("Op = %q, want update_note", typed.Op)
	}
}

func TestMissingNoteIsNotFound(t *testing.T) {
	app, _ := testApplication(t)
	_, err := app.GetNote(context.Background(), "doc_does_not_exist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing note: got %v, want not found", err)
	}
	if errors.Is(err, ErrConflict) {
		t.Error("a not-found error also matched the conflict sentinel")
	}
}

func TestBlankIdentifiersAreRejectedBeforeTheRepository(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	cases := map[string]func() error{
		"get note":       func() error { _, err := app.GetNote(ctx, "  "); return err },
		"update note":    func() error { _, err := app.UpdateNote(ctx, UpdateNoteInput{}); return err },
		"delete note":    func() error { return app.DeleteNote(ctx, DeleteNoteInput{}) },
		"list revisions": func() error { _, err := app.ListRevisions(ctx, ""); return err },
		"get revision":   func() error { _, err := app.GetRevision(ctx, "doc_x", ""); return err },
		"get resource":   func() error { _, err := app.GetResource(ctx, ""); return err },
		"restore rev":    func() error { _, err := app.RestoreRevision(ctx, RestoreRevisionInput{NoteID: "doc_x"}); return err },
	}
	for name, call := range cases {
		if err := call(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v, want invalid input", name, err)
		}
	}
}

func TestRevisionHistoryAndRestore(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	created := mustCreate(t, app, "History", "first body")

	updated, err := app.UpdateNote(ctx, UpdateNoteInput{
		ID: created.ID, RevisionID: created.RevisionID,
		Title: "History", Body: "second body", MIMEType: "text/markdown",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	revisions, err := app.ListRevisions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revisions) < 2 {
		t.Fatalf("got %d revisions, want at least 2", len(revisions))
	}
	for _, item := range revisions {
		if item.NoteID != created.ID {
			t.Errorf("revision %s belongs to %q, want %q", item.ID, item.NoteID, created.ID)
		}
	}

	first, err := app.GetRevision(ctx, created.ID, created.RevisionID)
	if err != nil {
		t.Fatalf("get revision: %v", err)
	}
	if first.Body != "first body" {
		t.Errorf("revision body = %q, want the original", first.Body)
	}

	restored, err := app.RestoreRevision(ctx, RestoreRevisionInput{
		NoteID: created.ID, RevisionID: created.RevisionID, BaseRevisionID: updated.RevisionID,
	})
	if err != nil {
		t.Fatalf("restore revision: %v", err)
	}
	if restored.Body != "first body" {
		t.Errorf("restored body = %q, want the original", restored.Body)
	}
	// Restoring appends; it must not discard the revision it replaced.
	after, err := app.ListRevisions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list revisions after restore: %v", err)
	}
	if len(after) <= len(revisions) {
		t.Errorf("restore did not append a revision: %d then %d", len(revisions), len(after))
	}
}

func TestDeleteTrashesButKeepsTheNoteReadable(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	created := mustCreate(t, app, "Trash me", "body")

	if err := app.DeleteNote(ctx, DeleteNoteInput{ID: created.ID, RevisionID: created.RevisionID}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := app.GetNote(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("trashed note still readable through GetNote: %v", err)
	}
	recovered, err := app.GetNoteIncludingTrashed(ctx, created.ID)
	if err != nil {
		t.Fatalf("trashed note is unrecoverable: %v", err)
	}
	if !recovered.Trashed() {
		t.Error("recovered note does not report itself trashed")
	}
}

func TestSearchFindsCreatedNotes(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	mustCreate(t, app, "Zeppelin notes", "the airship body text")
	mustCreate(t, app, "Unrelated", "nothing to see")

	result, err := app.Search(ctx, SearchInput{CollectionID: "default", Query: "airship", Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Hits) != 1 {
		t.Fatalf("got %d hits, want 1: %+v", len(result.Hits), result.Hits)
	}
	if result.Hits[0].Title != "Zeppelin notes" {
		t.Errorf("hit title = %q", result.Hits[0].Title)
	}
}

func TestSearchRejectsUnknownSortAndNegativeLimit(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	if _, err := app.Search(ctx, SearchInput{Sort: "sideways"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("unknown sort: got %v, want invalid input", err)
	}
	if _, err := app.Search(ctx, SearchInput{Limit: -1}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("negative limit: got %v, want invalid input", err)
	}
	for _, sort := range []string{SortDefault, SortRelevance, SortUpdated} {
		if _, err := app.Search(ctx, SearchInput{CollectionID: "default", Query: "anything", Sort: sort, Limit: 5}); err != nil {
			t.Errorf("sort %q was rejected: %v", sort, err)
		}
	}
}

func TestSearchHitSourcesAreCopiedNotAliased(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	mustCreate(t, app, "Aliasing", "distinctive corpus word")

	first, err := app.Search(ctx, SearchInput{CollectionID: "default", Query: "distinctive", Limit: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(first.Hits) == 0 {
		t.Skip("no hit to inspect")
	}
	if len(first.Hits[0].Sources) == 0 {
		return
	}
	first.Hits[0].Sources[0] = "mutated-by-caller"
	second, err := app.Search(ctx, SearchInput{CollectionID: "default", Query: "distinctive", Limit: 5})
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if second.Hits[0].Sources[0] == "mutated-by-caller" {
		t.Error("a caller mutating a result changed what the next call returns")
	}
}

func TestCancelledContextIsRefusedBeforeTheRepository(t *testing.T) {
	app, _ := testApplication(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := app.GetNote(ctx, "doc_anything"); !errors.Is(err, context.Canceled) {
		t.Fatalf("get on cancelled context: got %v, want context.Canceled", err)
	}
	// Cancellation must stay comparable with errors.Is against the standard
	// library sentinel; re-typing it would break every select-driven caller.
	if _, err := app.CreateNote(ctx, CreateNoteInput{CollectionID: "default"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("create on cancelled context: got %v", err)
	}
	if KindOf(context.Canceled) != KindCanceled {
		t.Errorf("KindOf(context.Canceled) = %v", KindOf(context.Canceled))
	}
}

func TestNilContextIsRejected(t *testing.T) {
	app, _ := testApplication(t)
	//lint:ignore SA1012 the point of the test is that a nil context is refused
	if _, err := app.GetNote(nil, "doc_x"); !errors.Is(err, ErrInvalidInput) { //nolint:staticcheck
		t.Fatalf("nil context: got %v, want invalid input", err)
	}
}

func TestNewPanicsOnNilRepository(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(nil) did not panic")
		}
	}()
	New(nil)
}

func TestResourceStreamIsBoundedAndCancellable(t *testing.T) {
	app, backing := testApplication(t)
	ctx := context.Background()

	payload := strings.Repeat("resource-bytes ", 400)
	created, err := backing.CreateResource(ctx, store.CreateResourceRequest{
		CollectionID: "default",
		Filename:     "attachment.txt",
		MIMEType:     "text/plain",
		Content:      strings.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	metadata, err := app.GetResource(ctx, created.ID)
	if err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if metadata.SizeBytes != int64(len(payload)) || metadata.SHA256 == "" {
		t.Fatalf("resource metadata is wrong: %+v", metadata)
	}

	// The budget, not the payload, decides how much a caller can read.
	limit := int64(64)
	_, stream, err := app.OpenResource(ctx, created.ID, limit)
	if err != nil {
		t.Fatalf("open resource: %v", err)
	}
	read, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if int64(len(read)) != limit {
		t.Fatalf("read %d bytes, want the %d-byte budget", len(read), limit)
	}
	if string(read) != payload[:limit] {
		t.Error("bounded read returned the wrong bytes")
	}
	if stream.Remaining() != 0 {
		t.Errorf("Remaining = %d after exhausting the budget", stream.Remaining())
	}
	// Double close must be safe: the C ABI cannot guarantee a host releases a
	// handle exactly once.
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	// A full read gets everything.
	_, whole, err := app.OpenResource(ctx, created.ID, int64(len(payload))*2)
	if err != nil {
		t.Fatalf("open resource for full read: %v", err)
	}
	defer whole.Close()
	all, err := io.ReadAll(whole)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if string(all) != payload {
		t.Errorf("full read returned %d bytes, want %d", len(all), len(payload))
	}
}

func TestOpenResourceRejectsBadArguments(t *testing.T) {
	app, _ := testApplication(t)
	ctx := context.Background()
	if _, _, err := app.OpenResource(ctx, "res_x", 0); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("zero budget: got %v, want invalid input", err)
	}
	if _, _, err := app.OpenResource(ctx, "res_x", -1); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("negative budget: got %v, want invalid input", err)
	}
	if _, _, err := app.OpenResource(ctx, "", 10); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank id: got %v, want invalid input", err)
	}
	if _, _, err := app.OpenResource(ctx, "res_missing", 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing resource: got %v, want not found", err)
	}
}

func TestResourceStreamStopsOnCancellation(t *testing.T) {
	app, backing := testApplication(t)
	ctx, cancel := context.WithCancel(context.Background())

	created, err := backing.CreateResource(ctx, store.CreateResourceRequest{
		CollectionID: "default",
		Filename:     "cancel.txt",
		MIMEType:     "text/plain",
		Content:      strings.NewReader(strings.Repeat("x", 4096)),
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}
	_, stream, err := app.OpenResource(ctx, created.ID, 4096)
	if err != nil {
		t.Fatalf("open resource: %v", err)
	}
	defer stream.Close()

	buffer := make([]byte, 16)
	if _, err := stream.Read(buffer); err != nil {
		t.Fatalf("first read: %v", err)
	}
	cancel()
	if _, err := stream.Read(buffer); !errors.Is(err, context.Canceled) {
		t.Fatalf("read after cancel: got %v, want context.Canceled", err)
	}
}
