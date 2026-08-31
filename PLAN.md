# Plan: v0.8 — Installation, configuration, shared core, and portability

Status: **Planned from `ROADMAP.md` after v0.7 G20 completed on 2026-08-31.
No v0.8 item is approved or started. Product version remains 0.7.0 and the
canonical database schema remains v27.**

This plan follows `ROADMAP.md`, `FLUTTER_GO_CLIENT.md`, the G18 portability
evidence under `performance/v0.7-g18/`, and the item-writing rules in
`AGENTS.md`. Every item is independently approval-gated. Investigation slices
produce evidence and recommendations; they do not silently adopt a dependency,
change a platform boundary, or begin the implementation that follows.

## Outcome and boundaries

v0.8 turns the source-checkout application into an internally installable,
portable prerelease while extracting one transport-neutral application facade
and a small versioned no-GUI C ABI. The existing REST adapter and Wails v2 GUI
remain supported consumers of the same semantics. One Android emulator must
load and exercise the shared core; physical Android and all iOS client work
remain post-1.0.

Not in v0.8:

- a public GitHub release, production signing/notarization, or distribution to
  an app store;
- a Flutter application, Flutter Web FFI, or a supported mobile release;
- physical Android-device or any iOS runtime claim;
- replacement of Wails v2 without a separately approved and successful spike;
- general remote REST/MCP/GUI authentication or multi-user hosting;
- a second canonical database owner or two SQLite engines concurrently opening
  the same live database;
- evidence-reserve/ISO writes, physical-media burns, pushes, or tags without
  their separate operational authorization.

## H0. Application-facade, C-ABI, and SQLite ownership investigation

**Goal.** Resolve the premises and blocking choices needed to build one
framework-neutral Go application core on desktop and an Android emulator.

**Scope.** Re-audit handler orchestration versus transport-neutral services;
prototype the smallest facade seam; validate `c-shared`/`c-archive`, exported
headers, ownership, cancellation, polling, streams, threads, shutdown, and
packaging. Compare a checksum-pinned upstream SQLite amalgamation control with
the exact `modernc.org/sqlite`/`modernc.org/libc` candidate recorded by G18.
On the approved API-35 emulator ABI, record SQLite version/compile options,
FTS5/JSON/WAL/integrity behavior, store/snapshot/sync tests, checkpointed
desktop/emulator database round-trip, performance, RSS, build/package size, and
proof that one engine owns the canonical file.

**Boundaries.** Investigation only. Do not change the production store driver,
adopt Room/Jetpack, land an ABI, or claim Android/iOS support. Jetpack's bundled
Kotlin driver is not a Go cgo link contract and Flutter Web is not a C-ABI
target.

**Dependencies.** v0.7 G18/G20 contracts and evidence.

**Working state.** A machine-checked recommendation names the facade owner,
SQLite package/version/checksum/update policy, compile options, minSdk/ABI set,
symbol/duplicate-engine policy, concurrency and WAL/crash lifecycle, database
compatibility result, ABI build form, measured costs, and rollback. No
production behavior changes.

**Validation and evidence.** Reproducible desktop and emulator probes, exact
dependency/license provenance, package/RSS/timing tables, cross-engine database
round-trip and integrity results, negative dual-owner test, source-audit drift
check, and an archived decision report.

**Open decisions**

- **Which SQLite implementation owns the v0.8 shared core? — Non-blocking for
  H0; blocking for H1.** Options: checksum-pinned upstream amalgamation with
  cgo, or exact modernc/libc pins. H0 must recommend one from emulator/runtime,
  compatibility, size, performance, maintenance, license, and platform
  evidence. Approving H0 approves only the comparison, not the winner's
  production adoption.
- **Where does the transport-neutral facade live? — Non-blocking for H0;
  blocking for H1.** Options: a new `internal/application` owner (recommended
  starting hypothesis because HTTP remains an adapter), or a narrowed
  `internal/service` package. The report must quantify dependency direction and
  migration cost before selection.
