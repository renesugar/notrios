package go_probe

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGoDirectiveRetentionAndTextRemoval(t *testing.T) {
	source := `package probe
// Export explains the operation.
//notrios:doc user export
//notrios:help archive writing
func Export() {}
`
	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	decl := file.Decls[0].(*ast.FuncDecl)
	if decl.Doc == nil || len(decl.Doc.List) != 3 {
		t.Fatalf("raw comment list = %#v", decl.Doc)
	}
	if got := decl.Doc.Text(); got != "Export explains the operation.\n" {
		t.Fatalf("CommentGroup.Text() = %q", got)
	}
	if !strings.Contains(decl.Doc.List[1].Text, "notrios:doc") {
		t.Fatalf("directive missing from CommentGroup.List: %#v", decl.Doc.List)
	}
}

func TestSpacingAndCaseAreOrdinaryComments(t *testing.T) {
	source := `package probe
// Value explains the declaration.
// notrios:doc user spaced
//Notrios:doc user uppercase
var Value = 1
`
	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	decl := file.Decls[0].(*ast.GenDecl)
	text := decl.Doc.Text()
	if !strings.Contains(text, "notrios:doc user spaced") || !strings.Contains(text, "Notrios:doc user uppercase") {
		t.Fatalf("non-directive spellings were unexpectedly removed: %q", text)
	}
}
