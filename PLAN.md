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

## H0. Application-facade, C-ABI, and SQLite ownership investigation — complete

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

**Open decisions — resolved 2026-08-31**

- **Which SQLite implementation owns the v0.8 shared core? — Resolved:**
  checksum-pinned official SQLite 3.53.4 amalgamation with cgo and hidden
  static linkage. Exact modernc/libc remained compatible but lost on real-store
  migration, runtime-support, size, RSS, and measured execution evidence.
- **Where does the transport-neutral facade live? — Resolved:** a new
  `internal/application` owner. `internal/service` remains an HTTP-coupled
  composition/lifecycle package and is not the application contract.
- **Which emulator ABI/minSdk is the v0.8 acceptance target? — Resolved for
  H1:** API-35 x86_64 is runtime-qualified; Android arm64-v8a is build-only
  until H8 repeats the full runtime matrix. No lower API or arm64 runtime claim
  is implied.

**Outcome (2026-08-31).** Selected a new `internal/application` facade owner,
the checksum-pinned official SQLite 3.53.4 amalgamation with cgo/hidden static
linkage, ABI major 1's frozen 12-symbol polling/stream contract, and API-35
x86_64 as the only runtime-qualified Android target; arm64-v8a is build-only.
The unchanged schema-v27 store/snapshot/sync code passed on Linux and the
emulator, the C-modernc-C desktop/Android round-trip passed, and measured
costs, ownership/WAL/crash policy, update/rollback, exact provenance, and
limitations are archived in
`plans/v0.8/002-application-facade-c-abi-sqlite-ownership-investigation.md`.
No production facade, driver, ABI, dependency, schema, or support claim was
landed.

## H1. Shared application facade and ABI-major-1 library — complete

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

**Open decisions — resolved by H0 on 2026-08-31**

- **H0's facade and SQLite selections — Resolved.** H1 uses a new
  `internal/application` owner and official SQLite amalgamation 3.53.4 with
  archive SHA3-256
  `628a44cfe82c66aed1ccbbe85a562d2e33ebe64b3288981ed76285612227934e`,
  hidden static cgo linkage, API-35 x86_64 runtime acceptance, and arm64-v8a
  build-only status. Exact compile/owner/WAL/ABI/update/rollback policy lives in
  H0's outcome archive. H1 remains separately approval-gated.

**Outcome (2026-09-01).** Delivered in four separately committed slices:
`internal/application` with a typed error model and AST-enforced neutrality
guards; nineteen REST call sites migrated with byte-identical response parity
proven by test; the H0-pinned SQLite 3.53.4 amalgamation vendored with static
hidden linkage, zero exportable `sqlite3_*` symbols, and a provenance verifier
in the scaffold gate; and `cmd/notrioslib` exposing exactly the frozen twelve
symbols over the same facade, with generation-bearing handles, double-enforced
database ownership, and a C host acceptance test. REST behavior, business
semantics, schema, and platform-support claims are unchanged. `handleResourceContent`,
`searchMerged`, and the whole MCP adapter deliberately remain on the store, each
with its reason recorded; the Android emulator matrix stays with H11. Archived
as `plans/v0.8/003-shared-application-facade-abi-library.md`.

## H2a. Mermaid renderer and security investigation — complete

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
  for H2a; blocking for H2. Answered 2026-09-01:** Mermaid 11.17.2, pinned and
  locally bundled, with `securityLevel: 'strict'`, `htmlLabels: false`
  everywhere, lowered `maxEdges`/`maxTextSize`, post-render SVG sanitisation
  that allowlists the `notrios` scheme and neutralises remote hrefs, catch-all
  failure to visible fenced source, and dynamic import so it loads only when a
  diagram is present. H2a considered recommending that Mermaid stay disabled
  and did not.

