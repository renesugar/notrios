package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// The job control plane over HTTP (v0.6 F6).
//
// Import/export/snapshot kinds remain watching-only because they name local
// paths. G15's separate sync routes may plan/start the two path-free ordinary
// sync kinds; they never make arbitrary job kinds remotely startable.

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "job records require the canonical store")
		return
	}
	job, err := s.store.GetJob(r.Context(), r.PathValue("job_id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no such job")
			return
		}
		writeStoreError(w, err, "job_read_failed")
		return
	}
	writeJSON(w, http.StatusOK, toAPIJob(job))
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "job records require the canonical store")
		return
	}
	req := store.JobListRequest{
		Kind:  strings.TrimSpace(r.URL.Query().Get("kind")),
		State: strings.TrimSpace(r.URL.Query().Get("state")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
			return
		}
		req.Limit = limit
	}
	list, err := s.store.ListJobs(r.Context(), req)
	if writeStoreError(w, err, "job_list_failed") {
		return
	}
	page := api.JobPage{Jobs: make([]api.JobStatus, 0, len(list.Jobs)), Truncated: list.Truncated}
	for _, job := range list.Jobs {
		page.Jobs = append(page.Jobs, toAPIJob(job))
	}
	writeJSON(w, http.StatusOK, page)
}

// handleCancelJob asks a running job to stop at its next durable boundary.
//
// A write, but a narrow one: it sets a flag on a record. It cannot reach the
// filesystem, cannot destroy anything, and the work it stops has already
// committed and checkpointed everything it finished.
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "job records require the canonical store")
		return
	}
	job, err := s.store.RequestJobCancel(r.Context(), r.PathValue("job_id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no such job")
			return
		}
		writeStoreError(w, err, "job_cancel_failed")
		return
	}
	// 200 rather than 202: the flag is set, which is the whole of what this
	// route promises. The job stopping is the job's business.
	writeJSON(w, http.StatusOK, toAPIJob(job))
}

func toAPIJob(job store.Job) api.JobStatus {
	status := api.JobStatus{
		ID:              job.ID,
		Kind:            job.Kind,
		State:           job.State,
		CollectionID:    job.CollectionID,
		Phase:           job.Phase,
		Processed:       job.Processed,
		Total:           job.Total,
		Summary:         job.Summary,
		Error:           job.Error,
		CancelRequested: job.CancelRequested,
		Settled:         job.Settled(),
		CreatedAt:       formatJobTime(job.CreatedAt),
		StartedAt:       formatJobTime(job.StartedAt),
		FinishedAt:      formatJobTime(job.FinishedAt),
		HeartbeatAt:     formatJobTime(job.HeartbeatAt),
	}
	return status
}

func formatJobTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
