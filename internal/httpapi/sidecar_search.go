package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/renesugar/notrios/internal/query"
	"github.com/renesugar/notrios/internal/store"
)

const (
	mergedSearchWindow      = 1000
	mergedSearchCacheSize   = 32
	mergedSearchSnapshotTTL = 10 * time.Minute
)

type mergedSearchSnapshot struct {
	ID           string
	Query        string
	CollectionID string
	Hits         []store.SearchHit
	Truncated    bool
	CreatedAt    time.Time
}

type mergedSearchCache struct {
	mu        sync.Mutex
	snapshots map[string]mergedSearchSnapshot
	order     []string
}

type mergedSearchCursor struct {
	Version    string `json:"v"`
	SnapshotID string `json:"s"`
	Offset     int    `json:"o"`
}

func newMergedSearchCache() *mergedSearchCache {
	return &mergedSearchCache{snapshots: map[string]mergedSearchSnapshot{}}
}

// searchMerged uses normal keyset paging for canonical-only search. When the
// optional sidecar contributes results, it builds a bounded immutable merged
// snapshot so sidecar-only hits remain stable on every page. OFFSET exists
// only inside this explicitly bounded in-memory snapshot, never in SQLite.
func (s *Server) searchMerged(ctx context.Context, req store.SearchRequest) (store.SearchResponse, error) {
	req = store.NormalizeSearchRequest(req)
	if cursor, recognized, err := decodeMergedSearchCursor(req.Cursor); recognized {
		if err != nil {
			return store.SearchResponse{}, err
		}
		return s.pageMergedSnapshot(cursor, req)
	}

	result, err := s.store.Search(ctx, req)
	if err != nil {
		return result, err
	}
	if s.sidecar == nil || strings.TrimSpace(req.Cursor) != "" {
		return result, nil
	}
	parsed := query.Parse(req.Query, time.Now())
	if parsed.IsEmpty() || parsed.Trashed {
		return result, nil
	}

	sidecarHits, err := s.sidecar.Search(ctx, parsed, mergedSearchWindow+1)
	if err != nil {
		log.Printf("search sidecar unavailable, using FTS5 only: %v", err)
		return result, nil
	}
	if len(sidecarHits) == 0 {
		return result, nil
	}

	all := append([]store.SearchHit(nil), result.Hits...)
	next := result.NextCursor
	for len(all) < mergedSearchWindow && next != "" {
		page, err := s.store.Search(ctx, store.SearchRequest{
			CollectionID: req.CollectionID,
			Query:        req.Query,
			Limit:        min(100, mergedSearchWindow-len(all)),
			Cursor:       next,
		})
		if err != nil {
			return store.SearchResponse{}, err
		}
		all = append(all, page.Hits...)
		next = page.NextCursor
	}
	truncated := next != ""
	if len(all) > mergedSearchWindow {
		all = all[:mergedSearchWindow]
		truncated = true
	}

	seen := make(map[string]bool, len(all))
	for _, hit := range all {
		seen[hit.ID] = true
	}
	sidecarAdded := false
	for index, hit := range sidecarHits {
		if len(all) >= mergedSearchWindow {
			truncated = truncated || index < len(sidecarHits)
			break
		}
		if seen[hit.DocumentID] {
			continue
		}
		doc, err := s.store.GetDocument(ctx, hit.DocumentID)
		if err != nil || doc.CollectionID != req.CollectionID {
			continue
		}
		seen[doc.ID] = true
		sidecarAdded = true
		snippet := hit.Abstract
		if snippet == "" {
			snippet = doc.Body
			if len(snippet) > 240 {
				snippet = snippet[:240]
			}
		}
		all = append(all, store.SearchHit{
			ID:           doc.ID,
			URI:          doc.URI,
			CollectionID: doc.CollectionID,
			NotebookID:   doc.NotebookID,
			Title:        doc.Title,
			Snippet:      snippet,
			UpdatedAt:    doc.UpdatedAt,
		})
	}
	if len(sidecarHits) > mergedSearchWindow {
		truncated = true
	}
	if !sidecarAdded {
		return result, nil
	}

	if len(all) <= req.Limit {
		return store.SearchResponse{Hits: all, Truncated: truncated}, nil
	}
	snapshotID, err := store.NewID("search_snapshot")
	if err != nil {
		return store.SearchResponse{}, err
	}
	snapshot := mergedSearchSnapshot{
		ID:           snapshotID,
		Query:        req.Query,
		CollectionID: req.CollectionID,
		Hits:         all,
		Truncated:    truncated,
		CreatedAt:    time.Now().UTC(),
	}
	s.searchCache.put(snapshot)
	return store.SearchResponse{
		Hits:       append([]store.SearchHit(nil), all[:req.Limit]...),
		NextCursor: encodeMergedSearchCursor(snapshotID, req.Limit),
		Truncated:  truncated,
	}, nil
}