**Outcome (2026-09-01).** Measured against the disabled baseline in Chromium
under the application's verbatim production CSP: **zero CSP violations and no
`unsafe-eval` needed**, because the two `new Function` sites in the dependency
tree are unreachable from Mermaid and do not survive the Vite bundle. Mermaid's
**default** configuration is not acceptable — a diagram label fetched a remote
image, and every diagram emitted the `foreignObject` the G18 contract asks to
refuse — but `htmlLabels: false` with strict security gave zero `foreignObject`,
zero cross-origin requests, and no script execution. Link navigation is inverted by
default: strict strips a `notrios://` note link and keeps a remote `https://`
one, because DOMPurify's URI allowlist admits no custom scheme, while `loose`
re-admits `javascript:` URLs. H2 must allowlist the `notrios` scheme and
neutralise remote hrefs, so a diagram reaches the user's own notes and a remote
URL is reached from the note that contains it. Cost
is roughly a doubled bundle (0.85 MB to about 1.75 MB gzipped), softened by code
splitting to 772 KiB for one flowchart. Enablement also needs three reviewed
licence-gate decisions, none a genuine licence problem. The 2000 ms render
deadline was **not** validated, because Mermaid's own `maxEdges` guard refuses
large graphs before layout; the Wails webview smoke, worker cancellation, and
accessibility review also remain undone and are recorded as such. Archived as
`plans/v0.8/004-mermaid-renderer-security-investigation.md`; Mermaid remains
disabled and no production file changed.

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

## H2b. Desktop external-link opening

**Goal.** Make a remote link in a note open the user's browser from the Wails
desktop window, as it already does in the loopback web UI.

**Why it sits here.** It was found while investigating H2a's link policy and is
that policy's dependency, so a reader meeting one should meet the other. It is
**not a Mermaid item**: it is a standalone desktop defect, it is approvable and
implementable without H2a or H2, and it must not wait on a Mermaid decision.

**Scope.** Intercept clicks on `http` and `https` anchors in the desktop shell
and route them through `window.runtime.BrowserOpenURL`. The preview already
routes `document://`, `resource://`, and `notrios://` through a `data-app-uri`
attribute and an onClick handler, so this extends an existing seam rather than
adding one. Keep the browser UI's `target="_blank"` path unchanged. Cover
`mailto:` as the same class of hand-off.

**Boundaries.** No change to what the sanitiser admits: this changes how an
already-permitted link is followed, never which links exist. No new outbound
request from the application itself — the browser makes the request, not
Notrios. No change to remote-media policy, image loading, or quarantine. No
Wails version change.

**Dependencies.** None. H2a recorded the finding; nothing blocks the fix.

**Working state.** Clicking a remote link in a note opens the system browser
from the desktop window and does nothing unexpected in a browser tab. The
window itself never navigates away from the application, which is the failure
this must not introduce: a webview that follows the link in place would replace
the running app with a website.

**Validation and evidence.** A GUI run confirming a click reaches the system
browser; a check that the webview did not navigate; a frontend test over the
click handler's routing decisions for remote, `mailto:`, in-app, and
unsupported schemes; and confirmation that the browser UI path is unchanged.
Record which desktop environment and WebKitGTK version the manual check ran on,
because the behaviour being fixed is webview-specific.

**Open decisions**

- **Should leaving the application be confirmed first? — Non-blocking; default
  is no prompt.** A remote link in a note is content the user wrote or imported,
  and the browser UI already follows it without asking, so a desktop-only
  prompt would be an inconsistency rather than a protection. The default is to
  open directly. If a confirmation is wanted later, the natural form is a
  preference, not a per-click dialog. Approving this item approves the default.

## H3. Installed-path, XDG, migration, and destructive-lifecycle investigation

**Goal.** Freeze a cross-platform installed-path contract and a fail-closed
end-user lifecycle before any runtime default or destructive Make target is
implemented.

**Scope.** Inventory every checkout-relative, executable-relative, profile, and
user-directory consumer. Define separate immutable program assets and mutable
config, data, state, cache, and runtime roots for Linux, Windows, and macOS.
On Linux, cover `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`,
`XDG_CACHE_HOME`, and `XDG_RUNTIME_DIR`, their standard fallbacks, absolute-path
requirement, and invocation-time overrides. Include the profile registry,
generated profile configs, databases/assets, projections, Recoll config/index,
quarantine, publication profiles, sync keys, catch-up/carrier/backup staging,
logs, snapshots, packaged web assets, deep-link registration, and paths named
inside profiles that live outside standard roots.

Design source, installed, and explicitly portable modes; first-run migration;
collision/interruption/rollback behavior; a machine-readable installed-artifact
manifest; GNU `prefix`, `exec_prefix`, `bindir`, `datarootdir`, `datadir`, and
`DESTDIR` staging semantics; and the complete `install`/`uninstall`/`purge`
contract. The purge design must inventory exact roots, stop or refuse live
processes, create and verify an owner-only restorable backup outside every
purge root by default, and delete only after backup success. It must separately
define config/data/state backup, cache/runtime disposal, symlinks, mount
boundaries, missing paths, external profile paths, and repeated/idempotent runs.

