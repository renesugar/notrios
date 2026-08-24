# v0.7 G15 — Durable sync jobs, scheduling boundaries, retries, and MCP control

Completed: 2026-08-24

Model: GPT-5 (exact serving variant unavailable)

## Outcome

G15 advances the canonical schema from v25 to v26 and makes explicitly started
sync work observable, cancellable, restart-safe, retryable, and bounded. It
does not add periodic synchronization, a general scheduler, arbitrary commands,
priorities, dependencies, a DAG, or cron.

The existing `jobs` record remains the common status surface. A one-to-one
`sync_jobs` row is the closed durable outbox for incremental, resource-fetch,
catch-up, and restore-preparation operation records. `sync_job_audit` stores
bounded content-free event codes/counts. Base job, sync extension, and audit
transitions commit together.

## Implemented contract

- One `BEGIN IMMEDIATE` claim transaction recovers stale workers, claims the
  oldest due row, and refuses another running job for the same opaque target.
  Separate targets remain independently claimable.
- Persisted targets are SHA-256-derived `target_...` identifiers. Directories,
  URLs, credential references, keys, raw argv, note content, and provider error
  detail never enter a job record.
- A sync round reports durable `plan`, `pull`, `resource_serve`, `push`, and
  `advertise` boundaries; wanted-resource jobs add `resource_fetch`. Restart
  preserves only bounded phase/count metadata and replans from canonical state
  vectors and G8's already verified chunks.
- Heartbeats no longer overwrite ordinary job progress. Cancellation is polled
  between boundaries and through the running context; settlement retains the
  last durable checkpoint.
- Every attempt has a 1-byte to 16-GiB encrypted-artifact allowance (64 MiB by
  default). One wrapper accounts round reads, writes, and materialization.
- `offline`, `quota`, `temporary`, and `byte_budget` failures use deterministic
  per-job ±20% jitter over exponential five-second backoff, capped at fifteen
  minutes and 32 attempts. Manual retry preserves the checkpoint; explicit
  local reset clears it and the attempt counter.
- A configured daemon drains queued/due work but never enqueues work itself.
  A profile used only to host a peer surface or establish a journal boundary
  still starts when no outbound directory/URL is configured; its job control
  reports unavailable until the destination exists.
- CLI adds `sync start` and `jobs retry [--reset]`; existing job status exit
  codes remain 0 succeeded, 1 failed, 3 queued/running, 4 cancelled, 5 unknown,
  and 6 interrupted.
- Local REST adds bounded plan/start/status/retry/reset/conflict routes. The
  OpenAPI document now has normalized method-for-method parity with all 93
  registered REST operations; this check also found and documented four G14
  carrier operations that had been missing from the specification.

## MCP decision and default

The approved policy is implemented with an orthogonal permission rather than a
fifth cumulative content tier. The non-blocking default is
`mcp.sync_scope=disabled`: approving G15 did not silently give an existing
read-only/editor/organizer agent network authority.

- `status`: `get_sync_status` and `list_sync_conflicts`.
- `control`: additionally `plan_sync`, `start_sync`,
  `request_resource_fetch`, `retry_sync_job`, and `cancel_sync_job`.

Control takes only byte/attempt limits and job IDs. Retry/cancel is restricted
to the MCP actor's own incremental/resource-fetch jobs. Generic job tools hide
sync records without explicit sync status authority. MCP never accepts or
returns a path, URL, credential, key, backup, artifact bytes, restore request,
peer enrollment/retirement, purge, catch-up/restore-prep control, reset, or
another actor's job control.

## Validation evidence

- `npm audit` before and after `npm ci`: 0 vulnerabilities.
- `npm run typecheck`; `npm test -- --run`: 15 files, 155 tests; `npm run build`.
- Focused G15 fixtures: v25-to-v26 migration, opaque-target/privacy bounds,
  atomic checkpoint/retry/reset/audit transitions, two-target claim behavior,
  stale heartbeat recovery and final-attempt exhaustion, deterministic jitter,
  offline/quota/budget retry, cancellation at a durable boundary, carrier byte
  accounting, REST request/output bounds, MCP scope/ownership/refusal, and CLI
  exit-code/command coverage.
- `bash scripts/validate-scaffold.sh`: complete Go suite passed; 70 required
  files present.
- `go vet ./...`; `git diff --check`; plan-loop and required-file checks.
- OpenAPI YAML parse and normalized route-drift check: 93 code operations and
  93 documented operations, no difference either direction.
- `bash scripts/build_docs_site.sh`: 15 pages indexed.
- `make gui`; `bash scripts/mvp_smoke.sh`.
- `bash scripts/run_performance_smoke.sh`: generated search benchmark passed.
- `bash scripts/run_offline_assets_check.sh`: no third-party requests or CSP
  violations; offline math rendered.

The first full-suite pass found two compatibility issues: job command rendering
had no mapping for the two runnable sync kinds, and older daemon fixtures used
a non-`none` target without an outbound destination. Both were corrected and
their original integration fixtures passed before the complete scaffold pass.

## Boundaries retained

Destructive physical restore still requires explicit local intent and emergency
backup. Catch-up/restore-preparation job kinds are representable for local
control but are not exposed through MCP. The worker is a durable-outbox drainer,
not a mobile background-service promise or provider polling cadence. G16 owns
the user interface; G17 owns peer expiry/retention; G20 owns release acceptance.
