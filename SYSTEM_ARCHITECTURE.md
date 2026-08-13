# System Architecture

## Design goal

Build a local-first companion service for very large note and document collections, supporting hundreds of thousands of notes plus embedded images, PDFs, archives, and imported conversations.

## Core boundary

```text
Built-in Wails GUI / Web UI / CLI / MCP client / third-party clients (REST/MCP today; C ABI planned)
        │
        ▼
Notrios service (notriosd)
├── REST API
├── MCP adapter
├── Document service
├── Notebook/tag service
├── Resource service
├── Search service (query-language adapter)
├── Link graph service
├── Import service
├── Archive/backup service
├── Publish service
├── Media policy service
├── Sync service (planned; disabled by default)
└── Recoll adapter (optional sidecar)
        │
        ├── SQLite + FTS5 canonical store
        ├── content-addressed asset store
        ├── managed filesystem projection
        └── derived Recoll/Xapian indexes
```

## Canonical storage

SQLite is the canonical application database. It stores:

- collections;
- notebooks (nested, emoji icons), tags, and search-notebook queries;
- documents;
- current document state;
- saved revisions;
- resources and blobs;
- document-resource references;
- document links and backlinks;
- source provenance and conversation threads (author, author ID, thread ID, reply-to, source URL);
- block anchors;
- media policy decisions;
- indexing outbox state.

Future sync operation/acknowledgement tables remain canonical local state, but
transport inbox/outbox files and remote stores do not. See
`SYNCHRONIZATION.md`.

SQLite FTS5 provides immediate search over managed documents. Recoll is an optional derived sidecar for front-matter field search, expensive extraction over arbitrary files, and broad filesystem search (see `RECOLL_INTEGRATION.md`); the service degrades gracefully to FTS5 when Recoll is absent.

## Document identity

Documents use stable application IDs independent of titles, filenames, folders, or source-system IDs. External source IDs are preserved in metadata:

```text
document://<collection>/documents/<document-id>
resource://<collection>/resources/<resource-id>
```

Exports can rewrite these into relative Markdown paths.

The desktop-facing stable link is planned as:

```text
notrios://databases/<database-id>/documents/<document-id>
```

The logical database identity is portable across replicas; a profile ID is
local configuration and therefore must not be required in a shared link. The
OS handler validates the URI and finds profiles mapped to that database ID. It
opens the unique match, prompts if several local profiles point at clones of
the same database, and reports missing/stale targets without silently choosing
another database.

v0.4 P5 implements this. `internal/stablelink` parses the URI strictly — the
value arrives from outside the application, so the only safe reading is one
that matches the documented shape exactly or fails. `internal/profiles` is the
explicit local registry mapping a logical database ID to a database path; it
never scans the filesystem, never infers a database from a path, and reports
every candidate rather than choosing when several profiles hold clones of one
database. Naming a profile settles ambiguity but cannot redirect a link into a
different database. A link naming a foreign database is never matched against
local IDs, because document IDs are unique per database rather than globally.
Resolution is local routing only: it opens a note in a database this machine
already has and never contacts a peer.

## Resource model

Resources are logical attachments or embedded media. Blobs are exact bytes addressed by hash. Multiple resources may share the same blob. Multiple documents may reference the same resource.

Remote resources must pass through media policy, quarantine, hash checks, and provenance recording before admission.

The resource service exposes read-only reference reports for exact duplicate
logical resources, unreferenced physical blobs, and direct per-notebook usage.
An optional perceptual-hash hook may compute additional blob hashes and suggest
review candidates, but it cannot change exact-blob identity or mutate content.
No perceptual algorithm ships in the core service.

Retention state lives on logical resources and begins when their final
document reference disappears. The garbage collector separates planning from
apply, rechecks references transactionally, and deletes a physical blob only
after its last logical resource is removed. A `RetentionGate` abstraction uses
local time eligibility in v0.3 and is the insertion point for v0.7 peer
acknowledgement watermarks.

## Import model

