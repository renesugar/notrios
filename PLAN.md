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

## Progress

Generated from `docs/docplan/PLAN_SLICES.json` by
`go run ./cmd/docplan --write`, and checked by `internal/docplan`, which fails
the build when the ledger, this document and the repository disagree.

**The rules for keeping it current are in [`AGENTS.md`](AGENTS.md)** — under
"Writing plan items", "Keeping the plan current" and "Plan archival" — because
this section is archived when the plan completes and the rules are not.

What follows is what is left. Each item's own text below is the record of what
happened, which is a different question.

<!-- notrios:generated:plan:progress:begin -->
**33 items: 25 complete, 2 in progress, 5 not started, 1 deferred.**

| Item | State | Slices done | Outstanding |
|---|---|---|---|
| H0. Application-facade, C-ABI, and SQLite ownership investigation | complete | 0/0 | — |
| H1. Shared application facade and ABI-major-1 library | complete | 0/0 | — |
| H2a. Mermaid renderer and security investigation | complete | 0/0 | — |
| H2. Bounded offline Mermaid enablement | complete | 0/0 | — |
| H2b. Desktop external-link opening | complete | 0/0 | — |
| H3. Installed-path, XDG, migration, and destructive-lifecycle investigation | complete | 0/0 | — |
| H4. Installed runtime paths, assets, and migration | complete | 0/0 | — |
| H4a. Distinct development and installed default ports in the documentation | complete | 0/0 | — |
| H4b. Verified backup before a startup schema migration | complete | 0/0 | — |
| H5. Safe Make install, uninstall, and purge lifecycle | complete | 0/0 | — |
| H6a. Desktop installer and GitHub-native build investigation | complete | 0/0 | — |
| H6. Ubuntu-priority installer package | complete | 0/0 | — |
| H8. Installed integration harness and Ubuntu baseline | complete | 0/0 | — |
| H14. Documentation actionability investigation | complete | 0/0 | — |
| H7. Windows and macOS installer workflow implementation | deferred | 0/0 | — |
| H9. Native credential-store selection and integration | complete | 5/5 | — |
| H10. Wails v3 migration spike | not-started | 0/2 | 2 |
| H11. Android-emulator shared-core acceptance | not-started | 0/2 | 2 |
| H12. Delayed GitHub native validation and develop-to-main pull request | not-started | 0/2 | 2 |
| H15. Complete the journey catalogues, and give the GUI an inventory | in-progress | 8/9 | 1 |
| H16. Reconcile the collection model with what is actually stored | complete | 7/7 | — |
| H17. Act on many notes at once, from the search results and from a query | not-started | 0/3 | 3 |
| H18. Make the features page usable, and generate the table under it | in-progress | 3/4 | 1 |
| H19. notriosctl search | complete | 4/4 | — |
| H13. v0.8 release wrap-up and branch synchronization | not-started | 0/2 | 2 |
| H20. Bring the atlas current, and stop it drifting again | complete | 4/4 | — |
| H21. Read a note and its structure, from the command line | complete | 4/4 | — |
| H22. Discover the values a query can name | complete | 3/3 | — |
| H23. One description of the command line, and --help everywhere | complete | 6/6 | — |
| H24. JSON is the output; a template makes it readable | complete | 2/2 | — |
| H25. Hold each command's flags to its description | complete | 2/2 | — |
| H26. Ask about one tag without fetching them all | complete | 4/4 | — |
| H27. Attach a file from the command line, without guessing where the link goes | complete | 5/5 | — |

### Started and not finished

**H15. Complete the journey catalogues, and give the GUI an inventory**

- `H15-G` Write GUI journeys for the twelve features that have an interface surface and none: collections, attachments, remote media, links, graph, query blocks, sync pairing, sync exchange, sync peers, sync recovery, jobs and profiles — *blocked* (blocked on: eleven of the twelve target surfaces carry no data-testid, so a journey could only locate them by shape; making them addressable in web/src is the prerequisite and is GUI work rather than journey writing)

**H18. Make the features page usable, and generate the table under it**

- `H18-D` Rewrite the twenty-nine summaries and surface notes in FEATURES.json for a reader rather than against the surfaces — *not-started*

### Not started

Written and not begun: H10, H11, H12, H17, H13. Their slices are listed under each item.
<!-- notrios:generated:plan:progress:end -->

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
landed. Archived as
`plans/v0.8/002-application-facade-c-abi-sqlite-ownership-investigation.md`.

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
as `plans/v0.8/003-shared-application-facade-abi-library.md`. Archived as
`plans/v0.8/003-shared-application-facade-abi-library.md`.

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

## H4. Installed runtime paths, assets, and migration — complete

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

## H4a. Distinct development and installed default ports in the documentation — complete

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

## H4b. Verified backup before a startup schema migration — complete

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

## H5. Safe Make install, uninstall, and purge lifecycle — complete

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

## H9. Native credential-store selection and integration — complete

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

**Outcome (2026-09-08).** Complete. An installed profile resolves sync
credentials through the operating system's store and refuses rather than
substitutes when it cannot; keys move between that store and the owner-only
development file in either direction; the provider layer is imported rather than
owned (`zalando/go-keyring`, MIT, with `99designs/keyring` rejected on recorded
grounds); the headless Linux tier is deferred to post-v1.0; and the
host-supplied credential contract is written in `FLUTTER_GO_CLIENT.md`.

Two of those were decided and evidenced long before the ledger said so -- the
provider choice in `performance/v0.8-h9/RESULTS.json`, the headless deferral in
`ROADMAP.md` -- and the ledger read `not-started` for both. That is the drift
this milestone keeps finding, this time in the plan's own bookkeeping.

**Writing the contract found that it needs no new FFI surface.** The reference
designs for host-supplied credentials reach for a bespoke protobuf channel or an
in-memory gRPC pipe; the ABI already carries both directions --
`notrios_call_start`/`poll` for host to core, `notrios_event_poll` for core to
host -- so the contract is two operations and one event rather than a transport.
A second mechanism for one kind of payload would be a second set of buffer
ownership rules beside `notrios_buffer_release`.

The recommendation inside it is on-demand supply rather than at-open, for a
reason the desktop case does not show: on iOS a host often cannot read its own
store without a user gesture, so a secret demanded at open would fail on a
locked device where an on-demand one merely waits.

**H9-F moved to post-v1.0** rather than staying blocked. Establishing locked and
unavailable behaviour for headless Windows and headless macOS needs hardware
that does not exist here, and the v1.0 priority is desktop GUI Notrios on
Ubuntu, which needs none of it. The boundary is unchanged on every platform --
a store that cannot be reached is refused, never substituted -- and what is
unestablished is only what "cannot be reached" looks like on those two. The
roadmap's headless section says so and says where it came from.

*Three questions asked of this design on 2026-09-08, and their answers.*

**Can the credential-store code be isolated to the GUI for v1.0.0? No.**
`internal/credentials` is reached by `cmd/notriosctl` -- `sync init` mints key
material, `sync migrate-credentials` moves it -- and by `internal/service`,
which the daemon runs. Isolating it to the GUI would mean a headless or
command-line install could not sync at all, and this item's own boundary
forbids the alternative: there is **no credential REST or MCP surface**, so the
command line is the only non-GUI way by design. Removing it from the CLI would
not relocate the capability, it would delete it.

What *is* separable, and already is, is the part worth separating: the port
(`internal/credentials`) from the adapter that knows a platform
(`native_desktop.go`) and the refusal that knows it is not one
(`native_unsupported.go`). "Which store" is isolated; "who needs a secret" is
not, and cannot be.

**Must the C ABI be free of any credential store? It already is.** Nothing in
`internal/abi` or `cmd/notrioslib` references credentials, and
`native_unsupported.go` states the intent in its own comment: on mobile the host
supplies the credential across the ABI and the core is never the owner. What is
missing is not the isolation but the *contract* -- which is this slice.

**Will Flutter Mobile and Flutter Desktop use different stores? Yes, and that
is the design constraint the contract has to answer.** A single Flutter codebase
targeting both would otherwise carry two credential paths: on desktop the core
can own the secret through `native_desktop.go`, on mobile it cannot. The
contract should therefore make host-supplied the shape *both* can use, with the
desktop core owning it only where no host offers to. One path that works
everywhere beats two that differ by target, because the difference would live in
the client rather than in the core, where nothing here can test it.

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

## H14. Documentation actionability investigation (opencode, free models, zvec-grep) — complete

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

**Outcome (2026-09-04).** Complete, and the two halves ended differently.

*The generation half shipped and stands on its own*, as the exit criteria said
it would whatever the models did: `docs/features.md` with a coverage gate at
zero backlog, three catalogues, a computed surface comparison, and eight defects
found by writing them -- two documented flags that did not exist, three product
gaps including a raw `FOREIGN KEY constraint failed` on a plausible user action,
a command missing from the CLI registry, and two containment defects in the
harness itself. `notriosctl notes create` exists because writing the catalogue
found the gap. None of that needed a model.

*The evaluation half is stopped, on its own criteria.* Those criteria are
conjunctive, and the ablation failed outright: with repeats, with tasks chosen
to be unguessable, and with a model fast enough to run a matrix, **zero of four
tasks were credited** -- the no-prose arm kept succeeding, because the task
statement paraphrases the command it is asking for. The tags case was flagged,
so that criterion held. The first criterion -- a free model beating the recorded
7/16 on the existing calibration -- was never run, and this says so rather than
leaving it looking pending: it could not change the verdict, because the
criteria are conjunctive and one had already failed, and running a roster at 71
to 377 seconds a call to confirm a conclusion already reached would have been
spending hours to learn nothing.

*So the "If it succeeds" clause does not fire, and no follow-up evaluation item
is written.* Making the no-prose arm a real control needs task statements that
convey intent without the command's own vocabulary, and it is not obvious such a
statement exists for most tasks -- a task named without its vocabulary may not
be a task a reader would recognise either. That is a research problem rather
than a documentation one, and a faster or stronger model makes it worse rather
than better: a better guesser satisfies the control arm more often, so fewer
pages earn credit.

Evidence under `performance/v0.8-h14/`: `ACTIONABILITY.json` (24 runs, three
arms, two repeats, one paid model with its authorisation recorded), the harness
that produced it, and the journey capture. Nothing here is a build gate and
nothing was added to `make validate`, as this item required of itself.

*The record of executing this is archived* as
`plans/v0.8/008-documentation-actionability-investigation.md`: every slice, the
harness findings, the two rounds of corrections that came from reading the pages
as a reader, and the run that stopped the evaluation half. It is kept verbatim,
and it is nine hundred lines that describe finished work -- which is why it is
there and not here.

## H15. Complete the journey catalogues, and give the GUI an inventory

**Ordering.** After H14, which built the machinery this fills in. Independent of
the installer chain. The GUI tagging work is a product change and can proceed on
its own; the rest is documentation.

**Goal.** Cover the tasks a person actually arrives with, on both surfaces, and
stop relying on someone remembering what the GUI can do.

**Why this is a separate item.** H14 built the catalogues, the coverage gate,
the comparison and the capture, and proved them by finding eight defects. It did
not fill them in. Ten command-line journeys and four interface journeys is a
demonstration, not a manual, and the difference matters because the machinery now
makes the gaps countable.

**Coverage today, measured rather than remembered.** Against the twelve tasks
named at the start of H14:

| Task | Command line | Wails GUI |
|---|---|---|
| create a note in a named notebook | covered | partial: opens the notebook chooser, does not finish |
| update a note in a named notebook | covered | none |
| delete a note in a named notebook | covered | partial: opens Trash only |
| search, demonstrating every query-language feature | covered: two journeys, tags and exclusion, titles and dates | none |
| notebooks defined by a query | covered | listed but not created |
| import from Joplin | covered | gap: no import screen; the directory chooser it needs exists |
| import from Obsidian | covered | gap: same |
| export the library | covered | gap: same |
| create a profile | covered | gap: same |
| synchronize with a replica on another drive | covered | none |
| back up and restore the library | covered: snapshot and verify; restore described | none |
| how Recoll is used | covered | none |

All twelve covered on the command line. None in the Wails GUI, which is the next
half of the work.

*A claim withdrawn.* Importing from Joplin was recorded as needing "an export
fixture this repository does not carry". It carries one:
`internal/docexec.SeedDocumentationImportFixtures` builds a minimal valid RAW
export -- one folder item and one note item, each a Markdown file whose trailing
lines hold the Joplin metadata -- and `docs/import-export.md` already documents
the import with command-line examples that G18d executes against it. The
limitation was asserted rather than checked, in an item whose entire subject is
documentation claims that nobody verified. The journey now uses the same fixture
shape and runs.

**A framing error, corrected 2026-09-03, and it changes how absences are read.**
The audit found the command line could not create a notebook or a saved search,
and both were recorded as explained asymmetries with a `surface_note`. That was
wrong, and the reason matters more than the two commands it cost.

An asymmetry is *explained* when a surface genuinely cannot do a thing.
Importing reads directories on this machine and a browser cannot; a profile
registry is about this machine and a service answering for one profile must not
reach another. Those are properties of the transport.

Creating a notebook has no such property. The capability lives once, in the
store, and REST, MCP, the interface and the command line are four adapters onto
it -- which is the whole premise of the shared-core work
`FLUTTER_GO_CLIENT.md` describes, where a future client reaches the same
application facade over a C ABI. **An absent adapter is work not done, and
writing `surface_note` beside it dresses that up as a decision.** A note in the
registry saying "the command line cannot do this" is not an explanation; it is
the gap restated in a tone that discourages fixing it.

So `notriosctl notebooks create` and `notebooks list` exist, and a saved search
follows immediately from them -- `notebooks create --query "tag:todo"` -- because
a query notebook is a notebook whose contents are whatever matches. They are one
command rather than two, because that distinction is one the storage draws and
the sidebar does not.

*The correction applies beyond these two.* Every remaining `surface_note` was
re-read against the test "could this surface do it, if someone wrote the
adapter?". The ones that survive are transport-constrained: importing and
exporting read local directories, snapshots name local paths, profiles are
machine-local, and credential migration is forbidden a REST surface by H9's own
boundary. Tagging in the interface remains what it always was -- a gap, recorded
as a gap.

**A missing command, found by the audit.** `notriosctl notes` has `create`,
`show`, `edit` and `move` and no `delete`. Deleting a note is reachable from
REST, MCP and the interface and from not the command line -- the same shape as
the tagging gap, found the same way, and it means the third task above cannot be
written as a command-line journey until the command exists. Restoring from Trash
has the same gap.

**Built 2026-09-03: the GUI can tag a note.** A control in the editor toolbar,
beside the notebook control and for the same reason -- both answer where this
note belongs, and both belong where the typing is. Tags render as chips with
their own remove buttons rather than as a comma-separated field, because editing
a list by careful deletion is how a stray comma becomes a tag nobody asked for.
Three REST calls behind it, the same ones MCP and the command line use.

*Two defects the work found in itself.* The tag panel photographed with the note
text showing through it: `md-editor-rt`'s dropdown supplies no background, and
the first capture is what revealed it -- a screenshot is a poor test of most
things and a very good test of whether something is legible. And the trigger was
a bare `<span>`, which the control crawl could not see. That was the crawl
finding an **accessibility defect**, since a screen reader would not have seen it
either; the fix was to make the element announce itself rather than to teach the
crawler to look for spans. Inventory 26 to 29.

