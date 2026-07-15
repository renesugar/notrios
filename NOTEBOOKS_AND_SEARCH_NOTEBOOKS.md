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

Built-in search notebooks (cannot be deleted in the UI or via the API):

| Name | Position | Query semantics | Notes |
|---|---|---|---|
| **All notes** | first in the sidebar | all non-deleted notes across all notebooks | default view on startup; uses incremental (cursor) search so startup never loads hundreds of thousands of notes at once |
| **Trash** | last in the sidebar | all notes marked deleted | see Trash semantics below |
| **Help** | normal position | `notebook:help` documentation notes | read-only; its notes cannot be edited or deleted by the user; content is seeded from `docs/` (see `DOCS_SITE.md`) |

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
- **Permanent deletion is only allowed for notes stored purely in the local database.** Notes imported from external sources that are marked deleted are simply excluded from queries and results — useful when exporting query results instead of the entire database (see `IMPORT_EXPORT_POLICY.md` for export/import and the dry-run import configuration flow).

## Query addressing

- `notebook:"notebook name"` (or `notebook:help` for single-word names) limits a search to a notebook. Matching is case-insensitive.
- If several sibling trees contain the same notebook name, the filter matches all of them; a path form (`notebook:"parent/child"`) may be added when nesting-disambiguation is needed.

## Protection rules summary

- "All notes" and "Trash": cannot be deleted, fixed first/last sidebar positions.
- "Help": cannot be deleted; notes inside are read-only.
- User search notebooks: deletable (notebook + query only).
- Regular notebooks: deletable per normal rules (future task defines whether their notes move to a parent, default notebook, or Trash — current decision: notes move to Trash).