- **Which emulator ABI/minSdk is the v0.8 acceptance target? — Non-blocking for
  H0; blocking for H1/H8.** Default evidence target: the existing API-35
  x86_64 AVD plus an Android/arm64 build-only artifact, with no arm64 runtime
  claim. H0 must state what changes if a second runtime ABI is required.

## H1. Shared application facade and ABI-major-1 library

**Goal.** Extract one transport-neutral application facade used by REST and a
small `cmd/notrioslib` C-ABI wrapper without changing business semantics.

**Scope.** Implement the H0-selected SQLite owner and facade; preserve the G18
ABI-major-1 contract for version/capability query, opaque generation-bearing
instance/call/stream handles, bounded serialized calls, typed errors,
cancellation/polling/events, explicit buffer ownership/release, and bounded
bulk streams. Migrate REST orchestration to the same facade and build/test
desktop shared/static artifacts plus the selected emulator artifact.

**Boundaries.** No Go pointer or live Store/service object crosses C-visible
memory; no callback from an arbitrary Go runtime thread; no one-function-per-
REST-route ABI; no large blob/archive in a JSON result; no new remote authority
or Flutter app. Wails v2 remains the desktop shell.

**Dependencies.** H0 complete with every blocking decision recorded as
resolved inside H0 and this item.

**Working state.** REST regression behavior remains unchanged, the library
opens an isolated profile, bounded calls and streams work, stale/wrong-instance
handles fail with typed errors, and shutdown/cancellation is deterministic.

**Validation and evidence.** ABI/header symbol audit, C host tests, leak/double-
release/stale-handle/concurrent-close tests, REST-versus-ABI parity fixtures,
resource stream/range tests, full Go/web/docs gates, desktop library inspection,
and emulator load smoke.

**Open decisions**

- **H0's facade and SQLite selections — Blocking.** Do not start H1 until H0
  records the selected options, exact pins/checksums, affected build targets,
  and rollback. No default is taken here because the choices change source
  ownership and every packaged binary.

## H2a. Mermaid renderer and security investigation

**Goal.** Determine whether and how the current GUI can enable Mermaid without
weakening offline, CSP, sanitization, or responsiveness guarantees.

**Scope.** Starting from the measured `noMermaid: true` baseline, compare the
minimum locally bundleable renderer/version candidates; inventory licenses,
transitive size, dynamic code/DOM behavior, CSP needs, sanitizer interaction,
malformed/oversized complexity, worker/cancellation options, and desktop/narrow
rendering. Produce representative and adversarial fixtures.

**Boundaries.** Investigation only: no production dependency or feature-status
change, no CDN/runtime fetch, and no execution of diagram source as script.

**Dependencies.** v0.7 G18/G18g offline and documentation-site evidence.

**Working state.** A recommendation, limit policy, failure presentation, bundle
budget, and implementation/rollback plan are archived; Mermaid remains disabled.

**Validation and evidence.** Exact package/license provenance, bundle analysis,
browser/Wails CSP and sanitizer probes, time/memory/DOM limits over adversarial
fixtures, narrow-layout/accessibility review, and zero-network proof.

**Open decisions**

- **Which renderer and containment design should H2 implement? — Non-blocking
  for H2a; blocking for H2.** Default investigation recommendation criteria are
  local-only assets, MIT/Apache compatibility, no CSP widening, bounded abort,
  source-visible failure, and acceptable measured bundle/runtime cost. H2a may
  also recommend keeping Mermaid disabled.

## H2. Bounded offline Mermaid enablement

**Goal.** Implement H2a's accepted renderer/containment option in the existing
GUI while preserving fenced source on every failure.

**Scope.** Pin and bundle the selected renderer, enforce source/complexity/time
limits, sanitize output, render asynchronously with cancellation, add accessible
fallback/status UI, and cover ordinary/malformed/oversized fixtures.

**Boundaries.** No remote assets, CSP relaxation, server-side diagram execution,
arbitrary HTML trust, documentation-site renderer migration, or Wails-version
change.

**Dependencies.** H2a complete and its renderer/containment decision explicitly
approved.

**Working state.** Supported diagrams render locally at desktop/narrow sizes;
unsafe, malformed, oversized, or timed-out diagrams retain readable source and
a bounded error. Existing Markdown/sanitizer behavior remains green.