func (s *Server) pageMergedSnapshot(cursor mergedSearchCursor, req store.SearchRequest) (store.SearchResponse, error) {
	snapshot, ok := s.searchCache.get(cursor.SnapshotID)
	if !ok {
		return store.SearchResponse{}, fmt.Errorf("%w: merged search snapshot expired", store.ErrInvalidCursor)
	}
	if snapshot.Query != req.Query || snapshot.CollectionID != req.CollectionID {
		return store.SearchResponse{}, fmt.Errorf("%w: cursor does not match this query", store.ErrInvalidCursor)
	}
	if cursor.Offset < 0 || cursor.Offset >= len(snapshot.Hits) {
		return store.SearchResponse{}, fmt.Errorf("%w: snapshot boundary is invalid", store.ErrInvalidCursor)
	}
	end := min(cursor.Offset+req.Limit, len(snapshot.Hits))
	response := store.SearchResponse{
		Hits:      append([]store.SearchHit(nil), snapshot.Hits[cursor.Offset:end]...),
		Truncated: snapshot.Truncated,
	}
	if end < len(snapshot.Hits) {
		response.NextCursor = encodeMergedSearchCursor(snapshot.ID, end)
	}
	return response, nil
}

func (c *mergedSearchCache) put(snapshot mergedSearchSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked(time.Now().UTC())
	if len(c.order) >= mergedSearchCacheSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.snapshots, oldest)
	}
	c.snapshots[snapshot.ID] = snapshot
	c.order = append(c.order, snapshot.ID)
}

func (c *mergedSearchCache) get(id string) (mergedSearchSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked(time.Now().UTC())
	snapshot, ok := c.snapshots[id]
	return snapshot, ok
}

func (c *mergedSearchCache) pruneLocked(now time.Time) {
	kept := c.order[:0]
	for _, id := range c.order {
		snapshot, ok := c.snapshots[id]
		if !ok || now.Sub(snapshot.CreatedAt) > mergedSearchSnapshotTTL {
			delete(c.snapshots, id)
			continue
		}
		kept = append(kept, id)
	}
	c.order = kept
}

func encodeMergedSearchCursor(snapshotID string, offset int) string {
	raw, _ := json.Marshal(mergedSearchCursor{Version: "m1", SnapshotID: snapshotID, Offset: offset})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeMergedSearchCursor(encoded string) (mergedSearchCursor, bool, error) {
	if strings.TrimSpace(encoded) == "" {
		return mergedSearchCursor{}, false, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return mergedSearchCursor{}, false, nil
	}
	var cursor mergedSearchCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Version != "m1" {
		return mergedSearchCursor{}, false, nil
	}
	if strings.TrimSpace(cursor.SnapshotID) == "" || cursor.Offset <= 0 {
		return mergedSearchCursor{}, true, fmt.Errorf("%w: merged search cursor is invalid", store.ErrInvalidCursor)
	}
	return cursor, true, nil
}
