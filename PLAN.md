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

## H2. Bounded offline Mermaid enablement — complete

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

- **H2a renderer/containment recommendation — Blocking. Satisfied 2026-09-01:**
  H2a recommended enabling, and H2 implemented that recommendation unchanged.

**Outcome (2026-09-01).** `mermaid@11.17.2` is pinned and bundled; diagrams
render in a preview post-pass in `web/src/mermaid-render.ts` rather than through
`md-editor-rt`, which keeps `noMermaid: true`, so the fenced source is what is
already on the page and every failure leaves it there. Configured strictly with
HTML labels off and bounded at 20 diagrams, 65,536 source bytes, 500 edges and
2,000 ms; the output is sanitised regardless. A browser probe under the verbatim
production CSP records zero violations, zero cross-origin requests, no script
execution, and no `foreignObject`. Two defects were found and fixed: sanitiser
ordering left an empty `<image>` behind, and mermaid drops a `notrios://` href
before the sanitiser sees it while keeping a remote `https` one, so note links
are now reattached from the source and routed as `data-app-uri`. Mermaid cannot
render under jsdom, so the renderer is injectable and unit tests use a stub
rather than passing because everything fails. The three H2a licence decisions
are implemented and recorded, with the `khroma` exemption pinned to a re-checked
file hash. 211 frontend tests, bundle 0.85 to 1.68 MB gzipped. **No Wails
webview smoke, no worker cancellation, and no screen-reader assessment** — each
recorded. Archived as
`plans/v0.8/006-bounded-offline-mermaid-enablement.md`.

## H2b. Desktop external-link opening — complete

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
  **Taken 2026-09-01: no prompt.**

**Outcome (2026-09-01).** `web/src/desktop.ts` feature-detects
`window.runtime.BrowserOpenURL` and hands `http`, `https`, and `mailto:` links
to the system browser; `PreviewPane`'s existing click handler gained one branch
and suppresses the default only when the desktop shell took the link. The
browser-tab path is untouched. Twelve new frontend tests, 172 to 184 overall,
both guards proven to fail against deliberate regressions, and the Go leg
verified end to end by shadowing `xdg-open`. **No click in a running GUI was
observed** — driving one inside a WebKitGTK webview is not scriptable here, so
the chain is proven at both ends and read from source in the middle, and the
archive records a manual check to run on a machine with a display. Nothing was
widened: the same three schemes the sanitiser already permits, refused again by
`isExternalLink` and a third time by Wails. Archived as
`plans/v0.8/005-desktop-external-link-opening.md`.

## H3. Installed-path, XDG, migration, and destructive-lifecycle investigation — complete

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
  for H5. Resolved in H3 as recommended.** Enumerate and back up eligible
  app-owned external paths but refuse to delete them automatically. Fixtured in
  both directions: a target that is an external path, and a target that
  contains one. A later explicit opt-in may be designed only with containment,
  ownership, and per-path confirmation; approval of H3 does not authorize
  deletion outside Notrios's standard roots.

All three decisions above were resolved in H3 as recommended, and H4 and H5
must restate the ones that bind them. H5 additionally needs a backup container
and destination decision; H3 recommends one owner-only tar with a per-file
SHA-256 manifest, written under `$XDG_STATE_HOME` outside every purge root by
construction, refusing rather than relocating when that destination is unsafe.

**Outcome (2026-09-01).** Complete. Archived as
`plans/v0.8/007-installed-path-xdg-migration-purge-investigation.md`; evidence
under `performance/v0.8-h3/`. Twenty-five path consumers inventoried and
anchored to exact source substrings, a six-root per-OS contract proposed in
`LAYOUT.json`, a 14-scenario resolution table generated from an executable
resolver model, and a purge oracle with 30 fixtures run against a real
temporary filesystem. No path default changed, no data moved, no Make target
added, nothing deleted — `validate_evidence.py` asserts the `./data` defaults
are untouched. Four findings drive H4: two disagreeing config-root resolvers,
of which the hand-rolled one accepts a relative `XDG_CONFIG_HOME` that the
standard library refuses; profile data placed under the config root; config and
`web/dist` resolved from the working directory ahead of the executable; and
primary data roots created `0755` while every derived artifact is `0700`.

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

- **H3 path and migration selections — Blocking. Satisfied by H3.** H3 records
  the per-platform roots, precedence, ownership and permissions in
  `performance/v0.8-h3/LAYOUT.json`; the resolution rules and their generated
  table in `RESOLUTION_TABLE.json`; and the migration trigger, copy-verify-
  journal-commit sequence, per-category treatment, collision refusal and
  rollback in `REPORT.md` section 4. H4 must restate the precedence decision it
  is bound by, reproduce `RESOLUTION_TABLE.json` from its Go resolver, and
  revisit `PATH_CONSUMERS.json` for every consumer it changes — the inventory's
  source anchors are asserted, so a changed consumer fails
  `performance/v0.8-h3/validate_evidence.py` until the entry is updated.
  The purge backup container remains open and belongs to H5.

**Outcome (2026-09-02).** Complete, in five slices: A the resolver, B the
config-root consumers, C the data/state/cache/runtime roots, D instance
isolation, assets and diagnostics, E migration.

**Slice E is narrower than H3 section 4 specified, and the reason is slice D.**
H3 described an installed binary meeting a checkout-relative layout and
reconciling two instances. Slice D made a checkout a genuinely separate
instance, so that case no longer exists: a checkout's roots already are
`./data`, and migration concerns one instance moving rather than two merging.
Reading the surface again against the shipped code, three of the four
situations H3 worried about need nothing:

- a configuration that states a path keeps it, absolute or relative, because
  `applyResolvedRoots` fills only keys the file left unsaid;
- a binary run from a checkout is in source mode, where every mutable root
  resolves to `./data` as before;
- an installed binary meeting an old config file reads the old locations, which
  is the "no implicit migration" boundary already asserted in
  `internal/config`.

One case does move: a pre-0.8 binary run with **no configuration file** kept its
library in `./data` relative to whatever directory it was launched from. Nothing
recorded that directory, so nothing can look it up -- but it can be noticed when
the user is standing in it again, which is what `notriosctl paths`, `doctor` and
`notriosctl migrate` now do. H3's copy-verify-journal-commit-retire sequence,
free-space refusal, collision refusal and per-category treatment are implemented
as specified; only the trigger changed, from "an old layout exists" to "an old
layout exists here and is not the one in use".

The plan is generated from `config.ResolvedPathMappings()` rather than a list of
its own, so a path added to Notrios cannot be silently left behind by migration.
Migration copies bytes and never opens the source database: opening it would run
the schema migrations against the user's only copy before any copy of it exists,
which is the defect H4b exists to remove.

H3 named one further consumer as "the only one needing migration rather than a
new default": a generated profile keeping its database under the *config* root
at `~/.config/notrios/profiles/<id>/data/notes.sqlite`. It needs no migration
either, and the reason is structural rather than lucky. A generated profile
writes its own configuration file stating every path absolutely, and the
registry additionally requires `database_path` to be absolute and refuses a
relative one. Both are `provided` keys, so `applyResolvedRoots` leaves them
alone. Slice C changed the default for *new* profiles only. Verified against a
synthesised pre-slice-C profile: `notriosctl config show` reports every path
with origin `file`, unchanged.

Two defects were found and fixed while building it. Documentation still
described the pre-slice-D asset search order, so `docs/installation.md` told
installed users the working directory is searched when slice D had deliberately
stopped searching it. And the first detection implementation fired on a library
that was *in use* -- a configuration saying `directory: ./data` resolves against
the working directory -- which would have told a user their current library was
stranded and offered to move it out from under the configuration naming it.
`Detect` now takes the database the process would actually open.

## H4a. Distinct development and installed default ports in the documentation

**Goal.** Let a development checkout and an installed instance run at the same
time without either being reconfigured, and make the documentation say which
address belongs to which.

**Scope.** Change the checkout's `config/config.example.yaml` to a development
port (recommended `127.0.0.1:8099`, with `public_base_url` to match) and leave
the compiled default -- the installed instance's address -- at
`127.0.0.1:8080`. Then reconcile the documentation: `8080` appears 25 times
across 9 pages in `docs/`, and each mention has to be read to decide whether it
describes running from a checkout or an installed instance. Update the G18a
inventory, the G18f content hashes, and any pinned counts that move.

**Boundaries.** No change to the compiled default, the resolver, profile
creation, or the port-collision refusal. This is a documentation and example
change, not a behaviour change: a user who has set `server.listen_addr`
explicitly is unaffected.

**Dependencies.** H4 slice D complete. The instance isolation it introduced is
what makes two simultaneous instances safe at all -- their databases are already
separate, and this only removes the port clash.

**Working state.** `make serve` in a checkout and an installed `notriosd` run
concurrently with no flags and no edits. Every documented URL matches the
instance the surrounding prose is describing.

**Validation and evidence.** Both instances started together, each reachable on
its own address and reading its own database; `docgen`, `docaudit`, G18a and
G18f green; a page-by-page record of which mentions were changed and which were
deliberately left.

**Open decisions**

- **Development port number -- Non-blocking.** Recommended default `8099`:
  clearly not a default anyone would pick by accident, adjacent enough to 8080
  to read as related, and outside the range a user is likely to have taken.

**Outcome (2026-09-02).** Complete. Evidence under `performance/v0.8-h4a/`,
validated from `make validate`. A checkout defaults to `127.0.0.1:8099` and an
installed instance keeps the compiled `127.0.0.1:8080`; both were started
together and each answered on its own address from its own database.

**It was four functional changes, not the one the scope anticipated.** Three
couplings only surfaced on contact:

- `make serve` passed `-addr 127.0.0.1:8080` *explicitly*, so the target whose
  whole purpose is running a checkout was itself forcing the collision. The
  example config alone would not have fixed it.
- `web/vite.config.ts` proxies `/api` and `/healthz` to the service running from
  this checkout. Left at 8080, `npm run dev` would have proxied a developer's
  requests to whatever else held that port -- possibly an installed Notrios, and
  therefore a different library.
- The bind-failure message added in H4 slice D recommended
  `-addr 127.0.0.1:8099`, which after this change is the port a checkout expects
  to own. The advice would have created the collision it exists to resolve. It
  now recommends 8081 and says the two defaults no longer clash.

The documentation rule was: a mention changes when the surrounding prose tells
the reader to start or open a source checkout; it stays at 8080 when it states
the compiled default, describes an installed or remote instance, or is a docexec
substitution token inside an executed example. That last case is why
`docs/api/rest.md` keeps 55 literals -- docexec replaces the token with the live
fixture URL, so the literal is a placeholder rather than a claim -- while its
prose did change, having told the reader to start from the checkout's example
config and then curl 8080.

`TestTheExampleConfigAndTheCompiledDefaultUseDifferentPorts` guards the
invariant. Nothing else would: both values are valid addresses and every other
test passes with them equal, because the failure only appears when two instances
run at once.

**One limitation recorded rather than papered over.** An unrelated process on
the development machine already holds `*:8080`, so the installed instance could
not be exercised on its own default; 8081 stood in. That the port was taken by
something that is not Notrios is itself the argument for the change.

**Why this is separate from H4.** H4 slice D closed the data-safety half of the
problem -- a checkout can no longer open an installed instance's library -- and
that fix stands alone. The port clash that remains is a documentation project
with a one-line code change attached, and bundling 25 prose edits into a slice
about path resolution would have hidden the change that mattered. Until this
lands, the second instance to start fails to bind with a message naming the
likely cause and the flag to fix it.

## H4b. Verified backup before a startup schema migration

**Goal.** Make an automatic schema migration recoverable. Today `Bootstrap`
migrates a user's only copy in place, with no backup and no notice, on every
open by every binary.

**Scope.** Before applying any migration that would raise `user_version`, copy
the database and its `-wal`/`-shm` sidecars to a named location, verify the copy
by SHA-256, and only then migrate. On success, report where the backup is and
retain it under a stated policy. On failure, leave the original untouched and
name the backup in the error. Log one line at startup when a migration happens
at all -- a user whose schema was upgraded should not have to infer it.

The migration must hold the existing database owner lock for its whole
duration, so a second process cannot open the database mid-migration.

**This needs no new root, and that is the finding.** A schema migration touches
only the database file; assets, projections and the index are untouched, so the
backup is the `.sqlite` plus sidecars, not a whole-library image. It belongs
**beside the database**, under the data root, for a reason that rules the other
roots out: the copy must land on the same filesystem as its source. A user may
point `data.directory` at another disk, so `<state>` is not guaranteed to be the
same device -- which would make the free-space check meaningless, the copy able
to fail part-way across devices, and rollback a copy rather than a rename.
`<cache>` is excluded outright: purge disposes of it without backing it up.

What it *does* need is a **named, discoverable** location rather than a
temporary directory. Recovery depends on a user finding the backup after a crash
or a refusal, so the path must be stable, reported by `notriosctl paths`, and
named in any failure message. Recommended
`<database directory>/pre-migration-backups/<from>-to-<to>-<timestamp>/`.

**Boundaries.** No change to the migration SQL, the forward-only ordering, or
`CurrentSchemaVersion`. No new root, no configuration key beyond an optional
override for the backup location, and no interactive prompt: this runs at
startup and must stay non-interactive. Not a whole-library snapshot --
`notriosctl snapshot create` already exists for that and is heavier than this
needs.

**Dependencies.** H4 slices A-D complete. Independent of H4 slice E: that
relocates a library between roots, this protects an in-place schema change, and
the two share only the copy-verify-commit shape.

**Working state.** Opening a library whose schema is older produces a verified
backup, a migrated database, and a startup line naming both. A migration that
fails leaves the original database exactly as it was and names the backup. An
open with insufficient free space refuses before touching anything.

**Validation and evidence.** Old database migrated with the backup verified
byte-for-byte; failed migration leaves the original unchanged; insufficient
space refuses before the first statement; a second process is refused during
migration; repeated opens after a successful migration do not re-backup; backup
permissions are owner-only; the reported path exists and is what
`notriosctl paths` says; an interrupted migration is undone at the next start
and the marker cleared; a rollback whose backup no longer matches is refused
with both copies intact; and a `-wal` left by the failed attempt does not
survive the rollback.

**Open decisions**

- **Retention of the backup after success -- Non-blocking.** Recommended
  default: keep the most recent one and report it, delete older ones. Deleting
  immediately makes the safety net useless the moment a problem surfaces later
  than the migration; keeping every one grows without bound on a library that
  migrates often.
- **Behaviour when the backup cannot be made -- Blocking for implementation.**
  Recommended default: refuse to migrate and refuse to open, naming the reason.
  Migrating anyway would be the current behaviour with an extra log line, which
  is the thing this item exists to remove.

**Outcome (2026-09-02).** Complete. Before any migration that would raise
`user_version`, the database and its `-wal`/`-shm` sidecars are copied to
`<database directory>/pre-migration-backups/<from>-to-<to>-<timestamp>/`, each
file hashed and read back to confirm what is on the disk rather than what went
towards it, and a `MANIFEST.json` written. The service, `notriosctl` and
`doctor` each report a migration and name the backup. Retention keeps the newest
and removes the rest, after success only. A fresh database (version 0) is
skipped: there is nothing yet to lose, and backing one up would leave a
directory of nothing beside every new library.

**Two deviations from the scope, both deliberate.**

*The lock.* The scope says to hold "the existing database owner lock". There is
no such lock on this path: the `flock` owner lock lives in `internal/abi` and is
taken only by `cmd/notrioslib`, never by `notriosd`, `notriosctl` or the GUI.
Reusing it would have deadlocked the one caller that does hold it, because flock
claims belong to an open file description, so a second descriptor on the same
file in the same process conflicts with the first. A separate `<db>.migrating`
flock is held across backup and migration together, non-blocking because startup
must not hang. The version is re-read once the lock is held: a process that lost
the race would otherwise back up and migrate a database another process had
already finished with.

*Failure auto-restores at the next start, which is better than the scope asked
for.* The first implementation refused to auto-restore, reasoning that writing
over a database the process still holds open, unattended, is a worse risk than
the one it fixes. That reasoning was sound and the conclusion was wrong, because
it assumed the repair had to happen in the failing process. It does not.

