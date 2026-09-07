# Security Review — Current Local Product and Planned Remote Surfaces

This review reflects the v0.7.0 release candidate (schema v27). Notrios is
local-first but
its REST/MCP listener, importers, preview, downloaded media, archive files,
published handoffs, and future sync transports are security boundaries.

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

## Current controls

### Resources and remote media

- Stored blob paths are SHA-256 content addresses, not user filenames.
- Responses use `X-Content-Type-Options: nosniff` and sanitized
  `Content-Disposition`.
- Referenced logical resources cannot be deleted directly; exact duplicate
  bytes share a blob.
- Remote URLs are statically scanned before fetch. Every redirect is
  re-evaluated; unsafe schemes/domains and connect-time private/link-local
  addresses are blocked, environment proxies are disabled, size is bounded
  while streaming, MIME is sniffed, SHA-256 is computed, and bytes remain in
  quarantine until admission.
- Localization checks exact-hash policy, creates resource/provenance rows, and
  rewrites Markdown in a new revision under optimistic concurrency.
- Resource reference reports are read-only. Exact SHA-256 remains the only
  deduplication identity; the optional perceptual hook ships disabled, accepts
  no filesystem path from callers, and can only emit validated review
  suggestions. Perceptual `block` rules are rejected.
- Resource GC starts its clock when the last reference disappears, distinguishes
  purged-note recovery windows, defaults to dry-run, and rechecks reference
  state transactionally on explicit CLI apply. The REST GC endpoint cannot
  apply. Immediate resource deletion and permanent note purge require
  object-specific confirmation headers.

Ordinary JSON/MCP bodies are bounded at 8 MiB and must contain exactly one
value; raw resources use the canonical 16 GiB object ceiling at HTTP and store
boundaries. Remaining: optional malware-scanner integration.
Perceptual similarity may suggest review but must never silently identify or
deduplicate content. v0.7 must replace the local retention gate with
peer-acknowledgement-aware eligibility.

### Markdown and WebView

- Preview removes active/unsafe elements, event attributes, and inline styles.
- Internal document/resource links route through application IDs rather than
  storage paths.
- Remote preview content does not cause localization or become a policy oracle.

Remaining: maintain a pinned reviewed sanitizer and browser-native regression
corpus.

**Third-party asset loading (fixed in v0.5 E6a).** The built-in UI used to fetch
KaTeX, highlight.js, echarts, cropperjs, and prettier from `unpkg.com` at
runtime — thirteen requests and 623 kB on every launch, in the desktop app too.
A local-first application that quarantines remote *images* behind a media policy
was loading remote *executable JavaScript* unchecked, and a CDN compromise would
have had script execution in the note editor. Those libraries are bundled or
disabled now, and `handleWebApp` serves a Content-Security-Policy
(`script-src 'self'`, `object-src 'none'`, `connect-src 'self'`,
`form-action 'none'`). `img-src` still admits remote images because the preview
is permitted to display them; localizing one remains a server operation under
the media policy. `scripts/run_offline_assets_check.sh` fails if any remote
asset returns.

The `notrios://` OS handler is implemented (v0.4 P5) under those constraints.
The parser is hand-written and strict rather than delegated to a permissive URL
library: scheme, authority, route, identifier charset, every length bound, and
the absence of query strings and percent-escapes are checked, and traversal,
embedded newlines, and control characters are rejected. Resolution consults
only databases the user registered explicitly; it never scans the filesystem,
never infers a database from a path, and refuses rather than choosing when
several profiles hold clones of one database. A link naming a foreign database
is never matched against local IDs. The registry is written owner-only because
it records local paths, and a corrupt registry is refused rather than partially
applied. The generated desktop entry claims `x-scheme-handler/notrios` only,
and installation requires an explicit `--apply`.

### REST and MCP

- Default binding is loopback.
- Ordinary REST, MCP, and web routes require a loopback connection and a local
  Host. A non-loopback listener admits only the finite authenticated peer-sync
  route set; a forwarding header cannot make a remote request local and an
  unknown sync-prefixed path cannot fall through to the web application.
- Browser mutations require an exact same-origin request (or the Wails origin);
  cross-site Fetch Metadata, malformed/null origins, and DNS-rebinding Hosts
  are refused. Origin checks supplement the loopback boundary and are not user
  authentication.
- Raw SQL, arbitrary filesystem access, and direct Recoll mutation are absent.
- MCP output is bounded and marks note content untrusted.
- Editor writes are scope-gated and destructive edits use revision
  preconditions.

