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

## Tool scopes

A **scope** decides which tools an MCP client sees and may call. There are four,
narrowest first:

| Scope | Adds | Use it when |
|---|---|---|
| `search-only` | search, and the notebook/tag/collection lists needed to search well — **no note bodies** | you want an agent that can find things and hand you links |
| `read-only` *(default)* | reading note content: bodies, outlines, line ranges, in-note search, links, resources, block and graph reads, the selection dry run | the ordinary case |
| `editor` | single-note writes: create, update, append, prepend, edit, delete, move, localize media | you actively want an agent editing notes |
| `organizer` | `run_batch` — bounded batch move/tag/trash/restore/duplicate over an explicit note list | you want an agent doing bulk organizing |

Scopes are **cumulative**: everything a narrower scope may call, a wider one may
call too.

```yaml
mcp:
  default_scope: "read-only"
```

Tools outside the active scope are hidden from `tools/list` **and refused when
called directly**, with an error naming the scope required and the scope in
force. Both come from the same table, so the list and the enforcement cannot
disagree.

`mcp.default_profile` is the deprecated former name of this key and is still
read, so existing configurations keep working. If both appear, **the narrower of
the two wins** and the service logs it: a key that quietly stops applying must
never widen what an agent may do. An unrecognized value falls back to
`read-only` with a warning, rather than failing closed in a way that looks like
a broken service.

### A scope is a guardrail, not authorization

Notrios is single-user: the administrator and the author are the same person, so
there is no second principal to authorize against. A scope is you narrowing what
*your own* agent may do — a seatbelt, not a lock. It is chosen in the same
configuration file you control, and it is not what makes the endpoint safe to
expose. Keeping the service on loopback is.

There is deliberately **no `administrator` scope**. Garbage collection, purge,
archive restore, and publication are not reachable over MCP at any scope, so a
scope naming them would cover an empty set. Those stay a deliberate act on the
command line.

## Read tools

`search_documents`, `plan_selection`, `get_document`, `get_documents`,
`list_collections`, `list_notebooks`, `get_notebook_tree`,
`get_notebook_notes`, `list_tags`, `list_search_notebooks`,
`list_document_links`, `list_document_resources`, `get_document_outline`,
`get_note_line_range`, `search_in_note`, `scan_remote_media`.

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

## Write tools (`editor` scope)

`create_note`, `update_note` (requires `base_revision_id`), `append_to_note`, `prepend_to_note`, `edit_note` (server-side string replacement — fails when the search text is ambiguous unless `replace_all` is set; supports `dry_run`), `delete_note` (requires `base_revision_id`; moves to Trash),
`move_note_to_notebook`, `localize_remote_media` (runs the same quarantine
pipeline as `notriosctl localize` — never a plain fetch).

## Organizer tools (`organizer` scope)

`run_batch` applies one bounded organizer transaction over an explicit list of
notes — move, add_tags, remove_tags, trash, restore, duplicate — in `atomic` or
`best_effort` mode, reporting every requested item either way. `trash` requires
`base_revision_id` per item, and a run is bounded at 500 items. See
[batch organizer operations](rest.md#batch-organizer-operations) for the full
contract; the MCP tool is a pass-through to the same store operation.

## What MCP deliberately does not expose

Several surfaces exist over REST and the CLI and are **not** MCP tools:
workspace lint and fix, graph traversal and the orphan/hub report, block
listing, embedded query-block evaluation, tag rename, notebook deletion and its
preview, garbage collection, archive export/verify/restore, and publication.

That is a choice, not an oversight. MCP is where untrusted model output meets
your library, so the tools it gets are reading, searching, and bounded
single-note edits with revision preconditions. Whole-library reports and
organizer operations stay on surfaces a person drives.

Trashed notes are outside the MCP surface entirely: they do not appear in
`search_documents` and `get_document` does not return them.

## Safety rules

- No raw SQL and no filesystem access are ever exposed.
- Note bodies returned to models are untrusted data, not instructions.
- Destructive operations demand revision preconditions, so a stale model can't clobber newer edits.
- Document bodies returned to MCP clients are truncated at `mcp.max_document_bytes` (default 64 KiB); search defaults to `mcp.max_results` per page.
- The endpoint has no authentication of its own — it is as exposed as the service port. Keep the service on loopback (the default) unless you fully trust the network, and leave the scope `read-only` unless you actively want LLM tools editing notes; Help-notebook notes stay read-only even in the `editor` profile.
