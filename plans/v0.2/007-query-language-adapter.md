# v0.2 Task R6 — Query-language adapter

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Changes

- New `internal/query` package: tokenizer/parser for the Notrios search language — quoted phrases, implicit AND, `title:`, `notebook:`, `tag:`, `author:`, `authorid:`, `since:`, `until:`, `is:trashed`; unknown operators fall back to literal terms. Timestamp semantics per `SEARCH_QUERY_LANGUAGE.md`: RFC3339 exact, date-only since=start/until=end-of-day in the selected timezone, time-only = today, digit-only = epoch (ms auto-detected).
- `internal/store/sqlite_query.go`: compiles the parsed query to FTS5 + SQL. `notebook:` resolves names case-insensitively and includes descendant notebooks (same-named matches union); `tag:` uses EXISTS joins; `author:`/`authorid:`/`since:`/`until:` join `document_sources` (time falls back to note creation time); `is:trashed` searches trashed notes with LIKE text matching (no FTS rows in trash); FTS `title:` column filters; bm25 ranking with snippets preserved.
- Cursor pagination: opaque base64 offset cursors carrying an FNV checksum of query+collection; replay against a different query is rejected; `NextCursor` returned when more rows exist — "All notes" can now stream incrementally.
- `store.Search` routes all REST/MCP/search-notebook queries through the adapter; legacy `searchLatest`/`searchFTS` paths removed (behavior preserved for empty and plain-term queries — all pre-existing tests pass unchanged).
- Fixed a cgo-adapter pitfall: numeric bounds are bound as TEXT, so time comparisons cast the parameter (`CAST(? AS INTEGER)`).

## Validation

`go test ./...` (3 parser tests + 3 store query tests incl. cursor binding), `check_required_files`, `validate-scaffold`, `mvp_smoke.sh`, `run_performance_smoke.sh` — all passing.