*One background failure made quieter.* Reading a note's tags reported to the
shared error banner, so opening a note whose tags could not be loaded replaced
"Filed this note in Work" with a fetch error. A read the user did not ask for
should not overwrite feedback about something they did; it now fails silently
and shows no tags, while add and remove still report properly.

*The guard from H14 slice A has been retired, as it asked to be.* It watched the
tags asymmetry and said that if it ever stopped reporting, either the gap had
been closed -- delete it and say so -- or the check had broken. It was closed, so
it is deleted with that note in its place. Nothing is unguarded by the removal:
`doccompare`'s unexplained-gap list is pinned empty, so a capability one surface
has and another lacks cannot reappear without a written reason or a failing
test.

**Tagging in the interface.** Tags are read-only there: the sidebar lists them
with counts and clicking one searches it. Adding and removing need a control on
the note, the REST calls already exist, and
`TestTaggingGainsAGUIJourneyWhenTheInterfaceCanTag` already fails the moment the
capability is recorded without a journey to match. That test was written as a
standing instruction and this item is the work it was waiting for.

**The registry's GUI column is not an inventory, and that is a defect worth
naming (2026-09-03).** Asked for a capability-by-adapter table, the registry
answers for the command line, REST and MCP from anchored enumerations, and for
the GUI from **whatever GUI journeys happen to have been written**. It therefore
understates: it records no GUI support for attachments, remote media or jobs,
while `web/src` carries resource handling in fifteen files, localization in six
and job handling in five.

The coverage gate never caught this, and could not: it enforces that every
*surface* is claimed by a feature, and the GUI is the one surface with nothing
to enumerate. So the gate is green while a whole column is wrong. Any table
built from the registry today must say which columns are measured and which are
merely recorded -- and this is the strongest argument yet for the control crawl
below, which turns the GUI column from an assertion into a measurement.

**Measured 2026-09-03: 26 distinct controls across six views, and the table was
wrong.** `performance/v0.8-h15/gui_controls.mjs` drives the GUI headlessly,
visits six views and records every interactive element. It enumerates and never
activates: delete, purge, retire and empty-trash are all reachable here, and a
crawler that clicked what it found would eventually find one of them.

*Identity had to come from structure, not from text, and the first run showed
why.* Keyed on labels, the crawl reported 67 "controls" -- of which nineteen
were note titles, because a list of search results renders one button per note.
That count would have grown with the library rather than described the
interface. Keyed on tag, role, classes and parent, those nineteen collapse into
one control seen 36 times, and the inventory drops to 26.

*It corrected five rows of the capability table immediately.* The registry
recorded no GUI support for synchronizing, peers, recovery, the graph report or
reading notes. The crawl found **Sync now**, **Peers**, **Retention**,
**Conflicts**, **Backup & recovery**, a **Reports** notebook and **All notes**
with a search field. Each correction now names the control that evidences it, so
the claim can be checked against the inventory rather than believed.

*Two limits, stated because the number will be quoted.* The crawl measures what
the interface **presents**, not what it can do -- a Peers tab proves peers are
shown, not that one can be retired -- so the mapping from control to capability
stays editorial and the crawl only bounds the GUI column from below. And
collapsing by shape can over-merge: four groups of unlabelled `li/menuitem`
elements are almost certainly several editor menus counted as four controls.

*Two bugs in the crawler itself, both mine and both instructive.* It passed its
collector to `page.evaluate` as a **string**, which constructs the function and
never calls it -- the identical mistake the journey capture made with its marker,
already found, already fixed and already commented, repeated within the day. And
a locator failed because I read a test id from my own truncated terminal output:
`sidebar-row-snb_all_note` for `sidebar-row-snb_all_notes`. The first cost a run;
the second silently dropped a whole view from the inventory until the validator
was written to refuse exactly that.

**Give the GUI a real inventory, by clicking it.** Every other surface can be
enumerated from source and is: 61 command-line usage forms, 63 configuration
keys, 109 REST operations, 46 MCP tools, each with a pinned count that fails when
it moves. The interface has none of that. G18a records it as nine *proposed*
journeys with `measurement_state: proposed_not_executed` -- a hand-written list
of things somebody thought the GUI did, not a measurement of what it offers.

So the proposal is right, and it is worth being precise about why: driving the
interface with Playwright to enumerate its menus, buttons and controls would give
the one surface that has no machine-derived inventory the same footing as the
other four. The features page could then be held to the same coverage rule on the
interface that it already meets on the other three, and a control nobody
documented would fail a build rather than wait to be noticed.

*Three limits, so the technique is adopted for what it does rather than what it
seems to promise.* It discovers **controls, not capabilities** -- the same
distinction that makes a features page more than a list of flags, and the
mapping from one to the other stays editorial. It cannot discover what is
**absent**: no amount of clicking reveals that tagging is missing, because
absence is not a control, and that gap was found by comparing surfaces rather
than by exploring one. And a crawler must not click everything: delete, purge and
retire are all reachable, so it enumerates and describes rather than activating,
and anything destructive is recorded from its label and left alone.

**The remaining GUI gaps, enumerated 2026-09-04, then measured, then built.**
The enumeration said eighteen of twenty-nine capabilities had no recorded GUI
surface. Widening the crawl took it to fifteen, and the three that moved were
moved by looking rather than by building anything. Building the rest of this
item took it to **five**: templates and tasks, batch operations, sync
credentials, the pre-0.8 layout move, and the MCP endpoint itself. Two of those
are transport-constrained and say so in the registry; the other three are
adapters nobody has written, recorded as such rather than dressed up as
decisions. Twenty-four features have a GUI surface; twelve of them now
have a captured journey, which is the next backlog rather than this one.

*The crawl was widened first, and it changed the answer.* The registry recorded
no GUI support for **attachments**, **remote media** or **jobs**, and the
suspicion written here was that the six-view crawl simply did not go where those
controls live. That was right about the conclusion and wrong about the reason.
They are not in other *places*. They are in other **states**: a note open rather
than a notebook listed, a note in the Trash, a note that actually points at a
remote image, the sync centre on a tab other than the one it opens on. Moving
between notebooks changes which notes are listed and not which controls exist,
which is why four of the original six views turn out to measure the same shell.

So the crawl now visits sixteen states, each a short sequence of steps from a
freshly loaded page, with the notes those states need seeded by the harness and
matched by title. The inventory went from 29 controls to 52. Nothing was added
to the interface: **23 controls were there the whole time and had never been
looked at**, including `delete-button`, `restore-button`, `purge-button`,
`localize-button`, the "Upload image/PDF/resource" field in the note inspector,
and every control on seven of the sync centre's eight tabs -- pairing codes,
carrier discovery, transport setup, the profile selector, snapshot creation and
download, catch-up and reset review.

Corrected in `FEATURES.json` on that evidence: **attachments** and **remote
media** have GUI surfaces and their notes saying otherwise were wrong; note
deletion, restore and purge join **write-notes**; profile *switching* is in the
GUI while creating and forgetting profiles is not. **Jobs** is left as it was,
because the crawl does not settle it either -- job rows render only once a job
exists and nothing seeded one, so that row is unmeasured rather than known
absent, and it now says so.

