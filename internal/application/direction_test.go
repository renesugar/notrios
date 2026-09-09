package application

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// Repository is the single declaration H0 permits to name store types: it is
// the transitional persistence seam. Everything else the package exports must
// be expressible without the store, or the facade is not transport-neutral and
// the C ABI cannot be built on it.
var storeSeamAllowlist = map[string]bool{"Repository": true}

// forbiddenImports are the dependencies whose presence would mean the facade
// had acquired a transport, a serialization format, or a filesystem.
var forbiddenImports = []string{
	"net/http",
	"encoding/json",
	"database/sql",
	"path/filepath",
	"github.com/renesugar/notrios/internal/httpapi",
	"github.com/renesugar/notrios/internal/service",
}

func parsePackage(t *testing.T) (*token.FileSet, []*ast.File) {
	t.Helper()
	fileSet := token.NewFileSet()
	packages, err := parser.ParseDir(fileSet, ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}
	pkg, ok := packages["application"]
	if !ok {
		t.Fatal("package application not found")
	}
	files := make([]*ast.File, 0, len(pkg.Files))
	for _, file := range pkg.Files {
		files = append(files, file)
	}
	return fileSet, files
}

func TestPackageDoesNotImportTransportOrSQL(t *testing.T) {
	_, files := parsePackage(t)
	for _, file := range files {
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			for _, forbidden := range forbiddenImports {
				if path == forbidden {
					t.Errorf("application imports %q; the facade must stay transport- and storage-neutral", path)
				}
			}
		}
	}
}

// TestExportedSurfaceIsTransportAndStorageNeutral is the mechanical form of
// H0's rule that the facade's exported operation contract must not expose the
// store. It walks every exported declaration and fails if a store type appears
// in a signature, a field, or a result, except in the documented seam.
//
// The point is that the exception cannot widen silently. Adding a second
// store-typed export requires editing storeSeamAllowlist, which is a visible
// decision in review rather than an accident.
func TestExportedSurfaceIsTransportAndStorageNeutral(t *testing.T) {
	fileSet, files := parsePackage(t)

	report := func(node ast.Node, what string) {
		t.Errorf("%s: exported surface %s names a store type; keep store types behind the facade",
			fileSet.Position(node.Pos()), what)
	}

	namesStore := func(node ast.Node) bool {
		found := false
		ast.Inspect(node, func(n ast.Node) bool {
			selector, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == "store" {
				found = true
				return false
			}
			return true
		})
		return found
	}

	for _, file := range files {
		for _, decl := range file.Decls {
			switch typed := decl.(type) {
			case *ast.FuncDecl:
				if !typed.Name.IsExported() {
					continue
				}
				// A method on an unexported type is not part of the surface.
				if typed.Recv != nil && !receiverIsExported(typed.Recv) {
					continue
				}
				if storeSeamAllowlist[typed.Name.Name] {
					continue
				}
				if namesStore(typed.Type) {
					report(typed, "func "+typed.Name.Name)
				}
			case *ast.GenDecl:
				if typed.Tok != token.TYPE {
					continue
				}
				for _, spec := range typed.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || !typeSpec.Name.IsExported() {
						continue
					}
					if storeSeamAllowlist[typeSpec.Name.Name] {
						continue
					}
					if namesStore(typeSpec.Type) {
						report(typeSpec, "type "+typeSpec.Name.Name)
					}
				}
			}
		}
	}
}

func receiverIsExported(fields *ast.FieldList) bool {
	if fields == nil || len(fields.List) == 0 {
		return false
	}
	expr := fields.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.IsExported()
}

// TestResultTypesCarryNoSerializationTags keeps a wire format from leaking in
// through the back door. store.Resource has JSON tags; the facade's Resource
// deliberately does not, because one adapter's field names must not become the
// contract every other adapter inherits.
func TestResultTypesCarryNoSerializationTags(t *testing.T) {
	fileSet, files := parsePackage(t)
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			structType, ok := n.(*ast.StructType)
			if !ok || structType.Fields == nil {
				return true
			}
			for _, field := range structType.Fields.List {
				if field.Tag != nil {
					t.Errorf("%s: struct field carries tag %s; serialization belongs to adapters",
						fileSet.Position(field.Pos()), field.Tag.Value)
				}
			}
			return true
		})
	}
}
