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
claims -- there is no Flutter toolchain on this machine; `ella-to/vault`'s mobile
behaviour; and the claim that macOS Keychain entries can be shared between a
Flutter client and a Go client given the same service name and app group.

**Open decisions**

- **Provider per supported OS — Blocking before implementation.** No provider
  is adopted until its availability, headless behavior, license, maintenance,
  packaging, backup/purge semantics, and rollback are recorded. Android may
  remain unresolved if H11 uses a test-only injected provider and makes no
  mobile-release claim.
- **Behaviour on a Linux install with no Secret Service — Blocking.** Failing
  closed is required and is not the whole answer: the user needs to be told why
  before they enrol, not when a sync first runs. Recommended: `notriosctl
  doctor` reports whether a native store is reachable, enrolment refuses with
  that reason, and the documentation says a headless install cannot hold sync
  credentials. `go-keyring` also needs `godbus/dbus/v5` v5.2.2 while this
  project already carries v5.1.0 indirectly through Wails, so the bump is part
  of the decision.
- **Whether Windows clients must share one store — Non-blocking, decide before
  a Flutter desktop client exists.** DPAPI and Credential Manager are different
  stores. Recommended: treat each client as owning its own credential and
  re-enrolling, rather than engineering a shared store, because pairing is
  already per-replica and a shared secret across two clients is a weaker
  boundary than two secrets.

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

**Scope.** Stand up `opencode` and `zg` locally; index the repository with a
**local** embedding; reuse the existing 8-case, 16-run contradiction calibration
in `performance/v0.7-g18f/ADVISORY_REPORT.json` to score candidate free models
against the recorded Qwen 2.5 Coder 1.5B baseline of 7/16; then build a small
task-to-command harness over a handful of pages and measure whether generated
command lines run in a sandbox. Report a recommendation with evidence, change no
prose, and add no build gate.

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

**Boundaries.** No prose is rewritten automatically, no model output is executed
outside the existing sandbox, no probabilistic result becomes a build gate, and
nothing is added to `make validate`. No paid model, no subscription, no recurring
charge. `zg` uses a local embedding model and its remote-data path stays off.
Notes, databases, evidence archives and anything under `data/` are never sent
anywhere. No change to docgen, docaudit, or any G18 gate.

**Dependencies.** None in this milestone. It reads the frozen G18a inventory,
the docaudit registry and the G18f calibration, all of which are already
committed.

**Working state.** A recorded run over a small page sample, with per-model
calibration scores, per-task three-arm results, the exact prompts and their
hashes, and a written recommendation on whether to proceed -- including "no" as
an acceptable outcome.

**Validation and evidence.** Calibration scores for each candidate model on the
same 16 runs the Qwen baseline used, so the comparison is like-for-like; the
three-arm results per task; every generated command with its exit status and
what it did; prompt and source hashes for reproducibility; and the tags case as
a positive control that the method must flag. Evidence under
`performance/v0.8-h14/`, validated the way other evidence directories are.

**Open decisions**

- **Sending repository documentation to a hosted model -- Blocking.** G18f's
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

- **Whether GLM-5.2 is actually free at the tier used -- Blocking before
  relying on it.** The supplied material contradicts itself, tabulating GLM-5.2
  as "Paid (~$0.49/M input)" in one comparison and describing a working
  `:free` endpoint in another. The constraint on this whole item is zero cost,
  so the endpoint must be confirmed free at the point of use, and the run
  aborted if any call would be billed. `cost_usd: 0` stays a recorded property
  of the evidence, as it is in G18f.
- **Whether the CLI is meant to have no add/remove-tag command -- Blocking for
  the follow-up, not for this investigation.** REST and MCP can tag a note and
  the CLI cannot. If that is deliberate the CLI guide should say so and point at
  the surfaces that can; if it is an oversight it is a product gap rather than a
  documentation one. The investigation records the question; it does not answer
  it.

**Exit criteria.** The method is worth adopting only if all of these hold: at
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
| Native credential store on a headless Linux install | H9 | Open; go-keyring needs a session-bus Secret Service, so a headless install cannot hold sync credentials and must say so before enrolment |
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
