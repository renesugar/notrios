package docaudit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const modulePath = "github.com/renesugar/notrios"

var (
	directiveLine = regexp.MustCompile(`^//notrios:(doc|help|enumerates|claim)(?:\s+(.+))?$`)
	idPattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
)

type parsedDirective struct {
	kind string
	args []string
}

func scanGoFragments(root string) ([]Fragment, error) {
	var fragments []Fragment
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == "dist" || name == "_site" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		importPath := modulePath
		if dir := filepath.ToSlash(filepath.Dir(rel)); dir != "." {
			importPath += "/" + dir
		}
		usedGroups := make(map[*ast.CommentGroup]bool)
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if groupHasNotrios(value.Doc) {
					usedGroups[value.Doc] = true
				}
				fragment, ok, err := fragmentFromGroup(value.Doc, goFunctionSymbol(value), importPath)
				if err != nil {
					return fmt.Errorf("%s: %w", rel, err)
				}
				if ok {
					fragments = append(fragments, fragment)
				}
			case *ast.GenDecl:
				groups := make([]*ast.CommentGroup, 0, 2)
				if groupHasNotrios(value.Doc) {
					groups = append(groups, value.Doc)
				}
				if len(value.Specs) == 1 && groupHasNotrios(specGroup(value.Specs[0])) {
					groups = append(groups, specGroup(value.Specs[0]))
				}
				if len(groups) > 1 {
					return fmt.Errorf("%s: declaration has multiple notrios directive groups", rel)
				}
				for _, group := range groups {
					usedGroups[group] = true
					name := ""
					if len(value.Specs) == 1 {
						name = specName(value.Specs[0])
					}
					fragment, ok, err := fragmentFromGroup(group, name, importPath)
					if err != nil {
						return fmt.Errorf("%s: %w", rel, err)
					}
					if ok {
						fragments = append(fragments, fragment)
					}
				}
			}
		}
		for _, group := range file.Comments {
			if groupHasNotrios(group) && !usedGroups[group] {
				return fmt.Errorf("%s: notrios directive group is not attached to a named declaration", rel)
			}
		}
		return nil
	})
	return fragments, err
}

func groupHasNotrios(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	for _, comment := range group.List {
		if strings.HasPrefix(comment.Text, "//notrios:") {
			return true
		}
	}
	return false
}

func fragmentFromGroup(group *ast.CommentGroup, symbol, importPath string) (Fragment, bool, error) {
	if group == nil {
		return Fragment{}, false, nil
	}
	var directives []parsedDirective
	for _, comment := range group.List {
		if strings.HasPrefix(comment.Text, "//notrios:") && !directiveLine.MatchString(comment.Text) {
			return Fragment{}, false, fmt.Errorf("invalid notrios directive %q", comment.Text)
		}
		match := directiveLine.FindStringSubmatch(comment.Text)
		if match == nil {
			continue
		}
		directives = append(directives, parsedDirective{kind: match[1], args: strings.Fields(match[2])})
	}
	if len(directives) == 0 {
		return Fragment{}, false, nil
	}
	if symbol == "" {
		return Fragment{}, false, fmt.Errorf("directive group is not attached to a named declaration")
	}
	fragment := Fragment{Anchor: "go:" + importPath + "#" + symbol, Prose: strings.TrimSpace(group.Text())}
	docCount := 0
	for _, directive := range directives {
		switch directive.kind {
		case "doc":
			docCount++
			if len(directive.args) != 2 {
				return Fragment{}, false, fmt.Errorf("doc directive requires audience and fragment id")
			}
			fragment.Audience, fragment.ID = directive.args[0], directive.args[1]
		case "help":
			if len(directive.args) != 2 {
				return Fragment{}, false, fmt.Errorf("help directive requires topic and section")
			}
			fragment.Topic, fragment.Section = directive.args[0], directive.args[1]
		case "enumerates":
			if len(directive.args) != 1 || fragment.Enumeration != "" {
				return Fragment{}, false, fmt.Errorf("enumerates directive requires exactly one anchor")
			}
			fragment.Enumeration = directive.args[0]
		case "claim":
			if len(directive.args) != 2 || fragment.ClaimID != "" {
				return Fragment{}, false, fmt.Errorf("claim directive requires exactly one id and check anchor")
			}
			fragment.ClaimID, fragment.ClaimCheck = directive.args[0], directive.args[1]
		}
	}
	if docCount != 1 {
		return Fragment{}, false, fmt.Errorf("directive group requires exactly one doc directive; mixed user/API blocks are forbidden")
	}
	if fragment.Audience != "user" && fragment.Audience != "api" && fragment.Audience != "maintainer" {
		return Fragment{}, false, fmt.Errorf("unknown audience %q", fragment.Audience)
	}
	if !idPattern.MatchString(fragment.ID) {
		return Fragment{}, false, fmt.Errorf("invalid fragment id %q", fragment.ID)
	}
	if fragment.Topic != "" && fragment.Audience != "user" {
		return Fragment{}, false, fmt.Errorf("help directive requires user audience")
	}
	if fragment.Prose == "" {
		return Fragment{}, false, fmt.Errorf("fragment %q has no ordinary prose", fragment.ID)
	}
	return fragment, true, nil
}

