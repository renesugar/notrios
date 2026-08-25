# Coding Client Handoff

This handoff applies to any coding agent or client continuing this project (Codex, Claude, aider, swival.dev, etc. — formerly `CODEX_HANDOFF.md`). The repository is designed so an agent can continue from repository files alone.

Current phase: v0.1 through **v0.6** are complete; product version is **0.6.0**
and the schema is **v27**. The eight v0.6 slices are archived under
`plans/v0.6/`: F0 (notebook targeting, landed with v0.5 E10), F1 (batch
organizer transactions), F2 (MCP tool scopes), F3 (MCP read coverage and HTTP
`Range`), F4 (note templates and task extraction), F5 (graph views that stay
readable at scale), F6 (job control plane), and F7 (documentation and release
wrap-up). The thirteen v0.5 slices remain archived under `plans/v0.5/`.

`PLAN.md` now holds the **active v0.7 native synchronization plan** with
independently approvable G0-G20 slices plus the newly inserted, blocking
G14a-G14e archive-scalability sequence, the newly inserted blocking G17a-G17b
evidence-preservation sequence, and the newly planned G18a-G18g documentation-
integrity/Hugo-Ledger sequence. The user's 2026-08-11
review resolved the policy decisions through G17, including mandatory payload
encryption and per-replica Ed25519 signatures. **G0-G17 are complete** and
archived under `plans/v0.7/`; their reviewed evidence is under
`performance/v0.7-g0/`, `performance/v0.7-g1/`, `performance/v0.7-g1a/`,
`performance/v0.7-g2/`, `performance/v0.7-g4/`, `performance/v0.7-g5/`,
`performance/v0.7-g6/`, `performance/v0.7-g7/`, `performance/v0.7-g8/`,
`performance/v0.7-g9/`, `performance/v0.7-g10/`, `performance/v0.7-g11/`, and
`performance/v0.7-g12/`, `performance/v0.7-g13/`, `performance/v0.7-g14/`,
`performance/v0.7-g14a/`, `performance/v0.7-g14b/`, and
`performance/v0.7-g14c/`, `performance/v0.7-g14d/`, and
`performance/v0.7-g14e/`, `performance/v0.7-g16/`, and
`performance/v0.7-g17/`. G15's archive is
`plans/v0.7/024-durable-sync-jobs.md`; G16's archive is
`plans/v0.7/025-sync-recovery-ui.md`; G17's archive is
`plans/v0.7/026-peer-retention-gc-repair.md`. **G17a is next, is not approved,
and blocks G17b, G18, and every GitHub push. G17b and G18/G18a-G18g are also
not approved.** G14b selected option B: a
required compatible same-schema SQLite-image plus bounded packed-assets
capability for whole-library full backup/catch-up, retaining packed semantic
archive-v2 for subset, merge, schema-independent interchange, and fallback.
G14c implements that exact representation, G14d integrates it, and G14e passed
the full-scale acceptance matrix and froze the format.

G15 advances schema v26 with a sync-only durable outbox over the F6 job
records. `internal/syncjobs/` drains only explicitly queued/due rows; it never
creates periodic work. Atomic target leases, content-free phase checkpoints,
heartbeats, cancellation, 64 MiB default/16 GiB maximum per-attempt artifact
budgets, stale-worker recovery, and deterministic jittered retry are live over
the existing directory/REST round. Targets persist only as opaque digests.
CLI adds `sync start` and `jobs retry [--reset]`; local REST adds bounded
plan/start/status/retry/reset/conflict routes. MCP has an orthogonal
`mcp.sync_scope=disabled|status|control`, default disabled, and can control only
its own path-free incremental/resource jobs—never locations, credentials,
keys, bulk bytes, enrollment, backup/restore, retirement, purge,
catch-up/restore-prep, reset, or another actor's job. The full Go/UI/docs/smoke
validation passed; OpenAPI and code now match all 93 registered operations.

G16 adds no schema. The responsive loopback/native Sync Center names the active
profile/database/replica; configures none/directory/REST with a native-only
folder chooser and explicit inbound switch; discovers, invites, pairs, and
separately grants complete-snapshot permission; shows durable job/peer/resource/
repair state; resolves body conflicts with an explicit two-parent revision; and
stages catch-up/reset for review. NPB1 wraps the verified physical NBK1 payload
key with an Argon2id password and inspection always leaves canonical state
untouched. Passwords are cleared and never stored. The warned `0600` provider
is injectable and shared live by pairing, the peer surface, and each job attempt.
The real two-daemon acceptance flow and regular Playwright desktop/mobile sweep
passed; no Browser plugin or mobile build is claimed. OpenAPI and code match all
106 normalized non-HEAD operations.

G17 advances schema v27. `internal/store/sync_retention.go` plans operation and
tombstone collection at the minimum of the configurable 90-day age floor, a
verified physical-snapshot vector, every active peer acknowledgement, and the
current vector; floors only advance. `sync retention --snapshot <dir>` fully
re-verifies the retained image and database identity before dry-run or apply,
and apply needs the exact digest. Sync-aware `gc` likewise requires a currently
verified snapshot. Signed ordinary `replica.retire` operations revoke old peer
credentials, propagate without requiring every peer online, expose peers that
have not acknowledged the decision, and prevent stale re-enrollment. Key
revocation alone deliberately keeps the acknowledgement watermark open. Signed
purge retains payload until safe and preserves compact permanent-death identity
afterward. Peers below a collected floor get typed, non-automatic snapshot
catch-up; repair selects only a retained snapshot covering every existing
floor, then newer log operations. The Sync Center is a path-free review surface
with explicit retirement confirmation and no retention-apply HTTP route.
OpenAPI/code parity is 109 operations. Generated full-corpus-scale evidence
measured 169,906,176 incremental bytes for 382,206 representative operations,
so the reviewed 90-day default remains.

