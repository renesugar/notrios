// Package clispec is the command line's one description of itself.
//
// Before this package there were three, maintained separately: the usage
// literal in printHelp, a usage function per command group, and each command's
// own flag definitions. They had already drifted -- eight subcommands existed,
// worked, and appeared in none of the help text -- and the drift was invisible
// because nothing compared any of them to the dispatcher.
//
// The cost was larger than a missing help line. docs/cli.md is generated from
// the printHelp literal, the Help notebook is seeded from that, and
// internal/docgen parsed the same literal to build the command-line surface
// inventory that the features coverage check, doccompare and the unclaimed
// ratchet all count against. So one hand-typed string was the user's help, the
// published reference, the offline documentation and the denominator of a gate.
// A command missing from it was missing from all of them at once.
//
// Here the description is data, the dispatcher is checked against it, and
// everything else is derived from it.
package clispec

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Schema is the registry's schema identifier.
const Schema = "notrios.clispec.commands.v1"

//go:embed commands.json
var registryJSON []byte

// Command is one thing the command line can be asked to do.
type Command struct {
	// Path is the words a user types after the program name: ["notes","show"].
	Path []string `json:"path"`
	// Purpose is the single line shown beside the command in a listing.
	Purpose string `json:"purpose"`
	// Usage is the flags and arguments, in the form the documentation uses.
	// It is described here rather than constructed: each command keeps its own
	// FlagSet, and a test checks that the two agree.
	Usage string `json:"usage"`
	// Notes are the qualifications a reader needs and a usage line cannot hold.
	Notes []string `json:"notes"`
	// Exempt, when set, is the reason this command is not offered to users.
	// It is a stated reason rather than silence, because silence is what let
	// eight commands go unmentioned.
	Exempt string `json:"exempt,omitempty"`
}

// Group is a command that exists only to hold subcommands.
type Group struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

// Registry is the whole command line.
type Registry struct {
	Schema   string    `json:"schema"`
	Program  string    `json:"program"`
	Tagline  string    `json:"tagline"`
	Groups   []Group   `json:"groups"`
	Commands []Command `json:"commands"`
}

// Load returns the embedded registry.
//
// It is embedded rather than read from disk so the binary describes itself
// wherever it runs: a help text that depends on a file beside the executable is
// a help text that is missing from an installed package.
func Load() (Registry, error) {
	var registry Registry
	if err := json.Unmarshal(registryJSON, &registry); err != nil {
		return registry, fmt.Errorf("clispec: %w", err)
	}
	if registry.Schema != Schema {
		return registry, fmt.Errorf("clispec: schema is %q, want %q", registry.Schema, Schema)
	}
	if len(registry.Commands) == 0 {
		return registry, fmt.Errorf("clispec: no commands")
	}
	return registry, nil
}

// Must returns the registry or panics.
//
// The registry is compiled into the binary, so a failure here is a build that
// should not have been produced rather than anything a user can cause.
func Must() Registry {
	registry, err := Load()
	if err != nil {
		panic(err)
	}
	return registry
}

// Name is the command as a user types it, without the program name.
func (c Command) Name() string { return strings.Join(c.Path, " ") }

// Form is the whole usage line, as the documentation shows it.
func (c Command) Form(program string) string {
	parts := append([]string{program}, c.Path...)
	line := strings.Join(parts, " ")
	if strings.TrimSpace(c.Usage) != "" {
		line += " " + c.Usage
	}
	return line
}

// UsageForms is the command line's surface inventory: one line per command a
// user may run, in a stable order.
//
// This is what internal/docgen reads. It reports offered commands only,
// because an exempt command is not part of the surface a reader is told about
// and counting it would make the coverage gate demand documentation for
// something deliberately undocumented.
func (r Registry) UsageForms() []string {
	forms := make([]string, 0, len(r.Commands))
	for _, command := range r.Offered() {
		forms = append(forms, command.Form(r.Program))
	}
	return forms
}

// Offered is every command a user may run, in path order.
func (r Registry) Offered() []Command {
	offered := make([]Command, 0, len(r.Commands))
	for _, command := range r.Commands {
		if command.Exempt == "" {
			offered = append(offered, command)
		}
	}
	sort.Slice(offered, func(i, j int) bool {
		return offered[i].Name() < offered[j].Name()
	})
	return offered
}

// Lookup finds the command at an exact path.
func (r Registry) Lookup(path ...string) (Command, bool) {
	want := strings.Join(path, " ")
	for _, command := range r.Commands {
		if command.Name() == want {
			return command, true
		}
	}
	return Command{}, false
}

// Group returns a group's declaration.
func (r Registry) Group(name string) (Group, bool) {
	for _, group := range r.Groups {
		if group.Name == name {
			return group, true
		}
	}
	return Group{}, false
}

// Children is every offered command directly under a path.
func (r Registry) Children(path ...string) []Command {
	children := []Command{}
	for _, command := range r.Offered() {
		if len(command.Path) != len(path)+1 {
			continue
		}
		if strings.Join(command.Path[:len(path)], " ") == strings.Join(path, " ") {
			children = append(children, command)
		}
	}
	return children
}

// GroupNames is every path that holds subcommands rather than doing something.
func (r Registry) GroupNames() []string {
	names := map[string]bool{}
	for _, command := range r.Commands {
		if len(command.Path) > 1 {
			names[command.Path[0]] = true
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	return ordered
}