Importers should be separate commands but shared code. They should write
through bounded canonical transactions, not directly into any search index.
Joplin RAW Export Directory is the preferred Joplin bulk-import format. Its
importer restores nested notebooks and source tags, performs input-scoped
batch lookups, and persists fingerprints/checkpoints. Large RAW inventories
use a temporary indexed SQLite manifest, bounded keyset pages, and a lean
routing parser instead of retaining note bodies and note-tag joins on the Go
heap. Canonical note batches commit revisions, FTS5, links, provenance, tags,
resources, outbox, item state, and the matching checkpoint atomically; a final
bounded link pass resolves targets created in later batches. With
`--preserve-source`, exact item bytes plus property order are stored in a
separate content-addressed source-bundle namespace; they are not canonical
notes or ordinary resource blobs.
Canonical RAW parsing splits only on CR/LF physical endings (OCR control
characters remain property data), derives titles from the first source line,
and uses one ordered-property parse for both effective fields and source-bundle
property order. J2/J3 real-export and complete transactional profiling gates
are satisfied by aggregate-only evidence under `performance/v0.4-j2/` and
`performance/v0.4-j3/`.
Obsidian vaults, Twitter/X archives, ChatGPT exports, and Claude exports are
normalized into notebooks/collections with source provenance rows (including
thread recovery for Twitter/X and conversation exports). The v0.3 scale design
uses one inventory, deterministic IDs, indexed fingerprints, bounded batches,
and resumable job checkpoints; native archive v2 and sync later reuse the same
immutable object/manifest layer.

## Built-in GUI

The built-in GUI is a Go/Wails v2 application (`notrios`) embedding the
service; the React frontend hosts inside the Wails window. Modes: default (GUI
+ local service), `-no-gui` (headless service, for users running a different
client), `-gui-only` (pure REST client, usable against a remote service and for
testing the API the way a third-party client would). Layout, themes, and
notebook sidebar behavior are specified in `UI_DESIGN.md`.

Wails v3 now documents one desktop/iOS/Android codebase. v3 is beta for desktop
and mobile support remains experimental. Migration is not a dependency for
sync design. A later spike must cover desktop parity, Android storage/lifecycle,
background transfer, safe-area/responsive UI, mobile file-dialog limitations,
and post-1.0 physical-device resource use before changing the stable v2 shell.

### Planned framework-neutral facade and native ABI

Before 1.0, v0.8 extracts transport-neutral application orchestration from the
HTTP adapter and exposes it through both the existing REST server and a small
versioned C ABI. The ABI is not a second Store API and does not expose Go
pointers: it owns instance lifecycle, opaque handles, bounded serialized
request/response calls, typed errors, cancellation/polling, bulk streams, and
explicit result-buffer ownership. See `FLUTTER_GO_CLIENT.md`.

The post-1.0 Flutter client uses Dart FFI on Android, iOS, Linux, macOS, and
Windows. Flutter Web is not a native-FFI target; it stays a REST client unless a
separate Go-Wasm/JavaScript adapter is approved. Wails mobile remains an option,
not the sole mobile architecture.

The GUI owns link interception, resource upload/download, preview sanitization, and routing to document/resource URIs.

v0.5 E5 added the two read-only service surfaces an editor needs while someone
types — bounded link-target suggestion and unsaved-buffer link resolution. The
buffer check parses the submitted body with the canonical extractor rather than
trusting a client-extracted target list, so a marker the editor draws matches
the link record a save will write; that keeps Markdown knowledge in one place.

**v0.5 E6 settled the editor question: Notrios stays on `md-editor-rt`, because
it *is* CodeMirror 6 and exposes it** (`PROJECT_DECISIONS.md` 20). It depends on
`@codemirror/{view,state,autocomplete,commands,language,search}` and surfaces
them through the `completions` prop, `config({codeMirrorExtensions})`,
`getEditorView()`, and `domEventHandlers`. The "migrate to CodeMirror for caret
position and inline widgets" framing had no content, and an earlier claim that
this editor could give neither was wrong — it reached several documents before
E6 corrected it. Broken links are underlined where they sit, the underlines
follow their text through edits, Ctrl-click opens a target, and completions
appear inline; the whole-note list is kept *alongside* the underlines because an
underline only helps where a reader is already looking.

The editor pane converts UTF-8 byte offsets from the service into CodeMirror's
UTF-16 indices (`web/src/editor-offsets.ts`). They agree on ASCII and diverge at
the first accent, so an unconverted offset marks the wrong text and drifts
further into the note; an offset landing inside a character is dropped rather
than rounded.

The frontend is offline-first as of E6a: KaTeX, highlight.js, and cropper are
bundled as local instances, echarts and prettier are disabled, and the service
serves a Content-Security-Policy with the UI. Nothing is fetched from a CDN at
runtime.

## REST and MCP

REST and MCP are adapters over the same service layer, and together must be
complete enough that a full-featured third-party note client can be built on
them alone. MCP must not expose raw SQL, arbitrary filesystem operations, or
unrestricted writes. MCP tools should return snippets and resource links first,
requiring explicit document/resource retrieval for larger content. For archive,
bulk, and sync jobs, MCP is a bounded control plane returning job IDs/status;
REST or immutable objects are the bulk-byte data plane.

