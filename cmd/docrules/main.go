// Command docrules checks that every root document says where the rules for
// keeping it current live, and writes the inventory table into AGENTS.md.
//
// It is separate from cmd/docplan for the same reason that one is separate from
// cmd/docgen: the plan's generator answers "what is left", this one answers
// "which document owns this fact", and a generator that answers two questions
// is one nobody can change safely.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/renesugar/notrios/internal/docrules"
)

func main() {
	root := flag.String("root", ".", "repository root")
	write := flag.Bool("write", false, "rewrite the inventory table in AGENTS.md, and the pointer in each document")
	flag.Parse()

	registry, err := docrules.Load(*root)
	if err != nil {
		fail(err)
	}

	if *write {
		written, err := writePointers(*root, registry)
		if err != nil {
			fail(err)
		}
		if err := writeTable(*root, registry); err != nil {
			fail(err)
		}
		fmt.Printf("documents inventory written: %d documents, %d pointers added\n", len(registry.Documents), written)
		return
	}

	problems := docrules.Check(*root)
	problems = append(problems, docrules.CheckTable(*root)...)
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, "documents:", problem)
		}
		os.Exit(1)
	}
	fmt.Printf("documents inventory valid: %d documents, %d exempt\n", len(registry.Documents), len(registry.Exempt))
}

// writePointers puts the generated pointer line into any registered document
// that lacks it, immediately after the document's title and opening paragraph.
//
// It is written rather than instructed because the wording is the same in
// twenty-two documents, and twenty-two hand-typed copies of one sentence is
// twenty-two chances for it to say something slightly different.
func writePointers(root string, registry docrules.Registry) (int, error) {
	written := 0
	for _, doc := range registry.Documents {
		full := filepath.Join(root, doc.Path)
		contents, err := os.ReadFile(full)
		if err != nil {
			return written, err
		}
		body := string(contents)
		if registry.Satisfied(doc, body) {
			continue
		}
		pointer := registry.Pointer(doc)
		updated, err := insertPointer(body, pointer)
		if err != nil {
			return written, fmt.Errorf("%s: %w", doc.Path, err)
		}
		if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

// insertPointer places the line after the opening paragraph, so a reader who
// stops at the first screen has still seen it, and a reader who wants the
// document's own subject is not made to step over a maintenance note first.
func insertPointer(body, pointer string) (string, error) {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return "", fmt.Errorf("does not begin with a title")
	}
	// Go past the title and the opening paragraph, plus any list, table or
	// fenced block that paragraph introduces -- CODING_STANDARDS.md opens with
	// a sentence whose whole meaning is the list under it, and a line dropped
	// between the two would read as though the list were the pointer's.
	at := 1
	first := true
	for at < len(lines) {
		for at < len(lines) && strings.TrimSpace(lines[at]) == "" {
			at++
		}
		if at >= len(lines) || strings.HasPrefix(lines[at], "#") {
			break
		}
		if !first && !introduced(lines[at]) {
			break
		}
		for at < len(lines) && strings.TrimSpace(lines[at]) != "" && !strings.HasPrefix(lines[at], "#") {
			at++
		}
		first = false
	}
	before := lines[:at]
	for len(before) > 0 && strings.TrimSpace(before[len(before)-1]) == "" {
		before = before[:len(before)-1]
	}
	after := lines[at:]
	for len(after) > 0 && strings.TrimSpace(after[0]) == "" {
		after = after[1:]
	}
	inserted := append([]string{}, before...)
	inserted = append(inserted, "", pointer, "")
	inserted = append(inserted, after...)
	return strings.Join(inserted, "\n"), nil
}

func writeTable(root string, registry docrules.Registry) error {
	rendered, err := registry.Render(root)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, docrules.AgentsPath), []byte(rendered), 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "docrules:", err)
	os.Exit(1)
}

// introduced reports whether a block belongs to the paragraph above it rather
// than standing on its own: a list, a table, or a fenced example.
func introduced(line string) bool {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
		return true
	case strings.HasPrefix(trimmed, "|"), strings.HasPrefix(trimmed, "```"):
		return true
	}
	return len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[2] == '.')
}
