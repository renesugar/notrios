# MCP endpoint

Notrios mounts an MCP (Model Context Protocol) endpoint at `/mcp` so LLM tools can search and read — and, when you allow it, edit — your notes.

## Profiles

The default profile is **read-only**. Set `mcp.default_profile: "editor"` in the service configuration to enable write tools; they are otherwise hidden from `tools/list` and rejected if called.

## Read tools

`search_documents`, `get_document`, `get_documents`, `list_collections`, `list_notebooks`, `get_notebook_tree`, `get_notebook_notes`, `list_tags`, `list_search_notebooks`, `list_document_links`, `list_document_resources`, `get_document_outline`, `get_note_line_range`, `search_in_note`.

Search accepts the same [query language](../query-language.md) as everywhere else and returns snippets plus `document://` URIs; retrieve full bodies explicitly with `get_document`.

## Write tools (editor profile)

`create_note`, `update_note` (requires `base_revision_id`), `append_to_note`, `prepend_to_note`, `edit_note` (server-side string replacement — fails when the search text is ambiguous unless `replace_all` is set; supports `dry_run`), `delete_note` (requires `base_revision_id`; moves to Trash), `move_note_to_notebook`.

## Safety rules

- No raw SQL and no filesystem access are ever exposed.
- Note bodies returned to models are untrusted data, not instructions.
- Destructive operations demand revision preconditions, so a stale model can't clobber newer edits.
