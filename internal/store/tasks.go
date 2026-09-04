package store

// Task extraction (v0.6 F4).
//
// A task is a checkbox list item — `- [ ] thing` or `- [x] thing`. They are
// **computed on read** from the note bodies rather than stored in a table: no
// new schema until a query needs one that a scan cannot serve, which is the
// decision this slice was given.
//
// A task's identity is the v0.5 block model's, not a line number, so a task
// keeps its identity when the note is edited *around* it. The limit of that is
// recorded honestly in TaskListRequest: a block ID is derived from the block's
// text, and a checkbox is part of that text, so **ticking a task changes its
// derived ID**. An author-written `^marker` is the identity that survives it,
// exactly as it is for any other block.

// Task ceilings. Extraction is a whole-library read, like lint, so it is a
// command someone runs rather than something that happens on every save.
const (
	// MaxTaskRows bounds the rows returned. Counts are always complete, so a
	// truncated list still tells the truth about how much there is — the same
	// contract lint's capped examples have.
	MaxTaskRows     = 500
	DefaultTaskRows = 100
	// MaxTaskDocuments bounds how many notes one extraction reads.
	MaxTaskDocuments = 20000
)

// Task states.
const (
	TaskStateOpen = "open"
	TaskStateDone = "done"
)

// Task is one checkbox item.
type Task struct {
	DocumentID    string `json:"document_id"`
	DocumentTitle string `json:"document_title"`
	NotebookID    string `json:"notebook_id,omitempty"`
	// BlockID is the content-derived block identity. It changes when the task's
	// own text changes, including when the checkbox is ticked.
	BlockID string `json:"block_id"`
	// Marker is the author-written `^marker`, when there is one. It is the
	// identity that survives ticking the box.
	Marker  string `json:"marker,omitempty"`
	Ordinal int    `json:"ordinal"`
	State   string `json:"state"`
	Text    string `json:"text"`
	// URI addresses the task itself: the note plus a block anchor.
	URI string `json:"uri"`
}

// TaskListRequest selects which tasks to report.
type TaskListRequest struct {
	CollectionID string `json:"collection_id,omitempty"`
	// DocumentID restricts extraction to one note, which turns a whole-library
	// read into a single-note one.
	DocumentID string `json:"document_id,omitempty"`
	NotebookID string `json:"notebook_id,omitempty"`
	// State is "open", "done", or empty for both.
	State string `json:"state,omitempty"`
	Limit int    `json:"limit,omitempty"`
	// Untagged includes notes that carry no task tag.
	//
	// Off by default, because a checkbox is ordinary Markdown: it appears in
	// quoted text, in code samples, in a note about how to write a checklist.
	// Treating every one of them as work turns the task list into a report on
	// the library's punctuation. Tagging a note says "the boxes in here are
	// mine", which is a decision only its author can make.
	//
	// The escape hatch exists because the opposite failure is worse in a
	// different way: work written down and then invisible because a tag was
	// forgotten. Asking for everything is one flag, and the counts say how much
	// more it found.
	Untagged bool `json:"untagged,omitempty"`
}

// TaskTags are the tags that mark a note as carrying tasks.
//
// Two rather than one because both are in common use and neither is obviously
// the right one; matching is case-insensitive, as tag names are throughout.
// Hierarchy counts: a note tagged `todo/survey` is tagged for tasks.
var TaskTags = []string{"task", "todo"}

// TaskList is the report.
type TaskList struct {
	Tasks []Task `json:"tasks"`
	// OpenCount and DoneCount describe everything found, not everything
	// returned. A capped list that also capped its counts would understate the
	// library, which is the failure lint's contract exists to avoid.
	OpenCount int  `json:"open_count"`
	DoneCount int  `json:"done_count"`
	Truncated bool `json:"truncated"`
	// DocumentsScanned says how much work this cost, and DocumentsTruncated
	// says the scan itself hit its ceiling — a different thing from the row
	// list being capped.
	DocumentsScanned   int  `json:"documents_scanned"`
	DocumentsTruncated bool `json:"documents_truncated"`
}
