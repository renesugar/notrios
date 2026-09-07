// Package docrules holds the inventory of root documents and checks that each
// one tells its reader where the rules for keeping it current live.
//
// Every root document is the home for one kind of fact. The failure this
// package exists to catch is a document that copies a fact whose home is
// somewhere else: the copy is right on the day it is written and wrong
// afterwards, and nothing notices, because prose has no gate. It had already
// happened three times over when this was written -- ROADMAP.md and
// CONTEXT_MAP.md both carried the same month-old sentence about which plan item
// was next, and PROMPT.md carried a second reading list that had drifted from
// the one in AGENTS.md.
//
// So the inventory is data rather than prose, the table in AGENTS.md is
// generated from it, and a document that stops naming AGENTS.md fails the
// build.
package docrules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Schema is the registry's schema identifier.
const Schema = "notrios.docrules.documents.v1"

// Kind says what a document is for, which decides how it is kept current.
type Kind string

const (
	// KindSource is AGENTS.md: the document the others point at.
	KindSource Kind = "source"
	// KindPointer exists only to redirect a reader somewhere else.
	KindPointer Kind = "pointer"
	// KindJournal records what happened, in the order it happened.
	KindJournal Kind = "journal"
	// KindReference states how something is now, and is kept current with it.
	KindReference Kind = "reference"
	// KindPolicy states what is allowed, and changes only by decision.
	KindPolicy Kind = "policy"
	// KindSnapshot describes a moment and is superseded rather than edited.
	KindSnapshot Kind = "snapshot"
)

// Document is one root document and the single kind of fact it owns.
type Document struct {
	Path string `json:"path"`
	Kind Kind   `json:"kind"`
	// Home is what this document, and no other, is the place to look for.
	Home string `json:"home"`
	// NotHere is the fact most likely to be copied in from somewhere else.
	NotHere string `json:"not_here"`
	// Section names the AGENTS.md section holding rules specific to this
	// document. Empty means the shared section governs it.
	Section string `json:"section,omitempty"`
}

// Exemption is a root document that carries no pointer, and why.
type Exemption struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Registry is the whole inventory: every root document is in one list or the
// other, which is what stops a new document from quietly skipping the rule.
type Registry struct {
	Schema    string      `json:"schema"`
	Section   string      `json:"section"`
	Documents []Document  `json:"documents"`
	Exempt    []Exemption `json:"exempt"`
}

// RegistryPath is where the inventory lives, relative to the repository root.
const RegistryPath = "docs/docrules/DOCUMENTS.json"

// Load reads the inventory.
func Load(root string) (Registry, error) {
	var registry Registry
	contents, err := os.ReadFile(filepath.Join(root, RegistryPath))
	if err != nil {
		return registry, err
	}
	if err := json.Unmarshal(contents, &registry); err != nil {
		return registry, fmt.Errorf("%s: %w", RegistryPath, err)
	}
	if registry.Schema != Schema {
		return registry, fmt.Errorf("%s: schema is %q, want %q", RegistryPath, registry.Schema, Schema)
	}
	if strings.TrimSpace(registry.Section) == "" {
		return registry, fmt.Errorf("%s: no shared section named", RegistryPath)
	}
	return registry, nil
}

// pointerless are the kinds that hold no pointer of their own: AGENTS.md is
// where the rules are, and CLAUDE.md is already a pointer to it.
func (d Document) pointerless() bool {
	return d.Kind == KindSource || d.Kind == KindPointer
}

// section is the AGENTS.md section a document's reader is sent to.
func (d Document) section(shared string) string {
	if d.Section != "" {
		return d.Section
	}
	return shared
}

// Check reports every way the inventory and the documents disagree.
//
// It returns problems rather than one error because a caller fixing several
// documents wants to see all of them, and because a check that stops at the
// first failure trains its reader to fix one thing per run.
func Check(root string) []string {
	registry, err := Load(root)
	if err != nil {
		return []string{err.Error()}
	}

	problems := []string{}
	known := map[string]bool{}
	sections := map[string]bool{}

	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return []string{fmt.Sprintf("cannot read AGENTS.md: %v", err)}
	}
	agentsBody := string(agents)

	for _, doc := range registry.Documents {
		if known[doc.Path] {
			problems = append(problems, fmt.Sprintf("%s is listed twice", doc.Path))
		}
		known[doc.Path] = true

		if strings.TrimSpace(doc.Home) == "" {
			problems = append(problems, fmt.Sprintf("%s names no home, so nothing decides what belongs in it", doc.Path))
		}
		if strings.TrimSpace(doc.NotHere) == "" {
			problems = append(problems, fmt.Sprintf("%s names nothing that does not belong in it", doc.Path))
		}

		body, err := os.ReadFile(filepath.Join(root, doc.Path))
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read %s: %v", doc.Path, err))
			continue
		}
		text := string(body)

		section := doc.section(registry.Section)
		sections[section] = true

		if doc.pointerless() {
			continue
		}
		if !strings.Contains(text, "AGENTS.md") {
			problems = append(problems, fmt.Sprintf(
				"%s does not point at AGENTS.md, so the rules for keeping it current are findable only by accident", doc.Path))
			continue
		}
		if !strings.Contains(text, section) {
			problems = append(problems, fmt.Sprintf(
				"%s points at AGENTS.md but not at %q, so its reader arrives at three hundred lines and no section", doc.Path, section))
		}
	}

	for _, exempt := range registry.Exempt {
		if known[exempt.Path] {
			problems = append(problems, fmt.Sprintf("%s is both registered and exempt", exempt.Path))
		}
		known[exempt.Path] = true
		if strings.TrimSpace(exempt.Reason) == "" {
			problems = append(problems, fmt.Sprintf("%s is exempt without a reason", exempt.Path))
		}
		if _, err := os.Stat(filepath.Join(root, exempt.Path)); err != nil {
			problems = append(problems, fmt.Sprintf("%s is exempt but missing: %v", exempt.Path, err))
		}
	}

	for section := range sections {
		if !strings.Contains(agentsBody, "## "+section) {
			problems = append(problems, fmt.Sprintf(
				"AGENTS.md has no %q section, but documents are sent there", section))
		}
	}

	found, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		problems = append(problems, err.Error())
	}
	for _, path := range found {
		name := filepath.Base(path)
		if !known[name] {
			problems = append(problems, fmt.Sprintf(
				"%s is a root document that is neither registered nor exempt in %s; "+
					"say what it is the home for, or say why it needs no pointer", name, RegistryPath))
		}
	}

	sort.Strings(problems)
	return problems
}
