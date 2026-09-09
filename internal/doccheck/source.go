package doccheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	modulePath         = "github.com/renesugar/notrios"
	maxDirectCallees   = 8
	maxSourceSliceByte = 64 << 10
)

// SourceSlice is the claim-blind declaration view given to the advisory
// reviewer. Source-adjacent declaration comments are deliberately omitted.
type SourceSlice struct {
	Root            string   `json:"root"`
	DirectCallees   []string `json:"direct_callees"`
	Source          string   `json:"source"`
	TruncatedCallee bool     `json:"truncated_callees"`
}

// ResolveSourceSlice resolves one frozen G18a anchor plus a bounded depth-one
// view. Explicit callees are used by the human-labelled calibration cases;
// otherwise statically named same-package Go calls are discovered.
func ResolveSourceSlice(ctx context.Context, root, anchor string, explicit []string) (SourceSlice, error) {
	switch {
	case strings.HasPrefix(anchor, "go:"):
		return resolveGoSourceSlice(root, anchor, explicit)
	case strings.HasPrefix(anchor, "ts:"):
		return resolveTypeScriptSourceSlice(ctx, root, anchor, explicit)
	default:
		return SourceSlice{}, fmt.Errorf("unknown source anchor %q", anchor)
	}
}

type goDeclaration struct {
	anchor  string
	symbol  string
	node    ast.Decl
	fset    *token.FileSet
	imports map[string]bool
}

func resolveGoSourceSlice(root, anchor string, explicit []string) (SourceSlice, error) {
	pkg, symbol, err := parseGoAnchor(anchor)
	if err != nil {
		return SourceSlice{}, err
	}
	declarations, err := loadGoPackage(root, pkg)
	if err != nil {
		return SourceSlice{}, err
	}
	rootDecl, ok := declarations[symbol]
	if !ok {
		return SourceSlice{}, fmt.Errorf("Go source anchor %q did not resolve exactly one declaration", anchor)
	}

	calleeSymbols := explicit
	truncated := false
	if len(calleeSymbols) == 0 {
		calleeSymbols = directGoCallees(rootDecl, declarations)
	}
	if len(calleeSymbols) > maxDirectCallees {
		calleeSymbols = calleeSymbols[:maxDirectCallees]
		truncated = true
	}

	callees := make([]goDeclaration, 0, len(calleeSymbols))
	for _, value := range calleeSymbols {
		calleePkg, calleeSymbol, err := parseGoAnchorOrSymbol(pkg, value)
		if err != nil {
			return SourceSlice{}, err
		}
		if calleePkg != pkg {
			return SourceSlice{}, fmt.Errorf("direct callee %q is outside root package %q", value, pkg)
		}
		decl, ok := declarations[calleeSymbol]
		if !ok {
			return SourceSlice{}, fmt.Errorf("direct callee %q did not resolve exactly one declaration", value)
		}
		if decl.symbol == rootDecl.symbol {
			continue
		}
		callees = append(callees, decl)
	}
	sort.Slice(callees, func(i, j int) bool { return callees[i].anchor < callees[j].anchor })

	var out strings.Builder
	writeDeclaration := func(label string, declaration goDeclaration) error {
		code, err := printDeclaration(declaration)
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "%s %s\n%s\n", label, declaration.anchor, code)
		return nil
	}
	if err := writeDeclaration("ROOT", rootDecl); err != nil {
		return SourceSlice{}, err
	}
	included := make([]string, 0, len(callees))
	for _, declaration := range callees {
		if err := writeDeclaration("DIRECT_CALLEE", declaration); err != nil {
			return SourceSlice{}, err
		}
		included = append(included, declaration.anchor)
	}
	if out.Len() > maxSourceSliceByte {
		return SourceSlice{}, fmt.Errorf("source slice for %q is %d bytes, limit %d", anchor, out.Len(), maxSourceSliceByte)
	}
	return SourceSlice{Root: anchor, DirectCallees: included, Source: out.String(), TruncatedCallee: truncated}, nil
}

