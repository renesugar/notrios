package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/docjourneys"
)

// TestCLIJourneysActuallyRun executes every journey in the catalogue against a
// disposable library.
//
// The point is not that the commands exit zero. It is that the postcondition
// holds afterwards -- that the thing the reader wanted actually happened. A
// command can exit zero having done nothing, and it can exit zero having done
// the opposite of what the page said; only looking at the state afterwards
// tells those apart.
func TestCLIJourneysActuallyRun(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	catalogue, err := docjourneys.Load(filepath.Join("..", "..", "docs", "docjourneys", "CLI_JOURNEYS.json"))
	if err != nil {
		t.Fatalf("loading the journey catalogue: %v", err)
	}
	if len(catalogue.Journeys) == 0 {
		t.Fatal("the catalogue is empty")
	}

	for _, journey := range catalogue.Journeys {
		t.Run(journey.ID, func(t *testing.T) {
			sandbox := t.TempDir()
			vault := filepath.Join(sandbox, "vault")
			if err := os.MkdirAll(vault, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(sandbox, "carrier"), 0o755); err != nil {
				t.Fatal(err)
			}
			// The manual step of the note journey, performed for it. A journey
			// that says "write a Markdown file" needs one to exist.
			if err := os.WriteFile(filepath.Join(vault, "Reed beds.md"),
				[]byte("---\ntags: [field]\n---\n\n# Reed beds\n\nSeen at dusk.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(sandbox, "notrios.yaml")
			if err := os.WriteFile(config,
				[]byte("sync:\n  rest:\n    credential_store: development-file\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			docsDir, err := filepath.Abs(filepath.Join("..", "..", "docs"))
			if err != nil {
				t.Fatal(err)
			}
			values := map[string]string{
				"db":      filepath.Join(sandbox, "notes.sqlite"),
				"assets":  filepath.Join(sandbox, "assets"),
				"vault":   vault,
				"out":     filepath.Join(sandbox, "out"),
				"out2":    filepath.Join(sandbox, "out2"),
				"keys":    filepath.Join(sandbox, "sync-keys.json"),
				"config":  config,
				"sandbox": sandbox,
				"docs":    docsDir,
				"carrier": filepath.Join(sandbox, "carrier"),
			}
			expand := func(command []string) []string {
				expanded := make([]string, 0, len(command))
				for _, argument := range command {
					expanded = append(expanded, docjourneys.Substitute(values, argument))
				}
				return expanded
			}

			for index, step := range journey.Steps {
				if step.Manual {
					continue
				}
				result := runCLIIn(t, sandbox, binary, expand(step.Command)...)
				if result.exitCode != 0 && !step.AllowFailure {
					t.Fatalf("step %d (%s) exited %d\nstdout: %s\nstderr: %s",
						index+1, strings.Join(step.Command, " "), result.exitCode, result.stdout, result.stderr)
				}
				// A later step can refer to what an earlier one made. Without
				// this a journey could only ever describe operations on things
				// that already existed, which is not what most tasks look like:
				// you make a note and then do something to it.
				var produced map[string]any
				if json.Unmarshal([]byte(result.stdout), &produced) == nil {
					if id, ok := produced["document_id"].(string); ok && id != "" {
						values["note"] = id
					}
				}
			}

			post := journey.Postcondition
			if post.FileExists != "" {
				path := docjourneys.Substitute(values, post.FileExists)
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("postcondition (%s) failed: %v", post.Narrative, err)
				}
			}
			if len(post.Command) == 0 {
				return
			}
			result := runCLIIn(t, sandbox, binary, expand(post.Command)...)
			combined := result.stdout + result.stderr
			for _, want := range post.Contains {
				if !strings.Contains(combined, want) {
					t.Errorf("postcondition (%s): output does not contain %q\nstdout: %s\nstderr: %s",
						post.Narrative, want, result.stdout, result.stderr)
				}
			}
			for _, unwanted := range post.Absent {
				if strings.Contains(combined, unwanted) {
					t.Errorf("postcondition (%s): output unexpectedly contains %q", post.Narrative, unwanted)
				}
			}
		})
	}
}
