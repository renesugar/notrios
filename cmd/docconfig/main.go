// Command docconfig generates the configuration key table in
// docs/configuration.md from internal/config itself.
//
// The table is derived rather than written because a hand-maintained list of 63
// keys is a list that disagrees with the code, and the README's project status
// had just finished demonstrating what that costs. Reflection over
// config.Config gives the paths and the types; config.Default() gives the
// defaults, which is the same function the service calls when nobody passes
// -config, so a default printed here is the default that applies.
//
// The prose above the table is written by a person and is not touched. This
// generator owns exactly one block.
//
//	go run ./cmd/docconfig            # check the table is current
//	go run ./cmd/docconfig --write    # rewrite it
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/renesugar/notrios/internal/config"
)

const (
	docPath = "docs/configuration.md"
	begin   = "<!-- notrios:generated:config:keys:begin -->"
	finish  = "<!-- notrios:generated:config:keys:end -->"
)

// row is one configuration key as the table shows it.
type row struct {
	path, kind, def, comment string
}

func main() {
	root := flag.String("root", ".", "repository root")
	write := flag.Bool("write", false, "rewrite the key table")
	flag.Parse()

	comments, err := fieldComments(filepath.Join(*root, "internal/config"))
	if err != nil {
		fail(err)
	}
	rows, sections := configRows(comments)

	full := filepath.Join(*root, docPath)
	rendered, err := render(full, rows, sections)
	if err != nil {
		fail(err)
	}
	current, err := os.ReadFile(full)
	if err != nil {
		fail(err)
	}
	if !*write {
		if string(current) != rendered {
			fmt.Fprintf(os.Stderr, "docconfig: the key table in %s is stale; "+
				"run `go run ./cmd/docconfig --write`\n", docPath)
			os.Exit(1)
		}
		fmt.Printf("configuration key table current: %d keys\n", len(rows))
		return
	}
	if err := os.WriteFile(full, []byte(rendered), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("configuration key table written: %d keys\n", len(rows))
}

// configRows is the walk, shared by the command and its gate so the check and
// the generator cannot disagree about what a key is.
func configRows(comments map[string]string) ([]row, int) {
	rows := []row{}
	// Sections are counted as well as leaves, because the two numbers differ
	// and a reader who has seen the other one deserves the arithmetic rather
	// than a contradiction: docs/docaudit's configuration surface counts every
	// JSON-tagged field, section names included, and this table lists only the
	// leaves somebody can set.
	sections := []string{}
	defaults := config.Default()
	walk(reflect.TypeOf(defaults), reflect.ValueOf(defaults), "", "", comments, &rows, &sections)
	return rows, len(sections)
}

// walk collects every JSON-tagged leaf, in declaration order.
//
// Declaration order rather than alphabetical: the structs are grouped by
// subject already, and sorting the table would scatter `data.state_dir` away
// from the three roots it belongs beside.
func walk(t reflect.Type, v reflect.Value, prefix, owner string, comments map[string]string,
	rows *[]row, sections *[]string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		path := tag
		if prefix != "" {
			path = prefix + "." + tag
		}
		value := v.Field(i)
		if field.Type.Kind() == reflect.Struct {
			*sections = append(*sections, path)
			walk(field.Type, value, path, field.Type.Name(), comments, rows, sections)
			continue
		}
		*rows = append(*rows, row{
			path:    path,
			kind:    field.Type.String(),
			def:     formatDefault(value),
			comment: comments[owner+"."+field.Name],
		})
	}
}

func formatDefault(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		if v.String() == "" {
			return "—"
		}
		return "`" + v.String() + "`"
	case reflect.Bool:
		return fmt.Sprintf("`%t`", v.Bool())
	case reflect.Slice:
		if v.Len() == 0 {
			return "—"
		}
		parts := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			parts = append(parts, fmt.Sprint(v.Index(i).Interface()))
		}
		return "`" + strings.Join(parts, ", ") + "`"
	default:
		return fmt.Sprintf("`%v`", v.Interface())
	}
}

// fieldComments reads the doc comment on each struct field.
//
// The comments are the only per-key prose that exists, and they are next to the
// code that uses each key rather than in a document that can disagree with it.
// A key with no comment gets an empty cell, which is visible and therefore
// fixable; inventing a description here would look like documentation and be
// nothing of the kind.
func fieldComments(directory string) (map[string]string, error) {
	fileSet := token.NewFileSet()
	packages, err := parser.ParseDir(fileSet, directory, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	collapse := regexp.MustCompile(`\s+`)
	comments := map[string]string{}
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(node ast.Node) bool {
				spec, ok := node.(*ast.TypeSpec)
				if !ok {
					return true
				}
				structType, ok := spec.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range structType.Fields.List {
					if field.Doc == nil || len(field.Names) == 0 {
						continue
					}
					text := collapse.ReplaceAllString(field.Doc.Text(), " ")
					comments[spec.Name.Name+"."+field.Names[0].Name] = strings.TrimSpace(text)
				}
				return true
			})
		}
	}
	return comments, nil
}

func render(path string, rows []row, sections int) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := []string{
		fmt.Sprintf("%d settable keys, generated from `internal/config` by "+
			"`go run ./cmd/docconfig --write`. A dash means the default is empty, which for a "+
			"path means \"work it out from the XDG roots\". The recorded configuration surface "+
			"counts %d, because it includes the %d section names that group these.",
			len(rows), len(rows)+sections, sections),
		"",
		"| Key | Type | Default | Notes |",
		"|---|---|---|---|",
	}
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("| `%s` | %s | %s | %s |",
			r.path, r.kind, r.def, r.comment))
	}
	body := string(contents)
	block := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(begin) + `.*?` + regexp.QuoteMeta(finish))
	if !block.MatchString(body) {
		return "", fmt.Errorf("%s has no generated markers for the key table", filepath.Base(path))
	}
	return block.ReplaceAllLiteralString(body,
		begin+"\n"+strings.Join(lines, "\n")+"\n"+finish), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "docconfig:", err)
	os.Exit(1)
}