func goFunctionSymbol(declaration *ast.FuncDecl) string {
	if declaration.Recv == nil || len(declaration.Recv.List) != 1 {
		return declaration.Name.Name
	}
	receiver := declaration.Recv.List[0].Type
	prefix := ""
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		prefix = "*"
		receiver = pointer.X
	}
	identifier, ok := receiver.(*ast.Ident)
	if !ok {
		return ""
	}
	return "(" + prefix + identifier.Name + ")." + declaration.Name.Name
}

func specGroup(spec ast.Spec) *ast.CommentGroup {
	switch value := spec.(type) {
	case *ast.TypeSpec:
		return value.Doc
	case *ast.ValueSpec:
		return value.Doc
	default:
		return nil
	}
}

func specName(spec ast.Spec) string {
	switch value := spec.(type) {
	case *ast.TypeSpec:
		return value.Name.Name
	case *ast.ValueSpec:
		if len(value.Names) == 1 {
			return value.Names[0].Name
		}
	}
	return ""
}

func resolveGoAnchor(root, anchor string, includeTests bool) error {
	const prefix = "go:"
	if !strings.HasPrefix(anchor, prefix) {
		return fmt.Errorf("malformed Go anchor %q", anchor)
	}
	parts := strings.SplitN(strings.TrimPrefix(anchor, prefix), "#", 2)
	if len(parts) != 2 || parts[1] == "" || (parts[0] != modulePath && !strings.HasPrefix(parts[0], modulePath+"/")) {
		return fmt.Errorf("malformed or external Go anchor %q", anchor)
	}
	rel := strings.TrimPrefix(parts[0], modulePath)
	rel = strings.TrimPrefix(rel, "/")
	directory := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := filepath.Glob(filepath.Join(directory, "*.go"))
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("missing Go package for %q", anchor)
	}
	matches := 0
	for _, path := range entries {
		if !includeTests && strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if goFunctionSymbol(value) == parts[1] {
					matches++
				}
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					if specName(spec) == parts[1] {
						matches++
					}
				}
			}
		}
	}
	if matches != 1 {
		return fmt.Errorf("Go anchor %q resolved %d declarations, want exactly one", anchor, matches)
	}
	return nil
}

func splitAnchorsByLanguage(anchors []string) (goAnchors, tsAnchors []string, err error) {
	seen := make(map[string]bool)
	for _, anchor := range anchors {
		if seen[anchor] {
			continue
		}
		seen[anchor] = true
		switch {
		case strings.HasPrefix(anchor, "go:"):
			goAnchors = append(goAnchors, anchor)
		case strings.HasPrefix(anchor, "ts:"):
			tsAnchors = append(tsAnchors, anchor)
		default:
			return nil, nil, fmt.Errorf("unknown source-symbol anchor kind %q", anchor)
		}
	}
	sort.Strings(goAnchors)
	sort.Strings(tsAnchors)
	return goAnchors, tsAnchors, nil
}
