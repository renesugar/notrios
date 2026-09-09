# Reference and dependency validation

Observed 2026-08-11. This is an engineering/license screen, not legal advice and
not a dependency lock. No dependency was added or upgraded in G0.

Primary project/package pages establish purpose, platform claims, and license.
Version observations use the primary package registry where available and
`git ls-remote --tags --refs` against the upstream repository. A tag's presence
does not by itself establish maturity or security. G9/G13 must pin exact
versions and review transitive licenses, advisories, release notes, supported Go
version, and cryptographic test vectors before adoption.

## Proposed dependency and platform matrix

| Candidate / reference | Status observed | License | Upstream platform/runtime claim | G0 disposition and reason |
|---|---|---|---|---|
| Go [`crypto/ed25519`](https://pkg.go.dev/crypto/ed25519) | Standard library in local Go 1.26.5; project language level is Go 1.25 | BSD-3-Clause | Go-supported platforms; constant-time private-key operations documented | **Preferred implementation candidate for G9.** It implements the resolved Ed25519 requirement without a new module. G9 still specifies domain separation, canonical bytes, key storage, and malformed-length handling; the primitive does not solve those. |
| [`golang.org/x/crypto/chacha20poly1305`](https://pkg.go.dev/golang.org/x/crypto/chacha20poly1305) | Registry showed module v0.54.0; this repository already has `x/crypto` v0.51.0 indirectly | BSD-3-Clause | Pure Go/assembly by Go target; exposes ChaCha20-Poly1305 and XChaCha20-Poly1305, with the extended nonce documented for random nonces | **AEAD candidate for G9, not selected by G0.** Exact algorithm, FIPS posture if required, nonce construction, associated data, streaming/container composition, and pin are G9 outputs. Promote to a direct pin only after that review. |
| [`golang.org/x/crypto/argon2`](https://pkg.go.dev/golang.org/x/crypto/argon2) | Same x/crypto module/status; `IDKey` implements Argon2id | BSD-3-Clause | Go-supported platforms; memory/CPU use depends on parameters | **Password-KDF candidate for G9/G10.** RFC recommendations are not blindly usable on constrained devices; G2 must measure desktop proxy bounds and G9 must version parameters/salt before selection. |
| [`zalando/go-keyring`](https://github.com/zalando/go-keyring) | Latest tag observed `v0.2.8`; repository active and carries a security policy | MIT | README: macOS through `/usr/bin/security`, Linux/BSD through Secret Service D-Bus, Windows Credential Manager | **Desktop secret-store adapter candidate for G13/G17 only.** It is not Android/iOS support, Linux requires a working Secret Service collection, and its string API/provider failures need lifecycle tests. Never become the core abstraction or plaintext fallback. |
| [`flutter_secure_storage`](https://pub.dev/packages/flutter_secure_storage) | pub.dev `11.0.0`, published five days before observation | BSD-3-Clause | Android, iOS, Linux, macOS, web, Windows; platform prerequisites and Android backup/migration warnings are documented | **Post-1.0 Flutter adapter candidate, not a v0.7 Go dependency.** Must be validated per platform, especially Linux keyring availability, Android backup exclusion/migration, Web HTTPS/origin behavior, and biometric accessibility. |
| [`rclone`](https://github.com/rclone/rclone) | Latest tag observed `v1.75.0` | MIT | Cross-platform CLI with many cloud/local backends | **Optional external test carrier only.** Official [`copy`](https://rclone.org/commands/rclone_copy/) does not delete destination files, and `--immutable` refuses modifications; official docs also warn `sync`/`move` can delete even with `--immutable`. No runtime/mobile dependency, OAuth coupling, or merge semantics. |
| Apache [Subversion](https://subversion.apache.org/docs/release-notes/) | 1.14.x is current LTS; 1.15 is in progress | Apache-2.0 | Source project; volunteer binary packages listed for Unix-like systems, macOS, and Windows | **Reference only.** Reuse monotonic revisions, change-log/state-summary, base-delta, dump filtering, and catch-up concepts. Do not link, port, or adopt `svnadmin dump`; Notrios has multi-writer offline merge, resources, encryption, and archive-v2 compatibility needs that SVN grammar does not express. |
| [`maxpert/marmot`](https://github.com/maxpert/marmot) | Current README describes Marmot v2; latest tag observed `v2.9.13-beta` | MIT | Go distributed SQLite server using gossip, MySQL protocol, distributed transactions, CDC, LWW/HLC, and server/cluster infrastructure | **Architecture reference only.** HLC ordering, row CDC, immutable/atomic CDC publication, anti-entropy, and snapshots are useful patterns. Its always-on distributed SQL server, LWW row replication, gossip/quorum, parser, and operational dependencies do not fit offline single-user Notrios carriers. |
| Cachapa [`crdt`](https://pub.dev/packages/crdt), [`sqlite_crdt`](https://pub.dev/packages/sqlite_crdt), and [`crdt_sync`](https://pub.dev/packages/crdt_sync) | Registry versions `5.1.3`, `3.0.4`, `1.0.10`; crdt/crdt_sync were published 21 months earlier and sqlite_crdt 9 months earlier | Apache-2.0 | Dart/Flutter; core and sync list Android/iOS/Linux/macOS/Web/Windows; sqlite adapter documents mobile, desktop/server prerequisites and experimental Web support even though registry platform detection shows Web | **Conceptual reference only.** Compact HLC/LWW records and WebSocket changesets are useful. Dart implementation, whole-record semantics, and missing Notrios tree/revision/resource/ack-GC requirements make wholesale adoption/porting a poor fit. The platform-report inconsistency reinforces per-target validation. |
| [`reearth/ygo`](https://github.com/reearth/ygo) | Latest tag observed `v1.48.0`; README states stable v1 API | MIT | Pure-Go Yjs V1/V2, server/persistence, and `gomobile` Android/iOS claims | **Deferred optional live-coediting candidate.** It solves Yjs document collaboration, not Notrios relational/database replication. If live editing enters scope, run a separate interoperability/security/size/mobile benchmark and choose among implementations then. |
| [`Deln0r/ygo`](https://github.com/Deln0r/ygo) | Latest tag observed `v1.15.0`; pkg.go.dev/repository describe pure-Go Yjs and mobile bindings | MIT | Pure Go, Yjs wire compatibility, server/client, `gomobile` claims | **Same deferred disposition as reearth/ygo.** Multiple fast-moving implementations are a reason to benchmark and pin later, not to make a v0.7 protocol depend on one now. |
| [`jedisct1/minisign`](https://github.com/jedisct1/minisign) | Latest tag observed `0.12`; upstream README says new features are going to minizign | ISC | Cross-platform command-line signing tool, Ed25519-based | **Rejected as a protocol/runtime dependency.** It would add an external executable, a separate key/file/signature grammar, process/argument handling, and packaging burden. Notrios needs in-process per-replica signatures bound to typed canonical artifacts, not detached signatures over a mutable folder. The general “sign before trusting carrier bytes” lesson is retained. |

All licenses above are permissive. The actual v0.7 dependency rule remains
MIT/Apache-2.0/BSD-compatible code only. Reference-only software is not copied,
ported, linked, vendored, or redistributed merely because its license would
permit it.

## Which supplied ideas fit

| Idea | Keep | Change / reject |
|---|---|---|
| State vector + change log | Per-replica contiguous sequences summarize what a peer durably applied; missing ranges drive delta exchange. | It is not a single central SVN revision, timestamp, or carrier manifest. Multi-writer causal gaps and retirement floors are explicit. |
| Base delta | An optional delta from a named parent can reduce transfer; complete result hash/object remains authoritative. | No SVN dump grammar and no unbounded delta chain. Missing/malicious base falls back to complete object or conflict/recovery. G1 chooses algorithm. |
| Immutable mailbox/shared ledger | Disjoint replica namespaces, immutable artifacts, stage/hash/rename, manifest last, scan/poll/manual correctness. | No shared mutable `manifest.json`, clearing another peer's mailbox, mtime trust, or watcher dependency. Carrier may disappear. |
| rclone/cloud folder | `rclone copy --immutable` can exercise the same directory protocol against mapped Google Drive. | No `sync`, `bisync`, `move`, delete, purge, bundled executable, mobile dependency, or separate cloud merge logic. |
| Detached signatures | Authenticate artifacts before trusting carrier content; per-device keys enable attribution/revocation. | In-process Ed25519 over domain-separated canonical artifacts, not one minisign signature over a folder/file and not hash-as-authentication. Payload encryption is separately mandatory. |
| OS keyring | Private keys/credentials should be referenced from platform secret storage, not config/argv/logs. | One injected interface with tested providers; `go-keyring` is desktop-only and failure must never fall back to a file silently. |
| Argon2id backup password | Memory-hard, salted, versioned password-derived wrapping key; wrong password is non-destructive. | Parameters require G2 device evidence; no online-only rate-limit assumption, stored password, universal fixed cost, or recovery backdoor. |
| SQLCipher | Local database-at-rest encryption could reduce powered-off device disclosure. | Separate feature: it neither secures carrier artifacts nor replaces OS/key lifecycle. No SQLCipher dependency in v0.7 G0. |
| QR pairing | Useful presentation for a small one-use enrollment offer/fingerprint. | QR bytes are untrusted input, not authority; require expiry, explicit confirmation, proof of possession, length limits, and no secret persistence/screenshots claim. Library choice waits for G4. |
| Whole-record HLC/LWW | HLC tie-breaking and compact record examples inform G5/G6. | Not a blanket merge rule. G1/G7 own body merge and per-field/set/tree semantics; wall clock never replaces replica sequence/dependency tracking. |
| Yjs/Ygo | Candidate for a later live collaborative editor and useful source of state-vector/interoperability test ideas. | Not the database sync engine and not justified for ordinary offline revision merge. |
| Native direct API on mobile | One transport-neutral replication core should serve desktop and future mobile without rclone. | v0.7 ships desktop/core evidence; mobile secret storage, lifecycle, scoped storage, battery, and physical-device support belong to later milestones. |

## Security construction constraints passed to G9

This matrix deliberately does not choose the full construction. G9 must produce
one versioned algorithm suite and test vectors satisfying all of these:

- standard-library Ed25519, with canonical artifact domain/type/version binding;
- an approved AEAD from a maintained BSD/MIT/Apache implementation;
- unique/non-repeating nonces under crash, clone, and concurrent publication;
- visible header bound both by signature and AEAD associated data;
- separate signing and encryption key IDs/lifecycles;
- recipient/key epochs and future-epoch exclusion after compromise revocation;
- versioned Argon2id password parameters selected from G2 evidence;
- no sender-selected downgrade, plaintext fallback, or algorithm proliferation;
- deterministic golden/cross-version fixtures and malformed-input/fuzz tests;
- zeroization/lifetime limitations documented honestly for Go-managed memory;
- secret-store failures are explicit and never write private material to config,
  local database, carrier, job record, CLI argument, or log.

## Primary source ledger

- Go Ed25519: <https://pkg.go.dev/crypto/ed25519>
- Go XChaCha20-Poly1305 candidate: <https://pkg.go.dev/golang.org/x/crypto/chacha20poly1305>
- Go Argon2id candidate: <https://pkg.go.dev/golang.org/x/crypto/argon2>
- Go keyring: <https://github.com/zalando/go-keyring>
- Flutter secure storage: <https://pub.dev/packages/flutter_secure_storage>
- rclone immutable/copy semantics: <https://rclone.org/docs/#immutable> and
  <https://rclone.org/commands/rclone_copy/>
- Subversion supported releases: <https://subversion.apache.org/docs/release-notes/>
- Marmot: <https://github.com/maxpert/marmot>
- Cachapa packages: <https://pub.dev/packages/crdt>,
  <https://pub.dev/packages/sqlite_crdt>, and
  <https://pub.dev/packages/crdt_sync>
- Go Yjs candidates: <https://github.com/reearth/ygo> and
  <https://github.com/Deln0r/ygo>
- Minisign: <https://github.com/jedisct1/minisign>