**Validation and evidence.** Unit/mutation tests, browser and Wails fixtures,
offline/CSP/external-request gates, accessibility and narrow-layout checks,
bundle/time/RSS comparison to H2a, full frontend/Go/docs gates, and rollback.

**Open decisions**

- **H2a renderer/containment recommendation — Blocking.** No fallback is
  implied; if H2a recommends remaining disabled, close or replan H2 instead of
  introducing a different renderer mid-item.

## H3. Installed-path, migration, and permission investigation

**Goal.** Select self-contained config/data/cache/log/runtime locations and a
lossless migration contract for Linux, Windows, and macOS.

**Scope.** Inventory current relative/source-checkout defaults and every path
consumer; compare native directory conventions and permission/backup behavior;
design installed versus portable/source modes, profile registry discovery,
first-run migration, collision/refusal, rollback, uninstall preservation, and
diagnostics. Include database/assets, projections, Recoll, quarantine,
snapshots, logs, web assets, and credentials references.

**Boundaries.** Investigation only; do not move user data or change defaults.
No unrestricted filesystem API, silent cross-volume copy, automatic deletion,
or credential migration without an owning credential-store item.

**Dependencies.** H0 may run in parallel, but H3 must consume the v0.7 profile
and backup/restore contracts.

**Working state.** A per-OS path matrix and atomic/resumable migration design
name detection, preflight space/permissions, backup, interruption, idempotence,
rollback, uninstall, and support diagnostics.

**Validation and evidence.** Disposable old/new layouts, permission and
collision fault injection, interrupted/resumed migration model, profile
discovery matrix, backup/restore proof, and no-user-data-loss oracle.

**Open decisions**

- **Installed versus portable-mode precedence — Non-blocking for H3; blocking
  for H4.** Recommended default: explicit CLI/config overrides first, explicit
  portable marker second, native installed locations otherwise; never infer
  portability from a writable current directory. H3 must show compatibility
  and migration consequences before approval.

## H4. Installed application layout and upgrade/uninstall behavior

**Goal.** Implement H3's accepted path and migration contract and package the
built web UI plus required SQLite/runtime dependencies with the application.

**Scope.** Add explicit installed/source/portable discovery, safe first-run
migration, packaged asset lookup, versioned upgrade/rollback, profile discovery,
and uninstall-preserves-data behavior. Produce internal Linux, Windows, and
macOS package layouts using reproducible build inputs where the host toolchains
permit.

**Boundaries.** Internal prerelease artifacts only; no public upload, signing,
notarization, app-store package, auto-update service, or unsupported platform
claim. Never delete canonical data during uninstall.

**Dependencies.** H1 and H3 complete; H3 precedence/migration decision approved.

**Working state.** Installed binaries find packaged web/runtime assets without
the source tree, create native private locations, discover profiles, migrate a
source layout once, restart cleanly, roll back from a verified backup, and
leave user data on uninstall.

**Validation and evidence.** Clean install, upgrade, interrupted migration,
rollback, uninstall/reinstall, missing/tampered asset, permissions, and package
inventory/license tests on each honestly supported build host.

**Open decisions**

- **Package formats per desktop OS — Non-blocking until H4 planning; blocking
  before artifact writes.** Recommended initial internal set: native Linux
  package plus unpacked verified Windows/macOS layouts when signing/toolchains
  are unavailable. H4 must record which formats are real installs versus
  layout evidence and must not label an unexecuted artifact supported.

## H5. Installed multi-profile and native-integration matrix

**Goal.** Prove installed instances preserve profile isolation and interact
safely with OS networking, deep links, shared directories, and native pickers.

**Scope.** Exercise multiple profiles/servers, port conflicts, URL-handler
registration, shared-carrier paths, firewall prompts, file/directory pickers,
GUI/service modes, restart, upgrade, and concurrent isolation in installed
Linux, Windows, and macOS environments available to the project.

**Boundaries.** No general remote API exposure, automatic firewall widening,
silent handler takeover, or fabricated result for an unavailable OS. Native
pickers return capabilities/selected paths only to approved local operations.

