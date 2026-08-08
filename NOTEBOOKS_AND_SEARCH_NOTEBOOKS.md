# Notebooks, Tags, and Search Notebooks

This document defines the Notrios notebook model. It is design-normative for schema task R3, API task R5, and the GUI tasks in `PLAN.md`.

## Notebooks

- Notebooks are nested (a notebook may have sub-notebooks), matching Joplin's UX (e.g. `Contacts` containing `Plumbers`, `Electricians`, `Carpenters`) so the experience is smooth for Joplin users.
- Every managed note belongs to exactly one notebook. The default notebook is **"Notes"**, created at bootstrap.
- Importers create source notebooks (e.g. "Twitter", "Bookmarks") and may reproduce the source's notebook hierarchy.
- An optional **emoji icon** can be associated with a notebook; the GUI displays it before the notebook name in the sidebar.
- Notebook names are **not case sensitive**: users cannot create two notebooks whose names differ only by case (enforced case-insensitively among siblings with a `NOCASE` unique index; the store layer rejects violations regardless of client).
- Notebook ordering in the sidebar: "All notes" first, then regular and user search notebooks, then "Trash" last.

## Tags

- Tags are stored in dedicated `tags`/`note_tags` tables (not only frontmatter).
- The sidebar lists tags with their note counts below the notebooks tree.
- `tag:"multi word"` searches are supported through the query language (`SEARCH_QUERY_LANGUAGE.md`).

## Search notebooks

A **search notebook** is a notebook whose contents are defined by a query instead of direct membership. Deleting a search notebook deletes only the notebook row and its query — never any notes.

Built-in search notebooks (created on a fresh database and not deletable through
the UI or API):

| Name | Position | Query semantics | Notes |
|---|---|---|---|
| **All notes** | first in the sidebar | all non-deleted notes across all notebooks (empty query) | default view on startup; uses incremental (cursor) search so startup never loads hundreds of thousands of notes at once |
| **Trash** | last in the sidebar | all notes marked deleted (reserved internal query `is:trashed`) | see Trash semantics below |

**Help** and **Reports** are implemented as built-in *regular* notebooks (not search notebooks). Help holds the read-only documentation notes seeded from `docs/` (see `DOCS_SITE.md`); Reports holds notes Notrios generates about the library itself, currently the graph hubs report (v0.6 F5). Neither can be deleted, the notes inside cannot be edited, deleted, or moved, and `notebook:help` searches Help like any notebook.

These two are the **read-only** builtin set, which the code asks for through
`store.IsReadOnlyNotebook(id)` rather than through `nb.Builtin`. The distinction
is load-bearing and the two sets are genuinely different:

| Set | Members | Test |
|---|---|---|
| Undeletable | Help, Reports, **Notes** | `nb.Builtin` **or** `id == DefaultNotebookID` |
| Read-only / system-authored | Help, Reports | `store.IsReadOnlyNotebook(id)` |

The default **Notes** notebook is bootstrap-created and undeletable, but it is
`builtin = 0` in the database because its content is the *user's*. Treating it
as read-only would silently exclude most of a library from the graph report,
from publication handoffs, and from lint. The predicate is named "read-only"
rather than "builtin" so that anyone who copies `DeleteNotebook`'s pair of checks
is contradicted by the name, and a test pins that `DefaultNotebookID` is not in
the set.

The fresh-database contract is therefore exactly five built-in navigation
entries:

- All notes — protected search notebook;
- Notes — protected default regular notebook;
- Reports — protected/read-only regular notebook;
- Help — protected/read-only regular notebook;
- Trash — protected search notebook.

Tests must create a database at a nonexistent path and assert all five IDs,
types, protection rules, queries, and sidebar anchors. A migration test asserts
the same contract without duplicating rows.

User-created search notebooks:

- Any saved query can become a search notebook (e.g. name **TODO** with query `tag:todo`).
- Users can delete their own search notebooks; only the notebook + query row is removed, no notes are touched.
- Search notebooks can have emoji icons like regular notebooks.

## Trash semantics

- Deleting a note marks it deleted (soft delete, already implemented in v0.1). Deleted notes:
  - do not appear in "All notes";
  - are excluded from all queries and query results;
  - appear only in the "Trash" search notebook.
- Undeleting a note in Trash makes it visible and searchable again.
- **Permanent deletion is currently only allowed for notes stored purely in the
  local database.** Notes imported from external sources that are marked
  deleted are simply excluded from queries/results. Under v0.7 sync, permanent
  delete emits a death certificate and removes payload bytes only after
  retention and active-peer acknowledgement rules permit it; see
  `SYNCHRONIZATION.md`.

## Query addressing

- `notebook:"notebook name"` (or `notebook:help` for single-word names) limits a search to a notebook. Matching is case-insensitive.
- If several sibling trees contain the same notebook name, the filter matches all of them; a path form (`notebook:"parent/child"`) may be added when nesting-disambiguation is needed.
- Implemented in v0.4 Q1: `category:` is an exact alias for `notebook:` for
  Twitter/Joplin-style search compatibility. `category:"All notes"` and
  `notebook:"All notes"` mean the builtin all-current-notes scope rather than a
  literal notebook named "All notes".

## Protection rules summary

- "All notes" and "Trash": cannot be deleted, fixed first/last sidebar positions.
- "Help" and "Reports": builtin regular notebooks; cannot be deleted, renamed, or moved; notes inside are read-only and cannot be moved in or out. Sidebar order is Reports, then Help, then Trash.
- The default "Notes" notebook cannot be deleted (it is the fallback home for restored notes) but is otherwise a normal notebook.
- User search notebooks: deletable (notebook + query only).
- Regular notebooks: deleting one (including its sub-notebooks) moves its notes to the Trash; nothing is lost. Trashed notes whose notebook was deleted are re-homed to the default "Notes" notebook so restore always has a valid destination.
