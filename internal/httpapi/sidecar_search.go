package httpapi

import (
	"context"
	"log"
	"time"

	"github.com/renesugar/notrios/internal/query"
	"github.com/renesugar/notrios/internal/store"
)

// searchMerged runs the canonical FTS5 search and, when a Recoll sidecar is
// attached, folds in sidecar-only hits (documents the derived index found
// that FTS5 missed — e.g. matches in tags, aliases, or attached-file text).
// FTS5 results always come first and remain authoritative; sidecar failures
// only log, never break search. Merging applies to the first page only so
// cursor pagination stays consistent, and never to trash queries (trashed
// notes are not projected).
func (s *Server) searchMerged(ctx context.Context, req store.SearchRequest) (store.SearchResponse, error) {
	result, err := s.store.Search(ctx, req)
	if err != nil {
		return result, err
	}
	if s.sidecar == nil || req.Cursor != "" || len(result.Hits) >= req.Limit {
		return result, nil
	}
	parsed := query.Parse(req.Query, time.Now())
	if parsed.IsEmpty() || parsed.Trashed {
		return result, nil
	}
	hits, err := s.sidecar.Search(ctx, parsed, req.Limit)
	if err != nil {
		log.Printf("search sidecar unavailable, using FTS5 only: %v", err)
		return result, nil
	}
	seen := map[string]bool{}
	for _, hit := range result.Hits {
		seen[hit.ID] = true
	}
	for _, hit := range hits {
		if len(result.Hits) >= req.Limit {
			break
		}
		if seen[hit.DocumentID] {
			continue
		}
		// Confirm against the canonical store: the projection may lag, and
		// GetDocument also filters out anything trashed since indexing.
		doc, err := s.store.GetDocument(ctx, hit.DocumentID)
		if err != nil {
			continue
		}
		seen[doc.ID] = true
		snippet := hit.Abstract
		if snippet == "" && len(doc.Body) > 0 {
			snippet = doc.Body
			if len(snippet) > 240 {
				snippet = snippet[:240]
			}
		}
		result.Hits = append(result.Hits, store.SearchHit{
			ID:           doc.ID,
			URI:          doc.URI,
			CollectionID: doc.CollectionID,
			NotebookID:   doc.NotebookID,
			Title:        doc.Title,
			Snippet:      snippet,
		})
	}
	return result, nil
}
