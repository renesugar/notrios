# v0.3 Task H2 — Remote-media scan endpoint

Status: **completed** (2026-07-16). Model: Claude Fable 5 (claude-fable-5).

## Goal

Let users and clients inspect a note's remote media and the policy verdicts
**before** anything is downloaded: per-URL allow/block/review decisions from
the H1 policy configuration, surfaced over REST, MCP, and in the GUI's note
inspector. Purely static — no downloads, no DNS resolution.

## What was built

### Policy engine (`internal/media`, new package)

- `Policy` built from `config.RemoteMediaConfig` (invalid default action
  normalizes to `review`).
- `Evaluate(url)` → action + human-readable reason. Order: blocked schemes →
  non-http(s) → private/loopback/link-local literals (`localhost`,
  `*.localhost`, IP literal classes; hostnames that merely *resolve* to
  private addresses are re-checked at fetch time in H3, since scanning does
  no DNS) → blocked domains → allowed domains → review domains → default.
  Domain patterns: `example.org` matches the apex exactly; `*.example.org`
  matches subdomains only. Blocked list always wins.
- `ScanBody(markdown)` extracts and evaluates: Markdown images/embeds with
  any URI scheme (so `file:`/`data:` images are flagged as blocked rather
  than silently ignored), plain links whose extension maps to a media class
  (image/video/pdf), and raw HTML `<img src>` tags; `document://` and
  `resource://` URIs excluded; duplicates reported once with the first
  occurrence's line number.
- `MediaClass(url)` maps to the H1 size-cap classes; `EvaluateURLs` handles
  explicit URL lists.

### REST (`internal/httpapi`)

- `POST /api/v1/documents/{id}/remote-media/scan` — implemented (was a
  stub): scans the stored body; an optional request body with `urls`
  evaluates that list instead (unsaved editor drafts). Response:
  `RemoteMediaScanResult{document_id, media[{url, media_class, action,
  reason, line}], counts}`. 404 for unknown documents.
- `GET /api/v1/media-policy` — active policy report (same shape as the
  status `media_policy` block, now shared via `mediaPolicyStatus()`).
- `POST /api/v1/media-policy/check-url` — evaluate explicit URLs; 400
  without `urls`.
- `remote-media/localize` remains a stub until H4.

### MCP

- Read-only tool `scan_remote_media(document_id | uri)` returning the same
  scan result; listed in `tools/list` for every profile.

### GUI

- `web/src/api.ts`: `scanRemoteMedia()` + types.
- Opening a note now also runs the scan (best-effort; failures never block
  the note) and the Note info inspector shows a "Remote media (n)" section
  with per-URL action badges (block red, review accent), media class,
  reason, and line — plus the hint that nothing has been downloaded.

### Docs

`api/openapi.yaml` (scan/check-url paths, `RemoteMediaScanResult`/`Decision`
schemas), `api/mcp-tools.md`, `API_SPEC.md` (staged-contract section updated
to implemented-vs-stub reality), `docs/gui.md` (inspector), `docs/service.md`
already documented the policy keys in H1.

## Validation evidence

- `go vet ./...`, `go test ./...` — 15 packages pass. New tests:
  `internal/media` (schemes, private addresses incl. allow_private_networks,
  domain-list precedence and wildcard semantics, media classes, body
  extraction, explicit URL lists) and `internal/httpapi/remote_media_test.go`
  (scan endpoint decisions/counts, 404, explicit-URL mode, media-policy
  endpoints, MCP tools/list + tools/call).
- `cd web && npm run typecheck && npm test` — 35 tests pass (2 new inspector
  tests); `npm run build` clean.
- `bash scripts/validate-scaffold.sh`, `bash scripts/mvp_smoke.sh` — pass.
- Live: seeded note with wikimedia image / loopback image / unknown-domain
  PDF returned allow/block/review with correct reasons and lines over REST,
  and the browser inspector displayed all three decisions.

## Follow-up tasks

- H3 (quarantine pipeline) re-applies this policy to every redirect hop and
  to resolved addresses at connect time, and records attempts in
  `media_policy_decisions`.
- H4 (localization) will act on `allow` decisions and expose the localize
  flow in the same inspector section.
- DB-stored `media_domain_rules` are not yet consulted by the policy engine
  (config lists only); wire them in when rule CRUD lands.
