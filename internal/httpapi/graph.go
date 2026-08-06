package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// Graph traversal over REST.
//
// All three routes are read-only and bounded before they run. A request naming a
// bound wider than the store's ceiling comes back `400 validation_failed`
// saying which ceiling it hit, rather than being clamped: a caller that asked
// for ten hops and received two cannot tell a small graph from a capped one.
//
// The service returns nodes, edges, and depths. It returns no coordinates, no
// clustering, and no layout — those are the client's, and putting them here
// would make every client take this one's opinion.

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	var req api.GraphRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, api.GraphResponse{Nodes: []api.GraphNode{}, Edges: []api.GraphEdge{}})
		return
	}
	graph, err := s.store.Graph(r.Context(), store.GraphRequest{
		Roots:            req.Roots,
		Direction:        req.Direction,
		Depth:            req.Depth,
		IncludeResources: req.IncludeResources,
		MaxNodes:         req.MaxNodes,
		MaxEdges:         req.MaxEdges,
	})
	if writeStoreError(w, err, "graph_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIGraph(graph))
}

func (s *Server) handleGraphPath(w http.ResponseWriter, r *http.Request) {
	var req api.GraphPathRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "path finding requires the canonical store")
		return
	}
	path, err := s.store.GraphPath(r.Context(), store.GraphPathRequest{
		From:      req.From,
		To:        req.To,
		Direction: req.Direction,
		MaxDepth:  req.MaxDepth,
		MaxVisits: req.MaxVisits,
	})
	if writeStoreError(w, err, "graph_path_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.GraphPathResponse{
		Status:       path.Status,
		Nodes:        toAPIGraphNodes(path.Nodes),
		Edges:        toAPIGraphEdges(path.Edges),
		Length:       path.Length,
		VisitedNodes: path.VisitedNodes,
		MaxDepth:     path.MaxDepth,
		MaxVisits:    path.MaxVisits,
	})
}

func (s *Server) handleGraphReport(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "graph reports require the canonical store")
		return
	}
	req := store.GraphReportRequest{CollectionID: r.URL.Query().Get("collection_id")}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
			return
		}
		req.Limit = limit
	}
	report, err := s.store.GraphReport(r.Context(), req)
	if writeStoreError(w, err, "graph_report_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.GraphReport{
		CollectionID:  report.CollectionID,
		DocumentCount: report.DocumentCount,
		LinkCount:     report.LinkCount,
		IsolatedCount: report.IsolatedCount,
		OrphanCount:   report.OrphanCount,
		Isolated:      toAPIGraphReportEntries(report.Isolated),
		Orphans:       toAPIGraphReportEntries(report.Orphans),
		Hubs:          toAPIGraphReportEntries(report.Hubs),
		Limit:         report.Limit,
		Truncated:     report.Truncated,
		ElapsedMS:     report.ElapsedMS,
	})
}

func toAPIGraph(graph store.GraphResponse) api.GraphResponse {
	return api.GraphResponse{
		Nodes:          toAPIGraphNodes(graph.Nodes),
		Edges:          toAPIGraphEdges(graph.Edges),
		Truncated:      graph.Truncated,
		TruncatedBy:    graph.TruncatedBy,
		RequestedDepth: graph.RequestedDepth,
		CompletedDepth: graph.CompletedDepth,
	}
}

func toAPIGraphNodes(nodes []store.GraphNode) []api.GraphNode {
	out := make([]api.GraphNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, api.GraphNode{
			ID:    node.ID,
			URI:   node.URI,
			Kind:  node.Kind,
			Label: node.Label,
			Depth: node.Depth,
		})
	}
	return out
}

func toAPIGraphEdges(edges []store.GraphEdge) []api.GraphEdge {
	out := make([]api.GraphEdge, 0, len(edges))
	for _, edge := range edges {
		out = append(out, api.GraphEdge{
			ID:        edge.ID,
			SourceID:  edge.SourceID,
			TargetID:  edge.TargetID,
			Kind:      edge.Kind,
			Status:    edge.Status,
			RawTarget: edge.RawTarget,
		})
	}
	return out
}

func toAPIGraphReportEntries(entries []store.GraphReportEntry) []api.GraphReportEntry {
	out := make([]api.GraphReportEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, api.GraphReportEntry{
			DocumentID: entry.DocumentID,
			URI:        entry.URI,
			Title:      entry.Title,
			InDegree:   entry.InDegree,
			OutDegree:  entry.OutDegree,
		})
	}
	return out
}
