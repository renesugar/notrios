package doccheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/docaudit"
)

const verdictGrammar = `root ::= "supported" | "contradicted" | "not-determinable"`
const yesNoGrammar = `root ::= "yes" | "no"`

// Completer is the deliberately small model boundary used by the advisory
// reviewer. The production implementation is Client; tests use a scripted
// implementation and never need a live model.
type Completer interface {
	Complete(context.Context, CompletionRequest) (CompletionResult, error)
}

type Calibration struct {
	Schema string            `json:"schema"`
	Labels []string          `json:"labels"`
	Policy string            `json:"policy"`
	Cases  []CalibrationCase `json:"cases"`
}

type CalibrationCase struct {
	ID            string   `json:"id"`
	Origin        string   `json:"origin"`
	Claim         string   `json:"claim"`
	Anchor        string   `json:"anchor"`
	DirectCallees []string `json:"direct_callees,omitempty"`
	Expected      string   `json:"expected"`
	Reason        string   `json:"reason"`
}

type Triage struct {
	Schema       string              `json:"schema"`
	Dispositions []TriageDisposition `json:"dispositions"`
}

type TriageDisposition struct {
	ID          string `json:"id"`
	Disposition string `json:"disposition"`
	Note        string `json:"note"`
}

type ReviewOptions struct {
	Root            string
	Repeats         int
	Temperature     float64
	CalibrationPath string
	TriagePath      string
	Fixtures        FixtureCatalog
	CalibrationOnly bool
}

type AdvisoryReport struct {
	Schema      string            `json:"schema"`
	GeneratedAt string            `json:"generated_at"`
	Policy      ReportPolicy      `json:"policy"`
	Calibration CalibrationReport `json:"calibration"`
	Reviews     []FragmentReview  `json:"reviews"`
	Summary     AdvisorySummary   `json:"summary"`
}

type ReportPolicy struct {
	EndpointScope string  `json:"endpoint_scope"`
	SourceScope   string  `json:"source_scope"`
	AutomaticEdit bool    `json:"automatic_edit"`
	BuildBlocking bool    `json:"build_blocking"`
	CostUSD       float64 `json:"cost_usd"`
}

type CalibrationReport struct {
	Schema          string                    `json:"schema"`
	Runs            []CalibrationCaseReport   `json:"runs"`
	ConfusionMatrix map[string]map[string]int `json:"confusion_matrix"`
	NegationCases   []string                  `json:"negation_cases"`
	Correct         int                       `json:"correct"`
	Total           int                       `json:"total"`
}

type CalibrationCaseReport struct {
	ID       string      `json:"id"`
	Expected string      `json:"expected"`
	Runs     []ReviewRun `json:"runs"`
	Variance bool        `json:"variance"`
}

type FragmentReview struct {
	ID          string             `json:"id"`
	Claim       string             `json:"claim"`
	Anchor      string             `json:"anchor"`
	EvidenceSHA string             `json:"evidence_sha256"`
	Runs        []ReviewRun        `json:"runs"`
	Variance    bool               `json:"variance"`
	ActionRuns  []ActionabilityRun `json:"actionability_runs"`
	Disposition *TriageDisposition `json:"disposition,omitempty"`
}

type ReviewRun struct {
	Repeat                int     `json:"repeat"`
	BlindExplanation      string  `json:"blind_explanation"`
	Verdict               string  `json:"verdict"`
	ConflictAnswer        string  `json:"conflict_answer"`
	SupportAnswer         string  `json:"support_answer,omitempty"`
	Model                 string  `json:"model"`
	SourceSHA256          string  `json:"source_sha256"`
	ExplanationPromptSHA  string  `json:"explanation_prompt_sha256"`
	VerdictPromptSHA      string  `json:"verdict_prompt_sha256"`
	TokensEvaluated       int     `json:"tokens_evaluated"`
	TokensPredicted       int     `json:"tokens_predicted"`
	PromptMilliseconds    float64 `json:"prompt_ms"`
	PredictedMilliseconds float64 `json:"predicted_ms"`
}