**Boundaries.** Investigation only: no path default changes, data movement,
Make lifecycle targets, installer dependency, Unix-domain-socket migration, or
deletion. `DESTDIR` stages immutable installed artifacts; it is not silently
prepended to a user's XDG runtime roots. Invalid, empty, relative, root, home,
or overly broad deletion targets must fail closed.

**Dependencies.** H0 may run in parallel. Consume the v0.7 profile,
backup/restore, secret-reference, evidence, and multi-instance contracts.

**Working state.** A source-audited per-OS path matrix and state machine specify
resolution precedence, ownership, permissions, migration, backup verification,
restore, uninstall preservation, purge confirmation, dry-run output, refusal,
and diagnostics. The report maps each existing path consumer to an owning H4
change or an explicit compatibility exception.

**Validation and evidence.** Disposable old/new layouts; exact path-resolution
tables; relative-XDG refusal/fallback; permission, disk-full, symlink, mount,
collision, and interrupted-backup models; external-profile fixtures; backup
restore proof; and a no-user-data-loss/no-broad-delete oracle.

**Open decisions**

- **Installed versus portable precedence — Non-blocking for H3; blocking for
  H4.** Recommended default: explicit CLI/config paths first, an explicit
  portable marker second, native installed locations otherwise; never infer
  portable mode from the current directory or its writability.
- **Default `make install` destination — Non-blocking for H3; blocking for
  H5.** Recommended default: an unprivileged user-local layout compatible with
  `$HOME/.local`, while retaining lowercase GNU directory-variable overrides
  and `DESTDIR` for package staging. Immutable assets follow install variables;
  mutable data follows native/XDG resolution at runtime and is not created by
  package staging.
- **Purge treatment of external profile paths — Non-blocking for H3; blocking
  for H5.** Recommended default: enumerate and back up eligible app-owned
  external paths but refuse to delete them automatically. A later explicit
  opt-in may be designed only with containment, ownership, and per-path
  confirmation; approval of H3 does not authorize deletion outside Notrios's
  standard roots.

## H4. Installed runtime paths, assets, and migration

**Goal.** Implement H3's accepted location and migration contract so binaries
operate outside a source checkout without changing explicit-path behavior.

**Scope.** Add one tested platform-path resolver for config/data/state/cache/
runtime and immutable assets; preserve explicit CLI/config precedence; add
installed/source/portable detection; resolve packaged `web/dist` and other
read-only assets; provide redacted path diagnostics; and implement explicit,
atomic, resumable migration from checkout-relative layouts with verified backup
and rollback. Update profile registry, generated profiles, sync keys,
publication profiles, Recoll, and transient service paths through the central
contract rather than scattered environment reads.

**Boundaries.** No implicit migration, current-directory portability guess,
unrestricted filesystem surface, credential-byte migration, automatic external
profile deletion, server transport change, package build, or destructive Make
target. Existing explicit absolute configs remain valid.

**Dependencies.** H1 and H3 complete; all H3 blocking decisions resolved in H3
and restated here.

**Working state.** A binary copied into an installed layout finds its immutable
UI/runtime assets, creates private native user roots on first use, reports
resolved paths without secrets, discovers isolated profiles, migrates only
after explicit review, survives interruption, and rolls back to a verified
restorable state.

**Validation and evidence.** Linux XDG override matrix in temporary roots;
Windows/macOS resolver unit fixtures; installed versus source/portable
precedence; clean start, migration, restart, interruption, collision,
permissions, missing/tampered assets, profile isolation, backup/restore, and
no-current-working-directory-dependency tests.

**Open decisions**

- **H3 path and migration selections — Blocking.** Do not start H4 until H3
  records the selected per-platform roots, precedence, ownership, migration
  trigger, backup format, external-path policy, and rollback.

## H5. Safe Make install, uninstall, and purge lifecycle

**Goal.** Provide end-user-location dogfooding targets that are auditable,
automation-safe, and unmistakably separate from development cleanup.

**Scope.** Add `.PHONY` `install`, `uninstall`, and `purge` targets and document
them beside `clean`/`clobber`. `install` deploys the GUI, daemon, CLI, immutable
web/assets, desktop metadata, and an ownership manifest to H3's user-local
layout, with GNU directory-variable and package-staging overrides.
`uninstall` reads the manifest and removes only artifacts installed by that
layout/version, leaving all user config, profiles, databases, assets, state,
backups, cache, runtime state, and external paths intact.