A marker file is written beside the database before the first migration
statement and removed after the last, so finding one means a migration did not
finish. The **next** open restores the copy the marker names, before the
database is opened at all -- no handle is held, no write-ahead log is being
replayed -- then clears the marker and stops with a message naming the versions,
the backup and the export/import route if the upgrade keeps failing. It follows
the same shape as the existing physical-restore marker.

Three details make it safe rather than merely automatic: the copy's hashes are
checked against what was recorded when it was taken, and a mismatch refuses and
leaves both files; a `-wal` left by the failed attempt is removed, or SQLite
would replay the failed migration back over the restored database; and the
marker is deleted last, so an interruption mid-restore simply repeats it.

Commands named in recovery messages are guarded against going stale in three
directions: every registered command must appear in `notriosctl help`, every
registered command must appear in a message, and every `notriosctl ...` found in
a rendered message must be registered. The third is what stops a new command
escaping the guard entirely. A recovery message naming a renamed command is
worse than none: it is read when the user has least room to improvise.

`internal/paths.FreeBytes` now holds the free-space probe that H4 slice E had
introduced privately, because two callers needed it and a platform probe that
exists twice eventually disagrees with itself.

**Why this is separate.** It was found while answering whether a reinstall
migrates an end user's schema. It does -- and the same investigation found that
an *older* binary silently rewrote a newer database's version downward, which is
fixed. The remaining gap is that the forward path, which works correctly, works
on the user's only copy.

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
- **Backup container and destination — Resolved in H5.** H3 had already built
  and proven the container in `test_backup_restore.py`: one owner-only
  `backup.tar` plus a `MANIFEST.json` carrying a per-file SHA-256 inventory and
  a hash of the archive itself, created `0600` from the start rather than
  chmod-ed afterwards. H5 adopts it unchanged. The destination is
  `<state parent>/notrios-purge-backups/<timestamp>/` -- a sibling of the state
  root, so it is outside every root purge removes by construction, and each run
  asserts that the oracle itself refuses that path before writing anything. An
  unsafe destination is refused rather than relocated.

**Outcome (2026-09-02).** Complete, in four slices: A install and the ownership
manifest, B uninstall, C purge, D tests and documentation.

**A hardcoded program-assets root made the recommended install unusable, and
implementing H5 is what found it.** `internal/paths` returned
`/usr/local/share/notrios` for every installed Linux instance while H3
recommends `$HOME/.local` as the default prefix precisely because it needs no
`sudo`, so `make install` wrote the interface and help to `~/.local` and the
installed binary looked for them under `/usr/local`. It is now derived from the
executable, as Windows and macOS already were; the derivation subsumes the
constant, so `/usr/local/bin` still yields `/usr/local/share/notrios` and a
system-wide install is unaffected. H3's `resolve_model.py` was amended in step
and `RESOLUTION_TABLE.json` regenerated.

**Three defects were found by running the thing rather than reading it.**
Install copied a stale `bin/notriosctl` built before H4 slice D, so the
installed CLI had no `paths` command; install now depends on `build`. Uninstall
reported a symlink as "outside every install root", which is true because
`realpath` resolves it away, but sends the reader hunting for a prefix problem;
the symlink check now runs first and both rules still catch it. And purge asked
the *wrong instance* where the notes were: `make purge` runs from the checkout,
so the installed binary inherited the checkout as its working directory and
resolved source mode, reporting `data` and `web/dist`. The oracle refused them
for being relative and the backup destination would have landed in the
repository -- two rules caught it -- but a purge that asks the wrong instance
has already failed before anything protects it. Roots are now resolved from
outside the checkout.

**The backup container was world-readable.** `os.makedirs(mode=...)` applies its
mode to the last component only, so the parent took the umask while the archive
inside was correctly `0600`. Contents were never exposed, but a readable parent
publishes that a user has backups and when they were taken. Both levels are
owner-only now.

**H3's layout puts installed artifacts inside the user's data root** when the
recommended prefix is used: `$(datadir)/notrios` is `~/.local/share/notrios`,
which is also `$XDG_DATA_HOME/notrios`. The manifest removes the installed files
and the data step removes what is left, so nothing is missed or deleted twice,
and the purge plan names the overlap rather than printing two lines about one
path.

**Script tests were not being run at all.** `scripts/test_check_agent_usage.py`
was listed in `check_required_files.py`, which asserts a file exists rather than
that it passes. `make validate` now discovers and runs `scripts/test_*.py`,
which picks up those 28 tests as well as the 23 new lifecycle ones.

## H6a. Desktop installer and GitHub-native build investigation — complete

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

**Outcome (2026-09-02).** Complete. Evidence under `performance/v0.8-h6a/`,
validated from `make validate`. Investigation only: no packaging dependency was
added, no workflow pushed, no artifact uploaded, and the validator asserts the
Makefile and release script still contain no reference to a packaging tool.

**Everything follows from one measured fact: `CGO_ENABLED=0` does not build.**
Not "builds without SQLite" -- `internal/store` declares its exported types
inside the cgo file, so the package fails to typecheck. Any packaging option
therefore needs a C toolchain for every target it claims.

With only the host compiler GoReleaser built 1 of 6 targets; the rest died in
`runtime/cgo` (`gcc_arm64.S: no such instruction`). With `gcc-aarch64-linux-gnu`
installed and `CC` set, `linux/arm64` produced a genuine aarch64 ELF and an
`arm64` `.deb`, and the vendored amalgamation needed no extra flags. That binary
**cannot be executed on this machine**, so it stays at level 2. One Ubuntu
runner can therefore *produce* both architectures cheaply; only an arm64 machine
can say whether the result works, and the claim ladder keeps those apart.

**The prototype `.deb` builds and is not a policy-compliant package.** `lintian`
reports 3 errors and 297 warnings: no copyright file (for an Apache-2.0
project), no changelog, an empty extended description, and
`undeclared-elf-prerequisites` -- it declares **no `Depends` at all** while
linking `libc` and `libm`, because nFPM does no shared-library dependency
resolution. The control archive holds only `control` and `md5sums`, so nothing
registers the `notrios://` handler on install. Its layout does match H5's
program-assets derivation, so a distro package finds its interface and help for
free.

Two operational findings: builds are **not reproducible by default** --
GoReleaser injects `-X main.date=<timestamp>`, recorded in Go build info even
though `main.date` does not exist here -- and `goreleaser --clean` empties
`dist/`, which this repository already uses for source release ZIPs. It removed
them during the investigation; nothing was lost only because each ZIP had
already been copied to the evidence directory.

**Recommendation.** For Ubuntu, GoReleaser plus nFPM is a reasonable selection,
with the `.deb` treated as a starting point rather than a package; H6 should
consider generating it from the layout `scripts/lifecycle.py` already installs
and records, so the install contract has one definition rather than two.
For Windows and macOS **no toolchain can be selected from this machine**:
`makensis` and the Wails CLI are present and could generate a Windows installer
at level 1, and macOS cannot be assessed here at all. The decision is between
provisioning native runners and postponing both platforms, and this
investigation deliberately does not pretend to settle it.

**Not verified, recorded rather than assumed:** GitHub-hosted runner
availability, cost and artifact retention were not checked against upstream
documentation in this pass, so no projection is offered; whether the arm64
`.deb` installs or runs on an arm64 machine; and Wails v2 native packaging
output on either platform.

## H6. Ubuntu-priority installer package — complete

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

- **Minimum Ubuntu versions/architectures — Resolved in H6.** Only this build
  host and only amd64 are claimed. H6a's arm64 cross-build is available and is
  deliberately not packaged: it has never been executed, and packaging it would
  invite exactly the build-only claim this decision forbids.

**Outcome (2026-09-02).** Complete. `make deb` builds an internal Ubuntu
package; evidence under `performance/v0.8-h6/`, validated from `make validate`.

**The toolchain selection was narrowed from H6a's, on H6a's own evidence.**
H6a called GoReleaser plus nFPM reasonable and then measured four gaps -- no
declared dependencies, no maintainer scripts, no copyright, source file modes.
Every one of them is something nFPM does not do and dpkg tooling does, and
`dpkg-deb`/`dpkg-shlibdeps` are present on every Ubuntu build host, so this adds
no dependency at all -- which is what H6a's "smallest maintainable toolchain"
goal actually asked for. Returning to nFPM later is configuration, not a
rewrite. The reasoning is recorded in `scripts/build_deb.sh` itself.

**The package is built from `scripts/lifecycle.py`'s layout, staged through
`DESTDIR`,** so `make install` and the package cannot disagree about where
anything goes. It deliberately ships **no install manifest**: dpkg owns the file
list for a packaged install and records its own md5sums, and a second ownership
record in the same tree is the drift H6a warned about.

**lintian went from 3 errors and 297 warnings to 0 errors and 4 warnings.**
Dependencies are computed by `dpkg-shlibdeps` from the ELF rather than written
by hand, giving seven entries where nFPM declared none. `libsqlite3-0` appears
only through WebKit for the desktop binary; `notriosd` and `notriosctl` link no
system SQLite, so the vendored-amalgamation provenance is unaffected. The four
remaining warnings are recorded as accepted gaps with reasons rather than
silenced with overrides.

**It was installed and executed, not inspected.** Real `dpkg`, with maintainer
scripts, upgrade and removal, into a sandbox root rather than onto the
development machine -- so the package mechanics are genuine while the prefix is
not `/usr`, and the evidence says so. Verified: the CLI runs with no checkout or
toolchain; the service serves the packaged interface on loopback with HTTP 200;
removal leaves the library byte-identical; reinstall finds it at schema 27;
upgrade `0.7.0-1` to `0.7.0-2` in place keeps it byte-identical; the package
contains no user-root paths; and the user service ships installed and not
enabled, with no `enable` call in `postinst`.

That verification is only possible because of H5's program-assets fix: the
binaries resolve their assets relative to the executable, so a package installed
under a sandbox prefix finds its own interface. Under the previous hardcoded
`/usr/local/share/notrios` this could not have been checked without modifying
the machine.

**H5 purge interoperability was a real gap and is fixed.** Purge refused to run
against a packaged install because there was no manifest. The fix is not to ship
one -- that is the drift again -- but to do the half purge owns: it deletes the
data, leaves the program, and says the package manager owns it. Three tests
cover it.

## H7. Windows and macOS installer workflow implementation — deferred to post-v1.0

**Deferred (2026-09-02).** Moved to `ROADMAP.md` under "Post-v1.0 — Windows and
macOS installers". It cannot be implemented until Windows and Apple hardware, or
hosted runners standing in for them, are available.

This is a scheduling decision, not a reduction in scope: the requirements below
are unchanged and travel with it. H6a measured why the wait is unavoidable --
`CGO_ENABLED=0` does not build this project, so every target needs its own C
toolchain, and a cross-built artifact that has never run is not evidence that
the platform works. `makensis` and the Wails CLI are installed on this machine
and could generate a Windows installer today; it would sit at claim level 1 and
nothing here could execute it, which is the situation the original scope already
told us to postpone explicitly rather than paper over.

v0.8 therefore ships Ubuntu only, and claims nothing for Windows or macOS.


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

## H8. Installed integration harness and Ubuntu baseline — complete

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

**Dependencies.** H4-H6 local implementation; H1 for shared-core lifecycle
parity. H12 provides remote native rows. H7 is deferred to post-v1.0, so the
Windows and macOS rows are postponed rather than pending -- an empty row with a
recorded reason, not a gap waiting to be filled this milestone.

**Working state.** Ubuntu has a complete result-bearing installed matrix.
Windows/macOS rows are explicitly postponed with H6a's measurement as the
reason, and are not presented as pending work. Profile/database/replica/path/port isolation and deep-link ambiguity
refusal remain intact.

**Validation and evidence.** Multi-instance process tests, collision/fault
fixtures, handler install/remove/readback, shared-drive removal, picker cancel/
permission, firewall observation, desktop GUI smoke, lifecycle target parity,
no-development-toolchain assertion, and cleanup audit.

**Open decisions**

- **Minimum matrix for a support claim — Resolved in H8.** Ubuntu passes thirteen
  executed rows; Windows and macOS pass none and are recorded as postponed, so
  neither is claimed. `validate_evidence.py` enforces it: a row marked
  `passed` for a platform this harness cannot execute is rejected, and a
  postponed row without a reason is rejected as indistinguishable from a
  forgotten one.

**Outcome (2026-09-02).** Complete. `make integration-matrix` runs sixteen rows
against a *packaged* installation in a disposable HOME; evidence under
`performance/v0.8-h8/`, validated from `make validate`.

Thirteen rows execute and pass: the installed CLI runs with no Go, Node, npm or
compiler on PATH; the service binds loopback and never a wildcard; a second
instance on a taken port refuses, names the cause, and does not recommend the
development port; two profiles keep separate databases and a duplicate listen
address is refused; the `notrios://` entry declares the scheme and an icon; a
note created through REST is linked and resolved with the documented 0/1/2 exit
codes; restarting preserves the library; uninstall removes the program and
leaves the library byte-identical; the desktop binary serves under `xvfb`; and
purge removes the roots while its verified backup survives outside them; and
seeded help pages are found by search like any other note.

Five rows are defined and postponed, each naming what it needs rather than being
silently absent: firewall observation needs root and this harness runs
unprivileged; the native file picker is a dialog a person drives; package
upgrade and removal was executed in H6 rather than repeated; and Windows and
macOS need the hardware H7 is deferred for.

**Every failure this produced was in the harness, not the product.** Nine of
them: `ss` peer and local columns confused, a helper that waited for a socket
already held by the first instance, the desktop entry looked for under HOME
rather than datarootdir, a URI shape the documentation contradicts, waiting for
a service process to exit, a listing endpoint that does not exist, a JSON report
parsed as a line, a document search that returns nothing over a CLI-seeded
library, and a CSV column read in the wrong case. Each was read before it was
believed; none were reported as defects.

One observation is recorded rather than chased: running an installed binary with
the checkout as its working directory resolves *source* mode, which is correct
and documented, and is how a stray `profile register` landed in the checkout's
registry during debugging before being removed with `profile forget`.

**A second observation was withdrawn as a false alarm, and a test was added
because of it.** H8 first reported that `POST /api/v1/search` returned no hits
over a library seeded by `notriosctl seed-help`. It does not. The cause was the
same working-directory mistake as above: `seed-help` had been run from the
checkout, so an installed binary resolved source mode and wrote the help notes
into the *checkout's* library, while the service under test correctly searched
the sandbox's. Two databases, and search honestly reporting nothing about the
one it was asked about. On a clean packaged install the seeded pages are found --
`Recoll` 5 hits, `troubleshooting` 4, `notriosctl` 5.

The claim was wrong and the gap it pointed at was real: help is seeded into a
read-only notebook in the user's own database precisely so it can be searched
like any other note, and nothing tested that. `help-notes-searchable` now does,
and `service-cli-same-library` guards the mistake that produced the false
alarm: `GET /api/v1/status` reports the open database path and its logical
`database_id`, so a test never has to infer which library it is talking to. The
harness asserts it on *every* service start, not only in that row, because the
failure is silent and its symptoms point at the feature rather than the setup.
The matrix is thirteen executed rows. Confirmed by mutation -- searching
for a term that appears in no page fails the row.

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

**Candidate combination under consideration (2026-09-02).** `zalando/go-keyring`
for every desktop client -- Wails today, a Flutter desktop client later -- and
`flutter_secure_storage` on iOS and Android only, fetching the secret at boot and
passing it to the Go core in memory across the Dart FFI bridge.

**It covers the platforms, and it is admissible only in one shape.** This item
already forbids a Flutter-side store as the Go core's *hidden* owner, and the
mobile half of the proposal is a Flutter-side store. The distinction that makes
it acceptable is explicitness: the core keeps a provider interface in which
"supplied by the host" is a first-class implementation with its own contract,
rather than the core reaching for a secret it cannot see or reason about. The
core must still fail closed when the host supplies nothing, must never treat a
host-supplied secret as more trustworthy than one it fetched itself, and must
not acquire a compile-time dependency on any Flutter component. Written that
way, mobile is one provider among several and the boundary holds.

**Verified locally rather than taken from the references.** Modules resolved
through the Go proxy, licences read from the module cache:

| Module | Version | Licence | Notes |
|---|---|---|---|
| `zalando/go-keyring` | v0.2.8 | MIT | no cgo; Linux via D-Bus Secret Service, Windows via `wincred` |
| `99designs/keyring` | v1.2.2 | MIT | alternative with encrypted-file fallbacks |
| `billgraziano/dpapi` | v0.5.0 | MIT | Windows DPAPI only |
| `keybase/go-keychain` | v0.0.1 | -- | Apple only |
| `ella-to/vault` | v0.0.4 | MIT | exists; its mobile claims are unverified |