The 2026-08-24 planning amendment adds G18a-G18g between the portability
handoff and G19 compatibility bridge. Two investigations first freeze a
cross-language source-anchor/claim grammar and a reproducible
`hugo-theme-ledger` integration. The implementation slices then add an honest
executed/generated/claimed/unverified audit, result-bearing CLI/config/REST/MCP
examples, browser-executed GUI journeys with action-length evidence, generated
user/API fragments with freshness checks, calibrated advisory blind-code
contradiction/actionability review, and the pinned Hugo/Ledger+Pagefind site.
Semantic similarity is explicitly rejected for truth checking; model output is
never a CI gate. Raw `docs/` Markdown remains the one source for protected
offline Help. Planning inventory found `docs/service.md`'s stale schema-v20
claim versus canonical v27 and preserves it as G18a calibration evidence before
the audited correction. No new slice is approved by this amendment.

The 2026-08-24 evidence-preservation amendment inserts G17a-G17b ahead of G18
and any GitHub push. A read-only inventory found 77 regular files in
`/home/renes/evidence/notrios`: 73 ZIPs and four PNGs, about 266 MB total, with
no existing detached-signature or RFC 3161 sidecars. G17a must freeze the full
all-file scope; define honest hash, signer-identity, third-party-time, and
custody claims; select the exact user-approved OpenPGP fingerprint and RFC 3161
authority/policy; and prove a deterministic CD-sized ISO contract with generated
fixtures only. G17b then backfills without rewriting original bytes, checks a
canonical chained manifest and outer ISO catalog into Git, and writes immutable
numbered ISO images only under `/media/renes/SEAGATE2TB/notrios-evidence/`.
Backfilled records are explicitly retroactive. ISO images, network timestamp
requests, signing-key operations, a GitHub push, and physical burning are
distinct permissions; neither planning amendment authorizes them. G17a and
G17b have no product/schema/runtime scope.

G14c adds production `sqlite-image+packed-assets.v1` creation and read-only
admission under `internal/snapshotimage/` and
`internal/store/sqlite_snapshot.go`. `notriosctl snapshot create` uses SQLite
Online Backup, securely clears the reviewed local/transient tables in the copy,
packs database-declared local blobs/source bundles into deterministic stored
USTAR files bounded at 256 MiB payload or 65,536 entries, resumes only verified
pack boundaries, restarts the image, and publishes the manifest last.
`notriosctl snapshot verify` checks exact schema/application capability,
hashes/lengths, SQLite integrity, identity, vector/floors, cleared state, safe
paths, deterministic headers, object hashes, and external completeness. It
returns install-ready staging only. G14d adds `snapshot restore --intent
replace|adopt`: a verified emergency physical snapshot, adjacent durable
roll-forward plan/startup blocker, staged activation with a fresh replica ID
and catch-up floors, external-index rebuild queue, and post-snapshot replay.
REST now produces and verifies the same physical snapshot through deterministic
sequential USTAR plus existing NBK1 frames; range resume beyond 3 GiB and
identical resumable directory bytes are tested. Generated 100k restore evidence
completed in 13.080 seconds at 24,788,992 bytes peak RSS and queued all 100,000
documents. No schema, compressor, dependency, REST/MCP path surface, or
archive-v2 behavior changed. G14c is archived as
`plans/v0.7/021-scalable-native-snapshot-representation.md`; G14d is archived as
`plans/v0.7/022-scalable-restore-catchup.md`; G14e is archived as
`plans/v0.7/023-full-scale-archive-catchup-acceptance.md`. Neither exFAT, a
cloud-provider rerun, an Android emulator, nor a physical device is claimed by
G14e.

G14e's resumable production harness reused unchanged import results but
re-fingerprinted the equivalent 382,206-document Joplin/Obsidian views and the
attachment workload. Nineteen privacy-sanitized phases pass: current/previous
packed archive-v2 verify/restore, physical first/verify/unchanged/restore for
both workloads, full REST/directory catch-up with emergency replacement and
post-vector replay, and frozen Restic/Borg integrity checks. The final catch-up
completed in 2,948.669 seconds at 210,010,112 bytes peak RSS with exact
canonical equality. Full scale exposed and fixed four G14d contract defects:
the ordinary 30-second timeout on synchronous snapshot creation, an 8 MiB
generic response ceiling truncating 16 MiB ranges, unconditional whole-library
reconciliation for a body edit, and quadratic repeated-prefix validation in
bounded directory publication. Tests cover each boundary; no format, schema,
compressor, third-party dependency, REST/MCP path surface, or automatic restore
was added. G15-G17 are now complete; G17a is next and blocks G17b, G18, and any
GitHub push. Every remaining item requires separate item-by-item user approval.

G14b added only investigation/prototype code and aggregate evidence. Its 57
validated full-corpus phase rows cover equivalent 382,206-document Joplin and
Obsidian views, an attachment-bearing workload, both native candidates, current
catch-up boundaries, stopped/online SQLite variants, Restic/Borg canonical and
1,237,553-file raw references, corruption refusal, unchanged snapshots, and a
distinct Google Drive copy. The image path was 1,884.2 seconds locally versus
4,384.0 for packed semantic reconstruction (2.33x faster), or 2,245.1 versus
4,457.6 seconds including provider evidence. The semantic artifact was 78.3%
smaller and remains first-class. Loose layouts failed file shape; raw
repository paths repeatedly failed memory. Import time/RSS remains separate
performance debt; G14e fixed the 3.22 GiB post-snapshot replay by scoping
reconciliation to admitted record families. G14b itself changed no production
format, schema, dependency, default, encryption, or catch-up behavior changed.

G14a adds only evidence/prototype code. Its 11 adapters map loose/packed
archive-v2 and catch-up, stopped/online/bundled SQLite-image candidates, and
restic/borg raw/canonical references onto nine common stages. Eighteen generated
phase rows validate immutable resume, source-read-only behavior, privacy,
arithmetic, exact wrapper hashes, semantic restore, and real incremental replay.
At 100k, loose export made 100,093 files; ZIP added 11.15% while NBK1 added
6,004 bytes; open and restore stayed under 92 MiB, but incremental replay
reached 797,937,664 bytes peak RSS. Those are baseline failures for G14b, not
production changes made by G14a.