**Dependencies.** H4 complete; H1 for shared-core lifecycle parity.

**Working state.** Every available platform has a result-bearing matrix;
unavailable combinations remain explicit with owner/reason. Profile/database/
replica/path/port isolation and deep-link ambiguity refusal remain intact.

**Validation and evidence.** Multi-instance process tests, collision/fault
fixtures, handler install/remove/readback, shared-drive removal, picker cancel/
permission tests, firewall observation, desktop GUI smoke, and cleanup audit.

**Open decisions**

- **Minimum OS matrix required to call H5 complete — Non-blocking default.**
  Default: real Ubuntu execution plus available Windows/macOS CI or VM execution;
  any unavailable native interaction is reported unverified and blocks a broad
  support claim, not the evidence slice itself. Approving H5 approves that
  honest-coverage default.

## H6. Native credential-store selection and integration

**Goal.** Replace the warned `0600` development secret file in installed
profiles with native credential-store providers while retaining an explicit
development fallback for source/test use.

**Scope.** Investigate then pin providers for supported Linux, Windows, and
macOS installations; define reference format, create/read/update/delete,
locked/unavailable/headless behavior, migration, backup exclusion, revocation,
and profile isolation. Evaluate Android only for H8 feasibility; do not use a
Flutter-side store as the Go core's hidden owner.

**Boundaries.** Never log, export, commit, or place secret bytes in evidence;
never silently fall back from an installed native store to plaintext; no
credential-management REST/MCP surface. `zalando/go-keyring` is a desktop
candidate, not an assumed Android answer.

**Dependencies.** H1 provider interface and H3/H4 installed-mode identity.

**Working state.** Supported installed profiles resolve opaque credential
references through the selected native store, fail closed when locked or
unavailable, and migrate only with explicit confirmation. Source/test mode
continues to label the owner-only file provider as development-only.

**Validation and evidence.** Exact dependencies/licenses, mocked contract
suite, native readback/delete/lock/session tests on available OSes, migration
and refusal fixtures, log/repository scans for secret material, and rollback.

**Open decisions**

- **Provider per supported OS — Blocking before implementation.** The item may
  perform a bounded comparison first, but no provider is adopted until its
  availability, headless behavior, license, maintenance, packaging, and
  rollback are recorded. Android may remain unresolved for v0.8 if H8 uses a
  test-only injected secret provider and makes no mobile-release claim.

## H7. Wails v3 migration spike

**Goal.** Determine whether Wails v3 can replace v2 later without risking the
v0.8 desktop product.

**Scope.** In an isolated prototype, compare dependency/license state, desktop
builds, bindings, menus/dialogs, web assets, deep links, lifecycle, multiple
profiles, packaging, and rollback. Record upstream beta/experimental status and
Android/iOS limitations current at execution time.

**Boundaries.** Investigation only and separately approved. Do not migrate the
production shell, remove Wails v2, or claim mobile support.

**Dependencies.** H4/H5 contracts provide the desktop baseline; the spike may
be scheduled later if current upstream maturity makes it low value.

**Working state.** A disposable, reproducible prototype and decision report
recommend migrate, defer, or reject. Production remains Wails v2.

**Validation and evidence.** Exact upstream/dependency provenance, build and
desktop regression matrix, package/RSS/startup comparison, native-integration
gaps, rollback rehearsal, and no-production-diff check.

**Open decisions**

- **When is the spike worth running? — Non-blocking default.** Default: run
  only after H5 establishes the v2 installed baseline and only on explicit user
  approval. If upstream remains beta or required desktop features regress,
  recommend deferral without a migration item.

## H8. Android-emulator shared-core acceptance

**Goal.** Prove the H1 library is a viable backend on one Android emulator
without presenting an Android or Flutter product.

**Scope.** Package/load the selected ABI and SQLite owner; exercise instance
open/close, profile sandbox, SQLite bootstrap/reopen/reboot, CRUD/FTS5 search,
bounded resource stream, cancellation/polling, sync capability negotiation,
WAL/integrity, crash/restart, and desktop/emulator checkpoint interchange.

