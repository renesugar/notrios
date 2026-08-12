# Plan Status

Updated: 2026-08-12 (G3 runtime profiles and multi-instance isolation complete; G4 next)

## Active milestone

**v0.1 through v0.6 are complete.** Product version is **0.6.0** and the
canonical schema is **v18**. v0.6 F0-F7 are archived under `plans/v0.6/`;
earlier milestones remain under their version directories.

`PLAN.md` is now the **active v0.7 native synchronization plan**. It contains
G0, G1, G1a, and G2-G20: evidence, profiles/local journal, state-vector convergence,
revision deltas/merge, lazy resources, secure container/catch-up,
ephemeral-directory and REST transports, jobs/UI/retention, shared-core/FFI/
Mermaid/mobile handoff, compatibility, and final validation. The user's
2026-08-11 review resolved every G0-G17 policy decision. **G0-G3 are complete**
and archived under `plans/v0.7/`; G4 is next and is not approved.

## v0.7 G3 completion — 2026-08-12

- Upgraded the owner-only stable-link registry to format v2 while preserving
  v1 reads and routing-only entries. Runtime entries bind a random local
  profile ID, database/replica IDs, and one generated owner-only config.
- Added `notriosctl profile create|show|list|validate|start`; show redacts the
  credential reference, start passes only `-config`, and the foreground
  launcher does not supervise or persist a process.
- Generated profiles resolve isolated absolute canonical/derived paths, a
  loopback listen address/public URL, and `sync.target none|directory|rest`.
  `none` is the first-class default; transfer remains unimplemented.
- Validation refuses exact/nested runtime path sharing, normalized loopback
  port collisions, stale config/database/identity bindings, and duplicate
  replica IDs. Raw copies require explicit adopt (same database, new replica)
  or fork (new database and replica).
- Service startup repeats the binding check. REST/UI report the active local
  profile, while replica ID stays out of REST. Stable-link open uses the
  selected profile's public URL and retains explicit ambiguity across valid
  replicas of one logical database.
- A real CLI integration fixture builds both binaries and runs two isolated
  daemons simultaneously. Product remains 0.6.0/schema v18; no journal,
  transport, MCP profile management, supervisor, scan, or dependency landed.

## v0.7 G2 completion — 2026-08-11

- Added an investigation-only pure-Go codec/resource harness under
  `performance/v0.7-g2/`; no production sync codec, schema, cryptography,
  transport, dependency, or mobile implementation landed.
- Compared canonical JSONL and compact canonical NCB1 at 100, 10,000, and
  100,000 generated aggregate-shaped operations. All six rows round-tripped
  exactly and deterministically; the 100k tier split into ten bounded candidate
  envelopes.
- Selected NCB1 plus a canonical-JSON outer manifest candidate because at
  10,000 operations it was 51.9% smaller raw, 16.3% smaller after deterministic
  gzip, decoded materially faster, and allocated about one-fifth as much as
  the JSONL prototype on the desktop proxy. G9 still owns promotion, goldens,
  per-kind payload schemas, cryptographic framing, and fuzzing.
- Fixed simultaneous envelope ceilings at 10,000 operations, 16 MiB canonical
  bytes, and 4 MiB compressed bytes, with 1 MiB record, 512 KiB payload,
  64-dependency, and 64:1 expansion sub-limits. Per-peer pending admission is
  10,000 operations/64 MiB disk-backed encoded bytes.
- Selected whole resources below 1 MiB and 1 MiB fixed chunks above it, up to
  16,384 chunks under the inherited 16 GiB resource ceiling. FastCDC remains
  deferred because static corpora contain no repeated binary-edit trace.
- Reused archive-v2 evidence showing 7,183×-8,314× pack file-count reductions;
  sync packs provisionally target 64 MiB, 4,096 objects, and 4 MiB trailers.
  Android limits remain provisional behind explicit v0.8 emulator and
  post-1.0 physical-device checklists.

## v0.7 G1a completion — 2026-08-11

- Added an investigation-only pure-Go matcher using 64-byte source blocks,
  rolling pseudo-Adler checksums, byte confirmation, and match extension, plus
  bounded constrained-VCDIFF and minimal-private-container serializers under
  `performance/v0.7-g1a/`.
- Ran 14 G1-compatible text fixtures and seven generated binary fixtures. Both
  formats reconstructed exactly and deterministically in all 21; VCDIFF was
  smaller than complete bytes in 19 and smaller than G1 line JSON in all 14
  text comparisons.
- Recorded the honest fallbacks: empty VCDIFF costs a five-byte header and
  unrelated 256 KiB random data was 22 bytes larger than complete content.
  Later code must require material benefit and retain the complete object.
- Demonstrated outbound interoperability: pinned Apache-2.0 xdelta3 and
  open-vcdiff builds decoded every representative Go-emitted stream exactly.
  The strict Go decoder accepted two of six external encodes and explicitly
  rejects valid compound opcodes outside its profile; it is not a general RFC
  decoder.
- Resolved the format recommendation to constrained RFC 3284 default-table
  VCDIFF, not Subversion svndiff or the private `NXD1` container. Kept the
  implementation spec-first and independently written with Apache behavioral
  attribution and no GPL-derived code.
- Kept the prototype under `performance/`. G2 owns production numeric limits;
  G7/G8 must separately approve a rewrite or deliberate promotion, require a
  real named parent and outer exact hashes, and preserve complete-object
  fallback. No production codec, schema, dependency, or mobile claim landed.

## 2026-08-11 pure-Go xdelta/VCDIFF plan amendment

- Added G1a as an independently approvable evidence spike before G2. It must
  distinguish Subversion's binary-safe xdelta matcher, Subversion's separate
  svndiff serialization, and RFC 3284 VCDIFF rather than treating them as one
  format.
- Resolved the runtime direction to Notrios-owned pure Go: no cgo, vendored
  C/C++, GPL-derived code, or external merge/delta dependency. External tools
  may serve only as test oracles.
- Kept xdelta/VCDIFF transport optimization separate from three-way conflict
  handling. G7 retains G1's bounded line-first/word-region behavior but owns its
  implementation; the earlier `epiclabs-io/diff3` recommendation is
  superseded.
- Made format interoperability, implementation provenance, and prototype
  promotion explicit non-blocking decisions with recorded defaults. The later
  approved G1a investigation resolved them as recorded above without adding a
  production codec, schema, or runtime dependency.

The roadmap no longer relies exclusively on Wails for mobile: v0.8 now includes
an evidence-first framework-neutral application facade, versioned no-GUI Go C
ABI/shared library, Android-emulator host smoke, and current-GUI Mermaid
enablement tests; v0.9 freezes the ABI candidate; v1.0 packages supported
library artifacts; and a separate post-1.0 Flutter client owns physical Android
and iOS validation plus native desktop clients. P6, the `movenotes-v3` bridge,
is v0.7 G19 gated on G9.

## v0.7 G1 completion — 2026-08-11

- Added reproducible aggregate-only corpus profiling and deterministic
  divergence/delta/merge workloads under `performance/v0.7-g1/`; 2,063,061
  bodies were profiled without committing paths, names, content, source hashes,
  databases, or resources.
- Reported one-hour/day/week/30-day results separately across 112 delta
  measurements, eighteen line/word/byte merge classifications, four 1 MiB
  scale probes, and seven rejected malformed/untrusted patch cases.
- Selected complete UTF-8 result objects plus optional beneficial named-parent
  line deltas. Missing/wrong bases, wrong result hashes, malformed/over-limit
  operations, and invalid UTF-8 fall back to the complete object rather than
  applying best effort.
- Selected bounded line-first three-way merge with Unicode-aware word-token
  refinement of conflict regions. Same-token and delete/edit overlap remains a
  durable typed conflict; concurrent same-element tag add/remove remains G6's
  ordered LWW case with both operations retained for audit.
- Probed the small MIT `github.com/epiclabs-io/diff3` candidate at exact commit
  `3b1669897fb1aa7c1fb2699a3c6a45bbb46e9ec1`; upstream and G1 UTF-8/conflict
  probes passed. It remains untagged and was not added as a dependency. The
  later G1a plan amendment supersedes it as a production candidate while
  preserving the probe as historical evidence.
- Preserved the evidence limit: static imports contain no independent-device
  history, so G1 makes no human concurrency/conflict-frequency or real
  revision-chain claim.

## v0.7 G0 completion — 2026-08-11

- Added the reviewed sync threat model, normative eleven-term glossary,
  thirty misuse cases traced to owning plan controls, and primary-source
  dependency/license/platform validation under `performance/v0.7-g0/`.
- Froze two security consequences later slices must implement: compromise
  revocation advances the encryption epoch for remaining peers, and
  carrier-visible addresses do not expose raw plaintext content hashes.
- Required domain-separated signatures over canonical outer artifacts and AEAD
  associated-data binding of the same visible header, while keeping signature,
  encryption, hash, enrollment, and replay state as separate controls.
- Preserved the honest current posture: no sync cryptography, enrollment,
  transport, REST authentication, or schema code landed. Current REST remains
  loopback/local only.
- Confirmed the Go standard Ed25519 package as the preferred primitive candidate;
  left AEAD/KDF suite and exact pins to G9/G2. Marmot, Cachapa, Ygo, Subversion,
  rclone, and minisign remain reference/test/rejected dependencies as recorded
  in the G0 matrix.

## 2026-08-11 synchronization decisions and Flutter/Go planning

- Recorded the user's selections for mandatory payload authenticated
  encryption, per-replica Ed25519 signatures, full-result-plus-optional-delta
  revisions, profile/enrollment/journal/convergence/conflict/resource/container/
  catch-up/transport/MCP/secret/retention policy, and marked them resolved in
  their owning items. Resolution is not implementation approval.
- Verified that Go `c-shared`/`c-archive` requires a main package and exported
  cgo symbols, and that cgo pointer ownership rules forbid the proposed client
  from retaining ordinary Go pointers. The planned ABI therefore uses opaque
  handles, bounded serialized calls, explicit output ownership, cancellation/
  polling, and stream handles over a shared application facade.
- Corrected the supplied cross-platform claim: Dart FFI targets Dart Native;
  Flutter Web uses JavaScript/Wasm interop and is not a consumer of the native
  Go C ABI. It remains a REST client unless a separate adapter is approved.
- Source inspection found current Mermaid rendering disabled, not merely
  untested: `web/src/editor-assets.ts` sets `noMermaid: true` after the offline
  asset hardening. v0.8 owns locally bundled, bounded, sanitized browser/Wails
  evidence before the feature can be enabled.
- `flutter_smooth_markdown` and `flutter_secure_storage` remain post-1.0
  candidates. Their package claims were recorded with explicit fidelity,
  security, platform-prerequisite, maintenance, and license gates rather than
  treated as architecture facts.
- Validation passed: `go vet ./...`, `go test ./...`, required-file and plan-loop
  checks, scaffold validation, frontend typecheck and all 155 tests plus build,
  docs-site build, `git diff --check`, and release ZIP creation/verification.
  `npm ci` still reports the pre-existing 1 moderate and 2 high advisories; this
  planning slice did not change dependencies.

## 2026-08-10 synchronization planning and consistency audit

- Replaced the seven-item sync draft with twenty-one resumable items. G0-G2 are
  evidence/decision spikes; G3-G20 separate identity, journaling, convergence,
  revision merge, lazy resources, crypto/container, catch-up, both transports,
  operations/UI/retention, installation handoff, compatibility, and wrap-up.
- Counter-proposed archive v2 rather than SVN dump as the wire container, while
  retaining the requested state-vector/change-log and base-delta model. A note
  revision binds a complete result object and may carry an optional transfer
  delta; three-way merge preserves disjoint edits and exposes overlaps.
- Recommended mandatory payload authenticated encryption plus per-replica
  Ed25519 signatures as separate G0 decisions. TLS remains required for
  non-loopback REST. No crypto control is described as implemented.
- Made the shared directory ephemeral/reconstructible and rclone a conformance
  carrier only. Added signed discovery/requests, snapshot catch-up/reset,
  authorized resumable REST backup download, password UI, multiple runtime
  profiles/server instances, and lazy resource materialization.
- Reconciled source truth across living docs: product 0.6.0/schema v18,
  baseline migrations through v17 plus the v18 bootstrap shim, MCP scope
  terminology, live blocks/batches/jobs/ranges, Reports protections, and the
  current Wails status (desktop beta, mobile experimental).
- Validation passed: `go test ./...`, `scripts/check_plan_loops.py`,
  `scripts/check_required_files.py`, `scripts/validate-scaffold.sh`, frontend
  typecheck/build, docs-site build, and `git diff --check`.

## Historical milestone notes