`purge` performs `uninstall` semantics plus H3's bounded mutable-state cleanup.
By default it shows the exact backup and deletion plan, requires an affirmative
interactive confirmation, creates an owner-only timestamped backup outside the
purge roots, verifies its manifest/hashes and offline restoration, and only
then deletes approved roots. `DRYRUN=1` is a non-interactive, zero-mutation
preview that lists exact installed files, mutable roots, exclusions, external
refusals, backup destination, and ordered actions for uninstall and purge;
GNU Make's `-n` alone is not sufficient evidence. `FORCE=1` suppresses the
prompt for headless automation but never bypasses validation or backup.
`NO_BACKUP=1` skips backup, emits a deep irreversible-loss warning naming
databases/assets/config/profiles/keys/state, and still prompts unless
`FORCE=1` is also set. Accept only the exact unset or `1` forms of these flags
and reject ambiguous values.

**Boundaries.** `clean` and `clobber` remain source-tree-only and never touch
installed/user data; lifecycle targets never clean the checkout. No raw
unresolved or broad recursive deletion, wildcard target, symlink traversal,
home/root deletion, credential disclosure, automatic external-path deletion,
package-manager database mutation, or network access. `FORCE=1` is not
`NO_BACKUP=1`. A non-interactive purge without `FORCE=1` fails closed rather
than hanging or reporting success.

**Dependencies.** H3 and H4 complete; H3 install-layout, backup, and external-
path decisions approved.

**Working state.** From a checkout, an isolated XDG user can install and use
Notrios outside the development tree, safely uninstall and reinstall without
data loss, preview every removal, purge with a verified backup, deliberately
skip backup only under the exact warning contract, and repeat every operation
idempotently.

**Validation and evidence.** Shell/Make fixture tests in disposable
`HOME`/XDG/`DESTDIR` roots; exact manifest inventory; install-use-uninstall-
reinstall; `DRYRUN=1` no-write/no-prompt proof; interactive accept/decline/EOF;
`FORCE=1` with and without `NO_BACKUP=1`; invalid flag values; failed/partial/
disk-full backup; offline restore; modified installed file; symlink/mount/
root/home/external-path refusal; concurrent process; repeated lifecycle; and
source-tree `clobber` separation.

**Open decisions**

- **Install-manifest ownership policy — Non-blocking default.** Default: remove
  only exact manifest-listed paths under validated install roots; preserve and
  report a user-modified or foreign-owned artifact rather than overwriting or
  deleting it silently.
- **Backup container and destination — Blocking before H5 implementation.**
  H3 must choose a restorable owner-only format and a default destination
  outside all purge roots, including behavior when the configured backup
  directory is unsafe, lacks capacity, or resolves through a symlink.

## H6a. Desktop installer and GitHub-native build investigation

**Goal.** Select the smallest maintainable installer toolchain and honest
support gates for Ubuntu, Windows, and macOS before adding package workflows.

**Scope.** Compare current Wails v2 native packaging, nFPM/GoReleaser, and
minimal platform-specific packaging for one GUI plus daemon/CLI. On Ubuntu,
prototype `.deb` contents, dependency declarations, desktop metadata, and
upgrade/remove behavior. For Windows, assess a native GitHub-hosted runner and
Wails/NSIS installer. For macOS, assess a native GitHub-hosted runner, `.app`
bundle and candidate `.dmg`/package creation. Record exact versions, licenses,
toolchain/runtime prerequisites, cgo/SQLite and WebView dependencies, output
reproducibility, unsigned-artifact behavior, secret/signing boundaries, GitHub
artifact retention, and runner cost. Verify current upstream documentation
rather than treating the references supplied with this plan as executable fact.

**Boundaries.** Investigation only. No production packaging dependency,
workflow push, installer upload, GitHub Release, signing/notarization secret,
support claim, Wails v3 adoption, or local attempt to emulate Apple hardware.
Cross-compilation or structural inspection is not native runtime evidence.

**Dependencies.** H3's layout contract; H4 may proceed after H3 while this
investigation runs.

**Working state.** A decision report selects Ubuntu packaging and either
selects a native-runner path for Windows/macOS or records a precise blocker and
postponement. It defines a four-level claim ladder: generated, structurally
inspected, natively installed/executed, and supported.

**Validation and evidence.** Exact primary-source/tool/license provenance;
minimal local Ubuntu package prototype; file/dependency/script inventory;
repeated-build comparison; candidate native-runner YAML validation; projected
minutes/storage; threat review for pull-request workflows and secrets; and
rollback/removal mapping to H5.