All are licence-compatible. `go-keyring` genuinely uses no cgo, which is worth
noting mostly because this project already requires cgo, so it adds no new
toolchain burden either way.

**Three findings the supplied references do not cover, and they shape the
work.**

*Headless Linux has no Secret Service.* `go-keyring` reaches
`org.freedesktop.secrets` over the session bus. A desktop session has one; a
server, a container, or an SSH session does not. H6 ships a **user service** that
can run in exactly those conditions, and this item's own boundary forbids
falling back to plaintext -- so such an installation simply cannot hold sync
credentials, and that has to be a stated outcome rather than a runtime surprise.

*Windows would split the two clients.* `flutter_secure_storage` uses DPAPI and
`go-keyring` uses Credential Manager. Both are encrypted at rest by the OS, and
they are different stores: a Flutter desktop client and a Wails client on the
same Windows machine would not see each other's secrets. The roadmap has both,
so this is a real incompatibility to decide rather than discover.

*The Linux provider is testable here, unlike H7's platforms.* This machine has
`gnome-keyring-daemon`, `kwalletd5`, and `org.freedesktop.secrets` on the
session bus, so the Linux provider can reach "natively installed and executed"
on H6a's ladder rather than stopping at inspection. The H8 matrix is the natural
home for the row.

**Not verified, and recorded as such.** `flutter_secure_storage`'s platform
claims; `ella-to/vault`'s mobile behaviour; and the claim that macOS Keychain
entries can be shared between a Flutter client and a Go client given the same
service name and app group. The first of those is now testable here rather than
unreachable -- see the toolchain note below -- so it stays on this list only
until someone runs it.

**Correction 2026-09-03.** The entry above previously justified the
`flutter_secure_storage` gap with "there is no Flutter toolchain on this
machine". That was wrong when it was written, and `FLUTTER_GO_CLIENT.md` --
item 1 on the `AGENTS.md` reading list -- already said so: its 2026-08-26
follow-up records Flutter Doctor passing every check, two working AVDs, and a
headless API-35 cold boot. Confirmed directly today: Flutter **3.44.9** stable
with Dart 3.12.2 at `~/flutter`, Android SDK **36.0.0** at `~/Android/Sdk` with
platform android-37.0 and all licences accepted, and Android Studio at
`/opt/android-studio` supplying the JBR. The practical consequence is that the
mobile half of the candidate combination stops being an argument from
documentation: `flutter_secure_storage` can be built, run on the existing API-35
emulator, and have its Android backup and migration behaviour observed, which is
the part this item called security-relevant. Nothing here has been run yet, and
this note claims availability only.

**`gopass` and `age` are now installed, so the headless tier is executable
here.** `gopass` **1.16.1** and `age` **1.1.1**, both at `/usr/bin`. On H6a's
ladder the headless provider can therefore reach "natively installed and
executed" rather than stopping at inspection, and the H8 matrix is the natural
home for the row, exactly as the Secret Service provider already is.

**Investigated 2026-09-02: `gopass`/`pass` for headless Linux.** The proposal was
to build a headless `secret-tool` equivalent on top of `gopass`, a GPG- or
age-encrypted file store backed by git, so that a server with no session bus can
still hold sync credentials. It is worth pursuing, but not in the shape the
supplied references describe, and not as a peer of the native stores.

*Mechanically it does clear the D-Bus obstacle.* `gopass` and `pass` are file
stores. They need no `org.freedesktop.secrets`, no session bus, and no desktop
session, so they work over SSH, in a container, and under the H6 user service --
exactly the conditions in which `go-keyring` cannot work at all.

*It does not make a headless install as safe as a desktop one, and the plan must
say so.* The secret is encrypted to a private key, and on an unattended server
that key must itself be openable without a human. Either it has no passphrase --
in which case the key is a `0600` file readable by the same account, and an
attacker who can read the credential file can read the key, which is the threat
model of the development file provider this item exists to replace -- or the
passphrase arrives at service start from a TPM-sealed systemd credential or an
operator, which does raise the bar but ends unattended startup. What `gopass`
buys unconditionally is protection against *offline* exposure: a stolen disk, a
backup, an accidental archive, a synced git remote. That is a real gain and a
different one from what a desktop keyring gives. So the honest outcome is a
third provider tier with a weaker, documented guarantee -- not a headless
equivalent of the native store, and never an automatic substitution, which would
be the silent downgrade this item's boundary forbids.

*Demonstrated on 2026-09-03 rather than argued, once `gopass` was installed.*
`gopass setup --crypto age --storage fs`, run in an isolated `GOPASS_HOMEDIR`
with `DBUS_SESSION_BUS_ADDRESS`, `DISPLAY`, and `XDG_RUNTIME_DIR` unset and no
tty, generates a passphrase-protected age identity by default -- the identities
file's own header reads `age-encryption.org/v1 -> scrypt` -- and then fails to
unlock it: `pinentry error: unexpected response: "S ERROR gnome3.isatty"`. So
gopass *out of the box* dies on a headless host at the same point `go-keyring`
does; the blocker is merely pinentry rather than D-Bus. The same environment
then round-tripped `age -e` and `age -d` successfully against a passphrase-less
`age-keygen` identity at mode `0600`. That is the whole finding in two commands:
the headless path exists, and its price is a plaintext private key readable by
the service account. The tier must therefore be described by what it defends
against -- offline copies -- and never as equivalent protection. Probe removed;
no secret material retained.

*Importing `gopass` as a library is strictly worse than the `go-keyring` it was
meant to avoid.* Measured here against v1.17.0 (MIT) by building a `main` whose
only statement is `api.New`: **24 modules, 108 non-stdlib packages, 18.6 MB**
against a 2.3 MB empty-Go-binary baseline -- a 16.3 MB addition, comparable to
the whole of the current 19.2 MB `notriosctl`. Six of those modules are already
in this project's 52-entry `go.sum`, so **18 are new**, against the 37 Go modules
G20 currently licence-audits. Among the six already present is
`godbus/dbus/v5`, which `gopass` needs at **v5.2.2** while this project carries
v5.1.0 -- so this route does not dodge the godbus bump recorded above, it forces
it -- and among the 18 new ones is `zalando/go-keyring` v0.2.8 itself. Licences
of all 24 were read from the module cache and are compatible, but
`hashicorp/golang-lru/v2` is **MPL-2.0**, a weak-copyleft class this project does
not otherwise carry, and `filippo.io/age` is BSD-3-Clause.

*Calling the binary instead costs no Go modules, and that generalises.* The
`secret-tool` finding already pointed at a provider that execs a helper; `pass`
and `gopass` fit the same shape, as does macOS `security`. One subprocess
provider contract therefore covers desktop Linux, headless Linux, and macOS, and
the difference between them becomes configuration rather than code. This is the
strongest argument yet for owning the provider layer rather than importing one.
It also inherits one obligation from the packaging finding below: a provider that
resolves its helper by name has to prove which program it found before trusting
it with a secret.

*The packaging cost is the same undeclared-dependency class H6a measured.* On
this machine `pass` 1.7.4-6 is installed only because `docker-desktop` depends
on it -- not manually, not by default -- exactly as `libsecret-tools` was
manually installed with no reverse dependencies. A `.deb` would need an explicit
`Depends:` that `dpkg-shlibdeps` cannot derive, because a helper process is not
a linked library.

*And on Debian and Ubuntu the name `gopass` does not identify a program.* The
archive's `gopass` 1.5.0 is `github.com/aviau/gopass`, a different project by a
different author; its own package description says "This package is not
gopass.pw (similar project with the same name)", and upstream's README warns
against installing it. Installing the real one made the collision visible in a
single command rather than theoretical: `apt-cache policy gopass` now lists
1.16.1 from `packages.gopass.pw` at pin priority 900 *and* Ubuntu's 1.5.0 at
500, two different upstreams answering to one package name, with only the pin
ordering separating them. So the references' `sudo apt install -y gopass` does not
install an older gopass, it installs an unrelated one -- and because both are
`pass`-compatible GPG stores their command surfaces overlap, so the wrong binary
would appear to work rather than fail cleanly. That is the worst failure shape
for a credential store, and it lands squarely on the subprocess provider, whose
whole premise is resolving a helper by name on `$PATH`. Consequences for the
design: the provider must never trust the name alone -- it takes a configured
absolute path, or verifies the binary's identity before first use, and refuses
rather than guesses; and installing the real gopass means adding a third-party
apt repository and signing key, which is a trust decision the Ubuntu archive
does not cover. This is the reason to prefer **`pass`** as the headless backend:
`pass` 1.7.4 is in the archive, is the reference implementation, and its name is
unambiguous. `age` 1.1.1 is likewise current in the archive.

*The GPG identity already on this machine cannot serve as the backend.* The
evidence-signing key is `sec#` -- the primary secret key is offline -- and it has
a signing subkey but no `[E]` encryption subkey, so it cannot decrypt. A
headless provider would need its own encryption key or age identity, which is
onboarding work rather than a detail.

*Three concrete errors in the supplied reference implementation, found by
compiling and running it.* First, it does not build: `gp.Set(ctx, path,
secretBytes)` passes a `[]byte` where the API takes a `gopass.Byter`, and
`[]byte` has no `Bytes()` method. Second -- and this one would have shipped --
`fmt.Fscan(os.Stdin, &secretBytes)` compiles and returns no error, but scanning
stops at the first whitespace: `"correct horse battery staple"` is stored as
`"correct"`, silently, and every later lookup succeeds and returns the truncated
value. Third, the walkthrough initialises with `--storage fs` and then claims the
git driver commits each change; `fs` is the non-git backend, `gitfs` is the git
one. The reference also pins `urfave/cli/v2` while `gopass` v1.17.0 itself uses
v3, so following it puts two major versions of one CLI library in a single
binary. Separately, the attribute-to-path scheme writes attribute names and
values as directory names, so a store pushed to a git remote publishes that
metadata in cleartext -- which for a notes application is a disclosure to weigh,
not a detail.

**Directive 2026-09-03: credential ownership is conditional on the platform,
and one method for all platforms is not viable.** The instruction is that
`Notrios` must not manage credentials on mobile at all. Instead the core exposes
a call through which the mobile frontend -- Flutter, or Tauri v2 -- supplies the
credential before the core needs it, and the core never reaches for a mobile
store itself. This is not a new provider so much as a promotion: the "supplied
by the host" implementation this item already admitted becomes the *only* mobile
shape rather than a tolerated one, and the reason is a build argument rather
than a security one. A Go package that carries iOS or Android implementations
drags Xcode and the NDK into the core's build matrix, and H7 is already deferred
past v1.0 precisely because that hardware is not available.

*Measured, and the real hazard is worse than the directive states.* The concern
is not only that a mobile-capable Go package would add toolchain dependencies.
It is that `zalando/go-keyring`, which supports no mobile platform whatsoever,
**compiles cleanly for them anyway**. Cross-built here with `CGO_ENABLED=0`
against v0.2.8:

| Target | Result | Why it matters |
|---|---|---|
| `linux/amd64` | builds | intended |
| `windows/amd64` | builds | intended |
| `darwin/arm64` | builds | intended |
| `android/arm64` | **builds** | `android` implies `linux`, so `keyring_unix.go` compiles and the binary reaches for `org.freedesktop.secrets` on a platform that has no D-Bus Secret Service |
| `js/wasm` | **builds** | falls through to `keyring_fallback.go` and returns `ErrUnsupportedPlatform` -- typed, but still only at runtime |
| `ios/arm64` | fails | only because cgo is disabled; that is a Go toolchain rule, not a `go-keyring` guard |

So the build emits no signal at all on Android, and by inference none on iOS
either, where `ios` implies `darwin` and `keyring_darwin.go` would compile and
then try to exec `/usr/bin/security`, which iOS neither ships nor permits. That
inference is not verified here and cannot be until H7's hardware exists. The
consequence for this item is direct: an accidental mobile build of the core
would satisfy every compile-time check and then fail exactly where "fail closed"
is least debuggable. The host-supplied architecture is therefore not merely
tidier, it is the only shape that can be *guarded*, and the guard has to be
explicit -- a build constraint that excludes every desktop provider from a
mobile build, and a cross-build gate that fails if one is linked in. Without
that, the architecture is a convention rather than a boundary.

*Four tiers, and each already has a different owner.*

- **Desktop** -- the core owns the credential through a native provider
  (Secret Service, Credential Manager, Keychain) and fails closed when it is
  locked or absent. Unchanged by this directive.
- **Headless** -- the core owns it through the opt-in `pass`/`age` tier
  recorded above, with its documented weaker guarantee.
- **Web** -- the browser never holds a sync credential; the service does, using
  whichever of the two tiers above its own host supports. This is already the
  shipped design and it is worth stating rather than rediscovering: the
  `sync-ui` routes are gated by `requireLocalSyncUI`, and `SyncLocalKeys` is
  commented "neither this interface nor any response can reveal one". Web needs
  no new surface, which is fortunate, because this item's boundary forbids the
  credential-management REST surface a browser would otherwise need.
- **Mobile** -- the host owns it. Flutter or Tauri v2 fetches from
  `flutter_secure_storage` or the Tauri equivalent and passes it in before first
  use; the core holds it in memory, treats it as no more trustworthy than one it
  fetched itself, and fails closed when the host supplies nothing.

*This dissolves the Windows question rather than answering it.* The third open
decision below asked whether a Flutter client and a Wails client on one Windows
machine must share a store, given that `flutter_secure_storage` uses DPAPI and
`go-keyring` uses Credential Manager. Under host-supplied credentials the
question does not arise: whichever client owns the secret supplies it, so the
two stores never need to agree. The directive's own framing covers the mobile
case -- a Flutter mobile client and a `go-keyring` desktop install are different
platforms and were never sharing anything -- and the same mechanism happens to
settle the Flutter-desktop case too, for the different reason that ownership
moves to the client. The decision is marked resolved on that basis.

*Tauri v2 is a genuine second candidate here, not a paper one.* Rust 1.97.0 and
cargo are installed on this machine, so a Tauri client could be built and its
mobile credential plugin exercised without new hardware. The `tauri` CLI is not
installed and none of its mobile claims -- including that its mobile support is
more established than Wails v3 -- has been checked. Recorded as a candidate to
verify, not a comparison already made.

*One consequence lands outside this item.* The credential-supply call is an ABI
operation, and `FLUTTER_GO_CLIENT.md`'s frozen G18 candidate is 12 symbols with
no such operation among them. Whether it becomes a thirteenth symbol or an
operation name inside the existing bounded-call dispatch is an H1 decision, but
H1 cannot be considered complete without it, and the frozen handoff contract is
not the place to change unilaterally.

**Where the architecture stands per platform, as of 2026-09-03.** Ownership is
settled everywhere; provider selection and verification are not.

| Platform | Who owns the credential | Settled? |
|---|---|---|
| Linux desktop | core, via Secret Service | ownership yes; provider blocked on the own-or-import decision |
| Linux headless | core, via the opt-in `pass`/`age` tier | shape yes, and executable here; shipping it is still an open recommendation |
| Windows desktop | core, via Credential Manager or DPAPI | ownership yes; which of the two follows from the own-or-import decision |
| macOS desktop | core, via Keychain | ownership yes; unverifiable here -- no Apple hardware, H7 deferred |
| Android | host supplies it | ownership yes, and the build guard is verified; the host-side provider is untested |
| iOS | host supplies it | ownership yes; the compile-time inference behind it is untested and stays that way until H7 |

*Three things remain genuinely open, and one of them is a gap rather than a
decision.* The two decisions are recorded below: whether to own the provider
layer or import one, and whether to ship the headless tier. The third is the
host-supplied contract, also below. The gap is this item's own Scope, which asks
for "locked/unavailable/headless behavior" across Linux, Windows and macOS,
while every headless finding recorded here is Linux-only. Headless is not a
Linux condition. A Windows service account has no interactive session and its
per-user Credential Manager store may not be reachable the way an interactive
logon's is; a macOS launch daemon running before any login faces a login
keychain that is still locked. Both mirror the Secret Service problem exactly,
neither has been examined, and neither can be examined on this machine. They are
recorded here so that "headless" is not quietly read as "Linux" when the Windows
and macOS providers are chosen.