func parseGoAnchor(anchor string) (string, string, error) {
	if !strings.HasPrefix(anchor, "go:") {
		return "", "", fmt.Errorf("malformed Go anchor %q", anchor)
	}
	parts := strings.SplitN(strings.TrimPrefix(anchor, "go:"), "#", 2)
	if len(parts) != 2 || parts[1] == "" || (parts[0] != modulePath && !strings.HasPrefix(parts[0], modulePath+"/")) {
		return "", "", fmt.Errorf("malformed or external Go anchor %q", anchor)
	}
	return parts[0], parts[1], nil
}

func parseGoAnchorOrSymbol(pkg, value string) (string, string, error) {
	if strings.HasPrefix(value, "go:") {
		return parseGoAnchor(value)
	}
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, " \t\r\n#") {
		return "", "", fmt.Errorf("malformed direct callee %q", value)
	}
	return pkg, value, nil
}

func loadGoPackage(root, pkg string) (map[string]goDeclaration, error) {
	rel := strings.TrimPrefix(pkg, modulePath)
	rel = strings.TrimPrefix(rel, "/")
	directory := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := filepath.Glob(filepath.Join(directory, "*.go"))
	if err != nil || len(entries) == 0 {
		return nil, fmt.Errorf("missing Go package %q", pkg)
	}
	declarations := make(map[string]goDeclaration)
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		imports := make(map[string]bool)
		for _, item := range file.Imports {
			name := ""
			if item.Name != nil {
				name = item.Name.Name
			} else {
				value := strings.Trim(item.Path.Value, "\"")
				name = filepath.Base(value)
			}
			imports[name] = true
		}
		for _, declaration := range file.Decls {
			symbols := declarationSymbols(declaration)
			for _, symbol := range symbols {
				if _, exists := declarations[symbol]; exists {
					return nil, fmt.Errorf("ambiguous Go declaration %s#%s", pkg, symbol)
				}
				declarations[symbol] = goDeclaration{
					anchor: "go:" + pkg + "#" + symbol, symbol: symbol,
					node: declaration, fset: fset, imports: imports,
				}
			}
		}
	}
	return declarations, nil
}

func declarationSymbols(declaration ast.Decl) []string {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		return []string{functionSymbol(value)}
	case *ast.GenDecl:
		var symbols []string
		for _, spec := range value.Specs {
			switch item := spec.(type) {
			case *ast.TypeSpec:
				symbols = append(symbols, item.Name.Name)
			case *ast.ValueSpec:
				for _, name := range item.Names {
					if name.Name != "_" {
						symbols = append(symbols, name.Name)
					}
				}
			}
		}
		return symbols
	}
	return nil
}

func functionSymbol(declaration *ast.FuncDecl) string {
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
		return declaration.Name.Name
	}
	return "(" + prefix + identifier.Name + ")." + declaration.Name.Name
}

func directGoCallees(root goDeclaration, declarations map[string]goDeclaration) []string {
	methods := make(map[string][]string)
	for symbol := range declarations {
		if index := strings.LastIndex(symbol, ")."); index >= 0 {
			methods[symbol[index+2:]] = append(methods[symbol[index+2:]], symbol)
		}
	}
	seen := make(map[string]bool)
	ast.Inspect(root.node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		symbol := ""
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if _, ok := declarations[fun.Name]; ok {
				symbol = fun.Name
			}
		case *ast.SelectorExpr:
			if identifier, ok := fun.X.(*ast.Ident); ok && root.imports[identifier.Name] {
				return true
			}
			if values := methods[fun.Sel.Name]; len(values) == 1 {
				symbol = values[0]
			}
		}
		if symbol != "" && symbol != root.symbol {
			seen[symbol] = true
		}
		return true
	})
	values := make([]string, 0, len(seen))
	for symbol := range seen {
		values = append(values, symbol)
	}
	sort.Strings(values)
	return values
}

