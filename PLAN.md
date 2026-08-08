# Plan: v0.7 — Versioning and synchronization

Status: **draft. Written 2026-08-08 from `ROADMAP.md` after v0.6 completed.
Product version is 0.6.0 and the schema is v18. Nothing here is approved: each
slice needs the user's word before any code is written, and eight decisions are
blocking — six of them open since v0.4.**

The v0.6 plan is archived at `plans/v0.6/000-v0.6-plan.md` with its eight
slices. `SYNCHRONIZATION.md` holds the protocol design and
`VERSIONING_AND_SYNC_POLICY.md` holds the invariants; this plan is the order the
work happens in and what each slice has to settle first. Read `AGENTS.md` before
writing or amending an item — in particular "Writing plan items", which requires
every unresolved decision to live inside the item it affects.

## The shape of this milestone, and why it is different

Every milestone so far has been additive: a new surface, a new table, a new
guarantee, and a library that behaves the same afterwards. **v0.7 is the first
that can lose a user's data by being subtly wrong**, because it merges two
divergent copies. The failure mode is not a crash; it is a note that quietly
becomes the wrong note on one machine three weeks later.

That changes how the slices are sized. Each lands with convergence tests before
the next begins, and there is a deliberate investigation slice in front of the
protocol work — the thing v0.6 learned twice, in F5a's withdrawal and F6's
narrowing, is that deciding on the evidence beats deciding on the plan.

**Not in v0.7:** live co-editing, multi-user roles, and encrypted relay
transports. All three sit under later `ROADMAP.md` headings and none is a
prerequisite for the others.

## G0. Investigation: what actually diverges in a real library

**This slice writes no protocol code.** It answers a question the rest of the
milestone would otherwise guess at: which rows in a real Notrios library change
concurrently, and how.

Take the corpora already in evidence — the 382,206-note Joplin import and the
generated profiles — and measure what a fortnight of ordinary use touches. How
many notes are edited on two machines between syncs; how often a notebook is
renamed on one while a note moves on the other; how often the same tag is added
twice; how large a change envelope is for a day's work.

The output is a written finding, not a feature. It should be enough to answer
questions 9 and 10 in `agent/OPEN_QUESTIONS.md` with numbers rather than taste.

**Why an investigation rather than a decision to write down:** the difference is
whether the premise holds. "Per-field or whole-record LWW" is unanswerable in
the abstract — it depends entirely on whether real conflicts are two people
editing the same field or two people editing different fields of one row, and
nobody involved knows which. That is the case `AGENTS.md` says gets a spike.

**Open decisions**

- **What counts as a realistic sync interval?** *Non-blocking.* Default if
  unanswered: measure at one hour, one day, and one week and report all three,
  because a laptop-and-phone pair and a laptop-and-backup-drive pair are
  genuinely different workloads and one number would hide it.

Working state: a document under `performance/v0.7-g0/` saying what diverges, how
often, and how large the resulting envelopes are — enough that G2's conflict
rules are chosen rather than assumed.

## G1. Profiles and identities

Explicit profiles, the logical database UUID, the replica/device UUID,
schema/protocol compatibility checks, bootstrap notebooks on a fresh replica,
validated `notrios://` routing, and a first-class `sync: none` target.

*Most of the machinery exists.* v0.4 P5 shipped the profile registry, the
logical/replica identity split, and `notrios://` routing; schema v12 carries the
identity rows. G1 turns those into a *peer* identity: what a replica advertises,
what it refuses to talk to, and what happens when two replicas turn out to be
clones of one database rather than paired copies.

**Open decisions**

- **What does a replica refuse?** *Blocking.* A peer on an older protocol
  version, an older schema, or a newer one each needs an answer, and "refuse
  everything unequal" is not obviously right — it would make a rolling upgrade
  impossible on a two-device setup. *Recommended:* the protocol version must
  match exactly; schema versions may differ within a declared compatibility
  range, and the older peer refuses records it does not understand rather than
  dropping fields silently.
- **Do clones sync?** *Blocking.* Two replicas sharing a database ID that were
  copied rather than paired are the case P5 already flags for `notrios://`
  routing. *Recommended:* yes, but only after an explicit adoption step. A clone
  and a peer are indistinguishable from the database ID alone, and silently
  merging a forgotten backup copy is the worst version of this feature.

Working state: two replicas can identify each other, agree they may talk, and
refuse when they may not — with nothing yet transferred.

## G2. Replication core

Immutable operations identified by `(replica_id, sequence)`, hybrid logical
clocks for deterministic conflict ordering, per-replica acknowledgement vectors
for completeness and GC, idempotent apply, record and field LWW registers,
set-membership tombstones, body-snapshot conflict copies, and deterministic
notebook-tree repair.

This is the slice that can lose data, and it is the one G0 exists to inform.

**Open decisions**

- **Per-field or whole-record LWW** (`OPEN_QUESTIONS.md` 9). *Blocking, waiting
  on G0.*
- **Add-wins or remove-wins for set membership** (question 10). *Blocking,
  waiting on G0.* Tags and notebook membership are the sets in question.
  *Leaning:* add-wins, because losing a tag someone added is quieter than
  keeping one someone removed — but that is exactly the intuition G0 should
  check rather than confirm.
- **Deterministic notebook-cycle repair** (question 15). *Blocking.* Two
  replicas can each move a notebook under the other, producing a cycle valid on
  neither. *Recommended:* the operation with the lower HLC wins and the loser is
  re-parented to the root with a visible report — silently discarding one is how
  a subtree disappears.
