package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

func scopedServer(t *testing.T, scope string) *Server {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	cfg := config.Default()
	cfg.MCP.DefaultScope = scope
	cfg.MCP.DefaultProfile = ""
	return NewServerWithOptions(ServerOptions{Store: st, Config: cfg})
}

func listedTools(t *testing.T, s *Server) []string {
	t.Helper()
	rr := doJSON(t, s, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	var response struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	names := make([]string, 0, len(response.Result.Tools))
	for _, tool := range response.Result.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

// callTool returns the JSON-RPC error message, or "" when the call succeeded.
func callToolError(t *testing.T, s *Server, name string) string {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"` + name + `","arguments":{}}}`
	rr := doJSON(t, s, http.MethodPost, "/mcp", body)
	var response struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode tools/call: %v (%s)", err, rr.Body.String())
	}
	if response.Error != nil {
		return response.Error.Message
	}
	if response.Result != nil && response.Result.IsError && len(response.Result.Content) > 0 {
		return response.Result.Content[0].Text
	}
	return ""
}

// The guard that makes the whole thing hold: a tool cannot be registered
// without someone deciding how much trust it needs. Without this, a new tool
// would appear in whatever scope its position in the list happened to give it.
func TestEveryMCPToolIsClassified(t *testing.T) {
	// The widest scope lists everything, which is what makes this exhaustive.
	s := scopedServer(t, MCPScopeOrganizer)
	listed := listedTools(t, s)
	if len(listed) == 0 {
		t.Fatal("the widest scope must list tools")
	}
	for _, name := range listed {
		if _, classified := mcpToolScopes[name]; !classified {
			t.Fatalf("tool %q has no entry in mcpToolScopes; classify it rather than letting it default", name)
		}
	}
	// And nothing is classified that does not exist — a stale entry would make
	// the table lie about the surface.
	registered := map[string]bool{}
	for _, name := range listed {
		registered[name] = true
	}
	for _, name := range classifiedMCPToolNames() {
		if !registered[name] {
			t.Fatalf("mcpToolScopes classifies %q, which is not a registered tool", name)
		}
	}
}

// Each scope lists exactly what it should — asserted as a whole set, so a tool
// silently moving tier fails here rather than in production.
func TestEachScopeListsExactlyItsTools(t *testing.T) {
	expected := map[string][]string{
		MCPScopeSearchOnly: {
			"get_notebook_tree", "list_collections", "list_notebooks",
			"list_search_notebooks", "list_tags", "search_documents",
		},
		MCPScopeReadOnly: {
			"get_document", "get_document_blocks", "get_document_outline", "get_documents",
			"get_graph", "get_graph_report", "get_lint_report",
			"get_note_line_range", "get_notebook_tree", "get_notebook_notes",
			"find_graph_path", "list_collections", "list_document_links",
			"list_document_resources", "list_notebooks", "list_search_notebooks",
			"list_tags", "list_tasks", "list_templates", "plan_selection",
			"read_resource", "run_note_query",
			"scan_remote_media", "search_documents", "search_in_note",
		},
	}
	for scope, want := range expected {
		sort.Strings(want)
		got := listedTools(t, scopedServer(t, scope))
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("scope %q lists\n  %v\nwant\n  %v", scope, got, want)
		}
	}

	// editor adds single-note writes and nothing else; organizer adds the batch
	// tool and nothing else. Asserted as deltas, so the test stays readable as
	// tools are added.
	readOnly := listedTools(t, scopedServer(t, MCPScopeReadOnly))
	editor := listedTools(t, scopedServer(t, MCPScopeEditor))
	organizer := listedTools(t, scopedServer(t, MCPScopeOrganizer))

	editorAdds := setDifference(editor, readOnly)
	sort.Strings(editorAdds)
	wantEditor := []string{"append_to_note", "create_from_template", "create_note", "delete_note", "edit_note", "localize_remote_media", "move_note_to_notebook", "prepend_to_note", "update_note"}
	if strings.Join(editorAdds, ",") != strings.Join(wantEditor, ",") {
		t.Fatalf("editor adds %v, want %v", editorAdds, wantEditor)
	}
	organizerAdds := setDifference(organizer, editor)
	if strings.Join(organizerAdds, ",") != "run_batch" {
		t.Fatalf("organizer adds %v, want [run_batch]", organizerAdds)
	}

	// Scopes are cumulative: each is a superset of the one before it.
	for _, pair := range [][2][]string{{editor, readOnly}, {organizer, editor}} {
		if len(setDifference(pair[1], pair[0])) != 0 {
			t.Fatalf("scopes must be cumulative; %v is not a superset of %v", pair[0], pair[1])
		}
	}
}

func setDifference(a, b []string) []string {
	inB := map[string]bool{}
	for _, item := range b {
		inB[item] = true
	}
	out := []string{}
	for _, item := range a {
		if !inB[item] {
			out = append(out, item)
		}
	}
	return out
}

// The point of F2. Filtering `tools/list` is presentation; this is the check
// that matters, and before F2 only write tools had one — a hidden *read* tool
// answered perfectly well when called directly.
func TestHiddenToolsAreRefusedWhenCalledDirectly(t *testing.T) {
	s := scopedServer(t, MCPScopeSearchOnly)
	listed := map[string]bool{}
	for _, name := range listedTools(t, s) {
		listed[name] = true
	}

	for _, name := range classifiedMCPToolNames() {
		if listed[name] {
			continue
		}
		message := callToolError(t, s, name)
		if message == "" {
			t.Fatalf("tool %q is hidden from search-only but answered when called", name)
		}
		if !strings.Contains(message, "scope") {
			t.Fatalf("the refusal for %q should name the scope needed, got %q", name, message)
		}
	}

	// A tool that does not exist is refused too, and not as a scope problem.
	if message := callToolError(t, s, "definitely_not_a_tool"); !strings.Contains(message, "unknown MCP tool") {
		t.Fatalf("an unknown tool should be refused as unknown, got %q", message)
	}
}

// The deprecated key still works, and cannot widen anything.
func TestDeprecatedProfileKey(t *testing.T) {
	cases := []struct {
		name       string
		scope      string
		profile    string
		want       string
		wantNoteIn string
	}{
		{"neither set defaults to read-only", "", "", MCPScopeReadOnly, ""},
		{"scope alone", MCPScopeEditor, "", MCPScopeEditor, ""},
		{"deprecated alone still applies", "", "editor", MCPScopeEditor, "deprecated"},
		{"both agree", MCPScopeEditor, "editor", MCPScopeEditor, "both"},
		// The one that matters: a half-migrated config must never end up wider
		// than either half intended.
		{"deprecated is narrower and wins", MCPScopeOrganizer, "read-only", MCPScopeReadOnly, "narrower"},
		{"new key is narrower and wins", MCPScopeSearchOnly, "editor", MCPScopeSearchOnly, "narrower"},
		{"an unknown value falls back to read-only", "nonsense", "", MCPScopeReadOnly, "not a known scope"},
	}
	for _, tc := range cases {
		got, note := effectiveMCPScope(tc.scope, tc.profile)
		if got != tc.want {
			t.Fatalf("%s: got %q, want %q (note %q)", tc.name, got, tc.want, note)
		}
		if tc.wantNoteIn != "" && !strings.Contains(note, tc.wantNoteIn) {
			t.Fatalf("%s: expected a note containing %q, got %q", tc.name, tc.wantNoteIn, note)
		}
		if tc.wantNoteIn == "" && note != "" {
			t.Fatalf("%s: expected no note, got %q", tc.name, note)
		}
	}
}

// Whatever the scope, the operations Notrios keeps off the model path stay off.
// This is the standing decision from v0.5, not a property of any one scope.
func TestNoScopeReachesWholeLibraryOperations(t *testing.T) {
	withheld := []string{
		"run_lint", "run_fix", "collect_garbage", "export_archive", "restore_archive",
		"publish", "rename_tag", "delete_notebook", "purge_document",
		// F5: regenerating the graph report scans the whole collection and
		// overwrites a note, and exporting the graph writes files to a path
		// someone chose. Both are CLI and REST, for the same reason as the rest
		// of this list.
		"write_graph_report", "export_graph",
	}
	for _, scope := range MCPScopes() {
		s := scopedServer(t, scope)
		listed := listedTools(t, s)
		for _, name := range listed {
			for _, forbidden := range withheld {
				if name == forbidden {
					t.Fatalf("scope %q exposes %q; whole-library operations stay on surfaces a person drives", scope, name)
				}
			}
		}
	}
}
