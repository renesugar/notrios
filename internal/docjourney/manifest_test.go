package docjourney

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndValidateHonestFixture(t *testing.T) {
	root, path := writeFixture(t)
	if got, err := LoadAndValidate(root, path); err != nil {
		t.Fatal(err)
	} else if len(got.Journeys) != 37 {
		t.Fatalf("journeys = %d", len(got.Journeys))
	}
}

func TestLoadAndValidateMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown field", func(m map[string]any) { m["extra"] = true }},
		{"duplicate id", func(m map[string]any) {
			js := m["journeys"].([]any)
			js[1].(map[string]any)["id"] = js[0].(map[string]any)["id"]
		}},
		{"missing source", func(m map[string]any) { m["journeys"].([]any)[0].(map[string]any)["path"] = "docs/missing.md" }},
		{"missing section", func(m map[string]any) { m["journeys"].([]any)[0].(map[string]any)["section"] = "missing-heading" }},
		{"wrong counts", func(m map[string]any) { m["journeys"].([]any)[0].(map[string]any)["state"] = "unverified" }},
		{"invalid viewport", func(m map[string]any) { m["viewports"].([]any)[0].(map[string]any)["width"] = 1 }},
		{"duplicate journey viewport", func(m map[string]any) {
			m["journeys"].([]any)[0].(map[string]any)["viewports"] = []any{"desktop", "desktop"}
		}},
		{"executed no postcondition", func(m map[string]any) { m["journeys"].([]any)[0].(map[string]any)["postconditions"] = []any{} }},
		{"executed no check anchor", func(m map[string]any) { delete(m["journeys"].([]any)[0].(map[string]any), "check_anchor") }},
		{"unverified no reason", func(m map[string]any) { delete(m["journeys"].([]any)[32].(map[string]any), "unrun_reason") }},
		{"invalid owner", func(m map[string]any) { m["journeys"].([]any)[0].(map[string]any)["ui_owner"] = "owner" }},
		{"wrong action owner kind", func(m map[string]any) {
			m["journeys"].([]any)[0].(map[string]any)["action_owners"] = []any{"go:package#Handler"}
		}},
		{"parent path escape", func(m map[string]any) { m["journeys"].([]any)[0].(map[string]any)["path"] = ".." }},
		{"unknown reason", func(m map[string]any) {
			m["journeys"].([]any)[32].(map[string]any)["unrun_reason"].(map[string]any)["code"] = "maybe"
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, path := writeFixture(t)
			raw, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			tc.mutate(m)
			out, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, path), out, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadAndValidate(root, path); err == nil {
				t.Fatal("mutation unexpectedly validated")
			}
		})
	}
}

func writeFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "gui.md"), []byte("# GUI\n\n## GUI\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ids := append([]string(nil), requiredLegacyIDs...)
	for i := 0; len(ids) < 37; i++ {
		ids = append(ids, fmt.Sprintf("journey-%02d", i))
	}
	m := map[string]any{"schema": Schema, "browser_policy": map[string]any{"preferred": "browser-plugin", "observed": "absent", "fallback": "playwright", "reason": "plugin unavailable in this environment"}, "viewports": []any{map[string]any{"id": "desktop", "width": 1440, "height": 960}, map[string]any{"id": "narrow-sync", "width": 390, "height": 844}}, "journeys": []any{}}
	js := m["journeys"].([]any)
	for i, id := range ids {
		j := map[string]any{"id": id, "path": "docs/gui.md", "section": "gui", "label": "Journey", "state": "executed", "ui_owner": "ts:web/src/App.tsx#App", "action_owners": []any{"ts:web/src/api.ts#search"}, "go_owners": []any{"go:example.test/internal/httpapi#(*Server).handleSearchPOST"}, "check_anchor": "go:example.test/cmd/client#TestJourneys", "case": "case", "viewports": []any{"desktop", "narrow-sync"}, "postconditions": []any{"visible result"}}
		if i >= 32 {
			delete(j, "case")
			delete(j, "check_anchor")
			delete(j, "viewports")
			delete(j, "postconditions")
			j["state"] = "unverified"
			j["unrun_reason"] = map[string]any{"code": "browser-owned", "detail": "covered by browser-owned shell", "owner": "ts:web/src/App.tsx#App"}
		}
		js = append(js, j)
	}
	m["journeys"] = js
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("manifest.json")
	if err := os.WriteFile(filepath.Join(root, path), b, 0600); err != nil {
		t.Fatal(err)
	}
	return root, path
}
