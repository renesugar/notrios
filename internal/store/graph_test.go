package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
)

// newGraphFixture builds a small library whose shape every traversal test can
// reason about:
//
//	chain:    a -> b -> c -> d          (a straight path, for depth and paths)
//	shortcut: a -> hub, b -> hub, c -> hub  (an in-degree hub)
//	island:   island                    (nothing links to or from it)
//	orphan:   orphan -> a               (nothing links to it)
//	trashed:  b -> trashed, then trashed goes to the Trash
//	resource: c embeds one image
//
// Documents are created in reverse dependency order and links rebuilt at the
// end, because a link only resolves once its target exists.
func newGraphFixture(t *testing.T) (*SQLiteStore, map[string]string) {
	t.Helper()
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	uri := func(id string) string { return DocumentURI("default", id) }
	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_pic", Filename: "pic.png", MIMEType: "image/png",
		Content: bytes.NewReader([]byte("\x89PNG\r\n\x1a\npic")),
	})
	if err != nil {
		t.Fatal(err)
	}

	bodies := []struct{ id, body string }{
		{"g_island", "Nothing points here and it points nowhere.\n"},
		{"g_trashed", "About to be trashed.\n"},
		{"g_hub", "Everyone links here.\n"},
		{"g_d", "The end of the chain.\n"},
		{"g_c", "[next](" + uri("g_d") + ")\n\n[hub](" + uri("g_hub") + ")\n\n![pic](" + resource.URI + ")\n"},
		{"g_b", "[next](" + uri("g_c") + ")\n\n[hub](" + uri("g_hub") + ")\n\n[gone](" + uri("g_trashed") + ")\n"},
		{"g_a", "[next](" + uri("g_b") + ")\n\n[hub](" + uri("g_hub") + ")\n"},
		{"g_orphan", "[a](" + uri("g_a") + ")\n"},
	}
	ids := map[string]string{"res": resource.ID}
	for _, entry := range bodies {
		doc, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: entry.id, Title: entry.id, Body: entry.body,
		})
		if err != nil {
			t.Fatalf("create %s: %v", entry.id, err)
		}
		ids[entry.id] = doc.ID
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{
		DocumentID: "g_c", ResourceID: resource.ID, RelationType: "embedded",
	}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range bodies {
		if err := st.RebuildDocumentLinks(ctx, entry.id); err != nil {
			t.Fatalf("rebuild %s: %v", entry.id, err)
		}
	}
	doc, err := st.GetDocument(ctx, "g_trashed")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: "g_trashed", BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	return st, ids
}

func graphNodeIDs(resp GraphResponse) []string {
	ids := make([]string, 0, len(resp.Nodes))
	for _, node := range resp.Nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func graphContains(resp GraphResponse, id string) bool {
	for _, node := range resp.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func graphDepthOf(t *testing.T, resp GraphResponse, id string) int {
	t.Helper()
	for _, node := range resp.Nodes {
		if node.ID == id {
			return node.Depth
		}
	}
	t.Fatalf("node %q is not in the graph: %v", id, graphNodeIDs(resp))
	return -1
}

// Depth was accepted and ignored before E4. This is the test that would have
// caught it: at depth 1 the far end of the chain is absent, at depth 3 it is
// present, and nothing about the request changes except the number.
func TestGraphHonoursDepth(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	shallow, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_a"}, Direction: GraphDirectionOutgoing, Depth: 1, MaxNodes: 100, MaxEdges: 100})
	if err != nil {
		t.Fatal(err)
	}
	if graphContains(shallow, "g_c") || graphContains(shallow, "g_d") {
		t.Fatalf("depth 1 reached past the first hop: %v", graphNodeIDs(shallow))
	}
	if !graphContains(shallow, "g_b") || !graphContains(shallow, "g_hub") {
		t.Fatalf("depth 1 missed the first hop: %v", graphNodeIDs(shallow))
	}

	deep, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_a"}, Direction: GraphDirectionOutgoing, Depth: 3, MaxNodes: 100, MaxEdges: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"g_a", "g_b", "g_c", "g_d", "g_hub"} {
		if !graphContains(deep, want) {
			t.Fatalf("depth 3 missed %s: %v", want, graphNodeIDs(deep))
		}
	}
	if got := graphDepthOf(t, deep, "g_d"); got != 3 {
		t.Fatalf("g_d depth = %d, want 3", got)
	}
	if got := graphDepthOf(t, deep, "g_hub"); got != 1 {
		t.Fatalf("hub reached at depth %d, want the nearest distance 1", got)
	}
	if deep.RequestedDepth != 3 || deep.CompletedDepth != 3 || deep.Truncated {
		t.Fatalf("an untruncated expansion should complete its depth: %+v", deep)
	}
}

func TestGraphDirectionSelectsTheEdgesFollowed(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	incoming, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_d"}, Direction: GraphDirectionIncoming, Depth: 2, MaxNodes: 100, MaxEdges: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"g_d", "g_c", "g_b"} {
		if !graphContains(incoming, want) {
			t.Fatalf("incoming depth 2 missed %s: %v", want, graphNodeIDs(incoming))
		}
	}
	if graphContains(incoming, "g_a") {
		t.Fatalf("incoming depth 2 reached three hops: %v", graphNodeIDs(incoming))
	}

	outgoing, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_d"}, Direction: GraphDirectionOutgoing, Depth: 2, MaxNodes: 100, MaxEdges: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(outgoing.Nodes) != 1 || outgoing.Nodes[0].ID != "g_d" {
		t.Fatalf("g_d has no outgoing links: %v", graphNodeIDs(outgoing))
	}
}