type ActionabilityRun struct {
	Repeat       int          `json:"repeat"`
	RawAttempt   string       `json:"raw_attempt"`
	Surface      string       `json:"surface"`
	Attempt      string       `json:"attempt"`
	PromptSHA256 string       `json:"prompt_sha256"`
	Model        string       `json:"model"`
	Fixture      FixtureMatch `json:"fixture"`
}

type FixtureMatch struct {
	Accepted bool   `json:"accepted"`
	ID       string `json:"id,omitempty"`
	State    string `json:"state,omitempty"`
	Reason   string `json:"reason"`
}

type AdvisorySummary struct {
	Fragments               int            `json:"fragments"`
	Verdicts                map[string]int `json:"verdicts"`
	Contradicted            []string       `json:"contradicted"`
	MissingDispositions     []string       `json:"missing_dispositions"`
	AcceptedFixtureAttempts int            `json:"accepted_fixture_attempts"`
}

type FixtureCatalog struct {
	Examples        []docaudit.ExampleCandidate
	ExampleStates   map[string]string
	ExampleSurfaces map[string]string
	Journeys        []JourneyFixture
}

type JourneyFixture struct {
	ID    string
	Label string
	State string
}

func LoadCalibration(path string) (Calibration, error) {
	var out Calibration
	if err := readStrictJSON(path, &out); err != nil {
		return out, err
	}
	if out.Schema != "notrios.g18a.contradiction-calibration.v1" || len(out.Cases) == 0 {
		return out, errors.New("unsupported or empty calibration set")
	}
	seen := map[string]bool{}
	for _, item := range out.Cases {
		if item.ID == "" || item.Claim == "" || item.Anchor == "" || !validVerdict(item.Expected) || seen[item.ID] {
			return out, fmt.Errorf("invalid calibration case %q", item.ID)
		}
		seen[item.ID] = true
	}
	return out, nil
}

func LoadTriage(path string) (map[string]TriageDisposition, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]TriageDisposition{}, nil
	}
	var input Triage
	if err := readStrictJSON(path, &input); err != nil {
		return nil, err
	}
	if input.Schema != "notrios.doccheck.triage.v1" {
		return nil, fmt.Errorf("unsupported triage schema %q", input.Schema)
	}
	out := make(map[string]TriageDisposition, len(input.Dispositions))
	for _, item := range input.Dispositions {
		if item.ID == "" || strings.TrimSpace(item.Disposition) == "" || strings.TrimSpace(item.Note) == "" || out[item.ID].ID != "" {
			return nil, fmt.Errorf("invalid triage disposition %q", item.ID)
		}
		out[item.ID] = item
	}
	return out, nil
}