G0 added no production sync code or dependency. It freezes the threat model,
normative glossary, thirty misuse/control traces, and upstream license/platform
matrix. Later crypto design must advance the encryption epoch when a compromised
replica is revoked, sign domain-separated canonical outer artifact bytes while
binding the visible header as AEAD associated data, and avoid exposing plaintext
content hashes as carrier routing names. Current REST still has no general
authentication and remains local/loopback-only.

G1 likewise added no production sync code, schema, or dependency. Its
aggregate-only scan covers 2,063,061 bodies without committing private paths or
content, and its deterministic workload selected complete UTF-8 revision
objects, optional beneficial named-parent line deltas, and line-first merge
with bounded Unicode-aware word-token refinement. Same-token and delete/edit
overlap becomes a durable typed conflict. The earlier
`github.com/epiclabs-io/diff3` recommendation is now superseded: it does not
provide binary delta encoding, and G7 will own a bounded pure-Go line/word merge
implementation.

G1a added an investigation-only pure-Go Subversion-style matcher, a bounded
constrained RFC 3284 VCDIFF codec, and a minimal private comparison container
under `performance/`; it added no production sync code or dependency. All 21
text/binary fixtures reconstructed exactly and deterministically. VCDIFF was
beneficial in 19 cases and smaller than G1's line JSON in all 14 comparisons;
empty and unrelated bytes retain complete-object fallback. Pinned xdelta3 and
open-vcdiff oracles decoded every representative Go stream exactly. The strict
Go decoder is not a general VCDIFF decoder, and the private container and
Subversion svndiff are not selected. Production use requires G2 bounds, a real
immutable named parent, exact outer hashes, and separate G7/G8 approval.

G2 also added no production sync code, schema, cryptography, transport, or
dependency. Its aggregate-only 100/10k/100k workload selected compact canonical
NCB1 operation records plus a canonical-JSON outer manifest candidate and
deterministic gzip: at 10,000 operations NCB1 was 51.9% smaller raw, 16.3%
smaller compressed, and materially cheaper to decode/allocate than the JSONL
prototype. Envelopes close at 10,000 operations, 16 MiB canonical bytes, or 4
MiB compressed bytes. Per-peer pending admission stops at 10,000 operations/64
MiB disk-backed bytes. Resources stay whole below 1 MiB and use 1 MiB fixed
chunks above it; sync packs provisionally target 64 MiB/4,096 objects/4 MiB
trailers. G9 must still promote or replace the codec, pin goldens, and implement
crypto/admission. FastCDC remains deferred. Emulator and physical-device
checklists prevent these desktop-proxy numbers from becoming a mobile claim.

G3 added production local profile/config isolation but no replication schema or
transport. The version-2 stable-link registry now supports generated runtime
profiles with a random local profile ID, one bound database/replica identity,
one owner-only config, isolated absolute runtime paths, a distinct loopback
port/public URL, and `sync.target: none` by default. `notriosctl profile
create|show|list|validate|start` is live; start launches only `notriosd -config
<path>` and is not a supervisor. Startup independently revalidates the binding.
Raw filesystem copies are refused until explicit adopt/fork, while two valid
replicas of one database remain explicit stable-link ambiguity. Status/UI name
the active profile. The real CLI fixture runs two daemons simultaneously.

G4 adds schema-v19 local replication durability but no transport, admission,
merge, cryptography, REST/MCP surface, or UI. Explicit enrollment records a
sequence-zero full-snapshot boundary for the current replica. Canonical-table
triggers feed one transient seam that allocates an immutable local operation
and advances the local contiguous vector inside the caller's transaction;
rollbacks and interrupted uncommitted transactions retain neither side.
`target: none` before enrollment stays journal-free, while a non-none target
establishes the boundary at startup. Identity rotation retires the allocator
and requires re-enrollment. The 100k import A/B produced exactly 300,000
operations with 23.9% elapsed and 92.8% database-byte overhead.

G5 advances the schema to v20 and adds transport-neutral admission, but still
no carrier, cryptography, REST/MCP/UI surface, background sync, or canonical
record merge/application. `internal/syncstate` fixes protocol 1.0 with schema
19-20 compatibility and three required capabilities, compares bounded vectors,
and plans deterministic missing ranges. Already configured local fixture peers
can submit strict normalized operations; gaps and missing dependencies remain
disk-backed under the G2 quotas, exact replay is inert, conflicting replay and
unknown records refuse, and one transaction moves all newly contiguous work
into the immutable operation set while updating gaps/vector/ack. A handshake
never auto-enrolls a peer. Three real local replicas plus a 100-seed model
converge after shuffle, duplicate, and drop-then-deliver schedules; restart,
injected rollback, clock/sequence skew, and sequence exhaustion retain the
correct boundary.

G6 advances schema v21 and atomically applies G6-owned canonical metadata after
G5 admission. Bounded HLCs order sparse field registers and LWW document-tag
elements by wall/logical/replica/sequence; per-replica HLC regression refuses
the whole admission. The fold starts at the explicit sequence-zero baseline,
keeps trash/restore distinct from permanent death certificates, and repairs
notebook cycles/orphans, document homes, and case-insensitive name collisions
deterministically with visible current repair rows. Sync-enabled local purge is
gated until G9 supplies signing; the internal G6 fixture seam only validates
mandatory structural signer/signature fields. G6 does not merge bodies, move
resource bytes, collect retained payloads, add a carrier, expose sync over
REST/MCP/UI, or authenticate peers.