// A ceiling that stops an expansion has to say so, and say which one it was.
func TestGraphTruncationNamesTheCeilingItHit(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	nodes, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_a"}, Direction: GraphDirectionBoth, Depth: 4, MaxNodes: 2, MaxEdges: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !nodes.Truncated || nodes.TruncatedBy != GraphTruncatedByNodes {
		t.Fatalf("node ceiling was not reported: %+v", nodes)
	}
	if len(nodes.Nodes) > 2 {
		t.Fatalf("node ceiling was not enforced: %d nodes", len(nodes.Nodes))
	}
	if nodes.CompletedDepth >= nodes.RequestedDepth {
		t.Fatalf("a truncated graph must not claim it completed its depth: %+v", nodes)
	}

	edges, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_a"}, Direction: GraphDirectionBoth, Depth: 4, MaxNodes: 100, MaxEdges: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !edges.Truncated || edges.TruncatedBy != GraphTruncatedByEdges {
		t.Fatalf("edge ceiling was not reported: %+v", edges)
	}
	if len(edges.Edges) > 1 {
		t.Fatalf("edge ceiling was not enforced: %d edges", len(edges.Edges))
	}
}

// A request wider than a ceiling is refused rather than clamped: silently
// returning less than was asked for is indistinguishable from there being less.
func TestGraphRefusesRequestsWiderThanTheCeilings(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()
	cases := map[string]GraphRequest{
		"depth":     {Roots: []string{"g_a"}, Depth: MaxGraphDepth + 1},
		"nodes":     {Roots: []string{"g_a"}, MaxNodes: MaxGraphNodes + 1},
		"edges":     {Roots: []string{"g_a"}, MaxEdges: MaxGraphEdges + 1},
		"direction": {Roots: []string{"g_a"}, Direction: "sideways"},
		"negative":  {Roots: []string{"g_a"}, Depth: -1},
	}
	for name, req := range cases {
		if _, err := st.Graph(ctx, req); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
	tooManyRoots := GraphRequest{Roots: make([]string, MaxGraphRoots+1)}
	for i := range tooManyRoots.Roots {
		tooManyRoots.Roots[i] = fmt.Sprintf("g_%d", i)
	}
	if _, err := st.Graph(ctx, tooManyRoots); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("root ceiling: err = %v, want ErrInvalidInput", err)
	}
}

