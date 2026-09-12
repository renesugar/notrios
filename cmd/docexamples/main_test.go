package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
}

// TestPublishedExamplesMatchTheirTrackedSet is the gate. Without it the tracked
// set is a second copy of the fences rather than their source, and a copy that
// nothing compares is worse than no copy at all.
func TestPublishedExamplesMatchTheirTrackedSet(t *testing.T) {
	root := repositoryRoot(t)
	for _, source := range Sets {
		set, err := load(root, source)
		if err != nil {
			t.Fatal(err)
		}
		rendered, hashes, err := renderDocument(root, set)
		if err != nil {
			t.Fatal(err)
		}
		current, err := os.ReadFile(filepath.Join(root, set.Document))
		if err != nil {
			t.Fatal(err)
		}
		if string(current) != rendered {
			t.Errorf("%s does not match %s; run: go run ./cmd/docexamples --write",
				set.Document, source)
		}

		// And the registry pins what was rendered. A generated fence with a
		// stale hash fails internal/docaudit, but it fails there as a mystery;
		// failing here says which tool to run.
		registry, missing, err := updateRegistry(root, hashes)
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) > 0 {
			t.Errorf("generated and not registered, so nothing runs them: %v", missing)
		}
		committed, err := os.ReadFile(filepath.Join(root, "docs/docaudit/registry.json"))
		if err != nil {
			t.Fatal(err)
		}
		if string(committed) != registry {
			t.Error("the registry's hashes for generated examples are stale; " +
				"run: go run ./cmd/docexamples --write")
		}
	}
}

// Every example either runs a command or says why none can. A published example
// with no check and no explanation is the defect this whole item removes, so it
// is refused at the source rather than noticed in review.
func TestEveryTrackedExampleIsCheckedOrExplained(t *testing.T) {
	root := repositoryRoot(t)
	for _, source := range Sets {
		set, err := load(root, source)
		if err != nil {
			t.Fatal(err)
		}
		for _, example := range set.Examples {
			id := example.ID(set.Document)
			switch {
			case example.Verify != "" && example.Postcondition == "":
				t.Errorf("%s runs a command and declares no postcondition", id)
			case example.Verify == "" && len(example.Unverifiable) < 40:
				t.Errorf("%s has no command and no real reason: %q", id, example.Unverifiable)
			}
			if len(example.Settings) == 0 {
				t.Errorf("%s sets nothing, so it demonstrates nothing", id)
			}
			if example.UseCase == "" {
				t.Errorf("%s has no use case, and the use case is the point", id)
			}
		}
	}
}

// The renderer refuses the shapes it cannot render honestly, rather than
// emitting something that looks like an example.
func TestTheRendererRefusesWhatItCannotRender(t *testing.T) {
	cases := []struct {
		name    string
		example Example
		wants   string
	}{
		{"no settings", Example{UseCase: "empty", Verify: "x", File: "a.yaml"}, "sets nothing"},
		{"unsectioned key", Example{UseCase: "flat", File: "a.yaml", Verify: "x",
			Settings: []Setting{{Key: "loose", Value: "1"}}}, "not a sectioned key"},
		{"too deep", Example{UseCase: "deep", File: "a.yaml", Verify: "x",
			Settings: []Setting{{Key: "a.b.c", Value: "1"}}}, "nests deeper"},
		{"value and list", Example{UseCase: "both", File: "a.yaml", Verify: "x",
			Settings: []Setting{{Key: "a.b", Value: "1", List: []string{"x"}}}}, "both a value and a list"},
		{"no value", Example{UseCase: "blank", File: "a.yaml", Verify: "x",
			Settings: []Setting{{Key: "a.b"}}}, "has no value"},
		{"verified with no file", Example{UseCase: "nofile", Verify: "x",
			Settings: []Setting{{Key: "a.b", Value: "1"}}}, "needs a file to write"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := testCase.example.Render()
			if err == nil {
				t.Fatalf("rendered a %s example instead of refusing it", testCase.name)
			}
			if !strings.Contains(err.Error(), testCase.wants) {
				t.Errorf("refused with %q, wanted something about %q", err, testCase.wants)
			}
		})
	}
}

// The generated block is a fence internal/docaudit will scan, so the markers
// have to sit outside it. A marker inside the fence would be published as part
// of the command.
func TestMarkersSitOutsideTheFence(t *testing.T) {
	root := repositoryRoot(t)
	for _, source := range Sets {
		set, err := load(root, source)
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(root, set.Document))
		if err != nil {
			t.Fatal(err)
		}
		for _, example := range set.Examples {
			id := example.ID(set.Document)
			begin, finish := markers(id)
			pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(begin) +
				"\n```[a-z]+\n.*?\n```\n" + regexp.QuoteMeta(finish))
			if !pattern.Match(body) {
				t.Errorf("%s: the markers do not wrap exactly one fence", id)
			}
		}
	}
}
