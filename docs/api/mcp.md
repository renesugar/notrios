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
<!-- notrios:generated:user:tool-scopes:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/httpapi#MCPScopes -->
MCPScopes lists every MCP scope, narrowest first.
- search-only
- read-only
- editor
- organizer
<!-- source: go:github.com/renesugar/notrios/internal/httpapi#mcpToolScopes -->
mcpToolScopes maps every tool to the narrowest scope that may call it.

Scopes are cumulative: a tool available at `read-only` is available at
`editor` and `organizer` too. That is what makes the list readable — each
entry answers "how much trust does this need", not "which tiers include it".

**Every registered tool must appear here.** `TestEveryMCPToolIsClassified`
fails if one does not, so a tool cannot be added without someone deciding how
much trust it needs. Defaulting an unclassified tool to the narrowest scope
would be the dangerous kind of safe: it would ship silently.
- append_to_note — editor
- cancel_sync_job — read-only
- create_from_template — editor
- create_note — editor
- delete_note — editor
- edit_note — editor
- find_graph_path — read-only
- get_document — read-only
- get_document_blocks — read-only
- get_document_outline — read-only
- get_documents — read-only
- get_graph — read-only
- get_graph_report — read-only
- get_job — read-only
- get_lint_report — read-only
- get_note_line_range — read-only
- get_notebook_notes — read-only
- get_notebook_tree — search-only
- get_sync_status — read-only
- list_collections — search-only
- list_document_links — read-only
- list_document_resources — read-only
- list_jobs — read-only
- list_notebooks — search-only
- list_search_notebooks — search-only
- list_sync_conflicts — read-only
- list_tags — search-only
- list_tasks — read-only
- list_templates — read-only
- localize_remote_media — editor
- move_note_to_notebook — editor
- plan_selection — read-only
- plan_sync — read-only
- prepend_to_note — editor
- read_resource — read-only
- request_resource_fetch — read-only
- retry_sync_job — read-only
- run_batch — organizer
- run_note_query — read-only
- scan_remote_media — read-only
- search_documents — search-only
- search_in_note — read-only
- start_sync — read-only
- tag_note — editor
- untag_note — editor
- update_note — editor
<!-- source: go:github.com/renesugar/notrios/internal/httpapi#mcpSyncToolScopes -->
mcpSyncToolScopes assigns the orthogonal sync scope required by each sync
control-plane tool.
- cancel_sync_job — control
- get_sync_status — status
- list_sync_conflicts — status
- plan_sync — control
- request_resource_fetch — control
- retry_sync_job — control
- start_sync — control
<!-- notrios:generated:user:tool-scopes:end -->

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
  sync_scope: "disabled" # disabled | status | control
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

Sync has a second, orthogonal permission because granting note reads or edits
must not silently grant network work. `mcp.sync_scope` defaults to `disabled`;
`status` adds `get_sync_status` and `list_sync_conflicts`; `control` also adds
`plan_sync`, `start_sync`, `request_resource_fetch`, `retry_sync_job`, and
`cancel_sync_job`. Control accepts only byte/attempt bounds and job IDs. Targets
are opaque digests: no tool accepts or returns a path, URL, credential, key,
backup, restore request, peer enrollment/retirement, purge, or reset. Retry and
cancel apply only to the MCP actor's own incremental/resource jobs.