**Adding headless Windows and headless macOS to the matrix raises four new
decisions, and they are not the Linux one repeated.** Two facts read from the
candidate sources on 2026-09-03 shape all four. `go-keyring`'s macOS path runs
`/usr/bin/security add-generic-password` with **no keychain argument**, so it
always lands in the default keychain -- the user's login keychain, which is
exactly the one still locked before anyone logs in; it cannot express
`/Library/Keychains/System.keychain`, which is the keychain a daemon could
actually use. And `wincred.NewGenericCredential` sets `Persist =
PersistLocalMachine`, so a Windows credential survives logoff and reboot rather
than evaporating with the logon session. Everything else below is documented
behaviour that cannot be tested on this machine, and is marked as such.

- *(The four decisions below are deferred to post-v1.0 with the headless
  provider, and are retained because the work will need them.)*
- **Is "headless" a provider tier or a provider parameter — Deferred.** On Linux headless needs a genuinely different provider,
  because Secret Service is absent. On Windows it probably needs none: under a
  service logon the DPAPI user keys are available and `PersistLocalMachine`
  keeps the credential across restarts, so the desktop provider should serve
  unchanged. On macOS it needs the same binary pointed at a different keychain
  -- which `go-keyring` cannot do. So modelling headless as one cross-platform
  tier is probably wrong; it is a distinct provider on one platform, a no-op on
  another, and a parameter on a third. Deciding this before writing the provider
  contract matters, because the tier shape leaks into the interface.
- **Whether machine-scope or root-scope storage is admissible at all —
  Deferred.** The Linux headless tier is user-scope: an `age` identity at `0600`.
  The macOS System keychain is root-scope, and a Windows service running as
  LocalSystem would be machine-scope. Both mean any administrator on the box
  reads the credential, which is a different threat model from "the service
  account reads it". A uniform user-scope-only rule is defensible and would
  forbid both, at the cost of constraining how the service may be installed.
  This constraint does not exist in the plan today.
- **Which Windows logon types count as supported headless — Deferred.** A service logon and a network logon are not the same condition:
  DPAPI user keys are available to the former and not, in general, to the latter,
  so a Notrios reached over SSH or WinRM may be unable to open a credential that
  the same account can open as a service. "Headless Windows" is therefore at
  least two matrix rows, and one of them may be unsupportable. Recommended:
  support the service logon, refuse with a typed reason on the other, and say so
  before enrolment rather than at first sync.
- **Whether the guarantee is stated per platform — Deferred for the headless
  case; for v0.8 it is one sentence about an interactive desktop session.** The three headless guarantees are
  genuinely different: a plaintext key at `0600` on Linux, real user-scope DPAPI
  on Windows under a service logon, root-scope on macOS if the System keychain is
  used. Headless Windows may well be *stronger* than headless Linux. One
  sentence saying "headless protection is weaker" would therefore be false on at
  least one platform, and this item forbids overclaiming in either direction.
  Recommended: `doctor` and enrolment name the actual scope per platform.

*Not a new decision.* Whether the matrix may carry rows that cannot be executed
here is settled precedent: H8's validator already asserts that no non-Linux row
claims execution, so these arrive as inspection-level rows under the existing
rule rather than as a new choice.

**Evaluated 2026-09-03: `99designs/keyring` v1.2.2 replaces `zalando/go-keyring`
as the desktop candidate, with one mandatory condition.** It is the better
choice, though not because it solves headless -- it does not -- and it carries a
dependency regression that has to be accepted knowingly.

*What it actually gives, measured.* Backends are `wincred`, `keychain`
(`darwin && cgo`), `secretservice`, `kwallet`, `keyctl` (all `linux`), `pass`
(`!windows`), `file`, and `array`. Three of those matter here. **`KeychainName`
lets it address a named macOS keychain** -- `keychain.go` sets `kc.path =
cfg.KeychainName + ".keychain"` -- where `zalando/go-keyring` passes no keychain
argument at all and is therefore stuck with the locked login keychain; that was
the macOS blocker in the headless decisions above. **`keyctl`** is a headless
Linux option needing neither D-Bus nor GPG. And a single interface with explicit
backend selection turns the "tier or parameter" question into configuration
rather than architecture. Licences are MIT except `godbus/dbus`, which is BSD;
all compatible.

*It does not dissolve the headless problem, and the plan should not say it
does.* The `file` backend takes a `FilePasswordFunc`; the supplied
`TerminalPrompt` needs a tty a headless service does not have, and
`FixedStringPrompt` means the application itself holds the passphrase. The
`pass` backend reaches GPG and lands on the passphrase-less-key result already
demonstrated with `age`. `keyctl` lives in the kernel keyring, and the source
notes possession is lost above the session keyring, so it does not survive a
reboot. The physics are unchanged: on an unattended host the unlock secret must
still come from somewhere, and every option is a variation on that one problem.
What changes is that the variations are now selectable behind one interface.

*The mandatory condition: `AllowedBackends` must be pinned, per platform, to
exactly one backend.* `Open` sets `cfg.AllowedBackends = AvailableBackends()`
when it is nil, then walks `backendOrder` -- wincred, keychain, secretservice,
kwallet, keyctl, pass, **file** -- and on any failure `continue`s to the next,
recording it only through `debugf`. So the library's default behaviour on a
Linux box with no Secret Service is to walk down and open a passphrase-protected
file instead, quietly. That is precisely this item's central boundary -- never
silently fall back from an installed native store to plaintext -- violated by the
default. Adoption therefore means one pinned backend per platform, a typed
refusal when it is unavailable, and a test that asserts the refusal rather than
a substitution. Pinned that way it is strictly better than the alternative;
unpinned it is worse than what is shipped today.

*The cost is a dependency regression, and it is real.* It requires
`github.com/godbus/dbus` at an unversioned 2019 pin -- not `/v5` -- while this
project already carries `godbus/dbus/v5 v5.1.0` through Wails, so the binary
would hold two D-Bus majors, one of them unmaintained for six years. It also
pins `gsterjov/go-libsecret` at a 2016 revision. `zalando/go-keyring` by
contrast uses a maintained `godbus/dbus/v5 v5.2.2`. Five new modules against
this project's 52-entry `go.sum`, so the size is not the issue; the staleness
is. Separately, the macOS Keychain backend is behind `darwin && cgo`, so a
`CGO_ENABLED=0` build drops it from `supportedBackends` silently; this project
uses cgo regardless, but the gate must assert the backend is present rather than
assume it.

*Consequences for the decisions above.* This narrows the own-or-import decision
towards importing, because a written-here provider layer would now have to
reimplement named-keychain selection, `keyctl`, and `pass` to match. It answers
the tier-or-parameter question as *parameter*, since backend choice is already
configuration. It leaves the machine-versus-user-scope and Windows-logon-type
decisions untouched, because those are properties of the platforms rather than
of any library.

**Tested 2026-09-03: vendoring `99designs/keyring` and owning the patches, and
whether a cleaner package exists.** Both questions were answered by building,
not by reading.

*The staleness is real and it is worse than "hasn't been updated in a while".*
Release dates from the proxy: `99designs/keyring` v1.2.2 **2022-12-19**;
its `godbus/dbus` pin **2019-07-26**; `gsterjov/go-libsecret` **2016-10-01**.
Against that, `zalando/go-keyring` v0.2.8 is **2026-03-23** and
`docker/docker-credential-helpers` v0.9.9 is **2026-08-26**, eight days old.

*The port is mechanical, which settles the vendoring question.* Copying
`99designs/keyring` and `go-libsecret` into one tree and rewriting the imports
from `github.com/godbus/dbus` to `github.com/godbus/dbus/v5` -- a `sed`, with
**no code changes at all** -- compiles clean for `linux/amd64`, `windows/amd64`
and `darwin/arm64`. The old D-Bus module disappears entirely, so the project
carries one godbus major rather than two, which was the principal objection to
adopting the library at all. The ported tree is **1,708 non-comment lines**
(1,407 keyring, 301 libsecret). The `go-libsecret` D-Bus surface is ten
identifiers, all unchanged in v5, which is why nothing needed rewriting.

*The strongest argument for owning it is not the dependencies -- it is the
fallback.* `Open`'s walk down `backendOrder`, continuing past each failure with
only a `debugf`, is not a bug to report upstream; it is a deliberate design that
is simply wrong for this item's policy. Vendoring lets that be replaced with
fail-closed selection rather than worked around by every caller, and a patch a
project intends to keep forever is exactly the kind that belongs in-tree. This
repository already vendors third-party source under a pinned checksum --
`internal/store/sqlite3_amalgamation.c` -- so the pattern and its gates exist.

*The costs of vendoring, stated plainly.* Owning the tree means owning CVE
response for 1,708 lines of credential-handling code with no upstream to
inherit fixes from; the vendored MIT source must be recorded in G20's licence
inventory and the copyright file; and the copy needs its own gate -- it builds
on the supported targets, and the fail-closed selection is asserted by a test
rather than assumed.

*No package avoids these problems, and the reason is structural.* Each candidate
trades one for another. `zalando/go-keyring` is maintained and pure Go, but
addresses no named keychain, offers no backend choice, and has no headless story.
`99designs/keyring` has the coverage -- named keychain, `keyctl`, `pass`, `file`
-- and is three and a half years stale. `docker/docker-credential-helpers` is the
freshest and leanest, requiring only a current `wincred` v1.2.3, `keybase/
go-keychain` and `x/sys`, but **both its `secretservice` and `osxkeychain`
backends are cgo**: the Linux one links `libsecret`, which adds an LGPL C library
and a build dependency this project does not have today, and it ships no `file`
or `keyctl` backend, so it has no headless answer beyond `pass`. Its shape is
also Docker's helper protocol rather than a library interface.

*And none of them solves headless, which is the through-line.* `gopass` did not,
`99designs/keyring` does not, and `docker-credential-helpers` does not, because
headless unlock is a property of the platforms rather than of any Go package. No
amount of library selection changes it; only the four decisions recorded above
do.

**Deferred 2026-09-03: the headless *provider* moves to post-v1.0; the headless
*refusal* does not.** Notrios is an end-user application driven by a UI, and a
remote server deployment is a future goal rather than a v0.8 one. Scoping
headless out is a scheduling decision and it unblocks nearly everything above.
Two things must be said about it plainly, because one of them is a trap.

*The condition can still arise on an ordinary desktop install, so detection
stays in scope.* H6 installs a systemd **user** service, `WantedBy=default.target`
and not enabled. If a user runs `loginctl enable-linger`, or logs in over SSH and
the service starts there, that user manager runs with no graphical session and
no unlocked keyring -- without anyone having deployed anything "headless". So
what is deferred is the provider that would work in that state. What is not
deferred, and is cheap, is noticing it: `doctor` reports whether a native store
is reachable, enrolment refuses with that reason rather than proceeding, and the
refusal is asserted by a test. Deferring the detection as well would leave the
shipped configuration with undefined behaviour in a state it can reach on its
own, which is this item's fail-closed boundary broken by omission rather than by
decision.

*Deferring headless reverses the library choice, which is worth stating because
it is not obvious.* The case for `99designs/keyring` rested entirely on headless:
its named-keychain support existed to escape the login keychain that is locked
*before login*, and `keyctl`, `pass` and `file` are headless backends. With a
logged-in UI user, the login keychain is the correct store and those backends
have no purpose. So `zalando/go-keyring` becomes the better answer again -- it is
maintained to 2026-03-23 against v1.2.2's 2022-12-19, is pure Go, needs no
vendoring and no CVE ownership, and costs one `godbus/dbus/v5` bump from v5.1.0
to v5.2.2 rather than a second D-Bus major. It is 623 lines to audit instead of
1,708 to own. The `99designs` evaluation above is kept because the deferral is a
scheduling decision, and the headless work will want it back.

*What this unblocks.* Provider per supported OS becomes decidable now: Secret
Service on Linux, Credential Manager on Windows, the login Keychain on macOS,
all through `go-keyring`, all user-scope, all interactive-session-only. The
own-or-import decision resolves to import. Vendoring becomes moot. Of the four
headless decisions, tier-or-parameter and machine-versus-user-scope and the
Windows logon-type question all become moot for v0.8, and the per-platform
guarantee statement collapses to one sentence about an interactive desktop
session. Only hardware-blocked verification remains, which H7 already governs.

*A correction this deferral forces.* The note above claimed H1 "cannot be
considered complete without" a credential-supply ABI operation. That was too
strong. The G18 ABI candidate is a bounded-call dispatch surface -- an operation
name plus a payload -- rather than one C function per route, so a credential
operation can be added later under a new operation name without breaking
compatibility. Since the Flutter client is post-1.0 and H11 injects test-only
secrets, the host-supplied contract can be designed when a real client exists.

**Slice A complete 2026-09-03: the provider seam, the native provider, and the
two guards.** `internal/credentials` holds a `Provider` interface -- name,
availability, get, set, delete over an opaque `Reference` -- with a native
implementation over `zalando/go-keyring`, a process-memory implementation for
tests and for a host that supplies a secret across the ABI, and a `Select` that
refuses rather than substitutes. `godbus/dbus/v5` moved v5.1.0 to v5.2.2 as
expected; the full suite and the Wails `gui` build are unchanged by it. G20 now
inventories 39 Go modules.

*A measurement decided the storage design before any code was written.* The
question was whether to put the sync key material in the native store directly.
Measured by growing a real `synckeys.KeyFile`: 262 bytes fresh, 1,640 at ten
peers, **3,010 at twenty**, 6,844 at forty peers and twenty epochs. Windows
Credential Manager caps a blob at 2,560 bytes, so the document crosses the limit
at roughly eighteen peers -- meaning the direct approach would pass every test
written today and fail in the field once a user had paired enough replicas. The
design is therefore envelope encryption: a fixed 32-byte data key in the native
store, the key file encrypted at rest beside the profile registry where H4's
backup and purge semantics already know to find it. `MaxSecretBytes` is set to
2,048 and refuses anything larger, so the constraint is enforced rather than
remembered.

*Both guards were verified by mutation, not by being written.* Removing
`!android` from the desktop provider's build constraint makes the cross-build
gate fail with `android/arm64 links zalando/go-keyring`, and making `Select`
return an unavailable provider instead of refusing makes the fail-closed test
fail. The refusal test runs against a stub rather than the real store, because
on any developer desktop the keyring always answers and the one branch this item
exists to guarantee would otherwise be exercised nowhere.

*Two corrections.* The cross-build gate covers **iOS as well as Android**: the
note above said `ios/arm64` could not be checked without cgo, which is true of
`go build` but not of `go list -deps`, and the mutated run reported
`ios/arm64 links zalando/go-keyring` alongside the Android row. And G20 recorded
`godbus/dbus/v5` as MIT; its licence file is **BSD-2-Clause** at both v5.1.0 and
v5.2.2. Both are in the allowed set, so nothing was admitted that should not
have been, but the entry was wrong and is now corrected.

**Slice B complete 2026-09-03: envelope encryption, wired but not yet
default.** `synckeys` gained a sealed form -- AES-256-GCM under a 32-byte data
key, a fresh nonce on every save, and authenticated data binding the ciphertext
to its own version and algorithm so an edited header cannot change the rules the
reader applies. `internal/service` gained `nativeSyncSecretStore`, which keeps
the data key in the operating system's store and the sealed material on disk,
and `sync.rest.credential_store` selects between it and the development file.

*The default is deliberately unchanged.* An installed profile still uses the
warned `0600` file, because switching an existing library to the keychain
without moving its material would strand it. The default flips in the migration
slice, not before, which is what keeps this slice a capability rather than a
break.

*Four behaviours are asserted rather than assumed.* A sealed file contains
neither the signing key, the group key nor the data key -- verbatim or
base64-encoded -- and is mode `0600`. A wrong data key, a short one, or a single
flipped ciphertext byte all produce `ErrSealMismatch` rather than altered
material. Sealed and plaintext files refuse each other by name, `ErrSealRequired`
and `ErrNotSealed`, because a migration that guessed would be a migration that
destroyed key material. And `Create` removes the sealed file it just wrote if
the data key cannot be stored, since the alternative is a file nothing can open
that makes every later `Create` refuse.