G7 advances schema v22 and converges note bodies. A revision is an immutable
object naming its parents, its exact content hash, and its byte length; the
capture trigger refuses an enrolled revision without one, and the upgrade
backfills every existing revision plus a synthesized linear parent chain,
breaking `created_at` ties by insertion order rather than by random id.
`internal/syncdelta` is the reviewed promotion of the G1a VCDIFF prototype with
production bounds and a benefit gate; the unselected `NXD1` container and the
stream wrappers were not promoted. `internal/syncbody` implements G1's bounded
line-first merge with **single-line** word refinement — two randomized cases
proved a wider region invents lines — plus the revision DAG and the derived
merge and conflict identities. Admission verifies a reconstructed body against
its exact hash before any canonical write; a missing base or corrupt patch
becomes `missing_base` or the terminal `unverified` in
`sync_revision_pending_bodies`, never a best-effort patch. Overlapping edits
become a durable typed conflict on the same document with its two revisions
stored sorted, so both replicas derive one identity. `current_revision_id` is
derived from the revision graph, not last-writer-wins. Real two-replica evidence
transferred 55.37%, 27.92%, 16.49%, and 12.06% of the complete-body
counterfactual at G1's four offline intervals with every document converging.
G7 does not move resource bytes, add a carrier or wire codec, sign anything, or
expose revisions, deltas, or conflicts over REST/MCP/UI.

G8 advances schema v23 and converges attachments before their bytes. A blob row
may now exist without a file — `blobs.availability` is `local` or `unavailable`,
and a trigger refuses in both directions any row whose availability contradicts
whether it has a storage path — so a note can reference an attachment this
replica has not downloaded, with no placeholder bytes anywhere.
`internal/syncassets` holds G2's whole-below-1-MiB and 1-MiB-chunk plan, the
16,384-chunk and 16 GiB ceilings, manifests with per-chunk hashes and a
content-addressed digest, and the eager/pinned/lazy policy whose threshold is
deliberately the same mebibyte that decides chunking. Bytes are fetched through
a transport-neutral `ObjectProvider` that G11 and G14 will implement; chunks are
staged outside the content-addressed tree, transfers resume from verified
segments, and nothing is installed until the manifest digest, each chunk hash,
the whole-object hash, the length, and the sniffed content type all agree.
**Two behaviors changed elsewhere on purpose:** an archive-v2 export now refuses,
naming the object, rather than omitting unmaterialized bytes, and garbage
collection removes an unmaterialized blob's transfer state and staged chunks
with it. Resource deltas were considered and not implemented — G1a's benefit
case needs a named immutable parent, which resources do not have.

G9 adds `internal/syncwire` and **no schema change and no dependency**. It is
the canonical NCB1 operation block, the NEV1 envelope, deterministic gzip, and
the NAR1 artifact: AES-256-GCM under a key derived per artifact by HKDF-SHA256
from a fresh 32-byte salt, signed with Ed25519 over domain-separated canonical
outer bytes. Encrypt-then-sign lets a receiver reject a forgery without
decrypting, and the canonical header is both the derivation salt and the AEAD
associated data. The visible header carries only G0's routing tuple, with
routing names as keyed HMAC blinds rather than plaintext content hashes.
Advancing an encryption epoch and retiring one are separate acts, so revocation
does not cost a library its own history. Every primitive is Go standard library.
**Two things a later agent should know:** G2's fixed sixteen-byte identifier
assumption did not survive production identifiers and was replaced with length
prefixes, so the promoted codec is not byte-identical to the prototype that
justified it; and `SignDeathCertificate`/`VerifyDeathCertificate` supply the
signing G6 recorded as owed, but the enrolled-purge path is deliberately **not**
switched over — that belongs with G17's retention horizon. The store still
journals and admits JSON operations locally; the canonical codec is a wire
format, not the journal's storage.

G10 advances schema v24 with the durable catch-up state machine. Requests and
responses are signed; only an active enrolled peer **explicitly permitted** as a
snapshot source may answer; competing offers are chosen among, never merged; and
the state machine is an explicit transition table in which a restore in progress
cannot re-fetch underneath itself, be cancelled, or be expired by a clock.
Cutover requires an explicit restore intent and writes no peer acknowledgement.
**The thing a later agent most needs to know:** a replica built from a snapshot
has no predecessor operation rows, which broke G5 admission outright until
`sync_catchup_floors` was added. A floor is the only thing permitted to stand in
for a missing predecessor, and a replica without one still leaves such
operations pending. Password wrapping is Argon2id, which promotes
`golang.org/x/crypto` from indirect to direct at the same version.

G11 adds `internal/synccarrier`, `internal/synckeys`, and `notriosctl sync`, and
**no schema change**. The shared folder is a disposable postbox: every path
segment below `notrios-sync/v1/` is a keyed blind, every artifact is a G9
sealed artifact, and every writable path lives inside the writing replica's own
namespace. **Three things a later agent should know.** First, the illustrative
layout in `SYNCHRONIZATION.md` was wrong and is corrected there — object paths
published plaintext content hashes, envelope names published sequence ranges,
and a separate acknowledgement class duplicated what a contiguous vector already
says. Second, artifacts are named by what they logically are rather than by
their sealed bytes, because every seal draws a fresh salt; a publisher
republishes when its own copy is *unreadable*, not merely absent, which is what
repairs a torn artifact and what keeps a quiet carrier from growing. Third, a
round starts from what the journal remembers each peer acknowledged rather than
from what the folder says, so an empty or deleted carrier is not a standoff and
removable media converges in two trips. Measured: the carrier layer is about 1%
of an exchange — SQLite admission is the rest. `internal/synckeys` is the warned
`0600` development secret provider; v0.8 still owns the platform store, and G13
still owns real pairing.

