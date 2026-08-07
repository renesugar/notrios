package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// callToolJSON calls a tool and returns its structured result.
func callToolJSON(t *testing.T, s *Server, name, arguments string) map[string]any {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"` + name + `","arguments":` + arguments + `}}`
	rr := doJSON(t, s, http.MethodPost, "/mcp", body)
	var response struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
			Content           []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode %s: %v (%s)", name, err, rr.Body.String())
	}
	if response.Error != nil {
		t.Fatalf("%s failed: %s", name, response.Error.Message)
	}
	if response.Result.IsError {
		text := ""
		if len(response.Result.Content) > 0 {
			text = response.Result.Content[0].Text
		}
		t.Fatalf("%s returned an error: %s", name, text)
	}
	return response.Result.StructuredContent
}

func readOnlyServer(t *testing.T) *Server {
	t.Helper()
	return scopedServer(t, MCPScopeReadOnly)
}

// The default is metadata and nothing else. Streaming attachment bytes into a
// model's context by accident is precisely what the bulk/control-plane split
// exists to prevent.
func TestMCPReadResourceDefaultsToMetadataOnly(t *testing.T) {
	s := readOnlyServer(t)
	id := seedResource(t, s, "notes.txt", "text/plain", "the quick brown fox")

	out := callToolJSON(t, s, "read_resource", `{"resource_id":"`+id+`"}`)
	if out["filename"] != "notes.txt" || out["mime_type"] != "text/plain" {
		t.Fatalf("metadata missing: %+v", out)
	}
	if out["sha256"] == "" || out["uri"] == "" {
		t.Fatalf("a caller needs the hash and URI to decide what to do next: %+v", out)
	}
	if out["text"] != nil {
		t.Fatalf("bytes must not arrive unasked: %+v", out["text"])
	}
}

// Asked for, a text-like resource comes back — and offset/length make it a
// range read, which is why REST gained Range in this slice.
func TestMCPReadResourceReadsTextInSlices(t *testing.T) {
	s := readOnlyServer(t)
	id := seedResource(t, s, "notes.txt", "text/plain", "0123456789abcdef")

	whole := callToolJSON(t, s, "read_resource", `{"resource_id":"`+id+`","include_text":true}`)
	if whole["text"] != "0123456789abcdef" {
		t.Fatalf("full text: %+v", whole["text"])
	}
	if whole["truncated"] != false {
		t.Fatalf("a complete read must not claim truncation: %+v", whole)
	}

	slice := callToolJSON(t, s, "read_resource", `{"resource_id":"`+id+`","include_text":true,"offset":10,"length":3}`)
	if slice["text"] != "abc" {
		t.Fatalf("range read: %+v", slice["text"])
	}
	if slice["truncated"] != true {
		t.Fatalf("more remained, so truncated must say so: %+v", slice)
	}
}

// A binary resource is described, never transcribed. An image or a PDF is not
// something a model should receive as "text".
func TestMCPReadResourceRefusesToTranscribeBinary(t *testing.T) {
	s := readOnlyServer(t)
	id := seedResource(t, s, "photo.png", "image/png", "\x89PNG\r\n\x1a\nbinary-ish")

	out := callToolJSON(t, s, "read_resource", `{"resource_id":"`+id+`","include_text":true}`)
	if out["text"] != nil {
		t.Fatalf("binary bytes must not be returned as text: %+v", out["text"])
	}
	note, _ := out["note"].(string)
	if !strings.Contains(note, "not a text-like resource") {
		t.Fatalf("the refusal should say why: %q", note)
	}
	// Metadata still comes back, so the caller can fetch it over REST if it
	// genuinely wants the bytes.
	if out["mime_type"] != "image/png" || out["uri"] == "" {
		t.Fatalf("metadata should survive the refusal: %+v", out)
	}
}

// The read tools F3 adds are the ones a model needs to cite precisely and to
// see structure — and each is bounded by the same store ceiling a person hits.
func TestMCPReadToolsReachTheirSurfaces(t *testing.T) {
	s := readOnlyServer(t)
	ctx := context.Background()
	target, err := s.store.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Target", Body: "# Heading\n\nbody text\n"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	source, err := s.store.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Source",
		Body:  "[to target](document://default/documents/" + target.ID + ")\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	blocks := callToolJSON(t, s, "get_document_blocks", `{"document_id":"`+target.ID+`"}`)
	if list, ok := blocks["blocks"].([]any); !ok || len(list) == 0 {
		t.Fatalf("expected blocks: %+v", blocks)
	}

	graph := callToolJSON(t, s, "get_graph", `{"document_id":"`+source.ID+`","depth":1}`)
	if graph["nodes"] == nil {
		t.Fatalf("expected graph nodes: %+v", graph)
	}

	path := callToolJSON(t, s, "find_graph_path", `{"from_document_id":"`+source.ID+`","to_document_id":"`+target.ID+`"}`)
	if path["status"] == nil {
		t.Fatalf("a path result must report its status: %+v", path)
	}

	report := callToolJSON(t, s, "get_graph_report", `{}`)
	if report["document_count"] == nil {
		t.Fatalf("expected a report: %+v", report)
	}

	lint := callToolJSON(t, s, "get_lint_report", `{}`)
	if lint["checks"] == nil {
		t.Fatalf("expected lint checks: %+v", lint)
	}

	// The query block reaches the same parser the search box uses.
	block := `{"block":"query: Target"}`
	query := callToolJSON(t, s, "run_note_query", block)
	if query["spec"] == nil {
		t.Fatalf("expected a parsed spec: %+v", query)
	}
}

// A malformed query block is a result carrying `error`, not a failed call —
// the same rule E7 set for the REST route and for rendering inside a note.
func TestMCPRunNoteQueryReportsABadBlockAsAValue(t *testing.T) {
	s := readOnlyServer(t)
	out := callToolJSON(t, s, "run_note_query", `{"block":"nonsense: true"}`)
	if out["error"] == nil || out["error"] == "" {
		t.Fatalf("a malformed block should carry an error field: %+v", out)
	}
}

// Lint over MCP must stay content-free. A broken wikilink's raw text is
// frequently the title of a private note, which is exactly why the report
// carries a hash and a location instead.
func TestMCPLintReportCarriesNoNoteText(t *testing.T) {
	s := readOnlyServer(t)
	ctx := context.Background()
	if _, err := s.store.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Has a broken link",
		Body:  "[gone](document://default/documents/doc_absent_secret_title)\n",
	}); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	out := callToolJSON(t, s, "get_lint_report", `{}`)
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	// Assert a finding was actually produced. Without this the leak check
	// below would pass on an empty report, which proves nothing.
	if !strings.Contains(string(encoded), "broken_document_link") ||
		!strings.Contains(string(encoded), "target_sha256") {
		t.Fatalf("expected a broken-link finding with a hashed target: %s", encoded)
	}
	if strings.Contains(string(encoded), "doc_absent_secret_title") {
		t.Fatalf("the lint report leaked the offending target text: %s", encoded)
	}
}