**Open decisions**

- **Package formats and orchestration — Non-blocking for H6a; blocking for
  H6/H7.** Starting candidates are Ubuntu `.deb`, Windows NSIS `.exe`, and a
  macOS `.app` inside an appropriate native distribution container. H6a may
  change or reject any candidate from evidence. Prefer platform-native Wails v2
  tooling plus a small Linux packager over adding a universal orchestrator
  unless one configuration demonstrably reduces risk.
- **Unsigned Windows/macOS artifacts in v0.8 — Non-blocking default.** They may
  be retained only as clearly labelled internal candidates after native
  execution. They are not end-user releases and do not imply v1.0 support.

## H6. Ubuntu-priority installer package

**Goal.** Produce and execute an internal Ubuntu installer that needs no source
checkout, Go, Node, npm, Wails CLI, compiler, or development headers at runtime.

**Scope.** Implement H6a's selected `.deb`-class package with the GUI, daemon,
CLI, immutable web assets, icon/desktop metadata, license/notices, runtime
dependency declarations, version metadata, and remove/upgrade scripts. Reuse
H4's path resolver and H5's preservation contract; do not create mutable user
data during package staging. An optional user service must remain disabled
until explicitly enabled and must not widen network access.

**Boundaries.** Internal prerelease only: no public upload, apt repository,
automatic service enablement, root-owned user data, signing claim, or broad
Linux support. Package removal leaves user data; destructive cleanup remains
the explicit H5 purge operation.

**Dependencies.** H4, H5, and H6a complete; H6a's Linux format/tool decision
resolved.

**Working state.** On a clean supported Ubuntu environment, a user installs the
package, launches the GUI and CLI without the repository or developer
toolchain, creates and reopens data, upgrades, removes, reinstalls, and retains
the data. Package inventory and runtime dependencies are exact.

**Validation and evidence.** Lint/package inspection; clean native install and
GUI/daemon/CLI smoke; offline post-download runtime; desktop entry/icon;
loopback-only listener; multi-profile; upgrade/downgrade refusal; package
remove/reinstall; H5 purge interoperability; dependency/license/SBOM inventory;
tamper and missing-runtime errors; and artifact hash.

**Open decisions**

- **Minimum Ubuntu versions/architectures — Non-blocking default.** Default:
  test the repository's current Ubuntu host plus the exact GitHub Ubuntu runner
  selected by H6a, initially amd64. Do not claim another release or
  architecture from build-only evidence.

## H7. Windows and macOS installer workflow implementation

**Goal.** Implement H6a-approved native GitHub workflows and package candidates
without pretending unavailable hardware was tested locally.

**Scope.** Add least-privilege, pinned GitHub Actions jobs on Windows and macOS
native hosted runners. Build GUI/daemon/CLI and immutable assets, produce the
selected installer/package, inspect contents, install into a disposable
account/context, launch native GUI plus CLI/daemon smoke, exercise user paths,
upgrade/remove/reinstall, verify data preservation, and upload only bounded
internal workflow artifacts. Keep platform-specific scripts small and share
closed manifest and behavioral assertions with Ubuntu.

**Boundaries.** Local implementation and static validation occur before any
push. Native execution waits for H12's delayed external step. No signing/
notarization secrets, release publication, mutable floating action pins,
workflow execution from untrusted fork code with write permissions, support
claim from compilation alone, or local macOS/Windows fabrication. If H6a
records an infeasible platform, close or replan this item for that platform
instead of creating a decorative installer.

**Dependencies.** H4-H6a complete; H12 supplies the first authorized remote
execution and result readback.

**Working state.** Locally, deterministic workflow/package definitions and
native test harnesses are ready and Ubuntu-equivalent assertions pass where
portable. After H12, each feasible platform has native result-bearing install
evidence; any postponed platform names its exact blocker, owner, and next gate.

**Validation and evidence.** Workflow policy/static checks; pinned action and
tool provenance; native build/install/launch/upgrade/remove/reinstall logs;
artifact inventories/hashes; no-source/no-toolchain runtime assertion; user-
data preservation; secret/log scan; cleanup/process audit; and honest support
matrix. H7 is not complete merely because YAML was committed.

**Open decisions**

- **Feasible platform set — Blocking.** Adopt H6a's per-platform decision.
  Windows/macOS may be postponed if the native build, runtime, packaging, or
  licensing gate cannot pass; Ubuntu priority is not permission to lower their
  evidence threshold.