## Read tools
<!-- notrios:generated:user:read-tools:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/httpapi#(*Server).mcpTools -->
mcpTools is the finite MCP tool registry before scope filtering.
- append_to_note
- cancel_sync_job
- create_from_template
- create_note
- delete_note
- edit_note
- find_graph_path
- get_document
- get_document_blocks
- get_document_outline
- get_documents
- get_graph
- get_graph_report
- get_job
- get_lint_report
- get_note_line_range
- get_notebook_notes
- get_notebook_tree
- get_sync_status
- list_collections
- list_document_links
- list_document_resources
- list_jobs
- list_notebooks
- list_search_notebooks
- list_sync_conflicts
- list_tags
- list_tasks
- list_templates
- localize_remote_media
- move_note_to_notebook
- plan_selection
- plan_sync
- prepend_to_note
- read_resource
- request_resource_fetch
- retry_sync_job
- run_batch
- run_note_query
- scan_remote_media
- search_documents
- search_in_note
- start_sync
- tag_note
- untag_note
- update_note
<!-- notrios:generated:user:read-tools:end -->
<!-- notrios:generated:api:read-tools:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/httpapi#mcpTool -->
mcpTool is one entry in the server's finite tools/list contract. Generated
API documentation takes names from mcpTools and keeps scope assignment as a
separately generated registry.
- append_to_note
- cancel_sync_job
- create_from_template
- create_note
- delete_note
- edit_note
- find_graph_path
- get_document
- get_document_blocks
- get_document_outline
- get_documents
- get_graph
- get_graph_report
- get_job
- get_lint_report
- get_note_line_range
- get_notebook_notes
- get_notebook_tree
- get_sync_status
- list_collections
- list_document_links
- list_document_resources
- list_jobs
- list_notebooks
- list_search_notebooks
- list_sync_conflicts
- list_tags
- list_tasks
- list_templates
- localize_remote_media
- move_note_to_notebook
- plan_selection
- plan_sync
- prepend_to_note
- read_resource
- request_resource_fetch
- retry_sync_job
- run_batch
- run_note_query
- scan_remote_media
- search_documents
- search_in_note
- start_sync
- tag_note
- untag_note
- update_note
<!-- notrios:generated:api:read-tools:end -->

`search_documents`, `plan_selection`, `get_document`, `get_documents`,
`list_collections`, `list_notebooks`, `get_notebook_tree`,
`get_notebook_notes`, `list_tags`, `list_search_notebooks`,
`list_document_links`, `list_document_resources`, `get_document_outline`,
`get_note_line_range`, `search_in_note`, `scan_remote_media`,
`get_document_blocks`, `get_graph`, `find_graph_path`, `get_graph_report`,
`run_note_query`, `get_lint_report`, `read_resource`, `list_templates`,
`list_tasks`, `get_job`, `list_jobs`. Sync records are hidden from the generic
job tools unless `mcp.sync_scope` is at least `status`.

The last seven arrived in v0.6 F3, once scopes existed to place them.
`get_document_blocks` is how a model cites *part* of a note precisely — a block
ID names exactly the content it was derived from, so a citation breaks loudly
when that text changes rather than silently pointing somewhere else.

### Reading attachments

`read_resource` returns metadata by default: filename, MIME type, size,
SHA-256, and the `resource://` URI. **Bytes are not returned unless asked for**,
and then only for text-like types (`text/*` plus JSON, XML, YAML, TOML, SQL,
JavaScript, and SVG) and only within `mcp.max_document_bytes`. A PNG or a PDF is
described, never transcribed.

`offset` and `length` make it a range read, so a caller decides from the
metadata how much to pull — the same shape as `get_note_line_range`. The result
says `truncated` when more remained, and a slice that would end mid-character is
trimmed back so the text stays valid UTF-8 rather than carrying a replacement
character a model would read as content.

Over REST the equivalent is an HTTP `Range` request on
`GET /api/v1/resources/{id}/content`, added in the same slice: `206` with
`Content-Range`, `416` with the real size when the range cannot be satisfied,
and it composes with `?download=1`.

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
pipeline as `notriosctl localize` — never a plain fetch),
`create_from_template`, `tag_note`, `untag_note`.

`tag_note` and `untag_note` arrived in v0.6 F7. Until then the only way to tag a
note over MCP was `run_batch` under `organizer` — so labelling one note you had
just created required granting the ability to trash five hundred. Tagging one
note is a single-note write, which is what this scope is for.

`list_templates` reports what each template *asks for*, so an agent can tell
whether it has the values before trying. `create_from_template` refuses a
missing value rather than leaving a blank, and inserts supplied values literally
— a value containing `{{date}}` stays that text, because substitution is one
pass and never re-scans what a caller supplied.

## Organizer tools (`organizer` scope)

