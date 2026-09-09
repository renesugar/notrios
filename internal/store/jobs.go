package store

import (
	"fmt"
	"strings"
	"time"
)

// The job control plane (v0.6 F6, extended for sync by v0.7 G15).
//
// A job record is a durable answer to "what happened to the long thing I
// started". The base record is **not** a scheduler or general queue:
//
//   - **Records survive a restart; the work does not.** An interrupted import
//     already resumes through its own durable checkpoints, and a second resume
//     mechanism layered on top would give two answers to one question. The
//     record says the run stopped; running the same import again continues it.
//     G15's closed sync-only companion is the narrow exception: its row is a
//     durable outbox and its phase checkpoint points back to canonical vectors
//     and verified chunks. It still has no arbitrary command, priority,
//     dependency, or cadence.
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
	JobKindImportJoplinRaw   = "import_joplin_raw"
	JobKindImportObsidian    = "import_obsidian"
	JobKindExportArchiveV2   = "export_archive_v2"
	JobKindSnapshotImage     = "snapshot_image"
	JobKindSyncIncremental   = "sync_incremental"
	JobKindSyncResourceFetch = "sync_resource_fetch"
	JobKindSyncCatchup       = "sync_catchup"
	JobKindSyncRestorePrep   = "sync_restore_prep"
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

	// Sync jobs are a deliberately closed, bounded durable outbox. These limits
	// keep its checkpoint and audit metadata content-free and cheap to expose.
	MaxSyncJobCheckpointBytes = 4096
	MaxSyncJobTargetBytes     = 128
	MaxSyncJobAttempts        = 32
	DefaultSyncJobAttempts    = 8
	MaxSyncJobAuditRows       = 200
	DefaultSyncJobAuditRows   = 50
	MaxSyncJobByteBudget      = int64(16 << 30)
	DefaultSyncJobByteBudget  = int64(64 << 20)
)

// JobKinds lists the kinds this build knows how to run.
func JobKinds() []string {
	return []string{
		JobKindImportJoplinRaw, JobKindImportObsidian, JobKindExportArchiveV2, JobKindSnapshotImage,
		JobKindSyncIncremental, JobKindSyncResourceFetch,
	}
}

// IsSyncJobKind reports whether a kind belongs to G15's durable sync outbox.
func IsSyncJobKind(kind string) bool {
	switch kind {
	case JobKindSyncIncremental, JobKindSyncResourceFetch, JobKindSyncCatchup, JobKindSyncRestorePrep:
		return true
	default:
		return false
	}
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

// Sync job actors are provenance, not identities or authorization roles.
const (
	SyncJobActorCLI     = "cli"
	SyncJobActorREST    = "rest"
	SyncJobActorMCP     = "mcp"
	SyncJobActorService = "service"
)

// SyncJob is the sync-only extension of one ordinary Job record. TargetID is
// an opaque, stable digest chosen by the configured target adapter; it is never
// a directory, URL, credential reference, key, or peer secret.
type SyncJob struct {
	Job           Job            `json:"job"`
	Actor         string         `json:"actor"`
	TargetID      string         `json:"target_id"`
	Attempt       int            `json:"attempt"`
	MaxAttempts   int            `json:"max_attempts"`
	NextAttemptAt time.Time      `json:"next_attempt_at,omitempty"`
	RetryCode     string         `json:"retry_code,omitempty"`
	ByteBudget    int64          `json:"byte_budget"`
	BytesUsed     int64          `json:"bytes_used"`
	Checkpoint    map[string]any `json:"checkpoint,omitempty"`
	LeaseOwner    string         `json:"-"`
}

type CreateSyncJobRequest struct {
	Kind        string
	Actor       string
	TargetID    string
	ByteBudget  int64
	MaxAttempts int
}

type SyncJobCheckpoint struct {
	Phase      string
	Processed  int64
	Total      int64
	BytesUsed  int64
	Checkpoint map[string]any
}

// SyncJobAuditEvent is content-free operational evidence. Details are bounded
// counts/codes only; callers must never place paths, URLs, keys, or note text in
// them.
type SyncJobAuditEvent struct {
	ID        string         `json:"id"`
	JobID     string         `json:"job_id"`
	EventType string         `json:"event_type"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type SyncJobAuditList struct {
	Events    []SyncJobAuditEvent `json:"events"`
	Truncated bool                `json:"truncated,omitempty"`
}

// SyncConflictSummary is the deliberately content-free conflict view exposed
// by G15. It names stable records and revision IDs but never document titles,
// bodies, or stored conflict-region text.
type SyncConflictSummary struct {
	ID             string `json:"id"`
	DocumentID     string `json:"document_id"`
	BaseRevisionID string `json:"base_revision_id,omitempty"`
	RevisionA      string `json:"revision_a"`
	RevisionB      string `json:"revision_b"`
	Kind           string `json:"kind"`
	CreatedAt      string `json:"created_at"`
}

type SyncConflictPage struct {
	Conflicts []SyncConflictSummary `json:"conflicts"`
	Truncated bool                  `json:"truncated,omitempty"`
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
