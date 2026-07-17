# v0.3 Task H4 — Localize remote media

Status: **completed** (2026-07-17). Model: Claude Fable 5 (claude-fable-5).

## Goal

Convert a note's remote media into local content-addressed resources through
one shared engine, per `SECURITY_AND_MEDIA_POLICY.md`: quarantine fetch →
exact-hash rule check → admission → resource attachment → Markdown rewrite
to `resource://` URIs in a new revision guarded by a base-revision
precondition. Every entry point (REST, CLI, MCP, GUI, import-time) uses the
same engine; dry runs never fetch a byte.

## What was built

### Engine (`internal/localize`, new package)

`localize.New(cfg.RemoteMedia, store)` + `LocalizeDocument(ctx, Options)`:

- Refuses read-only notes (Help notebook, Trash) with `ErrReadOnly`.
- Scans the stored body (H2 engine); partitions into fetchable
  (allow, or review with `AllowReview`), review, blocked.
- **Dry run** returns per-URL decisions with `would_localize: true` —
  no fetching, no writes, no revision.
- Fetches via the H3 quarantine pipeline (created lazily, so constructing
  a server has no filesystem side effects; attempts recorded through the
  store adapter).
- **Admission**: `media_hash_rules` exact-hash check first — `block` rules
  refuse (reported under blocked, quarantine file removed), `review` rules
  divert to the review list; clean content is admitted via
  `store.CreateResource` (content-addressed → dedup by construction),
  attached (`embedded` for images, `attachment` otherwise), and the
  quarantine file removed. Admission outcomes append `admitted`/`refused`
  rows to the `media_policy_decisions` audit trail after the pipeline's
  `quarantined` rows.
- **Rewrite**: each localized URL is string-replaced with the resource URI;
  when anything changed, `UpdateDocument` writes a new revision with the
  base-revision precondition (defaults to the note's current revision; REST
  and MCP require an explicit one for non-dry runs).
- Result shape: the staged `api.RemoteMediaResult`
  (`revision_id`, `localized[]`, `blocked[]`, `review[]`, `failed[]`).

### Store

`MediaHashRule` + `AddMediaHashRule` (upsert; refuses perceptual+block per
policy) and `FindMediaHashRule`; media methods added to the `store.Store`
interface (`RecordMediaAttempt`, `ListMediaAttempts`, hash rules).

### Entry points

- **REST**: `POST /documents/{id}/remote-media/localize` implemented (was
  the last stub): 428 without `base_revision_id`/`If-Match` on non-dry
  runs, 403 for read-only notes, `allow_review`/`dry_run` in the request.
- **CLI**: `notriosctl localize [--dry-run] [--allow-review]
  [--base-revision rev] <document-id>`; plus `--localize-media` on
  `import joplin-raw` and `import obsidian` (their reports now carry the
  touched document IDs, JSON-hidden) running the engine per imported note.
- **MCP**: `localize_remote_media` (editor profile, requires
  `base_revision_id` unless `dry_run`).
- **GUI**: the inspector's remote-media section gains a "Localize allowed
  media" button (editable notes with allowed decisions only); success
  reloads the note (new revision + fresh scan) and reports counts.

### Docs

OpenAPI (localize description, request fields, 403/428), `api/mcp-tools.md`,
`API_SPEC.md` (remote media section: H2–H4 implemented), `docs/cli.md`
(localize + `--localize-media`), `docs/service.md` (policy paragraph
updated from "not implemented yet"), `docs/gui.md`,
`SECURITY_AND_MEDIA_POLICY.md` status, `DATABASE_SCHEMA.md` (hash rules
consulted).

## Validation evidence

- `go vet ./...`, `go test ./...` — 16 packages. New tests:
  `internal/localize` (rewrite + revision + attachment + audit trail +
  byte-exact resource content; dry run fetches nothing and writes nothing;
  review opt-in; hash-blocked content never reaches the store; stale
  base-revision fails; trashed notes refused; content dedup by exact hash)
  and `internal/httpapi` (REST 428/200/no-op re-run, REST dry run, MCP
  localize happy path + missing precondition + read-only-profile refusal).
- `cd web && npm run typecheck && npm test` — 37 tests (2 new localize
  button tests); `npm run build` clean.
- `bash scripts/validate-scaffold.sh`, `bash scripts/mvp_smoke.sh` — pass.
- Live: against a running `notriosd` and a loopback content server,
  `notriosctl localize --dry-run` reported allow/block without fetching;
  the real run admitted the PNG (`resource://…` rewrite, new revision),
  left the blocked URL untouched, and emptied the quarantine directory.

## Follow-up tasks

- H5: perceptual-hash hooks in the admission slot; `resource_hashes`
  population; dedup/reference reports.
- `media_domain_rules` (DB-stored domain rules) still await rule CRUD.
- URL rewriting is exact-string replacement (an identical URL in prose is
  also rewritten); AST-precise rewriting is deferred to the CodeMirror/
  unified editor milestone.
- `--localize-media` covers Joplin RAW and Obsidian; the Twitter/ChatGPT/
  Claude importers rarely carry live remote media and can adopt the flag
  when needed.