**v0.6 made both halves concrete, and the second one narrower than planned.**
Tool visibility is four cumulative scopes — `search-only`, `read-only`,
`editor`, `organizer` — enforced at the call site rather than only in the tool
listing, since a hidden tool that answers when invoked directly is not hidden
(F2). Every registered tool carries a scope, checked by a test that walks the
whole surface, so a new tool cannot arrive unclassified.

The job control plane (F6) watches and stops; it does not start. Every job kind
names a filesystem path, and "MCP must not expose arbitrary filesystem
operations" does not stop applying because the path is wrapped in a job record.
Starting a job is a CLI act, which is the same line archive export, verify, and
restore already draw.

Three disclosure rules follow from what a job record holds: its parameters never
leave the machine, the MCP view omits the free-text failure message because a
filesystem error reads like a path, and a summary crosses because it is counts
by construction.

## Recoll integration

The service owns canonical notes and resources. It writes a managed filesystem projection and queues changes through a durable indexing outbox. Recoll (user-installed, optional, external process — GPL licensing boundary in `RECOLL_INTEGRATION.md`) indexes the projection through a generated config and an enhanced from-scratch front-matter handler, providing field search, derived metadata, and arbitrary-file search. The query-language adapter (`SEARCH_QUERY_LANGUAGE.md`) translates one bounded expression tree to FTS5/exact SQL and Recoll. It owns uppercase OR, implicit AND, prefix negation, grouping, phrases, typed fields, and the category/notebook alias; no backend receives a weakened expression.

Unbounded local lists page directly from SQLite with query-bound keysets:
`(updated_at, id)` for chronological orders and `(score, id)` for reproducible
FTS5 relevance. Optional Recoll merging is a distinct bounded contract: the
service freezes at most 1,000 merged hits in an immutable, expiring in-memory
snapshot. Cursors never expose SQLite offsets or sidecar row IDs.

## Publishing

Publishing is not the same as backup. Notrios publication profiles select a
public subset of notes/resources, sanitize metadata, rewrite private links
safely, and emit a subset-scoped native-archive-v2 handoff. A separately
maintained `movenotes-v3/notrios2sql.py` importer is planned to consume the
handoff — that bridge is deferred to v0.7, gated on the sync snapshot/change
container slice that still extends archive v2;
`movenotes-v3` owns Obsidian/Quartz and Hugo/Ledger generation, with Pagefind or
Bluge according to site scale. v0.4 shares the neutral
selection/link/resource/privacy plan between full archive, subset transfer, and
publication handoff rather than duplicating those publishing engines.
P7 implements the publication itself: saved profiles record selection and
privacy decisions only, `publish plan` reviews, and `publish run` refuses unless
the library still matches the reviewed digest. The projection publishes current
revisions only and rewrites links to withheld or unresolved targets, dropping
their link records so a published archive cannot name what it withheld. Notrios
never executes note content and runs no build step.

P1 implements that neutral layer as a read-only canonical SQLite planner with
typed notebook/tag/query/explicit-ID selectors, target-specific privacy
defaults, reachable-resource and link-boundary analysis, capped content-free
details, and a digest over the complete manifest. REST and MCP call the same
Store method; neither accepts SQL, paths, or output commands. See
`SELECTION_AND_PRIVACY_PLANNER.md`.

P2 adds schema-v12 logical `database_id` and per-writable-copy `replica_id`,
plus the separate `internal/archivev2` format/verifier package. Archive objects
are immutable SHA-256 files; strict typed JSONL records and a final manifest
bind P1 selection, snapshot identity, schema/capabilities, MIME, sizes, counts,
and references. Verification has no Store write dependency. P3/P3a/P3b stream
exports at real-library scale — the object inventory lives in checksummed index
chunks under an `ab/cd` fanout, and an optional packed layout collapses file
count for the future sync transports. P4 restores only after complete
verification and explicit replace/merge/fork/adopt identity planning, and marks
an interrupted restore so a partial library cannot pass as complete. See
`NATIVE_ARCHIVE_V2.md`.

G2's investigation candidate keeps the archive-v2 JSONL snapshot contract
unchanged but recommends compact canonical NCB1 records inside incremental
change envelopes, with a canonical-JSON outer manifest and bounded
deterministic gzip. The production gate is G9. Envelopes close at 10,000
operations, 16 MiB canonical bytes, or 4 MiB compressed bytes; resources stay
whole below 1 MiB and use 1 MiB fixed chunks above it. These are desktop-proxy
bounds pending the v0.8 emulator and post-1.0 physical-device gates. See
`performance/v0.7-g2/`.