func TestGraphExcludesTrashedNotesAndOptionalResources(t *testing.T) {
	st, ids := newGraphFixture(t)
	ctx := context.Background()

	withoutResources, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_b"}, Direction: GraphDirectionOutgoing, Depth: 2, MaxNodes: 100, MaxEdges: 100})
	if err != nil {
		t.Fatal(err)
	}
	if graphContains(withoutResources, "g_trashed") {
		t.Fatalf("a trashed note is not a neighbour: %v", graphNodeIDs(withoutResources))
	}
	if graphContains(withoutResources, ids["res"]) {
		t.Fatalf("resources are opt-in: %v", graphNodeIDs(withoutResources))
	}

	withResources, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_c"}, Direction: GraphDirectionOutgoing, Depth: 1, IncludeResources: true, MaxNodes: 100, MaxEdges: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !graphContains(withResources, ids["res"]) {
		t.Fatalf("include_resources did not add the embedded resource: %v", graphNodeIDs(withResources))
	}
}

func TestGraphPathFindsTheShortestChain(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	path, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_d", Direction: GraphDirectionOutgoing})
	if err != nil {
		t.Fatal(err)
	}
	if path.Status != GraphPathFound || path.Length != 3 {
		t.Fatalf("path = %+v, want a found path of length 3", path)
	}
	want := []string{"g_a", "g_b", "g_c", "g_d"}
	if len(path.Nodes) != len(want) {
		t.Fatalf("path nodes = %v, want %v", path.Nodes, want)
	}
	for i, id := range want {
		if path.Nodes[i].ID != id {
			t.Fatalf("path node %d = %s, want %s", i, path.Nodes[i].ID, id)
		}
	}
	if len(path.Edges) != len(want)-1 {
		t.Fatalf("a path of %d nodes has %d edges, got %d", len(want), len(want)-1, len(path.Edges))
	}

	// The hub gives a two-hop undirected route, which is what "both" should find.
	undirected, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_c", Direction: GraphDirectionBoth})
	if err != nil {
		t.Fatal(err)
	}
	if undirected.Status != GraphPathFound || undirected.Length != 2 {
		t.Fatalf("undirected path = %+v, want length 2 through the hub or the chain", undirected)
	}

	same, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_a"})
	if err != nil {
		t.Fatal(err)
	}
	if same.Status != GraphPathFound || same.Length != 0 || len(same.Nodes) != 1 {
		t.Fatalf("a note is zero hops from itself: %+v", same)
	}
}

// The three ways of not finding a path are different answers and must not be
// collapsed: only one of them is a statement about the library.
func TestGraphPathDistinguishesNoPathFromGivingUp(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	none, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_island", Direction: GraphDirectionBoth})
	if err != nil {
		t.Fatal(err)
	}
	if none.Status != GraphPathNoPath {
		t.Fatalf("an unreachable note is no_path, got %+v", none)
	}

	shallow, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_d", Direction: GraphDirectionOutgoing, MaxDepth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if shallow.Status != GraphPathDepthExhausted {
		t.Fatalf("a path longer than max_depth is depth_exhausted, not %q", shallow.Status)
	}
	if len(shallow.Nodes) != 0 {
		t.Fatalf("a search that found nothing must return no path: %+v", shallow)
	}

	budget, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_island", Direction: GraphDirectionBoth, MaxVisits: 1})
	if err != nil {
		t.Fatal(err)
	}
	if budget.Status != GraphPathBudgetExhausted {
		t.Fatalf("a search stopped by its visit budget is budget_exhausted, not %q", budget.Status)
	}

	// Direction matters: the chain only runs one way.
	backwards, err := st.GraphPath(ctx, GraphPathRequest{From: "g_d", To: "g_a", Direction: GraphDirectionOutgoing})
	if err != nil {
		t.Fatal(err)
	}
	if backwards.Status != GraphPathNoPath {
		t.Fatalf("following links as written, d does not reach a: %+v", backwards)
	}
}

func TestGraphPathValidatesItsEndpointsAndBounds(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	if _, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing endpoint err = %v, want ErrNotFound", err)
	}
	// A trashed note is not an endpoint either.
	if _, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_trashed"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("trashed endpoint err = %v, want ErrNotFound", err)
	}
	for name, req := range map[string]GraphPathRequest{
		"missing from": {To: "g_a"},
		"depth":        {From: "g_a", To: "g_d", MaxDepth: MaxGraphPathDepth + 1},
		"visits":       {From: "g_a", To: "g_d", MaxVisits: MaxGraphPathVisits + 1},
		"direction":    {From: "g_a", To: "g_d", Direction: "downhill"},
	} {
		if _, err := st.GraphPath(ctx, req); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
}

