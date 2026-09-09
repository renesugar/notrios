package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/clispec"
)

// This file is the check that did not exist. Three descriptions of this command
// line were maintained separately -- the help text, a usage function per group,
// and each command's flags -- and nothing compared any of them to the
// dispatcher, so eight subcommands existed and were mentioned nowhere. The
// registry is now the description, and this walks the dispatch to hold it to it.

// dispatcher is one function that routes on its first argument.
type dispatcher struct {
	// routes maps the word a user types to the function that handles it.
	routes map[string]string
	// order preserves the source order, for readable failures.
	order []string
}

// dispatchTree walks from main() and returns every command path the program
// actually accepts.
func dispatchTree(t *testing.T) map[string]bool {
	t.Helper()
	leaves, _ := dispatchWalk(t)
	return leaves
}

// dispatchHandlers maps each command path to the function that runs it, which
// is what lets a check ask what flags that command actually accepts.
func dispatchHandlers(t *testing.T) map[string]string {
	t.Helper()
	_, handlers := dispatchWalk(t)
	return handlers
}

func dispatchWalk(t *testing.T) (map[string]bool, map[string]string) {
	t.Helper()

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the command directory: %v", err)
	}
	dispatchers := map[string]dispatcher{}
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
			if routed := routesOf(function.Body); len(routed.routes) > 0 {
				dispatchers[function.Name.Name] = routed
			}
		}
	}
	if _, ok := dispatchers["main"]; !ok {
		t.Fatal("main() does not route on a first argument; this test no longer describes the program")
	}

	leaves := map[string]bool{}
	handlers := map[string]string{}
	var walk func(function string, path []string, depth int)
	walk = func(function string, path []string, depth int) {
		if depth > 4 {
			t.Fatalf("dispatch nested deeper than four words at %q", strings.Join(path, " "))
		}
		routed, ok := dispatchers[function]
		if !ok {
			if len(path) > 0 {
				leaves[strings.Join(path, " ")] = true
				handlers[strings.Join(path, " ")] = function
			}
			return
		}
		for _, word := range routed.order {
			if isHelpWord(word) {
				continue
			}
			next := append(append([]string{}, path...), word)
			walk(routed.routes[word], next, depth+1)
		}
	}
	walk("main", nil, 0)
	return leaves, handlers
}

func isHelpWord(word string) bool {
	return clispec.IsHelpRequest(word)
}

// routesOf finds a function's routing switch: one whose cases are string
// literals and whose body dispatches on the caller's first argument.
func routesOf(body *ast.BlockStmt) dispatcher {
	routed := dispatcher{routes: map[string]string{}}
	ast.Inspect(body, func(node ast.Node) bool {
		statement, ok := node.(*ast.SwitchStmt)
		if !ok || statement.Tag == nil || len(routed.routes) > 0 {
			return true
		}
		if !routesOnFirstArgument(statement.Tag) {
			return true
		}
		for _, clause := range statement.Body.List {
			caseClause, ok := clause.(*ast.CaseClause)
			if !ok {
				continue
			}
			called := calledFunction(caseClause.Body)
			for _, expression := range caseClause.List {
				literal, ok := expression.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				word, err := strconv.Unquote(literal.Value)
				if err != nil {
					continue
				}
				routed.routes[word] = called
				routed.order = append(routed.order, word)
			}
		}
		return true
	})
	if len(routed.routes) == 0 {
		// A group with exactly one subcommand guards it with a comparison
		// rather than a switch -- `if args[0] != "archive-v2"`. Five of them do,
		// and a walker that only understood switches would have called each a
		// leaf and quietly agreed with a registry that disagreed with it.
		for _, word := range comparedFirstArguments(body) {
			routed.routes[word] = ""
			routed.order = append(routed.order, word)
		}
	}
	return routed
}

// comparedFirstArguments finds the subcommand names a function accepts by
// comparing its first argument against a literal.
func comparedFirstArguments(body *ast.BlockStmt) []string {
	words := []string{}
	ast.Inspect(body, func(node ast.Node) bool {
		comparison, ok := node.(*ast.BinaryExpr)
		if !ok || (comparison.Op != token.NEQ && comparison.Op != token.EQL) {
			return true
		}
		if !routesOnFirstArgument(comparison.X) {
			return true
		}
		literal, ok := comparison.Y.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		word, err := strconv.Unquote(literal.Value)
		if err == nil && word != "" {
			words = append(words, word)
		}
		return true
	})
	return words
}

// routesOnFirstArgument recognises the two spellings this program uses:
// `switch args[0]` inside a subcommand, and `switch cmd` in main.
func routesOnFirstArgument(tag ast.Expr) bool {
	switch expression := tag.(type) {
	case *ast.Ident:
		return expression.Name == "cmd"
	case *ast.IndexExpr:
		identifier, ok := expression.X.(*ast.Ident)
		if !ok || identifier.Name != "args" {
			return false
		}
		index, ok := expression.Index.(*ast.BasicLit)
		return ok && index.Value == "0"
	}
	return false
}

// calledFunction is the run* function a case hands off to, if it has one.
func calledFunction(statements []ast.Stmt) string {
	name := ""
	for _, statement := range statements {
		ast.Inspect(statement, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if !ok || !strings.HasPrefix(identifier.Name, "run") {
				return true
			}
			if name == "" {
				name = identifier.Name
			}
			return true
		})
	}
	return name
}

// TestEveryDispatchedCommandIsDescribed is the direction that failed: a command
// added to the dispatcher and to nothing else.
func TestEveryDispatchedCommandIsDescribed(t *testing.T) {
	registry := clispec.Must()
	described := map[string]bool{}
	for _, command := range registry.Commands {
		described[command.Name()] = true
	}
	missing := []string{}
	for command := range dispatchTree(t) {
		if !described[command] {
			missing = append(missing, command)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d commands are dispatched and described nowhere: %v\n"+
			"add them to internal/clispec/commands.json, or mark one exempt with the reason it is not for users",
			len(missing), missing)
	}
}

// TestEveryDescribedCommandIsDispatched is the other direction: help that
// promises something the program will not do.
func TestEveryDescribedCommandIsDispatched(t *testing.T) {
	dispatched := dispatchTree(t)
	registry := clispec.Must()
	phantom := []string{}
	for _, command := range registry.Commands {
		if !dispatched[command.Name()] {
			phantom = append(phantom, command.Name())
		}
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Errorf("%d described commands are not dispatched: %v", len(phantom), phantom)
	}
}