*Two findings about the measurement itself, which matter more than the count.*
The first widened run reached eight of its sixteen states and the Go test around
it passed: a locator had been written from a `title` attribute that is not the
button's accessible name, so all eight sync states timed out, the crawl exited 0,
and the inventory was simply smaller. That is the identical failure to the one
this whole exercise exists to correct -- a measurement that quietly shrinks looks
exactly like an interface with fewer controls -- so the crawl now exits non-zero
on any unreached state and names each one. The second: the figures quoted in the
paragraph this replaces ("resource handling in fifteen files, localization in
six, job handling in five") counted tests and stylesheets. In source alone it is
about ten, five and two. They were used to argue the gaps were false, which they
were, but the argument was padded.

The validator pins the control count and 16 states, requires each state that was added
for a specific control to actually contain it, and requires every state that is
not a recorded empty one to show something the opening state does not. Eight
mutations were checked and all eight failed the validator: a silently unreached
state, a control leaving the state added to reveal it, a control disappearing
entirely, a state's controls moving elsewhere, a duplicate, a drift of one in
either direction, and a reverted schema. Four sync tabs -- retention,
attachments, conflicts, repairs -- are recorded as reached and empty, because a
library with no conflicts and no missing resources has nothing to show there.
That is a limit of what is seeded, written down rather than left to be
rediscovered.

*Five are file-picker work, and they are one job wearing five hats.* Importing
from a Joplin RAW directory, importing an Obsidian vault, **importing from
another Notrios library**, exporting a library, and taking a snapshot all need
the same thing: a directory chosen, a look before anything is applied, and a
report afterwards. The Notrios import was missing from the first version of this
list, which was an odd omission given that the group already contained the
export: an export nothing can read back is half a feature.

The picker is already built, and the earlier draft of this section was wrong to
plan it as new work. Wails v2 exposes `runtime.OpenDirectoryDialog`, and
`cmd/notrios/gui_wails.go` already calls it: `NativeUIBridge.ChooseSyncDirectory`
opens a native directory dialog, the bridge is bound only in the mode where this
process owns the service (`runGUI(svc.Handler, true)`, against
`runGUI(proxy, false)` for `-gui-only`), and `web/src/components/SyncCenter.tsx`
consumes it through `window.go.main.NativeUIBridge`. The work is therefore to
generalise one existing method -- a chooser that takes the dialog title and the
caller's purpose -- not to build a picker. That is the third time in H15 that an
unmeasured "missing" turned out to be present.

The chooser is the smaller half. A chosen directory has nowhere to go: REST
exposes no operation that starts an `import_joplin_raw`, `import_obsidian`,
`export_archive_v2` or `snapshot_image` job, only listing, reading, cancel,
retry and reset, and the one start route it does have is the closed, path-free
sync one. The bridge must therefore carry the job start as well as the chooser,
bound in the same mode, with the dry run and the report rendered from the job
record the existing endpoints already return. The open decision below records
why that is the right shape rather than a workaround.

None of that is a reason to leave the feature unbuilt. Running as a desktop app,
the Wails window has the same filesystem access any other desktop program has,
and that is the mode nearly everyone will use. Build it there in full: chooser,
dry run, apply, report. The browser is the narrower case, it is detectable at
runtime by exactly the check `SyncCenter` already makes -- whether
`window.go.main.NativeUIBridge` is bound -- and the correct behaviour there is to
grey the affected menu items and controls out with a short reason. Detect the
mode; do not lower the desktop app to the browser's ceiling.

**Import and export built 2026-09-04.** Five operations behind one control:
import from Joplin, from Obsidian and from another Notrios library; export this
library; take a snapshot. They reach the core through `NativeUIBridge` in
`cmd/notrios/gui_transfer.go` rather than over HTTP, because no REST route
starts them, and the bridge is bound only when this process owns the store --
`runGUI(svc.Handler, svc)` against `runGUI(proxy, nil)`. That makes the browser
case structural: no bridge, no methods, and the header control renders disabled
with the reason instead of disappearing.

*Three narrowings, each made deliberately and commented where it was made.* The
GUI exports the **whole** library, because choosing a subset by notebook, tag or
query means seeing what it selects first, and blank selector fields next to a
folder chooser would produce exports nobody can predict. The Obsidian dry run
writes **no** import configuration into the vault, unlike the command line's,
because silently writing a file into somebody's vault in exchange for looking at
it is a poor trade. And importing from Notrios offers **merge only**:
`archivev2` has four restore intents and three of them are database-universe
surgery -- replace overwrites this library with the archive's identity, adopt
requires an empty target, fork mints a new database -- so a dropdown offering
all four beside a folder chooser would let somebody replace their library while
believing they were adding to it. Merge keeps this library's identity and admits
the archive's records as foreign, which is what "import from another Notrios
library" sounds like it means.

*The Notrios import is also the best-behaved of the five.* Its look-before-you-
apply step is `archivev2.VerifyDirectory`, which opens no database at all, where
the importers' dry runs simulate a write. `Restore` runs the same verification
again before its first write and refuses on failure, so declining to look first
is safe: looking is for the person, not for the machine.

*Six mutations were checked and all six failed a test.* The header control
hidden rather than disabled; verification swapped for the merge it exists to
precede; the dry-run flag flipped from true to false; the chooser handed the
wrong purpose. That last one caught a real defect rather than a hypothetical:
generalising `ChooseSyncDirectory` into `ChooseDirectory(purpose)` left
`SyncCenter` calling a method that no longer existed, and its test passed anyway
because the test mocked the old name. The sync test now asserts the purpose and
not merely the call.

*One limit, recorded rather than left to be found.* The crawl is a browser, so it
can see the Import/Export control and cannot open what is behind it. The
inventory now records whether each control was ever found enabled, and the
validator requires this one never to be -- an enabled one would mean the bridge
gate had broken open and a browser was being offered an operation it cannot
perform. The modal's own controls are covered by the frontend tests, which
render it directly. Twelve capabilities now have no GUI surface, down from
eighteen.

**The publication handoff has no consumer, tracked 2026-09-04.** Notrios can
produce a publication and nothing can read it. `PUBLISHING_POLICY.md` divides
the work deliberately -- Notrios owns selection, private and draft exclusion,
resource reachability, link rewriting for withheld targets, metadata stripping
and a checksum-verified handoff; the separately maintained MIT-licensed
`movenotes-v3` owns the projections, "Obsidian vault output for interoperability
and Quartz", Hugo/Ledger, Pagefind and Bluge -- and it names `notrios2sql.py` as
the consumer. That script was never written.

The rest of that pipeline was. `movenotes-v3` already has `joplin2sql.py`,
`obsidian2sql.py`, `sql2obsidian.py` and `obsidian2site.py`, with tests: every
stage after the first. So the gap is one adapter, not a missing capability, and
the boundary this repository documents does not need changing to close it.

*Why not emit an Obsidian vault from here instead.* It was raised, and the
motivation is sound -- more tools publish from vaults than from anything
Notrios emits. Against it: it duplicates `sql2obsidian.py`, obliges this
repository to own wikilink rewriting and frontmatter mapping permanently, and
weakens the one guarantee publishing has. `publish run` binds a publication to
the digest somebody reviewed, re-planning and refusing on a mismatch, then
checking the written manifest against it. A vault has no archive-v2 manifest, so
a direct emitter would need a weaker integrity story for the single output that
rewrites note content. `movenotes-v3/NOTRIOS_IMPORT_PLAN.md` records the work
and its decisions; this entry exists so the dependency is visible from here
rather than only from there.

*Two are the query language appearing where it already belongs.* A **search
notebook** is a saved query, and the GUI already has the box that takes that
query. Creating one should be an action on a search that ran -- "keep this as a
notebook" -- rather than a separate form, because a query someone has just seen
work is the one they want to keep. **Live query blocks** are the same language a
third time, inside a note.

*Four are ordinary editing surfaces the GUI simply lacks.* Renaming a tag
hierarchy, managing collections, templates and tasks, and batch organiser
operations. Each has a REST surface and no interface.

*Two are reporting rather than doing.* **Keeping a library healthy** -- lint and
garbage-collection reports -- and **publishing**, whose review step is the whole
point and is better suited to a screen than to a terminal.

**Collections: show the origin, and let a search ask for it. Added 2026-09-04.**
A collection is a note's provenance -- `joplin-raw-2026-07`, `twitter-archive`,
`research-pdfs` -- and the place where `write`, `resources`, `publishing`,
`graph`, `remote media` and `mcp` capabilities are declared. Two things are
missing, and only one of them is interface work.

*The note does not say where it came from.* The inspector shows an ID and a
revision. Adding the collection beside them is small and is most of what a
person wants from this capability: a note that has been silent about its origin
starts answering.

*No surface can filter by it.* `category:` is an alias for `notebook:`, not for
collection, so "show me what I imported from Joplin" is unanswerable from the
search box, the command line, REST and MCP alike. `collection:"..."` has to
agree across `SEARCH_QUERY_LANGUAGE.md`, the parser, the SQLite join and the
Recoll compiler, and the front-matter projection needs the field or a Recoll
hit cannot match on it. That is the substantial half and it is not GUI work at
all.

*Not a tag, and the reason matters.* Using a tag to mark provenance was
considered and rejected. A note has exactly one collection and many tags, so a
tag permits two provenances, which means nothing. Tags are editable, so
provenance would become something a person can rewrite by typing, when the
whole value of "this came from a Twitter archive" is that the note cannot say
otherwise. And a tag carries no capabilities, so it would describe the same
notes while enforcing none of the rules that are the point.

*Creating and reconfiguring collections stays on the command line*, with the
other writes, and the sidebar gains nothing: most libraries hold exactly one
collection, and a permanent heading listing one item is clutter.

*Two are genuine boundaries and should be recorded as such rather than built.*
`notriosctl migrate` relocates the directories the running program is serving,
which a program cannot sensibly do to itself. **Choosing where sync keys are
kept** stays command line only because H9 forbids a credential REST surface, and
the Wails GUI reaches the core over exactly that surface -- so this is a
boundary for the GUI *as currently built*, and would stop being one if the GUI
reached the core through the C ABI instead. Worth saying plainly, because it is
the first case where the shared-library work would change what a surface can do.

*One is not a capability a GUI hosts.* **Letting an AI assistant use your
library** is the MCP endpoint. A GUI can show that it is on, and which scopes are
granted, and that is worth doing -- but the endpoint is not something the GUI
offers a person.

**Scope.** Add tag add and remove to the interface, with a journey and captured
screenshots. Add `notes delete` and `notes restore` to the command line. Write
the missing journeys on both surfaces for the twelve tasks, including a query
journey that demonstrates each query-language feature with a worked example.
Build a Playwright control crawl that enumerates the interface and reports
controls no feature claims. Record, for every feature, whether each surface
supports it -- which the registry already holds and the comparison already
prints, so this is filling it in rather than inventing it.

**Boundaries.** No journey is written for something a surface cannot do. The
crawler enumerates and does not activate anything destructive. Screenshots come
from the seeded fixture and never from a real library. No prose is generated by a
model; H14 recommended against that and the recommendation stands.

**Dependencies.** H14's catalogues, coverage gate, comparison and capture. The
GUI tagging work depends on nothing else.

**Working state.** Every task above is covered on both surfaces or recorded as
impossible on one with the reason; the interface can tag; the crawler produces a
control inventory with a pinned count; and the features page claims every control
the crawler finds.

**Validation and evidence.** Every command-line journey executed with its
postcondition; every interface journey captured with markers derived from the
elements clicked; the control inventory with its count and the list of unclaimed
controls; and the coverage table above, regenerated rather than retyped, so it
cannot drift from the catalogues. Evidence under `performance/v0.8-h15/`.

**Progress 2026-09-03: the command-line gaps, and a bug they exposed.**
`notriosctl notes delete` and `notes restore` exist, closing the third row of
the table above. Delete prints the exact line that undoes it, so a person who
changes their mind does not have to go and find out how. There is no
confirmation flag: this is not a purge, the note goes to Trash and stays there
until Trash is emptied, and asking someone to confirm a reversible act teaches
them to confirm without reading.

*It exposed a bug in a command written three commits earlier.* `notes show`
used `GetDocument`, which excludes notes in Trash, so a just-deleted note
reported as **"no note"** -- the same conflation `tags list` had already been
fixed for, and worse here, because it is the answer someone gets seconds after
deleting while trying to confirm what happened and find the id to restore. The
`trashed_at` field that command already printed was dead code until now. It
looks in Trash as well, and the journey asserts a trashed note is still
readable.

*Two journeys added, and a fact for readers found by writing one.* Delete and
restore, and importing an Obsidian vault -- which showed that **front-matter
tags do not become Notrios tags**. The documented tag handling concerns Joplin
RAW, which carries explicit tag files, so this is recorded as something a reader
needs to know rather than claimed as a defect: the journey says to add them
afterwards with `tags add`. Twelve command-line journeys now, and the
features-without-a-journey ratchet drops to 21.

**The same framing error again, and a more useful boundary in its place
(2026-09-03).** Importing was recorded as command line only, with the
justification that it "reads directories on this machine, which a browser cannot
do and an API should not". That reasoning describes the **browser** mode. The
GUI is a Wails desktop application -- `./bin/notrios` is the app and its local
service together -- and a desktop application can open a native directory
picker. `FLUTTER_GO_CLIENT.md` says as much: "shared directories and pickers"
and "file-picker handoff" are capabilities the planned ABI must expose.

So importing from Joplin, exporting, snapshots, profiles and publishing are all
**gaps in the GUI rather than boundaries of it**. What is genuinely constrained
is the browser mode, and that is a property of a delivery mode rather than of a
surface. Every one of those notes now says so.

*What this leaves as a real boundary.* Almost nothing, which is the honest
answer. `notriosctl migrate` stays command line only for a reason that survives
the test: it relocates the directories the running program is using, and a
program cannot sensibly do that to itself while serving them. Credential
migration stays command line only because H9's boundary forbids the surface, not
because the GUI could not host it. Everything else is work.

*Terminology, corrected at the same time.* These pages used "the interface" to
mean the GUI, in a product with four surfaces, producing sentences like
"neither the command line nor the interface; reachable only over REST or MCP" --
which says a capability is not reachable from an interface, only from two other
interfaces. All eighteen uses now say "GUI", which is the term the product's own
`docs/gui.md` uses.

**Progress (2026-09-08).** H15-H and H15-I are done; H15-G is blocked on work
that is not journey writing.

*Command-line journeys: sixteen features lacking one, down to six.* Ten were
written and every one of them runs -- reading a note and what it is made of,
tags, templates and tasks, the graph, links, saved searches, jobs, the pre-0.8
migration, remote media, and publishing.

**The six that remain are not one backlog, and calling them one was the mistake
in the first telling.** Reviewed 2026-09-08:

- **`batch-operations` is a gap, and H17 closes it.** That item makes `batch`
  reachable from a terminal; a journey follows from the command existing.
- **`query-blocks` is a boundary.** A query block is a rendering *inside a
  note*: the note carries a fenced query and the interface shows what it matches
  in place. At a terminal the same question is `notriosctl search`, and the
  formatting is a template tool's job. A command that ran a block's query would
  be a second way to run a query, which is the duplication this milestone keeps
  removing.
- **`mcp-endpoint` is a boundary.** It is a served surface, used by software
  with an MCP client. Its command-line journey would be "start the service and
  connect something else", which documents the client rather than this program.
- **`attachments` is a boundary for the half that is missing.** Attaching means
  placing a `resource://` link at a point in the body that the author chose, and
  a command that put it somewhere of its own choosing would be guessing at the
  one thing only the writer knows. The reading half is the right command-line
  scope. *If* an attach command is ever wanted, the shape that does not guess is
  bytes in and a URI out -- `resources add` printing the `resource://` link for
  the author to place with `notes edit` -- and that is a decision for whoever
  wants it rather than a gap in this item.
- **`sync-peers` and `sync-recovery` are blocked on the harness, and the reason
  first given here was wrong.** It said the runner offers one `{db}`; it offers
  `{sandbox}` too, and `Substitute` replaces anywhere in an argument, so a
  second library at `{sandbox}/second/notes.sqlite` is expressible today. The
  actual obstacle is the pairing ceremony: `sync invite` prints a one-use code
  that is deliberately carried out of band, and the runner captures only
  `document_id` from a step's output, so no later step can spend it. That is
  liftable and worth lifting -- let a step name a value to capture from its JSON
  -- and it is the only one of the six that is a limitation of the tooling.

*Writing them found a defect.* `notriosctl migrate --json` printed prose on all
three "nothing to migrate" paths -- which is the *common* path, since most runs
have nothing to migrate. A caller that asked for JSON received a sentence on the
branch it was most likely to take. Fixed: the three answers are reported as
`{"migrated": false, "reason": …}` when `--json` is set, and unchanged
otherwise.

*GUI journeys: blocked, and not on effort.* Of the twelve features with a GUI
surface and no journey, **eleven have no `data-testid` anywhere in the H15
control crawl.** The crawl pinned 59 controls and only 22 carry one, and the
target surfaces -- collections, attachments, the graph, links, query blocks,
jobs, profiles, and four sync tabs -- are almost entirely in the other 37. Only
`remote-media` is addressable today, through `localize-button`.

A journey against an unaddressable control has to locate it by shape, which is
the brittle form this catalogue exists to avoid, and a journey added without
being captured under `NOTRIOS_GUI_JOURNEYS=1` would be a claim with no evidence
behind it -- the failure this milestone was created to stop. So the prerequisite
is making those surfaces addressable in `web/src`, which is interface work
rather than documentation work, and this slice waits on it rather than
pretending the obstacle is time.

**Open decisions**

- **(resolved 2026-09-03, confirmed by the user 2026-09-08) Whether the command
  line gets `notes delete` -- was blocking for one journey.** Deleting was
  reachable from REST, MCP and the GUI and not the command line. Recommended and
  accepted: add `notes delete` and `notes restore`, because "delete a note in a
  notebook" is one of the tasks this item exists to document, and because a
  delete that can only be undone through a different surface is a poor boundary.
  Both commands exist and inherit the Trash semantics rather than inventing
  anything.

  Two entries for this decision stood here for five days -- the resolution and
  the original question, unchanged -- which is how a resolved decision comes to
  be answered twice. The resolution is the record; the question is kept inside
  it because the reasoning is the part worth having.

  It closed less than it appeared to. Neither command was added to
  `notriosctl help`, so the gap this decision existed to close is still
  invisible to the coverage gate that measures such gaps. H23 fixes that.
- **How much of the query language one journey should demonstrate -- Decided
  2026-09-08: one journey per idea.** Text, tags, notebooks, dates, negation,
  grouping. A reader looking up how to exclude a tag should not have to read
  eleven other examples first, and each becomes separately executable, which a
  twelve-step journey does not: a single failing step there takes eleven working
  ones down with it and says nothing about which idea broke.

  **And a GUI journey does not mirror a command-line one.** They answer
  different questions -- the command line's is "what do I type", the GUI's is
  "where do I click" -- and a catalogue that paired them would either invent GUI
  steps for capabilities the GUI does not have, or hold the command line back to
  what a browser can reach. Import and export are the standing example: their
  controls are greyed out in a browser because no REST route starts those jobs,
  so a mirrored journey would be a screenshot of a disabled button.
- **What an import, export or snapshot control does in a browser -- Decided:
  greyed out with the reason.** This entry has been wrong twice, so it records
  what was checked rather than what seemed reasonable.

  The first draft recommended greying out because a browser cannot open a native
  dialog. The second flipped to "render the path field anyway", reasoning from
  `SyncCenter` that a typed path costs nothing. Both arguments were about paths,
  and paths are not what settles it: **the REST surface has no operation that
  starts an import, export or snapshot job at all.** `api/openapi.yaml` exposes
  `/api/v1/jobs` for listing, `{job_id}` for reading, and cancel, retry and reset
  -- and one start, `/api/v1/jobs/sync/start`, whose description names it "the
  exception to watching-only" precisely because its controls are closed and
  path-free. The four job kinds in question (`import_joplin_raw`,
  `import_obsidian`, `export_archive_v2`, `snapshot_image`) have no start route,
  and the listing endpoint deliberately never returns their parameters because
  those "may name places on the local machine".

  So the control is not unavailable in a browser as a matter of taste. The
  capability is genuinely unreachable over the only surface a browser has, and
  offering a path field would mean *adding* the path-accepting HTTP surface the
  API has deliberately declined to have -- the same class of decision as H9's
  refusal of a credential REST surface. Grey it out and say why.

  `SyncCenter` is not a counter-example. It does accept a directory over HTTP,
  and `internal/httpapi/sync_ui.go:83-94` gates that whole surface on the caller
  being loopback, answering 403 `loopback_only` otherwise. The existing rule is
  therefore that a path-accepting surface is restricted to a caller on the same
  machine -- which is the rule to follow if any of this is ever exposed over
  HTTP, not an argument that it already is.

  **Consequence for the build, and it is the larger half.** Generalising
  `ChooseSyncDirectory` gets a chosen directory and nothing to do with it: the
  desktop app must also *start the job*, and cannot do so over REST. The bridge
  therefore gains both -- a chooser and a job start -- bound only in the mode
  where this process owns the service, exactly as `ChooseSyncDirectory` already
  is. That is what makes the greying-out structural rather than cosmetic: in
  browser mode neither method is bound, so the capability is absent rather than
  merely hidden.
- **Whether creating a search notebook is an action on a search or a form --
  Non-blocking, decide before building it.** Recommended: an action on a search
  that has just run. The query someone wants to keep is the one they have
  watched work, and a separate form asks them to retype it and to be right the
  second time without feedback.
- **Whether the control crawl becomes a gate or stays a report -- Decided
  2026-09-04: the crawl stays a report, and the *inventory* became the gate.**

  Running the crawl in `make validate` is refused, and the reason is not
  flakiness. It needs Playwright, a browser, built web assets and a service
  seeded with particular notes; on this machine Playwright is not even a
  dependency of this repository -- it is borrowed from another checkout's
  `node_modules` through `PLAYWRIGHT_MODULE`. `make validate` has to stay
  runnable offline and cheaply, and a gate that needs all of that is a gate
  somebody switches off.

  That decision leaves exactly one hole, and it is the hole the crawl exists to
  close: the inventory is a committed file describing an interface that can move
  without it. Somebody adds a button, never runs the crawl, and every check
  keeps passing against a measurement of something that no longer exists. So the
  inventory now records an **interface signature** -- a digest of the things
  that decide what the crawl would find: test ids, interactive elements, and the
  roles that make a non-element behave as one. The validator recomputes it from
  source with no browser. Prose, styling and comments do not move it; adding,
  removing or renaming a control does, and the build then says to run the crawl.
  Checked both ways: an added `<button>` fails it, a comment does not.

  *It found something on its first run, which is the argument for it.* The
  library-health button in the header was added in this same item, in a commit
  after the last crawl, and the pinned count went on passing at 58 while the
  interface had 59 controls. Nothing could have noticed. The count is 59 now,
  and the signature is what will notice the next one.

  The duplication -- the signature is computed in Python for the validator and
  in JavaScript for the crawl -- is deliberate and its failure mode is safe: two
  implementations that drift produce a mismatch, which stops the build and asks
  for a crawl, rather than passing quietly. Both were run against this tree and
  agree byte for byte.
- **Whether interface journeys should cover command-line-only tasks by pointing
  at them -- Non-blocking.** Importing, exporting, snapshots and profiles have no
  interface equivalent. Recommended: the interface page names them and links to
  the command-line journey rather than staying silent, because a reader who
  cannot find a task does not conclude it is command line only; they conclude it
  is missing.

### The desktop harness captures as well as asserts -- Done

**What was blocked, and by what.** Four capabilities had a GUI surface and no
GUI journey: importing, exporting, snapshots and publishing. The recorded reason
was not laziness -- the journey capture drives a browser, and in a browser those
controls are correctly disabled, because every one of them names a folder on the
machine that owns the library and no REST route starts them. The panel could not
be reached to photograph. The desktop harness could already drive the real
application under Xvfb; it could only assert. Now it captures.

**How a desktop journey stays falsifiable without a DOM.** A browser journey
points at a locator, and a locator that stops matching fails the run rather than
producing a confident picture of the wrong place. There is no DOM to query from
xdotool, so that property is obtained another way: every desktop step declares
the lines it expects in the application's own transcript, and those lines name
the operation *and* the path it ran against. A Tab that stops one control short
cannot satisfy `joplin-apply requested for /tmp/...`, so the journey fails
instead of photographing the wrong panel. The catalogue marks these journeys
`driver: "desktop"`, the Playwright runner skips them and keeps their rows, and
the page labels them "desktop app only" -- a reader in a browser cannot do them.

**Three faults the pictures found, none of which a passing test would have
shown.**

*The first pictures were of the state before each step.* Three journeys that
open the same dialog produced the same photograph three times, byte for byte,
because a picture taken before the keystroke shows the state you left rather
than the one the step produces. Desktop steps are now photographed after they
act and after the application says the act finished. The manifest merge had the
same shape of bug: keyed by step, it left a row behind whenever a journey lost
one, so the file claimed a screenshot that no longer existed. It replaces by
journey now.

*The export report read `documents: 0`.* The capture library was empty, so the
picture showed the feature working on nothing. It seeds the same fixtures the
browser capture uses, and the report now reads `documents: 23 · objects: 24`.

*Publishing was not reachable, and the reason was a defect in the product.*
After **Review what this would publish**, the review replaces the button that
had focus, focus falls back to the body, and the next Tab starts again at the
dialog's close button -- so somebody working by keyboard is thrown back past
every other operation in the dialog to reach the one field the review has just
asked them for. The folder typed after a review went nowhere at all, which is
how it was found. The panel now leaves the caret in the destination field when a
plan arrives, and `publish-panel.test.tsx` fails without that. The earlier record
said the panel could not be reached by keyboard at all under xdotool; the
narrower truth is that it could, and what stopped it was this.

Twelve images, four journeys, `docs/journeys-gui.md` regenerated, and four
`surface_note` entries corrected -- they said no journey could be written, and
that is no longer true. `TestDesktopPublishThroughTheWindow` is deleted rather
than left skipped: it existed to drive publishing through the window, the
capture now does exactly that, and a permanently skipped placeholder beside
working code reads as an absence that nobody has got to. What it actually knew
-- that synthetic Tab moves focus without dispatching a DOM keydown, so the page
cannot help a keyboard walk -- is recorded where that constraint is now lived
with, in the capture itself.

*One thing the pictures show that is worth a second look.* The snapshot report
reads `objects: 0` beside an export reading `objects: 24` -- both are correct
(a snapshot counts asset-store objects, of which the fixture has none), but a
reader comparing the two panels has no way to know that. Recorded rather than
adjusted.

### Saving stays explicit, and the unsaved draft is protected -- Done

Joplin autosaves and has no save button, and the obvious question is why this
does not. The answer is in the store rather than in taste: every save writes a
`document_revisions` row, revisions replicate (`sync_revisions.go`,
`sync_revision_apply.go`), and nothing prunes them -- the only statement that
removes them is the purge. Autosaving here would turn a morning of typing into
a hundred synced revisions of the same note, on every device. So the button
stays, and it correctly reads "Save revision".

That choice creates the obligation this step discharges: if work can be
unsaved, the interface has to protect it rather than let a click throw it away.

- **It says so.** An "Unsaved changes" chip sits beside the save button
  whenever the editor differs from the note as it was loaded. A read-only note
  can never be dirty, so the chip cannot appear on one.
- **Nothing replaces the editor without asking.** Opening another note,
  starting a new one, and reloading a note the service rewrote all go through
  one guard that names the note and says what is discarded. A refusal reaches
  the service not at all -- the same rule the Trash confirmations follow. The
  two confirmations that destroy a note say that unsaved changes go with it.
- **A reload finds the work again.** The draft is written to `localStorage`
  under `notrios.draft.v1`, and restored on start: if it belongs to a saved
  note, that note is loaded first, so the restored text saves as a revision of
  it rather than as a second note with the same words in it. A draft outranks a
  startup deep link, because the link names something safely in the store and
  the draft exists nowhere else.
- **It does not promise what it cannot do.** A draft too large for browser
  storage, or storage that refuses the write, changes the chip's tooltip
  instead of being silently dropped.

The draft is the reader's own note text, kept in their own browser, sent
nowhere, and removed the moment it is saved or discarded.

**The window's own close button.** `beforeunload` covers a reload and a closed
browser tab. It does not cover the desktop window: on Linux the title bar's
close is a GTK delete-event that Wails turns straight into a quit, and File →
Quit takes the same route. `OnBeforeClose` is the only place that question can
be asked, and it is answered in Go -- so Go has to be told, which is what the
`WindowState` binding is for. It carries one boolean and nothing the person
wrote, and it is bound in *both* modes rather than only where this process owns
the store: a `-gui-only` window edits notes too and its close button is just as
final, and the object reaches no store, no filesystem and no network.

An unanswered dialog fails closed -- an error showing it, or an Escape, keeps
the window open, because the cost of that is a second click and the cost of the
other choice is the person's work. The dialog names no note, which keeps
content out of that layer and avoids a hazard worth recording: on Linux Wails
passes the message to `gtk_message_dialog_new` as the *format* string, so a note
titled "50% done" would make GTK read an argument nobody passed.

Driven for real by three tests against the running application, which read its
own transcript for what it decided rather than inferring it from pixels:
`TestDesktopCloseAsksBeforeDiscardingUnsavedWork` types, asks the window
manager to close (openbox's Alt+F4, a true WM_DELETE_WINDOW), waits for the
native dialog, dismisses it, and checks the application is still there -- then
undoes the work and closes with Ctrl+Q, because a guard that could refuse but
never accept would trap somebody in a window they cannot close.
`TestDesktopCloseProceedsWhenAskedTo` answers the dialog yes and checks the
window goes: what counts as "yes" is a string GTK chooses, not this code.

**What the dialog may promise, measured.** The first wording offered to discard
the work. That was wrong, and finding out why was the useful part.
`TestDesktopKeepsTheDraftAcrossACrash` kills the application outright and
starts it again: the draft comes back, because WebKitGTK keeps localStorage in
a SQLite database under the data directory and that survives a killed process.
So closing the window discards nothing, and the dialog now says the changes
will be waiting -- which also avoids a race that had no good answer, since
clearing the draft from `OnBeforeClose` would be a message to a webview whose
process is already leaving.

That test failed first, and its failure was the harness rather than the
product: it killed the application milliseconds after the window reported the
change, before the debounced write had happened at all. It now waits for the
draft's key to appear in the webview's storage file before killing anything --
the precondition stated as itself rather than as a duration somebody guessed.
The residue is a real limit and is documented: a machine that dies in the first
moment after a keystroke can lose the last few words.

Covered by `web/src/__tests__/draft.test.ts` (storage, damage, refusal) and
`web/src/__tests__/draft-protection.test.tsx` (the three properties above, in
the app shell). One consequence for the journeys: each runs in a fresh browser
context, so a dirty editor in one cannot silently decline a click in the next.

## H17. Act on many notes at once, from the search results and from a query

**Goal.** Make `batch` reachable by a person and by a script. The capability
exists on REST as `POST /api/v1/batch` and on MCP as `run_batch` under the
organizer scope; neither the interface nor the command line can call it, and
each needs a different missing piece first.

**What batch already is.** Move, add_tags, remove_tags, trash, restore and
duplicate over an explicit list of notes, bounded at 500 and refused rather
than truncated. Every item reports `applied`, `skipped`, `failed` or
`rolled_back`; `mode` changes what a failure does and never what the report
says. A run that happened is a 200 even when every item failed, because
per-item failure is the report's content rather than the request's fate.
`request_key` makes a retry safe, and a key reused with different arguments is
refused rather than answered from the earlier run.

### The interface needs multi-select, not a batch screen

Batch is what the feature calls, not what it is. The missing primitive is
selecting more than one note: `SearchPane` tracks a single `selectedDocumentID`
and renders each hit as a button that opens it.

The shape to follow is Joplin's, which is well understood by anyone migrating.
Selecting several results replaces the editor and preview with a panel of the
operations that apply to a set -- tag, move to a notebook chosen from a
dropdown, duplicate, delete, copy links -- rather than opening a note nobody
asked to read.

*One driver does not carry over.* In Joplin, bulk move is partly repair: the
interface can leave you in a different notebook than you think, so notes land
in the wrong place and are moved in a batch afterwards. This interface already
guards against that specific failure -- the notebook control shows the open
note's own notebook rather than the sidebar's selection, deliberately. Bulk
move here is ordinary reorganisation, not a workaround, which lowers its
urgency without removing the need.

*What selection means for the editor -- half settled.* Selecting notes and
having the note you were reading disappear is abrupt if it was unsaved. The
unsaved half is now handled everywhere else in the editor: one guard asks
before anything replaces its contents, and the draft survives a reload. The
multi-select panel must go through that same guard rather than around it, which
leaves one question of its own -- whether leaving the selection returns to the
note that was open, or to an empty editor.

### The command line needs a way to name a set

A batch over an explicit list of ids is unusable from a terminal unless
something produces ids. **That was this item's blocker and it is gone.** When
this was written there was no `notriosctl search`, "by an existing decision that
reading is what the interface and the API are for" -- a decision H21 found was
never made, only written down afterwards to explain an absent adapter. H19 built
the command, H21 built the reading commands beside it, and a search now returns
each hit's identifier. The command-line half of this item starts from a
`notriosctl search` that exists.

That also makes `batch-operations` reachable for a command-line journey: it is
one of the six features H15-H could not cover, and the only one of those six
that is a gap rather than a boundary. Closing this item closes that too.

So the useful form is a query rather than a list: select with the same query
language the search box takes, then act. That keeps one language across
surfaces and avoids adding a general search command as a side effect.

It must show the selection first. Every destructive or wide-reaching command
here already works that way -- `tags rename` dry-runs by default, the importers
scan before writing, `publish run` refuses unless the reviewed digest still
matches -- and a batch that moved forty notes because a query matched more than
its author expected is exactly the failure that pattern exists to prevent.

### Open decisions

- **Whether the command line grows `notriosctl batch` or the operations grow a
  `--query`.** One command with a `--query` and an operation argument keeps the
  vocabulary in one place; `notes move --query` spreads it across the commands
  that already exist and reads more naturally for each one.
- **Whether the interface uses `request_key` at all.** It matters for a client
  that can be interrupted and retried. A window that has just issued one
  request and is waiting for its report may not need it, and a key generated
  per click is a key that never gets reused.
- **Whether `export` belongs in the panel.** Joplin offers it. Exporting a
  selection here means a publication or an archive subset, both of which name a
  folder and are therefore desktop-only, so it would be the one item in the
  panel that is sometimes absent.

## H16. Reconcile the collection model with what is actually stored — complete

**Goal.** Decide what a collection is, then make the schema, the API, the
documentation and the code agree. Everything below was found while making
`--collection` work and is deliberately left for a decision rather than patched
in passing.

**Why a step of its own.** H15 fixed a bug: `documents.collection_id` is a
foreign key, nothing but bootstrap, sync and restore ever inserted a collection,
and so every importer's `--collection` flag failed on the constraint with
`sqlite step rc=19: FOREIGN KEY constraint failed` for any value but `default`.
That is now fixed, and fixing it surfaced a set of disagreements that are not
bugs so much as unmade decisions.

**The model, as clarified during H15.** Notes are imported into a *notebook*,
which is what a person browses and searches. A collection is provenance: a note
carries at most a collection identifier, and the collection row is information
about that identifier. The two are not alternatives, and the earlier reading of
collections as somewhere notes live was wrong.

### What disagrees

*The schema is smaller than its documentation.* `DATABASE_SCHEMA.md` describes
`kind` (`managed`, `external`, `imported`, `sidecar_indexed`, `projection`),
`capabilities_json` and `settings_json`. The migration has `id`, `name`,
`description` and `created_at`. Three documented fields do not exist.

*So the API reports fields the store cannot hold.* `api.Collection` carries
`kind` and `capabilities`. `kind` is answered `managed` for every row because
that is the only honest answer available, and `capabilities` is a hard-coded
list in `ListCollections` -- `documents, search, resources, links, graph` --
identical for every collection. A client cannot distinguish a read-only archive
from a managed library, which is the distinction the field exists for.

*Two REST handlers were stubs and nothing noticed.* `POST /api/v1/collections`
validated its input and echoed it back as 201 without touching the store, and
`GET /api/v1/collections/{id}` answered `default` from a placeholder whose
description read "Placeholder collection for scaffold validation" and 404'd
everything else as "not available in scaffold server". Both are fixed. The
reason they survived is worth keeping: no test read a collection back after
creating one, so a handler that echoed its input looked exactly like one that
worked.

*Capabilities are declared nowhere and enforced nowhere.* The capability list is
the reason the concept earns its place -- an imported archive that is searchable
and linkable but not writable -- and no code consults it. Nothing refuses a
write because a collection says it is read-only.

### Decisions, answered 2026-09-04

**`kind` and capabilities are removed from the API rather than stored.** A
capability nothing checks is a comment in a database, and this one is worse than
that: `kind` is answered `managed` for every row and `capabilities` is a
hard-coded list identical for every collection, so both fields are ceremony that
reads as a contract. They go from `api.Collection`, from the OpenAPI schema
(where `kind` and `capabilities` are currently *required*), and from MCP's
`list_collections`; the documented columns that never existed go from
`DATABASE_SCHEMA.md`. A collection keeps `id`, `name`, `description`,
`created_at` -- which is all the store has ever held.

**`external` and `sidecar_indexed` are vocabulary, and stop being written down
as a plan.** Checked rather than assumed: the Recoll sidecar indexes exactly one
directory, and it is the projection of this library's own notes
(`internal/recoll/recoll.go` writes `topdirs = <projectionDir>`). Recoll is a
different way to *search Notrios notes*, not a way to browse foreign material as
notes. Nothing else indexes anything Notrios does not own, so no code will ever
produce a collection of either kind, and the enumeration listing them
disappears with `kind` itself.

**A collection is neither deletable nor renameable in the sense that matters:
where a note originated does not change.** The identifier a note carries is a
statement about the past, and the past is not editable; the foreign key that
made this question unavoidable is right to exist. What follows is that there is
no `DELETE /api/v1/collections/{id}` to write and no identifier rewrite to
support -- not that the feature is missing.

**`--collection` stops being a scope on `fix`, and stops being a scope on
`export archive-v2`.** Two different reasons.

*On `fix`, a collection is not a scope but a repair.* The only safe change a
repair can make to a collection identifier is to resolve one that no longer
names anything, and the answer for a note whose provenance is unusable is to put
it where Notrios can act on it -- `default` -- having first ensured the note
satisfies the schema Notrios requires of its own notes.

*On `export archive-v2`, the collection is carried, not selected on.* Every note
goes into the archive with the identifier it has, and a note with none came from
Notrios. The format already does this: each document, resource and source bundle
record carries its own `collection_id` (`internal/archivev2/export.go`), and the
manifest lists the collections the records actually used.

### What answering them exposed, and it is larger than the questions

Verifying the fourth answer against the code found the same defect in four
places, and only one of them had ever been noticed.

**An unspecified collection silently means `default`, and then filters.** Not in
one path -- in every path that selects notes other than search:

| Path | Where | What it means today |
|---|---|---|
| `lint` | `sqlite_lint.go:32-34` | reports on default-collection notes only |
| `fix` | `sqlite_fix.go:25-27` | repairs default-collection notes only |
| selection (`export archive-v2`, publication) | `sqlite_selection.go:59-61` | `--target full_archive` archives default-collection notes only |
| search | fixed in H15 | now spans every collection |

So a library with any imported collection has a **"complete backup" that is not
complete**, a workspace lint that reports a clean library while another
collection rots, and a repair that cannot reach the notes that need it. The
Library Health panel in the interface inherits all of it, because it calls lint
and fix through the bridge. Nothing warns; the flag defaults to `default` and
the report says `collection_id: default` as though that had been asked for.

This is the same defect the search fix removed in H15, in the paths nobody
re-read afterwards -- which is the more useful finding: the bug was never
*about* search.

### Open decisions that follow

- **Whether "no collection named" means every collection -- Decided
  2026-09-04, including publication, and built.** Someone who migrated from
  Joplin would want lint, fix and selection to work on their notes even though
  the collection says the notes came from Joplin. That is the whole answer: the
  collection records where a note came from, not whether the tools apply to it,
  and a migrated library is one library.

  Publication takes the same widening rather than an exception. A profile's
  selectors are what limit a publication, and they say notebooks, tags and
  queries -- not provenance; a profile that means provenance can say
  `collection:` and be read literally. The safety net stands where it was:
  `publish run` re-plans and refuses unless the reviewed digest still matches,
  so the counts are read before anything is written. `docs/publishing.md` now
  says a selector written earlier may match more than it did, because that is
  the sentence somebody needs before their next review.

  *An unspecified collection is now every collection wherever a read is
  scoped*, which turned out to be more places than the four this item found:
  lint, fix, selection, graph report, graph export, tasks, templates, live
  query blocks, link suggestions, link checking, and the title and filename
  lookups that resolve `[the plan](Kitchen)`. The last two were found by the
  test suite rather than by reading -- widening the editor's link checking
  without them turned a resolvable link into an unresolved one, which is the
  kind of half-done change that a mechanical edit produces and only a test
  catches. Creating still defaults to `default`: a note made here has this
  library's provenance, and that asymmetry is the model rather than an
  inconsistency.

  One behaviour changed beyond the scope rule, and it is worth stating: a
  link written by name now resolves across collections, so the same title in two
  of them is *ambiguous* rather than quietly resolving to this library's copy.
  Refusing to guess is the behaviour to want there.

  `--collection` is gone from `fix` and `export archive-v2` per the answer
  above, and on `lint`, `graph report`, `graph export` and `export archive` it
  narrows rather than presumes. The five importers keep theirs: an import writes
  a collection, and that is a different verb.

  Covered by `internal/store/collection_scope_test.go` -- one migrated library,
  four questions: lint sees the note, fix reaches it, a full archive contains
  it, and asking for one collection still narrows.
- **Whether a dangling collection identifier is repaired by adopting the note
  into `default` or by recreating the missing collection row -- Decided
  2026-09-08: recreate the row.** Adopting rewrites provenance; recreating
  preserves the identifier and admits only that the description of it was lost.
  The label decision above is what makes this affordable: a recreated row can be
  renamed into something meaningful, so nobody has to rewrite a note's origin to
  get a readable name. Adopting was simpler and was the wrong kind of simple --
  it destroys the one fact the field exists to carry, and it is one-way.
- **What "the necessary schema for Notrios" means as a precondition of that
  repair**, and whether the repair runs by default or only when asked for. Every
  other `fix` kind is mechanical and reversible in effect; this one changes what
  a note says about where it came from.
- **Whether removing `kind` and `capabilities` ships in v0.8 -- Decided
  2026-09-08: yes.** They are `required` in the OpenAPI response schema, so
  removing them is a breaking change to a documented shape. Pre-1.0, with no
  consumer that reads either field -- checked: nothing in `web/`, and the only
  readers are the two handlers that emit them -- removing them now is the honest
  move. The alternative preserved a promise nobody was relying on.

- **Whether a collection's label is editable -- Decided 2026-09-08: yes, and it
  already is.** `PATCH /api/v1/collections/{collection_id}` is routed today, so
  the question was keep-or-remove rather than write-or-not. Keeping it is what
  makes the answer to the identifier question affordable: an import mints an
  identifier from whatever `--collection` was typed, and two Joplin exports
  imported months apart become two collections whose ids carry dates. Being able
  to call one "Joplin export, January 2026" afterwards costs nothing and changes
  no provenance, because the identifier is untouched. Editing the *label* and
  editing the *identifier* are different acts, and only the first is offered.

- **What happens to the hardcoded `capabilities` in archive-v2 -- Decided
  2026-09-08: remove it.** `store.Collection.Capabilities` is a constant --
  `documents, search, resources, links, graph` for every row -- and
  `internal/archivev2` writes it into every collection record. After the API
  removal it would exist only to be written into archives, recording the same
  five words about every collection anyone ever had.

  **This is a cross-repository change and the order is not optional.**
  `movenotes-v3`'s `notrios_archive.py` declares `capabilities` in the
  *required* half of the collection payload and enforces sortedness, so an
  archive written without it is refused by that verifier. The verifier must stop
  requiring the field before Notrios stops writing it; the other order makes
  every archive written in between unreadable to the importer.

  Not to be confused with the archive's own `required_capabilities` -- the five
  base capabilities that gate whether a reader may open an archive at all. That
  is a real mechanism and is untouched. The word does two jobs and only one of
  them means anything.
- **Whether a collection's `name` and `description` stay editable.** The
  identifier is immutable and the row undeletable, but correcting a label is not
  a change to where a note came from. If they are editable there is a
  `PATCH /api/v1/collections/{id}` to write, and if they are not, the row is
  written exactly once and only an import can write it.

### Scope, now that the answers are known

1. Remove `kind` and `capabilities` from `api.Collection`, the OpenAPI schema
   and MCP's `list_collections`; remove the three columns that never existed
   from `DATABASE_SCHEMA.md`.
2. Make an unspecified collection mean every collection in `lint`, `fix` and
   selection, as it already does in search -- subject to the publication
   decision above -- and drop `--collection` as a scope from `fix` and
   `export archive-v2`.
3. Add the dangling-identifier repair to `fix`, in whichever of the two forms
   the decision above settles on.
4. Keep the collection row write-once, or add the label edit, per the last
   decision above.

A test for each of the four selection paths that a note in a second collection
is seen: that is the property none of them had, and a count that only ever ran
against a single-collection library could not have shown it.

### Depends on H15

`collection:` querying and the note-inspector display land in H15. This step is
what decides whether a collection is a label or a contract; the display is
honest either way, because it shows what the note records.


**Outcome (2026-09-08).** Done. `kind` and `capabilities` are gone from
`api.Collection`, the OpenAPI schema and `list_collections`; the hardcoded
capabilities are no longer written into archive-v2; `DATABASE_SCHEMA.md`
describes the three columns the table has rather than the three it never had;
and a note whose collection identifier names no row is reported by lint.

**The archive change was a cross-repository change, and the order was the whole
risk.** `movenotes-v3`'s verifier had `capabilities` in the *required* half of
the collection payload, so an archive written without it would have been
refused. That verifier stopped requiring the field first (`cc4ec25`), with a
test that proves the old shape refused and the new one accepts both; Notrios
stopped writing it second. The other order would have made every archive written
in between unreadable by the importer.

**And the reading direction nearly lost more than the writing one.** Deleting
the field from `CollectionRecord` compiled, and would have made
`archivev2.decodeStrict` -- which sets `DisallowUnknownFields` -- reject every
archive Notrios had already produced. The field stays, read and never written,
with `omitempty` doing the work. Two tests hold both directions: a new record
carries no `capabilities`, and an older one still decodes with its five values
intact.

`POST /api/v1/collections` still accepts `kind` and ignores it. Removing it from
the response was the point; refusing it on the way in would be a second break
for a caller written against the older shape, over a field that was never
stored.

**On the dangling-identifier repair: decided, detected, and deliberately not
built.** `documents.collection_id` is a foreign key and `PRAGMA foreign_keys` is
ON, so no supported write can produce the state -- a physical restore suspends
the constraint while installing an image, which is how a library can *arrive*
in it. Detection is what makes the decision actionable and is read-only, so lint
gained `dangling_collection`; the repair is not written, because it would be
code for a condition nothing reachable produces, and the answer is recorded so
that whoever first observes one does not have to decide under pressure. The
schema precondition, asked in the original decision, has an answer: the repair
needs the constraint back on, because a recreated row it cannot verify is a
guess.
## H18. Make the features page usable, and generate the table under it

**Ordering.** After H15, whose registry and coverage gates this builds on.
Independent of the installer chain and of H16.

**Goal.** Turn `docs/features.md` from a machine-checked list into a page a
person can read, and generate the capability-by-surface summary from the same
registry so it cannot drift from the prose beside it.

**Why it needed a step.** The page's guarantees were about *existence*: every
command, REST operation and MCP tool is claimed by some capability, and a
capability may not claim a surface that is not there. Both are worth having and
neither says anything about whether the page is any good. It rendered as
twenty-nine bullets, each a title, a summary, a bracketed score (`CLI 3, REST 9,
MCP 4, GUI 2`) and a paragraph of caveats -- a format that answers "does this
exist" and defeats reading. And its hand-written opening said tagging was
reachable from neither the command line nor the interface, months after both
were built: the failure this page exists to prevent, on the page itself.

**What was done 2026-09-04.**

*Seven drafts, one canonical list.* `docs/features.md` was rewritten against
seven independently generated feature inventories (`opencode`, one per model,
from a shared prompt). They were scored by mapping each draft's sections onto a
canonical list of 48 end-user capabilities -- the registry's 29 plus the
nineteen the drafts evidence that the registry does not carry as its own row
(the local-first premise, run modes, draft protection, revision history, paste
as Markdown, protected items, link help, the URL handler, external links,
Recoll, the Help notebook, themes, the REST API, configuration, install
lifecycle, archive verification, and paste/table handling among them). The
mapping is a judgement and is recorded as one; the ranking it produced is in the
commit message rather than in the documentation, because a model leaderboard is
not a fact about Notrios.

*The generated half now reads as prose.* `(Registry).Lines` renders one `###`
section per capability -- title, summary, the surface note as its own paragraph,
and a sentence saying where it can be reached -- instead of a bullet with a
bracketed score. Three things had to change to allow it: the generator now
passes through a value that is already block Markdown rather than prefixing a
bullet, a section now ends at the next heading of its own level *or* at another
configured section rather than at any heading at all, and the counts moved from
brackets in the prose to a column in a table, which is the shape a number
belongs in.

*The table is the answer to "can I do this from here?".* A new
`(Registry).SurfaceTable` fragment renders one row per capability and one column
per surface -- desktop app, command line, REST, MCP, and the shared library --
with the number of operations each spends on it. It is generated from
`FEATURES.json`, so it cannot disagree with the sections above it.

*The shared-library column is empty, and is checked to stay honest.* The C ABI
exists (H1), and nothing in the registry claims it as a way to perform a
capability. The column is rendered as dashes and `Check` **refuses** a claim
there, because that surface publishes no inventory to verify one against -- the
column can only be filled the day the ABI says what it offers. A column of
hopeful ticks is the one thing this page must never contain.

**What the table exposed.** Searching a library is the only everyday capability
with no command line at all: `Search your notes` reads `GUI 1, REST 3, MCP 2`
and a dash. That is H19.

**Still open.** The prose in `FEATURES.json` is per-capability and was written
against the surfaces rather than for a reader; the drafts are better at leading
with the user's goal ("I use Notrios to ..."). A pass over the 29 summaries and
surface notes in that voice is worth doing, and is deliberately not done here
because rewriting twenty-nine paragraphs in the same sitting as the machinery
that renders them makes both harder to review.

## H19. `notriosctl search` — complete

**Ordering.** After H18, which found the gap, and after H22, which supplies the
values `notebook:` and `collection:` can name -- a query language nobody can
enumerate the terms of is a language you have to already know.

**Goal.** Make a library searchable from a script: the same query language the
interface uses, results as JSON carrying each note's stable link, and the paging
a program needs.

**Why.** Every other everyday capability has a command line. Search does not,
and the consequence is sharper than a missing convenience: nothing at the
command line produces note identifiers, so `notes show`, `notes move`, `tags
add` and the rest can only be used on an id somebody already has. The workaround
in the documentation today is `export archive --query`, which applies the query
language to a *file export* -- an answer to a different question. H17's
query-driven batch work needs this for the same reason.

**Shape.**

- `notriosctl search [--db …] "<query>"` -- the query language exactly as the
  search box parses it, including `collection:` and `notebook:` terms.
- `--json` -- an array of hits, each with the note id, title, notebook,
  collection, updated time, and the `document://` URI that is its stable link.
  Human output stays the default; JSON is what a pipeline asks for.
- `--limit N` -- how many hits to return, bounded by the same ceiling the API
  uses, with `next_cursor` reported so a caller can continue.
- `--count` -- the number of matches rather than the matches. **This one is not
  free**: `store.SearchResponse` carries hits, a cursor and a truncation flag
  and no total, so `--count` needs either a counting query in the store or an
  honest refusal to guess. Paging the whole result set to count it would be a
  lie about cost on a large library. Decide before building.
- `--links` -- emit `notrios://` links rather than `document://` ones, for
  pasting into another machine's library; `notriosctl link` already produces
  that form for one note.

**Boundaries.** Read-only. It prints what a search returns and changes nothing,
which is what lets it be safe in a pipe. Trash stays out unless `is:trashed`
asks for it, exactly as elsewhere.

**Working state.** `notriosctl search "tag:todo" --json --limit 5` prints five
hits with their stable links; the features table shows a command-line column for
`Search your notes`; and a command-line journey demonstrates finding a note and
acting on it with the id the search returned.

**Outcome (2026-09-08).** Done. The loop the command line could not close is
closed: a search returns note identifiers, and the identifiers are what every
other command takes. A journey runs that loop end to end -- write, search,
count, read, tag -- and a test checks the round trip directly, because "the id a
search returns is one another command accepts" is the whole point and worth
asserting rather than assuming.

**The `--count` decision, which the item said to make before building: a
counting query.** The refusal it was weighed against would have been defensible
only if counting meant paging everything, and it does not. Both search paths
already compile the query into a `WHERE` clause and arguments -- the FTS-anchored
one and the predicate one -- so the same compiled predicate is counted instead of
paged, through joins that mirror each select shape exactly. A count over
different joins would answer a different question while looking like the same
one.

`TestSearchCountAgreesWithPaging` is the check that matters: for three queries
across both paths, `--count` must equal what paging actually returns. Breaking
it by one made all three fail, which is what a second answer to the same
question looks like when it drifts.

`--count` refuses `--limit`, `--cursor` and `--links` rather than ignoring them.
They are about the hits and a count returns none; accepting input that changes
nothing is what H25 spent its time removing.

**A change to the item as written.** It said human output stays the default with
`--json` beside it. H24 has since decided the opposite for this command line --
one output format, rendered by a template when a person wants a table -- and a
new command with two forms would contradict the decision recorded three items
ago. So `search` prints JSON, like the other seventy-nine, and `docs/cli.md`
shows the gomplate pipeline for anyone who wants a table.

Two smaller decisions worth recording. `--links` resolves each hit through
`StableDocumentURI` rather than assembling a link locally, so a search prints
the same link `notriosctl link` prints, and a test compares the two. And a
malformed query is reported as a query problem with a pointer to the language
reference, because the parser's own message gives no indication that the thing
at fault is what the person typed.

## H20. Bring the atlas current, and stop it drifting again — complete

**Ordering.** After the root-document inventory, which found the drift.
Independent of everything else; it touches one document and adds one check.

**Goal.** Make `CONTEXT_MAP.md` answer the question it exists for -- "where does
this live, and what is it for?" -- for the code as it is now, and make the parts
of that answer that can be checked, checked.

**Why.** The atlas had rotted in three separate ways at once, and each is a
different kind of failure. It carried a copied fact (`H0 is complete; H1 is
next`, a month stale). It was missing eleven packages, including every
documentation gate that now fails builds, despite `CODING_STANDARDS.md`
carrying a rule to add them -- a rule in a document nobody consults while adding
a package. And its newest section is dated 2026-07-16, so fourteen finished v0.8
items left no trace in the map of the codebase they changed. The copied fact and
the missing package names are fixed. What is left is the part that needs more
than an edit: a hundred lines of chronological "task R__ additions" sections
that record *when* something arrived rather than *where it is*, which is why
adding to the map feels like appending to a changelog and is why nobody did.

**Shape.**

- **The v0.8 items get real entries.** Fourteen complete items, described the way
  the rest of the atlas describes work -- what the package is for, not what the
  slice was called.
- **A gate for the thing that actually goes missing.** Every directory under
  `internal/` and `cmd/` appears in the atlas, checked in `internal/docrules`
  alongside the pointer checks. This is the mechanical half of the rule that
  `CODING_STANDARDS.md` used to state and nothing enforced: eleven packages
  slipped past it. A package is a durable, enumerable thing, so it can be
  checked; "major files or directories" cannot.
- **The root-documents list is generated.** The atlas opens by listing the root
  documents and what each is for, which is exactly what
  `docs/docrules/DOCUMENTS.json` now holds. Generate that section from the
  registry rather than keeping a second copy that can disagree with the first.
- **The chronological sections are resolved**, one way or the other -- see the
  decision below.

**Boundaries.** The atlas describes; it does not become a second architecture
document, and it does not acquire status. Nothing here regenerates prose about
what a package *does* from its doc comment: a one-line hand-written purpose is
the point of the map, and a generated one would restate the code to a reader who
is looking for orientation. No change to any package.

**Open decisions.**

*What happens to the twenty-one chronological "additions" sections.*
Non-blocking; the default below is taken if no answer comes.

- **Reorganise by location (default).** Merge them into the existing
  location-shaped sections, so the map has one entry per thing and a reader
  looks up a path. History is not lost -- it is in the archived plans, which is
  its home.
- *Keep them, and append a v0.8 section.* Cheaper, and preserves the record of
  what arrived when, but leaves the map answering two questions badly and makes
  the package gate awkward: a package could satisfy it from a 2026 section that
  no longer describes it.
- *Split the document.* A location map plus a separate chronology. Rejected in
  advance unless asked: two documents where one is stale is worse than one.

The recommendation is the default. What makes the atlas rot is that adding to it
means choosing a section by date, and the correct date section is always the one
that does not exist yet.

**Working state.** `go test ./internal/docrules/` fails when a new package under
`internal/` or `cmd/` has no entry, and passes with all of them present;
`CONTEXT_MAP.md` names every current package and every root document, with the
document list generated; and adding a package to the repository without touching
the atlas fails a build rather than a review.

**Outcome (2026-09-08).** Done, and the check found more than the reorganisation
did.

The atlas is organised by location now: nine sections named after where things
are, replacing twenty-two named after the task that added them. Every one of the
176 path entries was moved verbatim rather than rewritten, so nothing was lost
in the reshuffle and nothing was quietly reworded. Twelve prose bullets that
were changelog rather than atlas -- "Git repository initialized", "Rewritten:
ENVIRONMENT_SETUP.md" -- were dropped, because what arrived when is in the
archived plans, which is its home.

**The gate found five missing packages where a grep found one.** `internal/`
and `cmd/` are enumerated and matched against backticked paths in the document,
and the difference matters: `paths`, `media` and `jobs` are ordinary words that
appear in prose, so a search for bare names reported them as recorded when they
were not. `internal/application`, `internal/clispec`, `internal/jobs`,
`internal/media` and `internal/paths` now have entries; four of the five predate
v0.8 entirely. There is a test for the word-matching failure specifically,
because a gate that can be satisfied by coincidence is worse than none.

The root-document list is generated from `docs/docrules/DOCUMENTS.json` -- the
same registry that decides what each document is the home for. The atlas and the
inventory were two lists of the same thing, and the hand-written one had drifted.

The rule this replaces was one sentence in `CODING_STANDARDS.md`: "Update
`CONTEXT_MAP.md` when adding major files or packages." What is checked is
narrower than what that promised -- packages, not "major files or directories" --
because a package is a durable, enumerable thing and "major" is a judgement
nobody can automate. A narrower promise that is kept beats a broader one that
eleven packages walked past.

## H21. Read a note and its structure, from the command line — complete

**Ordering.** After H23, which makes the help text the truth about what the
command line has, because otherwise nothing added here is visible to the gates
that are supposed to check it. Beside H19 and H22: search finds the id, this
reads what the id points at.

**Goal.** A person or a script working at a terminal can read a note, see what
it is made of, and pull a file out of it, without opening a browser or writing
an HTTP client.

**Why.** `notriosctl notes show` exists and prints JSON metadata; everything
else about a note's structure is reachable only over REST and MCP. Both of those
surfaces are already complete for this -- the gap is one adapter, which is the
shape v0.8 has now found five times.

The features registry states the opposite, and states it as a decision:
`read-notes` carries `"No command line: reading a note is what the GUI and the
API are for"`. That is false in two directions at once. `notes show` reads a
note today, so the claim is wrong about the present; and the reason given
describes reading as something a terminal has no business doing, which is not a
boundary anybody chose -- it is a gap being explained after the fact. This is
the third such sentence H18's review has produced, and the pattern is worth
naming: an absent adapter tends to acquire a justification.

**What already exists, and is not built again.** This is written down because
the item's first job is to say so in the documentation:

| Ask | REST | MCP | Command line today |
|---|---|---|---|
| Read a note | `GET /documents/{id}`, `/body` | `get_document`, `get_documents` | `notes show` (JSON only) |
| Its structure | `/outline`, `/blocks`, `/lines` | `get_document_outline`, `get_document_blocks`, `get_note_line_range` | none |
| Its attachments | `GET /documents/{id}/resources` | `list_document_resources` | none |
| One attachment's bytes | `GET /resources/{id}/content` | `read_resource` | none |
| Its links | `GET /documents/{id}/links` | `list_document_links` | none |

**Shape.**

- `notriosctl notes show --document <id>` renders the note as Markdown with
  YAML front matter, to standard output. `--json` returns the structured form
  instead; `--output <file>` writes to a file rather than the terminal.
- **The front matter is the one Notrios already writes.** `internal/projection`
  renders notes as Markdown with front matter carrying `id`, `title`,
  `notebook`, `collection`, source provenance and `tags` -- it is what Recoll
  indexes and what an Obsidian-shaped reader expects. Reuse `renderNote` rather
  than writing a second renderer: two front-matter formats in one product is a
  bug waiting for the first person who round-trips through the wrong one.
- `notriosctl notes outline --document <id>` -- the headings and their anchors.
- `notriosctl notes resources --document <id>` -- what is attached, with ids,
  MIME types and sizes.
- `notriosctl notes links --document <id>` -- the links out of the note,
  resolved and broken alike, since a broken link is the interesting one.
- `notriosctl resources get --resource <id> --output <file>` -- the bytes.
  Retrieval belongs to `resources`, which already exists, and listing belongs to
  `notes`, because "what is attached to this note?" is a question about the note.
- Every listing takes `--json`; the human form stays the default. This item
  settles `notes show` and the commands it adds; H24 applies the same rule to
  the rest of the command line, where six commands of eighty-one offer both
  forms today.
- Every new command is declared in H23's command registry, which is what makes
  it appear in `notriosctl help`, in `docs/cli.md`, and in the coverage gate.
  A command added without it is a command the documentation cannot see.

**Boundaries.** Read-only. Nothing here writes a note, attaches a file, or
changes a link. Trash is reported rather than hidden, as `notes show` already
does, because a script that cannot tell a trashed note from a missing one will
eventually overwrite one.

**Open decisions.**

- **Whether `notes show` changes its default output -- Non-blocking; the
  default below is taken if no answer comes.** It prints JSON today and the
  requested contract is Markdown by default with `--json` for the structured
  form.
  - *Change the default, keep JSON behind `--json` (default).* One read
    command, and the JSON form stays available. The break is cheap: `notes show`
    has never appeared in `notriosctl help`, so no documented contract changes,
    and v0.8 is unreleased.
  - *Add `notes read` and leave `show` alone.* No break, but two commands that
    read a note, and the reader has to learn which.
  - The recommendation is the default. Say it in the v0.8 release notes as a
    behaviour change anyway, because "it was undocumented" is a reason, not an
    excuse.
- **Where attachment retrieval lives -- Non-blocking.** `resources get` as
  above, rather than `notes get-resource`. A resource is addressable on its own
  and can be referenced by several notes, so hanging retrieval off one note
  would misdescribe the model.

**Working state.** `notriosctl notes show --document <id>` prints a Markdown
note another application can read; `--json` prints the structured form;
`notes resources`, `notes links` and `notes outline` answer their questions;
`resources get --output` writes a file whose bytes match the stored resource;
the features table shows a command-line column for `Read a note and its
structure`; and the registry's claim that reading has no command line is gone,
replaced by what each surface actually offers.

**Outcome (2026-09-08).** Done, and it found a defect in the surfaces it was
supposed to be catching up with.

`notes show` prints the note as Markdown with front matter and `--json` gives
the fields; `notes outline`, `notes resources` and `notes links` answer what a
note is made of; `resources get` writes an attachment's bytes. Every one takes
`--output`, which keeps the exit code a shell redirect throws away.

The Markdown comes from `internal/projection`, exported as `RenderDocument`
rather than copied, so the command line and the Recoll projection cannot
disagree about what front matter a note carries. That reuse paid immediately: a
note in Trash renders with `trashed:` in its front matter, because
`doc.DeletedAt` was already on the document the renderer was handed.

**The defect.** `notes outline` needed a heading parser, and there was already
one -- and then a second. `httpapi` had its own `slugifyHeading` alongside
`markdownblocks.Slugify`, and the two disagreed on everything outside ASCII:
`Café notes` is stored as `café-notes` and the outline reported `caf-notes`,
while a heading written in Japanese was reported with an **empty** anchor. So
`GET /api/v1/documents/{id}/outline` and `get_document_outline` had been handing
back anchors that resolve to nothing, as though they were links. The outline is
now derived from the same parse that stores the slug, which makes them the same
thing rather than two things that agree; `slugifyHeading` is gone, and a test
checks every reported anchor against the ones the note carries.

**A second wrong-answer-shaped-right.** `store.ListDocumentLinks` took
`outgoing`, `incoming` or `both`, and an unrecognised direction fell through
both branches and returned an empty page. `?direction=out` answered "this note
has no links" with a 200. It is refused now. I found this by passing `out`
myself and believing the empty result for a minute, which is exactly what a
caller would have done.

A correction to my own work: the first Markdown path re-read the note by id
through `projection.RenderNote`, which excludes Trash, so `notes show` on a
trashed note reported "no note" -- undoing the careful handling the JSON path
already had. `TestNoteDeleteAndRestore` caught it. The renderer takes the
document the caller resolved now, and both output forms are asserted to say a
note is in Trash.

The command line uses the API's own words for `--direction` rather than a
shorter pair of its own, because two vocabularies for one idea is the thing this
milestone keeps finding.

**Not done here:** blocks, line ranges and earlier revisions have no command.
They are read surfaces REST and MCP have and the command line does not, and the
guide says so rather than implying the set is complete.

## H22. Discover the values a query can name — complete

**Ordering.** Before H19, which needs it: a query can name a notebook or a
collection, and nothing at the command line says which ones exist.

**Goal.** Answer, from the command line, the two questions a person must answer
before they can write a query: what notebooks are there, and what collections
are there.

**Why.** `collection:` and `notebook:` narrow a search, and H16 made a search
span every collection so that those terms mean something. But the values are
undiscoverable from a terminal. `notriosctl notebooks list` exists; there is no
`collections` command at all, and the only way the command line brings a
collection into existence is as a side effect of `import --collection <id>`,
through `ensureCollectionOrExit`, which creates it silently if it is new.

The features registry again says otherwise: `collections` claims that "creating
and reconfiguring collections stays on the command line and over REST". Half of
that is true. A collection can be created on the command line only by importing
into it, and it cannot be listed, named, shown or reconfigured there at all.

REST and MCP are complete here too -- `GET /api/v1/collections`,
`/collections/{id}`, `list_collections`, `list_notebooks`, `get_notebook_tree`
-- so this is one adapter again, and the documentation must say so rather than
implying the capability is new.

**Shape.**

- `notriosctl collections list [--json]` -- id, name, and how many notes name
  each, because a collection with no notes is the interesting one after an
  import.
- `notriosctl collections show --collection <id> [--json]`.
- `notriosctl notebooks list` gains `--json`, and prints the notebook id
  alongside the name in the human form. A name is what a person reads and an id
  is what a query takes, and today the command prints the sidebar's view.
- The CLI guide gains the sentence that ties them together: run these to find
  the values, then use them in `notriosctl search`.
- Both commands are declared in H23's command registry, so `--help` describes
  them and the coverage gate counts them.

**Boundaries.** Listing and showing only. **No `collections create`, `rename`
or `delete` here** -- H16 owns what a collection's identity means, whether one
can be renamed or deleted, and what happens to notes that name it. Adding
management commands before that decision would build the thing H16 is deciding
about.

**Working state.** `notriosctl collections list` names every collection in a
library imported from Joplin and from Obsidian; `notriosctl notebooks list
--json` gives ids a script can put into a query; the features table shows a
command-line column for `Group libraries into collections`; and both registry
entries describe the surfaces that exist rather than the ones assumed.

**Outcome (2026-09-08).** Done. `notriosctl collections list` and
`collections show` exist, `notebooks list` prints a table with `--json` beside
it, and both guides say how to get from a listing to a query.

The count turned out to be the part worth having. A collection is provenance
rather than a place, so the only thing that says whether an import landed is how
many notes name it -- `store.CollectionNoteCounts` groups live documents by
collection, excluding Trash, and a test proves the exclusion by removing it and
watching the number lie.

Two documentation defects were found rather than assumed. The `collections`
entry in the features registry claimed that "creating and reconfiguring
collections stays on the command line and over REST"; on the command line a
collection could only be created as a silent side effect of
`import --collection <new-id>` through `ensureCollectionOrExit`, and could not
be listed, named or shown at all -- a surface that wrote and could not read.
And `docs/query-language.md` documented `notebook:` and never mentioned
`collection:`, three weeks after H16 made a search span every collection so that
`collection:` would mean something.

A correction to this item's own plan text: it said `notebooks list` "prints the
sidebar's view" and needed ids adding. It already printed ids -- in JSON, and
only in JSON. The real gap was the reader, not the data, so the change is a
table by default rather than new fields. Both forms are now asserted, because
both are contracts.

`collection:"<id>"` is offered as a query term rather than as a command. The
first draft printed `notriosctl search 'collection:"default"'`, which names a
command H19 has not built yet: a hint that tells someone to run something the
program will reject is worse than no hint.

**What this deliberately did not do.** No `collections create`, `rename` or
`delete`: H16 owns what a collection's identity means and what happens to notes
that name one. `kind` and `capabilities` are not printed either -- the store
hard-codes the capability list and records no kind, so printing them would hand
the reader a constant dressed as a fact about their library.

**An asymmetry recorded rather than hidden:** the command line reports the note
count and REST and MCP do not. The count is derived and the underlying listing
is symmetric across all three, but H16 should decide whether it belongs in the
API rather than leaving the difference to be discovered.

## H23. One description of the command line, and `--help` everywhere — complete

**Ordering.** First of the three. It is the source the other two are checked
against, and until it is done a command added by H21 or H22 can be invisible to
every gate meant to check it.

**Goal.** Asking any part of `notriosctl` for help gets help, in one form; and
the command line has one description that the dispatcher, the help, the
published guide and the coverage gates all read, rather than three that can
disagree.

**Why.** Three separate hand-maintained descriptions of the same command line
exist today, and they already disagree.

*Asking for help behaves three different ways.* Measured, not assumed:

- `notriosctl notes --help` prints **`unknown notes subcommand "--help"`** and
  exits **2**. Asking a command group for help is reported as a usage error.
  Every group does this -- `notes`, `tags`, `sync`, `resources`, the rest.
- `notriosctl notes show --help` exits 0 and prints Go's `flag` default dump:
  `-body` rather than the documented `--body`, no positional arguments at all,
  and no prose. `notriosctl import obsidian --help` does not mention that a
  vault directory is required, which is the one thing a reader needs.
- `notriosctl help notes` ignores its argument and prints the whole top-level
  help. There is no per-command help path.

*And the descriptions have already drifted.* Eight subcommands exist, work, and
appear in none of it: `notes show`, `notes edit`, `notes delete`,
`notes restore`, `sync handshake`, `sync retention`, `sync retire`, and
`sync start`.

*The second consequence is silent, and it is the reason this item is first.*
`internal/docgen`'s `cliUsageForms` derives the entire command-line surface
inventory by parsing the usage literal in `printHelp` -- whose own doc comment
calls it "the finite command and flag usage registry". So a command absent from
that literal is absent from `docs/cli.md`, which is generated from it, from the
Help notebook seeded from that, from the features coverage check, from
`doccompare`, and from the `"cli": 0` ratchet in `unclaimedBaseline`. The
ratchet is honest about what it measures, and what it measures is a string
literal. **Nothing in the repository compares that literal to the dispatcher.**

H15 shows the cost. It added `notes delete` and `notes restore` on 2026-09-03
specifically to close a command-line gap, and did not add them to the literal --
so the gap it closed is still invisible to the gate that measures such gaps, and
absent from the published CLI reference.

Adding `--help` by hand to every subcommand would make a fourth description.
That is why this item is about the source rather than the symptom.

**Shape.**

- **One command registry in code**, declaring for each command and subcommand
  its name, its one-line purpose, its flags, and its positional arguments. The
  dispatcher reads it, so a command that exists is described by construction and
  the two cannot drift.
- **`--help` and `-h` work at every level**, print the same shape, and exit 0:
  on `notriosctl` itself, on a group (`notriosctl notes --help` lists the
  group's subcommands), and on a subcommand (its purpose, flags in the `--flag`
  form the documentation uses, and its positional arguments). `notriosctl help
  <command> [<subcommand>]` gives the same text, because both spellings are ones
  people try.
- **A group asked for help is not an error.** Exit 0, and nothing printed to
  standard error.
- **`cliUsageForms` reads the registry** rather than parsing a string literal,
  so the surface inventory becomes a fact about the command line instead of a
  fact about its help text. `docs/cli.md` is generated from the same registry
  and gains the eight missing commands with it.
- **Machine-readable discovery.** `notriosctl help --json` emits the registry:
  every command, its flags and its arguments. The documentation tools consume
  that rather than re-parsing prose, and so can anything else that needs to
  discover what this command line offers -- which is the same reason
  `list_collections` exists rather than a page describing collections.
- **Re-baseline afterwards.** `unclaimedBaseline` and
  `docs/docfeatures/FEATURES.json` are updated once the inventory reflects the
  command line, so the newly visible commands are claimed by a feature rather
  than quietly raising the backlog.

**Boundaries.** No command is added or removed and no behaviour changes; this
item makes what exists visible and describes it once. H21 and H22 add commands,
and they add them to the registry, which is what makes them countable.

**Open decisions.**

- **Whether the four `sync` subcommands are for users -- Non-blocking.**
  Recommended: document them. If any is a daemon-side or test-only entry point
  it goes on an explicit exemption list with a stated reason, as the
  root-document inventory does for feature contracts. Silence is the condition
  being fixed, so the exemption has to be written down rather than assumed.
- **How far the registry goes -- Non-blocking; default named.** Declaring flags
  in the registry could mean the registry also constructs the `flag.FlagSet`,
  removing the second declaration entirely.
  - *Describe, do not construct (default).* The registry declares what a
    command takes; each command keeps its own `FlagSet`; a test checks that the
    two agree. Smaller change, and a mismatch fails rather than hides.
  - *Construct from the registry.* No possible mismatch, but it touches every
    command's parsing in an item that promised to change no behaviour.
  - The recommendation is the default: get one description of *what commands
    exist* first, since that is the drift that has actually happened. Unifying
    flag parsing can follow once nothing depends on the literal.

**Working state.** `notriosctl notes --help` lists the note subcommands and
exits 0; `notriosctl notes show --help` shows `--document` and `--body` in the
documented form with its positional arguments; `notriosctl help --json` lists
every dispatched command; `go test ./...` fails when a dispatched subcommand is
missing from the registry or when a registry entry names a command that is not
dispatched; `docs/cli.md` and the Help notebook list all of them; and the
unclaimed-CLI baseline describes the command line rather than its help text.

**Outcome (2026-09-08).** Done, and it found more than it set out to.

`internal/clispec` is the description: 79 commands, each with a purpose, a usage
form and its notes, embedded in the binary so an installed package describes
itself. `printHelp` renders it, `docs/cli.md` is generated from it, and
`internal/docgen` reads it instead of parsing a string literal -- so the surface
inventory went from 69 forms to 79 and became a fact about the command line.

One rule decides what asking for help means, in `helpAsked`, ahead of every
dispatcher. `notriosctl notes --help` lists the note subcommands and exits 0
where it used to print `unknown notes subcommand "--help"` and exit 2;
`notriosctl notes show --help` prints the documented `--document` and `--body`
rather than the flag package's `-body`; `notriosctl import obsidian --help`
names `<vault-dir>`, which no `--help` had ever mentioned. `help <command>`,
`<command> --help`, `-h` and `<command> help` all answer, because all four are
things people type. Thirteen per-group guards were written first and then
removed: they were a second copy of one rule, which is the fault being fixed.

`cmd/notriosctl/commands_test.go` walks the dispatch from `main` and compares it
to the registry in both directions. Writing it found a second dispatch shape --
five groups with one subcommand guard it with `if args[0] != "archive-v2"`
rather than a switch -- which a switch-only walker would have called a leaf while
quietly agreeing with a registry that disagreed with the program.

Two combined help lines were hiding commands behind a pipe: `sync init|status`
and `publish profile list|delete`. Split, they are four.

**Three false claims were in `docs/docfeatures/FEATURES.json`, and the mechanism
that let them stand is the point.** `read-notes` said "No command line: reading a
note is what the GUI and the API are for" while `notes show` existed;
`write-notes` said editing and deleting "happen in the GUI or over REST/MCP"
five days after `notes edit` and `notes delete` shipped. The coverage ratchet
read `"cli": 0` throughout, because it counted the help text and those commands
were not in it. Making the inventory real turned 13 surfaces unclaimed at once;
all 13 are now claimed, and the baseline is 0 again against a denominator that
means something.

A correction to this item's own work, recorded because it is the failure this
item exists to prevent: the first draft of the registry invented flags for
`sync retire`, `sync retention`, `sync handshake` and `sync start` rather than
reading them, and `notes edit --message` was missed. `docs/cli.md` disagreed with
itself in the same commit -- the generated list said `sync retire --key <id>` and
the hand-written section below said `--peer <replica-id>`. Corrected from each
command's own usage string. A description written from memory is the thing being
replaced.

A fourth reader of the literal surfaced during validation:
`performance/v0.7-g18a/build_inventory.py` parsed the same raw string with a
regex to count usage forms. It now counts the registry. Two frozen records moved
with it -- G18a's inventory count from 69 to 79, and G18f's recorded hash of
`docs/cli.md` -- because both recompute from the tree rather than from
themselves, which is why they noticed.

**Not done here:** the registry describes flags and does not construct them, as
the recorded decision said. A test that each command's `FlagSet` matches its
registry entry closes the remaining gap between the two; it is H25. The four `sync` subcommands are documented rather
than exempted; none looked internal once read.

## H24. JSON is the output; a template makes it readable — complete

**Ordering.** After H23. Independent of H21 and H25.

**Goal.** Keep one output format and document how to render it for a person,
rather than maintaining a second rendering inside every command.

**Why.** The question this item started from was whether listing commands offer
`--json`. Counting inverted it: of 81 commands, six offer both forms -- `paths`,
`migrate`, `config show`, `notebooks list`, `collections list`,
`collections show`, every one added during v0.8 -- about forty-five print JSON
and nothing else, and `doctor` prints prose and nothing else. So a program can
already parse nearly everything, and the missing half was the human form.

**Decided (2026-09-08, by the user): the output stays as it is.** A second
rendering in every command is a second thing to keep current, and this milestone
exists because of what happens to things that must be kept current by hand. JSON
is the interchange format; a person who wants a table pipes it through a
template. `gomplate` does this in one line and is not a dependency of anything
here -- Notrios neither ships it, requires it, nor knows about it.

That also settles the risk the earlier draft was blocking on: no default
changes, so no script breaks.

**Shape.**

- **Document it, with examples that run.** A section in `docs/cli.md` showing a
  listing rendered as an aligned table, a field pulled out for a shell variable,
  and a filter -- each executed by the documentation gates rather than written
  from memory, since a template that does not run is worse than no example.
- **`doctor` gains `--json`.** It is the one command a person can read and a
  script cannot parse, which is the wrong way round under a JSON-first contract:
  the whole point of the decision is that machine-readable output is the thing
  that always exists.
- **No new human forms, and no defaults flipped.**

**Boundaries.** No command's existing output changes. `gomplate` is named in the
documentation as one tool that works; nothing in the build, the packaging, or
the tests requires it, and no code path invokes it.

**Open decisions.**

- **What to do about the five commands that already have both forms --
  Non-blocking; the default below is taken if no answer comes.** `paths`,
  `config show`, `migrate`, `notebooks list` and `collections list`/`show`
  default to a human form with `--json` beside it, which is now the exception
  rather than the rule.
  - *Leave them (default).* They exist, are tested, and every one offers
    `--json`, so nothing is unparseable. Removing them would change output,
    which is the thing this decision says not to do, and the two listings exist
    precisely so somebody can read an id at a glance.
  - *Make JSON their default too, for one rule with no exceptions.* Tidier, and
    it breaks the scripts H22 shipped this week.
  - The recommendation is to leave them and say in the guide which commands have
    a human default, so the exception is documented rather than discovered.

**Working state.** `docs/cli.md` shows a working `gomplate` pipeline for a
listing, executed by the documentation gates; `doctor --json` reports what
`doctor` prints; and no other command's output has changed.

**Outcome (2026-09-08).** Done, and smaller than the item it replaced, which was
the point of the decision.

`docs/cli.md` gained "Reading JSON output": an aligned table, one field for a
shell variable, and a filter, each run against a seeded library with gomplate
5.2.0 before being written. Running them found what an invented example gets
wrong -- `%d` and `%f` both fail on a JSON number and `%v` prints it either way.
They are registered as reviewed-unrun rather than executed, because gomplate is
a host tool this repository does not ship, require or invoke, and running them
in the gate would make an external installation a build dependency of the very
page that says it is not one.

`doctor --json` reports the run a script could not read: every check with its
`state`, its `required` flag, and its detail. `required` sits beside `state`
because the two answer different questions -- what doctor found, and whether it
is allowed to be like that -- and a monitoring script should not infer the second
from the wording of the first.

The prose form is byte-identical to before. Both forms come from one pass over
the checks rather than two code paths, which is what makes the parity test
possible: every check in the JSON appears in the prose with the same detail, and
the two report the same number of checks. Breaking that on purpose fails, and
the first attempt at proving it did not -- it sabotaged a branch the sandbox
never reaches, so the test passed and proved nothing until the other branch was
broken instead.

The early exit matters more than it looks. `doctor` stops before its later
checks when the config will not load, and previously that printed two lines and
left; a `--json` caller would have got prose or nothing. One `finish()` prints
and exits, so the failing path reports the same shape as a full run, still
exiting 1.

**The non-blocking decision was taken as recommended:** the seven commands with
a human default keep it, and the guide names all seven so the exception is
documented rather than discovered.

## H25. Hold each command's flags to its description — complete

**Ordering.** After H23, which is where the gap it closes was left open.

**Goal.** A flag named in a command's description exists, a flag the command
accepts is described, and neither can drift from the other.

**Why.** H23 made the command line describe itself once and stopped one level
short: `internal/clispec` describes each command's flags as a usage string and
`flag.FlagSet` declares them separately, and nothing compares the two. The
recorded decision was to describe rather than construct, which was right for the
size of that change and leaves this open.

It is not hypothetical. Writing H23's registry, four `sync` commands got flags
that were invented rather than read -- `sync retire --key <id>` where the command
takes `--peer <replica-id>` -- and `notes edit --message` was missed entirely.
The generated `docs/cli.md` disagreed with its own hand-written section in the
same commit, which is how it was caught: by a human reading a diff, which is the
mechanism this milestone keeps trying to replace.

**Shape.**

- Parse each command's flag declarations from `cmd/notriosctl` and compare them
  with the `--flag` tokens in its registry usage string.
- A described flag the command does not accept fails. A flag the command accepts
  and the description omits fails, unless it is a common flag the registry
  declares once -- `--config`, `--db`, `--asset-store` appear on nearly every
  command and repeating them in every usage string would make the descriptions
  unreadable.
- The check names both the command and the flag, since the fix differs: correct
  the description, or admit the flag was undocumented.

**Boundaries.** Still describe, not construct. Each command keeps its own
`FlagSet`; this makes disagreement fail instead of removing the possibility. If
that turns out to be the wrong line, constructing the FlagSet from the registry
is a separate item with its own risk.

**Working state.** `go test ./...` fails when a command's usage string names a
flag it does not accept, or accepts an uncommon flag it does not name; the four
`sync` commands and `notes edit` pass because they were corrected by hand, and
breaking one of them on purpose fails the build.

**Outcome (2026-09-08).** Done. Both directions fail, and the first run found
one described flag that does not exist and **thirty-five that do and were never
mentioned**.

*Described and not accepted:* `publish profile list --name`. My own mistake from
H23 -- splitting `publish profile list|delete [--name <profile>]` into two
commands gave `--name` to both, and only `delete` takes it.

*Accepted and not described:* seventeen commands. `publish profile save` alone
hid ten, including `--query`, `--exclude-tags` and `--private-tags`, which decide
what a publication contains. `profile create` hid seven. Every addition was
composed from the flag's own declared type, default and description rather than
from memory, because writing H23's registry from memory is what produced the
invented flags this item exists to catch.

**A third category the item did not anticipate: a flag accepted and discarded.**
`tags list` took `--tag` because the tag subcommands shared one `FlagSet`
helper, and never read it -- so `notriosctl tags list --tag todo` returned every
tag in the library, which reads as a filter that found everything. There is no
honest way to describe that, so it is gone: `tags list` has its own flag set and
now refuses the flag. `notes show --body` is the same shape and kept
deliberately, so `accepts_ignored` declares it with its reason rather than
leaving it to be discovered.

Two details worth recording, both of which would have made the gate quietly
useless:

- **Flags are not always in the command's own body.** The sync commands take
  theirs from `newSyncFlags`, the note commands from `newDocumentFlags`. A check
  reading only the function would have reported every sync command as accepting
  nothing and passed.
- **`fs.Var(value, "name", ...)` names its flag second**, where every other
  declarer names it first. A first-argument reader gets it silently wrong, and
  did: my first pass reported `templates create --set` as described-but-absent
  when the command accepts it.

The exemptions are checked rather than trusted. `--config`, `--db` and
`--asset-store` are declared common once instead of repeated in eighty-five
usage strings, and a test refuses a "common" flag that fewer than two commands
take, so the exemption cannot become a hiding place.

`tags list` splitting its flag set is written without a conditional on purpose.
A branch would leave the gate guessing about what the command accepts, and a
gate that guesses reports problems nobody can act on.

## H26. Ask about one tag without fetching them all — complete

**Ordering.** After H25. Independent of H19, though a library with
`notriosctl search` makes the counts here more useful.

**Goal.** Answer "does this tag exist, and how much is on it?" without
retrieving every tag in the library, on all three surfaces.

**Why.** There is no way to ask about one tag. `notriosctl tags list`,
`GET /api/v1/tags` and `list_tags` each return the whole vocabulary, take **no
filter and no limit**, and are the only thing there is. So testing whether
`todo` exists means fetching every tag and searching the result, on a surface
whose whole point is answering a narrow question cheaply.

The counts are already right and are not the gap: `tags list` reports live
non-deleted note counts per tag, and so do the REST and MCP forms. What is
missing is narrowing.

The two workarounds are both poor, and the second is worse than it looks.
Filtering `tags list --json` fetches the entire vocabulary to answer one yes or
no. And `tags rename --from <name> --to <anything>` exits 1 with `not found: no
tag named ...` when the tag is absent -- an existence test built out of a
*write* command, which needs a `--to` the caller does not want and which will
eventually be run without the dry run by someone who forgot.

**This is a capability gap, not a missing adapter**, which makes it different in
kind from the reading and discovery items either side of it. No surface has it,
so nothing here is catching the command line up with REST; all three change
together, and the item is written that way so the command line does not become
the only place you can ask.

`tags list --tag <name>` used to look exactly like the answer. It was accepted
and silently ignored -- `tags list --tag todo` returned every tag in the library
-- and H25 removed it, because a filter that returns everything is worse than no
filter. That flag is why this item exists: somebody wrote the signature for this
capability and never wrote the capability.

**Shape.**

- `notriosctl tags show --tag <name>` -- the tag, its live note count, and its
  children when the name has any. **Exit 1 when the tag does not exist**, so a
  script can test existence on the exit code without parsing anything, the way
  `notes show` already reports a missing note.
- `tags list --prefix <p>` -- the tags under one branch. Tags are hierarchical
  (`shopping/mall`, and `tags rename --include-children` renames a whole
  branch), so a prefix is how a person browses a large vocabulary and is the
  same narrowing `--tag` should have been.
- `tags list --limit N` with the truncation reported, because a listing that
  silently stops is a listing that lies about a library's size.
- The same narrowing on `GET /api/v1/tags` and `list_tags`, so the three
  surfaces keep answering the same questions. `store.ListTags` grows the
  parameters rather than gaining a second query beside it.
- `docs/cli.md` and the query-language page say how to test for a tag and how to
  browse a branch, next to where they already explain `tag:`.

**Boundaries.** Read-only. Nothing here creates, renames or deletes a tag --
`tags add`, `tags remove` and `tags rename` already do that and are unchanged.
No new tag model: hierarchy is whatever `/` already means, and this item does
not decide anything about it.

**Open decisions.**

- **Whether `tags show` reports the notes carrying the tag -- Non-blocking; the
  default below is taken if no answer comes.**
  - *Report the count only (default).* The count is what makes the answer cheap,
    and listing the notes is a search: `tag:todo` is exactly that query, and
    H19 is the command for running it. Two ways to list the same notes is the
    duplication this milestone keeps removing.
  - *Report the first N notes as well.* Convenient, and it makes `tags show` a
    second search surface with its own paging, ordering and truncation rules to
    keep consistent with the real one.
- **Whether an absent tag is exit 1 or an empty result -- Non-blocking.**
  Recommended: exit 1, matching `notes show` and `collections show`, both of
  which already refuse rather than return nothing. A caller that prefers a
  parseable answer can read `--json`, which prints the refusal too.

**Working state.** `notriosctl tags show --tag todo` prints the tag and its
count and exits 0; the same for a tag that does not exist exits 1 and says so;
`tags list --prefix shopping` lists only that branch; `tags list --limit 1`
reports that it truncated; `GET /api/v1/tags?prefix=` and `list_tags` narrow the
same way; and the features table shows the capability on all three surfaces
rather than on one.

**Outcome (2026-09-08).** Done, on all three surfaces in one change, which is
what a capability gap asks for and a missing adapter does not.

`store.TagQuery` grew the parameters rather than gaining a second query beside
`ListTags`, so there is one place that knows how a tag listing narrows.
`tags show --tag <name>` exits 1 when the tag is absent; `GET /api/v1/tags?name=`
returns 404 for the same reason -- a lookup that finds nothing is not an empty
list, and returning one would make "no such tag" and "a tag with nothing on it"
the same answer, which is the distinction the caller asked for. `list_tags`
takes the same `name`, `prefix` and `limit`.

**A prefix is a branch, not a string match.** `shopping` contains
`shopping/mall` and does not contain `shoppingcart`, because the separator is
part of what a branch means. The wildcard case is the one worth having written a
test for: a tag named `50%` must be a prefix of its own children and not of
everything in the library, so the prefix is escaped for `LIKE` and a test tags a
note `50%`, `50%/off` and `500` and checks that only the first two come back.

`Truncated` is carried on the page rather than left to the caller to infer from
a row count. The query asks for one row more than the limit so the flag is
observed, and the boundary is tested: a limit exactly equal to the number of
tags is not a truncation, and an off-by-one there would report one on every full
page.

The shell example in `docs/cli.md` is executed rather than illustrated, against
a tag that the fixture puts on a note first -- running the idiom against a
library with no such tag would exercise the shell and prove nothing.

**The non-blocking decisions were taken as recommended.** `tags show` reports
the count and its children and not the notes: listing the notes carrying a tag
is `tag:todo`, a search, and H19 is the command for running one. A missing tag
exits 1, matching `notes show` and `collections show`.

Changing `ListTags`'s signature reached eight call sites across the store, the
HTTP layer, the Joplin importer and their tests. That is the cost of one query
path, and it is the right cost: a second narrowing query beside the first is how
two answers to the same question come to disagree.

## H27. Attach a file from the command line, without guessing where the link goes — complete

**Ordering.** After H21, which built the reading half. Independent of everything
else.

**Goal.** Put a file into a library from a terminal and get back the link,
leaving where that link goes to the person writing the note.

**Why.** The command line reads attachments -- `notes resources` lists what a
note carries, `resources get` writes one out, `resources report` covers the
library -- and cannot add one. That asymmetry was recorded as a boundary in H15
on the reasoning that attaching means placing a `resource://` link at a point in
the body only the author knows. The reasoning was right about *placement* and
wrong to stop there: placement is the author's, and the bytes are not.

**The product already separates the three acts**, which is what makes this
admissible:

1. **the resource** -- bytes in the content-addressed store, with an id and a
   `resource://` URI (`POST /api/v1/resources`);
2. **the reference** -- a row saying this note has this attachment, which is
   what `notes resources` lists (`POST /api/v1/documents/{id}/resources/{id}`);
3. **the link in the body** -- where it renders, which the author writes.

A command that did all three would be guessing at the third. A command that does
the first two and *prints* the URI is not guessing at anything.

**Shape.**

- `notriosctl resources add --file <path> [--filename <name>] [--document <id>]`
  creates the resource from local bytes and prints its id, `resource://` URI,
  MIME type, size and SHA-256. With `--document` it also records the reference,
  so `notes resources` lists it; without, the resource exists unattached and
  `resources report` will say so.
- **It never writes to a note body.** That is the boundary, not an omission, and
  it is what the output is for: the URI is the thing to paste.

- **`notriosctl notes append --document <id>` puts text at the end of a note**,
  because otherwise pasting the URI is worse than it sounds. Asked on
  2026-09-08 and checked: **no surface can patch a range of a note body.** REST
  and MCP can read one -- `GET /documents/{id}/lines?start=&end=` and
  `get_note_line_range` -- and neither can write one; the writes available
  anywhere are append, prepend, and replace the whole body. The command line has
  none of the three except whole-body replacement through `notes edit`.

  So without this, "paste the URI" means reading the entire note, editing it
  elsewhere, and writing the entire note back -- a read-modify-write over the
  whole body to add one line, with every concurrent edit in between silently
  lost. `append` and `prepend` already exist on REST and MCP and are the two
  writes that need no range; adding them to the command line is a missing
  adapter rather than a new capability, and it is what makes the rest of this
  item usable.

  *Reading a range and patching a range are not in this item.* Reading one is a
  missing adapter too and should follow. **Patching one exists nowhere**, and a
  capability no surface has is a decision rather than a gap -- what a patch
  means when a note changed underneath it is the question, and answering it
  belongs somewhere other than an item about attachments.
- The type is sniffed and admitted the way every other resource is, rather than
  trusted from the extension. `--filename` names the file for a reader when the
  path's own name is not the right one.
- Bounded by the same `MaxResourceContentBytes` ceiling the HTTP surface
  enforces, and refused rather than truncated.

**Boundaries.** Local bytes only. This is not a downloader: a URL belongs to
`notriosctl localize`, which goes through the domain policy, quarantine, hashing
and SSRF protections that `SECURITY_AND_MEDIA_POLICY.md` requires and this
command has no business reimplementing.

**What it unblocks.** `attachments` is one of the six features with no
command-line journey, and the only one of them recorded as a boundary that this
item turns back into a gap worth closing. With `resources add` there is a
journey: add a file, see it listed on the note, read its bytes back, and place
the link. The ratchet's floor drops with it.

**Open decisions.**

- **Whether `--document` belongs on this command at all -- Non-blocking; the
  default below is taken if no answer comes.**
  - *Keep it (default).* Adding a file to a note is one intention, and making a
    person run two commands to express it invites the second being forgotten --
    leaving an unreferenced resource that `resources report` then reports as
    rubbish.
  - *Split it*, with a separate `notes attach --document --resource`. Cleaner
    against the model, and it is the model the API already exposes as two
    routes.
  - The recommendation is to keep it, because the failure mode of splitting is
    silent litter and the failure mode of combining is a flag somebody does not
    need.

**Working state.** `notriosctl resources add --file photo.png --document <id>`
prints a `resource://` URI and `notes resources --document <id>` lists it;
`resources get` on that id writes back bytes identical to the file; nothing in
the note's body changed until `notes append` is asked to change it; and a
command-line journey covers add, list, read and place without a whole-body
round trip.

**Outcome (2026-09-08).** Done. `resources add` puts local bytes in and prints
the `resource://` URI; `--document` records the reference in the same step;
`notes append` and `notes prepend` place the link. A test asserts the note body
is byte-identical before and after adding a file, because that is the boundary
the command exists to respect rather than a nicety.

**It found a defect in the store, and my own comment found it.** I wrote that
the type is left to the store, "which sniffs the bytes on ingest", and the code
beside it passed `MIMETypeFromFilename` -- which suppresses the sniff. Removing
that argument made every file `application/octet-stream` instead, which is when
the real fault appeared: `NormalizeCreateResourceRequest` fills an absent MIME
type with `application/octet-stream` *before* `writeBlob` runs, `writeBlob`
treats that same value as "unspecified" and sniffs correctly, and then
`firstNonEmptyString(req.MIMEType, blob.MIMEType, …)` took the placeholder back.
The store identified a PNG and recorded octet-stream. Two lines disagreed about
what the placeholder means; the fix is what `writeBlob` already believed. A text
file named `.png` is now `text/plain` and a real PNG is `image/png`, and a test
proves the old behaviour fails.

That affected every caller supplying no type, not only this command.

**On the non-blocking decision: `--document` stays**, as recommended. Splitting
it would match the API's two routes more exactly and its failure mode is silent
litter -- an unreferenced resource that `resources report` later flags as
rubbish -- while combining's failure mode is a flag somebody does not need.

`store.JoinNoteText` moved out of `internal/httpapi`, where the REST and MCP
append paths both used it privately. Two implementations of "where does the
newline go" would have disagreed about a note that ends without one, and the
command line was about to become the second.

**This closed the `attachments` gap in H15.** The ratchet's floor drops from six
to five, and the comment says why attachments left the list rather than only
that it did.

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

Which items are complete, and which is next, is in the generated progress log
above and in `docs/docplan/PLAN_SLICES.json`. It is not restated here: this
paragraph said "H4 is the next incomplete item" for nine days after H4 finished.
No unapproved item begins until the user says to proceed with it.
