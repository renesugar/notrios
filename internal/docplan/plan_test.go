package docplan

import (
	"path/filepath"
	"strings"
	"testing"
)

// The checks are worth more than the format, so each one is exercised against
// a ledger that breaks it. A gate nobody has watched fail is a gate nobody
// knows the shape of.
func TestCheckRefusesEveryWayALedgerCanLie(t *testing.T) {
	root := repositoryRoot(t)
	headings := map[string]string{"H1": "A finished thing — complete", "H2": "An unfinished thing"}
	done := Slice{ID: "H2-A", Statement: "something", State: SliceDone,
		Evidence: "go:github.com/renesugar/notrios/internal/docplan#Check"}

	cases := []struct {
		name   string
		ledger Ledger
		want   string
	}{
		{"an item nobody planned", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}}}, "in PLAN.md and not in the ledger"},
		{"an item that left the plan", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemInProgress, Slices: []Slice{done, {ID: "H2-B", Statement: "x", State: SliceNotStarted}}},
			{ID: "H9", State: ItemNotStarted}}}, "in the ledger and not in PLAN.md"},
		{"a heading and a state that disagree", Ledger{Items: []Item{
			{ID: "H1", State: ItemInProgress, Slices: []Slice{{ID: "H1-A", Statement: "x", State: SliceNotStarted}}},
			{ID: "H2", State: ItemInProgress, Slices: []Slice{done, {ID: "H2-B", Statement: "x", State: SliceNotStarted}}}}},
			"heading says"},
		{"done without evidence", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemInProgress, Slices: []Slice{
				{ID: "H2-A", Statement: "x", State: SliceDone}, {ID: "H2-B", Statement: "y", State: SliceNotStarted}}}}},
			"done and names no evidence"},
		{"evidence that does not resolve", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemInProgress, Slices: []Slice{
				{ID: "H2-A", Statement: "x", State: SliceDone, Evidence: "file:no/such/file.go"},
				{ID: "H2-B", Statement: "y", State: SliceNotStarted}}}}},
			"names a path that is not there"},
		{"evidence for unfinished work", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemInProgress, Slices: []Slice{
				{ID: "H2-A", Statement: "x", State: SliceInProgress, Evidence: "file:PLAN.md"},
				{ID: "H2-B", Statement: "y", State: SliceNotStarted}}}}},
			"names evidence but is not done"},
		{"blocked without a reason", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemInProgress, Slices: []Slice{
				{ID: "H2-A", Statement: "x", State: SliceBlocked}}}}},
			"blocked without saying what on"},
		{"finished but calling itself in progress", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemInProgress, Slices: []Slice{done}}}},
			"every slice is done"},
		{"not started with work done", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemNotStarted, Slices: []Slice{done}}}},
			"not started with 1 slices done"},
		{"complete with work outstanding", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemComplete, Slices: []Slice{
				{ID: "H2-A", Statement: "x", State: SliceNotStarted}}}}},
			"complete with 1 slices outstanding"},
		{"a slice belonging to another item", Ledger{Items: []Item{
			{ID: "H1", State: ItemComplete}, {ID: "H2", State: ItemInProgress, Slices: []Slice{
				{ID: "H9-A", Statement: "x", State: SliceNotStarted}}}}},
			"does not belong to H2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := strings.Join(Check(root, tc.ledger, headings), "\n")
			if !strings.Contains(problems, tc.want) {
				t.Fatalf("expected a problem containing %q, got:\n%s", tc.want, problems)
			}
		})
	}
}

func TestEvidenceKindsResolve(t *testing.T) {
	root := repositoryRoot(t)
	for _, evidence := range []string{
		"go:github.com/renesugar/notrios/internal/docplan#Ledger",
		"file:PLAN.md",
		"test:TestEvidenceKindsResolve",
	} {
		if err := resolveEvidence(root, evidence); err != nil {
			t.Errorf("%s should resolve: %v", evidence, err)
		}
	}
	for _, evidence := range []string{"", "PLAN.md", "go:example.com/other#Thing", "test:TestNothingIsCalledThis"} {
		if err := resolveEvidence(root, evidence); err == nil {
			t.Errorf("%q should not resolve", evidence)
		}
	}
	_ = filepath.Join
}
