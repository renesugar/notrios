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

	"github.com/renesugar/notrios/internal/docplan"
)

const (
	ledgerPath  = "docs/docplan/PLAN_SLICES.json"
	planPath    = "PLAN.md"
	roadmapPath = "ROADMAP.md"
	begin       = "<!-- notrios:generated:plan:progress:begin -->"
	finish      = "<!-- notrios:generated:plan:progress:end -->"
	roadBegin   = "<!-- notrios:generated:roadmap:status:begin -->"
	roadFinish  = "<!-- notrios:generated:roadmap:status:end -->"
)

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
		fmt.Printf("progress log and roadmap status written: %d items\n", len(ledger.Items))
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
