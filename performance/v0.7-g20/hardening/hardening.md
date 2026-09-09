# Security Hardening Review: Notrios v0.7

## Evidence Basis

This portfolio derives from the sealed G20 Codex Security scan of revision
`a3cea5a39f9c0bfe20e80c7615b503ac09967cce`. The scan reported six validated,
high-confidence findings: remote ordinary-API exposure, desktop TLS drift,
cross-origin loopback mutation, unbounded ordinary bodies, hostile carrier
paths, and unsafe legacy-archive paths. Its manifest SHA-256 is
`061c5d52f17208dcd1ec2ebff9f0c2b0abb8048e9c47f5064fe265f71861f661`.

I inspected the affected boundaries and the post-scan G20 changes. The working
tree has intentionally drifted from the scanned baseline because tactical
remediations are being verified there; this design product does not itself
prove those remediations complete.

## Constraints

We are closing v0.7, so the balanced constraint is to preserve loopback REST,
MCP, Wails, authenticated remote peer sync, resumable large snapshots, tolerant
legacy imports, and removable/FUSE carriers without introducing a new service
or protocol. Reverse proxies and unauthenticated remote ordinary APIs are not
supported. No measured latency or memory budget was supplied.

## Opportunity Portfolio

| Opportunity | Evidence | Options | Recommendation | Proposal |
| --- | --- | --- | --- | --- |
| Own network admission in one lifecycle boundary | Remote ordinary-API exposure, desktop TLS drift, browser mutation, and unbounded-body findings | 1. Central listener guard; 2. Split local and peer listeners | Keep Option 1 for v0.7; prefer Option 2 if authenticated general remote access or proxy support becomes a requirement | [Network admission ownership](proposals/network-admission-ownership.md) |
| Keep untrusted trees behind explicit root capabilities | Hostile carrier and legacy-archive findings | 1. Subsystem-owned rooted access; 2. Shared safe-filesystem capability package | Keep Option 1 while there are two semantically different consumers; reconsider Option 2 when a third appears or hardlink/bind-mount policy expands | [Root capability boundaries](proposals/root-capability-boundaries.md) |

## Recommendation Summary

I recommend the two focused Option 1 designs under the release constraints.
They move enforcement to the correct choke points without adding a second
listener, process, or cross-package abstraction during release closure. The
attractive part is not simply smaller code: ordinary network access, TLS
selection, body admission, and descendant filesystem access each gain one
owner that future callers cannot casually bypass.

We should be honest about the limits. A loopback reverse proxy can erase the
real peer address, and a trusted root can contain hardlinks or bind mounts that
`os.Root` alone does not classify as hostile. Those are explicit deployment
assumptions, not properties proved by the v0.7 fixes. A requirement for proxy
deployment or general remote GUI/API access would make split listeners plus
general authentication the better network option. A third hostile-tree
consumer or a stronger cross-platform inode policy would make the shared
filesystem capability package worth its migration cost.

## Next Decisions

- Decide in v0.8 whether remote ordinary access remains unsupported or receives
  general authentication and a separate listener/profile.
- Decide whether reverse proxies, in-root hardlinks, and bind mounts enter the
  supported threat model; until then, keep their current explicit exclusions.
- Revisit shared safe-filesystem infrastructure only when reuse or platform
  policy justifies it; do not abstract the two current consumers prematurely.