func Run(ctx context.Context, model Completer, options ReviewOptions) (AdvisoryReport, error) {
	var report AdvisoryReport
	if model == nil || options.Root == "" || options.Repeats < 2 || options.Repeats > 5 || options.Temperature < 0 || options.Temperature > 1 {
		return report, errors.New("invalid review options")
	}
	calibration, err := LoadCalibration(options.CalibrationPath)
	if err != nil {
		return report, err
	}
	triage, err := LoadTriage(options.TriagePath)
	if err != nil {
		return report, err
	}
	report = AdvisoryReport{
		Schema: "notrios.doccheck.advisory.v1", GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Policy:  ReportPolicy{EndpointScope: "loopback-only", SourceScope: "repository-source-only; no notes or databases", AutomaticEdit: false, BuildBlocking: false, CostUSD: 0},
		Summary: AdvisorySummary{Verdicts: map[string]int{}},
	}
	report.Calibration = CalibrationReport{Schema: calibration.Schema, ConfusionMatrix: emptyConfusionMatrix()}
	for _, item := range calibration.Cases {
		slice, err := ResolveSourceSlice(ctx, options.Root, item.Anchor, item.DirectCallees)
		if err != nil {
			return report, fmt.Errorf("calibration %s: %w", item.ID, err)
		}
		caseReport := CalibrationCaseReport{ID: item.ID, Expected: item.Expected}
		for repeat := 1; repeat <= options.Repeats; repeat++ {
			run, err := reviewOnce(ctx, model, item.Claim, slice, repeat, options.Temperature)
			if err != nil {
				return report, fmt.Errorf("calibration %s repeat %d: %w", item.ID, repeat, err)
			}
			caseReport.Runs = append(caseReport.Runs, run)
			report.Calibration.ConfusionMatrix[item.Expected][run.Verdict]++
			report.Calibration.Total++
			if item.Expected == run.Verdict {
				report.Calibration.Correct++
			}
		}
		caseReport.Variance = reviewVariance(caseReport.Runs)
		report.Calibration.Runs = append(report.Calibration.Runs, caseReport)
		if strings.Contains(item.ID, "negat") {
			report.Calibration.NegationCases = append(report.Calibration.NegationCases, item.ID)
		}
	}
	if options.CalibrationOnly {
		return report, nil
	}

	fragments, err := docaudit.ScanFragments(options.Root)
	if err != nil {
		return report, err
	}
	for _, fragment := range fragments {
		if fragment.Audience != "user" {
			continue
		}
		slice, err := ResolveSourceSlice(ctx, options.Root, fragment.Anchor, nil)
		if err != nil {
			return report, fmt.Errorf("fragment %s: %w", fragment.ID, err)
		}
		item := FragmentReview{ID: fragment.ID, Claim: fragment.Prose, Anchor: fragment.Anchor}
		for repeat := 1; repeat <= options.Repeats; repeat++ {
			run, err := reviewOnce(ctx, model, fragment.Prose, slice, repeat, options.Temperature)
			if err != nil {
				return report, fmt.Errorf("fragment %s repeat %d: %w", fragment.ID, repeat, err)
			}
			item.Runs = append(item.Runs, run)
			report.Summary.Verdicts[run.Verdict]++
			action, err := actionabilityOnce(ctx, model, fragment.Prose, repeat, options.Temperature, options.Fixtures)
			if err != nil {
				return report, fmt.Errorf("fragment %s actionability repeat %d: %w", fragment.ID, repeat, err)
			}
			item.ActionRuns = append(item.ActionRuns, action)
			if action.Fixture.Accepted {
				report.Summary.AcceptedFixtureAttempts++
			}
		}
		item.Variance = reviewVariance(item.Runs)
		item.EvidenceSHA = evidenceHash(item)
		if containsVerdict(item.Runs, "contradicted") {
			report.Summary.Contradicted = append(report.Summary.Contradicted, item.ID)
			if disposition, ok := triage[item.ID]; ok {
				copyValue := disposition
				item.Disposition = &copyValue
			} else {
				report.Summary.MissingDispositions = append(report.Summary.MissingDispositions, item.ID)
			}
		}
		report.Reviews = append(report.Reviews, item)
	}
	report.Summary.Fragments = len(report.Reviews)
	return report, nil
}

