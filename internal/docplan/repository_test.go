package docplan

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
}
