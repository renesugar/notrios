# v0.2 Task R7 — Recoll integration (replaces sist2)

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5). Recoll 1.x binaries were installed on the dev machine, so the live pipeline was verified end-to-end, not just documented.

## Changes

- Outbox wiring: create/update/revision-restore/trash-restore enqueue `upsert`; soft-delete/purge/notebook-delete enqueue `delete` — all inside the canonical SQLite transaction. `PendingProjectionJobs`/`CompleteProjectionJob` drain/ack with failure retry accounting.
- `internal/projection`: renders notes as Markdown with YAML front matter (id/title/notebook/author/author_id/published/thread_id/reply_to/source_url/tags) under `<projection>/notes/<doc_id>.md`; `SyncOutbox` + `FullSync`.
- `internal/recoll`: `Sidecar` generates the Recoll config (topdirs, `underscoreasletter`, `fields` prefixes XTAG/XAUTHORID/XNOTEID/XTHREADID/XREPLYTO/XNOTEBOOK/XSOURCEURL/XALIAS, `publishedts` int value slot 1001, mimeconf exec filter), embeds and installs the **from-scratch Apache-licensed** `notrios_md_handler.py` (YAML via PyYAML-or-fallback, TOML via tomllib, tags/aliases duplicated into keywords), runs `recollindex -c`, queries via `recollq -c -F "url title abstract"` (base64 field parsing), and compiles the parsed Notrios query to Recoll syntax (`publishedts:a..b` ranges; trash queries never go to Recoll).
- Search merge: `httpapi.searchMerged` folds sidecar-only hits (verified against the canonical store) after FTS5 hits on first pages; sidecar errors log and fall back to FTS5.
- `notriosd`: when `search_sidecar.enabled`, generates config, full-syncs the projection, indexes, attaches merged search, and drains the outbox + reindexes every 30s. Missing binaries at any step degrade to FTS5-only without failing startup.

## Validation

`go test ./...` including `TestLiveRecollPipeline` (real recollindex/recollq: tag phrase, authorid, author phrase, body term, and publishedts range queries against the generated config + handler; skipped when Recoll is absent), projection outbox/full-sync tests, compile/parse unit tests, fake-sidecar merge + graceful-failure test; `validate-scaffold`, `mvp_smoke.sh` — all passing.
