package store

import (
	"fmt"
	"strings"
	"time"
)

// The job control plane (v0.6 F6).
//
// A job record is a durable answer to "what happened to the long thing I
// started". It is **not** a scheduler, not a queue, and not a work-resumption
// mechanism:
//
//   - **Records survive a restart; the work does not.** An interrupted import
//     already resumes through its own durable checkpoints, and a second resume
//     mechanism layered on top would give two answers to one question. The
//     record says the run stopped; running the same import again continues it.
//   - **No dependencies and no DAG.** Status is queryable by ID and legible to
//     a shell — `notriosctl jobs status <id>` exits non-zero unless the job
//     succeeded, and `--wait` blocks until it settles — which is enough for
//     `job-a && job-b`. Sequencing stays in the caller's script.

// Job states. The first two are live; the last three are settled, meaning the
// record will not change again.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
	// JobInterrupted is **derived, never stored**. A process that dies cannot
	// write its own epitaph, so a job left in `running` with a heartbeat older
	// than JobHeartbeatTimeout is reported this way. Storing it would need a
	// writer to decide another process is dead, and two Notrios processes
	// against one database would take turns declaring each other's work over.
	JobInterrupted = "interrupted"
)

// Job kinds. Closed, like the template vocabulary: a kind names a specific
// operation with specific parameters, and an open set would make
// `jobs show --command` a guess.
const (
	JobKindImportJoplinRaw = "import_joplin_raw"
	JobKindImportObsidian  = "import_obsidian"
	JobKindExportArchiveV2 = "export_archive_v2"
	JobKindSnapshotImage   = "snapshot_image"
)

const (
	// JobHeartbeatInterval is how often a running job touches its record. It is
	// written at durable batch boundaries, so it costs one small update per
	// batch rather than a timer.
	JobHeartbeatInterval = 5 * time.Second
	// JobHeartbeatTimeout is how long a silent `running` job is believed. It is
	// generous on purpose: one slow batch on a large library must not make a
	// healthy job look dead, and the only cost of being late is that a watcher
	// waits a little longer to learn the truth.
	JobHeartbeatTimeout = 2 * time.Minute
	// MaxJobParameterBytes bounds the stored parameter map.
	MaxJobParameterBytes = 8192
	// MaxJobSummaryBytes bounds the stored result summary. A summary is counts
	// and durations; a report is what the command prints.
	MaxJobSummaryBytes = 8192
	// MaxJobErrorBytes bounds a stored failure message.
	MaxJobErrorBytes = 2048
	// MaxJobRows and DefaultJobRows bound a listing.
	MaxJobRows     = 500
	DefaultJobRows = 50
)

// JobKinds lists the kinds this build knows how to run.
func JobKinds() []string {
	return []string{JobKindImportJoplinRaw, JobKindImportObsidian, JobKindExportArchiveV2, JobKindSnapshotImage}
}

// IsSettledJobState reports whether a state will not change again.
func IsSettledJobState(state string) bool {
	switch state {
	case JobSucceeded, JobFailed, JobCancelled, JobInterrupted:
		return true
	default:
		return false
	}
}

// JobParameter is one stored input, tagged with whether its value is a local
// filesystem path.
//
// The tag exists because of what a job record is for. Reproducing a run needs
// the path; showing a job over REST or MCP must not disclose one. Storing raw
// argv would have made that impossible to separate after the fact — and would
// have captured any secret that happened to be on the command line — which is
// why parameters are stored and the command is *rendered* from them.
type JobParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Path marks a value that names a location on this machine.
	Path bool `json:"path,omitempty"`
}

// Job is one run of a long operation.
type Job struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	State        string `json:"state"`
	CollectionID string `json:"collection_id,omitempty"`

	// Phase, Processed, and Total are the last progress report. Total is zero
	// when the operation cannot know it in advance, which is honest rather than
	// a fabricated denominator.
	Phase     string `json:"phase,omitempty"`
	Processed int64  `json:"processed"`
	Total     int64  `json:"total"`

	// Summary is a bounded, content-free result — counts and durations, never
	// note text and never a path.
	Summary map[string]any `json:"summary,omitempty"`
	// Error is the failure message. It may contain a local path, so the MCP
	// view omits it; see JobPublicView.
	Error string `json:"error,omitempty"`

	// Parameters reproduce the run. Local-only: never returned over REST or
	// MCP, only rendered by `notriosctl jobs show --command`.
	Parameters []JobParameter `json:"parameters,omitempty"`

	CancelRequested bool      `json:"cancel_requested,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	StartedAt       time.Time `json:"started_at,omitempty"`
	FinishedAt      time.Time `json:"finished_at,omitempty"`
	HeartbeatAt     time.Time `json:"heartbeat_at,omitempty"`
}

// Settled reports whether the job will not change again.
func (j Job) Settled() bool { return IsSettledJobState(j.State) }

// CreateJobRequest starts a record. The work has not begun.
type CreateJobRequest struct {
	Kind         string
	CollectionID string
	Parameters   []JobParameter
}

// JobListRequest pages a listing, newest first.
type JobListRequest struct {
	Kind  string
	State string
	Limit int
}

// JobList is a bounded listing.
type JobList struct {
	Jobs      []Job `json:"jobs"`
	Truncated bool  `json:"truncated,omitempty"`
}

// JobProgress is one report from a running job.
type JobProgress struct {
	Phase     string
	Processed int64
	Total     int64
}

// validate normalizes a creation request.
func (r *CreateJobRequest) validate() error {
	r.Kind = strings.TrimSpace(r.Kind)
	known := false
	for _, kind := range JobKinds() {
		if kind == r.Kind {
			known = true
		}
	}
	if !known {
		return fmt.Errorf("%w: job kind must be one of %s", ErrInvalidInput, strings.Join(JobKinds(), ", "))
	}
	if strings.TrimSpace(r.CollectionID) == "" {
		r.CollectionID = "default"
	}
	for _, parameter := range r.Parameters {
		if strings.TrimSpace(parameter.Name) == "" {
			return fmt.Errorf("%w: a job parameter needs a name", ErrInvalidInput)
		}
	}
	return nil
}
