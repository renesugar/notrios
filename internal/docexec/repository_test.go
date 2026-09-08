package docexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/docaudit"
	"github.com/renesugar/notrios/internal/service"
	"github.com/renesugar/notrios/internal/store"
)

type repositoryFixture struct {
	root       string
	svc        *service.Service
	server     *httptest.Server
	captures   []httpCapture
	schemaErrs []error
	documentID string
	otherID    string
	notebookID string
	resourceID string
	jobID      string
	databaseID string
	revisionID string
	templateID string
	imports    DocumentationImportFixtures
	importBase DocumentationImportSnapshot
}

type httpCapture struct {
	method, path string
	status       int
	request      []byte
	response     []byte
}

type repositoryExamples struct {
	t       *testing.T
	root    string
	cli     string
	daemon  string
	openAPI *OpenAPIValidator
	mu      sync.Mutex
	fixture map[string]*repositoryFixture
}

// TestRepositoryExamples is the single manifest-owned runtime check anchor.
// Every adapter below executes the literal, hash-pinned fence in fresh state;
// it does not substitute a different safer operation.
func TestRepositoryExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("G18d builds and executes the published CLI examples")
	}
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
	manifest, err := LoadManifest(root, "performance/v0.7-g18a/INVENTORY.json", "docs/docaudit/registry.json")
	if err != nil {
		t.Fatal(err)
	}
	validator, err := LoadOpenAPI(filepath.Join(root, "api/openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	h := &repositoryExamples{t: t, root: root, openAPI: validator, fixture: make(map[string]*repositoryFixture)}
	h.cli = buildRepositoryBinary(t, root, "notriosctl", "./cmd/notriosctl")
	h.daemon = buildRepositoryBinary(t, root, "notriosd", "./cmd/notriosd")
	t.Cleanup(h.close)
	report, err := manifest.Run(context.Background(), RunOptions{
		Adapters: map[string]Adapter{
			"config-fragment":  h.configFragment,
			"loopback-shell":   h.loopbackShell,
			"cli-shell":        h.cliShell,
			"cli-edge":         h.cliEdge,
			"daemon-lifecycle": h.daemonLifecycle,
		},
		ResolveSubstitution: h.resolve,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 131 -> 133 entries in v0.8 H4 slice D: the `paths` and `config show`
	// synopses in docs/cli.md. Executed is unchanged at 63 -- both are
	// bracketed-optional synopses, registered as illustrative placeholders like
	// every other one in that document, and the commands themselves are covered
	// by executed tests in cmd/notriosctl.
	// 133 -> 137 entries in v0.8 H4 slice E: the `migrate` synopsis and its
	// dry-run example in docs/cli.md, the upgrade recipe in
	// docs/installation.md, and the asset-installation block added to the same
	// page. Executed is unchanged at 63: each one either relocates this user's
	// library or writes to a system directory as root, so all four are
	// reviewed unrun reasons, and the command is covered by executed tests in
	// cmd/notriosctl instead.
	// 137 -> 139 entries in v0.8 H5: docs/installation.md replaced its
	// hand-rolled install recipe with make install, uninstall and purge --
	// five new examples, three retired. Executed is unchanged at 63: each one
	// either installs into this user's home or deletes their library, so all
	// five are reviewed unrun reasons, covered by executed tests in
	// scripts/test_lifecycle.py which run them against a disposable HOME.
	// 141 -> 142 entries in v0.8 H9 slice D: the migrate-credentials example.
	// 13 -> 14 topics too: docs/journeys-gui.md is a new topic with one
	// example and nothing executed, which is why executedTopics stays 12.
	// 142 -> 143 in v0.8 H14 slice D: the GUI screenshot regenerate command,
	// which launches a browser and rewrites committed images, so it is a
	// reviewed unrun reason rather than something a test run should do.
	// Executed is unchanged at 63: it writes into this user's real credential
	// store, so it is a reviewed shared-user-state reason covered by executed
	// tests in cmd/notriosctl that run it against a sandboxed library.
	// 142 -> 144 entries and 63 -> 64 executed in v0.8 H22, and one new topic:
	// the collections synopsis in docs/cli.md is a bracketed-flag form and joins
	// the illustrative set, while the two discovery commands in
	// docs/query-language.md are literal and run -- the command line could
	// create a collection through an import and could not name one back, so
	// there was nothing to document until now.
	// 144 -> 147 entries in v0.8 H24: three gomplate pipelines in docs/cli.md.
	// Executed stays 64 -- gomplate is a host tool Notrios does not ship or
	// require, and running them would make an external installation a build
	// dependency of the documentation gate.
	// 147 -> 148 entries in v0.8 H21: the note-reading synopsis in docs/cli.md,
	// a bracketed-flag form whose four commands are covered by executed tests.
	// 148 -> 150 entries and 64 -> 65 executed in v0.8 H26: the tags show
	// synopsis, and the shell existence check beside it, which runs against a
	// tag put on a note first so that finding it means something.
	// 150 -> 151 entries in v0.8 H19: the search synopsis in docs/cli.md, a
	// bracketed-flag form whose command is covered by executed tests and by an
	// executed journey.
	if report.Executed != 65 || report.Entries != 151 || len(report.Topics) != 14 {
		t.Fatalf("unexpected G18d coverage: %+v", report)
	}
	executedTopics := 0
	for _, topic := range report.Topics {
		if topic.Counts[string(docaudit.GradeExecuted)] > 0 {
			executedTopics++
		}
	}
	// Index is an external clone/build/GUI recipe and therefore has a reviewed
	// host/network reason. Every other command-bearing topic has a scratch run.
	//
	// 12 -> 13 in v0.8 H22: query-language gained one. It had no executed
	// example because it had nothing runnable to show -- the page documented
	// terms that name a notebook or a collection and no way to find out which
	// ones exist.
	if executedTopics != 13 {
		t.Fatalf("executed topic coverage = %d, want 13 plus reviewed index exception: %+v", executedTopics, report.Topics)
	}
	assertRepositoryCoverageContracts(t, manifest)
	writeRepositoryReport(t, report)
}

func assertRepositoryCoverageContracts(t *testing.T, manifest Manifest) {
	t.Helper()
	surfaces := make(map[string]bool)
	postconditions := make(map[string]bool)
	for _, entry := range manifest.Entries {
		if entry.Registered.State != docaudit.GradeExecuted {
			if strings.Contains(entry.Registered.Unrun.Detail, "PENDING G18d fixture:") {
				t.Fatalf("provisional unrun reason remains on %s", entry.Candidate.ID)
			}
			continue
		}
		surfaces[entry.Registered.Execution.Surface] = true
		postconditions[entry.Registered.Execution.Postcondition.Kind] = true
	}
	for _, surface := range []string{"cli", "config", "rest", "mcp"} {
		if !surfaces[surface] {
			t.Errorf("no executed %s example", surface)
		}
	}
	// These distinct postconditions pin the non-vacuous fixture claims named by
	// G18d: precedence/defaults, redaction, confirmation, MCP scope, dry-run
	// parity, and guarded cleanup/rollback behavior.
	for _, kind := range []string{"mcp-scope", "sync-rest-policy", "status-redacted", "rest-resource-delete", "mcp-schema-scope", "dry-run-unchanged", "cli-fix-parity", "rest-concurrency"} {
		if !postconditions[kind] {
			t.Errorf("required G18d semantic postcondition %q is not executed", kind)
		}
	}
}

func writeRepositoryReport(t *testing.T, report RunReport) {
	t.Helper()
	path := os.Getenv("NOTRIOS_DOCEXEC_REPORT")
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write G18d report: %v", err)
	}
}

func buildRepositoryBinary(t *testing.T, root, name, pkg string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", target, pkg)
	command.Dir = root
	command.Env = os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return target
}

func (h *repositoryExamples) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, fixture := range h.fixture {
		if fixture.server != nil {
			fixture.server.Close()
		}
		if fixture.svc != nil {
			_ = fixture.svc.Close()
		}
	}
}

func (h *repositoryExamples) resolve(entry ManifestEntry, source string) (string, bool) {
	fixture := h.ensureFixture(entry)
	values := map[string]string{
		"loopback.base_url":      fixtureURL(fixture),
		"cli.binary":             h.cli,
		"daemon.binary":          h.daemon,
		"archive.full_dir":       filepath.Join(fixture.root, "backups", "full"),
		"archive.subset_dir":     filepath.Join(fixture.root, "transfer", "research"),
		"import.joplin_dir":      fixture.imports.Joplin,
		"import.obsidian_dir":    fixture.imports.Obsidian,
		"import.twitter_dir":     fixture.imports.Twitter,
		"import.chatgpt_dir":     fixture.imports.ChatGPT,
		"import.chatgpt_file":    filepath.Join(fixture.imports.ChatGPT, "conversations.json"),
		"import.claude_dir":      fixture.imports.Claude,
		"import.claude_file":     filepath.Join(fixture.imports.Claude, "conversations.json"),
		"import.archive_v1_dir":  fixture.imports.ArchiveV1,
		"import.joplin_plan":     filepath.Join(fixture.root, "joplin-plan.json"),
		"import.obsidian_plan":   filepath.Join(fixture.root, "obsidian-plan.json"),
		"seed.document_id":       fixture.documentID,
		"seed.other_document_id": fixture.otherID,
		"seed.notebook_id":       fixture.notebookID,
		"seed.resource_id":       fixture.resourceID,
		"seed.job_id":            fixture.jobID,
		"seed.database_id":       fixture.databaseID,
		"seed.revision_id":       fixture.revisionID,
		"seed.template_id":       fixture.templateID,
		"graph.output_dir":       filepath.Join(fixture.root, "graph"),
	}
	value, ok := values[source]
	return value, ok
}

func fixtureURL(fixture *repositoryFixture) string {
	if fixture.server == nil {
		return ""
	}
	return fixture.server.URL
}

func (h *repositoryExamples) ensureFixture(entry ManifestEntry) *repositoryFixture {
	h.mu.Lock()
	defer h.mu.Unlock()
	if fixture := h.fixture[entry.Candidate.ID]; fixture != nil {
		return fixture
	}
	fixture := &repositoryFixture{root: h.t.TempDir()}
	h.fixture[entry.Candidate.ID] = fixture
	switch entry.Registered.Execution.Fixture {
	case "seeded-loopback", "seeded-cli":
		h.seed(fixture)
		if entry.Registered.Execution.Fixture == "seeded-loopback" {
			fixture.server = httptest.NewServer(h.schemaHandler(fixture, fixture.svc.Handler))
		} else {
			// The CLI examples need to be pointed at the seeded library. They
			// used to find it by accident: the compiled default was ./data
			// relative to the working directory, and the sandbox ran the shell
			// there. H4 removed that working-directory dependence, so the
			// fixture now states where the library is, like a real user's
			// configuration does.
			writeDaemonConfig(h.t, fixture.root, "127.0.0.1:0")
		}
	case "daemon-scratch":
		writeDaemonConfig(h.t, fixture.root, "127.0.0.1:0")
	case "config-fragment":
	default:
		h.t.Fatalf("unknown repository fixture %q", entry.Registered.Execution.Fixture)
	}
	return fixture
}

func (h *repositoryExamples) seed(fixture *repositoryFixture) {
	cfg := config.Default()
	cfg.Data.Directory = filepath.Join(fixture.root, "data")
	cfg.Data.DatabasePath = filepath.Join(cfg.Data.Directory, "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(cfg.Data.Directory, "assets")
	cfg.Data.ProjectionDir = filepath.Join(cfg.Data.Directory, "projections")
	cfg.RemoteMedia.QuarantineDir = filepath.Join(cfg.Data.Directory, "quarantine")
	cfg.Data.StateDir = cfg.Data.Directory
	cfg.Data.CacheDir = cfg.Data.Directory
	cfg.Data.RuntimeDir = cfg.Data.Directory
	// Left at the compiled default, this resolved to ./data/search-index
	// relative to the test process, creating internal/docexec/data in the
	// source tree on every run.
	cfg.SearchSidecar.IndexDir = filepath.Join(cfg.Data.Directory, "search-index")
	svc, err := service.New(cfg)
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.svc = svc
	ctx := context.Background()
	work, err := svc.Store.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: "nb_work", Name: "Work"})
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.notebookID = work.ID
	public, err := svc.Store.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: "nb_public", Name: "Public"})
	if err != nil {
		h.t.Fatal(err)
	}
	research, err := svc.Store.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: "nb_research", Name: "Research"})
	if err != nil {
		h.t.Fatal(err)
	}
	target, err := svc.Store.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_target", Title: "Graph Target", Body: "target"})
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.otherID = target.ID
	match, err := svc.Store.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_work", NotebookID: work.ID, Title: "Work Match", Body: "# Agenda\n\n## Getting Started\n\n- apples\n- [ ] seeded task\n\nMarked paragraph ^my-anchor\n\nlinks to [target](document://default/documents/" + target.ID + ")\n"})
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.documentID = match.ID
	fixture.revisionID = match.CurrentRevisionID
	identity, err := svc.Store.GetDatabaseIdentity(ctx)
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.databaseID = identity.DatabaseID
	template, err := svc.Store.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_template", Title: "Project template", Body: "```note-template\nprompt: project\nprompt: owner\n```\n\n# {{project}}\n\nOwner: {{owner}}\n"})
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.templateID = template.ID
	if _, err := svc.Store.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_kitchen", Title: "Kitchen", Body: "autocomplete target"}); err != nil {
		h.t.Fatal(err)
	}
	for _, id := range []string{"doc_a", "doc_b"} {
		if _, err := svc.Store.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: id, Title: id, Body: "batch fixture"}); err != nil {
			h.t.Fatal(err)
		}
	}
	for _, item := range []struct{ id, notebook, title, tag string }{
		{"doc_private", work.ID, "Private Match", "private"},
		{"doc_public", public.ID, "Public Publish", "publish"},
		{"doc_research", research.ID, "Research Shared", "shared"},
	} {
		doc, err := svc.Store.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: item.id, NotebookID: item.notebook, Title: item.title, Body: item.title})
		if err != nil {
			h.t.Fatal(err)
		}
		if _, err := svc.Store.AddDocumentTag(ctx, doc.ID, item.tag); err != nil {
			h.t.Fatal(err)
		}
	}
	if _, err := svc.Store.AddDocumentTag(ctx, match.ID, "todo"); err != nil {
		h.t.Fatal(err)
	}
	if _, err := svc.Store.AddDocumentTag(ctx, match.ID, "project"); err != nil {
		h.t.Fatal(err)
	}
	if _, err := svc.Store.AddDocumentTag(ctx, target.ID, "project/child"); err != nil {
		h.t.Fatal(err)
	}
	resource, err := svc.Store.CreateResource(ctx, store.CreateResourceRequest{PreferredID: "res_fixture", Filename: "chart.png", MIMEType: "image/png", Content: strings.NewReader("fixture image bytes")})
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.resourceID = resource.ID
	job, err := svc.Store.CreateJob(ctx, store.CreateJobRequest{Kind: store.JobKindImportObsidian, Parameters: []store.JobParameter{
		{Name: "collection", Value: "default"},
		{Name: "batch-size", Value: "25"},
		{Name: "_source", Value: "/home/you/My Vault", Path: true},
	}})
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.jobID = job.ID
	if err := os.WriteFile(filepath.Join(fixture.root, "chart.png"), []byte("fixture image bytes"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fixture.root, "joplin-export"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "joplin-export", "note.md"), []byte("Body\n\nid: fixture-note\ntitle: Planned Import\ntype_: 1\n"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	fixture.imports, err = SeedDocumentationImportFixtures(fixture.root)
	if err != nil {
		h.t.Fatal(err)
	}
	fixture.importBase, err = SnapshotDocumentationImportState(ctx, svc.Store)
	if err != nil {
		h.t.Fatal(err)
	}
}

func (h *repositoryExamples) schemaHandler(fixture *repositoryFixture, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestBytes, _ := io.ReadAll(request.Body)
		request.Body = io.NopCloser(bytes.NewReader(requestBytes))
		if request.URL.Path != "/mcp" {
			clone := contractRequest(request)
			clone.Body = io.NopCloser(bytes.NewReader(requestBytes))
			if err := h.openAPI.ValidateRequest(clone); err != nil {
				fixture.schemaErrs = append(fixture.schemaErrs, err)
			}
		}
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, request)
		responseBytes := append([]byte(nil), recorder.Body.Bytes()...)
		if request.URL.Path != "/mcp" {
			clone := contractRequest(request)
			clone.Body = io.NopCloser(bytes.NewReader(requestBytes))
			if err := h.openAPI.ValidateResponse(clone, recorder.Code, recorder.Header(), responseBytes); err != nil {
				fixture.schemaErrs = append(fixture.schemaErrs, err)
			}
		}
		fixture.captures = append(fixture.captures, httpCapture{method: request.Method, path: request.URL.Path, status: recorder.Code, request: requestBytes, response: responseBytes})
		for key, values := range recorder.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(responseBytes)
	})
}

