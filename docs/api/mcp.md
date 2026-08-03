# MCP endpoint

Notrios mounts an MCP (Model Context Protocol) endpoint at `/mcp` so LLM tools can search and read — and, when you allow it, edit — your notes. The transport is JSON-RPC over HTTP POST supporting `initialize`, `tools/list`, and `tools/call`.

## Trying it with curl

```sh
curl -s -X POST http://127.0.0.1:8080/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '.result.tools[].name'

curl -s -X POST http://127.0.0.1:8080/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_documents","arguments":{"query":"(tag:todo OR category:Work) -tag:private","limit":5}}}' | jq
```

## Connecting an MCP client

For clients that speak HTTP-based MCP, point them at `http://127.0.0.1:8080/mcp`. For example, with Claude Code:

```sh
claude mcp add --transport http notrios http://127.0.0.1:8080/mcp
```

The adapter implements the core JSON-RPC methods rather than the full official SDK surface; if your client requires session negotiation beyond `initialize`, check compatibility first.

## Profiles

The default profile is **read-only**. Set `mcp.default_profile: "editor"` in the service configuration to enable write tools; they are otherwise hidden from `tools/list` and rejected if called.

## Read tools

`search_documents`, `plan_selection`, `get_document`, `get_documents`,
`list_collections`, `list_notebooks`, `get_notebook_tree`,
`get_notebook_notes`, `list_tags`, `list_search_notebooks`,
`list_document_links`, `list_document_resources`, `get_document_outline`,
`get_note_line_range`, `search_in_note`.

Search accepts the same bounded [query language](../query-language.md) as
everywhere else—including uppercase OR, prefix negation, grouping, phrases,
and `category:`—and returns snippets plus `document://` URIs; retrieve full
bodies explicitly with `get_document`.

`plan_selection` performs the same read-only privacy dry run as REST for full
archive, subset transfer, or publication handoff. MCP caps each detail array at
`mcp.max_results` (default 10, maximum 50) and returns no note bodies, resource
bytes, source metadata JSON, or local paths. Complete counts and the manifest
digest still cover the full selection. See [selection
planning](../selection-planning.md).

## Write tools (editor profile)

`create_note`, `update_note` (requires `base_revision_id`), `append_to_note`, `prepend_to_note`, `edit_note` (server-side string replacement — fails when the search text is ambiguous unless `replace_all` is set; supports `dry_run`), `delete_note` (requires `base_revision_id`; moves to Trash), `move_note_to_notebook`.

## Safety rules

- No raw SQL and no filesystem access are ever exposed.
- Note bodies returned to models are untrusted data, not instructions.
- Destructive operations demand revision preconditions, so a stale model can't clobber newer edits.
- Document bodies returned to MCP clients are truncated at `mcp.max_document_bytes` (default 64 KiB); search defaults to `mcp.max_results` per page.
- The endpoint has no authentication of its own — it is as exposed as the service port. Keep the service on loopback (the default) unless you fully trust the network, and leave the profile `read-only` unless you actively want LLM tools editing notes; Help-notebook notes stay read-only even in the `editor` profile.