func reviewOnce(ctx context.Context, model Completer, claim string, source SourceSlice, repeat int, temperature float64) (ReviewRun, error) {
	explainPrompt := explanationPrompt(source.Source)
	explanation, err := model.Complete(ctx, CompletionRequest{Prompt: explainPrompt, Temperature: temperature, NPredict: 256, Stop: []string{"<|im_end|>"}})
	if err != nil {
		return ReviewRun{}, err
	}
	explanation.Content = strings.TrimSpace(explanation.Content)
	if explanation.Content == "" {
		return ReviewRun{}, errors.New("model returned an empty blind explanation")
	}
	conflictPrompt := conflictClassificationPrompt(claim, explanation.Content)
	conflict, err := model.Complete(ctx, CompletionRequest{Prompt: conflictPrompt, Temperature: temperature, NPredict: 8, Stop: []string{"<|im_end|>", "\n"}, Grammar: yesNoGrammar})
	if err != nil {
		return ReviewRun{}, err
	}
	conflictAnswer := strings.TrimSpace(conflict.Content)
	if conflictAnswer != "yes" && conflictAnswer != "no" {
		return ReviewRun{}, fmt.Errorf("invalid conflict answer %q", conflictAnswer)
	}
	value, supportAnswer, supportPrompt := "contradicted", "", ""
	var support CompletionResult
	if conflictAnswer == "no" {
		supportPrompt = supportClassificationPrompt(claim, explanation.Content)
		support, err = model.Complete(ctx, CompletionRequest{Prompt: supportPrompt, Temperature: temperature, NPredict: 8, Stop: []string{"<|im_end|>", "\n"}, Grammar: yesNoGrammar})
		if err != nil {
			return ReviewRun{}, err
		}
		supportAnswer = strings.TrimSpace(support.Content)
		if supportAnswer != "yes" && supportAnswer != "no" {
			return ReviewRun{}, fmt.Errorf("invalid support answer %q", supportAnswer)
		}
		value = "not-determinable"
		if supportAnswer == "yes" {
			value = "supported"
		}
	}
	modelName := conflict.Model
	if modelName == "" {
		modelName = explanation.Model
	}
	return ReviewRun{
		Repeat: repeat, BlindExplanation: explanation.Content, Verdict: value, ConflictAnswer: conflictAnswer, SupportAnswer: supportAnswer, Model: modelName,
		SourceSHA256: digest(source.Source), ExplanationPromptSHA: digest(explainPrompt), VerdictPromptSHA: digest(conflictPrompt + "\x00" + supportPrompt),
		TokensEvaluated:       explanation.TokensEvaluated + conflict.TokensEvaluated + support.TokensEvaluated,
		TokensPredicted:       explanation.TokensPredicted + conflict.TokensPredicted + support.TokensPredicted,
		PromptMilliseconds:    explanation.Timings.PromptMilliseconds + conflict.Timings.PromptMilliseconds + support.Timings.PromptMilliseconds,
		PredictedMilliseconds: explanation.Timings.PredictedMilliseconds + conflict.Timings.PredictedMilliseconds + support.Timings.PredictedMilliseconds,
	}, nil
}

func actionabilityOnce(ctx context.Context, model Completer, prose string, repeat int, temperature float64, fixtures FixtureCatalog) (ActionabilityRun, error) {
	prompt := actionabilityPrompt(prose)
	result, err := model.Complete(ctx, CompletionRequest{Prompt: prompt, Temperature: temperature, NPredict: 192, Stop: []string{"<|im_end|>", "\n\n"}})
	if err != nil {
		return ActionabilityRun{}, err
	}
	raw := strings.TrimSpace(result.Content)
	surface, attempt := parseActionAttempt(raw)
	return ActionabilityRun{Repeat: repeat, RawAttempt: raw, Surface: surface, Attempt: attempt, PromptSHA256: digest(prompt), Model: result.Model, Fixture: fixtures.Match(surface, attempt)}, nil
}

func explanationPrompt(source string) string {
	return "<|im_start|>system\nExplain only behavior established by the provided declaration and bounded direct same-package callees. The documentation claim is deliberately withheld. State uncertainty for anything the source view cannot establish. Do not infer motivations or external-provider behavior. Be concise.\n<|im_end|>\n<|im_start|>user\nSOURCE VIEW:\n" + source + "\n<|im_end|>\n<|im_start|>assistant\n"
}

func classificationInput(claim, explanation string) string {
	return "\n<|im_end|>\n<|im_start|>user\nCLAIM:\n" + claim + "\n\nINDEPENDENT EXPLANATION:\n" + explanation + "\n<|im_end|>\n<|im_start|>assistant\n"
}

func conflictClassificationPrompt(claim, explanation string) string {
	return "<|im_start|>system\nAnswer exactly yes or no. Does the independent explanation directly conflict with any material part of the claim by stating an incompatible number, opposite behavior, or negation? Missing information is no, not a conflict. Examples: claim 'schema is 20' and explanation 'schema is 27' => yes. Claim 'schema is 27' and explanation 'schema is 27' => no. Claim gives a motivation and explanation only describes behavior => no." + classificationInput(claim, explanation)
}

func supportClassificationPrompt(claim, explanation string) string {
	return "<|im_start|>system\nAnswer exactly yes or no. Does the independent explanation explicitly establish every material part of the claim? Missing information, motivation, or external behavior is no. Examples: claim 'schema is 27' and explanation 'schema constant is 27' => yes. Claim gives a motivation and explanation only describes behavior => no." + classificationInput(claim, explanation)
}

