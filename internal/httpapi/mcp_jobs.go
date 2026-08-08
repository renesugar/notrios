package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// Job tools for MCP: watching only, and content-free (v0.6 F6).
//
// **A model may not start a job.** Every kind this build knows how to run — the
// two importers and archive export — takes a filesystem path, and the standing
// decision since v0.4 is that no MCP surface accepts one. Wrapping an operation
// in a job record does not change what the operation does, so `start` would
// reopen exactly the hole F3 recorded a reason for closing.
//
// **A model may not cancel one either.** Cancelling is safe for the data, but a
// model deciding to stop a four-hour import a person started is not a decision
// it should be making. REST and the CLI both offer cancel to whoever is
// driving them.

// mcpJobView is the narrowed job a model sees.
//
// Two fields are dropped relative to REST, both because they routinely contain
// a local filesystem path: the **parameters** (never returned outside the CLI
// at all) and the free-text **error**, since a failure from a filesystem
// operation reads like `open /home/someone/private/x.md: permission denied`. A
// model that needs the reason has a person to ask, and the ID to ask about.
func mcpJobView(job api.JobStatus) map[string]any {
	view := map[string]any{
		"id":        job.ID,
		"kind":      job.Kind,
		"state":     job.State,
		"processed": job.Processed,
		"total":     job.Total,
		"settled":   job.Settled,
	}
	if job.Phase != "" {
		view["phase"] = job.Phase
	}
	if job.CreatedAt != "" {
		view["created_at"] = job.CreatedAt
	}
	if job.FinishedAt != "" {
		view["finished_at"] = job.FinishedAt
	}
	if len(job.Summary) > 0 {
		// A summary is counts and durations by construction — the CLI builds it
		// — so it crosses. Nothing in it is note text or a path.
		view["summary"] = job.Summary
	}
	if job.State == store.JobFailed {
		view["failed"] = true
		view["detail_available_locally"] = "notriosctl jobs show " + job.ID
	}
	return view
}

func (s *Server) mcpGetJob(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		JobID string `json:"job_id"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	job, err := s.store.GetJob(r.Context(), args.JobID)
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(mcpJobView(toAPIJob(job)))
}

func (s *Server) mcpListJobs(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		Kind  string `json:"kind,omitempty"`
		State string `json:"state,omitempty"`
		Limit int    `json:"limit,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	list, err := s.store.ListJobs(r.Context(), store.JobListRequest{
		Kind: args.Kind, State: args.State, Limit: args.Limit,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	views := make([]map[string]any, 0, len(list.Jobs))
	for _, job := range list.Jobs {
		views = append(views, mcpJobView(toAPIJob(job)))
	}
	return mcpStructured(map[string]any{"jobs": views, "truncated": list.Truncated})
}
