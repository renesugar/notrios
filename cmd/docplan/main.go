// Command docplan checks the plan's slice ledger and writes the progress log
// into PLAN.md.
//
// It is deliberately not part of cmd/docgen. That generator serves the user
// manual: its fragments come from Go doc comments, its slots are anchored to
// help topics, and everything it touches is seeded into the Help notebook as
// notes somebody reads. The plan is not user documentation and must not become
// a Help note, so it gets a generator of its own that shares the checking and
// nothing else.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sort"

	"github.com/renesugar/notrios/internal/docplan"
	"github.com/renesugar/notrios/internal/version"
)

const (
	ledgerPath   = "docs/docplan/PLAN_SLICES.json"
	planPath     = "PLAN.md"
	roadmapPath  = "ROADMAP.md"
	readmePath   = "README.md"
	begin        = "<!-- notrios:generated:plan:progress:begin -->"
	finish       = "<!-- notrios:generated:plan:progress:end -->"
	roadBegin    = "<!-- notrios:generated:roadmap:status:begin -->"
	roadFinish   = "<!-- notrios:generated:roadmap:status:end -->"
	readmeBegin  = "<!-- notrios:generated:readme:status:begin -->"
	readmeFinish = "<!-- notrios:generated:readme:status:end -->"
	evidBegin    = "<!-- notrios:generated:readme:evidence:begin -->"
	evidFinish   = "<!-- notrios:generated:readme:evidence:end -->"
)

// evidenceIndex counts the evidence directories per milestone.
//
// The paragraph it replaces named 13 of 77 and stopped at v0.5, so v0.7, v0.8,
// v0.8e, v0.9 and v1.0 -- the large majority of the evidence in this repository
// -- went unmentioned. It is a list of directories on disk, which is exactly
// what should never be maintained by hand.
//
// Counts and one example each, not 77 names: a reader wants to know the
// evidence exists, roughly how much of it there is, and how to find the rest.
// `ls performance/` does the rest better than a wall of directory names.
func evidenceIndex(root string) ([]string, int, error) {
	entries, err := os.ReadDir(filepath.Join(root, "performance"))
	if err != nil {
		return nil, 0, err
	}
	milestone := regexp.MustCompile(`^(v[0-9]+\.[0-9]+[a-z]?)(?:-.*)?$`)
	counts := map[string]int{}
	first := map[string]string{}
	order := []string{}
	total := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		match := milestone.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		key := match[1]
		if counts[key] == 0 {
			order = append(order, key)
			first[key] = entry.Name()
		}
		counts[key]++
		total++
	}
	sort.Strings(order)
	rows := []string{
		"| Milestone | Evidence directories | For example |",
		"|---|---|---|",
	}
	for _, key := range order {
		rows = append(rows, fmt.Sprintf("| %s | %d | `performance/%s/` |",
			key, counts[key], first[key]))
	}
	return rows, total, nil
}

// archivedMilestones lists the milestone directories under plans/.
//
// Derived rather than listed, because a list is the thing that goes stale: this
// is the same file that spent a milestone and a half announcing v0.8 as current.
// Only version-shaped names are reported; `plans/scaffold` and `plans/mvp` are
// records of how the repository started rather than milestones with a number,
// and the README says so in prose beside the generated block.
func archivedMilestones(root string) ([]string, error) {
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

func main() {
	root := flag.String("root", ".", "repository root")
	write := flag.Bool("write", false, "rewrite the progress log in PLAN.md")
	flag.Parse()

	ledger, err := docplan.Load(filepath.Join(*root, ledgerPath))
	if err != nil {
		fail(err)
	}
	headings, err := docplan.PlanHeadings(filepath.Join(*root, planPath))
	if err != nil {
		fail(err)
	}
	problems := docplan.Check(*root, ledger, headings)
	problems = append(problems, docplan.CheckPlanPointer(filepath.Join(*root, planPath))...)
	problems = append(problems, docplan.CheckRoadmapPointer(filepath.Join(*root, roadmapPath))...)
	archived, err := archivedMilestones(*root)
	if err != nil {
		fail(err)
	}
	evidenceRows, evidenceTotal, err := evidenceIndex(*root)
	if err != nil {
		fail(err)
	}
	evidenceLines := append([]string{
		fmt.Sprintf("%d evidence directories under [`performance/`](performance/), generated "+
			"by `go run ./cmd/docplan --write`. Each holds the record for one plan item: what "+
			"was measured, how, and what was not.", evidenceTotal),
		"",
	}, evidenceRows...)
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, "plan ledger:", problem)
		}
		os.Exit(1)
	}

	// Both documents are rendered from one ledger, so the plan's progress log
	// and the roadmap's status sentence cannot disagree about the same facts.
	targets := []struct {
		path          string
		begin, finish string
		lines         []string
	}{
		{planPath, begin, finish, ledger.ProgressLines()},
		{roadmapPath, roadBegin, roadFinish, ledger.RoadmapStatusLines()},
		{readmePath, readmeBegin, readmeFinish, ledger.ReadmeStatusLines(version.Version, archived)},
		{readmePath, evidBegin, evidFinish, evidenceLines},
	}
	for _, target := range targets {
		full := filepath.Join(*root, target.path)
		rendered, err := render(full, target.begin, target.finish, target.lines)
		if err != nil {
			fail(err)
		}
		current, err := os.ReadFile(full)
		if err != nil {
			fail(err)
		}
		if !*write {
			if string(current) != rendered {
				fmt.Fprintf(os.Stderr, "plan ledger: the generated block in %s is stale; run `go run ./cmd/docplan --write`\n", target.path)
				os.Exit(1)
			}
			continue
		}
		if err := os.WriteFile(full, []byte(rendered), 0o644); err != nil {
			fail(err)
		}
	}
	if *write {
		fmt.Printf("plan, roadmap and README blocks written: %s, %d items, %d archived milestones\n",
			ledger.Milestone, len(ledger.Items), len(archived))
		return
	}
	fmt.Printf("plan ledger valid: %d items\n", len(ledger.Items))
}

func render(path, begin, finish string, lines []string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	body := string(contents)
	block := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(begin) + `.*?` + regexp.QuoteMeta(finish))
	if !block.MatchString(body) {
		return "", fmt.Errorf("%s has no generated markers for this block", filepath.Base(path))
	}
	return block.ReplaceAllLiteralString(body, begin+"\n"+strings.Join(lines, "\n")+"\n"+finish), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "docplan:", err)
	os.Exit(1)
}
