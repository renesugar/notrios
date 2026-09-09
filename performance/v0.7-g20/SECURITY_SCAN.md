# G20 security scan and remediation record

## Canonical baseline scan

Codex Security Standard scan `a8529c1e-bc60-472a-ae36-de6067741fdd`
completed against Git revision
`a3cea5a39f9c0bfe20e80c7615b503ac09967cce` on 2026-08-31.

- Scan mode and coverage: repository, complete, seven reviewed surfaces, no
  deferred candidates.
- Findings: six validated, high confidence; two high severity and four medium
  severity.
- Snapshot: `codex-security-snapshot/v1:sha256:ca4e5ab0f03fd2ae4edac30421288c61c159b04f856021f589b5b7baa548a311`.
- Sealed manifest SHA-256:
  `061c5d52f17208dcd1ec2ebff9f0c2b0abb8048e9c47f5064fe265f71861f661`.
- Findings SHA-256:
  `0e84855ae4f18d528d23a2965d88643351a0748ecfa363b5d8f1b78136eaf7ba`.
- Coverage SHA-256:
  `f9ac37f7b978fd43e42234cf7de278ca06b9da10c52f64049e2249663ca9b840`.
- Limitation: the TAC connector could not be verified in this environment.

The canonical scan describes the vulnerable baseline. The G20 working tree is
deliberate source drift and contains the candidate remediations summarized
below. The scan seal is evidence of the findings, not evidence that the later
patches work.

## Finding disposition

| Finding | Baseline severity | G20 disposition |
| --- | --- | --- |
| Non-loopback listener exposes unauthenticated ordinary APIs (`csf_806a49159c767d9afd647cb6`) | High | Addressed by a finite remote peer-route mux plus actual loopback-peer and local-Host admission for ordinary APIs. Forwarding headers grant no authority; unknown sync-looking routes do not fall through to the web UI. |
| Desktop command serves plaintext despite TLS configuration (`csf_ed7b60861f56c5e547852a9a`) | High | Addressed by one `Service.ListenAndServe` selector used by both executables and refusal of partial certificate/key configuration. GUI-only implicit URLs select the configured scheme. |
| Cross-origin pages can mutate the loopback API (`csf_493daca733a8eb3401c88af2`) | Medium | Addressed by central unsafe-method origin admission for exact HTTP/HTTPS application origins and Wails, with malformed, opaque, mismatched, and cross-site requests refused. Listener Host admission closes the DNS-rebinding equal-origin form. |
| Ordinary request bodies are unbounded (`csf_c86971e0236bb990154d64fa`) | Medium | Addressed by an 8 MiB exact-one-value JSON envelope, the same bounded/EOF policy for sync-control and Sync UI decoders, HTTP and store 16 GiB resource ceilings, and partial-temp cleanup. |
| Hostile carrier entries can escape or block filesystem operations (`csf_adb83c6f7c60a4270cef81ff`) | Medium | Addressed by root-relative descriptor operations, no-follow/nonblocking directory opens on Linux/Android/macOS, bounded directory batches and scan counts, regular-file/identity/size checks, single-link writable snapshot partials, and unlink-before-create fallback. |
| Legacy archive import follows unsafe descendant entries (`csf_ff3c60302e53c6c528fb82d3`) | Medium | Addressed by one pinned selected root, regular descriptor checks, preflight before canonical writes, incremental inventories, 1,000,000-note/resource and 2,000,000-scanned-entry ceilings, 16 GiB aggregate notes, 1 TiB aggregate resources, and one validated resource-name inventory. |

## Independent correction-cycle review

A fresh read-only reviewer inspected only the original findings and the
candidate patch. It confirmed five gaps: writable hardlinked snapshot partials,
blocking/unbounded carrier directory listing, unbounded legacy archive
cardinality/aggregate work, prefix-based remote sync route admission, and two
alternate single-decode JSON helpers. G20 corrected all five before final
validation. The workflow permits one such correction cycle; no second review
was used to manufacture agreement.

## Verification contract

The security gate runs the complete `internal/service`, `internal/httpapi`,
`internal/archive`, and `internal/synccarrier` suites. Focused regressions cover
remote ordinary/peer/unknown routes, spoofed forwarding and DNS-rebinding Host,
TLS and partial TLS, browser origins, exact bounded JSON and trailing values,
resource admission, carrier symlink/hardlink/special/cardinality cases, large
snapshot resume, and archive link/special/count/aggregate behavior before
persistence.

Linux is the release validation platform for descriptor anchoring. Windows has
a native open-handle link-count implementation; Unix link counts are read from
the opened descriptor metadata. Plan 9 and js fail closed for writable partials
and are not supported v0.7 runtime targets. On non-Linux/Android/macOS systems,
`os.Root` plus lstat/open/fstat identity remains active, but the Unix
no-follow/nonblocking directory-race guarantee is not claimed without platform
execution evidence.

## Structural hardening guidance

The derived, revisable portfolio is [hardening/hardening.md](hardening/hardening.md),
with machine-readable analysis in [hardening/hardening.json](hardening/hardening.json).
It recommends the focused central-listener guard and subsystem-owned root
capabilities for v0.7. Split listeners/general authentication and a shared
safe-filesystem package remain design options for v0.8 if their triggering
requirements appear. These proposals are not additional claims of remediation.