## H8. Installed integration harness and Ubuntu baseline

**Goal.** Prove installed instances preserve profile isolation and native
integration, and create one reusable matrix for H12's Windows/macOS execution.

**Scope.** Exercise multiple profiles/servers, loopback port conflicts, URL
handler registration, shared-carrier paths, firewall behavior, file/directory
pickers, GUI/service modes, restart, package upgrade/removal, concurrent
isolation, and H5 lifecycle behavior. Execute all applicable rows on Ubuntu and
encode platform-specific Windows/macOS rows for native execution in H12.

**Boundaries.** No general remote API exposure, automatic firewall widening,
silent handler takeover, fabricated result for an unavailable OS, or support
claim from a skipped row. Native pickers return capabilities/selected paths
only to approved local operations.

**Dependencies.** H4-H7 local implementation; H1 for shared-core lifecycle
parity. H12 provides remote native rows.

**Working state.** Ubuntu has a complete result-bearing installed matrix.
Windows/macOS rows are either executable in H12 or explicitly postponed by
H6a/H7. Profile/database/replica/path/port isolation and deep-link ambiguity
refusal remain intact.

**Validation and evidence.** Multi-instance process tests, collision/fault
fixtures, handler install/remove/readback, shared-drive removal, picker cancel/
permission, firewall observation, desktop GUI smoke, lifecycle target parity,
no-development-toolchain assertion, and cleanup audit.

**Open decisions**

- **Minimum matrix for a support claim — Non-blocking default.** A platform is
  supported only after a native clean install, launch/use, upgrade, removal,
  reinstall, data-preservation, and cleanup pass. An unavailable row may close
  this evidence slice as postponed but blocks the platform support claim.

## H9. Native credential-store selection and integration

**Goal.** Replace the warned `0600` development secret file in supported
installed profiles with native credential-store providers while retaining an
explicit development fallback for source/test use.

**Scope.** Investigate then pin providers for supported Linux, Windows, and
macOS installations; define reference format, create/read/update/delete,
locked/unavailable/headless behavior, migration, backup exclusion, revocation,
purge interaction, and profile isolation. Evaluate Android only for H11
feasibility; do not use a Flutter-side store as the Go core's hidden owner.

**Boundaries.** Never log, export, commit, include in a purge backup, or place
secret bytes in evidence; never silently fall back from an installed native
store to plaintext; no credential-management REST/MCP surface.
`zalando/go-keyring` is a desktop candidate, not an assumed Android answer.

**Dependencies.** H1 provider interface and H3-H5 installed identity/lifecycle.

**Working state.** Supported installed profiles resolve opaque credential
references through the selected native store, fail closed when locked or
unavailable, and migrate only with explicit confirmation. Source/test mode
continues to label the owner-only file provider as development-only.

**Validation and evidence.** Exact dependencies/licenses; mocked contract
suite; native readback/delete/lock/session tests on available OSes; migration,
backup/purge, and refusal fixtures; log/repository/artifact scans for secret
material; and rollback.

**Open decisions**

- **Provider per supported OS — Blocking before implementation.** No provider
  is adopted until its availability, headless behavior, license, maintenance,
  packaging, backup/purge semantics, and rollback are recorded. Android may
  remain unresolved if H11 uses a test-only injected provider and makes no
  mobile-release claim.

## H10. Wails v3 migration spike

**Goal.** Determine whether Wails v3 can replace v2 later without risking the
v0.8 desktop product or installer schedule.

**Scope.** In an isolated prototype, compare dependency/license state, desktop
builds, bindings, menus/dialogs, web assets, deep links, lifecycle, multiple
profiles, installer packaging, and rollback. Record upstream status and mobile
limitations current at execution time.

**Boundaries.** Investigation only and separately approved. Do not migrate the
production shell, remove Wails v2, make H6/H7 depend on v3, or claim mobile
support.

**Dependencies.** H6-H8 provide the Wails v2 installed baseline; the spike may
be deferred if current upstream maturity makes it low value.

**Working state.** A disposable, reproducible prototype and decision report
recommend migrate later, defer, or reject. Production remains Wails v2.

**Validation and evidence.** Exact upstream/dependency provenance; build and
desktop regression matrix; package/RSS/startup comparison; native-integration
gaps; rollback rehearsal; and no-production-diff check.

**Open decisions**

- **When is the spike worth running? — Non-blocking default.** Run only after
  H8 establishes the v2 installed baseline and only on explicit user approval.
  If upstream maturity or required desktop features do not pass, recommend
  deferral without a migration item.