**Boundaries.** No physical device, iOS, UI, app-store artifact, background/
battery claim, production secure-store claim, or Flutter client. No second
SQLite engine may open the canonical file.

**Dependencies.** H0 decisions and H1 complete; H3 path contract applied to an
emulator sandbox. H6 Android provider may remain a documented gap if secrets
are injected only by the test host.

**Working state.** The exact emulator/API/ABI loads the shared library and all
bounded lifecycle/storage/search/stream/cancel/sync probes pass or produce typed
failures. Unsupported ABIs/platforms remain explicit.

**Validation and evidence.** Clean/cold/reboot runs, ABI/symbol and package
inventory, SQLite version/options, FTS5/JSON/WAL/integrity, crash injection,
desktop round-trip, timing/RSS/package-size measurements, adb cleanup, and
leftover-process audit.

**Open decisions**

- **Runtime ABI beyond the H0 default — Non-blocking default.** Default: require
  the existing API-35 x86_64 runtime and an Android/arm64 build-only artifact.
  A physical/arm64 runtime remains post-1.0 unless separately authorized; this
  limits the support claim rather than weakening the x86_64 acceptance gate.

## H9. v0.8 installation and portability release wrap-up

**Goal.** Reconcile every approved v0.8 promise and produce internal installable
prerelease artifacts plus a verified source snapshot.

**Scope.** Run full repository, ABI, installed-path/migration, package,
multi-profile/native-integration, credential-store, Mermaid (if approved), and
Android-emulator gates. Reconcile product/version/schema/docs/API/dependency
licenses, security posture, upgrade/rollback/uninstall, supported-platform
claims, and archive the milestone.

**Boundaries.** No public GitHub release, push, tag, signing/notarization,
app-store upload, physical mobile artifact, evidence reserve/ISO write, or burn
without separate authorization. Fix only defects in approved v0.8 contracts;
new features return to planning.

**Dependencies.** H0-H8 as applicable; a deferred H2/H7 must have an explicit
closed disposition rather than an invented result.

**Working state.** Product/docs/packages agree, every claimed platform has real
evidence, unsupported combinations are explicit, internal artifacts verify,
the v0.8 plan is archived, and the next plan is derived from `ROADMAP.md` only
after user review.

**Validation and evidence.** Full Go/frontend/docs/security/dependency gates;
ABI/header and emulator matrices; clean install/upgrade/rollback/uninstall;
artifact inventories and hashes; source ZIP through
`scripts/package_release.sh`/`check_release_zip.py`; release checklist and
handoff updates.

**Open decisions**

- **v0.8 product/schema number — Non-blocking until H9.** Default: product
  `0.8.0`; change schema only for a canonical migration actually required by an
  approved item. The completion report must list every schema step rather than
  incrementing for packaging alone.
- **Which internal artifacts are retained and where? — Non-blocking default.**
  Default: retain only verified, non-secret prerelease artifacts in the local
  evidence directory; no reserve/ISO/media write. Any external publication or
  custody action requires separate approval.

## Decisions register

This is an index only; each decision is owned and explained inside its item.

| Decision | Owner | Status |
|---|---|---|
| Shared-core SQLite owner/version/checksum | H0/H1 | Open; H0 investigation, blocking H1 |
| Application facade package owner | H0/H1 | Open; H0 investigation, blocking H1 |
| Emulator ABI/minSdk acceptance | H0/H8 | Open with API-35 x86_64 default |
| Mermaid renderer/containment | H2a/H2 | Open; H2a investigation, blocking H2 |
| Installed/portable path precedence | H3/H4 | Open with explicit-override/native default |
| Desktop package formats/support claims | H4/H5 | Open; evidence-dependent |
| Native credential providers | H6 | Open and blocking implementation |
| Wails v3 spike timing/outcome | H7 | Explicit approval required; production stays v2 |
| v0.8 product/schema number | H9 | Open with product 0.8.0/no gratuitous schema default |
| Internal artifact custody | H9 | Local evidence-only default; external actions separately authorized |

H0 is the next incomplete item and remains unapproved. Do not begin it until the
user explicitly says to proceed with H0.
