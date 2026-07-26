# Security Review — Current Local Product and Planned Remote Surfaces

This review reflects the repository after v0.3 H6. Notrios is local-first but
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
corpus. A future `notrios://` OS handler must validate scheme, length,
profile/database IDs, route type, and stale targets; malformed external input
must never switch profiles or invoke arbitrary filesystem paths.

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

Remaining v0.3/v0.4 work: bounded batches/checkpoints, exact source bundles,
path/symlink/zip-bomb defenses for every container, size/count/depth ceilings,
manifest-last/checksum verification, snapshot consistency, and explicit
archive compatibility. Native archive v1 is not a disaster-recovery backup.

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
