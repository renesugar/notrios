package store

import (
	"context"
	"errors"
	"testing"
)

func TestDocumentSourceUpsertAndLookup(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	doc, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "tweet", Body: "hello"})

	src, err := st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID:   doc.ID,
		SourceSystem: "Twitter",
		ExternalID:   "post-189734",
		Author:       "Alice Smith",
		AuthorID:     "alice@example.social",
		ThreadID:     "thread-918",
		ReplyTo:      "post-189700",
		SourceURL:    "https://example.invalid/post/189734",
		PublishedAt:  "2026-07-13T18:42:07Z",
	})
	if err != nil {
		t.Fatalf("SetDocumentSource: %v", err)
	}
	if src.SourceSystem != "twitter" {
		t.Fatalf("source system must be normalized to lowercase: %+v", src)
	}
	if src.PublishedTS != 1783968127 {
		t.Fatalf("published_ts not derived from ISO timestamp: %+v", src)
	}

	// Upsert replaces fields for the same document.
	src, err = st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID:   doc.ID,
		SourceSystem: "twitter",
		ExternalID:   "post-189734",
		Author:       "Alice S.",
		AuthorID:     "alice@example.social",
		PublishedAt:  "1783968127", // epoch seconds accepted too
	})
	if err != nil || src.Author != "Alice S." || src.PublishedTS != 1783968127 {
		t.Fatalf("upsert failed: %+v err=%v", src, err)
	}

	got, err := st.GetDocumentSource(ctx, doc.ID)
	if err != nil || got.ExternalID != "post-189734" {
		t.Fatalf("GetDocumentSource: %+v err=%v", got, err)
	}

	found, err := st.FindDocumentBySource(ctx, "TWITTER", "post-189734")
	if err != nil || found != doc.ID {
		t.Fatalf("FindDocumentBySource: %q err=%v", found, err)
	}
	if _, err := st.FindDocumentBySource(ctx, "twitter", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Local notes have no provenance row.
	local, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "local", Body: "local"})
	if _, err := st.GetDocumentSource(ctx, local.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for local note, got %v", err)
	}
}

func TestThreadRecoveryInChronologicalOrder(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	mk := func(title, externalID, replyTo, published string) Document {
		t.Helper()
		doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: title, Body: title})
		if err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		if _, err := st.SetDocumentSource(ctx, SetDocumentSourceRequest{
			DocumentID:   doc.ID,
			SourceSystem: "twitter",
			ExternalID:   externalID,
			ThreadID:     "thread-1",
			ReplyTo:      replyTo,
			PublishedAt:  published,
		}); err != nil {
			t.Fatalf("SetDocumentSource: %v", err)
		}
		return doc
	}

	// Created intentionally out of chronological order.
	reply2 := mk("second reply", "p3", "p2", "2026-07-13T12:00:02Z")
	root := mk("root post", "p1", "", "2026-07-13T12:00:00Z")
	reply1 := mk("first reply", "p2", "p1", "2026-07-13T12:00:01Z")

	thread, err := st.ListThreadDocuments(ctx, "thread-1")
	if err != nil {
		t.Fatalf("ListThreadDocuments: %v", err)
	}
	if len(thread) != 3 || thread[0].DocumentID != root.ID || thread[1].DocumentID != reply1.ID || thread[2].DocumentID != reply2.ID {
		t.Fatalf("thread not in chronological order: %+v", thread)
	}
	if thread[1].ReplyTo != "p1" {
		t.Fatalf("reply_to lost: %+v", thread[1])
	}

	// Trashed notes disappear from thread recovery without being purged.
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: reply1.ID, BaseRevisionID: reply1.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	thread, _ = st.ListThreadDocuments(ctx, "thread-1")
	if len(thread) != 2 {
		t.Fatalf("trashed note must be excluded from thread, got %d", len(thread))
	}
}

func TestPurgeRefusesExternallySourcedNotes(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	doc, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "imported", Body: "imported"})
	if _, err := st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID:   doc.ID,
		SourceSystem: "chatgpt",
		ExternalID:   "conv-1/msg-1",
	}); err != nil {
		t.Fatalf("SetDocumentSource: %v", err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	if err := st.PurgeDocument(ctx, doc.ID); !errors.Is(err, ErrProtected) {
		t.Fatalf("expected ErrProtected purging externally-sourced note, got %v", err)
	}
	trash, _ := st.ListTrash(ctx, DocumentPageRequest{Limit: 10})
	if len(trash.Documents) != 1 {
		t.Fatalf("externally-sourced note must stay in trash, got %d", len(trash.Documents))
	}

	// Provenance can be set for trashed documents (importer backfill).
	if _, err := st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID:   doc.ID,
		SourceSystem: "chatgpt",
		ExternalID:   "conv-1/msg-1",
		Author:       "assistant",
	}); err != nil {
		t.Fatalf("SetDocumentSource on trashed doc: %v", err)
	}
}
