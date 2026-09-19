package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/clispec"
)

// The declaration a recipe carries beside its steps: the CLI commands and REST
// routes it drives (v1.0 J15).
//
// A fence cannot fail when a command it documents stops existing. This can:
// every name a recipe declares is checked against the surface that defines it,
// and against the recipe's own steps, so the declaration cannot drift from the
// text it sits beside in either direction.
//
// Checked against what the program is, not a second list of what it should be:
// commands come from internal/clispec, which the CLI's own tests hold to the
// dispatch tree, and routes from the mux registrations in internal/httpapi.

// route matches a registered route: s.mux.HandleFunc("GET /api/v1/documents", …)
var routePattern = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+ [^"]+)"`)

// pathParameter matches a route's {placeholder}, which a published example
// replaces with a real value or a shell variable.
var pathParameter = regexp.MustCompile(`\{[^}]+\}`)

// surfaces are the CLI commands and REST routes this build actually has.
type surfaces struct {
	commands map[string]bool
	routes   []string
}

func loadSurfaces(root string) (surfaces, error) {
	found := surfaces{commands: map[string]bool{}}
	registry, err := clispec.Load()
	if err != nil {
		return found, fmt.Errorf("read the CLI spec: %w", err)
	}
	for _, command := range registry.Commands {
		found.commands[command.Name()] = true
	}

	entries, err := os.ReadDir(filepath.Join(root, "internal/httpapi"))
	if err != nil {
		return found, fmt.Errorf("read the HTTP API package: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, "internal/httpapi", name))
		if err != nil {
			return found, err
		}
		for _, match := range routePattern.FindAllStringSubmatch(string(body), -1) {
			found.routes = append(found.routes, match[1])
		}
	}
	sort.Strings(found.routes)
	if len(found.commands) == 0 || len(found.routes) == 0 {
		return found, fmt.Errorf("no commands or no routes were found; this check would assert nothing")
	}
	return found, nil
}

// registers reports whether the server registers a route, allowing a published
// example to put a real value where the route has a {placeholder}.
func (s surfaces) registers(want string) bool {
	for _, route := range s.routes {
		if route == want {
			return true
		}
	}
	return false
}

// checkUses holds a recipe to its declaration, both ways.
func (s surfaces) checkUses(id string, example Example) []error {
	problems := []error{}
	text := example.stepText()

	for _, command := range example.Uses.CLI {
		name := strings.TrimSpace(command)
		if !s.commands[name] {
			problems = append(problems, fmt.Errorf("%s declares the CLI command %q, which is not in the CLI spec", id, name))
			continue
		}
		if !strings.Contains(text, "notriosctl "+name) {
			problems = append(problems, fmt.Errorf("%s declares the CLI command %q and never runs it", id, name))
		}
	}

	for _, route := range example.Uses.REST {
		route = strings.TrimSpace(route)
		method, path, found := strings.Cut(route, " ")
		if !found {
			problems = append(problems, fmt.Errorf("%s declares the route %q, which is not \"METHOD /path\"", id, route))
			continue
		}
		if !s.registers(route) {
			problems = append(problems, fmt.Errorf("%s declares the route %q, which the server does not register", id, route))
			continue
		}
		if !mentions(text, method, path) {
			problems = append(problems, fmt.Errorf("%s declares the route %q and never calls it", id, route))
		}
	}
	return problems
}

// stepText is a recipe's rendered lines, which is what the declaration is
// checked against.
func (e Example) stepText() string {
	lines := make([]string, 0, len(e.Steps))
	for _, step := range e.Steps {
		lines = append(lines, step.Run+step.Comment)
	}
	return strings.Join(lines, "\n")
}

// mentions reports whether a recipe calls a route. A published example writes
// the path with values where the route has placeholders and may omit a GET, so
// the check is on the literal parts of the path plus, for anything other than
// GET, the method.
func mentions(text, method, path string) bool {
	for _, literal := range pathParameter.Split(path, -1) {
		literal = strings.TrimSuffix(literal, "/")
		if literal == "" {
			continue
		}
		if !strings.Contains(text, literal) {
			return false
		}
	}
	if method == "GET" {
		return true
	}
	return strings.Contains(text, "-X "+method) || strings.Contains(text, "--request "+method)
}