*Three of those were confirmed by mutation.* Skipping the seal in `save` makes
the no-key-material test report the signing and group keys appearing
base64-encoded; fixing the nonce instead of drawing a fresh one makes the nonce
test fail after a single save; and keeping the orphaned file instead of removing
it makes the cleanup test fail. The native path was exercised end to end against
this machine's real Secret Service, including reopening through a second
provider instance to prove the data key genuinely came back out of the keychain.

*The gates did their job on the way through.* Adding one config key moved
G18a's pinned key count 62 to 63 and `Default` 52 to 53, made `docs/service.md`
stale until regenerated, and then required the G18f generated hash to be
refreshed. All three are recorded here because a config key that could be added
without any of them firing would mean the doc gates had stopped watching.

**Slice C complete 2026-09-03: the refusal, and two divergences it exposed.**
`notriosctl doctor` now reports which store a profile uses and whether it can be
reached -- required for a configured native store, informational for the
development file, which it names as protected by permissions rather than a
keychain. `sync init` refuses before enrolling rather than after, and the
`sync-ui` surface carries the reason instead of a generic
`sync_setup_unavailable`, so the person at the enrolment screen learns that this
machine has no keyring rather than reading it in a service log.

*Writing that found the CLI and the service disagreeing about where the secret
lives -- twice.* Every `notriosctl sync` command resolved key material through
its own `keyPath`, which honoured the `--keys` flag and then the default path
and **ignored `sync.rest.key_file` entirely**, while the service honours it. A
profile that set it therefore had the command and the service reading different
files, both reporting success. Worse, the CLI called `synckeys.Open` and
`Create` directly, so a profile configured for the keychain would have had
`sync init` write a plaintext key file while the service expected a sealed one.
That is the same class as the defect the H8 matrix found -- a command and a
service addressing different libraries -- and slice B's new configuration is
what would have made it reachable. Both are fixed by routing every call site
through one `syncKeyStore` resolver, and `keyPath` is deleted rather than left
beside it, since a second copy of the resolution is how the first divergence
happened.

*And a third, found by a test that failed for the right reason.* The refusal
test passed a config naming an unknown store and the CLI enrolled anyway,
because configuration is parsed by an explicit key switch and slice B added the
struct field without the parser case. The key was unreachable from a config file
for one whole slice. It now parses and round-trips through the profile writer.

*Guards.* Dropping the pre-enrolment refusal makes the refusal test fail. A
refused enrolment is asserted to leave no key material behind. And an end-to-end
test drives the compiled CLI against this machine's real credential store,
reads the data key back out with a second process, and confirms it opens the
sealed file the CLI wrote -- the only test here that could not pass against an
in-memory stand-in.

**Slice D complete 2026-09-03: migration, in both directions.**
`notriosctl sync migrate-credentials --to native|development-file` moves
existing key material between the development file and the operating system's
store. It is a command rather than something a configuration change does,
because the failure is silent and permanent: key material cannot be regenerated
-- peers have already published artifacts the current group key decrypts -- so a
profile that switched stores by itself and then could not find its keys would be
indistinguishable from one that never had any.

*The ordering is the design.* The new copy is written to a staging path, then
reopened and checked -- signing key, private key bytes, group key, epoch and key
id -- and only then does the data key get stored and the file get renamed into
place. Every failure path leaves the material readable by the store it started
in: a staging write that fails removes the staging file, a data-key store that
fails removes it too, and a rename that fails deletes the data key that would
otherwise point at material the user does not have. `ReplaceMaterial` copies
retired epochs and paired peers along with the keys, because a migration that
dropped a peer would silently break verification of that peer's next artifact.

*Refusals.* `--confirm` is required and is a flag rather than a prompt, so the
same command works in a script and the record of what was agreed to is in the
user's shell history; `--dry-run` prints the plan and writes nothing. Migrating
to the store already in use is refused by name. An occupied data-key slot is
refused rather than overwritten, because the key already there opens material
somewhere and replacing it would strand that material forever.

*Verified by mutation, and the best result came from the product rather than the
test.* Making the migration mint fresh material instead of copying it did not
merely fail an assertion -- the command's own verification caught it, printed
"the migrated key material has a different signing key", and left the original
untouched. Removing the occupied-slot refusal makes that test fail. The round
trip runs against this machine's real credential store and back again, which is
also the rollback this item's validation asks for.

*Documentation, and what the gates asked for.* `docs/cli.md` gained "Where the
key material is kept", which explains that changing the setting does not move
the keys and why. That one section moved five pinned counts: G18a's manual
sections 215 to 216 and its inventory, the docaudit surface to 142 executables
and 296 unverified against a 382 denominator, G18d's registry to 142 entries and
79 unverified, and the G18f generated hash. The new example is registered
`shared-user-state` rather than `illustrative-placeholder`, because its commands
are literal and would write into the reader's own keychain; it is executed
against a sandboxed library by the Go tests instead, and the reason says so.

*Deliberately not done in this slice: the default is still the development
file.* Making an installed profile default to the native store is a behaviour
change for libraries that already exist, and the safe shape for it is not
obvious -- an installed profile holding a plaintext key file would either break
on upgrade or quietly keep using the weaker store, and this item's boundary has
something to say about the second. It is recorded as an open decision rather
than chosen here.

**The default flip, 2026-09-03, and the two defects it exposed.** The rule is
the one recommended: an explicit setting always wins; a library that already has
key material keeps the store that holds it; otherwise an installed profile gets
the keychain and a source checkout gets the development file. Enforcing it in
one function that all three callers share was the lesson from slice C, and it
paid twice.

*The credential reference was not unique per key file, and the flip made that
reachable.* The account was the database identity, which sounds like profile
isolation and is not: a second replica is made by adopting the first's database
identity, so two replicas of one library on one machine share a database id and
hold different key material in different files. With both defaulting to the
keychain, the second replica's enrolment overwrote the first's data key and
stranded a sealed file nothing could open. The sync tests failed with "sync key
file did not open with the supplied data key", which is exactly what that is.
`credentials.SyncReference` now folds a hash of the absolute key-file path into
the account.

*The default depended on the process rather than the library.* The first rule
asked whether *this process* was installed, and a service started from a
checkout and a command run from a sandbox resolve different modes while
addressing the same library -- so they disagreed about where its keys lived, and
the daemon refused to start against a file the CLI had just sealed. The rule now
asks the library first: sealed material means the keychain, plaintext material
means the file, and only a library with no material at all falls through to the
process's mode. That is both a fix and a better rule, because the library is
what the answer is actually about.

*A test of mine was writing into the developer's real keyring and leaving it
there.* The full suite left one entry behind on every run. It was
`TestCLIHonoursTheConfiguredKeyFile` from slice C, which configures a key file
and nothing else, so the new default sent it to the keychain. Bisecting found it;
it now pins the development file, as do the shared sync harnesses, which are
about the protocol rather than the store. The suite now leaves the keyring
exactly as it found it, and that is checked rather than assumed.

**Slice E complete 2026-09-03: the boundaries, enforced rather than stated.**

*Key material never reaches a purge backup, and that is checked three ways.*
The key file lives in the config root, whose policy is `backup_and_verify`, so
until now a purge copied a library's sync keys into an ordinary tar in a
directory chosen for convenience -- the password beside the lock. `lifecycle.py`
now filters it out of the archive and the inventory, names every excluded file
in the plan **before** the confirmation so a user who wants their sync identity
has a chance to copy it, records what was left out in the manifest, and says
that a data key in the operating system's store is not removed by purge. The
sealed form is excluded too: alone it is ciphertext, but the data key survives a
purge, so the pair would be recoverable. `verify_purge_backup` then checks the
archive itself, which is what makes this a boundary rather than an intention --
with the filter removed the purge **halts before deleting anything**, reporting
that the backup could not be verified.

*An existing test asserted the opposite and was reversed rather than deleted.*
`test_the_notes_can_be_restored_offline` required the sync keys to be in the
backup, which was right before this item and is exactly what its boundary
forbids. The assertion is inverted with the reason beside it, so the change is
visible to whoever reads it next.

*Nothing prints key material, and a scan says so rather than a review.* Six
commands -- `sync init`, `sync status`, `sync peers`, `doctor`, `paths
--no-redact`, `config show` -- are searched for three secrets in three
encodings, along with the sealed file itself. The risk is not that someone
deliberately prints a key; it is that a report gains a field. Adding a
`signing_key` entry to `Redacted()` is caught in two commands at once.

*Evidence: `performance/v0.8-h9/`, validated and wired into the scaffold.* 22
behaviours executed, 2 inspected, 12 mutations, 5 defects, 3 deferrals. Its
validator enforces this item's own boundary on itself -- any base64-shaped run
of 40 characters or more is refused, so the file cannot come to carry a key --
and refuses a behaviour that claims execution on Windows or macOS, since neither
has a host here. It also cross-checks the adopted module and its two companions
against G20's licence inventory, so this record cannot drift from what ships.
Both checks were confirmed by mutation.

**Open decisions**

- **Whether installed profiles default to the native store — Resolved and
  implemented 2026-09-03, as recommended.** An installed profile defaults to the
  native store; a source checkout keeps the development file, so a developer's
  throwaway libraries never reach their real keychain; and a library that
  already holds key material keeps whatever holds it, with `doctor`, `sync
  init`, `sync status`, the sync UI and one startup line all naming
  `migrate-credentials`. Implementing it exposed two real defects, recorded
  below. The original framing follows.
- **(superseded) Whether installed profiles default to the native store.** The capability, the refusal, and the migration
  all exist; what is unresolved is what an *existing* installed profile does on
  upgrade. Flipping the default strands a library whose key material is still in
  the development file, and keeping the old file when the default says native is
  the silent downgrade this item's boundary forbids. Recommended: the installed
  default becomes native for a profile with no key material yet, while a profile
  that already has a development file keeps using it and is told -- by `doctor`,
  by the sync UI, and once at startup -- to run `sync migrate-credentials`. That
  is not a silent fallback, because nothing changes underneath the user and
  every surface says what is happening; but it is a judgement about how loud is
  loud enough, and it is worth confirming rather than assuming.
- **Provider per supported OS — Discharged for desktop except backup and
  purge, which slice E owes.** The rule stands: no provider is adopted until
  its availability, headless behavior, license, maintenance, packaging,
  backup/purge semantics, and rollback are recorded. As of slice D most of that
  exists -- `zalando/go-keyring` is adopted and probed for availability, its
  licence and those of `danieljoos/wincred` and the `godbus/dbus/v5` bump are in
  G20's 39-module inventory, headless behaviour is a typed refusal, and rollback
  is `migrate-credentials --to development-file`. What is still missing is the
  backup and purge semantics of a sealed key file and a stored data key, which
  is slice E. Windows and macOS remain inspected rather than executed, which H7
  governs and this item cannot fix. The original rule text follows.
  No provider
  is adopted until its availability, headless behavior, license, maintenance,
  packaging, backup/purge semantics, and rollback are recorded. Android may
  remain unresolved if H11 uses a test-only injected provider and makes no
  mobile-release claim -- but as of 2026-09-03 that is a choice about scope
  rather than a limit of this machine, because the Flutter and Android
  toolchains and the API-35 emulator are all present and could answer the
  `flutter_secure_storage` questions directly.
- **Behaviour on a Linux install with no Secret Service — Resolved for
  desktop in slice C; the tier itself is deferred.** What was blocking here was
  telling the user before they enrol, and that shipped: `doctor` reports whether
  the configured store is reachable and fails the check when it is not, `sync
  init` refuses before enrolling with the reason, and the `sync-ui` surface
  carries the reason instead of a generic unavailability. The `godbus/dbus/v5`
  bump to v5.2.2 happened in slice A with the full suite and the Wails `gui`
  build unchanged. What remains is whether to ship the `pass`/`age` tier at all,
  and that moved to post-v1.0 with the rest of the headless work. The original
  analysis follows.
  Failing
  closed is required and is not the whole answer: the user needs to be told why
  before they enrol, not when a sync first runs. Recommended: `notriosctl
  doctor` reports whether a native store is reachable, enrolment refuses with
  that reason, and the documentation states the guarantee the installation
  actually has. `go-keyring` also needs `godbus/dbus/v5` v5.2.2 while this
  project already carries v5.1.0 indirectly through Wails, so the bump is part
  of the decision. The 2026-09-02 investigation above changes what the refusal
  can offer: a `pass`/`gopass` helper is a genuine third option rather than a
  dead end, so the decision is now whether to ship it. Recommended: ship it as
  an opt-in tier the operator selects explicitly, never as an automatic
  fallback; state in `doctor`, at enrolment, and in the documentation that its
  protection is against offline exposure and not against compromise of the
  account that runs the service; and refuse rather than downgrade when the
  operator has selected nothing. Prefer `pass` over `gopass` as the backend:
  `gopass` names two different programs on Debian and Ubuntu, and the helper
  must be pinned by absolute path or verified by identity in either case.
- **Whether Windows clients must share one store — Resolved 2026-09-03 by the
  platform-conditional directive.** DPAPI and Credential Manager are different
  stores, and under host-supplied credentials they never have to agree: the
  client that owns the secret supplies it. This lands where the earlier
  recommendation pointed anyway -- each client owns its own credential and
  re-enrols -- but by removing the question rather than choosing a side.
  Pairing is already per-replica, so this is also the stronger boundary.
- **Whether to own the provider layer or import one — Resolved for desktop
  2026-09-03: import.** Deferring headless removed every reason to write the
  layer here, since `keyctl`, `pass` and named-keychain selection were what a
  hand-written provider would have been for. `zalando/go-keyring` is adopted
  behind this item's own narrow interface, so the answer stays a provider swap
  if the headless work reopens it. The mobile half of the original argument --
  owning both ends on Windows to share a container with a Flutter client -- is
  moot under the platform-conditional directive, because the host supplies the
  credential there and the two stores never have to agree. The measurements that
  informed it follow.
  Measured, non-test, non-comment Go lines: `zalando/go-keyring`
  623, `danieljoos/wincred` 347, `godbus/dbus/v5` 6,338; importing `gopass`
  instead costs 18 new modules and 16.3 MB. A provider layer written here would exec
  `/usr/bin/security` on macOS, `secret-tool` or `pass`/`gopass` on Linux, and
  call DPAPI through `golang.org/x/sys/windows`, which this project already
  carries indirectly -- roughly 300 lines and no new Go modules. It also owns
  both ends on Windows, which is the only route to sharing a container with a
  Flutter client, since matching `flutter_secure_storage`'s DPAPI layout means
  binding to an undocumented implementation detail of a third party. The cost is
  that every Linux and macOS path becomes a runtime binary dependency the
  packaging gates cannot derive, and that the edge cases `go-keyring` has
  absorbed become ours. Recommended: keep the H1 provider interface narrow
  enough -- get, set, delete over an opaque reference -- that either answer is a
  provider swap rather than a redesign, and decide only when H10 and H11 have
  said which clients are real. The headless tier lands in the owned layer either
  way, because no library provides it.
- **Vendor `99designs/keyring` or depend on it — Moot for v0.8, reopened with
  the headless work.** With headless deferred, `zalando/go-keyring` is adopted
  and nothing is vendored. The analysis below stands for when headless returns.
  Superseded reasoning, kept deliberately: The port to `godbus/v5` is proven mechanical, the tree is
  1,708 lines, the repository already vendors pinned third-party source, and the
  fail-closed selection this item requires is a permanent divergence from
  upstream's design rather than a fix upstream would accept. Against that,
  vendoring transfers CVE response for credential-handling code onto this
  project. Recommended: vendor, with a pinned upstream revision recorded the way
  the SQLite amalgamation is, a licence-inventory entry, and a gate asserting
  both that it builds and that an unavailable backend refuses rather than
  substitutes.
