package docfeatures_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/docgen"
)

// The Reports section of docs/operations.md lists every report Notrios can
// produce (v1.0 J40-F). Nothing found it before: each report was described
// where it was built, and no page had a section a reader could find by looking
// for "reports".
//
// The rule is mechanical so that it can fail. A command, REST route or MCP tool
// whose name says "report" is a report, and must be named in the section, in
// the form a reader would type it; and everything the section names must still
// exist. Reports with other names -- lint, fix, gc, doctor -- are listed by
// choice, and the second half of the rule keeps them honest too.

const reportsDocument = "docs/operations.md"

func reportsSection(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, reportsDocument))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	start := strings.Index(text, "\n## Reports\n")
	if start < 0 {
		t.Fatalf("%s has no \"## Reports\" section", reportsDocument)
	}
	section := text[start+1:]
	if end := strings.Index(section[len("## Reports\n"):], "\n## "); end >= 0 {
		section = section[:len("## Reports\n")+end]
	}
	return section
}

// reportSurfaces is every surface whose name says "report", as a reader would
// write it: `notriosctl graph report`, `notriosctl paths --report`,
// `GET /api/v1/graph/report`, `get_graph_report`.
func reportSurfaces(surfaces docgen.RepositorySurfaces) []string {
	out := []string{}
	for _, form := range surfaces.CLI {
		fields := strings.Fields(form)
		command := []string{}
		for _, field := range fields {
			if strings.HasPrefix(field, "[") || strings.HasPrefix(field, "-") || strings.HasPrefix(field, "<") {
				break
			}
			command = append(command, field)
		}
		name := strings.Join(command, " ")
		switch {
		case strings.Contains(name, "report"):
			out = append(out, name)
		case strings.Contains(form, "--report"):
			out = append(out, name+" --report")
		}
	}
	for _, operation := range surfaces.REST {
		if strings.Contains(operation, "report") {
			out = append(out, operation)
		}
	}
	for _, tool := range surfaces.MCP {
		if strings.Contains(tool, "report") {
			out = append(out, tool)
		}
	}
	sort.Strings(out)
	return out
}

func TestEveryReportIsListed(t *testing.T) {
	root := filepath.Join("..", "..")
	surfaces, err := docgen.Surfaces(root)
	if err != nil {
		t.Fatal(err)
	}
	section := reportsSection(t, root)
	reports := reportSurfaces(surfaces)
	if len(reports) < 10 {
		t.Fatalf("only %d report surfaces found; the rule has stopped seeing them: %v", len(reports), reports)
	}
	for _, report := range reports {
		if !strings.Contains(section, "`"+report+"`") {
			t.Errorf("%s is a report and the Reports section of %s does not list it", report, reportsDocument)
		}
	}
}

var (
	sectionCommand = regexp.MustCompile("`(notriosctl [a-z][a-z -]*?)`")
	sectionRoute   = regexp.MustCompile("`((?:GET|POST|PUT|PATCH|DELETE) /[^`]+)`")
	sectionTool    = regexp.MustCompile("`([a-z]+(?:_[a-z]+)+)`")
)

func TestTheReportsSectionNamesOnlyWhatExists(t *testing.T) {
	root := filepath.Join("..", "..")
	surfaces, err := docgen.Surfaces(root)
	if err != nil {
		t.Fatal(err)
	}
	section := reportsSection(t, root)
	commands := map[string]bool{}
	for _, form := range surfaces.CLI {
		fields := []string{}
		for _, field := range strings.Fields(form) {
			if strings.HasPrefix(field, "[") || strings.HasPrefix(field, "<") {
				break
			}
			fields = append(fields, field)
		}
		for i := range fields {
			commands[strings.Join(fields[:i+1], " ")] = true
		}
	}
	routes := map[string]bool{}
	for _, operation := range surfaces.REST {
		routes[operation] = true
	}
	tools := map[string]bool{}
	for _, tool := range surfaces.MCP {
		tools[tool] = true
	}
	for _, match := range sectionCommand.FindAllStringSubmatch(section, -1) {
		command := strings.TrimSpace(match[1])
		// A flag is part of how a report is asked for: `paths --report`.
		bare := strings.TrimSpace(strings.Split(command, " --")[0])
		if !commands[bare] {
			t.Errorf("the Reports section names %q, which is no command", command)
		}
		if bare != command && !strings.Contains(strings.Join(surfaces.CLI, "\n"), strings.TrimPrefix(command, bare+" ")) {
			t.Errorf("the Reports section names %q, and %s has no such flag", command, bare)
		}
	}
	for _, match := range sectionRoute.FindAllStringSubmatch(section, -1) {
		if !routes[match[1]] {
			t.Errorf("the Reports section names %q, which is no REST operation", match[1])
		}
	}
	for _, match := range sectionTool.FindAllStringSubmatch(section, -1) {
		if strings.HasSuffix(match[1], "_report") && !tools[match[1]] {
			t.Errorf("the Reports section names %q, which is no MCP tool", match[1])
		}
	}
}