func contractRequest(request *http.Request) *http.Request {
	clone := request.Clone(request.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = "127.0.0.1:8080"
	clone.Host = "127.0.0.1:8080"
	return clone
}

func (h *repositoryExamples) configFragment(_ context.Context, invocation Invocation) (AdapterResult, error) {
	path := filepath.Join(h.t.TempDir(), "fragment.yaml")
	if err := os.WriteFile(path, []byte(invocation.Body+"\n"), 0o600); err != nil {
		return AdapterResult{}, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return AdapterResult{}, err
	}
	ok := false
	switch invocation.Entry.Registered.Execution.Postcondition.Kind {
	case "mcp-scope":
		ok = cfg.MCP.DefaultScope == "read-only" && cfg.MCP.SyncScope == "disabled" && cfg.Server.ListenAddr == config.Default().Server.ListenAddr
	case "sync-rest-policy":
		ok = cfg.Server.ListenAddr == "0.0.0.0:8443" && cfg.Sync.Target == "rest" && cfg.Sync.REST.Enabled && cfg.Sync.REST.RequireTLS && cfg.Sync.REST.RequestsPerMinute == 120 && cfg.Sync.REST.MaxBodyBytes == 1<<20
	}
	return AdapterResult{Kind: "config", Status: 0, PostconditionOK: ok, Detail: "parsed fragment did not produce its documented override/default effects"}, nil
}

func (h *repositoryExamples) loopbackShell(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	fixture := h.ensureFixture(invocation.Entry)
	output, status, err := runPinnedShell(ctx, fixture.root, h.cli, invocation.Body)
	if err != nil {
		return AdapterResult{}, fmt.Errorf("literal shell exit %d: %w\n%s", status, err, output)
	}
	if len(fixture.schemaErrs) != 0 {
		return AdapterResult{}, errors.Join(fixture.schemaErrs...)
	}
	if len(fixture.captures) == 0 {
		return AdapterResult{}, fmt.Errorf("literal example made no loopback request")
	}
	last := fixture.captures[len(fixture.captures)-1]
	ok, detail := h.loopbackPostcondition(fixture, invocation, output)
	kind := "http"
	if invocation.Entry.Registered.Execution.Surface == "mcp" {
		kind = "mcp"
	}
	return AdapterResult{Kind: kind, Status: last.status, PostconditionOK: ok, Detail: detail}, nil
}

func (h *repositoryExamples) loopbackPostcondition(fixture *repositoryFixture, invocation Invocation, output string) (bool, string) {
	kind := invocation.Entry.Registered.Execution.Postcondition.Kind
	switch kind {
	case "status-redacted":
		lower := strings.ToLower(output)
		return strings.Contains(output, "running") && strings.Contains(output, fixture.root) &&
				!strings.Contains(lower, "credential") && !strings.Contains(lower, "password") && !strings.Contains(lower, "private key"),
			"status omitted its scratch storage identity or disclosed credential material"
	case "document-created":
		result, err := fixture.svc.Store.Search(context.Background(), store.SearchRequest{Query: `"Meeting notes"`, Limit: 10})
		return err == nil && len(result.Hits) == 1, "meeting note was not created exactly once"
	case "seeded-search":
		return strings.Contains(output, "Work Match") && !strings.Contains(output, "Private Match"), "query did not include the public match while excluding private"
	case "dry-run-unchanged":
		for _, id := range []string{"doc_work", "doc_private", "doc_public", "doc_research"} {
			if _, err := fixture.svc.Store.GetDocument(context.Background(), id); err != nil {
				return false, "selection planning changed canonical documents"
			}
		}
		return strings.Contains(output, "digest") || strings.Contains(output, "included"), "selection plan was vacuous"
	case "graph-report":
		return strings.Contains(output, "document_count") && strings.Contains(output, "link_count") && !strings.Contains(output, `"link_count":0`), "graph report lacked seeded documents or links"
	case "mcp-schema-scope":
		listed, called := false, false
		for _, capture := range fixture.captures {
			var request map[string]any
			var response map[string]any
			if json.Unmarshal(capture.request, &request) != nil || json.Unmarshal(capture.response, &response) != nil || response["jsonrpc"] != "2.0" {
				return false, "invalid JSON-RPC envelope"
			}
			if request["method"] == "tools/list" {
				listed = strings.Contains(string(capture.response), `"inputSchema"`) && strings.Contains(string(capture.response), `"search_documents"`)
			}
			if request["method"] == "tools/call" {
				called = response["result"] != nil && response["error"] == nil
			}
		}
		return listed && called, "MCP list/call did not expose schemas and a bounded read-only result"
	case "rest-batch":
		for _, id := range []string{"doc_a", "doc_b"} {
			doc, err := fixture.svc.Store.GetDocument(context.Background(), id)
			if err != nil || doc.NotebookID != fixture.notebookID {
				return false, "batch did not move both seeded documents"
			}
		}
		return strings.Contains(output, "doc_a") && strings.Contains(output, "doc_b"), "batch response did not account for both documents"
	case "rest-link-check":
		return strings.Contains(output, "Kitchen") && strings.Contains(output, "gone"), "link check did not classify both seeded targets"
	case "rest-suggest":
		return strings.Contains(output, "Kitchen"), "autocomplete omitted the seeded Kitchen note"
	case "rest-links":
		return strings.Contains(output, fixture.otherID) && strings.Contains(output, fixture.revisionID), "links, graph, or revisions were vacuous"
	case "rest-blocks":
		return strings.Contains(output, `"heading_slug":"agenda"`), "block listing omitted the seeded heading identity"
	case "rest-job":
		job, err := fixture.svc.Store.GetJob(context.Background(), fixture.jobID)
		return err == nil && job.CancelRequested && strings.Contains(output, fixture.jobID), "job did not list/read/cancel the seeded job"
	case "rest-note-read":
		return strings.Contains(output, "Work Match") && strings.Contains(output, "seeded task"), "metadata and raw body did not agree"
	case "rest-concurrency":
		doc, err := fixture.svc.Store.GetDocumentIncludingTrashed(context.Background(), fixture.documentID)
		return err == nil && !doc.DeletedAt.IsZero() && doc.Title == "Meeting notes v2", "guarded update/delete did not leave the revised note in Trash"
	case "rest-graph-report":
		return strings.Contains(output, "document_count") && strings.Contains(output, "link_count") && !strings.Contains(output, `"link_count":0`), "graph report was vacuous"
	case "rest-notebook-preview":
		_, err := fixture.svc.Store.GetNotebook(context.Background(), fixture.notebookID)
		return err == nil && strings.Contains(output, fixture.notebookID), "preview changed the notebook or omitted its impact"
	case "rest-tag-rename":
		tags, err := fixture.svc.Store.ListDocumentTags(context.Background(), fixture.documentID)
		if err != nil {
			return false, err.Error()
		}
		for _, tag := range tags {
			if tag.Name == "project" {
				return strings.Contains(output, `"dry_run":true`) && strings.Contains(output, `"to":"work"`), "dry-run response did not preview the hierarchy rename"
			}
		}
		return false, "default dry run changed the seeded tag"
	case "rest-link-resolve":
		return strings.Contains(output, fixture.documentID) && strings.Contains(output, "resolved"), "stable link did not resolve locally"
	case "rest-resources":
		return strings.Contains(output, fixture.resourceID) && strings.Contains(output, "sha256") && strings.Contains(output, "fixture image bytes"), "resource upload/reference/range reports were vacuous"
	case "rest-resource-delete":
		_, err := fixture.svc.Store.GetResource(context.Background(), fixture.resourceID)
		return errors.Is(err, store.ErrNotFound), "confirmed resource remained after deletion"
	case "rest-note-query":
		return strings.Contains(output, fixture.documentID) && !strings.Contains(output, "Private Match"), "query block did not return the seeded bounded match"
	case "rest-graph-path":
		return strings.Contains(output, `"status":"found"`) && strings.Contains(output, fixture.otherID), "shortest path did not find the seeded edge"
	case "rest-note-edits":
		doc, err := fixture.svc.Store.GetDocument(context.Background(), fixture.documentID)
		return err == nil && strings.Contains(doc.Body, "follow-up item") && strings.Contains(doc.Body, "apples"), "dry run changed the body or append missed its requested line"
	case "rest-tasks":
		return strings.Contains(output, "seeded task") && strings.Contains(output, fixture.documentID), "task views omitted the seeded open task"
	case "rest-template":
		result, err := fixture.svc.Store.Search(context.Background(), store.SearchRequest{Query: `"Apollo kickoff"`, Limit: 10})
		return err == nil && len(result.Hits) == 1 && strings.Contains(output, "project"), "template prompts or substituted note were missing"
	case "rest-lint":
		return strings.Contains(output, "checks") && !strings.Contains(output, "Private Match"), "lint report was empty or leaked note content"
	case "rest-graph-note":
		doc, err := fixture.svc.Store.GetDocument(context.Background(), store.GraphReportNoteID)
		return err == nil && doc.Title == store.GraphReportNoteTitle && strings.Contains(output, "document://"), "graph report note was not created at its stable identity"
	case "rest-sidecar":
		lower := strings.ToLower(output)
		return strings.Contains(lower, "disabled") && !strings.Contains(lower, "credential"), "sidecar status was absent or disclosed credentials"
	case "mixed-resource-report":
		lower := strings.ToLower(output)
		return strings.Count(lower, "resource") >= 2 && strings.Contains(output, fixture.resourceID) && !strings.Contains(output, fixture.root), "CLI/REST resource reports were vacuous or disclosed local paths"
	}
	return false, "unknown loopback postcondition " + kind
}

func (h *repositoryExamples) cliShell(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	fixture := h.ensureFixture(invocation.Entry)
	if fixture.server != nil {
		fixture.server.Close()
		fixture.server = nil
	}
	if fixture.svc != nil {
		if err := fixture.svc.Close(); err != nil {
			return AdapterResult{}, err
		}
		fixture.svc = nil
	}
	if invocation.Entry.Registered.Execution.Postcondition.Kind == "cli-publish-plan" {
		prep := "notriosctl publish profile save --name public-site --query 'notebook:\"Public\"' --link-action plain_text --exclude-tags private,draft,internal"
		if output, status, err := runPinnedShell(ctx, fixture.root, h.cli, prep); err != nil {
			return AdapterResult{}, fmt.Errorf("prepare publish profile exit %d: %w\n%s", status, err, output)
		}
	}
	if invocation.Entry.Registered.Execution.Postcondition.Kind == "tag-exists" {
		// The page shows testing for a tag in a shell. Running it against a
		// library with no such tag would exercise the idiom and prove nothing,
		// so the tag is put there first and the example has to find it.
		prep := "notriosctl tags add --db data/notes.sqlite --document " + fixture.documentID + " --tag todo"
		if output, status, err := runPinnedShell(ctx, fixture.root, h.cli, prep); err != nil {
			return AdapterResult{}, fmt.Errorf("prepare tag exit %d: %w\n%s", status, err, output)
		}
	}
	if invocation.Entry.Registered.Execution.Postcondition.Kind == "cli-open" {
		prep := "notriosctl profile register --name fixture --db data/notes.sqlite"
		if output, status, err := runPinnedShell(ctx, fixture.root, h.cli, prep); err != nil {
			return AdapterResult{}, fmt.Errorf("prepare stable-link profile exit %d: %w\n%s", status, err, output)
		}
	}
	if err := h.prepareDocumentationImport(ctx, fixture, invocation.Entry.Registered.Execution.Postcondition.Kind); err != nil {
		return AdapterResult{}, err
	}
	output, status, err := runPinnedShell(ctx, fixture.root, h.cli, invocation.Body)
	if err != nil {
		return AdapterResult{}, fmt.Errorf("literal shell exit %d: %w\n%s", status, err, output)
	}
	ok := false
	switch invocation.Entry.Registered.Execution.Postcondition.Kind {
	case "version-output":
		ok = strings.TrimSpace(output) != ""
	case "tag-exists":
		ok = strings.Contains(output, "the tag exists")
	case "discovery-listing":
		// The documented point of these two commands is that they hand back
		// identifiers a query can name, so the check is that identifiers came
		// back -- not that a table was printed.
		ok = strings.Contains(output, "COLLECTION") && strings.Contains(output, "NOTEBOOK") &&
			strings.Contains(output, "default") && strings.Contains(output, "nb_") &&
			strings.Contains(output, "\"tags\"")
	case "archive-created":
		ok = fileExists(filepath.Join(fixture.root, "backups", "full", "manifest.json")) && fileExists(filepath.Join(fixture.root, "transfer", "research", "manifest.json"))
	case "dry-run-unchanged":
		ok = strings.Contains(strings.ToLower(output), "dry") && !fileExists(filepath.Join(fixture.root, "data", "publish-profiles.json"))
	case "profile-roundtrip":
		ok = strings.Contains(output, "public-site") && !strings.Contains(readOptional(filepath.Join(fixture.root, "data", "publish-profiles.json")), "public-site")
	case "stable-link":
		ok = strings.Contains(output, fixture.documentID) && strings.Contains(output, "notrios://")
	case "note-moved":
		st, closeStore, err := openFixtureStore(fixture)
		if err == nil {
			defer closeStore()
			doc, getErr := st.GetDocument(ctx, fixture.documentID)
			ok = getErr == nil && doc.NotebookID == fixture.notebookID
		}
	case "cli-graph-note":
		st, closeStore, err := openFixtureStore(fixture)
		if err == nil {
			defer closeStore()
			doc, getErr := st.GetDocument(ctx, store.GraphReportNoteID)
			ok = getErr == nil && doc.Title == store.GraphReportNoteTitle
		}
	case "cli-graph-export":
		ok = fileExists(filepath.Join(fixture.root, "graph", "nodes.csv")) && fileExists(filepath.Join(fixture.root, "graph", "edges.csv")) && strings.Contains(readOptional(filepath.Join(fixture.root, "graph", "edges.csv")), fixture.documentID)
	case "cli-archive-v1":
		ok = fileExists(filepath.Join(fixture.root, "my-archive", "manifest.json")) &&
			fileExists(filepath.Join(fixture.root, "todo-archive", "manifest.json")) && strings.Contains(output, `"notes"`)
	case "cli-snapshot-roundtrip":
		st, closeStore, err := openFixtureStore(fixture)
		if err == nil {
			defer closeStore()
			_, getErr := st.GetDocument(ctx, fixture.documentID)
			ok = getErr == nil && fileExists(filepath.Join(fixture.root, "notrios-physical-snapshot", "manifest.json"))
		}
	case "cli-fix-parity":
		ok = strings.Contains(output, `"apply": false`) && strings.Count(output, `"apply": true`) == 3
	case "cli-tag-rename":
		st, closeStore, err := openFixtureStore(fixture)
		if err == nil {
			defer closeStore()
			tags, tagErr := st.ListDocumentTags(ctx, fixture.documentID)
			if tagErr == nil {
				for _, tag := range tags {
					ok = ok || tag.Name == "work"
				}
			}
		}
	case "cli-publish-plan":
		ok = strings.Contains(output, "manifest_sha256") && strings.Contains(output, "public-site")
	case "cli-link-anchors":
		ok = strings.Count(output, fixture.documentID) >= 4 && strings.Contains(output, "agenda")
	case "cli-open":
		ok = strings.Contains(output, fixture.documentID) && strings.Contains(output, "http://127.0.0.1")
	default:
		if handled, importOK, importDetail := checkDocumentationImportPostcondition(ctx, fixture, invocation.Entry.Registered.Execution.Postcondition.Kind, output); handled {
			ok = importOK
			if !ok {
				return AdapterResult{Kind: "exit", Status: status, PostconditionOK: false, Detail: importDetail}, nil
			}
		}
	}
	detail := "CLI semantic postcondition failed"
	if !ok {
		detail = fmt.Sprintf("%s: output=%q", detail, output)
	}
	return AdapterResult{Kind: "exit", Status: status, PostconditionOK: ok, Detail: detail}, nil
}

func openFixtureStore(fixture *repositoryFixture) (*store.SQLiteStore, func(), error) {
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(fixture.root, "data", "notes.sqlite"), filepath.Join(fixture.root, "data", "assets"))
	if err != nil {
		return nil, func() {}, err
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		_ = st.Close()
		return nil, func() {}, err
	}
	return st, func() { _ = st.Close() }, nil
}