- **The host-supplied provider contract, and the guard that keeps it honest —
  Blocking before any mobile work.** The contract needs: when the host may
  supply a credential and what happens to calls that arrive before it does;
  whether a supplied credential can be replaced or revoked mid-session; its
  lifetime in core memory and where it is zeroed; that it is never written to
  the core's own store, a log, a purge backup, or evidence; and that supplying
  nothing is a typed refusal rather than a fallback. Recommended: model it on
  the existing `SyncSecretStore` seam, whose `AdoptGroupKey(keyID, epoch, key)`
  is already an inbound key path, so the host provider becomes another
  implementation rather than a parallel mechanism. The guard is the part that
  must not be deferred: a build constraint excluding every desktop provider from
  a mobile build, plus a cross-build gate that fails if one links in. The
  `android/arm64` result above is the reason -- without the gate, the mistake
  this architecture exists to prevent compiles silently.

  *Verified 2026-09-03 that conditional linking does work, and that the obvious
  way to write it does not.* Tagging the desktop provider `//go:build linux ||
  windows || darwin` is wrong: on `android/arm64` that file compiles **as well
  as** the mobile one, because `android` implies `linux`. In the probe a
  duplicate `newProvider` declaration caught it, which is luck -- a provider
  selected any other way would have linked `go-keyring` into the Android binary
  silently. The constraint that holds is:

  ```go
  //go:build (linux || windows || darwin) && !android && !ios   // desktop provider
  //go:build android || ios || js                               // host-supplied provider
  ```

  With those, all five targets build, and the linked package set is exactly
  right: `linux/amd64` links 3 keyring/D-Bus packages, `windows/amd64` 1,
  `darwin/arm64` 2, while `android/arm64` and `js/wasm` link **0** and the
  Android binary contains **0** `zalando` symbols by `go tool nm`. The
  dependency still appears in `go.mod` -- `go mod tidy` considers every build
  configuration -- so it stays inside G20's licence inventory, but it is never
  compiled for a mobile target and so drags in no NDK or Xcode requirement,
  which is the whole point of the directive. The gate is therefore cheap and
  needs no emulator: for each mobile target, assert that
  `GOOS=... go list -deps ./...` names no desktop credential package.

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
workflow permissions, checks, and artifacts. Execute H8 native hosted-runner
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
closed/deferred; H8 Ubuntu baseline passes. H7 is deferred to post-v1.0 and
contributes no workflow definitions to this milestone.

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

## H14. Documentation actionability investigation (opencode, free models, zvec-grep)

**Ordering.** Independent of the H5-H11 installer chain and can run at any
point. If it succeeds and the follow-up evaluation is approved, that evaluation
should land before H12, because documentation quality is part of what a release
claims.

**Goal.** Decide whether hosted free models driven by `opencode`, with local
semantic search from `zg` (zvec-grep), can answer a question the repository
currently cannot: **can a reader act on this page?** Concretely -- given only the
prose for one task, can a model produce a command line that actually runs and
does the thing? A page whose reader cannot take an action is a page that needs
work, however fresh, hashed and internally consistent it is.

**Why the existing machinery does not answer it.** Every documentation gate in
this repository checks *consistency*: docgen regenerates fragments, docaudit
anchors sections to source, G18a freezes an inventory, G18f hashes content and
counts enumerations, G18d executes registered examples. All of them can be green
while a page fails its reader, and that is not hypothetical -- H4 slice D found
`docs/installation.md` documenting a superseded asset search order, and slice E
found it again on another page, both with every gate passing. The gates check
that documentation is generated consistently and hashed, not that it is still
true or that anyone can use it.

**The known defect this must find, verified by hand.** Ask the documentation how
to create, edit or delete a tag on a note:

- `docs/cli.md` documents exactly one tag-mutating command, `notriosctl tags
  rename`. Tags otherwise appear only as filters (`--tags a,b`, `tag:todo`).
- `docs/gui.md` shows tags only as sidebar navigation with note counts and
  "click anything to search it".
- Yet the capability exists: REST has `POST` and `DELETE
  /api/v1/documents/{document_id}/tags/{tag}`, and MCP has `tag_note` and
  `untag_note` in the editor scope.

So a reader of either the CLI or the GUI guide cannot learn to tag a note, and
neither page points at the surface that can. This is a ready-made positive
control: a method that cannot find it is not worth adopting. It also raises a
separate product question recorded below -- whether the CLI is *meant* to have no
add/remove-tag command.

**Scope, in two parts.** *First*, write the documentation topics the repository
does not have: a features page answering "what can I do with this?", and from it
a catalogue of user journeys for the command line and then for the GUI, the GUI
ones illustrated with annotated screenshots. *Second*, stand up `opencode` and
`zg` locally; index the repository with a **local** embedding; reuse the
existing 8-case, 16-run contradiction calibration in
`performance/v0.7-g18f/ADVISORY_REPORT.json` to score candidate free models
against the recorded Qwen 2.5 Coder 1.5B baseline of 7/16; then build a small
task-to-command harness and measure whether generated command lines run in a
sandbox. The evaluation runs over the new pages as well as the existing ones,
which is the point of the ordering: a quality pass over documentation that does
not yet exist measures nothing.

*This reverses one of the item's own boundaries and that is deliberate.* H14
previously said "change no prose", because an investigation that rewrites what
it is measuring cannot report on it. That still holds for the evaluation: it
proposes, and a person decides. It does not hold for the generation, which is
now the first half of the item. The two are kept apart -- new prose is written
and reviewed before any model scores it, and no model output is committed as
documentation.

**The experiment must control for the model's own knowledge, and this is the
design point everything else depends on.** A capable model can produce a
plausible `notriosctl` invocation from familiarity with command-line conventions
alone, never having read the page. Scoring "did the command work?" would then
measure the model and report it as documentation quality. Every task therefore
needs three arms:

1. **prose-only** -- the page section and the task name;
2. **no-prose** -- the task name alone;
3. **misleading-prose** -- the section with one detail mutated, such as a renamed
   flag or an inverted default.

A page earns credit only when arm 1 succeeds *and* arm 2 fails. Arm 3 catches a
model that is ignoring the text it was given. Without arm 2 the whole exercise is
unfalsifiable.

**Why an executed command beats a label.** G18f's advisory asks a model to
choose among `supported`, `contradicted` and `not-determinable`, and the
recorded run scored 7/16 with both negation cases wrong -- the failure the
proposal names, where prose and its negation sit close together in the vector
space. An executed command sidesteps that entirely: nothing has to distinguish
"do X" from "do not do X" in an embedding, because the command either does X or
it does not. Scoring becomes deterministic, which is also what makes cheap models
usable -- they are being asked to draft, not to judge.

**Exit status is not the oracle; the observed state change is.** A command can
exit zero having done nothing, and -- the case that matters -- it can exit zero
having done the *opposite* of what the page described. Restoring a note and
purging it are both successful commands. So every task declares the state it
expects, and the sandbox database is inspected before and after, giving three
outcomes rather than two:

| Outcome | What it says about the page |
|---|---|
| the intended change happened | the prose is actionable |
| nothing happened, or the command failed | the prose is unclear or incomplete |
| the opposite or another destructive change happened | the prose actively misleads |

The third is the most valuable result and the one a pass/fail oracle would
record as a plain failure, indistinguishable from a typo. It is also where the
negation weakness resurfaces on the *documentation* side rather than the model
side: a page that reads as "notes in Trash are removed after 30 days" and a page
that reads as "notes in Trash are removed immediately" produce different commands
with different observable effects, and only the state check tells them apart.
`internal/docexec` already models this -- its registered examples carry a
`postcondition` describing what must be true afterwards, not merely an expected
exit status -- so the shape exists and needs reusing rather than inventing.

**Reuse the sandbox rather than build a second one.** `internal/docexec` already
runs 63 of 137 registered examples against a seeded loopback fixture with
substitutions for the base URL, binaries, seeded ids and scratch directories. The
difference here is only the source of the command: docexec runs commands
*transcribed from* the docs, this runs commands *synthesised from* the prose. The
gap between those two is exactly the thing being measured, so the fixture,
adapters and substitution machinery should be shared.

**The features page should be half generated, and the generated half is what
makes it trustworthy.** A features list is usually written by hand and quietly
goes stale. This repository already carries four anchored, counted surface
registries -- 59 CLI usage forms behind `printHelp`, 63 configuration keys, 109
REST operations and 46 MCP tools -- each with an owner anchor and a pinned count
that fails when it moves. So the inventory of *what exists* can be generated the
way every other fragment is, and only the description of *what it is for* is
written by a person.

That split buys a check nothing currently performs: **every anchored surface
must be claimed by at least one feature entry, and an unclaimed surface fails.**
A capability that no feature names is by definition a capability no reader can
discover, which is precisely the tags defect recorded above -- REST and MCP can
tag a note, and neither the CLI guide nor the GUI guide says so. Today that was
found by hand. With a coverage gate it would have been found by the build. This
is the most valuable thing in the item and it does not need a model at all.

*Flags are not features, and the mapping is the editorial work.* Reading
`cmd/notriosctl` yields switches, not answers to "what can I do with this?".
`--materialize N` is a flag; "sync two of your own libraries through a folder
you both can reach" is a feature. The generator produces the surface inventory
and the coverage obligation; a person writes the capability prose against it.
Any attempt to generate the prose from flag names would produce a second copy of
the reference documentation and call it a features page.

**Journeys: the command line first, and it is the specification.** The
instruction to start with the command line is right for a reason worth
recording: a CLI journey can be *executed*, deterministically, against a seeded
library, and `internal/docexec` already does exactly that for 63 registered
examples with postconditions describing what must be true afterwards. A GUI
journey needs a browser and is slower, flakier and harder to assert. So the CLI
journey is written and executed first, and it becomes the statement of what the
task *is*; the GUI journey is then checked against it rather than invented
beside it.

The starting catalogue, which is a floor rather than a ceiling: create, update
and delete a note in a named notebook; search, demonstrating every query-language
feature with a worked example; notebooks defined by a query; import from Joplin;
import from Obsidian; export the library; create a profile such as
`personal_notes` or `work_notes`; synchronize with a replica on another drive,
including a cloud folder mapped locally; back up and restore the library; and
how Recoll is used. Tagging a note is deliberately on the list too, because at
the time of writing the command line cannot do it -- the journey is the thing
that makes that visible instead of arguable.

**Comparing the two catalogues is a defect finder, not a formatting exercise.**
Each GUI journey is compared to its command-line counterpart, and the comparison
has three possible outcomes, all of which are findings: the GUI can do something
the CLI cannot, the CLI can do something the GUI cannot, or the two do the same
thing by different names. The tags case is already a worked example of the
second, and it generalises -- this comparison is the systematic version of the
hand-found positive control. Every difference is recorded as either a documented
deliberate asymmetry or a product gap, and the investigation does not decide
which; it presents them.

**Screenshots: reuse the runner, and derive the marker from the click.** The
machinery is largely present. `performance/v0.7-g18e/browser_journeys.mjs`
already launches headless Chromium through Playwright, drives journeys with
`getByRole` and `locator` and **already takes screenshots** -- it simply writes
them to `/tmp` and records the paths, one per viewport, at the end of a run.
What is missing is per-step capture, annotation, and a place for them to live.

The annotation should be drawn from the locator that is about to be clicked,
using its bounding box, rather than placed at coordinates written down by hand.
A hand-placed circle is a second description of the interface that drifts the
moment a button moves; a circle derived from the element is correct by
construction, and if the locator stops matching, the journey fails rather than
producing a confident picture of the wrong place. Draw it by injecting an
overlay into the page before the capture rather than compositing afterwards:
that renders at the page's own device pixel ratio, needs no second imaging
toolchain, and keeps the marker in the same coordinate space as the element.
Each step then carries the screenshot and a sentence saying what the user is
doing and why -- the descriptive text is the documentation, and the picture
supports it.

**Boundaries.** No prose is rewritten automatically, no model output is executed
outside the existing sandbox, no probabilistic result becomes a build gate, and
nothing is added to `make validate`. No screenshot is ever taken of a real
library: every capture comes from the seeded fixture, because a screenshot of a
GUI is a picture of somebody's notes and this repository does not carry those. No subscription and no recurring charge.

*The zero-cost boundary was relaxed on 2026-09-03, deliberately and with
numbers.* Free endpoints proved too slow and too throttled to run a matrix: 71
to 377 seconds a call where they answered at all, and two of the four tried were
rate-limited on contact. The user authorised `openrouter/z-ai/glm-5.3-flash` at
$0.075 in and $0.25 out per million, which answers in 18 seconds. Cost is not
the constraint at any plausible multiplier. The whole documentation corpus is
293,267 characters -- roughly 55,000 to 73,000 tokens -- but the harness sends
one task's prose per call, so a full pass over every journey and arm is about
3,250 tokens of prompt, and a run with repeats stays under a cent even allowing
an order of magnitude for the agent's own system prompt and tool round-trips.
The harness still refuses a slug without `free` unless `--allow-paid` is given
**with a written reason**, and that reason is recorded in the evidence, so a
paid run cannot happen by accident or without saying why. `zg` uses a local embedding model and its remote-data path stays off.
Notes, databases, evidence archives and anything under `data/` are never sent
anywhere. No change to docgen, docaudit, or any G18 gate.

**Dependencies.** None in this milestone. It reads the frozen G18a inventory,
the docaudit registry and the G18f calibration, all of which are already
committed.

**Working state.** A features page whose surface inventory is generated and
whose capability prose is written, with every anchored surface claimed; a
command-line journey catalogue whose steps execute against a seeded library; a
GUI journey catalogue with annotated per-step screenshots; a recorded comparison
of the two catalogues naming every asymmetry; and then a recorded model run over
a page sample, with per-model calibration scores, per-task three-arm results,
the exact prompts and their hashes, and a written recommendation on whether to
proceed -- including "no" as an acceptable outcome.

**Validation and evidence.** For the generation half: the surface-coverage
check, failing on any anchored surface no feature claims; every command-line
journey executed with its postcondition asserted; every GUI journey run in the
browser with its screenshots produced from locators that still match; and the
catalogue comparison, with each asymmetry classified as deliberate or a gap. For
the evaluation half: calibration scores for each candidate model on the same 16
runs the Qwen baseline used, so the comparison is like-for-like; the three-arm
results per task; every generated command with its exit status and what it did;
prompt and source hashes for reproducibility; and the tags case as a positive
control that the method must flag. Evidence under `performance/v0.8-h14/`,
validated the way other evidence directories are.

**Open decisions**

**Slice A complete 2026-09-03: the coverage check, and it found the control by
itself.** `internal/docfeatures` loads `docs/docfeatures/FEATURES.json` and
compares it against the four surface inventories, which it gets from a new
`docgen.Surfaces` rather than reading the sources again -- the same extractors
that already produce the pinned counts of 59 CLI usage forms, 109 REST
operations, 46 MCP tools and 37 GUI journeys. A second extraction would be a
second opinion about what exists, and the value of a coverage check is that
there is only one.

*It reproduces the tags defect mechanically.* Run against the repository with a
registry containing one feature, it reports `asymmetry tag-a-note has=[rest mcp]
missing=[cli gui]`. That gap was found by reading pages by hand; it is now found
by the build, with no model involved. `TestTheTagsAsymmetryIsStillReported`
fails if it stops being reported, and says which of the two possible reasons the
reader should check.

*It checks both directions, and only one of them is ratcheted.* An unclaimed
surface is a capability no reader can discover; there are 210 of them today, so
that check is a **ratchet** -- the backlog may exist and may not grow. A gate
demanding zero on the day it was written would have been red immediately and
switched off within a week. A phantom claim, a feature naming a surface that no
longer exists, is absolute: there is no backlog of those, and a page describing
something that was removed is a defect from the moment it happens. It is also
exactly what a consistency check cannot see, because such prose is perfectly
consistent with itself.

*Both were confirmed by mutation.* Dropping a claim pushes REST from 107 to 109
unclaimed and fails the ratchet; adding a claim on a nonexistent tool is
reported by name. The ratchet also fails when a count goes **down** without the
baseline being lowered, so an improvement is locked in rather than left as slack
for the next regression to spend.

*What slice A does not do.* The registry holds one feature. Writing the other
capability entries -- the editorial half, which is the part no generator can do
-- is the next slice, and each one lowers a baseline.

**Slice B, first half, complete 2026-09-03: the registry is written and the
ratchet is now a gate.** All 29 features are recorded and every one of the 214
surfaces is claimed -- 60 CLI usage forms, 109 REST operations, 46 MCP tools.
The baselines are zero, which turns the ratchet into an absolute gate: a new
command, operation or tool added without a feature entry now fails, rather than
being tolerated as backlog. Clearing it in one slice was possible because the
grouping is editorial rather than laborious; twenty-nine capabilities cover a
surface area that reads as much larger when listed as flags.

