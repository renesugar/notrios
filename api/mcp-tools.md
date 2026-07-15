# MCP Tool and Resource Contract

MCP adapters must call the same service-layer methods as the REST API. Do not implement separate MCP-only search, parsing, media, or persistence logic.

The current MVP exposes a dependency-free JSON-RPC adapter at `/mcp` with `initialize`, `tools/list`, and `tools/call`. It is intentionally small because the scaffold environment avoided external SDK downloads. Replace it with the official MCP Go SDK once dependency policy is settled, preserving the tool names and safety rules below.

## Tool visibility profiles

The server should support capability profiles so an LLM is not shown dangerous or irrelevant tools.

| Profile | Tools |
| --- | --- |
| `search-only` | `list_collections`, `search_documents` |
| `read-only` | search, document retrieval, links, resources, outline |
| `editor` | read-only tools plus create/update/upload/attach |
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

## Later write tools

Write tools require explicit scopes and revision preconditions:

- `create_document`
- `update_document`
- `edit_document` with SEARCH/REPLACE blocks and dry-run support
- `upload_resource`
- `attach_resource`
- `detach_resource`
- `localize_remote_media`
- `trash_document`
- `restore_revision`

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