func printDeclaration(declaration goDeclaration) (string, error) {
	var node ast.Decl
	switch value := declaration.node.(type) {
	case *ast.FuncDecl:
		copyValue := *value
		copyValue.Doc = nil
		node = &copyValue
	case *ast.GenDecl:
		copyValue := *value
		copyValue.Doc = nil
		copyValue.Specs = make([]ast.Spec, 0, len(value.Specs))
		for _, spec := range value.Specs {
			switch item := spec.(type) {
			case *ast.TypeSpec:
				copyItem := *item
				copyItem.Doc, copyItem.Comment = nil, nil
				copyValue.Specs = append(copyValue.Specs, &copyItem)
			case *ast.ValueSpec:
				copyItem := *item
				copyItem.Doc, copyItem.Comment = nil, nil
				copyValue.Specs = append(copyValue.Specs, &copyItem)
			default:
				copyValue.Specs = append(copyValue.Specs, spec)
			}
		}
		node = &copyValue
	default:
		return "", fmt.Errorf("unsupported declaration for %q", declaration.anchor)
	}
	var out bytes.Buffer
	if err := printer.Fprint(&out, declaration.fset, node); err != nil {
		return "", err
	}
	return out.String(), nil
}

type typeScriptSliceResponse struct {
	Slices []struct {
		Anchor string `json:"anchor"`
		Source string `json:"source"`
	} `json:"slices"`
}

func resolveTypeScriptSourceSlice(ctx context.Context, root, anchor string, explicit []string) (SourceSlice, error) {
	anchors := append([]string{anchor}, explicit...)
	if len(anchors)-1 > maxDirectCallees {
		anchors = anchors[:maxDirectCallees+1]
	}
	arguments := []string{filepath.Join(root, "scripts", "doccheck_ts.mjs"), "--root", root}
	for _, value := range anchors {
		arguments = append(arguments, "--anchor", value)
	}
	command := exec.CommandContext(ctx, "node", arguments...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return SourceSlice{}, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return SourceSlice{}, err
	}
	limited := io.LimitReader(stdout, maxSourceSliceByte+1)
	encoded, readErr := io.ReadAll(limited)
	waitErr := command.Wait()
	if readErr != nil {
		return SourceSlice{}, readErr
	}
	if waitErr != nil {
		return SourceSlice{}, fmt.Errorf("TypeScript source resolver: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
	}
	if len(encoded) > maxSourceSliceByte {
		return SourceSlice{}, fmt.Errorf("TypeScript source resolver output exceeds %d bytes", maxSourceSliceByte)
	}
	var response typeScriptSliceResponse
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return SourceSlice{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return SourceSlice{}, fmt.Errorf("TypeScript source resolver returned trailing JSON")
	}
	if len(response.Slices) != len(anchors) {
		return SourceSlice{}, fmt.Errorf("TypeScript source resolver returned %d slices, want %d", len(response.Slices), len(anchors))
	}
	byAnchor := make(map[string]string, len(response.Slices))
	for _, item := range response.Slices {
		byAnchor[item.Anchor] = item.Source
	}
	var out strings.Builder
	for index, value := range anchors {
		source, ok := byAnchor[value]
		if !ok || strings.TrimSpace(source) == "" {
			return SourceSlice{}, fmt.Errorf("TypeScript source resolver omitted %q", value)
		}
		label := "DIRECT_CALLEE"
		if index == 0 {
			label = "ROOT"
		}
		fmt.Fprintf(&out, "%s %s\n%s\n", label, value, source)
	}
	if out.Len() > maxSourceSliceByte {
		return SourceSlice{}, fmt.Errorf("source slice for %q is %d bytes, limit %d", anchor, out.Len(), maxSourceSliceByte)
	}
	return SourceSlice{
		Root: anchor, DirectCallees: append([]string(nil), anchors[1:]...),
		Source: out.String(), TruncatedCallee: len(explicit) > maxDirectCallees,
	}, nil
}

// repositoryRoot resolves a caller-supplied root without accepting a missing
// directory. It is kept here for focused tests and command setup.
func repositoryRoot(root string) (string, error) {
	value, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(value)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("repository root %q is not a directory", value)
	}
	return value, nil
}
