package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ17LinkRebuildTransactionCost measures what v1.0 J5 could only infer.
//
// J5 found the Obsidian importer 2.6x slower than the Joplin importer on the
// same 382,206 notes, and the code showed why it might be: both call the same
// rebuildDocumentLinksLocked, but RebuildDocumentLinks wraps each document in
// its own BEGIN IMMEDIATE/COMMIT while RebuildImportDocumentLinksBatch wraps a
// whole batch in one. That is 382,206 commits against 3,823.
//
// "Might be" is where J5 stopped, and arithmetic consistent with a hypothesis is
// not the hypothesis proven. This runs both paths over the *same* documents in
// the *same* library and reports the ratio, which is the cheap decisive
// experiment: minutes against the 4.5 hours a re-import would cost.
//
// It needs a real library, so it skips without one:
//
//	NOTRIOS_J17_LIBRARY=/path/to/notes.sqlite go test ./internal/store \
//	  -run TestJ17LinkRebuildTransactionCost -count=1 -v
//
// It only reads and rewrites link rows that rebuilding re-derives anyway, so it
// is safe against a corpus library -- but it writes, so it is pointed at J5's
// disposable copy rather than at anything a person owns.
func TestJ17LinkRebuildTransactionCost(t *testing.T) {
	path := os.Getenv("NOTRIOS_J17_LIBRARY")
	if path == "" {
		t.Skip("set NOTRIOS_J17_LIBRARY to a library to measure against")
	}
	sample := 2000
	batchSize := 500 // maxImportLookupItems

	st, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	// A search rather than a scan: it is the listing this store offers, and any
	// document it returns is a real one with a real body, which is what the
	// rebuild cost depends on.
	// Paged, because Search caps a page at the configured max_limit -- 100 by
	// default -- and asking for 2,000 in one call silently returns 100. The
	// first version of this test did exactly that and failed with "only 100
	// documents available", which is the clamp doing its job and this test
	// having assumed otherwise.
	ids := make([]string, 0, sample)
	cursor := ""
	for len(ids) < sample {
		found, err := st.Search(ctx, store.SearchRequest{Query: "the", Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(found.Hits) == 0 {
			break
		}
		for _, hit := range found.Hits {
			ids = append(ids, hit.ID)
		}
		if found.NextCursor == "" {
			break
		}
		cursor = found.NextCursor
	}
	if len(ids) < batchSize {
		t.Fatalf("only %d documents available; need at least %d", len(ids), batchSize)
	}
	t.Logf("measuring %d documents in a library at %s", len(ids), path)

	// Per document: one transaction each, which is what the Obsidian importer
	// does today.
	started := time.Now()
	for _, id := range ids {
		if err := st.RebuildDocumentLinks(ctx, id); err != nil {
			t.Fatalf("per-document rebuild of %s: %v", id, err)
		}
	}
	perDocument := time.Since(started)

	// Batched: one transaction per batchSize documents, which is what the
	// Joplin importer does. The same documents, immediately afterwards, so the
	// page cache favours this run rather than the one being argued for.
	started = time.Now()
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		// The batch call also records an import checkpoint, which is part of
		// what it is for: the Joplin importer resumes from it. That is extra
		// work this path does and the per-document path does not, so if the
		// batch still wins it wins carrying weight.
		if err := st.RebuildImportDocumentLinksBatch(ctx, store.ImportLinkBatchRequest{
			DocumentIDs: ids[start:end],
			Checkpoint: store.ImportCheckpoint{
				SourceSystem:         "j17-measurement",
				SourceKey:            path,
				CollectionID:         "default",
				InventoryFingerprint: "j17-measurement-fingerprint",
				Phase:                "links",
				NextIndex:            end,
				TotalItems:           len(ids),
				Status:               "running",
			},
		}); err != nil {
			t.Fatalf("batch rebuild [%d:%d]: %v", start, end, err)
		}
	}
	batched := time.Since(started)

	perDocumentEach := perDocument / time.Duration(len(ids))
	batchedEach := batched / time.Duration(len(ids))
	t.Logf("per-document: %s total, %s per document (%d transactions)",
		perDocument.Round(time.Millisecond), perDocumentEach.Round(time.Microsecond), len(ids))
	t.Logf("batched:      %s total, %s per document (%d transactions)",
		batched.Round(time.Millisecond), batchedEach.Round(time.Microsecond),
		(len(ids)+batchSize-1)/batchSize)
	if batched > 0 {
		t.Logf("ratio: %.2fx", float64(perDocument)/float64(batched))
		t.Logf("extrapolation NOT performed: 382,206 documents were not measured here")
	}
}

// TestJ17DocumentWriteTransactionCost measures the second theory, after the
// first one failed.
//
// J5's gap between the two importers is 2.6x on the same notes. J17-A measured
// the link rebuild and found 1.51x, which does not account for it. The
// remaining structural difference is the document write itself: the Obsidian
// importer calls UpdateDocument and SetDocumentSource per document -- and
// UpdateDocument opens its own BEGIN IMMEDIATE -- while the Joplin importer
// hands a whole batch to ApplyImportDocumentBatch.
//
// Both paths here write the same bodies to the same documents and set the same
// source, so the only difference is how many transactions carry the work. The
// batch path additionally records item state and a checkpoint, which the
// per-document path does not, so again it wins carrying weight if it wins.
//
//	NOTRIOS_J17_LIBRARY=/path/to/notes.sqlite go test ./internal/store \
//	  -run TestJ17DocumentWriteTransactionCost -count=1 -v
func TestJ17DocumentWriteTransactionCost(t *testing.T) {
	path := os.Getenv("NOTRIOS_J17_LIBRARY")
	if path == "" {
		t.Skip("set NOTRIOS_J17_LIBRARY to a library to measure against")
	}
	// 400 per path: the per-document half of the first run took most of 17
	// minutes, and a measurement that costs a coffee break gets run once and
	// believed. This is enough to separate the two paths.
	const sample = 800
	const batchSize = 400

	st, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	documents := make([]store.Document, 0, sample)
	cursor := ""
	for len(documents) < sample {
		found, err := st.Search(ctx, store.SearchRequest{Query: "the", Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(found.Hits) == 0 {
			break
		}
		for _, hit := range found.Hits {
			document, err := st.GetDocument(ctx, hit.ID)
			if err != nil {
				t.Fatal(err)
			}
			documents = append(documents, document)
		}
		if found.NextCursor == "" {
			break
		}
		cursor = found.NextCursor
	}
	if len(documents) < 2*batchSize {
		t.Fatalf("only %d documents available; need %d", len(documents), 2*batchSize)
	}
	documents = documents[:sample]
	half := len(documents) / 2
	// Two disjoint halves, because a document that has just been rewritten is
	// not in the same state as one that has not, and running both paths over
	// the same rows would hand the second one a warmed page cache and a
	// different revision chain.
	perDocumentSet, batchSet := documents[:half], documents[half:]
	t.Logf("measuring %d documents per path in %s", half, path)

	started := time.Now()
	for _, document := range perDocumentSet {
		updated, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID: document.ID, Title: document.Title, Body: document.Body + "\n",
			BaseRevisionID: document.CurrentRevisionID, Message: "j17 measurement",
		})
		if err != nil {
			t.Fatalf("per-document update of %s: %v", document.ID, err)
		}
		if _, err := st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{
			DocumentID: updated.ID, SourceSystem: "j17-measurement", ExternalID: updated.ID,
		}); err != nil {
			t.Fatalf("per-document source of %s: %v", document.ID, err)
		}
	}
	perDocument := time.Since(started)

	started = time.Now()
	for start := 0; start < len(batchSet); start += batchSize {
		end := start + batchSize
		if end > len(batchSet) {
			end = len(batchSet)
		}
		mutations := make([]store.ImportDocumentMutation, 0, end-start)
		for _, document := range batchSet[start:end] {
			mutations = append(mutations, store.ImportDocumentMutation{
				Action: "update",
				Document: store.CreateDocumentRequest{
					PreferredID: document.ID, CollectionID: document.CollectionID,
					NotebookID: document.NotebookID, Title: document.Title,
					Body: document.Body + "\n", Message: "j17 measurement",
				},
				BaseRevisionID: document.CurrentRevisionID,
				Source: store.SetDocumentSourceRequest{
					DocumentID: document.ID, SourceSystem: "j17-measurement",
					ExternalID: document.ID,
				},
				// A real item state, not SkipState: the store refuses to skip
				// import metadata for a document that changed, which is
				// correct -- an import that rewrote a note and recorded
				// nothing about it could not resume. It is also more work for
				// the batch path than the per-document path does here.
				State: store.ImportItemState{
					SourceSystem: "j17-measurement", SourceKey: path,
					CollectionID: document.CollectionID, ItemKey: document.ID,
					ItemType: "note", Fingerprint: document.CurrentRevisionID,
					TargetID: document.ID, Action: "update", ProcessedAt: time.Now(),
				},
			})
		}
		if err := st.ApplyImportDocumentBatch(ctx, store.ImportDocumentBatchRequest{
			Documents: mutations,
			Checkpoint: store.ImportCheckpoint{
				SourceSystem: "j17-measurement", SourceKey: path, CollectionID: "default",
				InventoryFingerprint: "j17-measurement-fingerprint", Phase: "documents",
				NextIndex: end, TotalItems: len(batchSet), Status: "running",
			},
		}); err != nil {
			t.Fatalf("document batch [%d:%d]: %v", start, end, err)
		}
	}
	batched := time.Since(started)

	t.Logf("per-document: %s total, %s each (%d documents, %d transactions at minimum)",
		perDocument.Round(time.Millisecond),
		(perDocument / time.Duration(half)).Round(time.Microsecond), half, half)
	t.Logf("batched:      %s total, %s each (%d documents, %d transactions)",
		batched.Round(time.Millisecond),
		(batched / time.Duration(len(batchSet))).Round(time.Microsecond),
		len(batchSet), (len(batchSet)+batchSize-1)/batchSize)
	if batched > 0 {
		t.Logf("ratio: %.2fx", float64(perDocument)/float64(batched))
	}
	t.Log("extrapolation NOT performed: this is one phase over 1,000 documents, " +
		"not an import of 382,206")
}
