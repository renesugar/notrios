# MCP Tool and Resource Contract

MCP adapters must call the same service-layer methods as the REST API. Do not implement separate MCP-only search, parsing, media, or persistence logic.

The current MVP exposes a dependency-free JSON-RPC adapter at `/mcp` with `initialize`, `tools/list`, and `tools/call`. It is intentionally small because the scaffold environment avoided external SDK downloads. Replace it with the official MCP Go SDK once dependency policy is settled, preserving the tool names and safety rules below.

## Tool visibility profiles

The server should support capability profiles so an LLM is not shown dangerous or irrelevant tools.

| Profile | Tools |
| --- | --- |
| `search-only` | `list_collections`, `search_documents` |
| `read-only` | search, document retrieval, links, resources, outline |
| `editor` | read-only tools plus current note mutations and media localization |
| `organizer` | editor tools plus move/rename/tag/link repair |
| `administrator` | import, publish, index, delete, policy tools |

## MVP tools — implemented

### list_collections

Returns collections and their capabilities.

Input: `{}`

Output:

```json
{
  "collections": [
    {"id":"personal","name":"Personal", "capabilities":["documents","search","resources"]}
  ]
}
```

### search_documents

Searches documents with conservative defaults.

`query` uses the shared bounded expression language: implicit AND, uppercase
`OR`, prefix `-`, parentheses, phrases, typed fields, and `category:` as a
`notebook:` alias. Maximum query length is 4,096 UTF-8 bytes (also advertised
as JSON Schema `maxLength`); the service additionally enforces 256 tokens and
16 parenthesis levels.

Input:

```json
{
  "collections": ["personal"],
  "query": "sqlite fts5",
  "limit": 10,
  "cursor": null,
  "include_body": false,
  "snippet_characters": 400
}
```

Output uses the same shape as REST `SearchResponse` plus MCP resource links for each hit.

### plan_selection

Runs the shared P1 selection/privacy planner without writing files or changing
canonical state. Required `target` is `full_archive`, `subset_transfer`, or
`publication_handoff`. Typed selectors are recursive notebook IDs, tags, one
bounded query, and up to 1,000 explicit document IDs; policy fields are closed
booleans/enums rather than arbitrary maps.

Output is the REST-compatible content-free plan: stable IDs, resource hashes,
complete counts, internal/private/broken/external link decisions, source-bundle
policy results, exclusions, metadata decisions, warnings, and a deterministic
manifest digest. Detail arrays use `mcp.max_results`; bodies, raw resource
bytes, source metadata JSON/URLs, raw broken-link context, SQL, and local paths
are never returned.

### get_document

Retrieves one document by `document://` URI or document ID.

Required controls:

- maximum returned bytes;
- optional section/heading selection;
- document content marked as untrusted data.

### get_documents

Batch retrieves selected documents. Default maximum: 5 documents.

### list_document_links

Lists outgoing/incoming links for one document, including context snippets and resolution status.

### list_document_resources

Lists embedded/attached resources for one document, without returning binary bytes by default.

### get_document_outline

Returns headings/blocks so an LLM can request relevant sections instead of whole long documents.

### list_notebooks

List all notebooks (flat, with parent IDs, emoji icons, builtin flags). No arguments.

### get_notebook_tree

Return the nested notebook tree in sidebar order. No arguments.

### list_tags

List tags with their current non-deleted note counts. No arguments.

### list_search_notebooks

List query-backed search notebooks in sidebar order ("All notes" first, "Trash" last, builtin rows flagged). No arguments.

### get_note_line_range

Read a 1-indexed inclusive slice of a note body by line numbers.

### search_in_note

Case-insensitive search within one note; returns matches with line numbers and context.

### get_notebook_notes

Input accepts `notebook_id`, optional `limit`, and opaque optional `cursor`.
Output is the REST-compatible `{documents, next_cursor}` page; cursors are
bound to that notebook and must be returned unchanged.

List current notes directly in one notebook.

### scan_remote_media

Report the remote-media policy decision (`allow`/`block`/`review`, with a
reason, media class, and line number) for every remote image/media URL in one
note. Purely static — nothing is downloaded, not even DNS lookups (v0.3 task
H2). Takes `document_id` or a `document://` URI.

## Editor-profile write tools — implemented (task R8)

Exposed by `tools/list` and callable only when `mcp.default_profile` is `editor`; the default read-only profile hides and rejects them.

- `create_note(title, body?, notebook_id?)`
- `update_note(document_id, base_revision_id, title?, body?)` — optimistic concurrency required.
- `append_to_note(document_id, text)` / `prepend_to_note(document_id, text)`
- `edit_note(document_id, search, replace?, replace_all?, dry_run?)` — server-side string replacement; fails if the search text is missing or matches multiple locations without `replace_all`.
- `delete_note(document_id, base_revision_id)` — moves the note to the Trash.
- `move_note_to_notebook(document_id, notebook_id)` — Help-notebook moves are refused.
- `localize_remote_media(document_id, base_revision_id, dry_run?, allow_review?)` — downloads policy-allowed remote media through the quarantine pipeline, stores it as local resources, and rewrites the note to `resource://` URIs in a new revision (v0.3 task H4). `dry_run` reports decisions without fetching; `allow_review` opts review-listed URLs in; blocked URLs are never fetched.

## Later write/control tools

Write tools require explicit scopes and revision preconditions:

- `upload_resource`
- `attach_resource`
- `detach_resource`
- `restore_revision`
- `run_batch` / `get_batch_status` for bounded move/duplicate/trash/tag/link
  organizer operations;
- `plan_sync` / `start_sync` / `get_sync_status` /
  `list_sync_conflicts` for bounded sync administration.

MCP does not carry native archives, change envelopes, or arbitrary blob bytes
in model context. Those use REST/object transfer; MCP returns job IDs and
bounded summaries.

## Resources

Template examples:

```text
document://{collection}/documents/{document_id}
document://{collection}/documents/{document_id}/body
resource://{collection}/resources/{resource_id}
resource://{collection}/resources/{resource_id}/content
```

For large repositories, `resources/list` should not enumerate every document. Search and graph tools should return resource links.

## Forbidden normal tools

- raw SQL execution;
- arbitrary filesystem path reads/writes;
- direct search-index (Recoll/Xapian) mutation;
- bypass media policy;
- bulk download of remote media without a policy and explicit scope.
