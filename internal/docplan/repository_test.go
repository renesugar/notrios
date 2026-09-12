package docplan

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/version"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
}

// TestPlanLedgerAgreesWithThePlan is the gate the ledger exists for.
//
// A ledger nobody checks becomes what it replaced. This repository has watched
// hand-maintained prose go stale while every other check was green -- a page
// that said tagging was unreachable months after it was built, a gap count of
// fifteen when it was five, a pinned control count of fifty-eight when the
// interface had fifty-nine -- so the ledger's claims are checked the same way
// the documentation's are. Its first run found four items whose Outcome block
// said Complete while their heading did not, which is the same class of defect
// on the plan itself.
func TestPlanLedgerAgreesWithThePlan(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := Load(filepath.Join(root, "docs", "docplan", "PLAN_SLICES.json"))
	if err != nil {
		t.Fatalf("loading the plan ledger: %v", err)
	}
	headings, err := PlanHeadings(filepath.Join(root, "PLAN.md"))
	if err != nil {
		t.Fatalf("reading PLAN.md: %v", err)
	}
	problems := Check(root, ledger, headings)
	problems = append(problems, CheckPlanPointer(filepath.Join(root, "PLAN.md"))...)
	problems = append(problems, CheckRoadmapPointer(filepath.Join(root, "ROADMAP.md"))...)
	for _, problem := range problems {
		t.Errorf("%s\n\nfix the ledger or the plan, then run: go run ./cmd/docplan --write", problem)
	}
}

// TestProgressLogIsCurrent keeps the generated section from drifting from the
// ledger it is rendered from, which is the only reason to generate it at all.
func TestProgressLogIsCurrent(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := Load(filepath.Join(root, "docs", "docplan", "PLAN_SLICES.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := os.ReadFile(filepath.Join(root, "PLAN.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(plan)
	const begin = "<!-- notrios:generated:plan:progress:begin -->"
	const finish = "<!-- notrios:generated:plan:progress:end -->"
	start, end := strings.Index(body, begin), strings.Index(body, finish)
	if start < 0 || end < start {
		t.Fatal("PLAN.md has no progress-log markers")
	}
	got := strings.TrimSpace(body[start+len(begin) : end])
	want := strings.TrimSpace(strings.Join(ledger.ProgressLines(), "\n"))
	if got != want {
		t.Error("the progress log in PLAN.md is stale; run: go run ./cmd/docplan --write")
	}

	// The roadmap quotes the same ledger, so it can go stale the same way --
	// and did, for a month, which is why it is generated now.
	roadmap, err := os.ReadFile(filepath.Join(root, "ROADMAP.md"))
	if err != nil {
		t.Fatal(err)
	}
	body = string(roadmap)
	const roadBegin = "<!-- notrios:generated:roadmap:status:begin -->"
	const roadFinish = "<!-- notrios:generated:roadmap:status:end -->"
	start, end = strings.Index(body, roadBegin), strings.Index(body, roadFinish)
	if start < 0 || end < start {
		t.Fatal("ROADMAP.md has no active-plan status markers")
	}
	got = strings.TrimSpace(body[start+len(roadBegin) : end])
	want = strings.TrimSpace(strings.Join(ledger.RoadmapStatusLines(), "\n"))
	if got != want {
		t.Error("the active-plan status in ROADMAP.md is stale; run: go run ./cmd/docplan --write")
	}

	// And the README, which is the document that proved the point. Its status
	// paragraph was written by hand and announced "v0.8 (current, 0.8.0)"
	// through the whole of v0.9 and the first three items of v1.0, never
	// mentioning v0.9 at all. Nothing failed, because nothing was checking.
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	body = string(readme)
	const readmeBegin = "<!-- notrios:generated:readme:status:begin -->"
	const readmeFinish = "<!-- notrios:generated:readme:status:end -->"
	start, end = strings.Index(body, readmeBegin), strings.Index(body, readmeFinish)
	if start < 0 || end < start {
		t.Fatal("README.md has no project-status markers")
	}
	archived, err := archivedMilestoneNames(root)
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(body[start+len(readmeBegin) : end])
	want = strings.TrimSpace(strings.Join(ledger.ReadmeStatusLines(version.Version, archived), "\n"))
	if got != want {
		t.Error("the project status in README.md is stale; run: go run ./cmd/docplan --write")
	}

	// The generated block replaces the version claim; nothing may keep a second
	// copy of it beside the block. "(current, 0.8.0)" is the exact string that
	// was wrong, and a milestone row that marks itself current is the shape of
	// the mistake rather than one instance of it.
	statusStart := strings.Index(body, "## Project status")
	statusEnd := strings.Index(body[statusStart:], "\n## ")
	if statusStart < 0 || statusEnd < 0 {
		t.Fatal("README.md has no Project status section")
	}
	section := body[statusStart : statusStart+statusEnd]
	if index := strings.Index(section, "(current"); index >= 0 {
		t.Errorf("a milestone row in README.md still marks itself current: %q; "+
			"the generated block above it owns that claim",
			strings.TrimSpace(section[index:min(index+40, len(section))]))
	}
}

// archivedMilestoneNames mirrors cmd/docplan's own derivation. It is duplicated
// rather than exported because the command owns the rendering and this test
// owns catching the command's output going stale; sharing the function would
// let one bug satisfy both.
func archivedMilestoneNames(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "plans"))
	if err != nil {
		return nil, err
	}
	shaped := regexp.MustCompile(`^v[0-9]+\.[0-9]+[a-z]?$`)
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() && shaped.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
