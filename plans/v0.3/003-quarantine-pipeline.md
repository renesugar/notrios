# v0.3 Task H3 — Quarantine download pipeline

Status: **completed** (2026-07-17). Model: Claude Fable 5 (claude-fable-5).

## Goal

Fetch policy-permitted remote media safely into the quarantine directory —
never the asset store — with the full SSRF/abuse protection set from
`SECURITY_AND_MEDIA_POLICY.md`, recording every attempt. Admission and
Markdown rewriting are task H4.

## What was built

### `internal/media.Fetcher` (fetch.go)

`NewFetcher(cfg, recorder)` + `Quarantine(ctx, {DocumentID, URLs,
AllowReview})` → one `FetchResult` per URL (`status` quarantined/refused,
policy action, reason, content type, size, SHA-256, quarantine path).

Protections, in order:

- **Static policy first**: blocked URLs — and review URLs without the
  explicit `AllowReview` opt-in — are refused with *zero network traffic*.
- **Redirect hops**: `CheckRedirect` re-evaluates the policy for every hop
  under the same permission the original fetch was authorized with, and
  enforces `max_redirects`; a hop to a blocked/review domain refuses the
  fetch before any connection to it.
- **Connect-time address check** (`net.Dialer.Control`): runs after DNS
  resolution with the concrete IP — private/loopback/link-local/unspecified
  addresses are refused unless `allow_private_networks`, which is the only
  reliable defense against DNS rebinding (the static scan does no DNS).
  Environment proxies are deliberately ignored (a proxy dials on our behalf
  and would bypass the check).
- **Size caps enforced while streaming**: per-class `max_bytes` limit with
  `io.LimitReader`; an oversized body aborts mid-stream and removes the
  partial file.
- **MIME sniffing**: `http.DetectContentType` on the first 512 bytes; a
  positive detection always beats the header (a lying `image/png` header on
  HTML refuses), while inconclusive sniffs (octet-stream/plain/xml — the
  SVG case) may fall back to the header type. Only image/*, video/*,
  audio/* (video class), and application/pdf are localizable.
- **Exact SHA-256** computed over the quarantined bytes; files land as
  `quarantine_dir/sha256-<hex><ext>` via a temp file + rename, so partial
  downloads never appear under a final name.
- Timeouts: per-request `fetch_timeout_seconds`, dial 15 s, TLS 10 s.

### Attempt records

- `media.Attempt` + `AttemptRecorder` interface (nil recorder = tests/ad-hoc;
  recording is best-effort auditing and never fails a completed fetch).
- `store.RecordMediaAttempt` / `store.ListMediaAttempts` (new
  `internal/store/sqlite_media.go`) write/read `media_policy_decisions`
  rows (`mpd_` IDs; nullable document ID for ad-hoc runs; newest first).

### Docs

`SECURITY_AND_MEDIA_POLICY.md` implementation-status note; `API_SPEC.md`
remote-media section (pipeline implemented, localize still stub);
`DATABASE_SCHEMA.md` (media_policy_decisions now populated; rule tables
still pending).

## Validation evidence

- `go vet ./...`, `go test ./...` — 15 packages pass. New tests
  (`internal/media/fetch_test.go`, httptest-backed): successful quarantine
  with hash/type/size verification; blocked URL refused with zero requests;
  review requires opt-in; streaming size-cap abort leaves no file;
  lying-header HTML refused; SVG header fallback works; redirect to blocked
  domain and redirect loops refused; non-200 refused; dial-address checks
  (unit) and an end-to-end DNS-rebinding simulation (allowed hostname
  resolving to loopback refused at connect time).
  `internal/store`: record/list round-trip incl. document-less attempts and
  required-field validation; fetcher→SQLite integration through the
  recorder adapter H4 will reuse.
- `bash scripts/validate-scaffold.sh`, `bash scripts/mvp_smoke.sh` — pass.
- `cd web && npm run typecheck && npm test` — 35 tests (web untouched).

## Follow-up tasks

- H4: admission (exact-hash block check → content-addressed store), Markdown
  rewriting to `resource://` in a new revision, dry-run, REST/CLI/MCP/GUI
  entry points, importer flag — all on top of `Fetcher`.
- H5 hooks perceptual hashing into the admission slot.
- The `media_hash_rules` exact-hash block check belongs in H4 admission
  (quarantine itself only computes the hash).
- `extensionForMIME` prefers common spellings (.jpg/.png/.pdf); other types
  take Go's first registered extension.
