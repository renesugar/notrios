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
	ledgerPath = "docs/docplan/PLAN_SLICES.json"
	planPath   = "PLAN.md"
	begin      = "<!-- notrios:generated:plan:progress:begin -->"
	finish     = "<!-- notrios:generated:plan:progress:end -->"
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
	if problems := docplan.Check(*root, ledger, headings); len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, "plan ledger:", problem)
		}
		os.Exit(1)
	}

	rendered, err := render(filepath.Join(*root, planPath), ledger)
	if err != nil {
		fail(err)
	}
	if !*write {
		current, err := os.ReadFile(filepath.Join(*root, planPath))
		if err != nil {
			fail(err)
		}
		if string(current) != rendered {
			fmt.Fprintln(os.Stderr, "plan ledger: the progress log in PLAN.md is stale; run `go run ./cmd/docplan --write`")
			os.Exit(1)
		}
		fmt.Printf("plan ledger valid: %d items\n", len(ledger.Items))
		return
	}
	if err := os.WriteFile(filepath.Join(*root, planPath), []byte(rendered), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("progress log written: %d items\n", len(ledger.Items))
}

var block = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(begin) + `.*?` + regexp.QuoteMeta(finish))

func render(path string, ledger docplan.Ledger) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	body := string(contents)
	if !block.MatchString(body) {
		return "", fmt.Errorf("PLAN.md has no progress-log markers")
	}
	generated := begin + "\n" + strings.Join(ledger.ProgressLines(), "\n") + "\n" + finish
	return block.ReplaceAllLiteralString(body, generated), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "docplan:", err)
	os.Exit(1)
}