## H11. Android-emulator shared-core acceptance

**Goal.** Prove the H1 library is a viable backend on one Android emulator
without presenting an Android or Flutter product.

**Scope.** Package/load the selected ABI and SQLite owner; exercise instance
open/close, profile sandbox, SQLite bootstrap/reopen/reboot, CRUD/FTS5 search,
bounded resource stream, cancellation/polling, sync capability negotiation,
WAL/integrity, crash/restart, and desktop/emulator checkpoint interchange.

**Boundaries.** No physical device, iOS, UI, app-store artifact, background/
battery claim, production secure-store claim, Flutter client, or desktop
installer support inference. No second SQLite engine may open the canonical
file.

**Dependencies.** H0 decisions and H1 complete; H3/H4 path contract applied to
an emulator sandbox. H9 Android provider may remain a documented gap if
secrets are injected only by the test host.

**Working state.** The exact emulator/API/ABI loads the shared library and all
bounded lifecycle/storage/search/stream/cancel/sync probes pass or produce
typed failures. Unsupported ABIs/platforms remain explicit.

**Validation and evidence.** Clean/cold/reboot runs; ABI/symbol and package
inventory; SQLite version/options; FTS5/JSON/WAL/integrity; crash injection;
desktop round-trip; timing/RSS/package-size measurements; adb cleanup; and
leftover-process audit.

**Open decisions**

- **Runtime ABI beyond the H0 default — Non-blocking default.** Require the
  existing API-35 x86_64 runtime and an Android/arm64 build-only artifact.
  A physical/arm64 runtime remains post-1.0 unless separately authorized; this
  limits the support claim rather than weakening x86_64 acceptance.

## H12. Delayed GitHub native validation and develop-to-main pull request

**Goal.** At the latest practical point, obtain native Windows/macOS evidence
and place the complete v0.8 change set under review without bypassing the
`develop` workflow.

**Scope.** Immediately before the first external write, fetch and re-audit
remote `main`/`develop`; merge current `main` into local `develop` only if it
is no longer an ancestor, resolve and rerun all local gates, and run the
mandatory evidence pre-push verifier. Push `develop`, create a `develop` to
`main` pull request with `gh`, and read back the exact head/base hashes,
workflow permissions, checks, and artifacts. Execute H7/H8 native hosted-runner
jobs, retrieve bounded artifacts/evidence, and make only the minimum follow-up
commit/push needed to record reviewed results and correct defects.

The planning-time read-only audit on 2026-08-31 found remote
`main=265ef4ef84ea90f0e325522a3a4308a5804f122c`, remote
`develop=26b0925c21b3ecc264c370936e42d4b973548b8d`, and local
`develop=fd2192d1e830833fcf74b191bfc01851a61ed8bd`; remote `main` is an
ancestor of local `develop`, which is 133 commits ahead of remote `develop`.
This is evidence for why synchronization is needed, not permission to assume
the state remains unchanged at H12.

**Boundaries.** This is the first planned GitHub push for v0.8. No force-push,
direct `main` commit, tag, GitHub Release, installer publication, secret-bearing
artifact, mutable action pin, or PR merge. Do not expose reserve/private
evidence. Every external write and artifact remains attributable and read back.

**Dependencies.** All locally executable H0-H11 work complete or explicitly
closed/deferred; H7 workflow definitions and H8 Ubuntu baseline pass.

**Working state.** The PR contains the reviewed v0.8 work, all required local
and GitHub checks are result-bearing, feasible Windows/macOS installer rows are
closed with native evidence, postponed rows are honest, and no uncommitted
result exists only on a runner or workstation.

**Validation and evidence.** Fresh ancestry/divergence report; clean tree;
`scripts/verify_evidence_pre_push.sh` before each push; exact PR/base/head
readback; least-privilege workflow audit; native job logs/artifact hashes;
downloaded artifact verification; support-matrix reconciliation; and no
release/tag check.

**Open decisions**

- **Remote drift at H12 — Non-blocking default.** If `main` advanced, merge it
  into `develop` without rewriting published history and revalidate before the
  first push. If the merge changes an approved contract, stop and ask rather
  than resolving policy implicitly.
- **PR merge authorization — Blocking for H13's branch synchronization.** H12
  opens and validates the PR but does not merge it. The owner must separately
  authorize the merge after reviewing the final checks and support claims.

## H13. v0.8 release wrap-up and branch synchronization