G12 changed **no production code**: it is the evidence run that puts G11's
carrier on a real provider. Two measurements from it constrain later work and
now live in `SYNCHRONIZATION.md`: through a Google Drive `rclone mount`, another
device's change took **45-57 seconds** to become visible, and **resolving a
known name is no fresher than listing the directory**. So publication order —
envelopes first, advertisement last — is a latency optimization on such a
carrier and *not* a correctness mechanism; a phase makes the advertisement
visible without its envelopes and asserts the reader claims no progress it did
not make. G15 should not schedule polls faster than a provider announces
changes. `docs/operations.md` now carries the operator section, the safe
commands, and the never-run rclone verbs, whose refusal the harness enforces in
code rather than in a comment.

G13 advances the schema to **v25** and adds the first authenticated surface this
project has ever had. A peer principal is one enrolled replica of one database,
proved by an Ed25519 signature over the method, path, database id, replica id,
timestamp, nonce, and body hash — never a bearer token. It authorizes
`/api/v1/sync/...` for that database **and nothing else**; a test compares an
ordinary note route's answer with and without a peer credential and requires
them identical. **Three things a later agent needs to know.** First, peer public
keys are database state now (`sync_peer_keys`), not key-file entries, so
enrolment and revocation are transactional and audited, and the carrier's
verifier became `syncwire.MultiVerifier{own key, database peers}`. Second,
G11's clear-text development bundle is **gone**: pairing is a short-lived,
single-use code under which the group key travels sealed, and `sync
bundle|pair` were replaced by `sync invite|join|accept|enroll`. Third, the
transport policy is a **startup refusal** — with the surface enabled, a
non-loopback listener without TLS makes `notriosd` exit and name the setting.
No data plane landed; G14 owns it.

G14 adds the REST data plane and **no schema change**. `internal/syncrest`
implements G11's `Carrier` over G13's signing client, so REST and a shared
folder are one protocol with two couriers — transcript parity by construction,
asserted by comparing what each carrier holds. Measured, REST costs +15.4% at
100 notes and +0.07% at 500 against the folder, so keeping the merge off the
server costs nothing. `internal/syncbackup` packs, seals in fixed authenticated
frames, and extracts safely; a downloaded snapshot passes four gates in order —
declared hash, frames, sequential USTAR container, **then the strict physical
snapshot verifier** — and backups
are addressed by opaque id, produced only for an explicitly permitted replica,
and fetched only by the one that asked. **The thing a later agent most needs to
know:** G13's per-address failure budget was being spent on every request rather
than on refusals, which one request per authentication hid and a data-plane
round exposed immediately as a `429` against a legitimate peer. It is now
checked before work and spent only on refusal, pairing excepted, and the
per-peer request budget rose from 120 to 600 a minute.

The 2026-08-15 scalability amendment reopened only the **physical full-snapshot
representation**, not G14's security or transport contract. G14 exported loose
archive-v2 objects, then stored every one as a ZIP entry before frame sealing;
at 100 and 500 notes that added about 25%, almost entirely ZIP entry metadata.
G14a-G14e completed that review and froze the compatible physical default.
G15 may now start only after explicit user approval; it must preserve this
format split and must not turn synchronous backup creation into an unbounded
ordinary API operation.

That review also added a second portability route. v0.8 now investigates and
builds a framework-neutral Go application facade and versioned no-GUI C ABI,
with Android-emulator-only pre-1.0 evidence; v1.0 packages the supported ABI
matrix; and a post-1.0 Flutter client owns physical mobile and native-desktop
delivery. Dart FFI is not the Flutter Web bridge. `FLUTTER_GO_CLIENT.md` records
the API/lifecycle/ownership/stream contract and source checks. The current GUI's
Mermaid support is **disabled**, not merely untested (`noMermaid: true`), and a
v0.8 offline/security-tested enablement slice owns it.

Latest completed feature validation is G14 (2026-08-14): two replicas
converging over REST with an asserted identical transcript, `206`/`416` range
behavior, an interrupted snapshot download resumed, verified, restored under an
explicit intent and then continuing incrementally, a tampered snapshot refused
at the transport's own hash, a backup refused to a replica that did not ask for
it, and the syncbackup frame suite. The preceding validation is G13
(2026-08-13): the seventeen-case
authentication matrix, the syncauth binding/replay/skew/limiter suite, the
store's enrolment and single-use-invitation fixtures, ten transport-policy
rows, and a cross-process pairing over a live `notriosd`. The preceding
validation is G12 (2026-08-13), whose eight conformance phases ran the shipped
round against a mounted Google Drive folder and a drive passed between peers,
and before that G11
(2026-08-13), which added the
synccarrier suite — two- and three-replica convergence through a real folder,
carrier deletion and republication, truncation, unenrolled signers, unpaired
replicas, foreign namespaces, provider sidecars and wrong-case names, idempotent
publication, cleanup refusing another namespace, correctness with cleanup off,
an unavailable mount, the no-rename fallback, vanished and oversized entries,
stable listing order, concurrent writers, removable media, attachment bytes on
request — plus a three-test multi-process `notriosctl sync` fixture and a
carrier-wide assertion that no title, body, replica id, or database id appears
in any path or byte of the folder. The preceding feature validation is G10
(2026-08-13). It includes G5's
admission suite plus exhaustive 40,320-order and 250-seed convergence models,
opposite-order real SQLite replicas, sparse-register, membership, lifecycle,
tree-repair, clock-regression, restart/upgrade, and purge-gate checks, followed
by the audit-first full repository, frontend, docs, smoke, and release checks
recorded in its archive. Regular
validation begins with audit/fix/reinstall/re-audit, while CI and release
packaging enforce a non-mutating audit gate. G7 added its own suite on top:
the syncdelta round-trip/hostile/limit/fuzz set, the syncbody merge and
merge-base fixtures with 4,000 committed randomized merges, and real two-replica
store fixtures covering clean merges, durable conflicts, four broken-delta
refusals, tampered operations, six delivery orders, delete/edit, restore/edit,
and the v22 upgrade backfill, plus G8's syncassets chunk/manifest/policy suite
and real two-replica attachment fixtures covering hostile sources, resume,
dedupe, restart, and the v23 backfill, plus G9's independently generated goldens, RFC/NIST
known-answer vectors, exhaustive tamper cases, and two fuzz targets, plus G10's
permission, offer-selection, state-machine, floor, and password fixtures. Start
G11 only after explicit user approval, then stop after its validation, commit,
verified ZIP, and handoff before G12.

