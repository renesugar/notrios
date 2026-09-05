package store

import (
	"fmt"
	"strings"
)

// Bounded graph traversal: neighbours at a depth, the shortest path between two
// notes, and the orphan/hub shape of the whole link graph.
//
// Three rules shape this file.
//
// Every traversal is bounded before it starts. A request naming a wider bound
// than the ceilings below is refused rather than quietly clamped, because a
// caller that asked for 50,000 nodes and received 5,000 cannot tell a capped
// graph from a small one.
//
// A bound that stops a traversal is named in the result. `TruncatedBy` says
// which ceiling was reached and `CompletedDepth` says how far the expansion got
// before it did, so a partial neighbourhood is never presented as a complete
// one. This is the same reason search reports `truncated` for its bounded
// sidecar snapshot.
//
// Exhausting a path budget is not the same answer as there being no path.
// `GraphPathNoPath` means the traversal covered everything reachable and found
// nothing; `GraphPathBudgetExhausted` means it stopped early and knows nothing.
// Collapsing the two would let a large library report "these notes are
// unconnected" when they are merely far apart.

// Graph traversal ceilings. The verifier of a request refuses anything wider;
// `api/openapi.yaml` publishes the same numbers.
const (
	// MaxGraphDepth bounds neighbourhood expansion. Depth 0 is the roots alone.
	MaxGraphDepth = 5
	// MaxGraphNodes and MaxGraphEdges bound one neighbourhood result.
	MaxGraphNodes = 5000
	MaxGraphEdges = 20000
	// DefaultGraphNodes and DefaultGraphEdges apply when a caller names neither.
	// They match the defaults `api/openapi.yaml` has published since the MVP.
	DefaultGraphNodes = 250
	DefaultGraphEdges = 500
	// MaxGraphRoots bounds how many notes one expansion may start from.
	MaxGraphRoots = 100
	// graphFrontierBatch is how many frontier IDs go into one `IN (...)` query.
	// It matches the selection planner's batch size for the same reason: it
	// keeps one statement's bind list bounded regardless of frontier size.
	graphFrontierBatch = 400
)

// Shortest-path ceilings.
const (
	// MaxGraphPathDepth bounds how many hops a path may be. Beyond a handful of
	// hops in a note graph almost everything reaches almost everything, so a
	// longer answer stops being an answer.
	MaxGraphPathDepth = 10
	// DefaultGraphPathDepth is the hop bound when a caller names none.
	DefaultGraphPathDepth = 6
	// MaxGraphPathVisits bounds the total nodes both search frontiers may
	// visit. Reaching it ends the search as `budget_exhausted`, never as
	// `no_path`.
	MaxGraphPathVisits     = 200_000
	DefaultGraphPathVisits = 50_000
)

// Report ceilings.
const (
	// MaxGraphReportItems bounds each example list in a graph report. Complete
	// counts are always reported; only the examples are capped.
	MaxGraphReportItems     = 1000
	DefaultGraphReportItems = 100
)

// Graph traversal directions.
const (
	GraphDirectionOutgoing = "outgoing"
	GraphDirectionIncoming = "incoming"
	GraphDirectionBoth     = "both"
)

// GraphRequest expands the neighbourhood of one or more root notes.
type GraphRequest struct {
	Roots     []string `json:"roots"`
	Direction string   `json:"direction,omitempty"`
	// Depth is how many link hops to follow. Zero returns the roots and the
	// edges between them; it is not a synonym for "unbounded".
	Depth            int  `json:"depth,omitempty"`
	IncludeResources bool `json:"include_resources,omitempty"`
	MaxNodes         int  `json:"max_nodes,omitempty"`
	MaxEdges         int  `json:"max_edges,omitempty"`
}