**Goal.** Reconcile every approved v0.8 promise, produce internal installable
prerelease artifacts and a verified source snapshot, then synchronize
`main`/`develop` through the reviewed PR.

**Scope.** Run full repository, ABI, installed-path/migration, lifecycle,
package, native-integration, credential-store, Mermaid (if approved), and
Android-emulator gates. Reconcile product/version/schema/docs/API/dependency
licenses, security posture, backup/restore, upgrade/uninstall/purge, and
supported-platform claims. Archive the milestone and build the source ZIP via
the repository packager. After the H12 PR is green and merge is explicitly
authorized, make the final minimal `develop` push, merge through the PR, fetch
the result, update local `main`, bring the merge result back into `develop`
without rewriting history, push that synchronization if necessary, and verify
no content divergence.

**Boundaries.** No public GitHub Release, tag, signing/notarization claim,
app-store upload, physical mobile artifact, evidence reserve/ISO write, or burn
without separate authorization. Do not merge a failing/unreviewed PR or commit
directly to `main`. Fix only approved-contract defects; new features return to
planning.

**Dependencies.** H0-H12 as applicable; deferred H2/H7/H10 work must have an
explicit closed disposition. H12's merge authorization is resolved.

**Working state.** Product/docs/packages agree; every claimed platform has
native evidence; unsupported combinations are explicit; internal artifacts and
source snapshot verify; the v0.8 plan is archived; the merged `main` tree and
back-synchronized `develop` tree have no content difference; and the next plan
is derived from `ROADMAP.md` only after user review.

**Validation and evidence.** Full Go/frontend/docs/security/dependency gates;
ABI/header and emulator matrices; clean install/upgrade/rollback/uninstall/
purge with backup restore; native package inventories/hashes; source ZIP via
`scripts/package_release.sh` and `scripts/check_release_zip.py`; final PR/check/
merge/branch readback; release checklist; and handoff updates.

**Open decisions**

- **v0.8 product/schema number — Non-blocking until H13.** Default: product
  `0.8.0`; change schema only for a canonical migration actually required by an
  approved item. List every schema step rather than incrementing for packaging.
- **Internal artifact custody — Non-blocking default.** Retain only verified,
  non-secret prerelease artifacts in the local evidence directory and bounded
  GitHub workflow artifacts required for native review. No GitHub Release,
  reserve/ISO/media write, or installer publication is implied.
- **PR merge method and final synchronization — Blocking before merge.** The
  owner selects the permitted GitHub merge method. After merge, require
  `main` to be an ancestor of `develop` and a zero content diff; never force
  branches to identical commit IDs when the chosen merge method legitimately
  creates a merge commit.

## Decisions register

This is an index only; each decision is owned and explained inside its item.

| Decision | Owner | Status |
|---|---|---|
| Shared-core SQLite owner/version/checksum | H0/H1 | Resolved and implemented in H1: vendored amalgamation 3.53.4, static hidden linkage |
| Application facade package owner | H0/H1 | Resolved and implemented in H1: `internal/application` |
| Emulator ABI/minSdk acceptance | H0/H11 | Resolved for H1: API-35 x86_64 runtime; arm64-v8a build-only |
| Desktop external-link opening | H2b | Open with a no-prompt default; found by H2a and independent of it |
| Mermaid renderer/containment | H2a/H2 | Recommended by H2a: Mermaid 11.17.2, strict security, `htmlLabels: false`, `notrios`-only link allowlist; H2 approval also accepts the bundle cost and three licence-gate decisions |
| Installed/portable path precedence | H3/H4 | Open with explicit-override/native default |
| User-local/GNU install layout | H3/H5 | Open with `$HOME/.local` default |
| Purge external-path and backup policy | H3/H5 | Open; safe refusal and verified-backup defaults |
| Desktop package formats/toolchain | H6a/H6/H7 | Open; evidence-dependent |
| Windows/macOS feasibility and support | H6a/H7/H12 | Open; native execution required |
| Native credential providers | H9 | Open and blocking implementation |
| Wails v3 spike timing/outcome | H10 | Explicit approval required; production stays v2 |
| GitHub PR merge and branch synchronization | H12/H13 | PR planned late; merge separately authorized |
| v0.8 product/schema number | H13 | Open with product 0.8.0/no gratuitous schema default |
| Internal artifact custody | H13 | Local/internal-only default; public release separately authorized |

H1 and H2a completed on 2026-09-01. H2 is the next incomplete item and remains
unapproved. Do not begin it until the user explicitly says to proceed with H2.