Two things a reader continuing this project should know about v0.6:

- **The v0.6 half-shipped job bullet is now resolved for sync.** Import/export/
  snapshot jobs remain watching-only because they name local paths. G15 adds a
  distinct closed, path-free start/control surface for incremental/resource
  sync only; it does not make arbitrary jobs remotely startable.
- **F7's reconciliation found three defects**, all fixed in it: the batch route
  applied `trash` and tag operations to notes in read-only notebooks that the
  single-note routes refuse, single-note tagging had no read-only guard at all,
  and six REST surfaces had neither an MCP tool nor a recorded reason.

Milestone detail follows. H1–H11 are archived under `plans/v0.3/`. The v0.4
slices are archived under `plans/v0.4/`:

- J1–J3: canonical Joplin RAW parsing, bounded relationship planning, and
  million-item transactional import throughput;
- Q1: bounded boolean/category search;
- P1: the shared selection/privacy planner;
- P2–P3b: the archive-v2 format and identity contract, streaming export, the
  large-library container revision, and the optional packed object layout;
- P4: verify and restore under mandatory intent;
- P5: stable external links and local resolution;
- P7: publication profiles and the privacy-reviewed handoff;
- P8: documentation and release wrap-up (version 0.4.0, the v0.4.0 release
  checklist, a verified Help reseed, and the v0.5 draft in `PLAN.md`).

P6, the `movenotes-v3` compatibility bridge, is deferred to v0.7 G19 and gated
on G9, which stabilizes the sync-era container capabilities it would pin.

Archive-v2 supports two object layouts. Loose `fanout` is the default and
deduplicates and resumes through the object tree. Opt-in `--pack` collapses a
382,206-note archive from 382,447 files to 46 at ~11% more disk and 1.29×
faster; the file-count collapse, not local speed, is what v0.7's REST and
folder/rclone transports need. That earlier conclusion is now a hypothesis to
retest end to end in G14b rather than the final default.

## First files to read

1. `AGENTS.md`
2. `README.md`
3. `PLAN.md`
4. `ROADMAP.md`
5. `CODING_CLIENT_HANDOFF.md`
6. `SYSTEM_ARCHITECTURE.md`
7. `API_SPEC.md`
8. `DATABASE_SCHEMA.md`
9. `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`,
   `RECOLL_INTEGRATION.md`, `SYNCHRONIZATION.md`, `DOCS_SITE.md`
10. `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`, `CONTEXT_MAP.md`
11. `agent/PLAN_STATUS.md`, `agent/ATTEMPT_LOG.jsonl`, `agent/MODEL_LOG.jsonl`
12. Release/history context when needed: `plans/mvp/MVP_RELEASE_REPORT.md`, `RELEASE_CHECKLIST.md`, `SECURITY_REVIEW.md`, `PACKAGING.md`

## Current state

The completed v0.1 MVP supports:

- `notriosd` local HTTP service;
- SQLite database creation, migration bootstrap, and default managed collection bootstrap;
- document create/read/update/patch/soft-delete;
- revision list/read/restore;
- SQLite FTS5 search over current non-deleted notes;
- content-addressed resource upload, metadata, content streaming, attachment, detachment, and safe delete;
- Markdown link/backlink parsing and graph slices;
- React/Vite UI using `md-editor-rt` with document/resource link routing;
- dependency-free MCP endpoint at `/mcp` (read-only default; editor writes);
- `notriosctl import joplin-raw` and `notriosctl import obsidian`;
- generated-dataset smoke/performance tests;
- release packaging and ZIP verification scripts.

The completed v0.2 redesign added notebooks/tags/search notebooks, source
provenance and threads, query language, optional Recoll, five importers, native
archive v1 interchange, Wails v2 GUI, and docs site. v0.3 added the
remote-media policy/scan/quarantine/localization surfaces, exact duplicate and
unreferenced resource reports, per-notebook resource usage, and an optional
review-only perceptual hook that is inert by default. H6 added schema-v8
resource retention state, configurable local retention, dry-run-first CLI
garbage collection, a read-only REST report, explicit confirmation for
permanent REST deletion, and a future sync-aware retention gate. Archive v1 is
not a full backup; P2 defines and verifies the v0.4 native archive-v2
full-snapshot/container layer reused by v0.7 sync, P3/P3a/P3b export it at real
library scale under two object layouts, and P4 verifies and restores it under
explicit replace/adopt/merge/fork intent. H7 added schema-v9 keyset
indexes and query-bound cursors for chronological/relevance, route-bound
notebook/Trash paging, live GET search, and bounded immutable snapshots for
optional FTS5/Recoll merging. Reproducible 10k/100k/500k evidence lives under
`performance/v0.3-h7/`. H8 added schema-v10 importer checkpoints and
fingerprints, exact optional source bundles, Joplin nested notebooks and stable
real tags, bounded batch lookups, dry-run/config parity, stable resource
refresh, interruption/resume, and generated 100/10k/100k evidence under
`performance/v0.3-h8/`. H9 applies the same generic state model to Obsidian:
nested vault notebooks and collision renames, exact Markdown/frontmatter and
non-Markdown source capture, alias/relative/embed/heading/block
canonicalization, stable resource refresh, resume/dry-run parity, and generated
100/10k/100k/500k evidence under `performance/v0.3-h9/`. H10 added schema-v11
durable projection retry/backoff, bounded drains, exact missing/stale/orphan
reconciliation, hardened cancellable Recoll processes/output, stable
deduplicated per-hit engine attribution, status/UI observability, and real
100k native Recoll evidence under `performance/v0.3-h10/`. H11 added the
cross-cutting maintenance guide (also reseeded into Help), reconciled living
specs and feature status, bumped product metadata to v0.3.0, completed the
release-candidate gates, and drafted the v0.4 plan. The 2026-08-02 follow-up
selected an external `movenotes-v3` archive bridge instead of duplicating
Obsidian/Quartz/Hugo publishing, implemented bounded boolean/category search,
and completed the J1–J3 Joplin prerequisite slices plus Q1. See
`agent/PLAN_STATUS.md`.

