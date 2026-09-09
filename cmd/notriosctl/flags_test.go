package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/clispec"
)

// H23 gave the command line one description of which commands exist and stopped
// short of their flags: internal/clispec describes them as a usage string,
// flag.FlagSet declares them separately, and nothing compared the two. Writing
// that registry produced `sync retire --key <id>` for a command that takes
// `--peer <replica-id>`, and missed `notes edit --message`, both caught only
// because a person read a generated diff. This closes it.

// describedFlags are the long flags a command's usage string names.
var describedFlagRE = regexp.MustCompile(`--([a-z0-9][a-z0-9-]*)`)

func describedFlags(usage string) map[string]bool {
	named := map[string]bool{}
	for _, match := range describedFlagRE.FindAllStringSubmatch(usage, -1) {
		named[match[1]] = true
	}
	return named
}

// flagDeclarations returns, per function in cmd/notriosctl, the flags it
// declares and the functions it calls.
//
// The calls matter because a command's flags are not always in its own body:
// the sync commands take theirs from newSyncFlags, the note commands from
// newDocumentFlags. A check that read only the function itself would report
// every sync command as accepting nothing and be quietly useless.
func flagDeclarations(t *testing.T) (map[string][]string, map[string][]string) {
	t.Helper()

	declared := map[string][]string{}
	calls := map[string][]string{}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the command directory: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			flags, called := flagsIn(function.Body)
			declared[function.Name.Name] = flags
			calls[function.Name.Name] = called
		}
	}
	return declared, calls
}

// flagsIn collects the flag names a body declares and the functions it calls.
func flagsIn(body *ast.BlockStmt) ([]string, []string) {
	flags := []string{}
	called := []string{}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if identifier, ok := call.Fun.(*ast.Ident); ok {
			called = append(called, identifier.Name)
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// `fs.String("name", ...)` names the flag first; `fs.Var(value,
		// "name", ...)` names it second, which a first-argument reader gets
		// silently wrong -- it did, on `templates create --set`.
		at := -1
		switch selector.Sel.Name {
		case "String", "Bool", "Int", "Int64", "Uint", "Uint64", "Float64", "Duration":
			at = 0
		case "Var", "TextVar", "Func", "BoolFunc":
			at = 1
		}
		if at < 0 || at >= len(call.Args) {
			return true
		}
		literal, ok := call.Args[at].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		if value, err := strconv.Unquote(literal.Value); err == nil && value != "" {
			flags = append(flags, value)
		}
		return true
	})
	return flags, called
}

// acceptedFlags is every flag a command takes, following the helpers it calls.
func acceptedFlags(function string, declared, calls map[string][]string, seen map[string]bool) map[string]bool {
	accepted := map[string]bool{}
	if function == "" || seen[function] {
		return accepted
	}
	seen[function] = true
	for _, flag := range declared[function] {
		accepted[flag] = true
	}
	for _, called := range calls[function] {
		// Only follow functions in this package that declare flags themselves;
		// anything else is ordinary work.
		if _, ours := declared[called]; !ours {
			continue
		}
		for flag := range acceptedFlags(called, declared, calls, seen) {
			accepted[flag] = true
		}
	}
	return accepted
}

// commonFor is the set a command need not repeat: `--config`, `--db` and
// `--asset-store` are on nearly every command, and naming them in eighty-five
// usage strings would make every one of them unreadable.
func commonFor(registry clispec.Registry, path []string) map[string]bool {
	common := map[string]bool{}
	for prefix, flags := range registry.CommonFlags {
		if prefix != "" && !strings.HasPrefix(strings.Join(path, " ")+" ", prefix+" ") {
			continue
		}
		for _, flag := range flags {
			common[flag] = true
		}
	}
	return common
}

// TestNoCommandDescribesAFlagItDoesNotAccept is the direction that produced a
// documented `sync retire --key` for a command taking `--peer`.
func TestNoCommandDescribesAFlagItDoesNotAccept(t *testing.T) {
	registry := clispec.Must()
	handlers := dispatchHandlers(t)
	declared, calls := flagDeclarations(t)

	problems := []string{}
	for _, command := range registry.Commands {
		handler := handlers[command.Name()]
		if handler == "" {
			continue
		}
		accepted := acceptedFlags(handler, declared, calls, map[string]bool{})
		for flag := range describedFlags(command.Usage) {
			if !accepted[flag] {
				problems = append(problems, command.Name()+" describes --"+flag+" and does not accept it")
			}
		}
	}
	sort.Strings(problems)
	for _, problem := range problems {
		t.Errorf("%s", problem)
	}
}

// TestNoCommandHidesAFlagItAccepts is the other direction: a flag that works
// and that `--help` never mentions, which is how eight whole commands went
// unmentioned before H23.
func TestNoCommandHidesAFlagItAccepts(t *testing.T) {
	registry := clispec.Must()
	handlers := dispatchHandlers(t)
	declared, calls := flagDeclarations(t)

	problems := []string{}
	for _, command := range registry.Commands {
		handler := handlers[command.Name()]
		if handler == "" {
			continue
		}
		common := commonFor(registry, command.Path)
		described := describedFlags(command.Usage)
		hidden := []string{}
		for flag := range acceptedFlags(handler, declared, calls, map[string]bool{}) {
			if described[flag] || common[flag] {
				continue
			}
			if _, declaredIgnored := command.AcceptsIgnored[flag]; declaredIgnored {
				continue
			}
			hidden = append(hidden, "--"+flag)
		}
		if len(hidden) > 0 {
			sort.Strings(hidden)
			problems = append(problems, command.Name()+" accepts "+strings.Join(hidden, ", ")+" and describes none of them")
		}
	}
	sort.Strings(problems)
	for _, problem := range problems {
		t.Errorf("%s", problem)
	}
}

// TestCommonFlagsAreActuallyCommon keeps the exemption honest. A flag excused
// as common on every command is an exemption; a flag excused as common that
// only one command takes is a hiding place.
func TestCommonFlagsAreActuallyCommon(t *testing.T) {
	registry := clispec.Must()
	handlers := dispatchHandlers(t)
	declared, calls := flagDeclarations(t)

	for prefix, flags := range registry.CommonFlags {
		for _, flag := range flags {
			takers := 0
			for _, command := range registry.Commands {
				handler := handlers[command.Name()]
				if handler == "" {
					continue
				}
				if prefix != "" && !strings.HasPrefix(command.Name()+" ", prefix+" ") {
					continue
				}
				if acceptedFlags(handler, declared, calls, map[string]bool{})[flag] {
					takers++
				}
			}
			if takers < 2 {
				t.Errorf("--%s is excused as common under %q and only %d command takes it",
					flag, prefix, takers)
			}
		}
	}
}

// TestAnIgnoredFlagIsDeclaredWithAReason keeps the last exemption from becoming
// a place to put anything awkward.
func TestAnIgnoredFlagIsDeclaredWithAReason(t *testing.T) {
	registry := clispec.Must()
	handlers := dispatchHandlers(t)
	declared, calls := flagDeclarations(t)

	for _, command := range registry.Commands {
		for flag, reason := range command.AcceptsIgnored {
			if strings.TrimSpace(reason) == "" {
				t.Errorf("%s ignores --%s without saying why", command.Name(), flag)
			}
			handler := handlers[command.Name()]
			if handler == "" {
				continue
			}
			if !acceptedFlags(handler, declared, calls, map[string]bool{})[flag] {
				t.Errorf("%s declares --%s ignored and does not accept it at all", command.Name(), flag)
			}
		}
	}
}