G4 adds schema-v19 local replication durability. Before explicit enrollment,
the journal is inert. Enrollment records a full-snapshot boundary at sequence
zero for the current replica. SQLite triggers on sync-relevant canonical tables
write into one transient capture seam; that seam allocates the next local
sequence, inserts an immutable operation, and advances the local contiguous
vector in the same transaction as the canonical row. Rollback therefore leaves
neither side behind. FTS, parsed links/blocks, projections, reports, jobs, and
import checkpoints have no capture triggers. Identity rotation retires the old
allocator and requires a new boundary. The schema reserves dependency, gap,
acknowledgement, pending-admission, and audit tables. At G4 completion, later
slices still owned their protocol behavior; G4 itself added no transport,
merge, encryption, REST, MCP, or UI.

G5 advances schema v20 and implements that reserved protocol
behavior without adding a carrier. `internal/syncstate` is a SQLite/HTTP/UI-free
core for protocol-1.0 compatibility, bounded vector comparison, deterministic
missing ranges, and strict operation normalization. The store persists only
explicitly configured compatible fixture peers, stages out-of-order or
dependency-blocked operations, and transactionally admits every now-contiguous
operation while updating gaps and the acknowledgement vector. An exact replay
is inert; a conflicting replay, unknown record, compatibility mismatch, sparse
sequence beyond the window, or quota overflow rolls back.

G6 advances the current schema to v21. Operations carry bounded durable HLCs;
scalar updates are sparse per-field LWW registers, document-tag rows are LWW
elements, and lifecycle/death-certificate state is distinct from absence. A
transport-neutral fold rebuilds the post-boundary metadata projection and
applies it atomically with admission. Notebook cycles, missing parents, missing
document homes, and uniqueness collisions produce deterministic visible repair
rows.

G7 advances the current schema to v22 and converges note bodies. A revision is
an immutable object naming its parents, its exact content hash, and its length;
it travels with its complete body inline or as a bounded named-base VCDIFF
delta, and nothing unverified reaches canonical storage. Concurrent edits to
different regions become a merge revision with a content- and parent-derived
identity; overlapping edits become a durable typed conflict on the same
document. `internal/syncbody` and `internal/syncdelta` are transport- and
storage-neutral, like the G5 state core and the G6 metadata core.

G8 advances the schema to v23 and converges attachments. A blob row may exist
without a file, so a note can reference an attachment this replica has not
downloaded; a structural trigger refuses any row whose availability contradicts
whether it has bytes. `internal/syncassets` owns the chunk plan, manifests, and
materialization policy, and the store fetches through an `ObjectProvider` that
G11 and G14 implement. Nothing is installed until the manifest digest, each
chunk hash, the whole-object hash, the length, and the sniffed content type
agree.

G9 adds `internal/syncwire`: the canonical NCB1 operation and NEV1 envelope
encoding, deterministic gzip, and the NAR1 artifact that seals a plaintext with
AES-256-GCM under a per-artifact HKDF-derived key and signs it with Ed25519.
Encrypt-then-sign lets a receiver reject a forgery without decrypting; the
canonical header is both the key-derivation salt and the associated data, so it
cannot be edited in transit. It adds no schema and no dependency. G13 still owns
authenticated enrollment and authorization, and v0.8 owns the secret store the
key interfaces stand in for.

## Optional derived systems

- go-git or Fossil can checkpoint projections but should not replace SQLite revisions.
- LadybugDB can be added later as a derived graph backend if advanced traversal/analytics are needed.
- Bluge remains an external `movenotes-v3`/Ledger publication dependency, not a
  Notrios search dependency. Notrios keeps FTS5/Recoll for application search
  and verifies only the archive/privacy integration boundary.
- Yjs-compatible Go CRDTs can be evaluated for optional simultaneous note
  editing; they do not replace the record-level database protocol in
  `SYNCHRONIZATION.md`.

## Cross-cutting design references

- Use `FEATURE_MATRIX.md` to decide whether a feature is MVP, soon, later, or optional.
- Use `UI_DESIGN.md` for built-in UI/editor behavior.
- Use `SELECTION_AND_PRIVACY_PLANNER.md` and `PUBLISHING_POLICY.md` for shared
  archive/subset/publication selection and public handoff policy.
- Use `VERSIONING_AND_SYNC_POLICY.md` for SQLite revisions, go-git, Fossil, and external-vault sync.
- Use `SYNCHRONIZATION.md` for database/replica identity, merge algorithms,
  transport, retention, backup/restore relationships, and validation.
- Use `WORKSPACE_MAINTENANCE.md` for Foam-style query blocks, lint/fix, outlines, and block anchors.
