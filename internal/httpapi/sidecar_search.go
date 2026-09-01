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
	"github.com/renesugar/notrios/internal/recoll"
	"github.com/renesugar/notrios/internal/store"
)

const (
	mergedSearchWindow      = 1000
	mergedSearchCacheSize   = 32
	mergedSearchSnapshotTTL = 10 * time.Minute
)

type mergedSearchSnapshot struct {
	ID           string
	QueryBinding string
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
// This orchestration deliberately stays on the store in v0.8 H1 slice B.
// Routing it through application.Search would convert every hit twice per page
// while the merge, cursor, and snapshot code around it still works in store
// types, and moving the merge itself into the facade would make the facade own
// the optional Recoll sidecar and the query parser. That is a design decision
// about where sidecar search belongs, not a mechanical migration, so it gets
// its own slice rather than a fork inside this one.
func (s *Server) searchMerged(ctx context.Context, req store.SearchRequest) (store.SearchResponse, error) {
	req = store.NormalizeSearchRequest(req)
	parsed, err := query.Parse(req.Query, time.Now())
	if err != nil {
		return store.SearchResponse{}, fmt.Errorf("%w: %v", store.ErrInvalidInput, err)
	}
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
	addCanonicalSources(result.Hits, parsed.HasPositiveTextAnchor())
	if s.sidecar == nil || strings.TrimSpace(req.Cursor) != "" {
		return result, nil
	}
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
		addCanonicalSources(page.Hits, parsed.HasPositiveTextAnchor())
		all = append(all, page.Hits...)
		next = page.NextCursor
	}
	truncated := next != ""
	if len(all) > mergedSearchWindow {
		all = all[:mergedSearchWindow]
		truncated = true
	}

	canonicalIndexes := make(map[string]int, len(all))
	for index := range all {
		canonicalIndexes[all[index].ID] = index
	}
	sidecarContributed := false
	sidecarOrder := make([]recoll.Hit, 0, len(sidecarHits))
	candidateIDs := make([]string, 0, len(sidecarHits))
	seenSidecar := make(map[string]bool, len(sidecarHits))
	for _, hit := range sidecarHits {
		if strings.TrimSpace(hit.DocumentID) == "" || seenSidecar[hit.DocumentID] {
			continue
		}
		seenSidecar[hit.DocumentID] = true
		if index, exists := canonicalIndexes[hit.DocumentID]; exists {
			all[index].SearchSources = addSearchSource(all[index].SearchSources, "recoll")
			sidecarContributed = true
			continue
		}
		sidecarOrder = append(sidecarOrder, hit)
		candidateIDs = append(candidateIDs, hit.DocumentID)
	}
	documents := make(map[string]store.Document, len(candidateIDs))
	for start := 0; start < len(candidateIDs); start += 500 {
		end := min(start+500, len(candidateIDs))
		batch, err := s.store.GetDocuments(ctx, candidateIDs[start:end])
		if err != nil {
			return store.SearchResponse{}, err
		}
		for id, document := range batch {
			documents[id] = document
		}
	}
	for index, hit := range sidecarOrder {
		if len(all) >= mergedSearchWindow {
			truncated = truncated || index < len(sidecarOrder)
			break
		}
		doc, exists := documents[hit.DocumentID]
		if !exists || doc.CollectionID != req.CollectionID {
			continue
		}
		sidecarContributed = true
		snippet := hit.Abstract
		if snippet == "" {
			snippet = truncateRunes(doc.Body, 240)
		}
		all = append(all, store.SearchHit{
			ID:            doc.ID,
			URI:           doc.URI,
			CollectionID:  doc.CollectionID,
			NotebookID:    doc.NotebookID,
			Title:         doc.Title,
			Snippet:       snippet,
			UpdatedAt:     doc.UpdatedAt,
			SearchSources: []string{"recoll"},
		})
	}
	if len(sidecarHits) > mergedSearchWindow {
		truncated = true
	}
	if !sidecarContributed {
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
		QueryBinding: parsed.Canonical(),
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

func addCanonicalSources(hits []store.SearchHit, ftsRelevance bool) {
	source := "sqlite"
	if ftsRelevance {
		source = "fts5"
	}
	for index := range hits {
		hits[index].SearchSources = addSearchSource(hits[index].SearchSources, source)
	}
}

func addSearchSource(sources []string, source string) []string {
	for _, existing := range sources {
		if existing == source {
			return sources
		}
	}
	return append(sources, source)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func (s *Server) pageMergedSnapshot(cursor mergedSearchCursor, req store.SearchRequest) (store.SearchResponse, error) {
	snapshot, ok := s.searchCache.get(cursor.SnapshotID)
	if !ok {
		return store.SearchResponse{}, fmt.Errorf("%w: merged search snapshot expired", store.ErrInvalidCursor)
	}
	parsed, err := query.Parse(req.Query, time.Now())
	if err != nil {
		return store.SearchResponse{}, fmt.Errorf("%w: %v", store.ErrInvalidInput, err)
	}
	if snapshot.QueryBinding != parsed.Canonical() || snapshot.CollectionID != req.CollectionID {
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
