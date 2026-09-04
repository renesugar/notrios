package docgen

import (
	"encoding/json"
	"fmt"
	"github.com/renesugar/notrios/internal/doccompare"
	"github.com/renesugar/notrios/internal/docfeatures"
	"github.com/renesugar/notrios/internal/docjourneys"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/docjourney"
)

const (
	configAnchor       = "go:github.com/renesugar/notrios/internal/config#Config"
	defaultAnchor      = "go:github.com/renesugar/notrios/internal/config#Default"
	cliHelpAnchor      = "go:github.com/renesugar/notrios/cmd/notriosctl#printHelp"
	restAnchor         = "go:github.com/renesugar/notrios/internal/httpapi#NewServerWithOptions"
	mcpToolsAnchor     = "go:github.com/renesugar/notrios/internal/httpapi#(*Server).mcpTools"
	mcpSyncScopeAnchor = "go:github.com/renesugar/notrios/internal/httpapi#mcpSyncToolScopes"
	mcpScopesAnchor    = "go:github.com/renesugar/notrios/internal/httpapi#MCPScopes"
	mcpToolScopeAnchor = "go:github.com/renesugar/notrios/internal/httpapi#mcpToolScopes"
	guiJourneyAnchor   = "go:github.com/renesugar/notrios/internal/docjourney#LoadAndValidate"
	featureAnchor      = "go:github.com/renesugar/notrios/internal/docfeatures#Registry"
	// Distinct from featureAnchor on purpose: the dispatch below is keyed by
	// the enumerates anchor, so two fragments that name the same symbol render
	// the same list. This one was written with #Registry and silently produced
	// the whole catalogue where it meant to produce the part of it with no
	// interface.
	guiAbsentAnchor    = "go:github.com/renesugar/notrios/internal/docfeatures#(Registry).WithoutGUILines"
	cliJourneyAnchor   = "go:github.com/renesugar/notrios/internal/docjourneys#Catalogue"
	guiCatalogueAnchor = "go:github.com/renesugar/notrios/internal/docjourneys#GUICatalogue"
	compareAnchor      = "go:github.com/renesugar/notrios/internal/doccompare#Difference"
)

// RepositoryResolver returns the closed enumeration adapters for the source
// registries approved by G18f. Unknown anchors fail instead of silently
// emitting an empty or hand-maintained list.
func RepositoryResolver(root string) Resolver {
	return func(anchor string) ([]string, error) {
		switch anchor {
		case configAnchor:
			return configKeys(), nil
		case defaultAnchor:
			return configDefaults(), nil
		case cliHelpAnchor:
			return cliUsageForms(root)
		case restAnchor:
			return restOperations(root)
		case mcpToolsAnchor:
			return mcpTools(root)
		case mcpSyncScopeAnchor:
			return sourceMapAssignments(filepath.Join(root, "internal/httpapi/mcp_scopes.go"), "mcpSyncToolScopes")
		case mcpScopesAnchor:
			return mcpScopeNames(root)
		case mcpToolScopeAnchor:
			return sourceMapAssignments(filepath.Join(root, "internal/httpapi/mcp_scopes.go"), "mcpToolScopes")
		case guiJourneyAnchor:
			return guiJourneys(root)
		case featureAnchor:
			registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
			if err != nil {
				return nil, err
			}
			return registry.Lines(), nil
		case guiAbsentAnchor:
			registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
			if err != nil {
				return nil, err
			}
			return registry.WithoutGUILines(), nil
		case cliJourneyAnchor:
			catalogue, err := docjourneys.Load(filepath.Join(root, "docs", "docjourneys", "CLI_JOURNEYS.json"))
			if err != nil {
				return nil, err
			}
			return catalogue.Lines(), nil
		case guiCatalogueAnchor:
			catalogue, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
			if err != nil {
				return nil, err
			}
			return catalogue.GUILines(), nil
		case compareAnchor:
			registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
			if err != nil {
				return nil, err
			}
			cli, err := docjourneys.Load(filepath.Join(root, "docs", "docjourneys", "CLI_JOURNEYS.json"))
			if err != nil {
				return nil, err
			}
			gui, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
			if err != nil {
				return nil, err
			}
			return doccompare.Lines(doccompare.Compare(registry, cli, gui)), nil
		default:
			return nil, fmt.Errorf("no repository enumeration adapter for %q", anchor)
		}
	}
}

