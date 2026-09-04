//go:build gui

package main

import (
	"context"

	"github.com/renesugar/notrios/internal/store"
)

// Mechanical repairs, planned and then applied.
//
// This goes through the bridge for the same structural reason import does —
// there is no REST route — but the reasoning behind that absence is different
// and worth being exact about, because the two look alike from the interface.
//
// Import has no route because a filesystem path is not something the API
// accepts. Fixing has none because the lint surface is deliberately read-only:
// REST is a remote, programmatic surface that MCP agents and other programs
// reach, and repairs to somebody's notes should not be one call away from an
// agent. None of that argues against a person pressing a button in the program
// that owns the library, which is what the default mode is — the window and the
// store are one process.
//
// Garbage collection stays on the command line, and not because a boundary
// forbids it. A fix writes a revision against a required base revision, so it
// is versioned and refuses if the note moved underneath it; `gc --apply`
// deletes blobs, and there is no revision to go back to. That difference is the
// whole reason one of these is here and the other is not.

// FixPlanReport is a planned repair run, or the outcome of one.
type FixPlanReport struct {
	Plan    store.FixPlan          `json:"plan"`
	Applied bool                   `json:"applied"`
	Results []store.FixApplyResult `json:"results,omitempty"`
}

// PlanFixes reports what mechanical repairs would change, and changes nothing.
//
// The plan carries a base revision per note, which is what makes applying it
// later safe: a note edited between the plan and the apply fails its own
// precondition rather than being repaired against text nobody looked at.
func (b *NativeUIBridge) PlanFixes() (FixPlanReport, error) {
	if b == nil || b.local == nil {
		return FixPlanReport{}, errNoLocalService
	}
	plan, err := b.local.Store.PlanWorkspaceFix(context.Background(), store.FixRequest{})
	if err != nil {
		return FixPlanReport{}, err
	}
	return FixPlanReport{Plan: plan}, nil
}

// ApplyFixes repairs the notes in a plan, one at a time.
//
// Each note is its own transaction with its own precondition, so a failure on
// one does not abandon the rest and does not hide the successes behind it. The
// results say what happened to each, including which ones refused.
//
// The plan is re-computed here rather than taken from the caller. A plan that
// travelled to the frontend and back could have been edited on the way, and
// applying edits the service did not compute is exactly the thing a
// precondition cannot protect against. Notes changed since the interface drew
// the plan fail their own base-revision check, which is the correct outcome and
// is reported per note.
func (b *NativeUIBridge) ApplyFixes() (FixPlanReport, error) {
	if b == nil || b.local == nil {
		return FixPlanReport{}, errNoLocalService
	}
	ctx := context.Background()
	plan, err := b.local.Store.PlanWorkspaceFix(ctx, store.FixRequest{})
	if err != nil {
		return FixPlanReport{}, err
	}
	report := FixPlanReport{Plan: plan, Applied: true, Results: []store.FixApplyResult{}}
	for _, document := range plan.Documents {
		result, applyErr := b.local.Store.ApplyDocumentFix(ctx, document)
		if applyErr != nil {
			result.DocumentID = document.DocumentID
			result.Error = applyErr.Error()
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}