// GraphNode is a document or resource node returned by graph expansion.
type GraphNode struct {
	ID    string `json:"id"`
	URI   string `json:"uri,omitempty"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
	// Depth is the hop distance from the nearest root. A client renders with
	// it; the service does not lay anything out.
	Depth int `json:"depth"`
}

// GraphEdge is one link edge returned by graph expansion.
type GraphEdge struct {
	ID        string `json:"id"`
	SourceID  string `json:"source_id"`
	TargetID  string `json:"target_id"`
	Kind      string `json:"kind,omitempty"`
	Status    string `json:"status,omitempty"`
	RawTarget string `json:"raw_target,omitempty"`
}

// Truncation reasons. An empty value means the traversal finished.
const (
	GraphTruncatedByNodes = "nodes"
	GraphTruncatedByEdges = "edges"
)

// GraphResponse contains one bounded neighbourhood.
type GraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	// Truncated is true when a ceiling stopped the expansion, and TruncatedBy
	// names which one.
	Truncated   bool   `json:"truncated,omitempty"`
	TruncatedBy string `json:"truncated_by,omitempty"`
	// RequestedDepth and CompletedDepth differ exactly when the expansion was
	// truncated: CompletedDepth is the deepest level that was expanded in full.
	RequestedDepth int `json:"requested_depth"`
	CompletedDepth int `json:"completed_depth"`
}

// GraphPathRequest asks for the shortest link path between two notes.
type GraphPathRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Direction "outgoing" follows links the way they were written; "both"
	// treats the graph as undirected, which is what a reader usually means by
	// "how are these two notes related". "incoming" reverses the edges.
	Direction string `json:"direction,omitempty"`
	MaxDepth  int    `json:"max_depth,omitempty"`
	MaxVisits int    `json:"max_visits,omitempty"`
}

// Shortest-path outcomes.
const (
	// GraphPathFound means Nodes/Edges describe a shortest path.
	GraphPathFound = "found"
	// GraphPathNoPath means everything reachable within MaxDepth was searched
	// and the target was not among it.
	GraphPathNoPath = "no_path"
	// GraphPathBudgetExhausted means the search stopped at MaxVisits. It is
	// deliberately distinct from GraphPathNoPath: nothing was proved.
	GraphPathBudgetExhausted = "budget_exhausted"
	// GraphPathDepthExhausted means the search reached MaxDepth without meeting.
	GraphPathDepthExhausted = "depth_exhausted"
)

// GraphPathResponse is one shortest path, or the reason there is none.
type GraphPathResponse struct {
	Status string `json:"status"`
	// Nodes runs from From to To inclusive; Edges has one fewer entry.
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	// Length is the hop count. Zero with status "found" means From == To.
	Length       int `json:"length"`
	VisitedNodes int `json:"visited_nodes"`
	MaxDepth     int `json:"max_depth"`
	MaxVisits    int `json:"max_visits"`
}

// GraphReportRequest asks for the shape of the whole link graph.
type GraphReportRequest struct {
	CollectionID string `json:"collection_id,omitempty"`
	// Limit caps each example list. Counts are always complete.
	Limit int `json:"limit,omitempty"`
}

// GraphReportEntry is one note in an example list.
type GraphReportEntry struct {
	DocumentID string `json:"document_id"`
	URI        string `json:"uri,omitempty"`
	Title      string `json:"title,omitempty"`
	InDegree   int64  `json:"in_degree"`
	OutDegree  int64  `json:"out_degree"`
}

// GraphReport describes the link graph without traversing it.
//
// A "hub" is ranked by in-degree rather than by total degree on purpose:
// out-degree is a property of how one author wrote one note, while in-degree is
// a property of how the rest of the library refers to it. The second is the one
// that says a note matters.
type GraphReport struct {
	CollectionID  string `json:"collection_id"`
	DocumentCount int64  `json:"document_count"`
	// LinkCount counts resolved links between two live documents — the edges a
	// traversal can actually follow.
	LinkCount int64 `json:"link_count"`
	// IsolatedCount is notes with no resolved document link in either
	// direction. OrphanCount is notes nothing links to, which includes the
	// isolated ones.
	IsolatedCount int64 `json:"isolated_count"`
	OrphanCount   int64 `json:"orphan_count"`

	Isolated []GraphReportEntry `json:"isolated"`
	Orphans  []GraphReportEntry `json:"orphans"`
	Hubs     []GraphReportEntry `json:"hubs"`

	Limit int `json:"limit"`
	// Truncated is true when an example list hit Limit. Counts are unaffected.
	Truncated bool `json:"truncated,omitempty"`
	// ElapsedMS records the cost of the scan, because this report reads the
	// whole library and an operator should be able to see what that cost.
	ElapsedMS float64 `json:"elapsed_ms"`
}

// normalizeGraphDirection accepts the three documented directions and refuses
// anything else rather than guessing at "out" or "in".
func normalizeGraphDirection(direction string, fallback string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "":
		return fallback, nil
	case GraphDirectionOutgoing:
		return GraphDirectionOutgoing, nil
	case GraphDirectionIncoming:
		return GraphDirectionIncoming, nil
	case GraphDirectionBoth:
		return GraphDirectionBoth, nil
	default:
		return "", fmt.Errorf("%w: direction must be outgoing, incoming, or both", ErrInvalidInput)
	}
}

// validate normalizes a neighbourhood request and refuses one that asks for
// more than a ceiling allows.
func (r *GraphRequest) validate() error {
	direction, err := normalizeGraphDirection(r.Direction, GraphDirectionBoth)
	if err != nil {
		return err
	}
	r.Direction = direction
	if len(r.Roots) > MaxGraphRoots {
		return fmt.Errorf("%w: at most %d roots may be expanded at once", ErrInvalidInput, MaxGraphRoots)
	}
	if r.Depth < 0 {
		return fmt.Errorf("%w: depth must not be negative", ErrInvalidInput)
	}
	if r.Depth == 0 {
		r.Depth = 1
	}
	if r.Depth > MaxGraphDepth {
		return fmt.Errorf("%w: depth %d exceeds the ceiling of %d", ErrInvalidInput, r.Depth, MaxGraphDepth)
	}
	if r.MaxNodes < 0 || r.MaxEdges < 0 {
		return fmt.Errorf("%w: max_nodes and max_edges must not be negative", ErrInvalidInput)
	}
	if r.MaxNodes == 0 {
		r.MaxNodes = DefaultGraphNodes
	}
	if r.MaxEdges == 0 {
		r.MaxEdges = DefaultGraphEdges
	}
	if r.MaxNodes > MaxGraphNodes {
		return fmt.Errorf("%w: max_nodes %d exceeds the ceiling of %d", ErrInvalidInput, r.MaxNodes, MaxGraphNodes)
	}
	if r.MaxEdges > MaxGraphEdges {
		return fmt.Errorf("%w: max_edges %d exceeds the ceiling of %d", ErrInvalidInput, r.MaxEdges, MaxGraphEdges)
	}
	return nil
}

func (r *GraphPathRequest) validate() error {
	direction, err := normalizeGraphDirection(r.Direction, GraphDirectionBoth)
	if err != nil {
		return err
	}
	r.Direction = direction
	r.From = strings.TrimSpace(r.From)
	r.To = strings.TrimSpace(r.To)
	if r.From == "" || r.To == "" {
		return fmt.Errorf("%w: from and to are both required", ErrInvalidInput)
	}
	if r.MaxDepth < 0 || r.MaxVisits < 0 {
		return fmt.Errorf("%w: max_depth and max_visits must not be negative", ErrInvalidInput)
	}
	if r.MaxDepth == 0 {
		r.MaxDepth = DefaultGraphPathDepth
	}
	if r.MaxDepth > MaxGraphPathDepth {
		return fmt.Errorf("%w: max_depth %d exceeds the ceiling of %d", ErrInvalidInput, r.MaxDepth, MaxGraphPathDepth)
	}
	if r.MaxVisits == 0 {
		r.MaxVisits = DefaultGraphPathVisits
	}
	if r.MaxVisits > MaxGraphPathVisits {
		return fmt.Errorf("%w: max_visits %d exceeds the ceiling of %d", ErrInvalidInput, r.MaxVisits, MaxGraphPathVisits)
	}
	return nil
}

func (r *GraphReportRequest) validate() error {
	// Empty means every collection; see CollectionScopeSQL.
	r.CollectionID = strings.TrimSpace(r.CollectionID)
	if r.Limit < 0 {
		return fmt.Errorf("%w: limit must not be negative", ErrInvalidInput)
	}
	if r.Limit == 0 {
		r.Limit = DefaultGraphReportItems
	}
	if r.Limit > MaxGraphReportItems {
		return fmt.Errorf("%w: limit %d exceeds the ceiling of %d", ErrInvalidInput, r.Limit, MaxGraphReportItems)
	}
	return nil
}