func configKeys() []string {
	var values []string
	walkConfigType(reflect.TypeOf(config.Config{}), "", true, func(path string, _ reflect.Value) {
		values = append(values, path)
	}, reflect.Value{})
	sort.Strings(values)
	return values
}

func configDefaults() []string {
	value := reflect.ValueOf(config.Default())
	var values []string
	walkConfigType(value.Type(), "", false, func(path string, field reflect.Value) {
		values = append(values, path+" = "+formatConfigValue(field))
	}, value)
	sort.Strings(values)
	return values
}

func walkConfigType(value reflect.Type, prefix string, includeGroups bool, add func(string, reflect.Value), current reflect.Value) {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		fieldValue := reflect.Value{}
		if current.IsValid() {
			fieldValue = current.Field(index)
		}
		fieldType := field.Type
		if fieldType.Kind() == reflect.Struct && fieldType != reflect.TypeOf(time.Duration(0)) {
			if includeGroups {
				add(path, fieldValue)
			}
			walkConfigType(fieldType, path, includeGroups, add, fieldValue)
			continue
		}
		add(path, fieldValue)
	}
}

func formatConfigValue(value reflect.Value) string {
	if !value.IsValid() {
		return ""
	}
	if value.Type() == reflect.TypeOf(time.Duration(0)) {
		return strconv.Quote(time.Duration(value.Int()).String())
	}
	encoded, err := json.Marshal(value.Interface())
	if err != nil {
		return fmt.Sprint(value.Interface())
	}
	return string(encoded)
}

func cliUsageForms(root string) ([]string, error) {
	path := filepath.Join(root, "cmd/notriosctl/main.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "printHelp" || function.Body == nil {
			continue
		}
		var literal string
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Print" {
				return true
			}
			value, ok := call.Args[0].(*ast.BasicLit)
			if ok && value.Kind == token.STRING {
				literal, _ = strconv.Unquote(value.Value)
			}
			return true
		})
		if literal == "" {
			break
		}
		var values []string
		for _, line := range strings.Split(literal, "\n") {
			if strings.HasPrefix(line, "  notriosctl ") {
				values = append(values, strings.TrimSpace(line))
			}
		}
		return values, nil
	}
	return nil, fmt.Errorf("printHelp usage literal was not found")
}

var routePattern = regexp.MustCompile(`HandleFunc\("(GET|POST|PUT|PATCH|DELETE|HEAD) ([^"]+)"`)

func restOperations(root string) ([]string, error) {
	entries, err := filepath.Glob(filepath.Join(root, "internal/httpapi/*.go"))
	if err != nil {
		return nil, err
	}
	registered := map[string]bool{}
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, match := range routePattern.FindAllStringSubmatch(string(body), -1) {
			if match[1] != "HEAD" && match[2] != "/" {
				registered[match[1]+" "+normalizeRESTPath(match[2])] = true
			}
		}
	}
	openapi, err := openAPIOperations(filepath.Join(root, "api/openapi.yaml"))
	if err != nil {
		return nil, err
	}
	if missing := setDifference(openapi, registered); len(missing) > 0 {
		return nil, fmt.Errorf("OpenAPI operations missing from REST registry: %s", strings.Join(missing, ", "))
	}
	if missing := setDifference(registered, openapi); len(missing) > 0 {
		return nil, fmt.Errorf("REST registry operations missing from OpenAPI: %s", strings.Join(missing, ", "))
	}
	values := make([]string, 0, len(registered))
	for value := range registered {
		values = append(values, value)
	}
	sort.Strings(values)
	return values, nil
}

func normalizeRESTPath(path string) string {
	switch path {
	case "/api/v1/sync/carrier/{namespace}/{class}", "/api/v1/sync/carrier/{class}/{name}":
		return "/api/v1/sync/carrier/{segment1}/{segment2}"
	default:
		return path
	}
}