*It found a defect I had introduced myself, three commits earlier.* `notriosctl
sync migrate-credentials`, added in H9 slice D, went into `printSyncUsage` and
`docs/cli.md` but never into `printHelp` -- which is the anchored, counted
registry. `notriosctl --help` did not mention it, the count stayed at 59, and no
gate fired, because nothing was inconsistent: the command simply was not
claimed anywhere the machinery looks. That is precisely the class of defect this
item exists to find, and it found one on its author. Registering it moved the
pinned count 59 to 60.

*And it caught three endpoints that do not exist.* Writing the registry, I
claimed `GET /api/v1/resources`, `GET /api/v1/selection/plan` and `GET
/api/v1/documents` -- all plausible, none real. The phantom check named all
three by feature. This is the direction a consistency gate cannot see, and the
first time it ran against real prose it caught the author inventing API.

*The asymmetry report is now a document in its own right.* Fourteen features
offer a capability on some surfaces and not others, each carrying a
`surface_note` saying whether that is deliberate. Most are: importing reads
directories on this machine, so it is command line only; profiles are about this
machine, so a service answering for one must not reach another; credential
migration is command line only because this milestone forbids a credential REST
surface. The one with no note is `tag-a-note`, which is the gap rather than a
decision, and the test that guards it says so.

**Slice B complete 2026-09-03: `docs/features.md`, and what one page costs.**
The page is written and its capability list is generated from the registry, so
the surfaces a reader is told about are the checked ones. It says plainly that
the four surfaces are not equivalent and why -- the command line owns anything
touching this machine, the GUI owns writing, REST and MCP are for other programs
-- and it names the one asymmetry that is a gap rather than a design.

*The page moved eleven pinned counts, and that is the finding.* G18a's document
count and grade denominator, the docs-site staging count and its test, G18b's
prototype builder, docaudit's manual sections, fragments, generated and
unverified counts and its denominator, G18f's slot count, its user/api split and
its unique-fragment count, G18g's repository-file count and its idempotency
assertion, and helpdocs' seed counts -- because `helpdocs` seeds every Markdown
file under `docs/`, so a new page becomes a new Help note automatically. Nothing
here was wrong; every one of those is a gate noticing a real change. But it is
worth recording what a documentation page costs in this repository before the
journey catalogues add several more.

*Two frozen v0.7 records had to be relaxed, and both relaxations are narrower
than they look.* G18c pinned `len(topics) == len(documents) == 15`; a frozen
v0.7 record cannot be edited to claim it always knew about a page written in
v0.8, so the document count became a floor -- the same shape the executables
assertion beside it already used. G18f asserted that its recorded advisory
reviews exactly matched the user fragments; those reviews are a record of an
actual Qwen run, and writing an entry for a new fragment would mean **inventing
model output that never existed**. So it became a subset check with the
direction that matters kept -- a recorded review naming a fragment that no
longer exists still fails -- plus an explicit assertion that `feature-surface`
is the one unreviewed fragment. It is recorded as unreviewed rather than assumed
to have passed, which is the same shape H8 uses for a row it cannot execute.

**Slice C complete 2026-09-03: eight executed command-line journeys, and five
defects found by running them.** `docs/docjourneys/CLI_JOURNEYS.json` holds each
journey's steps and the postcondition that confirms it; a test runs all eight
against disposable libraries and checks the state afterwards, not the exit
status. `docs/journeys-cli.md` is generated from the same catalogue, so the page
and the execution cannot drift. Every journey names a feature, and the count of
features with no journey is its own ratchet -- 24 today, allowed to shrink and
not to grow. That is a different gap from an unclaimed surface: one is a
capability nobody can discover, the other is one someone can find but has not
been shown how to use.

*Two flags I documented do not exist.* `paths --db` and `import obsidian
--notebook` were both written down from memory of neighbouring commands rather
than from the usage message, and both exit 2. That is precisely the failure mode
the executed-journey design exists to catch, arriving on the first run, in prose
written by someone who had just read the whole CLI surface to build the features
registry.

*And three product gaps, one of which is a defect rather than a design.* There
is no `notriosctl` command that writes a note, so a command-line note arrives by
import -- the test suite already worked around this, with a comment saying so.
The Obsidian importer cannot choose a notebook. And **`--collection` will not
create a collection**: naming one that does not exist fails with `FOREIGN KEY
constraint failed`, which is SQLite talking, not Notrios. A plausible user
action producing an internal error message is a defect, and it is recorded here
rather than smoothed over in the prose.

*A schema change the catalogue forced.* Requiring every step to be a
`notriosctl` command made "write a Markdown file" into a placeholder, which is
dishonest about what a task involves. Steps can now be marked manual: the reader
does them, the runner does not, and they appear in the page. A catalogue that
could only describe steps it can run would leave out the parts a reader is most
likely to get stuck on.

**`notriosctl notes create` added 2026-09-03, closing the gap slice C could not
document around.** The case for it was made by evidence rather than argument:
this repository's own sync tests already created notes by writing a Markdown
file and importing it as a one-file Obsidian vault, with a comment saying the
CLI could not do it; the features registry recorded `write-notes` as having no
command-line surface; and the journey catalogue hit the same wall when it tried
to write the task down. `runNoteMove` had already been added for exactly this
shape of gap -- reachable from the store, REST and MCP and from neither surface
a person uses -- so there was precedent as well.

The body comes from an argument, a file, or standard input, and standard input
is the one that matters: it makes a note the end of a pipeline rather than
something staged on disk first. A notebook is resolved before the write and
refused rather than guessed when the name is ambiguous, for the reason `notes
move` gives: notebook names are unique only among siblings. The refusal names
what the user asked for -- which is the contrast slice C found, where `import
--collection` fails the same case with a raw `FOREIGN KEY constraint failed`.

The journey is now a real one: create a note, file it by notebook, see the
refusal, and confirm both notes in an export. `write-notes` gains a command-line
surface and the usage-form count moves 60 to 61.

- **How the click marker is positioned -- Resolved before implementation:
  derived from the element, never written down.** Each step captures the
  bounding box of the locator it is about to click and draws the marker there,
  as an overlay injected into the page before the screenshot rather than
  composited afterwards. Three consequences follow and all three are the reason.
  A moved button moves the circle, so the picture cannot drift from the
  interface while still looking authoritative. A locator that stops matching
  fails the journey instead of producing a confident image of the wrong place --
  the failure is loud rather than silent, which is the property a screenshot in
  documentation otherwise lacks entirely. And drawing in-page keeps the marker
  in the element's own coordinate space at the page's device pixel ratio, so no
  second imaging toolchain is involved and no scaling arithmetic can be wrong.
  Hand-placed coordinates are refused outright: they are a second description of
  the interface, and a second description is a thing that disagrees with the
  first.
**Slice D complete 2026-09-03: three GUI journeys, five annotated screenshots,
and the failure the design was written to prevent -- caught, at last, by an
accident.** `performance/v0.8-h14/gui_journeys.mjs` drives the real interface,
resolves each step's locator, draws the marker from that element's bounding box,
photographs it, then acts. `docs/journeys-gui.md` is generated from the same
catalogue, so the sentence beside a picture is the string the runner used when
it took it.

*The marker function silently did nothing, and nothing noticed.* It was passed
to `page.evaluate` as a **string**, which evaluates the expression, constructs
the arrow function, ignores the argument and never calls it. Five screenshots
came out with no marker on them, no error anywhere, and every gate green -- a
confident picture of nothing, which is precisely the failure the derived-marker
decision was recorded to prevent. What caught it was two steps pointing at
different elements producing byte-identical files. That comparison is now a
check rather than a coincidence: **two steps with different locators may not
produce identical images**, because if the marker stops being drawn every step
in the same app state photographs the same way.

*The first working marker was also wrong, and visibly so.* Sized to the element,
it drew a 244-pixel ring over a full-width sidebar row: it swallowed five rows
and pointed at nothing. The fix separates two claims that had been conflated --
a thin outline for the element, which is how much of the screen is clickable,
and a fixed 40-pixel circle at its centre, which is where the click actually
goes. Both derived from the same box. It took looking at the picture to see
this, which is worth recording: the hash check proved the marker existed, and
only a person could tell it was useless.

*The gate runs without a browser.* Capture is opt-in behind
`NOTRIOS_GUI_JOURNEYS=1`, but a missing screenshot, a stale one, or a step that
starts pointing at a different element all fail an ordinary `go test` run, by
comparing committed images against the hashes recorded when they were taken.
Both halves were confirmed by mutation. Five images total 708 KB, well inside
the bound the decision below asked for.

*One dependency finding.* The Playwright browser journeys do not run from a
clean checkout: `playwright` is not a dependency of this repository, and the
runner resolves it from a sibling project through `PLAYWRIGHT_MODULE`. The
browsers are in the shared cache, so this machine works and a fresh one would
not. Recorded rather than fixed, because pinning a browser automation stack is a
dependency decision rather than a documentation one.

**Slice E complete 2026-09-03: the comparison, computed, and it finds the
control on its own.** `internal/doccompare` puts the two surfaces side by side
from three things that already exist -- which surfaces each capability claims,
which have a command-line journey, which have an interface journey -- and
reports 28 differences over 29 features. It is computed rather than written,
because a hand-maintained list of differences is a list that stops being true.

*Two kinds of disagreement, and they are not the same problem.* A **capability**
difference is something one surface can do and the other cannot: 15 command-line
only, 2 interface only, 7 reachable from neither. A **documentation** difference
is a capability both surfaces offer where only one journey is written: 4 of
those. The first is about the product, the second about these pages, and
conflating them would let a missing journey look like a design decision. The
comparison deliberately drops a feature's `surface_note` when the difference is
documentary, for exactly that reason.

*Exactly one difference is unexplained, and it is the one the item named at the
start.* Tagging a note is reachable over REST and MCP and from neither the
command line nor the interface, and no `surface_note` accounts for it. Every
other capability difference carries its reason -- importing reads directories on
this machine, profiles are about this machine, credential migration is forbidden
a REST surface by H9. That gap was found in this item's opening paragraph by
reading pages by hand; it is now produced by comparing two catalogues, and the
generated page prints it in bold as a gap rather than a decision.

*It is a ratchet, and it bites.* Removing the explanation from `profiles` makes
the unexplained list `[profiles tag-a-note]` and fails. So a capability that
stops reaching a surface must either gain a reason or be recorded as a gap; it
cannot arrive quietly.

- **Where the screenshots live -- Resolved 2026-09-03: `docs/images/journeys/`,
  committed.** They are generated artifacts, which this repository does not
  normally commit, and they churn on every interface change. The earlier
  recommendation here was not to commit them at all. That is revised, because
  checking the alternatives showed the objection was weaker than it looked and
  the cost of not committing was higher.

  **Not `data/assets`.** That is a user's asset store: `data/` is gitignored,
  the `data` root is `backup_and_verify`, and a purge deletes it. Documentation
  images living there would be backed up as though they were somebody's notes,
  destroyed by a purge, and in an installed profile would be written into the
  user's real library. The whole point of H3's six roots is that product content
  and user content are not the same thing.

  **Not `assets/` either, though it is the right *kind* of place.** It is
  committed, already holds `assets/icons/*/notrios.png`, and is staged into
  `program_assets` by the packaging -- but `build_deb.sh` stages `assets/icons`
  specifically rather than the directory wholesale, so putting journey images
  there would be relying on that narrowness to avoid shipping them.

  So `docs/images/journeys/`: committed, beside the pages that reference them,
  outside the path the packaging stages. A screenshot that is not committed does
  not appear when someone reads `docs/gui.md` on a git host, which is where most
  readers are, and that was the cost the earlier recommendation accepted too
  readily. The manifest survives the change of home and still earns its place:
  step id, locator and image hash are committed alongside, so a stale screenshot
  fails the gate rather than quietly misleading. Bound it deliberately -- one
  fixed viewport width, a cap on count and dimensions -- rather than discovering
  the repository size afterwards.
- **How much the new pages move the pinned counts -- Non-blocking but noisy.**
  One section and one example moved five pinned counts in H9 slice D. A features
  page plus two journey catalogues is a large multiple of that, across G18a's
  inventory and denominator, the docaudit surface, G18d's registry and the G18f
  hashes. Recommended: land the generation in slices, one catalogue at a time,
  and re-pin each time rather than once at the end, so a count that moves for the
  wrong reason is still findable.
- **Whether the journey catalogues are new pages or new sections -- Non-blocking,
  decide before writing.** `docs/cli.md` and `docs/gui.md` are already long, and
  every section in them is anchored to an owner. Recommended: separate pages,
  `docs/features.md` and two journey pages, because a journey catalogue has a
  different shape from a command reference and mixing them makes both worse --
  and because a separate page can be regenerated without re-hashing a reference
  page nobody changed.
- **Whether an executed GUI journey is required, or only an executed CLI one --
  Blocking for the gate design.** The browser journeys are opt-in today,
  behind `NOTRIOS_G18E_BROWSER=1`, because they need Playwright and a real
  browser. Making illustrated GUI journeys a committed claim while their
  execution stays optional would mean shipping pictures nothing verifies.
  Recommended: keep browser execution opt-in for `make validate`, but require it
  for the evidence bundle, so the claim is only made when it has been run --
  the same shape H8 uses for rows it cannot execute everywhere.

- **Sending repository documentation to a hosted model -- Resolved 2026-09-03:
  permitted for `docs/` prose and generated command text.** The user authorised
  it on the terms recommended below. Everything else stays loopback, and the
  endpoint is recorded per run. The exposure was always small -- the
  documentation is Apache-2.0 and written to be published -- but it is a change
  to G18f's recorded `endpoint_scope: loopback-only` and is recorded as one.
- **(superseded) Sending repository documentation to a hosted model.** G18f's
  recorded policy is `endpoint_scope: loopback-only`,
  `source_scope: repository-source-only; no notes or databases`, `cost_usd: 0`.
  Using hosted free models changes the first of those. The documentation is
  Apache-2.0 and written to be published, so the exposure is small, but it is a
  policy change and must be recorded as one rather than assumed. Recommended:
  permit hosted calls for `docs/` prose and generated command text only, keep
  everything else loopback, and record the endpoint used per run.
- **Which free models -- Non-blocking, and not decidable from a list.** The
  supplied recommendations describe capabilities that appear to be inferred from
  the model names rather than measured -- a `-fin` suffix read as "financial", a
  `-reasoning` suffix read as "a dedicated internal reasoning token track", a
  vendor read as "purpose-built for software engineering". Several named models
  cannot be verified from here at all. This is exactly what the calibration set
  is for: run the candidates on the same 16 runs and let the score decide.

  **Trial roster, in this order.** Order is a guess at capacity and nothing
  more; the calibration score replaces it as soon as there is one.

  1. `openrouter/z-ai/glm-5.2:free`
  2. `openrouter/nvidia/nemotron-3-ultra-550b-a55b:free`
  3. `openrouter/google/gemma-4-31b-it:free`
  4. `openrouter/nvidia/nemotron-3-super-120b-a12b:free`
  5. `openrouter/google/gemma-4-26b-a4b-it:free`
  6. `openrouter/minimax/minimax-m3:free`
  7. `openrouter/cohere/north-mini-code:free`
  8. `openrouter/thinkingmachines/inkling:free`

  Every one of them runs the same 16 calibration runs and the same three-arm
  tasks, so a claim about any of them is answered by a number. Exclude
  `openrouter/nvidia/nemotron-3.5-content-safety:free`, whose name at least is
  unambiguous. A model that cannot beat 7/16 is dropped rather than tuned.

- **Every model gets the same, deliberately small context -- Blocking for the
  harness design.** One task, one page section, no repository access, and no
  `zg` retrieval into the answering prompt. `zg` selects which sections to test
  and helps a human read the results; it does not enrich the prompt under test.

  This is a property of the measurement, not a limitation being worked around.
  The question is whether **the prose alone** is sufficient to act, so a model
  that has also read `cmd/notriosctl` will produce a correct command whether the
  page is any good or not -- scoring the codebase while appearing to score the
  documentation. That is the "measures the model, not the docs" failure the
  no-prose arm exists to catch, arriving through the context window instead of
  through the model's memory.

  It follows that context capacity is irrelevant here, and a model recommended on
  the strength of it earns no credit for that. Feeding a repository to a large
  context to cross-reference code against prose is a sound technique for a
  different question -- "does the documentation match the code?" -- which the
  existing anchored inventory and generated fragments already answer
  deterministically and for free.
