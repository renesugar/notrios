package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

type syncControlRequest struct {
	Kind        string `json:"kind,omitempty"`
	ByteBudget  int64  `json:"byte_budget,omitempty"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
}

func (s *Server) handlePlanSyncJob(w http.ResponseWriter, r *http.Request) {
	if !s.requireSyncJobs(w) {
		return
	}
	req, ok := decodeSyncControl(w, r)
	if !ok {
		return
	}
	kind, err := publicSyncKind(req.Kind)
	if err != nil {
		writeStoreError(w, err, "sync_plan_invalid")
		return
	}
	plan, err := s.syncJobs.Plan(r.Context(), s.syncTargetID, kind, req.ByteBudget)
	if writeStoreError(w, err, "sync_plan_failed") {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) handleStartSyncJob(w http.ResponseWriter, r *http.Request) {
	if !s.requireSyncJobs(w) {
		return
	}
	req, ok := decodeSyncControl(w, r)
	if !ok {
		return
	}
	kind, err := publicSyncKind(req.Kind)
	if err != nil {
		writeStoreError(w, err, "sync_start_invalid")
		return
	}
	job, err := s.syncJobs.Start(r.Context(), store.CreateSyncJobRequest{
		Kind: kind, Actor: store.SyncJobActorREST, TargetID: s.syncTargetID,
		ByteBudget: req.ByteBudget, MaxAttempts: req.MaxAttempts,
	})
	if writeStoreError(w, err, "sync_start_failed") {
		return
	}
	writeJSON(w, http.StatusAccepted, syncJobView(job))
}

func (s *Server) handleSyncJob(w http.ResponseWriter, r *http.Request) {
	if !s.requireSyncJobs(w) {
		return
	}
	job, err := s.store.GetSyncJob(r.Context(), r.PathValue("job_id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no such sync job")
			return
		}
		writeStoreError(w, err, "sync_job_read_failed")
		return
	}
	audit, err := s.store.ListSyncJobAudit(r.Context(), job.Job.ID, store.DefaultSyncJobAuditRows)
	if writeStoreError(w, err, "sync_job_audit_failed") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync_job": syncJobView(job), "audit": audit})
}

func (s *Server) handleRetrySyncJob(w http.ResponseWriter, r *http.Request) {
	s.handleRetryOrResetSyncJob(w, r, false)
}

func (s *Server) handleResetSyncJob(w http.ResponseWriter, r *http.Request) {
	s.handleRetryOrResetSyncJob(w, r, true)
}

func (s *Server) handleRetryOrResetSyncJob(w http.ResponseWriter, r *http.Request, reset bool) {
	if !s.requireSyncJobs(w) {
		return
	}
	job, err := s.store.RetrySyncJob(r.Context(), r.PathValue("job_id"), reset, time.Now())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no such sync job")
			return
		}
		writeStoreError(w, err, "sync_job_retry_failed")
		return
	}
	s.syncJobs.StartWake()
	writeJSON(w, http.StatusOK, syncJobView(job))
}

func (s *Server) handleSyncConflicts(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "sync conflicts require the canonical store")
		return
	}
	limit := boundedQueryLimit(r, 50, 200)
	if limit < 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
		return
	}
	page, err := s.store.ListSyncConflicts(r.Context(), limit)
	if writeStoreError(w, err, "sync_conflicts_failed") {
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) requireSyncJobs(w http.ResponseWriter) bool {
	if s.store == nil || s.syncJobs == nil || s.syncTargetID == "" {
		writeError(w, http.StatusServiceUnavailable, "sync_target_unavailable", "no runnable sync target is configured")
		return false
	}
	return true
}

func decodeSyncControl(w http.ResponseWriter, r *http.Request) (syncControlRequest, bool) {
	req := syncControlRequest{}
	if r.ContentLength != 0 {
		if err := decodeBoundedJSONBodyWithPolicy(w, r, &req, 16<<10, true); err != nil {
			if errors.Is(err, errRequestBodyTooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "sync-control request exceeds its JSON limit")
				return syncControlRequest{}, false
			}
			writeError(w, http.StatusBadRequest, "validation_failed", "request must be bounded sync-control JSON")
			return syncControlRequest{}, false
		}
	}
	if req.ByteBudget == 0 {
		req.ByteBudget = store.DefaultSyncJobByteBudget
	}
	return req, true
}

func publicSyncKind(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "incremental", store.JobKindSyncIncremental:
		return store.JobKindSyncIncremental, nil
	case "resource_fetch", store.JobKindSyncResourceFetch:
		return store.JobKindSyncResourceFetch, nil
	default:
		return "", fmt.Errorf("%w: kind must be incremental or resource_fetch", store.ErrInvalidInput)
	}
}

func boundedQueryLimit(r *http.Request, fallback, maximum int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return -1
	}
	if value > maximum {
		return maximum
	}
	return value
}

func syncJobView(job store.SyncJob) map[string]any {
	return map[string]any{
		"job": toAPIJob(job.Job), "actor": job.Actor, "target_id": job.TargetID,
		"attempt": job.Attempt, "max_attempts": job.MaxAttempts,
		"next_attempt_at": formatJobTime(job.NextAttemptAt), "retry_code": job.RetryCode,
		"byte_budget": job.ByteBudget, "bytes_used": job.BytesUsed, "checkpoint": job.Checkpoint,
	}
}
