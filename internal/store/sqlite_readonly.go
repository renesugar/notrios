package store

import "strings"

// SQL fragments for "is this note part of the live library the user owns".
//
// Two independent axes decide that, and the code has historically applied one
// without the other:
//
//   - **Notebook.** A note in a read-only builtin notebook is system-authored.
//     Notrios' own Help documentation links to itself heavily; the graph report
//     was counting those links as evidence that a note mattered.
//   - **State.** A trashed note is read-only (v0.5 E8) and is not part of the
//     workspace. Soft delete deliberately **leaves `document_links` intact** so
//     a restore can use them, so "the row is gone" is not true of its links.
//
// Either filter alone is incomplete, which is why they live in one place rather
// than being remembered separately at each call site.

// measuredDocumentSQL is the predicate for a note that counts, for the given
// table alias: both axes at once. It expects the read-only notebook IDs bound
// in order, which readOnlyNotebookArgs supplies.
func measuredDocumentSQL(alias string) string {
	return alias + ".deleted_at IS NULL AND " + notSystemAuthoredSQL(alias)
}

// notSystemAuthoredSQL is the notebook axis alone, for callers that already
// filter on state — lint, for instance, which has always excluded trashed notes
// and never excluded notes nobody can edit.
func notSystemAuthoredSQL(alias string) string {
	return "COALESCE(" + alias + ".notebook_id, '') NOT IN (" + readOnlyNotebookPlaceholders() + ")"
}

// readOnlyNotebookPlaceholders renders one `?` per read-only notebook, so
// adding a third one needs no SQL edit anywhere.
func readOnlyNotebookPlaceholders() string {
	ids := ReadOnlyNotebookIDs()
	return strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
}

// readOnlyNotebookArgs binds the IDs the placeholders expect.
func readOnlyNotebookArgs() []string {
	return ReadOnlyNotebookIDs()
}
