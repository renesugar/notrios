package facadeprobe

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func testStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Bootstrap(context.Background()); err != nil {
		s.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestFacadeCreateGetUpdateSearchMatchesStore(t *testing.T) {
	s := testStore(t)
	f := New(s)
	ctx := context.Background()
	created, err := f.Create(ctx, CreateNoteInput{ID: "doc_probe", CollectionID: "default", Title: "Facade title", Body: "alpha body", MIMEType: "text/markdown"})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := s.GetDocument(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != direct.ID || created.RevisionID != direct.CurrentRevisionID {
		t.Fatalf("create mismatch: facade=%+v direct=%+v", created, direct)
	}
	got, err := f.Get(ctx, direct.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != direct.ID || got.Body != direct.Body || got.RevisionID != direct.CurrentRevisionID {
		t.Fatalf("facade=%+v direct=%+v", got, direct)
	}
	updated, err := f.Update(ctx, UpdateNoteInput{ID: got.ID, RevisionID: got.RevisionID, Title: "Updated", Body: "alpha updated", MIMEType: "text/markdown"})
	if err != nil {
		t.Fatal(err)
	}
	directUpdated, err := s.GetDocument(ctx, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Body != directUpdated.Body || updated.RevisionID != directUpdated.CurrentRevisionID {
		t.Fatalf("update mismatch: %+v / %+v", updated, directUpdated)
	}
	_, err = f.Update(ctx, UpdateNoteInput{ID: got.ID, RevisionID: got.RevisionID, Title: "Stale", Body: "stale", MIMEType: "text/markdown"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update error=%v", err)
	}
	result, err := f.Search(ctx, SearchInput{Query: "updated", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].ID != direct.ID {
		t.Fatalf("search=%+v", result)
	}
}

func TestFacadeMapsNotFoundAndCancellation(t *testing.T) {
	f := New(testStore(t))
	_, err := f.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = f.Get(ctx, "missing")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestResourceStreamBoundsAndCancellation(t *testing.T) {
	// The real store path is exercised by creating a content-addressed resource.
	s := testStore(t)
	f := New(s)
	ctx := context.Background()
	r, err := s.CreateResource(ctx, store.CreateResourceRequest{CollectionID: "default", Filename: "x.txt", MIMEType: "text/plain", Content: strings.NewReader("abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := f.OpenResource(ctx, r.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
	if string(b) != "abc" {
		t.Fatalf("bounded content=%q", b)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	stream, err = f.OpenResource(ctx, r.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	stream.ctx = cctx
	if _, err := stream.Read(make([]byte, 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
}
