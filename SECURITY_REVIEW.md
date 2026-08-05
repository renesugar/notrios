# Security Review — Current Local Product and Planned Remote Surfaces

This review reflects the repository after v0.3 H10. Notrios is local-first but
its REST/MCP listener, importers, preview, downloaded media, future archive
files, and future sync transports are security boundaries.

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
