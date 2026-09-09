package docrules

import (
	"fmt"
	"strings"
)

// TableLines renders the inventory as the table AGENTS.md carries.
//
// The table is generated for the same reason the plan's progress log is: a
// hand-written list of twenty-five documents and what each is for is a list
// that goes wrong quietly. Here it can only go wrong loudly, because the
// registry it comes from is also what the build checks the documents against.
func (r Registry) TableLines() []string {
	lines := []string{
		fmt.Sprintf("**%d root documents have a home here, %d of them carrying a pointer back; %d more are exempt.**",
			len(r.Documents), len(r.Documents)-r.pointerless(), len(r.Exempt)),
		"",
		"| Document | The home for | Not here |",
		"|---|---|---|",
	}
	for _, doc := range r.Documents {
		name := fmt.Sprintf("`%s`", doc.Path)
		if doc.Section != "" {
			name += fmt.Sprintf(" *(%s)*", doc.Section)
		}
		lines = append(lines, fmt.Sprintf("| %s | %s | %s |", name, doc.Home, doc.NotHere))
	}
	lines = append(lines, "", "Exempt, because each is the contract for one feature rather than for the project:", "")
	for _, exempt := range r.Exempt {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", exempt.Path, exempt.Reason))
	}
	return lines
}

// Pointerless reports whether a document holds no pointer of its own.
func (r Registry) Pointerless(doc Document) bool { return doc.pointerless() }

// Satisfied reports whether a document already sends its reader to the right
// section of AGENTS.md, in whatever words. PLAN.md names three sections rather
// than one, and a check that demanded the generated sentence exactly would
// force it to say less than it needs to.
func (r Registry) Satisfied(doc Document, body string) bool {
	if doc.pointerless() {
		return true
	}
	return strings.Contains(body, "AGENTS.md") && strings.Contains(body, doc.section(r.Section))
}

// pointerless counts the documents that are the rules, or only point at them.
func (r Registry) pointerless() int {
	count := 0
	for _, doc := range r.Documents {
		if doc.pointerless() {
			count++
		}
	}
	return count
}

// Pointer is the line a registered document carries, so that the wording is
// written once and the same sentence appears in all of them.
func (r Registry) Pointer(doc Document) string {
	return fmt.Sprintf(
		"**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under \"%s\".",
		doc.section(r.Section))
}
