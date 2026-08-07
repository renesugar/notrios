# Security Review — Current Local Product and Planned Remote Surfaces

This review reflects the repository after v0.4 P8. Notrios is local-first but
its REST/MCP listener, importers, preview, downloaded media, archive files,
published handoffs, and future sync transports are security boundaries.

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

Remaining: generic upload limits and optional malware-scanner integration.
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
- Raw SQL, arbitrary filesystem access, and direct Recoll mutation are absent.
- MCP output is bounded and marks note content untrusted.
- Editor writes are profile-gated and destructive edits use revision
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

None of these surfaces is exposed over MCP. Lint, fix, graph traversal, block
listing, query-block evaluation, tag rename, notebook deletion, garbage
collection, archive operations, and publication are REST/CLI only. That keeps
whole-library reads and organizer writes on surfaces a person drives rather than
ones untrusted model output reaches.

Do not expose Notrios on a LAN/public address until authentication,
authorization, CSRF/CORS, TLS/reverse-proxy guidance, rate/request quotas, and
audit logging are implemented and tested. Future import/export/sync MCP tools
are a bounded control plane only; bulk bytes travel through constrained REST or
hash-verified objects.

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

## Planned synchronization threat boundary

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

## Deployment posture

Use the default loopback listener:

```yaml
server:
  listen_addr: "127.0.0.1:8080"
```

Current release testing is single-user/local. Public or multi-user deployment
is not approved by this review.