func actionabilityPrompt(prose string) string {
	return "<|im_start|>system\nUsing only the prose, decide whether it gives a concrete Notrios action. Output exactly one line. Use not-actionable when it does not. Otherwise use one of command: <literal command>, configuration: <literal configuration>, api: <literal request>, or gui: <literal journey>. Do not invent omitted values. No markdown.\n<|im_end|>\n<|im_start|>user\nPROSE:\n" + prose + "\n<|im_end|>\n<|im_start|>assistant\n"
}

func parseActionAttempt(raw string) (string, string) {
	value := strings.TrimSpace(strings.SplitN(raw, "\n", 2)[0])
	if value == "not-actionable" {
		return "not-actionable", ""
	}
	for _, surface := range []string{"command", "configuration", "api", "gui"} {
		prefix := surface + ": "
		if strings.HasPrefix(value, prefix) && strings.TrimSpace(strings.TrimPrefix(value, prefix)) != "" {
			return surface, strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	return "invalid", value
}

func (catalog FixtureCatalog) Match(surface, attempt string) FixtureMatch {
	if surface == "not-actionable" {
		return FixtureMatch{Reason: "model found no prose-only action"}
	}
	if surface == "invalid" || strings.TrimSpace(attempt) == "" {
		return FixtureMatch{Reason: "output did not satisfy the closed action format"}
	}
	if surface == "gui" {
		for _, item := range catalog.Journeys {
			if attempt == item.ID || attempt == item.Label {
				if item.State != "executed" {
					return FixtureMatch{Reason: "exact journey has no executed G18e fixture; not executed"}
				}
				return FixtureMatch{Accepted: true, ID: item.ID, State: item.State, Reason: "matched the closed G18e journey catalog"}
			}
		}
		return FixtureMatch{Reason: "no exact match in the closed G18e journey catalog; not executed"}
	}
	for _, item := range catalog.Examples {
		if strings.TrimSpace(item.Body) == attempt {
			state := catalog.ExampleStates[item.ID]
			executionSurface := catalog.ExampleSurfaces[item.ID]
			if state != "executed" || !surfaceMatchesFixture(surface, executionSurface) {
				return FixtureMatch{Reason: "exact documentation text has no matching executed G18d surface; not executed"}
			}
			return FixtureMatch{Accepted: true, ID: item.ID, State: state, Reason: "matched the closed G18d executable catalog"}
		}
	}
	return FixtureMatch{Reason: "no exact match in the closed G18d executable catalog; not executed"}
}

func surfaceMatchesFixture(requested, fixture string) bool {
	switch requested {
	case "command":
		return fixture == "cli"
	case "configuration":
		return fixture == "config"
	case "api":
		return fixture == "rest" || fixture == "mcp"
	default:
		return false
	}
}

func validVerdict(value string) bool {
	return value == "supported" || value == "contradicted" || value == "not-determinable"
}

func emptyConfusionMatrix() map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, expected := range []string{"supported", "contradicted", "not-determinable"} {
		out[expected] = map[string]int{"supported": 0, "contradicted": 0, "not-determinable": 0}
	}
	return out
}

func reviewVariance(runs []ReviewRun) bool {
	if len(runs) < 2 {
		return false
	}
	first := runs[0].Verdict + "\x00" + runs[0].BlindExplanation
	for _, run := range runs[1:] {
		if run.Verdict+"\x00"+run.BlindExplanation != first {
			return true
		}
	}
	return false
}

func containsVerdict(runs []ReviewRun, verdict string) bool {
	for _, run := range runs {
		if run.Verdict == verdict {
			return true
		}
	}
	return false
}

func evidenceHash(item FragmentReview) string {
	copyValue := item
	copyValue.EvidenceSHA = ""
	copyValue.Disposition = nil
	bytes, _ := json.Marshal(copyValue)
	return digest(string(bytes))
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func readStrictJSON(path string, dst any) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(bytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func SortFixtures(catalog *FixtureCatalog) {
	sort.Slice(catalog.Examples, func(i, j int) bool { return catalog.Examples[i].ID < catalog.Examples[j].ID })
	sort.Slice(catalog.Journeys, func(i, j int) bool { return catalog.Journeys[i].ID < catalog.Journeys[j].ID })
}
