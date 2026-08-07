package store

// Batch organizer transactions: the bulk half of the organizer, which v0.5
// deliberately left alone (`plans/v0.5/012-organizer-ux.md`).
//
// Three properties define it, and each is a decision about what a caller is
// owed when something goes wrong halfway.
//
// **A run reports every item, in both modes.** Atomic and best-effort differ in
// what they *do* on failure, not in what they say. A caller that gets "failed"
// with no per-item detail cannot tell which half happened.
//
// **A retry does the work once.** The key is stored, not remembered — a batch
// is retried exactly when something went wrong, and an in-memory map forgets
// precisely then.
//
// **Nothing here is a new way to reach a note.** Every operation is one the
// single-note surfaces already expose; a batch is a way to ask for many of them
// at once, not a way to ask for something else.

// Batch ceilings. A batch is bounded work with a readable report, not a job.
const (
	// MaxBatchItems bounds one request. Over it the request is refused rather
	// than truncated: a caller who asked for 900 notes and silently got 500
	// would have no way to know which 400 were left.
	MaxBatchItems = 500
	// MaxBatchTags bounds the tag list on one tag operation.
	MaxBatchTags = 50
	// MaxBatchRequestKeyBytes bounds the idempotency key.
	MaxBatchRequestKeyBytes = 200
)

// Batch modes.
const (
	// BatchModeAtomic applies every item or none. A single failure rolls the
	// whole run back, and the report still names the item that caused it.
	BatchModeAtomic = "atomic"
	// BatchModeBestEffort applies what it can, item by item, and reports each
	// outcome. This is the default: an organizer sweep over a hundred notes
	// should not be abandoned because one of them is protected.
	BatchModeBestEffort = "best_effort"
)

// Batch operations. Each maps to a single-note operation that already exists.
const (
	BatchOpMove       = "move"        // to another notebook
	BatchOpAddTags    = "add_tags"    //
	BatchOpRemoveTags = "remove_tags" //
	BatchOpTrash      = "trash"       // soft delete; needs a base revision
	BatchOpRestore    = "restore"     // from the Trash
	BatchOpDuplicate  = "duplicate"   //
)

// BatchOperations lists the supported operations in report order.
func BatchOperations() []string {
	return []string{BatchOpMove, BatchOpAddTags, BatchOpRemoveTags, BatchOpTrash, BatchOpRestore, BatchOpDuplicate}
}

// BatchItem is one note in a request.
//
// BaseRevisionID is the precondition, and it is per item rather than per
// request because a batch is a set of independent notes: one of them having
// moved on is not a reason to refuse the rest in best-effort mode. It is
// required only for operations that write a revision — today, trash.
type BatchItem struct {
	DocumentID     string `json:"document_id"`
	BaseRevisionID string `json:"base_revision_id,omitempty"`
}

// BatchRequest is one bounded organizer transaction.
type BatchRequest struct {
	// RequestKey makes a retry safe. Empty means "no idempotency", which is
	// honest rather than convenient: a caller that wants exactly-once has to
	// say so, and one that does not should not silently get replay semantics.
	RequestKey string      `json:"request_key,omitempty"`
	Operation  string      `json:"operation"`
	Mode       string      `json:"mode,omitempty"`
	Items      []BatchItem `json:"items"`
	// NotebookID is the destination for `move`.
	NotebookID string `json:"notebook_id,omitempty"`
	// Tags are the labels for `add_tags` and `remove_tags`.
	Tags []string `json:"tags,omitempty"`
}

// Batch item outcomes.
const (
	BatchStatusApplied = "applied"
	// BatchStatusSkipped is a no-op that was not a failure: a tag the note
	// already had, a note already in the notebook it was asked to move to.
	// Distinguishing it from "applied" is what makes a report honest about how
	// much actually changed.
	BatchStatusSkipped = "skipped"
	BatchStatusFailed  = "failed"
	// BatchStatusRolledBack marks an item that succeeded and was then undone
	// because a later item failed an atomic run. It is not "failed": nothing
	// was wrong with it.
	BatchStatusRolledBack = "rolled_back"
)

// BatchItemResult is one note's outcome.
type BatchItemResult struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
	// Reason explains a skip; Error explains a failure. They are separate
	// fields because conflating them makes "nothing to do" look like a fault.
	Reason string `json:"reason,omitempty"`
	Error  string `json:"error,omitempty"`
	// NewDocumentID is the copy produced by `duplicate`.
	NewDocumentID string `json:"new_document_id,omitempty"`
}

// BatchResult is the whole run.
type BatchResult struct {
	RequestKey string            `json:"request_key,omitempty"`
	Operation  string            `json:"operation"`
	Mode       string            `json:"mode"`
	Items      []BatchItemResult `json:"items"`
	Applied    int               `json:"applied"`
	Skipped    int               `json:"skipped"`
	Failed     int               `json:"failed"`
	RolledBack int               `json:"rolled_back"`
	// Replayed marks a response served from the idempotency ledger rather than
	// re-run. A caller that cannot tell a replay from a fresh run cannot tell
	// whether its retry was necessary.
	Replayed bool `json:"replayed"`
}

// NormalizeBatchRequest fills defaults. It does not validate; RunBatch does,
// because a rejected request should say what was wrong rather than be quietly
// repaired into something the caller did not ask for.
func NormalizeBatchRequest(req BatchRequest) BatchRequest {
	if req.Mode == "" {
		req.Mode = BatchModeBestEffort
	}
	return req
}