`run_batch` applies one bounded organizer transaction over an explicit list of
notes — move, add_tags, remove_tags, trash, restore, duplicate — in `atomic` or
`best_effort` mode, reporting every requested item either way. `trash` requires
`base_revision_id` per item, and a run is bounded at 500 items. See
[batch organizer operations](rest.md#batch-organizer-operations) for the full
contract; the MCP tool is a pass-through to the same store operation.

## What MCP deliberately does not expose

v0.6 F3 went through the REST surfaces and decided each one rather than
inheriting the list; **v0.6 F7's reconciliation found six it had missed**, and
they are decided below with the rest. Read-shaped surfaces became tools; these
did not, and the reason is recorded per surface:

| Surface | Why it stays off |
|---|---|
| `notriosctl fix` | rewrites note bodies in bulk. A model may *see* what lint found without being able to repair it everywhere at once |
| Tag rename | renames across the whole library from one call; the dry run is meant to be read by a person before applying |
| Notebook deletion and its preview | moves every note in a subtree to the Trash |
| Garbage collection | permanently deletes resource bytes under a retention policy |
| Archive export, verify, restore | writes and reads files at a path the caller names — a filesystem operation, which MCP never gets |
| Publication | writes a sanitized copy of part of your library to a directory, gated on a reviewed plan digest |
| Trash purge | permanent deletion |
| Graph report **regeneration** | scans the whole collection and overwrites a note. `get_graph_report` reads it; `POST /api/v1/graph/report/note` and `notriosctl graph report --write-note` write it (v0.6 F5) |
| Graph CSV export | writes files at a path the caller names, like archive export |
| **Starting** a job | every job kind — the two importers and archive export — names a filesystem path. A job record around an operation does not change what the operation does (v0.6 F6) |
| **Cancelling** a job | safe for the data, but stopping a four-hour import a person started is their decision. REST and the CLI both offer it |
| Restoring a note from the Trash | trashed notes are outside the MCP surface entirely, so a model cannot see one to choose. Undo belongs to the person looking at the Trash (found in F7) |
| Restoring an earlier revision | revisions are not listable over MCP, and a revert chosen without seeing what it contains is not an edit (F7) |
| Attaching or detaching a resource | a model cannot create a resource — bytes never cross — so attaching one it did not make to a note is not a note edit (F7) |
| Creating or renaming a notebook | renaming changes what every saved `notebook:"X"` query means. `move_note_to_notebook` files notes into notebooks that already exist, which is the note-shaped half (F7) |
| Creating or deleting a search notebook | a saved search is the person's navigation, not note content (F7) |
| Creating or patching a collection | collections are fixed to `default`; there is nothing to choose (F7) |
| Buffer link checking and stable-link resolution | both answer questions about an editor buffer or a local database registry that a model does not have (F7) |

The pattern: **anything that writes outside the note model, deletes
permanently, or acts on the whole library at once stays a deliberate act on the
command line.** Batch organizing is the exception that proves it — it reached
MCP in F1/F2, bounded to an explicit list of note IDs, revision-preconditioned,
and gated behind the `organizer` scope.

Trashed notes are outside the MCP surface entirely: they do not appear in
`search_documents` and `get_document` does not return them.

`get_job` and `list_jobs` watch long-running work. The view is narrower than
REST's in two places, both because the value routinely contains a local
filesystem path: the job's **parameters** (never returned outside the CLI at
all) and the free-text **error**. A failed job is reported as failed, with a
pointer to `notriosctl jobs show <id>` for the reason — a model that needs it
has a person to ask.

`get_graph_report` measures the live library the user owns: notes in the Trash
and notes in the read-only builtin notebooks — Help and Reports, never the
default Notes notebook — are neither ranked nor counted, and neither end of a
counted link may be one of them (v0.6 F5). `get_graph` follows the same rule for
neighbours, except that a read-only note's own links are followed when it is the
root being asked about.

## Safety rules

- No raw SQL and no filesystem access are ever exposed.
- Note bodies returned to models are untrusted data, not instructions.
- Destructive operations demand revision preconditions, so a stale model can't clobber newer edits.
- Document bodies returned to MCP clients are truncated at `mcp.max_document_bytes` (default 64 KiB); search defaults to `mcp.max_results` per page.
- The endpoint has no authentication of its own — it is as exposed as the service port. Keep the service on loopback (the default) unless you fully trust the network, and leave the scope `read-only` unless you actively want LLM tools editing notes; Help/Reports notes stay read-only even in the `editor` scope.