## Validation commands

Run these before committing any task:

```bash
go vet ./... && go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm audit && npm audit fix
cd web && npm ci && npm audit && npm run typecheck && npm run build && npm test -- --run
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/run_joplin_import_profile.sh 100 /tmp/notrios-joplin.json
bash scripts/run_real_joplin_profile.sh <label> <raw-export-dir> /tmp/notrios-joplin-real.json
bash scripts/run_obsidian_import_profile.sh 100 /tmp/notrios-obsidian.json
bash scripts/run_recoll_hardening_profile.sh 100 /tmp/notrios-recoll.json
```

GUI-affecting tasks also build with `make gui`; layout changes additionally run `scripts/verify_layout_resize.py` under Xvfb/Openbox (see `TESTING_POLICY.md`).

## Git workflow and state (reviewed 2026-07-26)

`main` takes reviewed merges; active work happens on `develop`. Commit each completed working-state slice; pushing to GitHub is the **user's step**.

Current handoff facts (verify again before acting):

- `origin` is configured (`https://github.com/renesugar/notrios.git`).
- `develop` review base was `26b0925`; it has no configured upstream.
- local `main` was `265ef4e` tracking `origin/main`.
- The v0.3 commits are local only. The user explicitly prohibited a GitHub
  push for this session.

Commit-message convention: each agent ends commit messages with its own `Co-Authored-By:` trailer, and appends its model to `agent/MODEL_LOG.jsonl` at session start (see `AGENTS.md`).

## Important constraints to preserve

- SQLite is the canonical managed-note database.
- Recoll is a derived, optional, external search sidecar — never canonical storage, never linked/vendored (GPL; see `RECOLL_INTEGRATION.md`).
- Project code must remain compatible with an MIT or Apache-2.0 license.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown and downloaded resources are untrusted; preview HTML must be sanitized.
- Remote media localization must go through media policy and quarantine checks.
- Exact SHA-256 is the only deduplication identity. Perceptual hashes are
  review suggestions only and no algorithm ships by default.
- Resource garbage collection must remain dry-run first, transactionally
  recheck references on apply, and pass future synchronization acknowledgement
  policy through `store.RetentionGate`.
- Every task must leave the repo in a working state; archive completed plans under `plans/`.
- Unbounded paging uses keysets/snapshots, not hidden offsets.
- Sync must follow `SYNCHRONIZATION.md` and the approved item in `PLAN.md`:
  canonical local stores, immutable operations/objects, contiguous state
  vectors, revision-aware merge, lazy resources, ephemeral-directory and REST
  adapters, secure snapshot catch-up, and acknowledgement-gated retention. The
  directory is disposable; rclone is a test carrier only, and `rclone sync` is
  not the merge algorithm.

## Local environment notes

Facts about the development machine that no other document records:

- `./data/` in the repo root holds a throwaway development database (`notes.sqlite` plus `assets/`, `quarantine/`, `projections/`, `search-index/`) created by ad-hoc service and GUI smoke runs. It is gitignored and safe to delete; the service recreates it on startup.
- `web/dist/` is gitignored; run `cd web && npm ci && npm run build` after a fresh clone (CI and `scripts/package_release.sh` build it too).
- `scripts/verify_layout_resize.py` needs Python `playwright` plus `xdotool`, `Xvfb`, and `openbox` (all installed system-wide here, but **no Python venv is committed** — create one with `python3 -m venv … && pip install playwright`; it can drive the system `google-chrome`, so no browser download is required).
- `npm test` under Node 22 prints a harmless `ExperimentalWarning: localStorage` — the real polyfill lives in `web/src/test/setup.ts` (explained in its comments).
- MCP write-tool tests may still exercise the deprecated `DefaultProfile`
  field as a compatibility case; new tests and configuration use
  `DefaultScope`/`mcp.default_scope`.
- The GUI before/after screenshots from the v0.2 conformance pass live in `/home/renes/prompts/` (outside the repo, intentionally uncommitted — transient browser-automation output is never committed).
- All verification servers, Xvfb displays, and browser sessions from prior agent sessions are stopped; session scratchpads lived under `/tmp` and are disposable.

## Environment limitations inherited from the scaffold

The scaffold was created in a restricted container. Still-open consequences:

1. The SQLite store uses a small local cgo adapter over system `libsqlite3`; the long-term driver choice is open.
2. The MCP adapter is dependency-free; the official MCP Go SDK can replace it later without changing tool semantics.
3. J1 matches canonical Joplin first-line titles and CR/LF-only metadata
   parsing, including OCR controls. J2 validates both supplied real exports and
   makes relationship planning proportional to parsed links. J3 adds bounded
   atomic canonical/checkpoint batches, an indexed temporary manifest, final
   links, and private-safe complete recipe-corpus evidence under
   `performance/v0.4-j3/`. Never commit private datasets or content-bearing
   evidence.
4. Q1 uses one bounded AST for uppercase `OR`, implicit `AND`, prefix
   negation, grouping, phrases, fields, and `category:`/`notebook:` aliases.
   SQLite is exact for every expression; supported shapes compile to Recoll
   with live parity tests, and unsupported sidecar shapes fall back explicitly
   to canonical SQLite rather than being approximated. Generated evidence is
   under `performance/v0.4-q1/`.
