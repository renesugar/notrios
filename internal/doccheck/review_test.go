package doccheck

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/docaudit"
)

func TestPromptContracts(t *testing.T) {
	source := "func Example() string { return \"ok\" }"
	p := explanationPrompt(source)
	if contains := "CLAIM:"; contains != "" && stringContains(p, contains) {
		t.Fatal("blind prompt contains claim marker")
	}
	if !stringContains(conflictClassificationPrompt("claim", "independent explanation"), "CLAIM:\nclaim") {
		t.Fatal("classification omitted claim")
	}
	if !stringContains(supportClassificationPrompt("claim", "independent explanation"), "INDEPENDENT EXPLANATION:\nindependent explanation") {
		t.Fatal("classification omitted explanation")
	}
	if !stringContains(actionabilityPrompt("use the command"), "PROSE:\nuse the command") {
		t.Fatal("action prompt omitted prose")
	}
	if !validVerdict("supported") || !validVerdict("contradicted") || !validVerdict("not-determinable") || validVerdict("maybe") {
		t.Fatal("verdict grammar drift")
	}
}

func TestFixtureActionMatchIsClosed(t *testing.T) {
	c := FixtureCatalog{
		Examples:      []docaudit.ExampleCandidate{{ID: "ex", Body: "notriosctl version"}, {ID: "unrun", Body: "notriosctl shared"}},
		ExampleStates: map[string]string{"ex": "executed", "unrun": "unverified"}, ExampleSurfaces: map[string]string{"ex": "cli"},
		Journeys: []JourneyFixture{{ID: "journey", Label: "Open page", State: "executed"}, {ID: "unrun-journey", Label: "Unavailable", State: "unverified"}},
	}
	if !c.Match("command", "notriosctl version").Accepted {
		t.Fatal("exact command was rejected")
	}
	if c.Match("api", "notriosctl version").Accepted || c.Match("command", "notriosctl shared").Accepted {
		t.Fatal("wrong-surface or unverified example accepted")
	}
	if c.Match("command", "curl http://evil").Accepted {
		t.Fatal("arbitrary command accepted")
	}
	if !c.Match("gui", "journey").Accepted || c.Match("gui", "other").Accepted {
		t.Fatal("GUI closure failed")
	}
	if c.Match("gui", "unrun-journey").Accepted {
		t.Fatal("unverified GUI journey accepted")
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type scriptedCompleter struct {
	t       *testing.T
	results []CompletionResult
	prompts []CompletionRequest
}

func (s *scriptedCompleter) Complete(_ context.Context, request CompletionRequest) (CompletionResult, error) {
	s.t.Helper()
	s.prompts = append(s.prompts, request)
	if len(s.results) == 0 {
		s.t.Fatal("unexpected model request")
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

func TestRunCalibratesBeforeReviewAndRecordsTriageGap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package fixture

// CalibrationValue is twenty seven.
const CalibrationValue = 27

// User prose is deliberately different from calibration prose.
//notrios:doc user user-fragment
const UserValue = 1
`
	if err := os.WriteFile(filepath.Join(root, "internal", "fixture", "fixture.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	calibration := Calibration{Schema: "notrios.g18a.contradiction-calibration.v1", Labels: []string{"supported", "contradicted", "not-determinable"}, Cases: []CalibrationCase{{
		ID: "control", Claim: "The value is 27.", Anchor: "go:github.com/renesugar/notrios/internal/fixture#CalibrationValue", Expected: "supported",
	}}}
	encoded, _ := json.Marshal(calibration)
	calibrationPath := filepath.Join(root, "calibration.json")
	if err := os.WriteFile(calibrationPath, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	model := &scriptedCompleter{t: t, results: []CompletionResult{
		{Content: "The constant is 27.", Model: "local"}, {Content: "no", Model: "local"}, {Content: "yes", Model: "local"},
		{Content: "The constant is 27.", Model: "local"}, {Content: "no", Model: "local"}, {Content: "yes", Model: "local"},
		{Content: "The constant is 1.", Model: "local"}, {Content: "yes", Model: "local"}, {Content: "gui: Open page", Model: "local"},
		{Content: "The constant is one.", Model: "local"}, {Content: "yes", Model: "local"}, {Content: "command: arbitrary", Model: "local"},
	}}
	report, err := Run(context.Background(), model, ReviewOptions{
		Root: root, Repeats: 2, Temperature: 0.1, CalibrationPath: calibrationPath,
		Fixtures: FixtureCatalog{Journeys: []JourneyFixture{{ID: "open", Label: "Open page", State: "executed"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(model.results) != 0 || len(report.Calibration.Runs) != 1 || report.Calibration.Correct != 2 || report.Calibration.Total != 2 {
		t.Fatalf("calibration report or scripted requests incomplete: %+v", report.Calibration)
	}
	if len(report.Reviews) != 1 || len(report.Reviews[0].Runs) != 2 || !report.Reviews[0].Variance {
		t.Fatalf("review report = %+v", report.Reviews)
	}
	if len(report.Summary.MissingDispositions) != 1 || report.Summary.MissingDispositions[0] != "user-fragment" {
		t.Fatalf("triage gap = %+v", report.Summary.MissingDispositions)
	}
	if !report.Reviews[0].ActionRuns[0].Fixture.Accepted || report.Reviews[0].ActionRuns[1].Fixture.Accepted {
		t.Fatalf("fixture closure = %+v", report.Reviews[0].ActionRuns)
	}
	if len(model.prompts) != 12 || !stringContains(model.prompts[0].Prompt, "CalibrationValue") || !stringContains(model.prompts[6].Prompt, "UserValue") {
		t.Fatal("request order did not calibrate first")
	}
	for _, index := range []int{1, 2, 4, 5, 7, 10} {
		if model.prompts[index].Grammar != yesNoGrammar {
			t.Fatalf("request %d binary grammar = %q", index, model.prompts[index].Grammar)
		}
	}
}