- **Where does a conflict copy live?** *Non-blocking.* Default if unanswered: a
  conflict produces a **revision** on the same note rather than a second note,
  and the GUI shows both. v0.5 made revisions first-class, and a second note
  would need its own title, notebook, and cleanup story.

Working state: two replicas converge on the same library from any interleaving
of the same operations, proven by fault-injection tests rather than by argument.

## G3. Native snapshot and change container

Reuse archive-v2 manifests and object storage for full snapshots and bounded
change envelopes; blobs publish before references and manifests publish last.

**Open decisions**

- **Envelope encoding and compression** (question 13). *Blocking.* It has to be
  deterministic: two replicas producing the same envelope must produce the same
  bytes, or dedupe and transitive verification both stop working.
- **Blob chunking threshold** (question 14). *Blocking.* This needs a
  measurement rather than a guess, and the archive-v2 pack work (P3b) is the
  closest existing evidence.

Working state: a snapshot and a change envelope both round-trip through the
archive-v2 verifier, and an interrupted publish leaves nothing that reads as
complete.

## G4. REST and folder/rclone transport

One protocol over authenticated REST and over immutable shared-folder objects.
`rclone copy --immutable` is a carrier; `rclone sync` and `bisync` are never the
merge algorithm. Same-machine folders, removable drives, and cloud remotes all
use one inbox/outbox layout.

**Open decisions**

- **Is end-to-end encryption mandatory in protocol v1?** (question 12.)
  *Blocking.* A shared folder on a cloud remote means a third party holds the
  bytes, which is a different threat model from REST between two machines on one
  LAN. *Recommended:* mandatory for the folder transport and optional for REST.
  Note that this is the first slice where "do not expose Notrios on a LAN or
  public address until authentication, authorization, CSRF/CORS, TLS guidance,
  quotas, and audit logging exist" in `SECURITY_REVIEW.md` stops being avoidable
  — G4 either implements that list or ships folder transport only.

Working state: two replicas sync over a directory and over REST, through the
same protocol and the same tests.

## G5. Operations and recovery

Durable outbox, retries and backpressure, peer retirement, tombstone and blob GC
watermarks, full resync past the retention horizon, conflict UI, the existing
replace/adopt/merge/fork restore intents, and fault-injection convergence tests.

**Open decisions**

- **Default offline retention horizon and peer-retirement UX** (question 11).
  *Blocking.* Too short and a phone left in a drawer silently forces a full
  resync; too long and tombstones accumulate forever.
- **Android envelope, blob, and pending limits** (question 16). *Non-blocking
  for this slice; blocking for any mobile client.* Default if unanswered: bound
  everything at the archive-v2 limits, which are known to work on desktop, and
  revisit against a real device.

Working state: a peer offline past the horizon is told to resync rather than
silently diverging, and GC never collects something a peer still needs.

## G6. Native archive compatibility bridge

Deferred from v0.4 P6 and **gated on G3, not on G4–G5**. Publish archive-v2 JSON
Schemas, golden fixtures, and capability bounds; add a compatibility command
producing sanitized deterministic test archives; coordinate the separately
maintained `movenotes-v3/notrios2sql.py` importer against them; add
cross-version consumer tests.

It waits for G3 because that slice extends the very container the contract would
pin. Pinning first would mean a reader integrated in v0.4 refusing every archive
written after v0.7 — a safe failure, but a second integration pass for no gain.
`movenotes-v3` had not started the importer as of 2026-08-05.

Working state: an external consumer can validate a Notrios archive against a
published schema and know which capabilities it needs.

## Decisions register

An index, not a home. Each decision lives in the item it affects, under that
item's **Open decisions** subsection (see `AGENTS.md`, "Writing plan items"); if
it is only listed here, the person approving the item will not see it.

| Decision | Item | Status |
|---|---|---|
| Realistic sync interval to measure | G0 | Open, non-blocking; default is to report three |
| What a replica refuses | G1 | **Open, blocking** |
| Whether clones may sync | G1 | **Open, blocking** |
| Per-field versus whole-record LWW (Q9) | G2 | **Open, blocking — waiting on G0** |
| Add-wins versus remove-wins (Q10) | G2 | **Open, blocking — waiting on G0** |
| Notebook-cycle repair (Q15) | G2 | **Open, blocking** |
| Where a conflict copy lives | G2 | Open, non-blocking; default is a revision |
| Envelope encoding and compression (Q13) | G3 | **Open, blocking** |
| Blob chunking threshold (Q14) | G3 | **Open, blocking** |
| Mandatory end-to-end encryption (Q12) | G4 | **Open, blocking** |
| Retention horizon and peer retirement (Q11) | G5 | **Open, blocking** |
| Android limits (Q16) | G5 | Open, non-blocking for G5 |

**Eight blocking decisions, and that is the honest count.** Six of them
(questions 9–16 in `agent/OPEN_QUESTIONS.md`) have been recorded as open since
v0.4 and deferred to "separate v0.7 plans" — this is that plan, and they now sit
in the items that need them. G0 exists because two of the eight deserve
measurements rather than opinions.

Nothing should start before its decisions are answered. v0.6 F1 was approved
with two decisions unseen because they sat in a milestone-level section rather
than in the item; this table is an index of the items, not a substitute for
reading them.
