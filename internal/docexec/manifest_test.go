package docexec

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/docaudit"
)

func fixtureManifest(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	doc := "# Configuration\n\n```sh\nnotriosctl status --name {{DB}}\n```\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "topic.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(root, "inventory.json"), map[string]any{"documents": []any{map[string]any{"path": "docs/topic.md", "sections": []any{map[string]any{"id": "configuration"}}}}}); err != nil {
		t.Fatal(err)
	}
	body := "notriosctl status --name {{DB}}"
	reg := docaudit.Registry{Schema: docaudit.RegistrySchema, Executables: []docaudit.RegisteredExample{{ID: "topic-configuration-example-1", Path: "docs/topic.md", Section: "configuration", Language: "sh", SHA256: HashFence(body), State: docaudit.GradeExecuted, Check: "go:github.com/renesugar/notrios/internal/docexec#TestManifest", Execution: &docaudit.ExampleExecution{Surface: "cli", Fixture: "scratch", Case: "status", Substitutions: []docaudit.ExampleSubstitution{{Token: "{{DB}}", Source: "database"}}, Expected: docaudit.ExampleExpected{Kind: "exit", Status: 0}, Postcondition: docaudit.ExamplePostcondition{Kind: "semantic", Detail: "status present"}}}}}
	if err := writeJSON(filepath.Join(root, "registry.json"), reg); err != nil {
		t.Fatal(err)
	}
	return root, body
}

func TestLoadManifestAndRun(t *testing.T) {
	root, _ := fixtureManifest(t)
	m, err := LoadManifest(root, "inventory.json", "registry.json")
	if err != nil {
		t.Fatal(err)
	}
	seen := ""
	report, err := m.Run(context.Background(), RunOptions{SubstitutionValues: map[string]string{"database": "scratch.sqlite"}, Adapters: map[string]Adapter{"status": func(_ context.Context, in Invocation) (AdapterResult, error) {
		seen = in.Body
		return AdapterResult{Kind: "exit", Status: 0, PostconditionOK: true}, nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, "scratch.sqlite") || report.Executed != 1 || len(report.Topics) != 1 {
		t.Fatalf("bad result: seen=%q report=%+v", seen, report)
	}
}

func TestManifestMutationsFail(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string)
	}{
		{"changed fence", func(root string) {
			p := filepath.Join(root, "docs", "topic.md")
			b, _ := os.ReadFile(p)
			_ = os.WriteFile(p, []byte(strings.Replace(string(b), "status", "validate", 1)), 0o644)
		}},
		{"changed hash", func(root string) {
			p := filepath.Join(root, "registry.json")
			b, _ := os.ReadFile(p)
			_ = os.WriteFile(p, []byte(strings.Replace(string(b), "sha256", "sha257", 1)), 0o644)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := fixtureManifest(t)
			tc.mutate(root)
			if _, err := LoadManifest(root, "inventory.json", "registry.json"); err == nil {
				t.Fatal("mutation unexpectedly loaded")
			}
		})
	}
}

func runnableManifest(t *testing.T) Manifest {
	root, _ := fixtureManifest(t)
	m, err := LoadManifest(root, "inventory.json", "registry.json")
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestRunContractMutationsFail(t *testing.T) {
	tests := []struct {
		name string
		opts RunOptions
	}{
		{"missing case", RunOptions{SubstitutionValues: map[string]string{"database": "x"}, Adapters: map[string]Adapter{}}},
		{"unused case", RunOptions{SubstitutionValues: map[string]string{"database": "x"}, Adapters: map[string]Adapter{"status": okAdapter, "extra": okAdapter}}},
		{"bad status", RunOptions{SubstitutionValues: map[string]string{"database": "x"}, Adapters: map[string]Adapter{"status": func(context.Context, Invocation) (AdapterResult, error) {
			return AdapterResult{Kind: "exit", Status: 7, PostconditionOK: true}, nil
		}}}},
		{"unresolved substitution", RunOptions{Adapters: map[string]Adapter{"status": okAdapter}}},
		{"bad postcondition", RunOptions{SubstitutionValues: map[string]string{"database": "x"}, Adapters: map[string]Adapter{"status": func(context.Context, Invocation) (AdapterResult, error) {
			return AdapterResult{Kind: "exit", Status: 0, Detail: "no record"}, nil
		}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runnableManifest(t).Run(context.Background(), tc.opts); err == nil {
				t.Fatal("mutation unexpectedly passed")
			}
		})
	}
}
func okAdapter(context.Context, Invocation) (AdapterResult, error) {
	return AdapterResult{Kind: "exit", Status: 0, PostconditionOK: true}, nil
}
func writeJSON(path string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
