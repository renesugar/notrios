# v0.7 G14 — REST sync data plane and resumable encrypted backup download

Status: **complete**, 2026-08-14. Product remains 0.6.0 and the canonical schema
remains **v25** — G14 adds no migration. Implemented by Claude Opus 5
(`claude-opus-5`) under Claude Code, after explicit user approval naming G14.

## Goal

Carry the same artifacts as G11 over a direct authenticated API, including
catch-up snapshot download.

## What landed

**`internal/syncrest`** — a `synccarrier.Carrier` implemented over G13's signing
client, plus the backup client: request, resumable download, and open-verify.
Because the exchange was written against that interface in G11, a REST peer and
a shared folder are the same protocol with a different courier. The plan asked
for "REST/directory golden transcript parity"; this is parity by construction.

**`internal/syncbackup`** — deterministic ZIP packing of an archive-v2
directory, extraction that refuses any entry escaping its destination, and
fixed-frame AES-256-GCM sealing whose per-frame associated data authenticates
the frame's index and whether it is the last, so a truncated or reordered
transfer cannot pass as a complete backup.

**`internal/httpapi/sync_data.go`** — seven routes behind G13's `requirePeer`:
five carrier operations and two backup operations. Publish and delete take no
namespace; the peer derives it from the authenticated principal.

**CLI** — `notriosctl sync exchange --url` and `sync fetch-backup --url --out`.

## The property worth stating

**REST never becomes the merge implementation, and it costs nothing to keep it
that way.** Measured, the exchange over an authenticated peer is +15.4% at 100
notes and +0.07% at 500 against the same exchange through a folder: SQLite
admission dominates, exactly as G11 found. Moving merge to the server would
optimize the part that was never the cost, and would give one library's rules
two implementations.

## The four gates a snapshot passes

In this order, and nothing canonical is written until all four are clear:

1. the transport's declared hash of the sealed bytes;
2. the frames, each authenticating its index and its finality;
3. the container, refusing any entry that would escape its destination;
4. **archive-v2's verifier**, which decides whether the result is a snapshot.

ZIP is the wrapper; archive-v2 is the correctness — the resolved decision, made
literal. A test asserts *which* layer refuses a tampered byte, not merely that
something does.

## The defect this work found

**G13 spent the failure budget on success.** The per-address limiter was charged
at the top of the middleware, before knowing whether the request would be
refused. With one request per authentication that was invisible; a data-plane
round makes a dozen or more, so the second round of the first REST exchange came
back `429` and a legitimate peer had throttled itself out of its own library.

The budget is now *checked* before any work and *spent* only on a refusal, so it
bounds guessing without bounding a peer that behaves. Pairing stays the
deliberate exception, because a pairing attempt is a guess at a secret. The
per-peer request budget rose from 120 a minute to 600 in the same change: the
old figure was chosen when a peer only ever said hello.

## Decisions taken, and why

**An opaque id, and only for the replica that asked.** A backup is a complete
copy of a library. Authorization for one is not authorization for another's, so
a different enrolled peer asking for it gets "no such backup" — the same answer
as for an id that never existed, since the difference would say which ids exist.

**Enrolment is not permission**, as G10 fixed: producing a snapshot requires the
explicit, revocable snapshot-source permission, checked before any work.

**Publishing takes no namespace.** The route is `PUT /carrier/{class}/{name}`
and the server computes the namespace from the principal, so a peer writing
into another's namespace is not refused — it is unrepresentable.

**`fetch-backup` stops at a verified archive** and prints the restore command.
What a restore does to a library is a decision with an intent; a download should
not make it.

## Validation

Evidence is `performance/v0.7-g14/` (`rest-data-results.json`, `README.md`,
`FINDINGS.md`, `validate_evidence.py`).

- Exchange: converged at 100 and 500 notes over both carriers, with identical
  transcripts and +15.4% / +0.07% REST overhead.
- Backup: produced, downloaded in bounded requests, resumed, verified, and
  restored — 207 and 1,007 records. Sealing adds 25%, and the findings say
  plainly that it is the ZIP container's per-entry headers over many small
  objects rather than the encryption, with P3b's `--pack` named as the fix
  nobody has measured at scale yet.
- `internal/syncrest`: two replicas converging over REST; the stored artifacts
  proved to be the same sealed `NAR1` bytes a folder holds; a peer refused a
  foreign namespace; `206` and `416` with a range matching the whole; an
  interrupted download resumed across many bounded calls, verified, restored
  with an explicit intent, and **then continued incrementally** — the whole G10
  loop closed; a tampered snapshot refused at the transport's own hash; a
  backup refused to a replica that did not ask for it; a multi-frame snapshot.
- `internal/syncbackup`: frames round-tripping at every boundary including
  empty and exactly one frame, a different key, a flipped bit, truncation, a
  dropped final frame, wrong magic, reordered frames, deterministic packing, and
  a container whose entries try to escape.

Repository validation ran audit-first per `AGENTS.md`.

## Out of scope, and still owned elsewhere

No scheduling, retries, or backpressure — G15 owns durable sync jobs, and every
exchange and download here is an explicit command. No UI (G16). No decrypted
streaming to MCP, no remote archive import request, and no arbitrary path
parameter anywhere: the two directories the data plane uses are derived from the
configured data directory. No network measurement — loopback on one host — and
no large-corpus run, which is G20's.