func runPinnedShell(ctx context.Context, root, cli, body string) (string, int, error) {
	bin := filepath.Join(root, "fixture-bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", -1, err
	}
	if err := symlinkCommand(cli, filepath.Join(bin, "notriosctl")); err != nil {
		return "", -1, err
	}
	curl, err := exec.LookPath("curl")
	if err != nil {
		return "", -1, err
	}
	if err := symlinkCommand(curl, filepath.Join(bin, "curl")); err != nil {
		return "", -1, err
	}
	jq := filepath.Join(bin, "jq")
	jqScript := `#!/bin/sh
if [ "$1" = "-r" ] && [ "$2" = ".current_revision_id" ]; then
  /usr/bin/python3 -c 'import json,sys; print(json.load(sys.stdin)["current_revision_id"])'
else
  /bin/cat
fi
`
	if err := os.WriteFile(jq, []byte(jqScript), 0o755); err != nil {
		return "", -1, err
	}
	command := exec.CommandContext(ctx, "/bin/bash", "-eu", "-o", "pipefail", "-c", body)
	command.Dir = root
	command.Env = []string{"HOME=" + root, "PATH=" + bin, "LANG=C", "LC_ALL=C"}
	combined, runErr := command.CombinedOutput()
	status := 0
	if runErr != nil {
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			status = exit.ExitCode()
		} else {
			status = -1
		}
	}
	return string(combined), status, runErr
}

