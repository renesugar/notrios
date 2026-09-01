package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"container/heap"
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Bounded graph traversal over the canonical link table.
//
// Nothing here uses a recursive CTE. A recursive walk over `document_links`
// decides how far it has gone only after SQLite has already walked, so the
// ceiling would be applied to a result rather than to the work. Expansion is
// therefore level by level in Go, one bounded `IN (...)` query per batch of the
// frontier, which is what makes MaxNodes and MaxEdges real limits on effort
// rather than on output.

// graphLinkRow is one link as traversal needs it: endpoints, kind, and enough
// to render an edge. Bodies, contexts, and offsets are deliberately absent.
type graphLinkRow struct {
	id               int64
	sourceID         string
	targetDocumentID string
	targetResourceID string
	targetURI        string
	relationType     string
	rawTarget        string
	resolutionStatus string
}

// graphDocument is the metadata a node needs. Loading it through
// getDocumentLocked would join the current revision and pull a body per node,
// which is thousands of bodies for one graph.
type graphDocument struct {
	id           string
	collectionID string
	title        string
	notebookID   string
}

func graphPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// liveDocumentsLocked returns the subset of ids that are live documents.
func (s *SQLiteStore) liveDocumentsLocked(ids []string) (map[string]graphDocument, error) {
	found := map[string]graphDocument{}
	for start := 0; start < len(ids); start += graphFrontierBatch {
		end := start + graphFrontierBatch
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		stmt, err := s.prepareLocked(`SELECT id, collection_id, COALESCE(title, ''), COALESCE(notebook_id, '')
			FROM documents
			WHERE deleted_at IS NULL AND id IN (` + graphPlaceholders(len(batch)) + `)`)
		if err != nil {
			return nil, err
		}
		if err := bindAll(stmt, batch); err != nil {
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		for {
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_DONE {
				break
			}
			if rc != C.SQLITE_ROW {
				err := s.stepErrLocked(rc)
				C.sqlite3_finalize(stmt)
				return nil, err
			}
			doc := graphDocument{
				id:           columnText(stmt, 0),
				collectionID: columnText(stmt, 1),
				title:        columnText(stmt, 2),
				notebookID:   columnText(stmt, 3),
			}
			found[doc.id] = doc
		}
		C.sqlite3_finalize(stmt)
	}
	return found, nil
}

// graphResourcesLocked loads the resource metadata a node needs.
func (s *SQLiteStore) graphResourcesLocked(ids []string) (map[string]Resource, error) {
	found := map[string]Resource{}
	for start := 0; start < len(ids); start += graphFrontierBatch {
		end := start + graphFrontierBatch
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		stmt, err := s.prepareLocked(`SELECT id, collection_id, COALESCE(filename, '')
			FROM resources WHERE id IN (` + graphPlaceholders(len(batch)) + `)`)
		if err != nil {
			return nil, err
		}
		if err := bindAll(stmt, batch); err != nil {
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		for {
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_DONE {
				break
			}
			if rc != C.SQLITE_ROW {
				err := s.stepErrLocked(rc)
				C.sqlite3_finalize(stmt)
				return nil, err
			}
			id := columnText(stmt, 0)
			found[id] = Resource{
				ID:           id,
				CollectionID: columnText(stmt, 1),
				Filename:     columnText(stmt, 2),
				URI:          ResourceURI(columnText(stmt, 1), id),
			}
		}
		C.sqlite3_finalize(stmt)
	}
	return found, nil
}

// graphLinksLocked reads every link touching one batch of frontier IDs.
// `incoming` selects the direction: the batch is matched against the target
// column instead of the source column, both of which are indexed.
func (s *SQLiteStore) graphLinksLocked(batch []string, incoming bool) ([]graphLinkRow, error) {
	matchColumn := "source_document_id"
	order := "source_document_id, source_line, source_column, id"
	if incoming {
		matchColumn = "target_document_id"
		order = "target_document_id, source_document_id, source_line, source_column, id"
	}
	stmt, err := s.prepareLocked(`SELECT id, source_document_id,
			COALESCE(target_document_id, ''), COALESCE(target_resource_id, ''),
			COALESCE(target_uri, ''), COALESCE(relation_type, ''),
			COALESCE(raw_target, ''), COALESCE(resolution_status, '')
		FROM document_links
		WHERE ` + matchColumn + ` IN (` + graphPlaceholders(len(batch)) + `)
		ORDER BY ` + order)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, batch); err != nil {
		return nil, err
	}
	rows := []graphLinkRow{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return rows, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		rows = append(rows, graphLinkRow{
			id:               columnInt64(stmt, 0),
			sourceID:         columnText(stmt, 1),
			targetDocumentID: columnText(stmt, 2),
			targetResourceID: columnText(stmt, 3),
			targetURI:        columnText(stmt, 4),
			relationType:     columnText(stmt, 5),
			rawTarget:        columnText(stmt, 6),
			resolutionStatus: columnText(stmt, 7),
		})
	}
}

// graphBuilder accumulates one bounded neighbourhood. It owns the ceilings so
// every insertion point checks the same two limits.
type graphBuilder struct {
	resp      *GraphResponse
	nodeDepth map[string]int
	edgeSeen  map[int64]bool
	maxNodes  int
	maxEdges  int
	// roots are the notes the caller named. A read-only builtin note's own
	// links are followed when it is a root and ignored otherwise — see
	// addGraphRow.
	roots map[string]bool
}

func newGraphBuilder(resp *GraphResponse, maxNodes, maxEdges int) *graphBuilder {
	return &graphBuilder{
		resp:      resp,
		nodeDepth: map[string]int{},
		edgeSeen:  map[int64]bool{},
		maxNodes:  maxNodes,
		maxEdges:  maxEdges,
		roots:     map[string]bool{},
	}
}

func (b *graphBuilder) truncated() bool { return b.resp.Truncated }

// addNode returns true when the node is present after the call. A node that
// does not fit truncates the graph rather than being silently dropped.
func (b *graphBuilder) addNode(node GraphNode) bool {
	if node.ID == "" {
		return false
	}
	if _, ok := b.nodeDepth[node.ID]; ok {
		return true
	}
	if len(b.resp.Nodes) >= b.maxNodes {
		b.resp.Truncated = true
		b.resp.TruncatedBy = GraphTruncatedByNodes
		return false
	}
	b.nodeDepth[node.ID] = node.Depth
	b.resp.Nodes = append(b.resp.Nodes, node)
	return true
}

func (b *graphBuilder) addEdge(id int64, edge GraphEdge) bool {
	if b.edgeSeen[id] {
		return true
	}
	if len(b.resp.Edges) >= b.maxEdges {
		b.resp.Truncated = true
		b.resp.TruncatedBy = GraphTruncatedByEdges
		return false
	}
	b.edgeSeen[id] = true
	b.resp.Edges = append(b.resp.Edges, edge)
	return true
}

// Graph expands the neighbourhood of the requested roots to the requested
// depth.
//
// Depth was accepted and ignored until v0.5 E4: the MVP implementation always
// returned the roots' immediate links, so a caller asking for three hops
// received one with nothing saying so.
func (s *SQLiteStore) Graph(ctx context.Context, req GraphRequest) (GraphResponse, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return GraphResponse{}, err
	}
	if err := req.validate(); err != nil {
		return GraphResponse{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	resp := GraphResponse{Nodes: []GraphNode{}, Edges: []GraphEdge{}, RequestedDepth: req.Depth}
	builder := newGraphBuilder(&resp, req.MaxNodes, req.MaxEdges)

	rootIDs := make([]string, 0, len(req.Roots))
	seenRoot := map[string]bool{}
	for _, root := range req.Roots {
		id := documentIDFromURI(root)
		if id == "" {
			id = strings.TrimSpace(root)
		}
		if id == "" || seenRoot[id] {
			continue
		}
		seenRoot[id] = true
		rootIDs = append(rootIDs, id)
	}
	roots, err := s.liveDocumentsLocked(rootIDs)
	if err != nil {
		return GraphResponse{}, err
	}
	frontier := make([]string, 0, len(rootIDs))
	for _, id := range rootIDs {
		doc, ok := roots[id]
		if !ok {
			continue
		}
		if !builder.addNode(GraphNode{ID: doc.id, URI: DocumentURI(doc.collectionID, doc.id), Kind: "document", Label: doc.title, Depth: 0}) {
			break
		}
		builder.roots[doc.id] = true
		frontier = append(frontier, doc.id)
	}
	if builder.truncated() {
		return resp, nil
	}

	for level := 0; level < req.Depth && len(frontier) > 0; level++ {
		if err := ctx.Err(); err != nil {
			return GraphResponse{}, err
		}
		next, err := s.expandGraphLevelLocked(builder, frontier, req, level+1)
		if err != nil {
			return GraphResponse{}, err
		}
		if builder.truncated() {
			return resp, nil
		}
		resp.CompletedDepth = level + 1
		frontier = next
	}
	// A frontier that empties before the requested depth means the reachable
	// graph was exhausted, which is a complete answer to the question asked.
	resp.CompletedDepth = req.Depth
	return resp, nil
}

// expandGraphLevelLocked walks one whole level and returns the next frontier.
func (s *SQLiteStore) expandGraphLevelLocked(builder *graphBuilder, frontier []string, req GraphRequest, depth int) ([]string, error) {
	next := []string{}
	for start := 0; start < len(frontier); start += graphFrontierBatch {
		end := start + graphFrontierBatch
		if end > len(frontier) {
			end = len(frontier)
		}
		batch := frontier[start:end]

		rows := []graphLinkRow{}
		if req.Direction == GraphDirectionOutgoing || req.Direction == GraphDirectionBoth {
			outgoing, err := s.graphLinksLocked(batch, false)
			if err != nil {
				return nil, err
			}
			rows = append(rows, outgoing...)
		}
		if req.Direction == GraphDirectionIncoming || req.Direction == GraphDirectionBoth {
			incoming, err := s.graphLinksLocked(batch, true)
			if err != nil {
				return nil, err
			}
			rows = append(rows, incoming...)
		}

		// One metadata load per level batch rather than one per edge.
		candidateDocs := []string{}
		candidateResources := []string{}
		seenDoc := map[string]bool{}
		seenResource := map[string]bool{}
		for _, row := range rows {
			for _, id := range []string{row.sourceID, row.targetDocumentID} {
				if id != "" && !seenDoc[id] {
					seenDoc[id] = true
					candidateDocs = append(candidateDocs, id)
				}
			}
			if req.IncludeResources && row.targetResourceID != "" && !seenResource[row.targetResourceID] {
				seenResource[row.targetResourceID] = true
				candidateResources = append(candidateResources, row.targetResourceID)
			}
		}
		docs, err := s.liveDocumentsLocked(candidateDocs)
		if err != nil {
			return nil, err
		}
		resources := map[string]Resource{}
		if len(candidateResources) > 0 {
			resources, err = s.graphResourcesLocked(candidateResources)
			if err != nil {
				return nil, err
			}
		}

		for _, row := range rows {
			added := s.addGraphRow(builder, row, docs, resources, req, depth)
			if builder.truncated() {
				return next, nil
			}
			next = append(next, added...)
		}
	}
	return next, nil
}

// addGraphRow places one link's endpoints and edge into the graph. It returns
// the newly reached documents, which is what the next level expands from — for
// an outgoing row that is the target, for an incoming row the source, and a
// resource or an unresolved target is always a leaf.
func (s *SQLiteStore) addGraphRow(builder *graphBuilder, row graphLinkRow, docs map[string]graphDocument, resources map[string]Resource, req GraphRequest, depth int) []string {
	if row.targetResourceID != "" && !req.IncludeResources {
		return nil
	}
	sourceDoc, ok := docs[row.sourceID]
	if !ok {
		// The source is trashed or gone. Soft delete clears a note's outgoing
		// links, so this is only reachable for a row mid-rebuild.
		return nil
	}
	// A system-authored note is not a neighbour of everything it names. The
	// graph report links to every hub it ranks, so without this each hub's
	// local graph would show the report at depth 1 — noise in exactly the view
	// F5 argues stays useful at scale, and the report is reachable from the
	// sidebar without being glued to the graph.
	//
	// **Unless it is the note the caller asked about.** Opening the report and
	// asking what surrounds it should show what it names; without the exemption
	// every Help page and the report itself would render an empty graph.
	if IsReadOnlyNotebook(sourceDoc.notebookID) && !builder.roots[sourceDoc.id] {
		return nil
	}

	reached := []string{}
	if _, known := builder.nodeDepth[row.sourceID]; !known {
		if !builder.addNode(GraphNode{ID: sourceDoc.id, URI: DocumentURI(sourceDoc.collectionID, sourceDoc.id), Kind: "document", Label: sourceDoc.title, Depth: depth}) {
			return nil
		}
		reached = append(reached, sourceDoc.id)
	}

	targetID := ""
	switch {
	case row.targetDocumentID != "":
		target, ok := docs[row.targetDocumentID]
		if !ok {
			// Points at a trashed or purged note. Lint reports that; a graph
			// showing it as a live neighbour would be wrong.
			return reached
		}
		targetID = target.id
		if _, known := builder.nodeDepth[targetID]; !known {
			if !builder.addNode(GraphNode{ID: target.id, URI: DocumentURI(target.collectionID, target.id), Kind: "document", Label: target.title, Depth: depth}) {
				return reached
			}
			reached = append(reached, target.id)
		}
	case row.targetResourceID != "":
		resource, ok := resources[row.targetResourceID]
		if !ok {
			return reached
		}
		targetID = resource.ID
		label := resource.Filename
		if label == "" {
			label = resource.ID
		}
		if !builder.addNode(GraphNode{ID: resource.ID, URI: resource.URI, Kind: "resource", Label: label, Depth: depth}) {
			return reached
		}
	default:
		// An unresolved target has no object to point at. It is kept as a leaf
		// node so a client can render a broken link, using the raw target as
		// its identity exactly as the MVP graph did.
		targetID = firstNonEmptyString(row.targetURI, row.rawTarget, "unresolved")
		if !builder.addNode(GraphNode{ID: targetID, URI: row.targetURI, Kind: firstNonEmptyString(row.resolutionStatus, "unresolved"), Label: targetID, Depth: depth}) {
			return reached
		}
	}

	builder.addEdge(row.id, GraphEdge{
		ID:        strconv.FormatInt(row.id, 10),
		SourceID:  row.sourceID,
		TargetID:  targetID,
		Kind:      row.relationType,
		Status:    row.resolutionStatus,
		RawTarget: row.rawTarget,
	})
	return reached
}

// pathStep records how a node was reached, so a meeting point can be unwound
// into a real path with real edges.
type pathStep struct {
	from string
	edge GraphEdge
}

// GraphPath finds a shortest link path between two notes.
//
// The search runs from both ends and expands whole levels alternately, taking
// the cheaper frontier each time. A note graph fans out fast — the single-ended
// search visits on the order of b^d nodes where the bidirectional one visits
// 2*b^(d/2) — and the visit budget is what makes the difference between an
// answer and a refusal on a large library.
//
// The three failure statuses are kept apart deliberately. `no_path` is a proof;
// `depth_exhausted` and `budget_exhausted` are admissions that the search
// stopped. Reporting either of the last two as `no_path` would tell a user two
// notes are unrelated when the search simply gave up.
func (s *SQLiteStore) GraphPath(ctx context.Context, req GraphPathRequest) (GraphPathResponse, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return GraphPathResponse{}, err
	}
	if err := req.validate(); err != nil {
		return GraphPathResponse{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	endpoints, err := s.liveDocumentsLocked([]string{req.From, req.To})
	if err != nil {
		return GraphPathResponse{}, err
	}
	if _, ok := endpoints[req.From]; !ok {
		return GraphPathResponse{}, ErrNotFound
	}
	if _, ok := endpoints[req.To]; !ok {
		return GraphPathResponse{}, ErrNotFound
	}

	resp := GraphPathResponse{
		Nodes:     []GraphNode{},
		Edges:     []GraphEdge{},
		MaxDepth:  req.MaxDepth,
		MaxVisits: req.MaxVisits,
	}
	if req.From == req.To {
		resp.Status = GraphPathFound
		resp.VisitedNodes = 1
		nodes, err := s.graphPathNodesLocked([]string{req.From})
		if err != nil {
			return GraphPathResponse{}, err
		}
		resp.Nodes = nodes
		return resp, nil
	}

	// "outgoing" walks links as written; "incoming" walks them backwards;
	// "both" ignores direction on each hop.
	forwardIncoming := req.Direction == GraphDirectionIncoming
	undirected := req.Direction == GraphDirectionBoth

	fwdDist := map[string]int{req.From: 0}
	bwdDist := map[string]int{req.To: 0}
	fwdParent := map[string]pathStep{}
	bwdParent := map[string]pathStep{}
	fwdFrontier := []string{req.From}
	bwdFrontier := []string{req.To}
	fwdDepth, bwdDepth := 0, 0

	for {
		if err := ctx.Err(); err != nil {
			return GraphPathResponse{}, err
		}
		resp.VisitedNodes = len(fwdDist) + len(bwdDist)
		if len(fwdFrontier) == 0 && len(bwdFrontier) == 0 {
			resp.Status = GraphPathNoPath
			return resp, nil
		}
		if fwdDepth+bwdDepth >= req.MaxDepth {
			resp.Status = GraphPathDepthExhausted
			return resp, nil
		}

		expandForward := len(bwdFrontier) == 0 ||
			(len(fwdFrontier) > 0 && len(fwdFrontier) <= len(bwdFrontier))

		var reached []string
		if expandForward {
			reached, err = s.expandPathLevelLocked(fwdFrontier, forwardIncoming, undirected, fwdDist, fwdParent, fwdDepth+1)
			if err != nil {
				return GraphPathResponse{}, err
			}
			fwdDepth++
			fwdFrontier = reached
		} else {
			reached, err = s.expandPathLevelLocked(bwdFrontier, !forwardIncoming, undirected, bwdDist, bwdParent, bwdDepth+1)
			if err != nil {
				return GraphPathResponse{}, err
			}
			bwdDepth++
			bwdFrontier = reached
		}
		resp.VisitedNodes = len(fwdDist) + len(bwdDist)

		// The shortest path through this level is the cheapest meeting node
		// among the ones just reached: whole levels are expanded, so every
		// closer meeting point was already checked.
		meeting, best := "", 0
		for _, id := range reached {
			fd, okF := fwdDist[id]
			bd, okB := bwdDist[id]
			if !okF || !okB {
				continue
			}
			if meeting == "" || fd+bd < best {
				meeting, best = id, fd+bd
			}
		}
		if meeting != "" {
			if best > req.MaxDepth {
				resp.Status = GraphPathDepthExhausted
				return resp, nil
			}
			return s.buildGraphPathLocked(resp, meeting, fwdParent, bwdParent, best)
		}

		if resp.VisitedNodes > req.MaxVisits {
			resp.Status = GraphPathBudgetExhausted
			return resp, nil
		}
	}
}

// expandPathLevelLocked visits one whole BFS level and records how each new
// node was reached. Only live document endpoints are followed: a resource is a
// leaf and a trashed target is not a neighbour.
func (s *SQLiteStore) expandPathLevelLocked(frontier []string, incoming, undirected bool, dist map[string]int, parent map[string]pathStep, depth int) ([]string, error) {
	reached := []string{}
	directions := []bool{incoming}
	if undirected {
		directions = []bool{false, true}
	}
	for start := 0; start < len(frontier); start += graphFrontierBatch {
		end := start + graphFrontierBatch
		if end > len(frontier) {
			end = len(frontier)
		}
		batch := frontier[start:end]
		for _, useIncoming := range directions {
			rows, err := s.pathLinksLocked(batch, useIncoming)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				neighbour := row.targetDocumentID
				from := row.sourceID
				if useIncoming {
					neighbour, from = row.sourceID, row.targetDocumentID
				}
				if neighbour == "" || from == "" {
					continue
				}
				if _, seen := dist[neighbour]; seen {
					continue
				}
				dist[neighbour] = depth
				parent[neighbour] = pathStep{
					from: from,
					edge: GraphEdge{
						ID:        strconv.FormatInt(row.id, 10),
						SourceID:  row.sourceID,
						TargetID:  row.targetDocumentID,
						Kind:      row.relationType,
						Status:    row.resolutionStatus,
						RawTarget: row.rawTarget,
					},
				}
				reached = append(reached, neighbour)
			}
		}
	}
	return reached, nil
}

// pathLinksLocked reads only document-to-document links whose far endpoint is a
// live note. The join is what keeps a trashed note out of a path.
func (s *SQLiteStore) pathLinksLocked(batch []string, incoming bool) ([]graphLinkRow, error) {
	matchColumn, joinColumn, order := "l.source_document_id", "l.target_document_id", "l.source_document_id, l.source_line, l.source_column, l.id"
	if incoming {
		matchColumn, joinColumn = "l.target_document_id", "l.source_document_id"
		order = "l.target_document_id, l.source_document_id, l.source_line, l.source_column, l.id"
	}
	stmt, err := s.prepareLocked(`SELECT l.id, l.source_document_id, l.target_document_id,
			COALESCE(l.relation_type, ''), COALESCE(l.raw_target, ''), COALESCE(l.resolution_status, '')
		FROM document_links l
		JOIN documents d ON d.id = ` + joinColumn + ` AND d.deleted_at IS NULL
		WHERE ` + matchColumn + ` IN (` + graphPlaceholders(len(batch)) + `)
			AND l.target_document_id IS NOT NULL
		ORDER BY ` + order)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, batch); err != nil {
		return nil, err
	}
	rows := []graphLinkRow{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return rows, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		rows = append(rows, graphLinkRow{
			id:               columnInt64(stmt, 0),
			sourceID:         columnText(stmt, 1),
			targetDocumentID: columnText(stmt, 2),
			relationType:     columnText(stmt, 3),
			rawTarget:        columnText(stmt, 4),
			resolutionStatus: columnText(stmt, 5),
		})
	}
}

// buildGraphPathLocked unwinds both parent chains through the meeting node.
func (s *SQLiteStore) buildGraphPathLocked(resp GraphPathResponse, meeting string, fwdParent, bwdParent map[string]pathStep, length int) (GraphPathResponse, error) {
	ids := []string{meeting}
	edges := []GraphEdge{}
	for node := meeting; ; {
		step, ok := fwdParent[node]
		if !ok {
			break
		}
		ids = append([]string{step.from}, ids...)
		edges = append([]GraphEdge{step.edge}, edges...)
		node = step.from
	}
	for node := meeting; ; {
		step, ok := bwdParent[node]
		if !ok {
			break
		}
		ids = append(ids, step.from)
		edges = append(edges, step.edge)
		node = step.from
	}
	nodes, err := s.graphPathNodesLocked(ids)
	if err != nil {
		return GraphPathResponse{}, err
	}
	resp.Status = GraphPathFound
	resp.Nodes = nodes
	resp.Edges = edges
	resp.Length = length
	return resp, nil
}

// graphPathNodesLocked labels the path in order. It is bounded by MaxDepth+1.
func (s *SQLiteStore) graphPathNodesLocked(ids []string) ([]GraphNode, error) {
	docs, err := s.liveDocumentsLocked(ids)
	if err != nil {
		return nil, err
	}
	nodes := make([]GraphNode, 0, len(ids))
	for i, id := range ids {
		doc := docs[id]
		nodes = append(nodes, GraphNode{
			ID:    id,
			URI:   DocumentURI(firstNonEmptyString(doc.collectionID, "default"), id),
			Kind:  "document",
			Label: doc.title,
			Depth: i,
		})
	}
	return nodes, nil
}

// hubHeap keeps the top-N notes by in-degree while streaming the library. A
// sorted insert would be O(documents × limit); this is O(documents × log limit)
// and holds `limit` entries, which is the point of a bounded report.
type hubHeap []GraphReportEntry

func (h hubHeap) Len() int { return len(h) }
func (h hubHeap) Less(i, j int) bool {
	if h[i].InDegree != h[j].InDegree {
		return h[i].InDegree < h[j].InDegree
	}
	// Among equal degrees the later ID is the weaker entry, so ties are broken
	// toward the lower ID and the report is deterministic.
	return h[i].DocumentID > h[j].DocumentID
}
func (h hubHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *hubHeap) Push(x any)   { *h = append(*h, x.(GraphReportEntry)) }
func (h *hubHeap) Pop() any     { old := *h; n := len(old); item := old[n-1]; *h = old[:n-1]; return item }

// GraphReport describes the whole link graph in one ordered scan.
//
// Every note is visited once and only bounded state is kept: two capped example
// lists and a `limit`-sized heap. That follows E2's lesson directly — the first
// lint implementation was slow because it read the link table once per check,
// and a whole-library report has to read the library once.
//
// **What it measures is the live library the user owns** — see
// measuredDocumentSQL. Two filters were missing before v0.6 F5:
//
//   - A **trashed** note's links still counted. The row set excluded trashed
//     notes, so one was never *ranked*, but the in-degree subquery placed no
//     condition on the link's source. A note in the Trash inflated the
//     in-degree of everything it had linked to, and a note linked only from the
//     Trash was never counted as an orphan. That was a pre-existing defect.
//   - A **system-authored** note's links counted too, which mattered the moment
//     this report began writing itself into the library: a report linking to
//     the top N hubs adds an incoming link to each of them and changes the
//     ranking the next generation sees. Excluding the report from its own
//     ranking would not have fixed that — the links still counted. The same
//     filter retires a quieter one: Notrios' own Help notes link to each other
//     heavily and had been inflating whatever they referenced all along.
//
// Both ends of a counted edge are filtered, not just the source, so LinkCount
// means what its documentation has always claimed: edges between two live
// notes that a traversal can actually follow.
func (s *SQLiteStore) GraphReport(ctx context.Context, req GraphReportRequest) (GraphReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return GraphReport{}, err
	}
	if err := req.validate(); err != nil {
		return GraphReport{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	started := time.Now()
	report := GraphReport{
		CollectionID: req.CollectionID,
		Limit:        req.Limit,
		Isolated:     []GraphReportEntry{},
		Orphans:      []GraphReportEntry{},
		Hubs:         []GraphReportEntry{},
	}

	// Both ends of every counted edge must be a note this report measures, and
	// so must the note being ranked. See measuredDocumentSQL for why each of the
	// three conditions is there — two of them were missing until F5.
	excluded := readOnlyNotebookArgs()
	measuredSource := measuredDocumentSQL("src")
	measuredTarget := measuredDocumentSQL("tgt")
	stmt, err := s.prepareLocked(`SELECT d.id, d.collection_id, COALESCE(d.title, ''),
			(SELECT COUNT(*) FROM document_links li
				JOIN documents src ON src.id = li.source_document_id
				WHERE li.target_document_id = d.id AND ` + measuredSource + `),
			(SELECT COUNT(*) FROM document_links lo
				JOIN documents tgt ON tgt.id = lo.target_document_id
				WHERE lo.source_document_id = d.id AND ` + measuredTarget + `)
		FROM documents d
		WHERE d.collection_id = ? AND ` + measuredDocumentSQL("d") + `
		ORDER BY d.id`)
	if err != nil {
		return GraphReport{}, err
	}
	defer C.sqlite3_finalize(stmt)
	// Bind order follows the statement: the in-degree subquery, the out-degree
	// subquery, then the collection and the row filter.
	args := append([]string{}, excluded...)
	args = append(args, excluded...)
	args = append(args, req.CollectionID)
	args = append(args, excluded...)
	if err := bindAll(stmt, args); err != nil {
		return GraphReport{}, err
	}

	hubs := &hubHeap{}
	heap.Init(hubs)
	for {
		if err := ctx.Err(); err != nil {
			return GraphReport{}, err
		}
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return GraphReport{}, s.stepErrLocked(rc)
		}
		entry := GraphReportEntry{
			DocumentID: columnText(stmt, 0),
			Title:      columnText(stmt, 2),
			InDegree:   columnInt64(stmt, 3),
			OutDegree:  columnInt64(stmt, 4),
		}
		entry.URI = DocumentURI(columnText(stmt, 1), entry.DocumentID)

		report.DocumentCount++
		report.LinkCount += entry.OutDegree
		if entry.InDegree == 0 {
			report.OrphanCount++
			if len(report.Orphans) < req.Limit {
				report.Orphans = append(report.Orphans, entry)
			} else {
				report.Truncated = true
			}
			if entry.OutDegree == 0 {
				report.IsolatedCount++
				if len(report.Isolated) < req.Limit {
					report.Isolated = append(report.Isolated, entry)
				} else {
					report.Truncated = true
				}
			}
		}
		if entry.InDegree > 0 {
			heap.Push(hubs, entry)
			if hubs.Len() > req.Limit {
				heap.Pop(hubs)
			}
		}
	}

	report.Hubs = append(report.Hubs, *hubs...)
	sort.Slice(report.Hubs, func(i, j int) bool {
		if report.Hubs[i].InDegree != report.Hubs[j].InDegree {
			return report.Hubs[i].InDegree > report.Hubs[j].InDegree
		}
		return report.Hubs[i].DocumentID < report.Hubs[j].DocumentID
	})
	report.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
	return report, nil
}