func TestGraphReportCountsOrphansIsolatesAndHubs(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	report, err := st.GraphReport(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	// g_island has no links either way; g_orphan links out but nothing links in.
	isolated := map[string]bool{}
	for _, entry := range report.Isolated {
		isolated[entry.DocumentID] = true
	}
	if !isolated["g_island"] {
		t.Fatalf("g_island is isolated: %+v", report.Isolated)
	}
	if isolated["g_orphan"] {
		t.Fatalf("g_orphan links out, so it is an orphan but not isolated: %+v", report.Isolated)
	}
	orphans := map[string]bool{}
	for _, entry := range report.Orphans {
		orphans[entry.DocumentID] = true
	}
	if !orphans["g_orphan"] || !orphans["g_island"] {
		t.Fatalf("orphans are notes nothing links to, including the isolated ones: %+v", report.Orphans)
	}
	if orphans["g_hub"] {
		t.Fatalf("the hub is linked to by three notes: %+v", report.Orphans)
	}
	if report.IsolatedCount > report.OrphanCount {
		t.Fatalf("every isolated note is also an orphan: %+v", report)
	}

	if len(report.Hubs) == 0 || report.Hubs[0].DocumentID != "g_hub" {
		t.Fatalf("the most-linked-to note ranks first: %+v", report.Hubs)
	}
	if report.Hubs[0].InDegree != 3 {
		t.Fatalf("g_hub in-degree = %d, want 3", report.Hubs[0].InDegree)
	}
	for i := 1; i < len(report.Hubs); i++ {
		if report.Hubs[i-1].InDegree < report.Hubs[i].InDegree {
			t.Fatalf("hubs are not ordered by in-degree: %+v", report.Hubs)
		}
	}
	if report.DocumentCount == 0 || report.LinkCount == 0 {
		t.Fatalf("report totals are empty: %+v", report)
	}
}

// A capped example list must not change a count, exactly as lint's detail cap
// does not change its counts.
func TestGraphReportCapIsOnExamplesNotCounts(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	full, err := st.GraphReport(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	capped, err := st.GraphReport(ctx, GraphReportRequest{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if capped.OrphanCount != full.OrphanCount || capped.IsolatedCount != full.IsolatedCount ||
		capped.DocumentCount != full.DocumentCount || capped.LinkCount != full.LinkCount {
		t.Fatalf("the cap changed a count: capped=%+v full=%+v", capped, full)
	}
	if len(capped.Orphans) != 1 || len(capped.Hubs) != 1 {
		t.Fatalf("limit 1 should show one example per list: %+v", capped)
	}
	if !capped.Truncated {
		t.Fatalf("a capped example list reports that it was capped: %+v", capped)
	}
	if _, err := st.GraphReport(ctx, GraphReportRequest{Limit: MaxGraphReportItems + 1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an over-limit report request is refused")
	}
}

// Traversal is read-only. Nothing about running it may leave a mark.
func TestGraphTraversalWritesNothing(t *testing.T) {
	st, _ := newGraphFixture(t)
	ctx := context.Background()

	before, err := st.GetDocument(ctx, "g_a")
	if err != nil {
		t.Fatal(err)
	}
	revisionsBefore, err := st.ListDocumentRevisions(ctx, "g_a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Graph(ctx, GraphRequest{Roots: []string{"g_a"}, Depth: 4, IncludeResources: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GraphPath(ctx, GraphPathRequest{From: "g_a", To: "g_d"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GraphReport(ctx, GraphReportRequest{}); err != nil {
		t.Fatal(err)
	}
	after, err := st.GetDocument(ctx, "g_a")
	if err != nil {
		t.Fatal(err)
	}
	revisionsAfter, err := st.ListDocumentRevisions(ctx, "g_a")
	if err != nil {
		t.Fatal(err)
	}
	if before.CurrentRevisionID != after.CurrentRevisionID || len(revisionsBefore) != len(revisionsAfter) {
		t.Fatalf("traversal changed the library: %s -> %s, %d -> %d revisions",
			before.CurrentRevisionID, after.CurrentRevisionID, len(revisionsBefore), len(revisionsAfter))
	}
}