func symlinkCommand(source, target string) error {
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Symlink(source, target)
}

func (h *repositoryExamples) daemonLifecycle(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	fixture := h.ensureFixture(invocation.Entry)
	address := unusedAddress(h.t)
	writeDaemonConfig(h.t, fixture.root, address)
	lines := runnableLines(invocation.Body)
	if len(lines) == 0 {
		return AdapterResult{}, fmt.Errorf("daemon example has no command")
	}
	daemonArgs := strings.Fields(lines[0])
	command := exec.CommandContext(ctx, daemonArgs[0], daemonArgs[1:]...)
	command.Dir = fixture.root
	command.Env = []string{"HOME=" + fixture.root, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	var logs bytes.Buffer
	command.Stdout, command.Stderr = &logs, &logs
	if err := command.Start(); err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() }()
	url := "http://" + address + "/healthz"
	healthy := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		response, err := http.Get(url) // #nosec G107 -- allocated loopback fixture only.
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok" {
				healthy = true
				break
			}
		}
	}
	if !healthy {
		return AdapterResult{}, fmt.Errorf("daemon never became healthy: %s", logs.String())
	}
	if len(lines) > 1 {
		args := strings.Fields(lines[1])
		doctor := exec.CommandContext(ctx, args[0], args[1:]...)
		doctor.Dir = fixture.root
		doctor.Env = command.Env
		if output, err := doctor.CombinedOutput(); err != nil {
			return AdapterResult{}, fmt.Errorf("documented doctor command: %w: %s", err, output)
		}
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		return AdapterResult{}, err
	}
	_ = command.Wait()
	return AdapterResult{Kind: "exit", Status: 0, PostconditionOK: healthy, Detail: "daemon lifecycle did not serve health"}, nil
}