- **Rate limits and spreading work -- Non-blocking.** Free tiers throttle. The
  harness must be resumable, cache by prompt hash the way the existing advisory
  report already records `explanation_prompt_sha256`, and record which model
  answered which task so a mixed run stays attributable. The supplied material
  predicts that the GLM free endpoint in particular returns 429 under sustained
  sequential use while the Gemma endpoints are steadier. That is a testable
  claim, so record observed throttling per model as a result rather than
  designing around it in advance: a model that cannot complete a run is unusable
  here however well it scores on the runs it does complete.

- **Whether GLM-5.2 is actually free at the tier used -- Resolved 2026-09-03,
  and the rule generalises.** A model is free when its slug contains `free`;
  the paid GLM-5.2 is a different slug without it. `opencode models` lists
  every model, so the free set is a filter rather than a judgement, and the
  harness refuses any `--model` whose slug does not contain `free` rather than
  trusting the caller. The account does not pay for overage automatically, so an
  accidental paid call fails rather than bills -- which makes this a checkable
  property rather than a promise. `cost_usd: 0` stays a recorded property of the
  evidence.
- **(superseded) Whether GLM-5.2 is actually free at the tier used.** The supplied material contradicts itself, tabulating GLM-5.2
  as "Paid (~$0.49/M input)" in one comparison and describing a working
  `:free` endpoint in another. The constraint on this whole item is zero cost,
  so the endpoint must be confirmed free at the point of use, and the run
  aborted if any call would be billed. `cost_usd: 0` stays a recorded property
  of the evidence, as it is in G18f.
- **Whether the CLI is meant to have no add/remove-tag command -- Answered
  2026-09-03 by adding one.** `notriosctl tags add`, `tags remove` and `tags
  list` close the command-line half of the gap this item opened with. `tags
  list` exists because the other two would otherwise be unverifiable from the
  surface that performs them: a command that changes something and offers no way
  to see the change asks its caller to take it on trust.

  Two refusals were written to the standard this milestone criticised elsewhere.
  The store answers a missing note and a missing tag with the same bare "not
  found", so the commands name what the user asked for instead -- the same
  failing that made `import --collection` report a raw `FOREIGN KEY constraint
  failed`. And `ListDocumentTags` returns an empty list for a note that does not
  exist, so `tags list --document` checks the note first: "this note has no
  tags" and "there is no such note" are different answers, and a command whose
  job is verifying an edit must not conflate them.

  The gates moved as they should. `doccompare`'s unexplained-gap list is now
  **empty** -- every capability one surface has and another lacks carries a
  written reason -- and the ratchet demanded that be locked in rather than left
  as slack. The features registry records the GUI half as still open, so it is
  an explained asymmetry rather than an unaccounted one. The slice A guard was
  narrowed rather than deleted, exactly as its own comment instructed a future
  reader to do. And the tagging journey, which could not be written before, now
  exists and executes.

  **The GUI half is still open**: tags remain sidebar navigation with note
  counts, and there is no control that adds or removes one.
- **(superseded) Whether the CLI is meant to have no add/remove-tag command.** REST and MCP can tag a note and
  the CLI cannot. If that is deliberate the CLI guide should say so and point at
  the surfaces that can; if it is an oversight it is a product gap rather than a
  documentation one. The investigation records the question; it does not answer
  it.

**Slice F, pilot complete 2026-09-03: the harness works, and the first complete
task failed the ablation.** `performance/v0.8-h14/actionability.py` builds the
three arms from the journey catalogue -- which already holds a task, its prose
and its postcondition -- asks a free model for one command, and runs it against
a disposable library. It refuses any `--model` whose slug does not contain
`free`, refuses to execute an argument vector containing anything a shell would
interpret, and replaces whatever paths the model names with the sandbox's own,
so the model does not choose which library it touches.

*The result that matters.* On `find-your-library`: the prose arm produced
`notriosctl paths` and the postcondition held; the **no-prose arm produced the
same command and the same result**. Under this item's own rule -- a page earns
credit only when the prose arm succeeds and the no-prose arm fails -- the task
scores **zero**, and correctly so: the model did not need the documentation, it
guessed a conventional command name. A two-arm version of this method would have
reported that page as actionable on the strength of a model that never read it.
That is the failure the ablation exists to catch and it caught it on the first
complete task, which is the most useful thing the pilot could have done.

*One run per arm is not enough, demonstrated by accident.* A manual no-prose
call to the same model on the same task returned `notriosctl config`, which does
not exist; the harness's returned `notriosctl paths`, which does. Same model,
same arm, opposite outcomes. The existing G18f calibration used two repeats per
case for this reason, and any real run of this method needs repeats before a
single result is read as a fact.

*Findings about the instrument, which is the other half of what a pilot is
for.* `opencode run` is an agentic CLI rather than a completion endpoint. Its
permission configuration asks before touching an external directory, every
sandbox is external, and a non-interactive run cannot answer -- so the call
**hangs until the timeout instead of failing**, which cost three arms before it
was diagnosed. `--pure --auto` is therefore required rather than preferred, and
the isolation this item depends on ("no repository access") rests on the sandbox
being empty rather than on the tool refusing. `opencode` also exits **zero** on
an upstream rate limit, so the harness reads the text rather than the status.

*Free endpoints throttle, as predicted, and it is a result rather than an
obstacle.* `openrouter/z-ai/glm-5.2:free` and
`openrouter/google/gemma-4-31b-it:free` were rate-limited upstream on first
contact; `opencode/nemotron-3-ultra-free` and
`openrouter/nvidia/nemotron-3-super-120b-a12b:free` answered. Latency ran 71 to
377 seconds per call, so the full roster over eight journeys and three arms is
several hours of wall clock -- which is why this is a pilot of one task and says
so.

*`zg` was indexed locally and used for selection, not for answering.* `zg index
--embedding local/potion-retrieval-32m` over a copy of `docs/` alone; no notes,
no database, no remote embedding. Querying it for "how do I add a tag to a note"
returns the features page, the registry and "Renaming a tag hierarchy" -- and
nothing that answers the question, because nothing does. The retrieval
corroborates the tags gap from a reader's angle, independently of the comparison
in slice E.

*Against the exit criteria, the honest reading is: not yet.* The criteria
require the ablation to separate arm 1 from arm 2 on a page known to be good,
and on the one task run it did not separate at all. That is a result about the
task rather than about the method -- `notriosctl paths` is guessable and a task
whose command is guessable cannot measure a page -- but the criteria are not met
and no amount of further running changes that for this task. Recommended before
any full evaluation: choose tasks whose commands are *not* conventional, repeat
each arm at least twice, and treat a task where no-prose succeeds as evidence
about the task rather than the documentation. If those do not produce
separation, the recommendation is to stop, and the investigation will still have
been worth doing -- it has already produced a features coverage gate, three
executed catalogues and five product defects without a model being involved at
all.

**Slice F, full run 2026-09-03: repeats, deliberately unguessable tasks, a
faster model -- and still nothing separates. That is the answer.** 24 runs: four
journeys marked `notrios-specific`, three arms, two repeats each, on
`openrouter/z-ai/glm-5.3-flash` at 18 to 30 seconds a call against the free
models' 71 to 377. **Zero of four tasks credited.**

| task | prose | no-prose |
|---|---|---|
| search with the query language | acted, acted | refused, **acted** |
| export and verify an archive | no-change, no-change | no-change, no-change |
| read the built-in help | acted, unavailable | **acted, acted** |
| move sync keys to the keychain | no-change, no-change | no-change, no-change |

*The no-prose arm keeps winning, and marking tasks "unguessable" did not stop
it.* The model produced `seed-help` twice with no documentation at all, and
`export archive --query` once. Guessability was recorded in the catalogue
precisely to remove this, and it did not, which points at something the pilot
did not show: **the task statement itself paraphrases the command.** Every arm
receives the journey's title and goal, and "Get the Notrios guides into the
library as notes" is very nearly a definition of `seed-help`. The no-prose arm
is therefore not a clean control -- it is the prose arm with the steps removed
but the answer still in the framing. That is the method's real limit, and it was
invisible until tasks chosen to defeat guessing failed to defeat it.

*Two tasks failed on the prose arm for reasons that are mine, not the pages'.*
`export archive-v2` produced the right command with the model's own output path,
and the postcondition checks a path the harness chose, so it recorded
`no-change` for a command that worked. A postcondition that depends on a path
the model picks cannot be written this way. The keychain task saw the model emit
a literal `{config}` placeholder it invented, which the harness stripped, leaving
a command that could not do the thing. Both are harness defects surfaced by
running it, and both are recorded rather than tuned away.

*A containment gap the run found in the harness itself.* One run produced
`export archive-v2 /backups/notrios-2026-08-04` -- a **positional path outside
the sandbox**, which the harness passed straight through. It failed only because
`/backups` does not exist; a model naming `/tmp` or a path under the user's home
would have been written to. Root flags were being stripped and positional paths
were not. Any argument that looks like a path is now redirected under the
sandbox, keeping its base name. Separately confirmed: `notriosctl` is not on
`PATH`, so the agent could not have read the usage text, and the no-prose
successes are genuine guesses rather than tool-assisted discovery -- which
matters, because that would have invalidated every result here.

*Recommendation: stop, and keep what the item already produced.* The exit
criteria require the ablation to separate arm 1 from arm 2 on a page known to be
good. With repeats, with tasks chosen to be unguessable, and with a model fast
enough to run a matrix, it separated on nothing. Making the no-prose arm a real
control needs task statements that convey a user's intent without paraphrasing
the command, and it is not obvious that such a statement exists for most tasks --
a task named without its own vocabulary may not be a task a reader would
recognise either. That is a research problem, not a documentation one.

The investigation was still worth doing, and not as consolation. Its
model-free half produced a features page with a coverage gate at zero backlog,
three catalogues -- nine executed command-line journeys, three photographed
interface journeys -- a computed surface comparison, and eight defects: two
documented flags that do not exist, three product gaps including a raw
`FOREIGN KEY constraint failed` on a plausible user action, a command missing
from the CLI registry, and two containment defects in the harness itself. The
`notes create` command exists because writing this found the gap. Not one of
those needed a model.

**Corrections 2026-09-03, from reading the pages as a reader rather than as
their author.** Five faults, and the last is the one that explains the others.

*The page contradicted the product.* `docs/journeys-cli.md` still said there is
no command that writes a note, three commits after `notes create` was added. The
page that this item built to catch stale documentation had gone stale, and no
gate noticed, because nothing was inconsistent -- the sentence was merely untrue.

*It mentioned tags and documented nothing about them.* The tagging journey was a
placeholder with the actual tagging as a manual step. It is now a real journey:
add two tags, take one off, and read the note back, with a postcondition that
asserts the removed tag is **absent** rather than only that the kept one is
present.

*The generated list showed titles and no commands.* A reader learned which tasks
existed and was left no better able to do any of them. The fragment now emits
every step with its command, which is what makes the page documentation rather
than a table of contents.

*The commands it would have shown were unusable anyway.* They carried the
sandbox plumbing every journey needs to run in isolation -- `--db {db}
--asset-store {assets}` -- which no reader ever types. Those flags are now
stripped when rendering, and remaining placeholders become angle-bracket
metavariables, because `{note}` is a substitution and `<note-id>` is an
instruction.

*And both pages read as test instructions.* They explained hash manifests,
mutation results, regenerate commands and the development history of their own
bugs. All of that is true and none of it belongs in front of someone trying to
tag a note; it belongs here. The pages now say only what a reader needs, name
each other so the two surfaces can be compared, and state plainly that tagging
is unavailable in the interface -- as a gap, in a sentence, rather than as a
registry classification.

*A capability finding fell out of the rewrite.* Making the two catalogues
parallel required asking, task by task, whether each surface can do the thing.
Tagging is the only capability the command line has and the interface does not,
and now both pages say so in the same words.

**Exit criteria.** The generation half stands on its own and is not conditional
on the model work: the features page, the two journey catalogues and the
comparison are worth having whether or not any model turns out to be usable, and
the surface-coverage check is worth having whether or not the pages are ever
scored. If the evaluation half is abandoned, the generation half still ships.

The evaluation method is worth adopting only if all of these hold: at
least one free model scores materially better than the 7/16 baseline on the
existing calibration; the three-arm ablation separates arm 1 from arm 2 on a page
known to be good, so the test can tell prose from prior knowledge; and the tags
case is flagged. Failing any of them, the recommendation is to stop, and the
investigation is still worth having done.

**If it succeeds.** Add a follow-up plan item to evaluate the documentation with
the method: a full pass over the user-facing pages, a ranked list of task topics
a reader cannot act on, and prose fixes for the worst of them -- with the
evaluation itself staying advisory and out of `make validate`. That item is not
written yet, deliberately: it should be scoped by what the investigation actually
finds rather than by what it is hoped to find.

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
| Desktop external-link opening | H2b | Resolved and implemented: no prompt; the browser-tab path is unchanged |
| Mermaid renderer/containment | H2a/H2 | Resolved and implemented in H2: Mermaid 11.17.2, strict security, `htmlLabels: false`, `notrios`-only links reattached from source |
| Installed/portable path precedence | H3/H4 | Resolved in H3: explicit, then explicit portable marker, then native; never inferred |
| Ubuntu packaging toolchain | H6a/H6 | Resolved in H6: dpkg-deb with dpkg-shlibdeps, staged from lifecycle.py, adding no build dependency; 0 lintian errors |
| Native credential store on a headless install | H9 (post-v1.0) | Open; go-keyring needs a session-bus Secret Service. A `pass`/`age` tier is now investigated and executable on this machine, so the Linux answer is an opt-in tier with a documented weaker guarantee rather than a refusal -- but headless is not a Linux-only condition and the Windows service-account and locked-macOS-Keychain cases are still unexamined |
| Windows and macOS toolchain | H6a/H7 | Deferred to post-v1.0: no toolchain is selectable without native runners, and the hardware is not available |
| Development versus installed default port | H4a | Resolved in H4a: checkout 8099 from the example config, installed 8080 from the compiled default |
| Pre-migration backup location and retention | H4b | Resolved in H4b: `pre-migration-backups/` beside the database, newest kept, no new root |
| Migration trigger narrowed from H3 section 4 | H4 slice E | Resolved; slice D removed the two-instance case, so the trigger is a pre-0.8 layout in the working directory that is not the library in use |
| User-local/GNU install layout | H3/H5 | Resolved in H3: `$HOME/.local`, GNU directory variables and `DESTDIR` retained |
| Purge external-path and backup policy | H3/H5 | Resolved: enumerate and back up, refuse to delete; H5 adopts H3's proven tar-plus-manifest container beside the state root |
| Installed program-assets root | H4/H5 | Resolved in H5: derived from the executable, so the GNU prefix decides it and a user-local install finds its own assets |
| Hosted free models reading repository documentation | H14 | Open; G18f recorded loopback-only, so this is a policy change and must be decided explicitly |
| Free-model choice for documentation evaluation | H14 | Open; supplied recommendations look name-inferred, so the existing 16-run calibration decides across an eight-model roster led by `glm-5.2:free` |
| Context given to the model under test | H14 | Resolved in planning: one page section only, identical for every model. Feeding the repository would score the codebase while appearing to score the documentation |
| Oracle for a generated command | H14 | Resolved in planning: the observed state change, not the exit status, with "did the opposite" scored separately from "did nothing" |
| CLI has no add/remove-tag command while REST and MCP do | H14 | Open; deliberate omission to document, or a product gap |
| Desktop package formats/toolchain | H6a/H6/H7 | Ubuntu resolved in H6 (dpkg-deb, 0 lintian errors); Windows and macOS deferred to post-v1.0 with the hardware |
| Windows/macOS feasibility and support | H6a/H7/H12 | Deferred to post-v1.0; native execution required and the hardware is not available |
| Native credential providers | H9 | Open and blocking implementation |
| Wails v3 spike timing/outcome | H10 | Explicit approval required; production stays v2 |
| GitHub PR merge and branch synchronization | H12/H13 | PR planned late; merge separately authorized |
| v0.8 product/schema number | H13 | Open with product 0.8.0/no gratuitous schema default |
| Internal artifact custody | H13 | Local/internal-only default; public release separately authorized |

H1, H2a, H2b, H2, and H3 completed on 2026-09-01. H4 is the next incomplete
item and remains unapproved. Do not begin it until the user explicitly says to
proceed with H4.
