package store

// Organizer operations: the destructive-shaped edits to a library's structure
// rather than to its notes.
//
// Two rules shape them, and they are the same two `PROJECT_DECISIONS.md` 18
// gave fix.
//
// **Nothing destructive happens without a report first.** A hierarchical tag
// rename runs as a dry run by default and reports every tag it would touch,
// with counts; deleting a notebook can be previewed before it is done. The
// report is produced by the code that does the work, not by a second
// implementation that predicts it — see RenameTag.
//
// **They stay single operations, not batches.** One rename, one notebook. A
// rename does touch many tags because a hierarchy is one thing, but there is
// no way here to submit a list of unrelated organizer edits and have them
// commit together. That is the v0.6 organizer's job and this slice must not
// pre-empt it.

// TagHierarchySeparator is the character that makes a tag name a path.
// `project/alpha` is a child of `project`; `projects` is not.
const TagHierarchySeparator = "/"

// MaxTagRenameTags bounds how many tags one rename may touch. A rename is one
// operation over a hierarchy, not a bulk edit, and a report nobody can read is
// not a report. Beyond the bound the rename is refused, not truncated: a
// half-renamed hierarchy is worse than no rename.
const MaxTagRenameTags = 500

// Tag rename actions.
const (
	// TagRenameActionRename means the destination name was free.
	TagRenameActionRename = "rename"
	// TagRenameActionMerge means a tag with the destination name already
	// existed, so the source's notes join it and the source tag disappears.
	TagRenameActionMerge = "merge"
)

// TagRenameRequest renames a tag, optionally carrying its children with it.
//
// DryRun is the safe direction: callers that forget it get a report rather
// than a change. The REST and CLI surfaces both default it to true.
type TagRenameRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
	// IncludeChildren also renames every tag under `From/`, keeping the
	// hierarchy's shape below the renamed prefix.
	IncludeChildren bool `json:"include_children,omitempty"`
	DryRun          bool `json:"dry_run,omitempty"`
}

// TagRenameChange is one tag's outcome, reported identically whether the run
// was a dry run or an apply.
type TagRenameChange struct {
	TagID  string `json:"tag_id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Action string `json:"action"`
	// MergedIntoTagID is the surviving tag when Action is merge.
	MergedIntoTagID string `json:"merged_into_tag_id,omitempty"`
	// Notes counts the current (non-trashed) notes carrying the source tag.
	Notes int64 `json:"notes"`
	// NotesGained is how many of those notes do not already carry the
	// destination tag, so a merge reports what actually changes for a reader
	// rather than only how big the source was. It equals Notes for a rename.
	NotesGained int64 `json:"notes_gained"`
}

// TagRenameResult is the whole run.
type TagRenameResult struct {
	From            string            `json:"from"`
	To              string            `json:"to"`
	IncludeChildren bool              `json:"include_children"`
	DryRun          bool              `json:"dry_run"`
	Changes         []TagRenameChange `json:"changes"`
	// Notes counts the distinct current notes the run touches.
	Notes int64 `json:"notes"`
	// Warnings name things the rename deliberately does not do: children left
	// behind, saved searches left pointing at the old name.
	Warnings []string `json:"warnings"`
}

// NotebookDeletionPreview is what deleting a notebook would do, computed
// without doing it.
//
// Deleting a notebook is trash-first: no note is lost, but every note in the
// subtree moves to the Trash and is re-homed to the default notebook so a
// later restore always has somewhere to land. That rule is invisible from a
// confirmation dialog that only says "delete?", which is what this exists to
// fix.
type NotebookDeletionPreview struct {
	NotebookID string `json:"notebook_id"`
	Name       string `json:"name"`
	// Notebooks counts this notebook plus its descendants, all of which go.
	Notebooks int64 `json:"notebooks"`
	// DescendantNames lists the descendant notebooks by name, bounded by
	// MaxNotebookPreviewNames; Truncated says when the list was cut.
	DescendantNames []string `json:"descendant_names"`
	Truncated       bool     `json:"truncated"`
	// Notes counts the current notes that would move to the Trash.
	Notes int64 `json:"notes"`
	// TrashedNotes counts notes already in the Trash that would be re-homed.
	// They are already invisible, but they are what a later restore lands on.
	TrashedNotes int64 `json:"trashed_notes"`
	// RehomeNotebookID is where every note in the subtree ends up.
	RehomeNotebookID string `json:"rehome_notebook_id"`
	// Deletable is false when the notebook is protected; Reason says why.
	Deletable bool   `json:"deletable"`
	Reason    string `json:"reason,omitempty"`
}

// MaxNotebookPreviewNames bounds the descendant list in a deletion preview.
const MaxNotebookPreviewNames = 50