func runnableLines(body string) []string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// writeDaemonConfig gives the sandbox a configuration the examples will find.
//
// It writes two copies. <root>/.config/notrios/config.yaml is where an
// installed binary looks, and the sandbox already runs with HOME=<root>, so
// this is the fixture behaving like a user who has a config rather than like a
// checkout. <root>/config/config.example.yaml stays because some documented
// examples name that path literally on the command line.
//
// Before H4 only the second existed and the CLI found it by looking in the
// working directory. That is the behaviour slice B removed -- an installed
// binary must not take its database path, listen address and remote-media
// policy from wherever it was launched -- so a fixture relying on it stopped
// working, and the examples silently ran against a different, empty database.
func writeDaemonConfig(t *testing.T, root, address string) {
	t.Helper()
	value := fmt.Sprintf("server:\n  listen_addr: %q\ndata:\n  directory: %q\n  database_path: %q\n  asset_store: %q\n  projection_dir: %q\nsync:\n  target: none\n", address, filepath.Join(root, "data"), filepath.Join(root, "data", "notes.sqlite"), filepath.Join(root, "data", "assets"), filepath.Join(root, "data", "projections"))
	for _, path := range []string{
		filepath.Join(root, ".config", "notrios", "config.yaml"),
		filepath.Join(root, "config", "config.example.yaml"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func readOptional(path string) string {
	value, _ := os.ReadFile(path)
	return string(value)
}
