# v0.7 G10 — snapshot catch-up and reset state machine

Status: **complete**, 2026-08-13. Product remains 0.6.0; the canonical schema is
now **v24**. Implemented by Claude Opus 5 (`claude-opus-5`) under Claude Code,
after explicit user approval naming G10.

## Goal

Bring a new, far-behind, repaired, or user-reset replica to a known state
quickly through existing verified backup/restore machinery.

## What landed

**`internal/synccatchup`** — transport- and storage-neutral, like the G5–G9
cores. Signed requests and responses over canonical length-prefixed bytes;
`SourcePolicy`; one-of-many offer selection; the state machine written out as an
explicit transition table; and payload-key wrapping in the two modes G10's
decision fixed — for the enrolled group, or under Argon2id for a portable backup.

**Schema v24** — `sync_catchup_sessions` (durable progress, snapshot vector,
explicit intent, reason), `sync_catchup_permissions` (separate from enrollment),
and `sync_catchup_floors`.

**Cutover** installs the snapshot's vector as this replica's own observation and
records a floor per replica, so the next exchange asks only for later envelopes.

## The defect the evidence run found

The first evidence run failed outright with `admitted operation has no
contiguous HLC predecessor`.

G5 admits an operation only when the row for the sequence before it is present.
A replica built from a snapshot does not have those rows and **correctly should
not** — the snapshot is that history, already folded into canonical state. So
the first operation after a snapshot looked like a gap and catch-up could not
resume at all.

The fix is the catch-up floor. Admission consults it in exactly two places: the
missing-predecessor check, and the below-the-vector check, which now reports an
already-contained operation as an inert duplicate rather than a conflict. A test
asserts the floor is the *only* thing allowed to stand in for a missing
predecessor: a replica without a snapshot still leaves the same operations
pending rather than admitting a history with a hole in it.

Every unit was already correct. The seam between the snapshot's vector and G5's
operation set was not, and only running the whole flow showed it.

## Decisions taken, and why

**Enrollment is not permission.** Answering a backup request means producing and
handing over a complete copy of the library, so a peer is allowed to be a
snapshot source explicitly, revocably, and only while it is an active enrolled
peer. Permission alone never makes an unenrolled replica a source.

**Offers are chosen among, never merged**, as the resolved decision required.
The furthest-ahead usable offer wins, ties break by responder id, and arrival
order does not matter — two requesters facing the same offers choose alike.

**The state machine is a table, not an inference.** There is no edge from
`restoring` back to `transferring`, and none from `restoring` to `cancelled`:
once canonical rows are being written, the answer is to finish or to fail.
Expiry skips those sessions for the same reason.

**Cutover requires an explicit intent** — there is no default, because an
unstated intent is how a restore becomes destructive by accident — and writes no
peer acknowledgement, because this replica holding a copy says nothing about
what a peer has durably admitted.

**One new direct dependency.** Password wrapping needs a memory-hard derivation,
which the standard library does not provide. `golang.org/x/crypto/argon2` was
already in the module graph as an indirect dependency of Wails, at the same
version, and is BSD-3-Clause; it is now direct. G9's zero-dependency position
covered the artifact crypto and still does.

## Validation

Evidence is `performance/v0.7-g10/` (`catchup-results.json`, `README.md`,
`FINDINGS.md`, `validate_evidence.py`), produced by real replicas walking the
shipped state machine rather than a shortcut.

- 200 and 1,000 notes: 90.91% of operations avoided, 90.18% and 93.87% of time
  saved (2,231 ms to 219 ms; 20,790 ms to 1,274 ms), converging in both cases.
- `internal/synccatchup`: request-signature field binding in six directions,
  permission, eight distinct reasons an offer is unusable, competing-offer
  selection with order-independence and tie-breaks, the full happy path and
  every refused transition, terminal states, Argon2id round trip with a wrong
  password distinguished from damage, and remaining-work planning.
- `internal/store`: permission requiring active enrollment, a session surviving
  every step including a resumed transfer, cutover refusing without an intent,
  the vector and floor it installs, the absence of any peer acknowledgement,
  expiry sparing a restore in progress, cancellation, and the floor letting
  admission resume while a replica without one does not.

Repository validation ran audit-first per `AGENTS.md`: frontend audit/ci/audit,
typecheck, 155 tests, build, offline asset check, `go vet ./...`, `go test
./...`, required files, plan loops, scaffold, docs site, MVP smoke, performance
smoke, all four evidence validators, `git diff --check`, and a verified release
ZIP.

## Out of scope, and still owned elsewhere

No carrier publishes a request or fetches an archive (G11, G14). The archive
container itself is P2–P4's and is not re-implemented or re-measured. No
REST/MCP surface takes a path, and no restore is automatic. Peer authentication
and pairing remain G13's; the secret store remains v0.8's. G11 is unapproved.