func openAPIOperations(path string) (map[string]bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	current := ""
	pathRE := regexp.MustCompile(`^  (/[^:]+):\s*$`)
	methodRE := regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)
	for _, line := range strings.Split(string(body), "\n") {
		if match := pathRE.FindStringSubmatch(line); match != nil {
			current = match[1]
			continue
		}
		if match := methodRE.FindStringSubmatch(line); match != nil && current != "" {
			result[strings.ToUpper(match[1])+" "+current] = true
		}
	}
	return result, nil
}

func setDifference(left, right map[string]bool) []string {
	var values []string
	for value := range left {
		if !right[value] {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

func mcpTools(root string) ([]string, error) {
	body, err := os.ReadFile(filepath.Join(root, "internal/httpapi/mcp.go"))
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`\bName:\s*"([a-z][a-z0-9_]*)"`)
	seen := map[string]bool{}
	for _, match := range re.FindAllStringSubmatch(string(body), -1) {
		seen[match[1]] = true
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values, nil
}

func sourceMapAssignments(path, name string) ([]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	start := strings.Index(string(body), "var "+name+" = map[string]string{")
	if start < 0 {
		return nil, fmt.Errorf("map %s was not found", name)
	}
	end := strings.Index(string(body)[start:], "\n}")
	if end < 0 {
		return nil, fmt.Errorf("map %s has no closing brace", name)
	}
	block := string(body)[start : start+end]
	re := regexp.MustCompile(`"([a-z][a-z0-9_]*)":\s*([A-Za-z][A-Za-z0-9]+)`)
	constants := sourceStringConstants(string(body))
	var values []string
	for _, match := range re.FindAllStringSubmatch(block, -1) {
		value := constants[match[2]]
		if value == "" {
			return nil, fmt.Errorf("scope constant %s has no string value", match[2])
		}
		values = append(values, match[1]+" — "+value)
	}
	sort.Strings(values)
	return values, nil
}

func sourceStringConstants(body string) map[string]string {
	re := regexp.MustCompile(`(?m)^\s*([A-Za-z][A-Za-z0-9]+)\s*=\s*"([^"]+)"`)
	values := map[string]string{}
	for _, match := range re.FindAllStringSubmatch(body, -1) {
		values[match[1]] = match[2]
	}
	return values
}

func mcpScopeNames(root string) ([]string, error) {
	body, err := os.ReadFile(filepath.Join(root, "internal/httpapi/mcp_scopes.go"))
	if err != nil {
		return nil, err
	}
	constants := sourceStringConstants(string(body))
	re := regexp.MustCompile(`return \[\]string\{([^}]+)\}`)
	match := re.FindStringSubmatch(string(body))
	if match == nil {
		return nil, fmt.Errorf("MCPScopes return registry was not found")
	}
	var values []string
	for _, name := range strings.Split(match[1], ",") {
		value := constants[strings.TrimSpace(name)]
		if value == "" {
			return nil, fmt.Errorf("scope constant %q has no string value", strings.TrimSpace(name))
		}
		values = append(values, value)
	}
	return values, nil
}

func guiJourneys(root string) ([]string, error) {
	manifest, err := docjourney.LoadAndValidate(root, "performance/v0.7-g18e/JOURNEYS.json")
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(manifest.Journeys))
	for _, journey := range manifest.Journeys {
		values = append(values, journey.Label+" — "+journey.State+" ("+journey.ID+")")
	}
	return values, nil
}

// RepositorySurfaces is the exported view of the four surface inventories,
// for the features registry to check itself against.
//
// It reuses the same extractors the generated fragments and the pinned counts
// already use, rather than reading the sources again. A second extraction would
// be a second opinion about what exists, and the whole value of a coverage
// check is that there is only one.
type RepositorySurfaces struct {
	CLI  []string
	REST []string
	MCP  []string
	GUI  []string
}

func Surfaces(root string) (RepositorySurfaces, error) {
	cli, err := cliUsageForms(root)
	if err != nil {
		return RepositorySurfaces{}, err
	}
	rest, err := restOperations(root)
	if err != nil {
		return RepositorySurfaces{}, err
	}
	mcp, err := mcpTools(root)
	if err != nil {
		return RepositorySurfaces{}, err
	}
	gui, err := guiJourneys(root)
	if err != nil {
		return RepositorySurfaces{}, err
	}
	return RepositorySurfaces{CLI: cli, REST: rest, MCP: mcp, GUI: gui}, nil
}
