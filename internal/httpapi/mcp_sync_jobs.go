package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

func (s *Server) mcpGetSyncStatus(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		JobID string `json:"job_id,omitempty"`
		Limit int    `json:"limit,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	if args.JobID != "" {
		job, err := s.store.GetSyncJob(r.Context(), args.JobID)
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(syncJobView(job))
	}
	limit := clampMCPLimit(args.Limit, 20)
	list, err := s.store.ListJobs(r.Context(), store.JobListRequest{Limit: store.MaxJobRows})
	if err != nil {
		return mcpToolResult{}, err
	}
	jobs := make([]map[string]any, 0, limit)
	for _, job := range list.Jobs {
		if !store.IsSyncJobKind(job.Kind) {
			continue
		}
		syncJob, getErr := s.store.GetSyncJob(r.Context(), job.ID)
		if getErr != nil {
			return mcpToolResult{}, getErr
		}
		jobs = append(jobs, syncJobView(syncJob))
		if len(jobs) == limit {
			break
		}
	}
	return mcpStructured(map[string]any{
		"target_id": s.syncTargetID, "available": s.syncJobs != nil, "jobs": jobs,
	})
}

func (s *Server) mcpListSyncConflicts(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		Limit int `json:"limit,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructuredResult(s.store.ListSyncConflicts(r.Context(), clampMCPLimit(args.Limit, 50)))
}

func (s *Server) mcpPlanSync(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	if s.syncJobs == nil {
		return mcpToolResult{}, fmt.Errorf("no runnable sync target is configured")
	}
	var args struct {
		ByteBudget int64 `json:"byte_budget,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	if args.ByteBudget == 0 {
		args.ByteBudget = store.DefaultSyncJobByteBudget
	}
	plan, err := s.syncJobs.Plan(r.Context(), s.syncTargetID, store.JobKindSyncIncremental, args.ByteBudget)
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(plan)
}

func (s *Server) mcpStartSync(r *http.Request, raw json.RawMessage, kind string) (mcpToolResult, error) {
	if s.syncJobs == nil {
		return mcpToolResult{}, fmt.Errorf("no runnable sync target is configured")
	}
	var args struct {
		ByteBudget  int64 `json:"byte_budget,omitempty"`
		MaxAttempts int   `json:"max_attempts,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	if args.ByteBudget == 0 {
		args.ByteBudget = store.DefaultSyncJobByteBudget
	}
	job, err := s.syncJobs.Start(r.Context(), store.CreateSyncJobRequest{
		Kind: kind, Actor: store.SyncJobActorMCP, TargetID: s.syncTargetID,
		ByteBudget: args.ByteBudget, MaxAttempts: args.MaxAttempts,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(syncJobView(job))
}

func (s *Server) mcpRetrySyncJob(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	job, err := s.mcpOwnedSyncJob(r, raw)
	if err != nil {
		return mcpToolResult{}, err
	}
	retried, err := s.store.RetrySyncJob(r.Context(), job.Job.ID, false, time.Now())
	if err != nil {
		return mcpToolResult{}, err
	}
	if s.syncJobs != nil {
		s.syncJobs.StartWake()
	}
	return mcpStructured(syncJobView(retried))
}

func (s *Server) mcpCancelSyncJob(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	job, err := s.mcpOwnedSyncJob(r, raw)
	if err != nil {
		return mcpToolResult{}, err
	}
	cancelled, err := s.store.RequestJobCancel(r.Context(), job.Job.ID)
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(mcpJobView(toAPIJob(cancelled)))
}

func (s *Server) mcpOwnedSyncJob(r *http.Request, raw json.RawMessage) (store.SyncJob, error) {
	var args struct {
		JobID string `json:"job_id"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return store.SyncJob{}, err
	}
	job, err := s.store.GetSyncJob(r.Context(), args.JobID)
	if err != nil {
		return store.SyncJob{}, err
	}
	if job.Actor != store.SyncJobActorMCP ||
		(job.Job.Kind != store.JobKindSyncIncremental && job.Job.Kind != store.JobKindSyncResourceFetch) {
		return store.SyncJob{}, fmt.Errorf("MCP may control only its own ordinary incremental or resource-fetch jobs")
	}
	return job, nil
}

func mcpStructuredResult[T any](value T, err error) (mcpToolResult, error) {
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(value)
}
