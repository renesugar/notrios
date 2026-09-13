package store_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ17WriteCostAgainstLibrarySize asks whether a single-document write costs
// what it costs because of the operation or because of the library.
//
// J17's two batching measurements agreed on something neither was looking for:
// updating one document in J5's 382,206-note library takes about **two
// seconds**, whether wrapped in its own transaction or in a batch of 400. That
// cannot be what an import pays -- the Joplin import wrote 382,206 notes in
// 1.72 h, about 16 ms each -- so either the cost appears only at size, or it is
// specific to that library.
//
// The difference matters beyond importing. If a write costs two seconds once a
// library is large, then *editing a note* costs two seconds, and that is a
// product fact rather than an importer one.
//
// Point this at libraries of different sizes, smallest first:
//
//	NOTRIOS_J17_LIBRARIES=/small/notes.sqlite:/large/notes.sqlite \
//	  go test ./internal/store -run TestJ17WriteCostAgainstLibrarySize -count=1 -v
func TestJ17WriteCostAgainstLibrarySize(t *testing.T) {
	list := os.Getenv("NOTRIOS_J17_LIBRARIES")
	if list == "" {
		t.Skip("set NOTRIOS_J17_LIBRARIES to colon-separated library paths")
	}
	const sample = 25

	for _, path := range strings.Split(list, ":") {
		if strings.TrimSpace(path) == "" {
			continue
		}
		t.Run(shortName(path), func(t *testing.T) {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			st, err := store.OpenSQLite(path)
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			ctx := context.Background()

			// Counted, not assumed: the point of the comparison is the size of
			// the library each write lands in.
			counted, err := store.SearchCount(ctx, st, store.SearchRequest{Query: "the"})
			if err != nil {
				t.Fatal(err)
			}

			found, err := st.Search(ctx, store.SearchRequest{Query: "the", Limit: sample})
			if err != nil {
				t.Fatal(err)
			}
			if len(found.Hits) == 0 {
				t.Skip("no documents to write to")
			}

			var total time.Duration
			written := 0
			for _, hit := range found.Hits {
				document, err := st.GetDocument(ctx, hit.ID)
				if err != nil {
					t.Fatal(err)
				}
				started := time.Now()
				if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
					ID: document.ID, Title: document.Title, Body: document.Body + "\n",
					BaseRevisionID: document.CurrentRevisionID, Message: "j17 write cost",
				}); err != nil {
					t.Fatalf("update %s: %v", document.ID, err)
				}
				total += time.Since(started)
				written++
			}
			t.Logf("%s: %d bytes, %d notes matching \"the\", %d writes, %s each",
				path, info.Size(), counted, written,
				(total / time.Duration(written)).Round(time.Millisecond))
		})
	}
}

func shortName(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/notes.sqlite"), "/")
	if len(parts) == 0 {
		return "library"
	}
	return fmt.Sprintf("%s", parts[len(parts)-1])
}
