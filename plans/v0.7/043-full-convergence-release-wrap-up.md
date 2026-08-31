# v0.7 G20 — full convergence, disaster recovery, security, and release wrap-up

Date: 2026-08-31
Models: GPT-5 (exact serving variant unavailable); Luna workers for bounded
license inventory, carrier/archive hardening, and evidence cross-reference
validation; parent architectural/security decisions, correction review, and
final acceptance
Working state: complete

## Goal and boundaries

Close v0.7 as one supported system: reconcile the milestone promises with
source, validate convergence and recovery across the implemented transports,
run a standard security scan and correct its validated findings, freeze exact
dependency-license evidence, prove clean schema upgrade, and produce a verified
source release candidate.

This slice adds no new transport, installer, mobile client, user-authentication
system, public deployment claim, or archive format. It does not reread the
private source corpus. No GitHub push, tag, public release, evidence-reserve or
ISO write, or physical-media burn is authorized by G20.

## Release identity and frozen evidence

The selected release identity is product `0.7.0`, canonical database schema
v27, with the v0.7 migration sequence v19-v27. Fresh bootstrap and upgrade
tests exercise the same result. `performance/v0.7-g20/REPORT.json` binds the
release matrix to aggregate-only evidence already accepted in earlier slices:

- G14e: 382,206 equivalent documents per full-scale view, including the
  103,349-document attachment view with 758 resources and 731 blobs;
- G17: 382,206 generated retention operations without rereading private data;
- G8: lazy materialization at 64 KiB, 1 MiB, 4 MiB, and 16 MiB, eight
  attachments per size.

The report pins each evidence file by SHA-256. G20 validators fail closed on
release identity, evidence drift, missing gates, advisory disposition,
dependency inventory, security scan identity, and hardening cross-references.

## Security scan and correction cycle

The standard repository scan completed with scan id
`a8529c1e-bc60-472a-ae36-de6067741fdd` against baseline
`a3cea5a39f9c0bfe20e80c7615b503ac09967cce`. Coverage is complete over seven
enumerated surfaces with no deferred surface. The sealed manifest, findings,
and coverage SHA-256 values are recorded in `REPORT.json` and
`SECURITY_SCAN.md`; six high-confidence findings were validated:

- ordinary REST/MCP/web routes could be reached after a non-loopback bind;
- CLI server paths could bypass centralized TLS selection;
- browser mutations lacked an explicit cross-origin boundary;
- JSON and raw-resource request bodies were not uniformly bounded;
- carrier descendant filesystem operations retained ambient path authority;
- legacy archive reads followed hostile descendants and lacked aggregate work
  bounds.

All six received source fixes and focused regressions. One allowed fresh static
bypass review found five narrower residual paths: hard-linked carrier partials,
special/unbounded carrier directories, unbounded archive aggregate/cardinality,
unknown sync-prefix fallthrough, and two alternate one-decode JSON helpers.
The single correction cycle closes all five and reruns the affected suites; no
second reviewer was used.

The service now owns TLS/plain serving selection and refuses partial TLS
configuration. Ordinary routes require a loopback connection and local Host;
remote connections enter a finite peer-sync mux rather than a string-prefix
allowlist. Browser writes require exact same-origin/Wails admission. JSON/MCP
bodies are capped at 8 MiB and exactly one value, while resource streaming and
the store share the 16 GiB object limit.

Carrier and legacy-archive descendants are opened relative to pinned roots,
regular identity is checked, directory reads are batched and bounded, and
archive note/resource counts plus aggregate bytes are preflighted before
canonical mutation. Writable carrier partials must be single-link files and
non-atomic fallback creates a fresh exclusive inode. The archive imports the
preflighted resource inventory rather than re-enumerating a hostile tree.

Residual limits are explicit: modern browser Origin/Fetch Metadata is assumed
for cross-site mutation defense; a separately configured loopback proxy can
republish local authority; concurrent hostile filesystem mutation is not a
transaction; and identical no-follow/link-count behavior is not certified on
Plan 9 or JavaScript targets. A Windows full-repository cross-build remains
blocked by unrelated cgo-excluded store types, so it is not claimed as G20
evidence.

## Structural hardening portfolio

The security workflow produced the derived portfolio under
`performance/v0.7-g20/hardening/`. It recommends the implemented central
listener guard for v0.7 and retains split local/peer listeners as a future
option if general remote/proxy requirements are approved. It recommends
subsystem-owned rooted filesystem capabilities now and defers a shared safe-root
package until another consumer or a stronger cross-platform policy justifies
the migration. Both opportunities include before/after Mermaid diagrams, exact
finding coverage, six-dimensional tradeoffs, acceptance criteria, migration,
rollback, and residual risks. The portfolio is a proposal artifact, not a
second implementation.

## Dependency, upgrade, and recovery acceptance

`DEPENDENCY_LICENSES.json` inventories the exact 37 Go modules and 251 npm
packages represented by committed module/lock files. Its offline validator
rejects drift, duplicates, unknown licenses, or policy-incompatible licenses.
All project-linked dependencies remain compatible with MIT/Apache-2.0 policy;
GPL Recoll/Xapian remains an optional external process.

`UPGRADE_ROLLBACK.md` records fresh bootstrap, sequential v19-v27 upgrade,
schema/security-surface survival, and the only supported rollback: take and
verify a pre-upgrade physical backup, then restore that complete database/assets
state with the older binary. In-place schema downgrade is not supported.

## Validation

The exact G20 release selectors pass for randomized three-replica convergence,
two-process directory and REST carriers, directory deletion/recreation,
catch-up/reset/retention/full-resync, lazy attachments, conflict and admission
faults, abuse cases, and fresh/upgrade schema paths. `make g20-validate` passes
11 evidence unit tests, the exact dependency inventory, all service/httpapi/
archive/synccarrier tests, and the frozen G8/G14e/G17 validators.

The regular validation pass also runs unrestricted `go test ./...`, `go vet
./...`, scaffold and required-file checks, documentation generation/anchor/
example/journey/site evidence, zero-vulnerability npm audits around clean
lockfile installs, frontend typecheck/tests/build, smoke/performance checks, and
the release packager/verifier. A documentation example hash changed when the
unsafe cross-machine GUI example was corrected to a loopback tunnel; its
registry digest was updated and both `internal/docaudit` and
`internal/docexec` then passed.

## Outcome

**Outcome (2026-08-31).** Complete, archived as
`plans/v0.7/043-full-convergence-release-wrap-up.md`. Product 0.7.0/schema v27
now has a closed release matrix, exact license inventory, upgrade/rollback
contract, complete standard security scan, one closed bypass-correction cycle,
and machine-checked structural-hardening portfolio. The v0.7 plan is archived
as `plans/v0.7/000-v0.7-plan.md`; the replacement `PLAN.md` contains the
unstarted v0.8 milestone derived from `ROADMAP.md`. Publishing and external
evidence/media actions remain separately approval-gated.
