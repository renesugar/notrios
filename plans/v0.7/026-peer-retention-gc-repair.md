# v0.7 G17 — Peer retirement, retention horizon, tombstone/resource GC, and repair

Completed: 2026-08-24

Model: GPT-5 (exact serving variant unavailable)

## Outcome

G17 advances the canonical schema to v27 and implements safe history/payload
collection without turning time, a backup sink, or credential revocation into
proof that an offline peer no longer needs data. The resolved configurable
default remains 90 days with a 30-day peer warning and at least one physical
snapshot that is re-verified at the destructive boundary.

## Safe-floor and repair contract

- Each replica's eligible operation sequence is the monotonic minimum of the
  age floor, current vector, selected verified-snapshot vector, and every active
  peer acknowledgement. Exact dry-run digests refuse changed state.
- `notriosctl sync retention --snapshot <dir>` verifies the complete physical
  snapshot and database identity before planning; `--apply` repeats that work
  and needs `--confirm-digest`. A verification row by itself cannot prove a
  directory still exists.
- A repair source must cover every previously collected floor. The repair plan
  names that snapshot plus newer log operations and is never auto-installed.
  Listing/admission below a floor returns typed `full_resync_required`; durable
  jobs expose only `catchup_required`, not replica/floor detail.
- Compaction captures the canonical metadata checkpoint and permanently retains
  death-certificate identity and revision order needed by later deterministic
  merge. Operation dependencies covered by the floor can then be removed.

## Retirement, purge, and resources

- `replica.retire` is a signed ordinary operation. Preview returns the exact
  `retire-peer:<replica_id>` confirmation and consequences. Apply retires the
  replica, revokes its old keys/snapshot permission, prevents stale re-
  enrollment, and reports active peers that have not acknowledged the decision.
  It does not require those peers online.
- Key revocation remains a distinct security act and deliberately does not
  release the acknowledgement watermark. Only explicit retirement does.
- Permanent purge on an enrolled library emits a signed death certificate and
  keeps the document payload until the safe floor. Payload collection marks
  newly unreachable resources but preserves compact death identity so stale
  replay cannot resurrect the document.
- The existing resource collector receives a conservative frozen gate: the
  retained snapshot and all active peer acknowledgements must cover the current
  vector. Sync-enrolled `notriosctl gc` therefore requires `--snapshot` and
  re-verifies it. Backup/export sinks never enter the active-peer set.

## Human/API boundary

The responsive Sync Center adds a retention section and peer retirement review.
It explains warning/horizon/full-resync states, eligible operation/tombstone
counts, the recovery source, and the non-automatic install boundary. The web
layer receives no filesystem path, retirement reason/signature/key, or apply
authority. OpenAPI adds the read-only retention operation and two retirement
review/apply operations; there is no HTTP retention apply route.

## Validation evidence

- Store fixtures cover two-active-peer watermark matrices, stale-digest
  refusal, a phone beyond its horizon, key revocation versus retirement,
  retirement propagation/re-enrollment refusal, snapshot-above-floor repair,
  signed purge/death survival, one-peer resource reachability, and typed
  below-floor catch-up.
- HTTP and React fixtures cover retention redaction/read-only behavior and
  preview plus explicit consequence confirmation for retirement. Eight focused
  Sync Center tests, the complete 16-file/163-test frontend suite, and the
  production desktop build/typecheck pass.
- Browser plugin unavailable; regular Playwright with installed Chrome passes
  the generated loopback profile at 1440×960 and 390×844. Page identity,
  meaningful content, viewport containment, mobile touch controls, Escape,
  no web apply/path control, and zero console warnings/errors pass.
- `go test ./... -count=1` and `scripts/validate-scaffold.sh` pass on the host,
  including socket/two-daemon integration tests and all 70 required scaffold
  files. npm audit before/after clean install reports zero vulnerabilities.
  OpenAPI/code parity is 109/109 normalized operations. The MVP service smoke,
  performance smoke, offline/CSP asset audit, docs build, `go vet ./...`, plan
  loop check, and `git diff --check` pass.
- `performance/v0.7-g17/RETENTION_COST.json` measures 382,206 generated
  representative operations—one per published full-corpus document—at
  169,906,176 incremental bytes (444.54 bytes/operation). A transparent
  5,000-operation/day 90-day projection is 200,043,377 bytes. No private corpus
  content/path/database was read; the evidence uses only the published count.

## Boundaries retained

Carrier cleanup is not canonical GC. Time alone proves nothing. Conflict
revisions and referenced data are not collected by this slice. A retired device
must reset and pair as a new replica. Snapshot catch-up and repair remain
verified, explicit, and non-installing until the existing destructive handoff.
The product remains 0.6.0; G18's shared-core/FFI/platform handoff is separate
and unapproved.