5. P1 provides one read-only Store/REST/MCP planner for recursive
   notebook/tag/query/explicit-ID selection. Target policies classify reachable
   resources and internal/private/broken links, hash source-bundle keys, strip
   paths/private metadata from API output, cap visible details, and bind the
   complete manifest to SHA-256. Generated 100k evidence is under
   `performance/v0.4-p1/`.
6. P2 adds schema-v12 stable logical database and per-writable-copy replica
   identities plus a separate read-only archive-v2 verifier. The manifest-last
   format uses strict typed JSONL records and immutable SHA-256 body/resource/
   source-bundle objects; schema/capability/MIME/size/count/path/depth and
   cross-reference checks complete before restore writes. Synthetic
   golden/adversarial fixtures live under `internal/archivev2/testdata/`.
7. P3/P3a/P3b make the container hold a real library: the object inventory
   lives in checksummed index chunks under an `ab/cd` fanout, the writer and
   verifier stream through external-sorted spools, and an optional `--pack`
   layout collapses file count behind the `objects.pack.v1` capability.
8. P4 adds `verify archive-v2` and `restore archive-v2 --intent
   replace|adopt|merge|fork`. Restore completes verification before its first
   canonical write, reads both layouts, re-hashes bytes at use, re-sniffs blob
   MIME, and records a schema-v13 `restore_state` marker so an interrupted
   restore cannot pass as a complete library. Resource and source-bundle
   coverage comes from the attachment-bearing Joplin corpus
   (`performance/v0.4-p4/`); neither recipe corpus carries attachments.
9. P5 adds the external `notrios://databases/{id}/documents/{id}` link, the
   strict `internal/stablelink` parser, the explicit `internal/profiles`
   registry (`~/.config/notrios/profiles.json`, override with `--registry` or
   `NOTRIOS_PROFILE_REGISTRY`), `POST /api/v1/links/resolve`, and the
   `notriosctl link|open|profile|register-url-handler` commands. Resolution is
   local routing only: it never contacts a peer, never scans the filesystem,
   and refuses rather than choosing when several profiles hold clones of one
   database.
10. P7 adds `notriosctl publish profile|plan|run`. A publication is a
    projection, not an archive of canonical state: current revisions only, no
    Trash/provenance/source bundles/saved searches, stripped revision metadata,
    links to withheld or unresolved targets rewritten, and their link records
    dropped. Publishing requires the digest of a reviewed plan and re-checks it
    before writing. Content-rewriting link actions remain refused for
    `full_archive`.
11. E4 replaced the MVP graph slice with bounded traversal: `POST /api/v1/graph`
    honours `depth` (it was declared and never read), `POST /api/v1/graph/path`
    finds a shortest path from both ends, and `GET /api/v1/graph/report` lists
    orphans, isolates, and in-degree hubs. A bound wider than a ceiling is
    refused rather than clamped; a traversal stopped by one reports
    `truncated_by` and `completed_depth`; and `no_path` is kept distinct from
    `depth_exhausted` and `budget_exhausted`, because only the first is a
    statement about the library.
12. E5 adds `GET /api/v1/links/suggest` (bounded title autocomplete returning
    IDs and titles only) and `POST /api/v1/links/check` (read-only resolution of
    an unsaved buffer, parsed by the canonical extractor so markers match what a
    save records). Schema v16 replaced `lower(title) = lower(?)` with a NOCASE
    index, which had made every title-resolved link a full scan of the document
    table on every save and every lint pass.
13. E6 weighed migrating to CodeMirror 6 and **declined**: `md-editor-rt` 6.5.3
    *is* CodeMirror 6 and exposes it (`completions`, `codeMirrorExtensions`,
    `getEditorView`, `domEventHandlers`), so the capabilities E5 wrongly recorded
    as unavailable were available all along. In-editor `[[` autocomplete, broken-
    link underlines, and Ctrl-click were implemented through those hooks for
    1.3 kB gzipped and no measurable typing cost. See `PROJECT_DECISIONS.md` 20.
14. E6a made the UI offline-capable. It had been fetching KaTeX, highlight.js,
    echarts, cropperjs, and prettier from `unpkg.com` at runtime — 13 requests,
    623 kB, on every launch — and math silently rendered as raw LaTeX without a
    network. Those are bundled or disabled now, `handleWebApp` serves a
    Content-Security-Policy, and `scripts/run_offline_assets_check.sh` fails if
    any of it returns. Cost: 151 kB gzipped and ~400 ms of first contentful
    paint, both measured.
15. E6b converts a pasted HTML table into a Markdown pipe table, so blocks, link
    extraction, and portable export can see into it. It refuses far more than it
    converts — merged cells, ragged rows, nested blocks, multi-line cells, a
    paste that merely contains a table — and every refusal falls through to the
    ordinary paste, so nothing pasted can be lost. Parsing is inert `DOMParser`;
    no HTML is re-emitted.
16. E7 renders a fenced ```note-query block through
    `POST /api/v1/note-queries/run`, which parses the block server-side with the
    same Q1 parser every search surface uses. A malformed block is a 200 with
    `error` so the note still renders. `SearchRequest` gained an explicit `Sort`
    because the order used to be implied by the query's shape. A publication
    carries the block's text, never a materialized result — asserted by test.
17. E8 puts trash-first deletion in the GUI — Move to Trash, Restore, Delete
    forever — and confirms a notebook deletion with the service's own
    `GET /api/v1/notebooks/{id}/deletion-preview`, including the re-homing rule
    that gives a later restore somewhere to land. `store.RenameTag`,
    `POST /api/v1/tags/rename`, and `notriosctl tags rename` add hierarchical
    tag rename whose **dry run is a rolled-back apply**: the real statements run
    inside a transaction, so a dry run and an apply cannot disagree. Dry run is
    the default on both surfaces. Bulk organizer operations remain v0.6.

(The formerly open "no browser testing" limitation is resolved: the GUI is browser-verified via Playwright, vitest/RTL covers the workspace, and `scripts/verify_layout_resize.py` covers native window resizing.)