**Embedded query blocks (v0.5 E7)** widen what note content can *ask for*, and
the containment is that they widen nothing. A fenced ```` ```note-query ````
block is parsed server-side by the same query parser the search box uses, so it
can express nothing its author could not already type there — no SQL, no
scripting, no filesystem reach, no output target. Blocks carrying `sql:`,
`file:`, or `exec:` keys are refused by name, and a SQL-injection-shaped
*query* is simply text to the parser. Results are capped at 100 rows with
visible truncation, and every rendered value is written as text or an element
attribute: a fixture gives a note the title `<img src=x onerror=…>` and asserts
it renders as text and sets nothing on `window`. A publication carries the
block's source, never a materialized result, so a published note cannot leak
what a query matched at export time.

**Organizer operations (v0.5 E8)** keep the existing asymmetry between
reversible and irreversible. Moving a note to the Trash is revision-
preconditioned and reversible; permanently purging one still requires the
object-specific `X-Notrios-Confirmation: purge-document:{id}` header and is
refused outright for externally-sourced notes. Tag rename is a dry run by
default on both REST and CLI, and the dry run is a rolled-back transaction
rather than a separate predictor, so a report cannot understate what an apply
does. A trashed note became readable through `GET /api/v1/documents/{id}` —
deliberately, and only there: it is still absent from search, from link
listings, and from MCP entirely, so `is:trashed` remains a scope a caller asks
for rather than one it falls into.

**MCP tool scopes are a guardrail, not authorization (v0.6 F2, implemented).** Notrios is
single-user: administrator and author are the same person, so there is no second
principal to authorize against. A scope is the user narrowing what their own
agent may do — a seatbelt, not a lock. It must not be cited as an access-control
boundary, and it does not make the endpoint safe to expose: the endpoint has no
authentication, and the scope is chosen by the same configuration file the
operator controls.

Four scopes exist — `search-only`, `read-only` (default), `editor`, `organizer`
— and they are **enforced at the call site**, not only by filtering
`tools/list`. Before F2 only write tools had a call-site check, so a read tool
hidden from a narrower tier answered perfectly well when invoked directly. Both
the listing and the check now read one table, and a test walks every registered
tool against every scope so a new tool cannot ship unclassified. The deprecated
`mcp.default_profile` key is still honoured; when it and `mcp.default_scope`
disagree the **narrower** wins, because a key that quietly stops applying must
never widen what an agent may do. Whole-library destructive operations
(garbage collection, purge, archive restore, publication) are deliberately
unreachable over MCP at all, so the guardrail is not the only thing standing
between model output and them.

None of these surfaces is exposed over MCP. Lint, fix, graph traversal, block
listing, query-block evaluation, tag rename, notebook deletion, garbage
collection, archive operations, and publication are REST/CLI only. That keeps
whole-library reads and organizer writes on surfaces a person drives rather than
ones untrusted model output reaches.

Do not expose ordinary Notrios REST, MCP, or GUI routes on a LAN/public address.
G20 enforces that rule even when the configured listener is non-loopback. Only
the authenticated peer-sync route set is remotely admitted, under the TLS,
rate, request-size, and audit controls described below. General remote access
would require a separately designed authentication and proxy boundary.

The job control plane (v0.6 F6) is the first of those bounded surfaces, and it
came out narrower than the plan bullet that asked for it. **A job can be watched
and stopped over REST, and only watched over MCP; it cannot be started from
either.** Every kind this build runs names a filesystem path, and putting a job
record around an operation does not change what the operation does — so a
`start` route would have reopened exactly the boundary the archive commands draw.

Three disclosure decisions follow from what a job record contains:

- **Parameters never leave the machine.** They name a vault directory or an
  export destination. `notriosctl jobs show` renders them; REST and MCP return
  none. They are stored as typed values rather than as raw argv, which also
  keeps any secret that happened to be on a command line out of the database.
- **The MCP view omits the failure message**, because a failure from a
  filesystem operation reads like `open /home/someone/private/x: permission
  denied`. The state is reported, with a pointer to the local command that has
  the reason. REST keeps the message: it is the surface a person drives.
- **A summary crosses because it is counts by construction** — documents,
  objects, bytes — never note text and never a path.

Cancellation is cooperative and can destroy nothing: it sets a flag, and the
work stops after a batch it has already committed and checkpointed.

### Imports and archives

- Importers use deterministic IDs, canonical store writes, content-addressed
  resources, provenance, dry runs for current sources, and no resurrection of
  trashed external items.
- Native archive import validates conflicts before writes.

Joplin RAW and Obsidian now use bounded batches, durable checkpoints,
fingerprints, dry-run diffs, and optional exact source bundles. Native archive
v1 is not a disaster-recovery backup.

Native archive v2 closes that container gap for backup/transfer: path/symlink
traversal refusal, size/count/depth ceilings, manifest-last completeness,
transitive checksum verification through index chunks, one read-transaction
snapshot boundary, and explicit capability/version compatibility. Restore
verifies completely before its first canonical write, re-hashes every blob,
revision body, and source-bundle item at the point of use rather than trusting
the earlier pass, re-sniffs blob MIME through the ordinary resource admission
path, and records a durable marker so an interrupted restore cannot pass as a
complete library. Export, verification, and restore are CLI-only; no REST or
MCP surface accepts an archive path or streams archive bytes.

### Publication handoffs

- A publication is a projection, not an archive of canonical state: current
  revisions only, no Trash, provenance, exact source bundles, saved searches, or
  revision metadata.
- Links to withheld or unresolved targets are rewritten out of the published
  bodies, and their link records are dropped rather than published: a record
  carries the withheld target's ID, its raw target, and a context excerpt of the
  surrounding sentence.
- A link span whose stored bytes no longer match is left alone and reported,
  never cut at a stale offset.
- Publishing requires the digest of a reviewed plan and re-checks it before
  writing, so a library change after the review stops the publication.
- A profile records selection and privacy decisions only. It cannot name a
  command, and Notrios never executes note content or runs a build step.
- `full_archive` is refused as a publication profile target, so the export that
  carries everything is not reachable through a publishing name.
- Read-only builtin notebooks — Help and Reports — are excluded from a
  publication handoff by the target's default policy (v0.6 F5), and the
  exclusion is named in the dry run rather than applied silently. It is not
  overridable: adding an opt-in later is easy, removing a leak is not. This
  closes two things at once — a generated graph report naming notes the
  publication itself withheld, and nothing having stopped a publication from
  dumping Notrios' own documentation onto someone's site. The default **Notes**
  notebook is emphatically not in this set. A full archive and a subset transfer
  keep both notebooks: the first is a backup and must restore faithfully, and
  the second moves notes between the user's own databases.

## Synchronization threat boundary

G4 implements only the local durability portion of this boundary: explicit
enrollment creates a snapshot floor, canonical writes and monotonic local
operations share one SQLite transaction, rollback removes both, and identity
rotation retires the old allocator. The journal is local plaintext canonical
state. It is not authenticated, encrypted, admitted from a peer, or exposed to
a transport/API; none of the later network/cryptographic controls below should
be inferred from schema v19.

G5 implements the non-cryptographic replay/gap boundary in schema v20. It
requires an explicitly configured same-database peer compatibility tuple,
validates protocol 1.0/schema 19-20/required capabilities, bounds vectors,
ranges, operation bytes, dependencies, pending disk usage, and sparse sequence
skew, and refuses unknown record/kind pairs. Exact normalized replay is inert;
a conflicting operation ID or sequence is rejected. Pending or rejected input
cannot advance a vector or acknowledgement, and an injected pre-commit failure
rolls the whole admission back. This is not authentication: G5 fixture peer
configuration carries no signing/encryption key and has no REST/MCP/transport
surface. G9/G13 must replace that local seam with proof-of-possession enrollment
and signed/encrypted artifacts before an untrusted carrier can call admission.

**G13 implements the REST security foundation, and it is the first authenticated
surface this project has ever had.** What it does *not* do is as important as
what it does:

- **A peer principal is one enrolled replica of one database.** It is proved by
  an Ed25519 signature over the request's method, path, database id, replica id,
  timestamp, nonce, and body hash — never a bearer token, so nothing reusable
  travels and a captured request is spent. Enrolled public keys live in schema
  v25 `sync_peer_keys`, where enrolment and revocation are transactional and
  audited; a revoked key is reported as unknown.
- **It authorizes `/api/v1/sync/...` on that database and nothing else.**
  Ordinary note routes keep their existing local, unauthenticated posture, and a
  test asserts that presenting a peer credential to one changes nothing about
  its answer. Sync authentication is not a login and creates no user concept.
- **Pairing is a short-lived, single-use code**, spent in one transaction, whose
  only job is to carry trust once. The group key travels back sealed under a key
  derived from that code, so no reusable library key exists in displayable text
  in any artifact — which replaces the clear-text development bundle G11 shipped.
- **The transport policy is a refusal, not a warning.** With the surface
  enabled, a non-loopback listener without TLS, a certificate without its key,
  or unreadable TLS material makes `notriosd` exit and name the setting. Only
  loopback plaintext is permitted, and only because it never leaves the machine.
- **The surface is not for browsers.** It emits no CORS header and refuses any
  request carrying `Origin`, `Cookie`, or `Referer`.
- **Refusals are uniform and audited.** One message and one status for every
  failed check; the reason is a closed vocabulary in the local audit log. Failed
  attempts spend a per-address budget, so guessing a key or a code is bounded by
  the limiter rather than by the network.
- **Still out of scope:** multi-user accounts or roles, remote authorization of
  any note route, and a public deployment claim. G14/G14d's data plane carries
  encrypted protocol artifacts and requester-authorized opaque physical
  snapshots only; it never accepts an archive/database path or exposes note
  routes to a peer credential. The private key material remains in the warned
  `0600` development file provider until v0.8 selects a platform store.

G14d physical replacement is also local-only and explicit. Exact schema,
application capability, manifest/database/pack hashes, identity, vector/floors,
and every local object are verified before cutover. A second verified physical
snapshot preserves the current canonical state before the first destructive
rename. An owner-only durable plan blocks ordinary startup throughout cutover,
and only the coordinator's narrow verification open can bypass that marker.
The installed image mints a fresh replica allocator and never treats snapshot
possession as a peer acknowledgement. Completed plans are retained as local
recovery records; emergency snapshots contain a full library and require the
same access protection as the canonical database.

G14e keeps those boundaries at full scale. Only an authenticated peer already
permitted as a snapshot source receives the exceptional two-hour synchronous
creation deadline; ordinary requests keep the short deadline, and a signed
Range response is bounded to one 16 MiB chunk. Directory resume may cache the
exact prefix length validated by the current process, but it still hashes the
completed sealed file against the authenticated digest before publication, and
a restarted process revalidates its durable prefix once. Incremental admission
selects only the metadata/body/asset reconcilers named by admitted record
families; this removes whole-library memory amplification without relaxing
operation validation or the single-transaction convergence boundary.

`SYNCHRONIZATION.md` requires:

- database/profile/replica identity negotiation and no silent universe merge;
- authenticated writers, transport confidentiality policy, replay-safe
  operation IDs, deterministic encoding, and strict size/count limits;
- SHA-256 verification before object admission, immutable names, quarantine,
  atomic publish, and manifest-last completeness;
- bounded pending dependencies/outboxes/retries and protection against disk,
  memory, connection, and decompression exhaustion;
- explicit peer retirement and acknowledgement-gated tombstone/blob GC;
- corrupt/stale/malicious peer audit records and full-resync recovery.

rclone/shared folders are untrusted carriers. `rclone sync` deletion is not
used. Nostr/public relays and BLE couriers are deferred because metadata,
retention, availability, key management, bandwidth, and abuse resistance add a
larger threat surface than REST plus immutable rclone objects.

The 2026-08-11 plan review selected mandatory authenticated encryption plus
per-replica Ed25519 signatures, but neither is a current control until its
approved implementation slice lands. A signature attributes canonical control/envelope bytes
to an enrolled and revocable replica; it does not replace encryption, TLS,
hash verification, or authorization. G13 keeps sync credentials scoped to sync
routes; they authorize no ordinary REST route.

**G0 completed the design threat model on 2026-08-11; it did not implement a
sync control.** The reviewed model and control trace are under
`performance/v0.7-g0/`. It freezes these requirements for later slices:

- discovery never enrolls a peer; enrollment explicitly binds database,
  replica, signing public key, encryption recipient/key epoch, capabilities,
  and active status through a one-use, expiring, proof-of-possession flow;
- signatures, AEAD, and hashes have separate jobs. Signed canonical outer
  artifact bytes bind visible routing and ciphertext commitment; AEAD binds the
  same header as associated data; exact hashes verify immutable object bytes.
  None of them alone supplies replay protection or authorization;
- replay/order safety also requires database/type/version binding, contiguous
  replica sequences, dependencies, state vectors, durable key/retirement
  status, and replay floors retained after payload collection;
- compromise revocation advances the encryption epoch for remaining active
  peers. Revoking only the Ed25519 key would still let a former replica read
  future traffic encrypted under a key it retained. Historical plaintext
  already obtained cannot be revoked;
- an active compromised replica can create valid signed/encrypted destructive
  operations. v0.7 is single-user, multi-device sync, so recovery depends on
  attribution, retained revisions/conflicts/tombstones, acknowledgement-gated
  GC, snapshots, and explicit restore intent rather than an invented
  per-operation human authorization signal;
- carrier-visible metadata is limited to justified opaque routing fields.
  Plaintext content hashes, names/titles, operation kinds, state vectors,
  request ranges, acknowledgement positions, and peer display names stay
  encrypted. Timing, frequency, ciphertext size, account/endpoint identity,
  and some routing linkage remain observable;
- parsers/decryptors/decompressors operate under compressed/expanded byte,
  ratio, count, depth, dependency, disk, concurrency, and time bounds. Cheap
  fixed-header and signature/replay gates precede expensive work where the
  final G9 format permits it;
- plaintext development mode must be explicit, loopback-only,
  non-interoperable with production protocol mode, visibly audited, and never
  selected through negotiation or failure fallback.

Current REST still has no general authentication and remains approved only for
the local/loopback deployment posture below. Planned sync-route authentication,
payload encryption, and TLS do not retroactively secure the present listener.

**G1 completed the revision/delta/merge investigation on 2026-08-11; it did not
implement patch admission or merging.** Later G7 code must name and verify the
delta base, reconstruct under byte/token/operation/insert/CPU bounds, validate
UTF-8 and the complete result hash, and fall back to the complete object on a
missing or refused delta. It must never apply a best-effort or partially
verified patch to canonical state. Same-token, delete/edit, malformed, and
over-limit cases remain durable typed conflicts rather than silent winners.

**G1a completed an investigation prototype; it is not a live binary-delta
decoder.** It demonstrated explicit bounds on
source/window/input/output sizes, instruction and address counts, varints,
integer arithmetic, expansion ratio, chain depth, memory, and CPU/cancellation.
It rejects corrupt/truncated streams, unsupported custom code tables and
secondary compressors, and malicious overlap without partially materializing a
canonical object. Its constrained RFC 3284 recommendation still requires a
named-parent check plus exact outer result-hash verification. External
C/C++/cgo tools are test oracles only. Until later approved G7/G8 implementation
passes the remaining production gates using G2's completed bounds, complete or
fixed-chunk verified objects remain the only planned binary-resource transfer
forms.

**G2 completed the bounds investigation; it did not add a live decoder or
admission path.** The G9 candidate closes an envelope at 10,000 operations,
16 MiB canonical bytes, or 4 MiB compressed bytes and independently limits one
record to 1 MiB, its payload to 512 KiB, dependencies to 64, and decompression
to 64:1. Pending unavailable dependencies stop at 10,000 operations/64 MiB of
disk-backed encoded bytes per peer and must backpressure/repair rather than
evict arbitrary prerequisites. Resources below 1 MiB remain whole; larger
resources use exact-hash-verified 1 MiB fixed chunks, at most 16,384 per 16 GiB
resource. G9 must still arrange cheap outer/signature/replay gates before
expensive decompression, pin compressor goldens, fuzz all parsers, and ensure
no canonical write occurs before complete verification. These numbers are
desktop proxies and may only be retained or lowered by the v0.8 emulator and
post-1.0 physical-device gates.

**G3 adds local process isolation, not sync authentication.** Generated runtime
profiles use one `0600` config per registry entry, absolute isolated storage
paths, loopback listeners, and `sync.target: none` by default. Startup rechecks
the config/registry/database identity binding and refuses duplicate paths,
ports, or replica IDs; a copied database requires explicit adopt or fork. CLI
show output reports only whether a credential reference is configured, and the
start command passes only the config path. This does not protect a user who can
read the account's files, does not authenticate REST, and does not implement a
sync transport or credential store.

## Deployment posture

Use the default loopback listener:

```yaml
server:
  listen_addr: "127.0.0.1:8080"
```

Current release testing is single-user/local. Public or multi-user deployment
is not approved by this review.

**One exception exists as of v0.7 G13, and G20 enforces it centrally.** The peer sync surface
may be reached from another machine when `sync.rest.enabled` is true, and the
service refuses to start rather than serve it on a non-loopback address without
TLS. That does not make Notrios a networked application: the only routes reachable
with a peer credential are the registered peer endpoints under
`/api/v1/sync/...` for the one database that credential belongs to; the prefix
alone grants nothing. Every other route keeps the local posture above. Exposing the
service itself — the note API, the GUI, MCP — to a network remains unapproved and
is refused by the listener guard.

The G20 security scan also hardened two filesystem trust boundaries. Carrier
and legacy-archive descendants are opened relative to a pinned root, regular
file identity is rechecked, directory iteration and aggregate work are bounded,
and writable carrier partials must be single-link files. Concurrent hostile
mutation is not transactional, and identical no-follow/link-count semantics are
not claimed for unsupported Plan 9 or JavaScript targets. The complete finding
disposition and structural options are in
`performance/v0.7-g20/SECURITY_SCAN.md` and
`performance/v0.7-g20/hardening/`.