Four v0.5 decisions are settled and recorded as `PROJECT_DECISIONS.md` 17–20:
block identity is strictly content-based; lint/fix stays single-note and
revision-preconditioned with anything bulk left to the v0.6 organizer; a
heading anchor in a stable link is a slug rather than percent-encoded text; and
the editor stays on `md-editor-rt`, which *is* CodeMirror 6 and exposes it.

Three v0.5 roadmap bullets did not ship and moved to v0.6 rather than being left
ambiguous: note templates, task extraction, and a graph *view* (E4 delivered the
traversal, path, and report data it would be built on).

**v0.6 open decisions answered (2026-08-07).** Nine were answered by the user
and five new ones arose from those answers, which is the normal shape of this
rather than a failure of the first round.

- **F2: `administrator` is not reachable over MCP.** Notrios is single-user, so
  administrator and author are the same person; destructive whole-library
  operations stay a deliberate act on the command line. *Consequence:* there are
  **four** MCP profiles, not five — a profile that cannot be selected is not a
  profile, and `ROADMAP.md` was corrected. Also recorded in `SECURITY_REVIEW.md`:
  a profile is a **guardrail, not authorization**, because there is no second
  principal to authorize against.
- **New blocking decision from that answer:** "profile" already means two
  unrelated shipped things — `mcp.default_profile` (tool visibility) and
  `notriosctl profile register` (a named local database, v0.4 P5). The user's
  answer to the mid-session question was given in the *database* sense, which is
  the collision doing its work. Recommended fix: rename the MCP concept to a
  scope, keeping `mcp.default_profile` as a deprecated alias.
- **F3:** read-shaped surfaces become MCP tools; resource reads return metadata
  plus a URI. **New:** byte-range resource reads were requested, mirroring
  `get_note_line_range` — but they land on a deferred gap, since REST does not
  honour `Range` on resource content. Recommended: implement range in REST once
  rather than giving MCP a private mechanism. `PLAN.md`'s scope-control section
  was corrected, since it had listed range requests as out of scope.
- **F4:** placeholders are both prompted and automatic, with a **closed**
  vocabulary — an open one is an expression language, which E7 spent a slice
  refusing. Tasks are computed on read.
- **F5a is withdrawn and F5 reframed.** The user's counter-proposal is stronger
  than the investigation: no global canvas (Obsidian's degrades to a hairball
  past a few thousand notes while its local graph stays useful), and instead a
  local graph, a **Top N hubs report written as a note**, and **export** for
  tools built for large graphs. Measuring a thing in order to reject it is only
  worth doing when the rejection is in doubt. **Most of it already exists:**
  `GET /api/v1/graph/report` has ranked hubs by in-degree since v0.5 E4 — the
  metric the analysis recommends — so F5 is presentation, not computation, and
  needs no Go graph library despite several suitable ones existing.
- **F6:** persist job records without resuming work; cooperative cancellation.
  **New:** reproducing a job's command line — recommended from stored
  *parameters* rather than raw argv, which would capture local paths and any
  secrets from the command line into the database. **New scope boundary:** job
  dependencies are scriptable through queryable status and `--wait`, and
  **Notrios does not become a scheduler** — a DAG would add retries, cycles, and
  orphaned waits, each a new way for a note database to be unavailable.
- Deferred with an owner: multi-user roles and an `author` concept, now
  `agent/OPEN_QUESTIONS.md` 20–21. Notrios has no author field at all, and the
  Obsidian importer preserves neither tags nor frontmatter properties, so author
  data in an imported vault is dropped today regardless.

**Round 2 (2026-08-07): the remaining five answered, each taking the
recommendation.** Fourteen decisions are now settled across both rounds.

- **F2:** the MCP concept is renamed. `mcp.default_profile` becomes
  `mcp.default_scope` — a **tool scope** — with the old key kept as a deprecated
  alias, because a config key that silently stops applying a *restriction* would
  widen what an agent may do without saying so; if both appear the narrower
  wins and the service logs it. *Implementing it found the collision is
  three-way, not two:* `notriosctl profile register`, `notriosctl publish
  profile save`, and the MCP key. That strengthens the answer — the first two
  are both "a saved named configuration", and the MCP one is a permission tier
  wearing the same word.
- **F3:** REST gains `Range` on resource content first, and the MCP tool uses
  it. Defaults recorded: resource content only (a note body is bounded, and
  `/lines` and `/search-in` already do partial reads with better semantics),
  compatible with `?download=1`, and `416` with `Content-Range` on an
  unsatisfiable range rather than silently returning everything.
  `API_SPEC.md`'s deferred-gaps list was corrected — and while there, "bulk
  organizer operations" was removed from it, since F1 shipped them.
- **F5:** hubs report is a stable note ID, overwritten, read-only; export is
  **CSV node and edge lists**, **CLI-only** (`notriosctl graph export`).
- **F6:** `notriosctl jobs show --command` renders stored parameters, never raw
  argv; no scheduler, and a documented exit-code contract before any DAG. Noted
  for when that contract is designed: it must distinguish *succeeded*, *failed*,
  *still running*, and *no such job*, or a script cannot tell "not finished"
  from "failed".

**v0.6 is complete.** All eight slices — F0, F1, F2, F3, F4, F5, F6, F7 — are
archived under `plans/v0.6/`, including a copy of the plan itself at
`plans/v0.6/000-v0.6-plan.md`. Product version is **0.6.0** and the schema is
**v18**. `PLAN.md` now holds the v0.7 draft, and **no v0.7 slice is approved**.

**F7 is complete**, archived as
`plans/v0.6/007-documentation-and-release-wrap-up.md`. Its reconciliation found
more than bookkeeping:

- **`POST /api/v1/batch` applied `trash`, `add_tags`, and `remove_tags` to notes
  in read-only notebooks** that the single-note routes refuse with 403 — `move`
  and `duplicate` were guarded, the rest were not. Demonstrated live: a seeded
  Help note was tagged and sent to the Trash through the batch route. The guard
  now runs once before the dispatch, so a later operation inherits it;
  `restore` is the deliberate exemption.
- **Single-note tagging had no read-only guard at all**, and a tag outlives a
  `seed-help` reseed because `note_tags` is keyed by a stable document ID.
- **Six REST surfaces had neither an MCP tool nor a recorded reason**, despite
  F3 claiming every surface was decided. Each is decided now.
- **`tag_note` and `untag_note` were added at `editor`.** Tagging one note was
  reachable only through `run_batch` under `organizer`, so labelling a note you
  had just created required granting the ability to trash five hundred.
- **One roadmap bullet shipped half and now says so:** MCP watches bulk jobs but
  does not start them. It does not move to v0.7 — it is a decision, not an
  omission.
- Two reconciliation checks are now mechanical rather than read: every
  registered MCP tool appears in `docs/api/mcp.md` (39 of 39), and REST routes
  and OpenAPI paths agree in both directions (63 paths, no drift).

**F6 is complete**, archived as `plans/v0.6/006-job-control-plane.md`. Schema
v18 `jobs`; the two batching importers and archive export record into it;
`notriosctl jobs list|status|show|cancel`, three REST routes, and two read-only
MCP tools read and stop it. `GET /api/v1/jobs/{job_id}` stopped being a stub.

- **Narrower than the plan bullet in one place, and this needs the user's
  attention.** The bullet said "MCP may start and watch a job". **Nothing starts
  a job over REST or MCP.** Every kind names a filesystem path, and the standing
  v0.4 decision — restated in `SECURITY_REVIEW.md`, hardened in F3 and F5 — is
  that no REST or MCP surface accepts one. A job record around an operation does
  not change what the operation does. Cancelling is offered over REST and the
  CLI but withheld from MCP: safe for the data, but stopping a person's
  four-hour import is their decision.
- **`interrupted` is derived, never stored.** A process that dies cannot write
  its own epitaph. A sweeper that wrote the state would have to decide another
  process is dead, and `notriosd` plus `notriosctl` on one database would take
  turns declaring each other's work over. A merely-slow job can still finish.
- **Records persist; the work does not.** Verified on real data: a 4,000-note
  import cancelled from another process stopped at 1,225, and rerunning the same
  command started at **1,250** and finished all 4,000. The importer's own
  checkpoint is the resume mechanism; a second one would give two answers.
- **The existing `AfterBatch` seam was already the right place to cancel** — it
  runs after the batch is committed and checkpointed, and aborting it aborts the
  import. Export has no such boundary and uses context cancellation instead;
  **both classify as `cancelled`, never `failed`**, because they are one event
  arriving two ways. Mutation-verified.
- **Parameters stored, command rendered.** Raw argv would have captured local
  paths and any secret on the command line. A `path` flag per parameter is what
  lets the CLI print a command while REST and MCP return no parameters at all.
- **No scheduler; the exit codes are why.** 0/1/2/3/4/5/6 for succeeded, failed,
  usage, running, cancelled, no-such-job, interrupted. **6 is not 1** — an
  interruption needs the command run again, which a failure does not.
- **Two defects found while building:** listing was not actually newest-first
  (`CURRENT_TIMESTAMP` has one-second resolution, so the tiebreak fell to a
  random ID), and `jobs status <id> --wait` exited 2 with a bare usage dump.
  A third was found in the docs: a **duplicate `JobStatus` schema** in
  `openapi.yaml`, where the stale placeholder was silently overriding the new
  definition.

**F5 is complete**, archived as
`plans/v0.6/005-graph-views-that-stay-readable.md`. Three deliverables replaced
the global canvas: a **local graph** in the GUI (depth 1–2, ceiling stated), the
**hubs report written as a read-only note** in a new builtin **Reports**
notebook, and **CSV node/edge export** (`notriosctl graph export`, CLI-only). No
schema change and no migration — bootstrap's `INSERT OR IGNORE` reaches an
existing database on the next open.

- **`store.IsReadOnlyNotebook`, deliberately not `IsBuiltinNotebook`.** One
  predicate replaced thirteen hard-coded `NotebookID == HelpNotebookID`
  comparisons, so Reports was protected the day it was added. The naming is
  load-bearing: *undeletable* is Help, Reports **and Notes** (`nb.Builtin` **or**
  the default ID), while *read-only* is Help and Reports alone. Notes is
  `builtin = 0` because its content is the user's, and a test pins that it is
  **not** read-only — getting that wrong would silently drop most of a library
  from the report, from publications, and from lint.
- **The graph report was measuring something other than the live library**, and
  one of the two missing filters was a **pre-existing defect**: the in-degree
  subquery placed no condition on the link's *source*, and soft delete
  deliberately keeps `document_links` so a restore can use them — so a trashed
  note kept propping up everything it had linked to, and a note linked only from
  the Trash was never an orphan. The second filter is what lets the report live
  in the library it measures: it links to every hub it ranks. Both ends of every
  edge are filtered, so `link_count` finally means what it always claimed.
- **Mutation-verified:** restoring the original subquery makes both new tests
  fail with the exact numbers the defect produced.
- **Traversal follows the same predicate with one exemption** — a read-only
  note's own links are followed when it is the root being asked about, or every
  Help page and the report itself would render an empty graph.
- **Publication excludes read-only notebooks by default**, reported in the dry
  run as `read_only_notebook:<id>` and not overridable; a full archive must stay
  faithful and a subset transfer moves notes between the user's own databases.
- **Lint skips content checks in read-only notebooks but not
  `projection_backlog`** — indexing drift is an operational fact about every
  note. Visible change: **71 unresolved-link findings disappear** from this
  repository's own seeded Help notebook, and `report_sha256` changes with them.
- **Markdown escaping could not save a hostile title.** Notrios' link parser
  forbids `]` in link text *even escaped*, so the first implementation generated
  links the store could not resolve. A bracketed title now stays verbatim beside
  a short `([open](…))` link; both branches asserted.
- **Browser verification found a defect the tests did not:** the read-only badge
  was hard-coded to "Read-only Help note" and said so on a Reports note. It now
  names the note's actual notebook.

