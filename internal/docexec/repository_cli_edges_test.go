package docexec

// This file contains the small CLI adapters whose examples are useful to run
// but whose result is not naturally covered by the larger repository harness.
// They are deliberately methods on repositoryExamples so the manifest runner
// can register them as closed, named adapters.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func (h *repositoryExamples) cliEdge(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	switch invocation.Entry.Registered.Execution.Postcondition.Kind {
	case "cli-lint":
		return h.runWorkspaceLint(ctx, invocation)
	case "cli-gc-dry-run":
		return h.runGarbageCollectionDryRun(ctx, invocation)
	case "cli-job-command":
		return h.runJobsShowCommand(ctx, invocation)
	case "cli-url-handler-preview":
		return h.runRegisterURLHandlerPreview(ctx, invocation)
	default:
		return AdapterResult{}, fmt.Errorf("unknown CLI edge postcondition %q", invocation.Entry.Registered.Execution.Postcondition.Kind)
	}
}

// runWorkspaceLint executes the read-only lint example. Findings are a valid
// result (the CLI uses exit 1 for a non-empty report), so status 1 is accepted
// while the JSON report and the seeded database provide the semantic fence.
func (h *repositoryExamples) runWorkspaceLint(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	fixture := h.ensureFixture(invocation.Entry)
	if err := h.closeFixture(fixture); err != nil {
		return AdapterResult{}, err
	}
	lines := runnableLines(invocation.Body)
	if len(lines) != 4 {
		return AdapterResult{}, fmt.Errorf("lint example has %d runnable lines, want 4", len(lines))
	}
	outputs := make([]string, len(lines))
	statuses := make([]int, len(lines))
	for i, line := range lines {
		output, status, err := runPinnedShell(ctx, fixture.root, h.cli, line)
		outputs[i], statuses[i] = output, status
		if err != nil && status != 1 {
			return AdapterResult{}, fmt.Errorf("lint line %d exit %d: %w\n%s", i+1, status, err, output)
		}
	}
	var report struct {
		Checks        []any `json:"checks"`
		TotalFindings int   `json:"total_findings"`
	}
	if json.Unmarshal([]byte(outputs[0]), &report) != nil || len(report.Checks) == 0 {
		return AdapterResult{}, fmt.Errorf("lint did not emit a JSON checks report: %q", outputs[0])
	}
	st, closeStore, openErr := openFixtureStore(fixture)
	if openErr != nil {
		return AdapterResult{}, openErr
	}
	defer closeStore()
	if _, getErr := st.GetDocument(ctx, fixture.documentID); getErr != nil {
		return AdapterResult{}, fmt.Errorf("lint changed canonical state: %w", getErr)
	}
	ok := report.TotalFindings > 0 && statuses[0] == 1 && statuses[1] == 1 && statuses[2] == 1 && statuses[3] == 0 && strings.TrimSpace(outputs[1]) == "" && strings.Contains(outputs[3], "unreferenced_resource")
	return AdapterResult{Kind: "exit", Status: 1, PostconditionOK: ok, Detail: "lint report, quiet mode, selected checks, or check inventory contradicted the documented exit contract"}, nil
}

// runGarbageCollectionDryRun accepts the documented planning exit and verifies
// that dry-run did not remove the seeded logical resource.
func (h *repositoryExamples) runGarbageCollectionDryRun(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	fixture := h.ensureFixture(invocation.Entry)
	if err := h.closeFixture(fixture); err != nil {
		return AdapterResult{}, err
	}
	output, status, err := runPinnedShell(ctx, fixture.root, h.cli, invocation.Body)
	if err != nil {
		return AdapterResult{}, fmt.Errorf("gc dry-run exit %d: %w\n%s", status, err, output)
	}
	var report struct {
		Eligible []any `json:"eligible"`
		Retained []any `json:"retained"`
		Removed  []any `json:"removed"`
	}
	if json.Unmarshal([]byte(output), &report) != nil || report.Removed == nil {
		return AdapterResult{}, fmt.Errorf("gc dry-run did not emit a plan: %q", output)
	}
	st, closeStore, openErr := openFixtureStore(fixture)
	if openErr != nil {
		return AdapterResult{}, openErr
	}
	defer closeStore()
	if _, getErr := st.GetResource(ctx, fixture.resourceID); getErr != nil {
		return AdapterResult{}, fmt.Errorf("gc dry-run removed the seeded resource: %w", getErr)
	}
	return AdapterResult{Kind: "exit", Status: status, PostconditionOK: strings.Contains(output, `"removed"`), Detail: "dry-run reported a retention plan and preserved the resource"}, nil
}

// runJobsShowCommand checks the local-only command rendering fence. The output
// must be a command derived from stored parameters and must not expose the
// scratch database path.
func (h *repositoryExamples) runJobsShowCommand(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	fixture := h.ensureFixture(invocation.Entry)
	if err := h.closeFixture(fixture); err != nil {
		return AdapterResult{}, err
	}
	lines := strings.Split(invocation.Body, "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "$ ") {
		return AdapterResult{}, fmt.Errorf("jobs example is not a two-line shell transcript")
	}
	output, status, err := runPinnedShell(ctx, fixture.root, h.cli, strings.TrimPrefix(lines[0], "$ "))
	if err != nil {
		return AdapterResult{}, fmt.Errorf("jobs show --command exit %d: %w\n%s", status, err, output)
	}
	want := strings.TrimSpace(lines[1])
	ok := strings.TrimSpace(output) == want && !strings.Contains(output, filepath.Join(fixture.root, "data"))
	return AdapterResult{Kind: "exit", Status: status, PostconditionOK: ok, Detail: "job command did not match the documented rendering from typed parameters"}, nil
}

// runRegisterURLHandlerPreview exercises the safe default: print the desktop
// entry, but do not write into the user's applications directory.
func (h *repositoryExamples) runRegisterURLHandlerPreview(ctx context.Context, invocation Invocation) (AdapterResult, error) {
	root := h.t.TempDir()
	output, status, err := runPinnedShell(ctx, root, h.cli, invocation.Body)
	if err != nil {
		return AdapterResult{}, fmt.Errorf("URL handler preview exit %d: %w\n%s", status, err, output)
	}
	var report struct {
		Applied bool   `json:"applied"`
		Scheme  string `json:"scheme"`
		Entry   string `json:"contents"`
	}
	if json.Unmarshal([]byte(output), &report) != nil {
		return AdapterResult{}, fmt.Errorf("URL handler preview was not JSON: %q", output)
	}
	ok := !report.Applied && report.Scheme == "notrios" &&
		strings.Contains(report.Entry, "x-scheme-handler/notrios") &&
		!fileExists(filepath.Join(root, ".local", "share", "applications", "notrios-url-handler.desktop"))
	return AdapterResult{Kind: "exit", Status: status, PostconditionOK: ok, Detail: "URL handler preview printed the entry without installing it"}, nil
}

// closeFixture stops the loopback server and service before a CLI opens the
// same SQLite database. It is idempotent so adapters remain safe on retries.
func (h *repositoryExamples) closeFixture(fixture *repositoryFixture) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if fixture.server != nil {
		fixture.server.Close()
		fixture.server = nil
	}
	if fixture.svc != nil {
		err := fixture.svc.Close()
		fixture.svc = nil
		return err
	}
	return nil
}
