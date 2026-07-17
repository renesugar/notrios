# MVP Task 5 Report — Markdown Link Parser and Graph

## Scope

Implemented the first working Markdown link/backlink/graph slice on top of the MVP Task 4 resource store.

## Implemented

- Added `internal/markdownlinks` with a conservative parser for:
  - standard Markdown links: `[text](target)`;
  - Markdown image embeds: `![alt](target)`;
  - Obsidian wikilinks: `[[target]]` and `[[target|alias]]`;
  - Obsidian embeds: `![[target]]`;
  - heading anchors such as `#Heading`;
  - block anchors such as `#^block-id`.
- Added store-level `DocumentLink`, `DocumentLinkPage`, `GraphRequest`, and `GraphResponse` models.
- Rebuilds outgoing `document_links` rows transactionally on document create, update, and revision restore.
- Clears outgoing links when a document is soft-deleted.
- Resolves app URIs:
  - `document://{collection}/documents/{id}`;
  - `resource://{collection}/resources/{id}`.
- Resolves title-style links and wikilinks by exact case-insensitive title within the collection.
- Resolves resource filename links by exact case-insensitive filename within the collection.
- Preserves unresolved, ambiguous, and external links instead of dropping them.
- Added live REST behavior for:
  - `GET /api/v1/documents/{document_id}/links?direction=outgoing|incoming|both`;
  - `POST /api/v1/graph` for a small graph slice around selected roots.
- Updated `/api/v1/status` to report links as implemented when a store is wired.
- Updated schema to `PRAGMA user_version = 4`.
- Updated the React/Vite UI to fetch and display outgoing links and backlinks for the opened note.

## Known limitations

- The Markdown parser is regex-based and conservative. It is sufficient for the MVP but not a complete CommonMark parser.
- Full block/heading indexing is not implemented yet; anchors are stored on link rows only.
- Graph expansion is intentionally shallow and root-oriented. It is not a replacement for a future graph backend or query language.
- Link navigation in the Markdown preview is deferred to PLAN.md task 6 when `md-editor-rt` is integrated.
- `/api/v1/links/resolve` remains a documented future endpoint.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

## Manual smoke test

A manual REST smoke test should create two documents where the second links to the first, then verify:

```bash
curl 'http://127.0.0.1:8080/api/v1/documents/<source>/links?direction=both'
curl 'http://127.0.0.1:8080/api/v1/documents/<target>/links?direction=incoming'
curl -X POST 'http://127.0.0.1:8080/api/v1/graph' \
  -H 'Content-Type: application/json' \
  -d '{"roots":["<source>"],"direction":"both","max_nodes":20,"max_edges":40}'
```