**F4 is complete**, archived as `plans/v0.6/004-templates-and-tasks.md`.
Templates are ordinary notes carrying a ```note-template block; tasks are
checkbox list items computed on read. No schema change and no migration.

- **The closed vocabulary is the safety argument.** A placeholder name is a
  declared `prompt:` or one of `date`, `time`, `datetime`, `title`, `notebook`;
  anything else errors and names the alternatives. An open vocabulary would be
  an expression language, which is what E7 spent a slice refusing.
- **Substitution is one pass, and that is a security property.** A supplied
  value containing `{{date}}` is inserted literally and never re-scanned, so it
  cannot introduce a placeholder, reach an automatic name it was not given, or
  recurse. Asserted directly.
- An unknown placeholder is an error **on the template, at listing time** — a
  typo discovered when someone tries to use the template is discovered too late.
  Malformed templates are listed with their error rather than hidden, since the
  author is who needs to find them.
- **Tasks are computed on read for a concrete reason:** `document_blocks` stores
  a hash and byte offsets but **not** block text, so there is nothing to query
  for "is this checked" — the body is read either way. A `LIKE` prefilter skips
  notes that cannot contain a checkbox, and `document_id`/`notebook_id` turn the
  whole-library scan into a small one.
- **The identity property has an honest limit, asserted in both directions.** A
  task keeps its block ID through edits *around* it (the test also asserts the
  ordinal moved, so it cannot pass vacuously). But the checkbox is part of the
  block's text — `- [ ] x` reaches the parser as `[ ] x` — so **ticking a task
  changes its derived ID**. That is the block model working as documented, and
  the answer is the one it already has: an author-written `^marker` outranks the
  derived ID and survives. The assertion is worded to fail loudly if the block
  model's contract ever changes.

**F3 is complete**, archived as
`plans/v0.6/003-mcp-read-coverage-and-ranges.md`. Seven read-shaped surfaces
became MCP tools under `read-only` — `get_document_blocks`, `get_graph`,
`find_graph_path`, `get_graph_report`, `run_note_query`, `get_lint_report`,
`read_resource` — and REST gained HTTP `Range` on resource content.

- **Every withheld surface was decided, not inherited**; the list *was* the
  deliverable. `docs/api/mcp.md` now carries a table with a reason per surface:
  fix, tag rename, notebook deletion, GC, archive operations, publication,
  purge. The pattern in one sentence: anything that writes outside the note
  model, deletes permanently, or acts on the whole library at once stays a
  deliberate act on the command line. Batch organizing is the exception that
  proves it — bounded to an explicit ID list and gated behind `organizer`.
- **Range came from `http.ServeContent`, not from an implementation.** The
  handler was doing `io.Copy`; it now type-asserts the store's `io.ReadCloser`
  to an `io.ReadSeeker` (an `*os.File` in practice) and delegates, so
  `OpenResourceContent`'s interface is unchanged and a non-seekable source falls
  back to a full copy without ranges. The test asserts `Content-Length`
  describes the *slice*, which is what proves the delegation overrides the full
  length the handler sets first.
- **`read_resource` returns metadata by default**; bytes only on request, only
  for text-like MIME types (an allowlist, not a "not binary" guess), only within
  `mcp.max_document_bytes`. A slice ending mid-character is trimmed so the text
  stays valid UTF-8 rather than handing a model a replacement character it would
  read as content, and the reader takes one byte past the limit so `truncated`
  can distinguish "there is more" from "that was exactly all of it".
- **Lint reaches MCP; fix does not.** Lint findings are content-free by
  construction — a location, a reason code, and a SHA-256 of the target, never
  the target text, because a broken wikilink's raw text is frequently a private
  note's title.
- **The F2 guard worked on its first real use.**
  `TestEachScopeListsExactlyItsTools` failed with a set diff the moment seven
  tools were added, because it asserts whole sets rather than spot-checking
  membership. And the lint leak test asserts a finding *exists* before asserting
  the report omits the target — checking only the absence would pass on an empty
  report.

**F2 is complete**, archived as `plans/v0.6/002-mcp-tool-scopes.md`. Four
cumulative MCP scopes — `search-only`, `read-only` (default), `editor`,
`organizer` — enforced **at the call site** from the same table that filters
`tools/list`, so the listing and the check cannot disagree. Before F2 only write
tools had a call-site guard; every read tool dispatched unconditionally, which
was latent rather than live only because no tier existed narrower than
read-only.

- `mcp.default_profile` → `mcp.default_scope`; the old key still works and the
  **narrower wins** on conflict. `config.Default()` now leaves both empty rather
  than pre-filling the deprecated one, which would have made every default
  config look half-migrated and warn for no reason.
- An unrecognized value falls back to `read-only` **with a warning** rather than
  failing closed — a typo silently disabling reads looks like a broken service,
  and the documented default plus a log line is debuggable.
- An **unclassified** tool is refused, never defaulted to the narrowest scope. A
  default would let a tool ship without anyone deciding how much trust it needs.
- `TestEveryMCPToolIsClassified` fails in both directions: a registered tool with
  no table entry, and a table entry naming no registered tool.
  `TestNoScopeReachesWholeLibraryOperations` asserts the standing v0.5 decision
  against tool names that do not exist yet, so adding one later fails the test
  and forces a deliberate choice.
- **Mutation-checked:** removing the call-site enforcement makes the hidden-tool
  test fail, and instructively — `append_to_note` executes and complains that
  `text is required`, which is exactly "hidden but answers when called".
- F1's batch surface arrived here as `run_batch` under `organizer`, which is
  where F1 said it belonged.

**Round 4 (2026-08-07): the last two F5 decisions resolved with the recommended
answers, and a third problem found while checking them.** Nineteen decisions
settled across four rounds; **nothing is blocking** and every task can start.

- **Corrected by the user before any code was written:** the predicate is
  **read-only builtin** notebooks — Help and Reports — never "builtin" loosely.
  The word covers two different sets and the existing code already needs both:
  *undeletable* is Help, Reports, and the default **Notes** notebook
  (`DeleteNotebook` checks `nb.Builtin` **and** `id == DefaultNotebookID`,
  because Notes is `builtin = 0` in the database), while *read-only /
  system-authored* is Help and Reports alone. Anyone implementing this by
  copying the deletion rule's pair of checks would have excluded every note in
  the default notebook — most of the library for most users — from the graph
  report, publications, and lint, **silently**. The predicate is to be named
  `store.IsReadOnlyNotebook(id)` rather than `IsBuiltinNotebook`, so a reader
  reaching for the familiar word is contradicted by the name, and a test must
  assert `DefaultNotebookID` is not in the set.
- **The graph report ignores links originating in a read-only builtin
  notebook.** It
  applies to the *whole* report, not just the hub ranking — a note that only the
  report links to would otherwise stop being an orphan, so `orphan_count` and
  `isolated_count` need the same filter or they quietly disagree with the hub
  list beside them. It also covers Help, whose notes link to each other heavily:
  **Notrios' own documentation has been inflating the in-degree of any note it
  referenced all along**, which is a pre-existing skew nobody had noticed.
- **Read-only builtin notebooks are excluded from publication handoffs**,
  reported in the dry run. Scoped to `publication_handoff` only, for reasons rather than by
  omission: a full archive is a backup and must be faithful, and a subset
  transfer moves notes between the user's own databases where their own Help and
  Reports notebooks are not a disclosure. Implemented as a rule in the
  publication target's default policy rather than a user-facing field — not
  something a user should configure, and not overridable in v0.6, because adding
  an opt-in later is easy and removing a leak is not.
- **New, found while checking:** the lint link scan filters on collection and
  `deleted_at` and nothing else, while `documentIsWritableLocked` refuses Help
  notes — so **lint already reports broken links inside Notrios' own
  documentation that `fix` structurally cannot repair and the user cannot edit
  either.** A stale hubs report would make this louder and more confusing.
  Recommended: lint skips notes in read-only builtin notebooks, same predicate;
  a finding nobody can act on is noise, not information. Non-blocking, default recorded.
- Also non-blocking, with a default: whether `POST /api/v1/graph` *traversal*
  ignores read-only-builtin-origin links as the report does. Default yes — otherwise every
  hub's local graph shows the report at depth 1, noise in exactly the view F5
  says stays useful at scale.
- An empty Reports notebook shows, consistent with All notes, Help, and Trash.
- **Second user correction, and it found a pre-existing defect.** The Trash is
  read-only too — but it is **not a notebook**: it is a search notebook, a saved
  query for soft-deleted notes (`snb_trash`, `is:trashed`), so a trashed note
  still belongs to whatever notebook it was in with `deleted_at` set. It can
  never appear in a notebook predicate. The point underneath is right though: a
  trashed note is read-only and its links should not count, which is a *state*
  filter independent of the *notebook* filter.

  Checking every surface against that axis: publication is already correct
  (`IncludeTrashed: false`), lint is already correct (joins `documents` on
  `l.source_document_id` with `deleted_at IS NULL`), and **the graph report is
  not**. Its row set excludes trashed notes so one is never ranked, but the
  in-degree subquery places no condition on the link's *source*, and
  `deleteDocumentLocked` deliberately leaves `document_links` intact so a
  restore can use them. **A trashed note therefore still inflates the in-degree
  of everything it linked to, and a note linked only from the Trash is never
  counted as an orphan.** Pre-existing defect, same shape as the report-note
  problem, found by following the same thread. F5 fixes both in one query.

*The round-3 entry below still says "builtin notebook" where it should say
"read-only builtin notebook". It is left as written: it records what was
recommended at the time, and this entry carries the correction forward.*

**Round 3 (2026-08-07).** A builtin **Reports** notebook, sitting above Help in
the last-anchored group, with the protection rule generalized from "is the Help
notebook" to "is a builtin notebook". Regeneration is explicit only. Sixteen
decisions are now settled across three rounds.

*Why the generalization matters:* **thirteen** places across `internal/httpapi`,
`internal/store`, `internal/localize`, and `internal/helpdocs` hard-code
`NotebookID == HelpNotebookID` to mean "protected". A single
`store.IsBuiltinNotebook(id)` predicate replaces all of them and makes the next
builtin free; adding a second constant to thirteen call sites would not. No
migration is needed — the `notebooks` table exists and bootstrap's
`INSERT OR IGNORE` reaches existing databases on next open.

**Two problems found while working out what that touches**, one of them
blocking:

- **The report participates in the graph it measures.** *Blocking.* A note
  linking to the top N hubs adds an incoming link to each, and the graph report
  counts *all* incoming links with no notebook filter — so generating the report
  changes the ranking the next generation sees. Excluding the report from its
  own ranking, already agreed, does not fix this: the links still count.
  Recommended: the report ignores links **originating in a builtin notebook**,
  which reuses the protection predicate and generalizes to any later generated
  note. Special-casing one note ID would leave the trap set.
- **A publication would carry the report, and the report names notes the
  publication excluded.** *Non-blocking, but a privacy boundary.* The hubs
  report draws titles and links from the whole library; publication excludes by
  *tag* only and has no notebook exclusion, so a broad selection sweeping in the
  report would publish titles the selection itself withheld — E7's
  export-inertness problem in a new shape. Recommended: builtin notebooks are
  excluded from publication handoffs by default and reported in the dry run.
  That also fixes a latent wart — nothing today stops a publication from
  dumping Notrios' own Help documentation into someone's site.

It blocked only F5's hubs-report deliverable, and was answered in round 4. All
of F5 shipped, and so has F6. **F7 is the last v0.6 item**, unblocked.

**Process correction (2026-08-07, from user feedback).** F1's two open
decisions were recorded in a milestone-level "Decisions required" section about
two hundred lines below the F1 item, and nowhere in the item itself — so the
task was approved without them being visible. An earlier draft of F1 *did*
carry the duplicate question inline; a conflicted rewrite of `PLAN.md` dropped
it and the milestone section did not compensate.

`AGENTS.md` gained a **"Writing plan items"** section making this explicit: an
item with an unresolved decision carries an `Open decisions` subsection inside
the item, naming the options, the recommendation, whether it blocks, and the
default that will be taken if no answer comes. An item whose *approach* is
uncertain — as opposed to its goal — gets an investigation slice before it, with
v0.5 E6 as the model. Milestone-level decision sections are an index, never the
only home; `agent/OPEN_QUESTIONS.md` holds only questions with no owning item.

Applied to the rest of v0.6: F2, F3, F4, and F6 now carry inline Open decisions,
**F2, F4, and F6 are blocking**, and **F5a** was added as an investigation slice
before F5 because how to render a bounded graph is an approach question that
cannot be settled on paper.

**F1 is complete**, archived as
`plans/v0.6/001-batch-organizer-transactions.md`. `POST /api/v1/batch` and
`store.RunBatch` apply one bounded organizer transaction — move, add_tags,
remove_tags, trash, restore, duplicate — over an explicit note list, in
`atomic` or `best_effort` mode. Schema is now **v17** (`batch_operations`).

**Both open decisions were taken rather than deferred**, since the task was
approved without answers. *Idempotency keys are database-scoped and persisted*:
an in-memory ledger forgets on restart, which is exactly when a batch is
retried, so the key, an argument fingerprint, and the first run's response are
stored — and a replay returns that response verbatim rather than recomputing it
against a library that has moved on. A key reused for different work is refused,
because answering it would hide a client bug. *A duplicate inherits content, not
identity*: body, notebook, tags, and resource references, but never the
`document_sources` row (two notes claiming one imported identity break re-import
and trip `duplicate_source_id`) and never the revision history.

Four item outcomes rather than two: `applied`, `skipped` (nothing to do),
`failed`, and `rolled_back` (succeeded, then undone by a later failure in an
atomic run). Conflating skip with failure makes "nothing to do" look broken and
overstates what a run changed; calling a rolled-back item "failed" blames it for
someone else's problem. Both modes report **every** requested item, and an
atomic report is never shorter than its request.

Atomic mode required extracting locked cores from five exported store methods
(`moveDocumentToNotebookLocked`, `addDocumentTagLocked`,
`removeDocumentTagLocked`, `deleteDocumentLocked`, `restoreDocumentLocked`),
since the exported ones each take the mutex and open their own transaction. The
tag helpers gained a "did anything change" return, which is what makes an honest
`skipped` possible.

Two defects caught before shipping: `ensureSchemaV17` was inserted *before*
`ensureSchemaV16`, so v16's `PRAGMA user_version = 16` ran last and bootstrap
reported the wrong version (caught by an existing status test); and `duplicate`
copied `document_resource_refs` naming a column `anchor` when it is
`anchor_json` (caught by reading the schema — the compiler cannot see inside a
SQL string). The atomic rollback test was verified to fail when the `ROLLBACK`
is replaced by a `COMMIT`.

Out of scope and recorded: stable Markdown-link copy (it produces clipboard text
rather than changing the library, so it is not a transaction), a GUI
multi-select (F1 is the API; the selection model is UI work F5 will want to
share), and MCP exposure (batch tools belong with the `organizer` profile, which
is F2 — adding them now would put bulk mutation on the default read-only
surface).

**E12 is complete**, archived as `plans/v0.5/016-readonly-title-control.md`. The
note title used `disabled` rather than `readOnly` when a note could not be
edited. Both refuse edits, but **`disabled` removes the control from the tab
order**, so a Help or trashed note's title could not be focused, scrolled with
the keyboard, selected, or copied — a title longer than the box was unreadable
by any means short of resizing the pane. Measured in a browser against a plain
non-React probe: `disabled` gives `focusable: false`, `readOnly` gives
`focusable: true`, with selection and scrolling otherwise identical. The CSS
selector moved with the attribute and gained `cursor: text`, since the control
is read-only rather than inert. The fixture types with `user-event` rather than
`fireEvent`, because `fireEvent.change` dispatches synthetically and sails past
`readonly` — it would have tested jsdom's laxness instead of the control. Noted
and deliberately not asserted: Chrome does not advance `selectionStart` on a
read-only input though the field still scrolls, reproduced on a plain input
outside React, so the obvious `press End; expect caret` test would fail for a
reason unrelated to Notrios.

**E11 is complete**, archived as
`plans/v0.5/015-finding-the-web-interface.md`. **The v0.5.0 candidate now has no
outstanding gates.**

- The GUI resolved `web/dist` relative to the process working directory, so
  `bin/notrios` run from `bin/` opened a window containing a JSON error whose
  message told the reader to rebuild assets that already existed. It now
  searches `--web-dir`, `server.web_dir`, the working directory, and the
  executable's own directory and its parent — the last two being what make
  `./bin/notrios` and `./notrios` both work. An explicit path is the *only*
  candidate, because a wrong explicit answer should fail rather than fall
  through to a directory that happens to work.
- The GUI refuses to start and names every directory tried, absolute and
  deduplicated; `-no-gui` starts anyway and says so (REST and MCP need no
  interface) and `-gui-only` is exempt entirely, since it renders what the
  remote service serves.
- **Two defects found while verifying, both invisible to unit tests.** The
  notebook control was appended after every built-in tool, and that toolbar
  scrolls horizontally, so it was off-screen at exactly the narrow widths where
  knowing your notebook matters; it now leads the toolbar. And a Help note's
  control read "Notes", because the label was looked up in the pickable options
  which deliberately omit builtins — so a note *in* one claimed to live
  somewhere else. The label is now resolved over the whole tree.
- Smaller: the dropdown got a border, caret, and a dashed disabled state; all
  seventeen `make` targets are documented with a network column; and
  `web/README.md` no longer refers to "Notes Companion" or `cmd/notesd`.
- Answered rather than changed: `make clean` does not remove
  `web/node_modules` — `make clobber` does, so a clean never forces a network
  reinstall. The target table says so on the `clean` row.

**E10 and F0 are complete**, done as one pass over the editor toolbar and
archived as `plans/v0.5/014-editor-toolbar-and-notebook-targeting.md`. E10 was
the v0.5.0 release-candidate toolbar fix; F0 was the first v0.6 task. They share
a component, a test file, and a browser sweep, so splitting them would have
meant restructuring the toolbar twice.

- The toolbar is now a title row plus an action row that stacks as a unit.
  Measured before and after across pane widths down to the 280 px minimum: the
  chip never shares the title's row, the rows at 280 px went 4 → 2, and the
  title width now decreases **monotonically** (it used to grow when the pane
  narrowed, because `flex: 1` expanded it into space a wrapped button vacated).
- **The container-query breakpoints sit above the measured content width on
  purpose.** `flex-wrap: wrap` is the base rule and the floor for an engine
  without container-query support; a breakpoint at the true fit-limit would
  leave a band of widths where `flex-wrap` wrapped one item raggedly before the
  all-or-nothing switch fired. Stacking slightly early makes the ragged state
  unreachable.
- "New note" moved to the search pane rather than being hidden on read-only
  notes: `onNewNote` had one caller, so hiding it would have stranded a reader
  on a library holding only the fifteen seeded Help notes.
- The notebook control lives in `md-editor-rt`'s own toolbar, verified rather
  than assumed — `defToolbars` plus a numeric `toolbars` entry take custom items,
  `DropdownToolbar` is exported, and the toolbar measured 35 px tall at both a
  300 px and a 900 px pane, so it scrolls instead of growing a row.
- Selection is tracked by notebook **ID**: a row's query is `notebook:"<name>"`
  and names are unique only among siblings, so `Contacts/Work` and
  `Personal/Work` share a query.
- The picker shows the **open note's** notebook, not the sidebar's selection, so
  reaching a note from "All notes" does not claim it lives elsewhere.
- `notriosctl notes move` closes the CLI gap and refuses an ambiguous notebook
  name rather than picking one.
- **A stub was passing over the feature**: every web fixture's `MdEditor` stub
  rendered only a textarea and dropped `defToolbars`, so the picker never
  reached the DOM. The stub now renders custom toolbar items — one that
  discards half the contract it stands in for will pass whatever is built
  against it.

`PLAN.md` now holds the **v0.6 plan** — notebook targeting and single-note move
(F0, complete), batch organizer transactions (F1), MCP
tool visibility profiles (F2), MCP coverage and resource reads (F3), templates
and task extraction (F4), a graph view (F5), a job control plane (F6), and the
wrap-up (F7). F0 is complete; **F1 onward need user approval before any code is
written.**

## 2026-08-02 follow-up review

- Compared the Go Joplin RAW importer with the real-data-tested
  `movenotes-v3/joplin2sql.py` and `notesdb.py` behavior.
- Found that Notrios' synthetic fixtures modeled `title:` metadata although
  canonical RAW stores titles on the first physical line; real imports could
  therefore use IDs as titles and retain duplicate title text in note bodies.
- Completed J1: CR/LF-only physical splitting, canonical title/body separation,
  unified ordered-property parsing, OCR control preservation, future/duplicate
  keys, delimiter whitespace, BOM support, and invalid-UTF-8 rejection.
- Completed J2: explicit per-type/malformed/unsupported inventory reporting,
  Markdown code-safe one-pass Joplin link rewriting, direct deduplicated
  resource relationships, unresolved-link counts, source-read-only CLI dry
  runs, and private-safe real-export evidence under `performance/v0.4-j2/`.
- The 1,237,553-item recipe dry run found 382,206 notes, exactly matching the
  corresponding Obsidian vault Markdown count. The 111,330-item attachment
  export planned 766 relationships over 763 resource records and reported five
  missing content files without silently dropping them.
- Completed J3: bounded canonical Store transactions with atomic checkpoints,
  an indexed temporary manifest/keyset spool, final cross-batch link pass,
  bounded notebook states, aggregate SQLite metrics, and private-safe complete
  interruption/resume/search/no-op evidence under `performance/v0.4-j3/`.
- The 1,237,553-item recipe profile imported 382,206 notes into matching
  document/revision/FTS/provenance rows and 842,813 tag relations. The full
  workflow completed in 58m11s; the 3.101 GB database was search-ready and a
  complete no-op added no revisions.
- Removed a correlated global tag count from the bounded document-membership
  query after a real 3 GB A/B measured 54.32s before and 0.42s after.
- Replaced duplicate Notrios Obsidian/Quartz/static-site work with a native
  archive-v2 handoff to a separately maintained `movenotes-v3/notrios2sql.py`
  importer. Movenotes owns Obsidian/Quartz and Hugo/Ledger+Bluge projections;
  Notrios owns selection/privacy/reachable-resource decisions.
- Completed Q1: one bounded AST for uppercase `OR`, implicit `AND`, prefix
  negation, grouping, phrases, fields, `category:` alias, and All-notes
  semantics; exact parameterized SQLite compilation, supported-shape Recoll
  compilation with explicit fallback, canonical cursor binding, recursive
  notebooks, and exact emoji handling.
- Live SQLite/Recoll parity covers grouped/negated fields, recursive category
  matching, authors, titles, and emoji. REST, MCP, search notebooks, GUI help,
  specifications, and generated scale tests all share the documented grammar.
- Completed P1: one read-only Store operation with typed recursive
  notebook/tag/query/explicit-ID selectors and reusable full-archive,
  subset-transfer, and publication-handoff privacy defaults. REST and MCP call
  the same planner with capped content-free details.
- P1 reports complete selected/excluded counts, exact reachable resources,
  internal/private/broken/external link decisions, hashed source-bundle keys,
  metadata preserve/strip decisions, warnings, and a deterministic manifest
  SHA-256 without returning bodies, bytes, source metadata JSON, or paths.
- Completed P2: schema-v12 persisted logical database and writable-copy replica
  identities; bootstrap/reopen stability and explicit replica rotation.
- Added the separate `internal/archivev2` manifest/typed-record contract,
  canonical commit digest, schema/capability negotiation, explicit
  replace/merge/fork/adopt identity decisions, and strict read-only directory
  verification before canonical writes.
- The complete synthetic golden archive covers all twelve record types and exact
  body/resource/source-bundle objects. Adversarial mutations reject absent or
  corrupt objects, traversal/symlinks/extras, unsupported compatibility,
  invalid MIME/counts/references/JSON, and bound overflows.

- H1: media-policy configuration and schema v7.
- H2: static remote-media scan, policy API/MCP, GUI decisions.
- H3: quarantine fetch with redirect/connect-time SSRF controls, size/MIME/hash
  checks, and attempt records.
- H4: shared localization engine exposed through REST/CLI/MCP/GUI/importer flag.
- H5: exact-hash/unreferenced/per-notebook reports through Store, REST, and
  CLI; pluggable perceptual admission/policy/report hook, inert by default.
- H6: schema-v8 retention state, dry-run-first local GC, read-only REST report,
  explicit permanent-delete confirmation, and a future sync-aware gate.
- H7: schema-v9 keyset indexes, query/sort-bound `k2` cursors, bounded stable
  merged-sidecar snapshots, live GET search, and reproducible
  10k/100k/500k performance profiles.
- H8: schema-v10 importer checkpoints/item fingerprints/exact source-bundle
  manifests; Joplin nested notebooks, stable real tags, bounded phases,
  dry-run/config parity, resource refresh, resume, and 100/10k/100k fixtures.
- H9: Obsidian nested vault hierarchy, conflict/config planning, exact
  Markdown/frontmatter/non-Markdown source bundles, bounded
  batches/fingerprints/checkpoints, canonical alias/relative/embed/anchor
  resolution, stable resource refresh, and 100/10k/100k/500k fixtures.
- H10: schema-v11 Recoll retry/backoff, exact reconciliation, hardened
  processes, stable attributed merges, status/UI, and 100k native evidence.
- H11: v0.3 documentation/release wrap-up, Help reseed, version 0.3.0,
  release-candidate validation, and verified packaging.

## 2026-08-04 session (Claude Code, Opus 5)

- Completed P3: `notriosctl export archive-v2` streams one SQLite
  read-transaction snapshot through the P1 planner into immutable SHA-256
  objects and publishes `manifest.json` last.
- Added a store-internal `ExportReader` contract (`internal/store/export.go`,
  `sqlite_export.go`) with bounded identity-scoped reads, visitor-streamed
  revisions/links, and blob/source-bundle content streams. It is deliberately
  separate from `Store` so no REST or MCP adapter can reach complete identity
  sets, raw source-bundle keys, or blob content.
- `ResolveSelection` reuses `planSelectionLocked`, so the archive binds exactly
  the `manifest_sha256` a dry-run `PlanSelection` reports for the same request.
- full_archive keeps trashed notes, complete revision history, provenance with
  private `metadata_json`, source bundles, all notebooks/tags, and builtin
  search notebooks. subset_transfer requires a selector, blanks private source
  metadata, exports only reachable notebooks plus ancestors and used tags, and
  omits whole-library search notebooks. Links to excluded targets are recorded
  as `target_excluded` with the target cleared, never as dangling references.
- publication_handoff and the `plain_text`/`redact` link actions are refused:
  both rewrite note content and belong to P7.
- Interruption semantics: objects and the manifest are staged in a sibling
  `<destination>.staging` directory and renamed in; an interrupted run leaves no
  manifest, a resumed run reuses published objects and prunes unlisted ones, and
  an archive that fails its own post-publication verification has its manifest
  removed.
- Generated 100/1,000/5,000-note evidence is under `performance/v0.4-p3/`:
  5,000 notes exported in 3.210 s, verified in 5.130 s, resumed with zero bytes
  rewritten, and used 46 MiB whole-process peak RSS.
- Recorded an open format bound rather than widening limits silently: one object
  per revision plus an inline manifest inventory caps an archive near 6,500
  objects (~6,400 notes), because the 4 MiB manifest bound binds before the
  nominal 10,000-object limit at ~645 bytes per descriptor. Archive v2 therefore
  cannot archive the supplied 382,206-note corpora at all.
- Resolved `agent/OPEN_QUESTIONS.md` question 17 as plan task **P3a**, then
  implemented it.

## 2026-08-04 P3a — archive-v2 large-library container revision

- The object inventory left `manifest.json` for checksummed index chunks
  (`application/vnd.notrios.archive-v2-index+jsonl`) listed in entry order.
  The commit digest binds each chunk hash and each chunk binds every object
  hash, so the checksum chain is unbroken while the manifest stays flat.
- Index chunks are named by the manifest, never by the index, so the inventory
  does not list itself. Entries are globally sorted by object hash across
  chunks.
- Every entry carries a discriminated `location`; `fanout` is the only layout
  this build writes, so a packed layout can arrive behind an optional
  capability instead of a second breaking revision.
- Objects moved to a two-level `objects/sha256/ab/cd/<hash>` fanout, matching
  the asset store and keeping directories near 25 entries at a million
  objects. Path order equals hash order, which makes pruning and file counting
  merges rather than set lookups.
- `objects.index.v1` is a required capability. Limits were re-derived from a
  1,000,000-note target: 8,000,000 objects, 1,000 index chunks of 10,000
  entries, 64,000,000 records, 1,000,000 notebooks; the manifest bound stayed
  at 4 MiB because it no longer scales with the archive.
- The writer keeps no object table: the published tree is the dedup index, and
  index entries stream through a 256-bucket external-sort spool.
- The verifier keeps only collections and the notebook tree in memory. Record
  identities and object hashes go to declaration/reference spools that are
  merge-joined per bucket. Composite keys fold consistency into the join — a
  revision key carries its document ID and a blob key its byte length — and
  "no extra files" is proven by counting rather than by a path set.
- The golden fixture is now produced by a generator that does not use the
  exporter, with a test asserting the committed fixture matches it byte for
  byte.

## 2026-08-04 P3b — packed object layout

- Added `pack` to the discriminated `location` union behind the optional
  `objects.pack.v1` capability. Packs are `kind: "pack"` index entries under the
  ordinary fanout, each ending in a self-describing trailer, so a pack verifies
  standalone and an object's identity never depends on where it is stored.
- The real 382,206-note corpus collapsed from 382,447 files to 46 and from
  48m26s to 37m28s — 1.29×, not the order of magnitude the fsync hypothesis
  predicted, because reading and hashing every revision body dominates both
  layouts. Packing costs ~11% more disk, so loose stays the default and
  `--pack` is opt-in; the file-count collapse is what v0.7's REST and
  folder/rclone transports need.
- An interrupted packed export restarts rather than resumes. The A/B also
  exposed and fixed a byte-accounting defect that double-counted packed objects
  against `MaxTotalBytes`.

## 2026-08-05 P4 — archive-v2 verify and restore

- `notriosctl verify archive-v2` and `notriosctl restore archive-v2 --intent
  replace|adopt|merge|fork` are live. Restore verifies an archive completely
  before its first canonical write, reads both object layouts, re-hashes bytes
  at the point of use, and re-sniffs blob MIME instead of trusting archive
  metadata.
- Schema v13 adds the durable `restore_state` marker, written before the first
  canonical row and cleared after the last, so a restore interrupted part-way
  cannot be mistaken for a complete library. Only `replace` recovers a marked
  library.
- The attachment-bearing Joplin corpus (111,330 items imported with
  `--preserve-source`) round-trips byte-identically under both layouts against a
  14-column aggregate including ordered `blobs.sha256` and
  `source_bundle_items.sha256` fingerprints. Evidence:
  `performance/v0.4-p4/`.
- Seven defects surfaced, six of them only at corpus scale or under fault
  injection: a quadratic restore lookup, `full_archive` dropping unreferenced
  resources, source bundles registered as ordinary blobs, a pack handle cache
  closing packs it was still reading, a partial library nothing marked as
  incomplete, container rows keeping the target's builtins, and the exporting
  schema version treated as a reader ceiling.
- Export deduplication became symmetric across layouts through the same bounded
  spool, and `record_counts` became a pointer so `omitempty` actually applies —
  which cut packed verify peak RSS 41% and runtime 31%.

## 2026-08-06 E9 — v0.5 documentation and release wrap-up

- Product version bumped to **0.5.0** across the service, CLI, MCP `serverInfo`,
  the web package metadata, and its lockfile. Schema stays v16.
- Documentation-site and Help-notebook pages caught up with E7 and E8: deleting
  and restoring notes plus notebook deletion in `docs/gui.md`; renaming a tag
  hierarchy and previewing a notebook deletion in `docs/operations.md`;
  `tags rename` in `docs/cli.md`; the tag-rename, deletion-preview, and
  note-query routes plus the trashed-note read contract in `docs/api/rest.md`;
  and new v0.5 highlights in `docs/index.md`. `notriosctl seed-help` mirrors the
  same directory, so the Help notebook and the site are the same source.
- `docs/operations.md` opened with "This guide covers the v0.3 workflows",
  which had been wrong for two milestones. Rewritten around the two rules the
  page actually follows: reports never write, and anything that writes is a dry
  run by default.
- **The MCP guide disagreed with the code.** Its read-tool list omitted
  `scan_remote_media` and its write-tool list omitted `localize_remote_media`,
  both of which are registered. Fixed, and a new section states what MCP
  deliberately does **not** expose — lint, fix, graph, blocks, query blocks, tag
  rename, notebook deletion, GC, archive operations, publication — as a standing
  decision rather than a gap.
- **Three v0.5 roadmap bullets did not ship**, and the reconciliation is the
  point of finding them: note templates and task extraction never entered
  `PLAN.md` at all, and a graph *view* was never in scope — E4 shipped the data
  and named itself "traversal, paths, and visualization data" for that reason.
  All three moved to v0.6 explicitly, in `ROADMAP.md`, `FEATURE_MATRIX.md`, and
  the release checklist's known boundaries, rather than being left ambiguous.
- **Two `ROADMAP.md` v0.6 bullets were already implemented**: LLM-safe surgical
  edits and LLM-safe SEARCH/REPLACE edits are the existing `edit_note` tool and
  `PATCH /api/v1/documents/{id}`, complete since R8. The v0.6 draft records this
  under "already implemented, deliberately not re-listed" instead of restating
  them as work and making the milestone look larger than it is.
- `RELEASE_CHECKLIST.md` gained a v0.5.0 section separating the reproducible
  release-candidate gates from repository-owner publishing, with the known
  boundaries stated plainly: no templates, no task extraction, no graph view, no
  bulk/batch operations, no GUI tag rename, no MCP maintenance surface, and no
  DevTools-Protocol measurement of the Wails webview.
- `TESTING_POLICY.md` gained the E8 entry, including the lesson browser
  verification taught: every unit fixture passed while the feature did not work.
- **The release archive shipped directories the release gate claimed it
  excluded.** `package_release.sh` excluded `data/*` and `check_release_zip.py`
  rejected `data/`-prefixed entries — both anchored at the archive root. Test
  runs leave a runtime layout beside the package that ran them
  (`internal/service/data/quarantine/`, `cmd/notriosctl/data/{projections,
  quarantine,search-index}/`), and every ZIP this project has produced carried
  those six entries past the check meant to reject them. They were empty, so
  nothing leaked — luck, not design: `*.sqlite` would have stopped a database
  but not a projection, a quarantine file, or a search index. `.gitignore` had
  it right all along, since a bare `data/` matches at any depth there. Both
  sides now match at any depth, plus `quarantine/`, `search-index/`, and
  `projections/`. Verified in both directions: the strengthened checker fails on
  the previously-built ZIP naming all six, and passes on the repackaged one
  (780 entries instead of 786).
- v0.5 archived as `plans/v0.5/000-v0.5-plan.md` and
  `plans/v0.5/013-v0.5-documentation-release-wrap-up.md`; `PLAN.md` now holds
  the **historical v0.6 draft** (F1–F7). This line records the state at that
  time; v0.6 is now complete.

## 2026-08-06 E8 — organizer UX: trash-first delete, restore, tag rename

- The GUI reaches the store's trash-first rule directly. The editor toolbar
  offers **Move to Trash** on an editable note; a trashed note opens with an
  "In the Trash" badge plus **Restore** and **Delete forever**. Before E8 the
  only way to undo a deletion was the CLI.
- **A trashed note does not share the Help note's badge.** Both are uneditable,
  but only one can be brought back, and "read-only" would hide exactly that.
- Deleting carries the revision the note was opened at, so a note edited
  elsewhere fails the precondition rather than being deleted out from under the
  other writer. A failed precondition is reported and the note stays open.
- Sidebar notebook rows carry a delete affordance, and only where the service
  would allow it. Clicking it asks the new read-only
  `GET /api/v1/notebooks/{id}/deletion-preview` and confirms with **that**
  answer: notebooks removed, notes that move to the Trash, and the notebook they
  are re-homed to so a later restore has a destination. A protected notebook
  previews as `deletable: false` with a reason rather than erroring — "what
  would happen" has an answer even when the answer is "nothing".
- **A dry run is a rolled-back apply, not a prediction.** `store.RenameTag`
  opens one transaction, runs the real statements, and rolls back when
  `dry_run` is set. Hierarchical renames cascade (renaming a child up onto its
  parent's name; renaming onto an occupied name merges), and cascading is where
  a separate predictor and applier drift apart.
- `dry_run` defaults to **true** on REST and CLI; only `"dry_run": false` /
  `--apply` writes. The CLI additionally exits 1 when a dry run's plan contains
  a merge, so a script that meant to rename does not silently combine two
  hierarchies.
- Hierarchy is matched by **path segment**, so `projects` is not a child of
  `project`, and folding is ASCII-only to match SQLite's `NOCASE` — which owns
  the unique index on tag names. A Unicode-aware fold would sweep up tags the
  database considers distinct.
- Renaming a tag into its own subtree is refused (the result would depend on row
  order); the remaining order-sensitive case is handled by processing
  shallowest-first, which frees the shallower name before the deeper tag needs
  it.
- A rename never rewrites note bodies (tags are relational) and never rewrites
  saved searches — it names them in `warnings`, because guessing which
  occurrences of a word are the tag silently changes what a search means.
- A rename enqueues projection upserts for every affected note, since the
  projection carries a note's tags and nothing else in the transaction would.
  A test asserts a dry run's rollback takes those outbox rows with it.
- The ceiling is 500 tags per rename, **refused rather than truncated**: a
  half-renamed hierarchy is worse than no rename.
- **Browser verification found a pre-existing defect**: `GET /api/v1/documents/{id}`
  returned 404 for a trashed note, so clicking a Trash row opened nothing. The
  codebase already disagreed with itself — `api.Document.Editable` is documented
  as false for trashed notes, `toAPIDocument` computes `deleted_at`, and
  `openStableLink` has a `trashed` branch that calls the document read — all of
  it dead. `store.GetDocumentIncludingTrashed` is a second, explicitly-named read
  used by exactly one caller (the REST document GET); `GetDocument` is unchanged,
  so writes and agent-facing reads still stop at the Trash. The read also had to
  start populating `DeletedAt`, which it selected and discarded. An existing test
  asserting the 404 now asserts the new contract plus the parts that must not
  change: still unwritable, still out of ordinary search.
- The client also stopped scanning remote media for a trashed note: localizing
  writes a revision a trashed note cannot take, so the scan could only produce an
  offer that must be refused.
- Out of scope and recorded as such: a GUI tag rename (E8 scoped rename to
  Store/REST/CLI), all bulk organizer operations (v0.6), and a projection
  enqueue on `AddDocumentTag`/`RemoveDocumentTag` — a pre-existing gap noted
  rather than widened, since adding one would put an outbox row per tag into
  every import.

## 2026-08-06 E7 — embedded query blocks

- A fenced ```note-query block renders as a live list of matching notes.
  `POST /api/v1/note-queries/run` parses the block and runs it through the same
  bounded Store search the search box, REST, and MCP use, so a block can express
  nothing its author could not already type into the search box.
- The block format is `key: value` lines, not the nested YAML
  `WORKSPACE_MAINTENANCE.md` sketched: the service carries no YAML parser, four
  directives do not justify adding one, and everything the sketch expressed as
  nested filters is already expressible in the Q1 query `query:` accepts. Keys
  are `query` (required), `fields`, `sort`, `limit`; an unknown key is an error
  whose message names the keys that work.
- **A malformed block is a 200 carrying `error`, not a 4xx.** The note has to
  render; only the block shows a problem. Every parser and query-language
  failure is a value on the result, with returned errors reserved for real
  storage faults.
- **`SearchRequest` gained an explicit `Sort`.** The order used to be implied by
  the query's shape — relevance for a positive text-only tree, chronological
  otherwise — so a block asking `sort: updated` over a text query would silently
  have got relevance. Sort is part of the cursor fingerprint, and an empty Sort
  keeps the previous behaviour exactly.
- Truncation comes from the search's own cursor rather than over-fetching by
  one: `NormalizeSearchRequest` clamps a limit at 100, so the extra row would
  have been silently dropped at exactly the block ceiling.
- Fields are opt-in with `title` always present — a query block is not a way to
  pull note bodies into a page that only wanted a list. Every rendered value is
  written as `textContent` or an attribute; a fixture asserts a title of
  `<img src=x onerror=…>` renders as text and sets nothing on `window`.
- A block never blocks the note: it renders "Running query…" in place, results
  arrive after, an unreachable service leaves a message inside the block, and
  navigating away aborts in-flight work so a late reply cannot write into
  another note's preview.
- **Export inertness is asserted, not assumed**: a published note whose block
  queries `tag:private` — the exact boundary a publication protects — comes out
  byte-identical with the fence intact and no trace of the withheld title.
- Out of scope and recorded as such: `links_to: "$current"` from the old sketch
  needs a link operator in the Q1 language, which touches the expression tree
  SQLite and Recoll both compile and belongs to its own slice.

## 2026-08-06 E6b — HTML table paste normalization

- A pasted HTML table rendered correctly but stayed raw HTML in the note source,
  where `markdownblocks` saw paragraphs, `markdownlinks` missed any `<a href>`
  inside it, and a publication handoff carried the HTML downstream. A `paste`
  handler on the editor now converts a simple table to a Markdown pipe table.
- **The refusals are the feature.** `merged-cells`, `ragged-rows`,
  `nested-block`, `multiline-cell`, `content-outside-table`, `multiple-tables`,
  `no-table`, `empty-table` — each returns a reason and falls through to the
  ordinary paste, so nothing a user pastes can be lost here. A mangled table is
  worse than an HTML one, because the HTML at least renders.
- Wrapper elements (`p`, `div`, `span`, `font`) are transparent because real
  pastes are full of them; a *second* paragraph or a `<br>` is multi-line
  content and is refused rather than flattened into a sentence nobody wrote.
- A link is emitted only when its href survives Markdown — a space ends an
  unquoted URL and an unescaped `)` closes the link early, the same rules E1a
  and E3 each met in a fixture. Otherwise the text is kept and the link dropped.
  `javascript:` is dropped outright.
- One implementation correction: a nested table was first reported as
  `multiple-tables`, because every `<table>` in the document was counted. Only
  outermost tables count now. The refusal was right either way; the reason code
  is the part a user reads.
- Parsing is `DOMParser`, which is inert — no scripts run, no resources are
  fetched — and no HTML is re-emitted. A fixture asserts a `<script>` and an
  `onerror` in a pasted cell leave no trace.
- Unit tests cannot cover the wiring, so the paste path was driven end to end in
  real headless Chrome with a genuine `ClipboardEvent`: a simple table became a
  Markdown table at the caret, a merged-cell table fell through to the
  plain-text flavour, and nothing executed. No scale profile — this is a
  per-paste transform with no library-size dimension.

## 2026-08-06 E6a — offline-first frontend assets

- Everything the editor would fetch is now bundled or turned off: local KaTeX,
  highlight.js (`lib/common`), and cropper instances; `noEcharts` and
  `noPrettier` for the two Notrios has no use for. Supplying an `instance` is
  what stops md-editor-rt injecting the tag — it skips both the script *and* the
  stylesheet, which is why `editor-assets.ts` imports the CSS explicitly.
- `handleWebApp` now serves a Content-Security-Policy. `script-src 'self'` is
  the directive that matters; `style-src` needs `'unsafe-inline'` because
  CodeMirror injects `<style>` elements, and `font-src` needs `data:` because
  the bundler inlines the smallest fonts. A test catches a regression; the
  header prevents one.
- Removed `remark-gfm`, `remark-math`, `rehype-katex`, and `rehype-sanitize` —
  declared, imported nowhere, and unusable with a markdown-it renderer.
  Removing `rehype-katex` is what made `katex` a direct dependency instead of
  something installed by accident.
- **Third-party requests 13 → 0, bytes 623 kB → 0.** The eager bundle grew
  151 kB gzipped (KaTeX 272 kB raw, cropper 37 kB, highlight.js common, plus
  260 kB of lazily-fetched KaTeX fonts).
- **First contentful paint regressed ~400 ms and DOMContentLoaded ~240 ms**,
  consistently across three runs. Reported rather than buried: it is the cost of
  parsing more JavaScript. Typing p50 is unchanged (30.0 → 29.7 ms); p95 went
  bimodal (124/56/127 against a tight 67/74/67) and is recorded as unresolved
  rather than averaged into a conclusion.
- The comparison flatters the before column, which had a fast connection to
  unpkg.com. Offline that column does not render math at all.
- `scripts/run_offline_assets_check.sh` + `check_offline_assets.mjs` is the
  guard: real headless Chrome, cache disabled, CDNs blocked, failing on any
  cross-origin request, injected remote script/stylesheet, CSP violation, or
  math that did not render. **Verified in both directions** — it fails on the
  pre-fix commit naming all 13 requests, and passes on the fixed tree. A check
  that cannot fail proves nothing. Evidence: `performance/v0.5-e6a/`.

## 2026-08-06 plan addition — E6a and E6b

A user question ("does the editor render KaTeX?") turned into two plan tasks.
Both were verified in a real browser rather than reasoned about.

- **Math renders, but only online.** `$…$` and `$$…$$` produce KaTeX output —
  3 `.katex` elements, `window.katex` an object. With the CDN blocked and a cold
  cache: 0 elements, `window.katex` undefined, and the formula silently renders
  as its raw LaTeX source. Code highlighting fails the same way.
- Cause: `md-editor-rt` does not bundle KaTeX. Its default config points at
  `https://unpkg.com/katex@0.16.33/...` and injects script/link tags at runtime.
  Loading one note issued **13 requests to unpkg.com** — katex js/css + 3 fonts,
  highlight.js + theme css, echarts, cropperjs js/css, prettier ×2.
- That is inconsistent with the project's own posture: the service binds to
  loopback, the media policy quarantines remote *images* and refuses to fetch a
  byte during a static scan, and meanwhile the UI loads remote *executable
  JavaScript* unconditionally, in the desktop app too. Recorded as **E6a**,
  including a CSP from the service so a regression is prevented rather than
  merely tested.
- **Corrected a user inference worth recording**: the dead `remark`/`rehype`
  dependencies do *not* mean links go unrendered. Verified in the browser —
  Markdown links render, `document://` gets `data-app-uri` and is intercepted,
  external links get `target=_blank rel=noreferrer`, Markdown and pasted HTML
  tables both render as tables, and `<script>`/`on*`/`style` are stripped.
  Rendering is markdown-it; sanitization is Notrios' own `normalizePreviewHTML`
  passed as the `sanitize` prop. `rehype-sanitize` is not involved and could not
  be. Recorded in `UI_DESIGN.md` so the inference does not recur.
- **E6b** added for HTML-table-to-Markdown conversion on paste. The argument is
  not cosmetic: a pasted table renders fine but stays raw HTML in the note
  source, so `markdownblocks` sees paragraphs, `markdownlinks` misses `<a href>`
  inside it, and a publication handoff carries raw HTML downstream. Simple
  tables only; anything with spans or nested blocks pastes unchanged rather than
  being mangled.

## 2026-08-06 E6 — CodeMirror decision: stay

- The task's premise was false and finding that out was the task.
  **`md-editor-rt` 6.5.3 is CodeMirror 6**: it depends on
  `@codemirror/{view,state,autocomplete,commands,language,search}` 6.x and
  exposes them through the `completions` prop,
  `config({ codeMirrorExtensions })`, `getEditorView()`, and
  `domEventHandlers`. So "migrate to CodeMirror to get source positions and
  inline widgets" has no content — we are on CodeMirror and they are available.
- **Corrected E5's claim** that the editor gives no caret position and no inline
  widgets. It came from reading part of the exposed interface and not the rest.
  The claim had reached `UI_DESIGN.md`, `PLAN.md`, `CODING_CLIENT_HANDOFF.md`,
  this file, `FEATURE_MATRIX.md`, and the archived E5 slice; all corrected.
- The prototype had to be real to be evidence, and it ran on the editor already
  shipping, so it was kept: `[[` autocomplete inside the editor, wavy underlines
  on broken links that map through document changes, and Ctrl-click via
  `posAtCoords`. No editor was replaced; the migration remains unapproved.
- `web/src/editor-offsets.ts` converts the service's UTF-8 byte offsets into
  CodeMirror's UTF-16 indices. They agree on ASCII and diverge on the first
  accent, so an unconverted offset underlines the wrong text. Offsets landing
  inside a character are dropped rather than rounded.
- `@codemirror/{autocomplete,state,view}` became direct dependencies pinned to
  the versions already installed. Verified empirically: one copy of each on disk
  and a byte-identical build, because two copies of `@codemirror/state` break
  CodeMirror at runtime.
- Measured in real headless Chrome on a 206,549-character note, three runs per
  arm: keystroke p50 30.9 → 30.0 ms, p95 71.4 → 67.4 ms, eager bundle 259.64 →
  260.94 kB gzipped. **1.3 kB and nothing measurable** — stated as "nothing
  measurable" rather than "faster" because first paint alone ranged 160–528 ms
  across three identical before runs.
- What migrating would still buy: dropping `@codemirror/language-data`, which is
  113 lazy chunks, 1.32 MB raw / 480 kB gzipped — 61% of the distribution by
  size and 97% by file count. Lazily loaded, so distribution size rather than
  first paint; not enough against re-implementing preview, sanitizer, toolbar,
  upload, and theming.
- **Not measured, and the task asked for it**: behaviour inside the Wails
  webview. WebKitGTK does not speak the DevTools Protocol the harness uses and
  no WebKit inspection tooling is installed here. `make gui` builds and the
  webview loads the identical bundle, so the code path is the one measured — the
  engine is not.
- Also noted: `remark-gfm`, `remark-math`, `rehype-katex`, and `rehype-sanitize`
  are declared in `web/package.json` and imported nowhere.
- New harness: `scripts/run_editor_profile.sh` + `scripts/measure_editor.mjs`,
  a CDP client written against Node's built-in WebSocket so measuring the editor
  added no dependency. Evidence under `performance/v0.5-e6/`.

## 2026-08-06 E5 — editor-pane link intelligence

- `GET /api/v1/links/suggest` is the bounded autocomplete an editor calls while
  typing: title-prefix matches first from an index range scan, then bounded
  interior-word matches from FTS5's `title` column so "plan" finds "Kitchen
  Plan". A suggestion is an ID, a title, and the canonical URI — never a body.
- `POST /api/v1/links/check` classifies the links in an **unsaved buffer** and
  writes nothing. It takes the body rather than a client-extracted target list,
  because deciding what is a link belongs to the canonical extractor; a
  TypeScript reimplementation would draw markers that disagree with the link
  records a save writes. A test asserts check and save agree link for link.
- Anchors into the note being edited resolve against the submitted body rather
  than the saved blocks: while someone types, the buffer is the truth about its
  own headings, and checking a just-typed `#new-section` against yesterday's
  rows would mark a correct link broken.
- Schema **v16** adds `documents(collection_id, deleted_at, title COLLATE
  NOCASE, id)` and the matching resource filename index. Title lookup ran
  `lower(title) = lower(?)`, which no index can serve, so every link that
  resolved by title was a full scan of the document table — once per link, on
  every save and every lint pass. NOCASE is the same comparison (SQLite's
  `lower()` folds ASCII only) and makes it a probe; it also turns
  `title LIKE 'prefix%'` into the range scan that bounds the suggestion endpoint.
- The profile measures the index rather than asserting it: each tier drops it,
  re-measures, and restores it. Indexed suggestion and buffer check are flat at
  0.51/1.03/0.44 ms and 1.40/8.30/1.40 ms p95 across 10k/100k/500k; unindexed
  they are linear at 41.6/1,324/5,390 ms and 19.5/1,000/4,198 ms — 5.4 seconds
  per keystroke at half a million notes, a cost every title-resolved link was
  already paying on every save and every lint pass. Evidence under
  `performance/v0.5-e5/`. One measurement is honestly not flat: a query matching
  nothing runs both passes to exhaustion and grows 0.62 → 5.83 ms, which is the
  FTS5 term dictionary growing; recorded rather than omitted.
- One latent defect fixed: `resolveLinkCandidateLocked` called an anchor-only
  link resolved even with no source document, producing the ID-less URI
  `document://default/documents/`. Unreachable from the save path, reachable the
  moment a caller could check a never-saved buffer.
- The web client gained a link picker that inserts a canonical URI at the caret
  and a located list of links that will not open. Both debounce, cancel the
  request they superseded, and degrade to nothing on error rather than to an
  error banner.
- E5 also recorded that `md-editor-rt` exposes no caret position and no inline
  widgets. **E6 found that wrong** and corrected it everywhere; see below.

## 2026-08-06 E4 — graph traversal, paths, and visualization data

- `POST /api/v1/graph` now honours `depth`. It had been declared in
  `api/openapi.yaml` and carried through two request structs since the MVP while
  `store.Graph` never read it, so every graph slice was the roots' immediate
  neighbours and a caller asking for three hops had no way to notice.
- Added `POST /api/v1/graph/path` (shortest path, searched from both ends) and
  `GET /api/v1/graph/report` (orphans, isolates, in-degree hubs). Store
  operations are `Graph`, `GraphPath`, and `GraphReport` in
  `internal/store/graph.go` and `sqlite_graph.go`. No MCP tool and no CLI: MCP
  profile expansion is v0.6's and this is a client-rendering feature.
- Ceilings are depth 5, 100 roots, 5,000 nodes, 20,000 edges, 10 path hops, and
  200,000 path visits. A request naming a wider bound is refused rather than
  clamped, because clamping reproduces the original defect in another form.
- A traversal stopped by a ceiling reports `truncated_by` and `completed_depth`,
  so a partial neighbourhood is never read as a complete one.
- The three ways of not finding a path are kept apart: `no_path` proves that
  everything reachable was searched, while `depth_exhausted` and
  `budget_exhausted` only say the search stopped.
- Expansion is level by level in Go with one bounded `IN (...)` query per 400-ID
  batch, not a recursive CTE: a recursive SQL walk decides how far it has gone
  only after SQLite has already walked, so the ceiling would bound the result
  rather than the work. Both frontier queries are covering index searches.
- Node metadata is batch-loaded (id, collection, title). Reusing
  `getDocumentLocked` would have pulled a note body per node.
- The API response became typed: `nodes`/`edges` were `[]map[string]any`, which
  is how a declared field could go missing without anything noticing.
- Cost at 10k/100k/500k: the bounded operations are flat across a fifty-fold
  library — depth-1 1.04/0.95/0.72 ms p95, depth-5 18.6/20.4/19.2 ms, path found
  2.2/1.9/2.0 ms — while the whole-library report is linear at 0.21/3.92/21.6 s,
  the same shape and order as lint on the same run. Peak RSS at 500k is unchanged
  at 376 MB. Evidence under `performance/v0.5-e4/`, with the caveat recorded
  beside the numbers: the generated library is a circulant graph whose frontier
  grows linearly, so traversal timings are a floor rather than a worst case, and
  the upper bound comes from the ceilings rather than from this measurement.

## 2026-08-06 document reconciliation

- Compared every living document with the source tree before starting E4.
  Corrected three "v0.5 is unstarted" claims (`README.md`, `ROADMAP.md`,
  `CONTEXT_MAP.md`), two stale schema numbers (`DATABASE_SCHEMA.md` said the
  migration creates through v14, this file said the schema is v13; both are v15),
  and the evidence listing in `README.md`.
- Found one real API/implementation gap and recorded it in `PLAN.md` E4 rather
  than patching it separately: `POST /api/v1/graph` accepts `depth` and
  `store.Graph` never reads it, so every graph slice is depth 1, and the OpenAPI
  node/edge defaults (250/500) disagree with the store's (100/200).
- Everything else matched: REST routes, the eleven lint check names, the MCP
  tool list, the blocks response fields, and the archive-v2 CLI surface.

## 2026-08-06 E1b — scheme-scoped anchor decoding

- Fixed a defect E1a left: `stablelink.Parse` accepted `#Kitchen%20Plan` and
  returned an anchor that could never resolve, because `validateID` refuses
  escapes but `validateAnchor` did not.
- The scheme is now the declaration. Percent-escapes are read as escapes only
  inside a `notrios://`, `document://`, or `resource://` link — where something
  declared itself a URI and RFC 3986 already defines `%20`. A bare Markdown
  anchor stays literal.
- A new prefix marker (`notrios+q:`) was considered and rejected: it would not
  help a pasted Obsidian URI, which carries no Notrios prefix, and it would add
  a second link spelling to the 18 non-test files that interpret a target or an
  anchor. Notrios already has Joplin's `:/` equivalent in its schemes.
- Targets stay literal: a wrong decode there opens the wrong note, while a wrong
  decode in an anchor lands in the right note and lint reports it. That
  asymmetry is why targets would need an explicit marker and anchors do not.
- Invalid escapes are left exactly as written and `+` is not a space; `%%20`
  decodes to `% `, matching every lenient decoder. `Parse` still returns the
  anchor as written so links round-trip byte for byte.
- Block anchors were deliberately left comparing in SQL: block IDs and markers
  are letters, digits, and dashes, so nothing in that charset needs escaping.
- Recorded as the amended `PROJECT_DECISIONS.md` 19: emit slugs; accept
  percent-encoded anchors inside URI-schemed links only.

## 2026-08-06 E3 — workspace fix

- `notriosctl fix` repairs the mechanically safe subset of what lint reports.
  Dry run is the default and prints the exact before/after of every edit.
- Each note is repaired against the revision its plan was computed from, with
  per-note outcomes rather than one transaction; a note edited in between fails
  and its content survives. Every fix writes an ordinary revision, so it is
  visible in history and revertible.
- `non_canonical_link_target` (a link that resolved by title or filename becomes
  the canonical URI) runs by default. `missing_alt_text` is opt-in because a
  filename is a starting point rather than a description, and
  `unlocalized_remote_media` is opt-in because it reaches the network — it runs
  the existing localization engine, so media policy, SSRF blocking, size/MIME
  checks, and hash rules all apply.
- Spans whose bytes are not what the plan recorded are skipped, never applied at
  an arbitrary position. Wikilinks are left alone: rewriting them replaces the
  author's syntax rather than repairing it.
- Stale link reference definitions are deliberately not fixed: they are not
  detected either, since reference definitions are outside the link extractor.
  Check and fix belong together in a later slice rather than fixing blind.
- Another Markdown lesson from a fixture: `[x](Kitchen Plan)` truncates at the
  space, so title-resolved Markdown links are the space-free ones — the same
  rule that decided E1a's slug form.

## 2026-08-06 E1a — heading anchors in stable links

- Schema v15 `document_blocks.heading_slug` with a partial index. A heading
  anchor now has something to compare against, which is what E2 said was
  missing.
- Obsidian's model was checked against its own documentation: headings by text,
  blocks by `^id`, both usable in `obsidian://open` with percent-encoded
  anchors. Notrios takes the model but not the encoding — P5 refuses
  percent-escapes, so a stable link carries the slug and resolution normalizes
  heading text to it. Recorded as `PROJECT_DECISIONS.md` 19.
- Two claims in the prompt's source material could not be verified and were not
  relied on: an Alt/Option "Copy obsidian URI" menu item, and `heading=`/`block=`
  parameters in the Advanced URI plugin.
- Precedence is author marker, then block ID, then heading slug. Repeated
  headings are disambiguated (`notes`, `notes-1`); a heading that slugs to
  nothing gets no slug rather than an invented one.
- The `unresolved_heading_anchor` lint check is restored. It runs in Go rather
  than SQL — matching an anchor means slugifying it with the parser's Unicode
  rules — riding E2's single link scan with a bounded 512-document slug cache.
- `notriosctl link --anchor` accepts a slug, heading text, a `^marker`, or a
  block ID and refuses an anchor that does not resolve; `--list-anchors` shows
  what a note offers.
- A fixture taught a real Markdown rule worth documenting: a space ends an
  unquoted URL, so `[x](…#Install & Setup)` truncates at the space and the
  heading-text spelling belongs in a wikilink. Independent support for the slug
  decision.
- Cost at 100k notes: nothing measurable. Lint 3.20 s → 3.05 s, save 4.17 ms →
  4.16 ms, database 114 MB unchanged, anchor resolution 0.276 → 0.404 ms p95 —
  the last two within run noise and reported as measured. Evidence under
  `performance/v0.5-e1a/`.

## 2026-08-05 E2 — workspace lint

- Ten read-only checks — broken document/resource links, ambiguous wikilinks,
  unresolved block anchors, duplicate external identities, missing titles,
  unlocalized remote media, missing alt text, unreferenced resources, and
  projection backlog — through `notriosctl lint` and
  `GET /api/v1/admin/lint/report`. No apply surface anywhere; fixing is E3.
- Findings locate rather than quote: document/resource ID, line, column, reason
  code, and a SHA-256 fingerprint of the target. A broken wikilink's raw text is
  frequently a private note's title, so it stays in the note.
- Every check streams its rows, folding each into the digest before discarding
  it, so counts and `report_sha256` describe the whole library at any detail cap
  and memory stays flat.
- `notriosctl lint` exits 0 clean and 1 with findings, so `--quiet` works in a
  hook.
- Heading anchors are deliberately unchecked: a heading anchor is a slug and
  block rows store a content hash rather than heading text, so there is nothing
  to compare against. Checking them needs a stored heading slug — a schema
  question for its own slice, recorded rather than guessed at.
- Two fixture corrections came from real behaviour: `![alt](x)` records as an
  `embed` rather than an `image`, which had left the alt-text check finding
  nothing; and `CreateDocument` substitutes "Untitled" for a blank title, so the
  missing-title state only arises from an importer or an interrupted restore.
- The first implementation ran each check separately and measured 61.3 s at
  500,000 notes. Per-check timings showed six checks each scanning
  `document_links`; they now share one ordered scan, taking 500k from 61.3 s to
  18.9 s and 100k from 10.9 s to 3.2 s with every lint test unchanged. Per-check
  timings stayed in the report.
- Full lint: 0.34 s / 3.20 s / 18.9 s at 10k/100k/500k, peak RSS flat against
  the same tier without lint. Evidence under `performance/v0.5-e2/`.

## 2026-08-05 E1 — block anchors and block-level addressability

- Schema v14 `document_blocks`: content-derived IDs, kind, heading level,
  authored marker, content hash, byte range, and ordinal, rebuilt from the body
  in the same transaction as the save that produced it. The migration file now
  creates through v14 and also folds in v13's `restore_state`, which had lived
  only in a shim.
- `internal/markdownblocks` splits headings, paragraphs, list items, fenced code
  blocks, and tables, bounded at 10,000 blocks per note.
- Identity follows the decision exactly: the hash covers document ID, kind,
  normalized text, and occurrence among identical blocks. Moving a block keeps
  its ID; editing its text mints a new one. Line endings and trailing
  whitespace are normalized first, so an editor that tidies a file on save does
  not break every anchor in it. The same sentence in two notes is two blocks.
- Author-written `^markers` are stored separately and resolve first: they are
  names the author chose and they survive edits the derived ID does not.
- `GET /api/v1/documents/{id}/blocks` lists blocks with per-block backlink
  counts and returns no block text. `notrios://`/`document://` anchors resolve
  to blocks, and a new `stale_anchor` status reports the note being present
  while the block is not — the visible consequence of content-based identity.
- Generated 10k/100k/500k evidence under `performance/v0.5-e1/`.

## 2026-08-05 P8 — v0.4 documentation and release wrap-up

- Bumped service, CLI, MCP, and web metadata to 0.4.0; the schema bootstraps and
  upgrades to v13.
- Added the v0.4.0 release-candidate and owner-publishing sections to
  `RELEASE_CHECKLIST.md`, including the boundaries a reader needs: a publication
  is a projection rather than a backup, no external tool is yet verified to
  consume the handoff, `--pack` requires a reader that understands
  `objects.pack.v1`, an interrupted packed export restarts, and the OS handler
  is Ubuntu-only.
- Reconciled the API implementation status (what is live over REST versus what
  is deliberately CLI-only) and extended `SECURITY_REVIEW.md` to cover
  publication handoffs.
- Help reseed verified: 15 pages seed, a second run keeps all 15 and updates
  none, including the new stable-links and publishing guides.
- Archived the completed v0.4 plan and drafted the v0.5 plan from `ROADMAP.md`.
  No v0.5 implementation has started.
- Full release validation passed, including `package_release.sh` and
  `check_release_zip.py`.

## 2026-08-05 P7 — publication profiles and privacy-reviewed handoff

- `notriosctl publish profile save|list|delete`, `publish plan`, `publish run`.
  Profiles record selection and privacy decisions only and are stored owner-only
  at `<data-dir>/publish-profiles.json`; `full_archive` is refused as a profile
  target so a backup is not reachable through a publishing name.
- Publishing is gated on the digest of a reviewed plan. `publish run` re-plans
  and refuses when the library no longer matches what was reviewed, because
  adding a note to the published notebook or removing a `private` tag changes
  what would go out.
- The projection publishes current revisions only and withholds Trash,
  provenance, exact source bundles, saved searches, and revision metadata.
- Content rewriting moved from "never allowed" to "not allowed for
  `full_archive`": a backup must reproduce canonical bytes, a projection may
  not carry a reference a reader cannot follow.
- Rewriting the body was only half the job. A link record carries the raw
  target, the resolved ID, and a context excerpt of the surrounding sentence, so
  records for rewritten links are dropped and `context` is cleared for every
  published link; retained links keep offsets shifted onto the published body.
- A span whose bytes no longer look like the link it describes is left alone and
  warned about rather than cut at a stale offset.
- The evidence run found that archive-v1 import never rebuilt links, so an
  imported library's internal links stayed unresolved until each note was
  edited — and a publication would have flattened all of them. `Import` now
  rebuilds links after every note exists.
- Evidence: `performance/v0.4-p7/publication-handoff-check.md`. No real-corpus
  publication run: no available corpus has the public/private boundary a
  profile exists to separate.

## 2026-08-05 P6 deferred to v0.7

- The movenotes-v3 compatibility bridge moved out of v0.4. In the replacement
  2026-08-10 plan it is G19, gated on G9 (the native change-envelope/container
  codec), not the whole milestone. The original draft called that gate slice 3.
- G9 reuses archive-v2 manifests and objects for snapshots and change
  envelopes, and G2's encoding/compression and resource-chunk investigations
  change container internals. Unknown record types are
  rejected, so sync-era additions arrive behind a new required capability and a
  reader pinned in v0.4 would refuse every archive written after v0.7 — a safe,
  loud failure, but a second integration pass of exactly the kind P3a and P3b
  were sequenced to avoid.
- `movenotes-v3` contains no `notrios2sql.py` and no reference to Notrios as of
  2026-08-05, so two of P6's four bullets are coordination against an importer
  nobody has started.
- P7 keeps its scope but no longer claims an external consumer: it emits a
  checksum-verified, explicitly subset-scoped archive-v2 handoff that Notrios'
  own verifier admits.
- Recorded as accepted decision 16 in `PROJECT_DECISIONS.md`; `ROADMAP.md` now
  lists the bridge as v0.7 G19 after G9.

## 2026-08-05 P5 — stable external links and local resolution

- `notrios://databases/{database_id}/documents/{document_id}[#anchor]` is
  parsed by `internal/stablelink`, hand-written and strict rather than built on
  `net/url`, with typed rejections so a foreign scheme is distinguishable from
  a broken Notrios link.
- `internal/profiles` is the explicit local registry mapping a logical database
  ID to a database path. It never scans the filesystem and never infers a
  database from a path. Clones of one database are reported with every
  candidate rather than picked; `--profile` settles that ambiguity but cannot
  redirect a link into a database that profile does not hold.
- A link naming a foreign database is never matched against local IDs, and the
  response omits title/URI/notebook, so it cannot probe what this database
  holds. Document IDs are unique per database, not globally.
- Store resolution returns `resolved`, `trashed`, `stale_target`, or
  `foreign_database`; `notrios://` links inside note bodies join the link graph
  as `resolved`/`external`/`unresolved`/`invalid`.
- Surfaces: `POST /api/v1/links/resolve`, `database_info.database_id` in
  `/status`, `notriosctl link|open|profile|register-url-handler`, preview
  routing, and the `#document=<id>` deep link.
- `open` exit codes are the contract (0 opened, 1 unresolvable, 2 malformed)
  because a protocol handler runs it without a terminal.
  `register-url-handler` prints unless `--apply` and claims
  `x-scheme-handler/notrios` only.
- Two corrections during the slice: the desktop entry needed an embedded
  `--config`, since a desktop launch otherwise resolves the service URL from
  built-in defaults and would open the note in whatever runs on port 8080; and
  the memoized database ID is cleared by `AdoptDatabaseIdentity`, which changes
  the universe every stable link in the database names.
- Evidence: `performance/v0.4-p5/stable-link-routing-check.md`. Browser-level
  verification of the deep link was not run — no Python `playwright` module is
  installed here.

## 2026-08-05 document reconciliation

- Compared every living document with the source tree before starting P5 and
  corrected the P3a/P3b/P4 status claims, the archive-v2 layout contradiction,
  the missing schema-v13 documentation, and the CLI surface listings in both
  `API_SPEC.md` and the user docs. Detail is in `PLAN.md`.

## 2026-07-26 plan/roadmap review

The review initiated by `/home/renes/prompts/notrios_codex_reviewplan.md`
updated the planning model without implementing future product features:

- confirmed the current `q1` cursor is offset-backed, SQL uses OFFSET, and
  navigation has a 100,000-offset ceiling; H7 now fixes this before importer
  scale hardening;
- added `SYNCHRONIZATION.md` for v0.7 identities, operation IDs/HLC/ack vectors,
  merge rules, conflicts, immutable archive/envelope objects, REST and
  folder/rclone transports, GC/retention, and convergence validation;
- made native archive v2 (v0.4) the full-snapshot/container layer reused by
  sync, while documenting archive v1 as non-lossless interchange;
- added scalable publication planning and a Bluge/Recoll/FTS adapter spike;
- kept Marmot, Cachapa, Ygo, Nostr, and bitchat as bounded design references,
  not dependencies; REST plus immutable rclone/folder transport comes first;
- documented Wails v3/mobile as a later migration spike. As of the replacement
  2026-08-10 plan, Wails v3 desktop is beta and mobile remains experimental;
- reconciled stale API/security/feature/docs claims after H4.

See the archived review under `plans/v0.3/000-plan-roadmap-review-2026-07.md`
when this session completes.

## Completed milestones

- v0.1 MVP is archived under `plans/mvp/`.
- v0.2 redesign R1–R16 is complete under `plans/v0.2/`.
- v0.3 H1–H11 are under `plans/v0.3/001`–`011`.
- GUI conformance/native-resize verification and the scaffold/MVP report
  archive moves are committed before the current review.

The authoritative capability summary is `CODING_CLIENT_HANDOFF.md`; historical
attempt detail remains append-only in `agent/ATTEMPT_LOG.jsonl`.

## Current working-state facts

- Branch: `develop`.
- Review base before this planning session: `aaff582`.
- Project license: Apache-2.0.
- Canonical store: SQLite plus content-addressed assets; FTS5/Recoll are derived.
- Current schema: v18 (`store.CurrentSchemaVersion`). The two baseline migration
  copies create through v17; `ensureSchemaV18` adds `jobs`, and the additive
  `ensureSchemaVn` shims upgrade older databases.
- Unbounded local traversal uses `(updated_at, id)` or `(score, id)` keysets;
  notebook and Trash pages are route-bound. Optional Recoll merge pages use a
  ten-minute, 1,000-hit immutable snapshot and report truncation explicitly.
- H7 generated profile evidence is under `performance/v0.3-h7/`.
- H8 generated Joplin dry-run/interruption/resume evidence is under
  `performance/v0.3-h8/`.
- H9 generated Obsidian dry-run/interruption/resume and scale evidence is under
  `performance/v0.3-h9/`.
- H10 native Recoll 100k index/query/drift/reconciliation evidence is under
  `performance/v0.3-h10/`.
- J2 private-safe real-export planner evidence is under
  `performance/v0.4-j2/`.
- J3 synthetic and private-safe full transactional evidence is under
  `performance/v0.4-j3/`.
- Q1 generated boolean/category scale evidence is under
  `performance/v0.4-q1/`; ordinary first/next/deep pages remain below the
  recorded 100 ms target, while the nonselective negation cost is reported
  separately.
- P1 generated 100k selection evidence is under `performance/v0.4-p1/`: the
  full plan covered 100,000 documents, 1,000 resources, and 200,000 links in
  16.239 seconds; whole-process peak RSS was 136 MiB.
- P2 synthetic golden/adversarial evidence lives with the verifier under
  `internal/archivev2/testdata/`; no content-bearing private corpus is needed
  for the format-contract slice.
- H11 local release gates pass; `docs/operations.md` is included in the
  docs site; P1/P2 expand it from 11 to 13 pages/Help notes with selection and
  archive-v2 safety guides.
- Product version: 0.6.0; current schema: v18.
- Resource reference report:
  `GET /api/v1/resources/reports/reference` and
  `notriosctl resources report`.
- Resource GC: `GET /api/v1/admin/gc/report` is read-only;
  `notriosctl gc` defaults to dry run and requires `--apply` to remove
  retention-expired, currently unreferenced resources. The local retention
  gate is replaceable with future peer-acknowledgement policy.
- Perceptual hashing: hook contract is wired, but no algorithm ships or is
  installed by default; all output is review-only.
- Fresh database builtins: All notes, Notes, Reports, Help, Trash.
- MCP default tool scope is `read-only`; `editor` and `organizer` writes are
  implemented. `mcp.default_profile` is only a deprecated configuration alias.
- Native archive v1 is query-scoped interchange, not full backup.
- Native archive v2 export, verify, and restore are live as `notriosctl export
  archive-v2`, `verify archive-v2`, and `restore archive-v2 --intent
  replace|adopt|merge|fork`. They are local CLI operations with no REST/MCP
  output-path surface. Restore verifies an archive completely before its first
  canonical write, and records a `restore_state` marker (schema v13) so a
  restore interrupted part-way cannot be mistaken for a complete library.
- Archive-v2 objects live at `objects/sha256/ab/cd/<hash>`; the object
  inventory lives in index chunks, not in the manifest.
- P3 generated 100/1,000/5,000-note export evidence is under
  `performance/v0.4-p3/`; P3a container evidence, including the real
  382,206-note Joplin corpus, is under `performance/v0.4-p3a/`; P3b's loose
  versus packed A/B is under `performance/v0.4-p3b/`; and the attachment-bearing
  round trip, which is the only corpus carrying resources and exact source
  bundles, is under `performance/v0.4-p4/`.
- Both object layouts restore to byte-identical libraries. Packing collapses
  215,484 files to 30 and is faster to export, but costs more memory to verify
  and restore because a pack trailer is read whole.
- An archive now admits up to 8,000,000 objects. The manifest carries index
  chunk descriptors and totals only, so its size does not track library size.
- External links: `notrios://databases/{id}/documents/{id}`. The local registry
  lives at `~/.config/notrios/profiles.json` (override with `--registry` or
  `NOTRIOS_PROFILE_REGISTRY`) and is written `0600`.
- P5 live routing evidence is under `performance/v0.4-p5/`; P7 publication
  evidence is under `performance/v0.4-p7/`.
- Publication profiles live at `<data-dir>/publish-profiles.json` (override with
  `--profiles` or `NOTRIOS_PUBLISH_PROFILES`), written `0600`.
- No GitHub push is authorized for this review.

## Open implementation blockers

- Long-term SQLite driver choice (current local cgo/libsqlite3 adapter).
- Official MCP Go SDK adoption/version.
- Sync decisions live in the owning G0-G17 items in `PLAN.md`; the register in
  that plan is an index. G0 is complete; G1 and every later slice remain
  approval-gated.
