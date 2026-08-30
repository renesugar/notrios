# v0.7 G18e — executed GUI user journeys and action-length baseline

Date: 2026-08-29
Model: GPT-5 parent; GPT-5.6 Luna workers for bounded manifest, browser-harness, evidence-validator, and validation-wiring work
Working state: complete; G18f remains unapproved

## Goal and outcome

Every current GUI procedure in the published and protected Help documentation
now has a finite, source-anchored journey record. The strict manifest contains
37 journeys: 32 execute through a real browser and five remain explicitly
unverified with closed reason codes and source owners. Viewport declarations
expand the executed set to 44 passing result rows, followed by five matching
unrun rows. Nothing is omitted or counted as a pass because it is inconvenient
to automate.

The original nine G18a journey identities remain present. The final set covers
search and pagination; create, edit, move, Trash, restore, purge, and notebook
deletion review; protected Help; themes; stable links; resources and remote
media; graph, table-paste, live-query, math/code, and pane-resize behavior;
destructive cancellation; and 12 Sync Center goals covering entry, setup,
pairing, jobs, lazy resources, conflicts, backup/recovery, retention, repairs,
and retirement.

## Manifest and execution contract

`internal/docjourney` strictly decodes the manifest, rejects path traversal and
unknown fields, requires the exact desktop 1440×960 and narrow Sync Center
390×844 viewports, resolves documentation headings, retains the original G18a
IDs, and enforces the 32/5 state split. `cmd/docjourney` reuses G18c's frozen
Go/TypeScript declaration resolver for 52 production anchors and the one
result-bearing test anchor. CI and release packaging run this source-drift gate
after the frontend compiler dependency is installed.

The opt-in Go test builds the real CLI and daemon, creates independent
content-free host/joiner pairs for both viewports, seeds only repository Help
and generated fixture text, starts loopback services, and invokes the repository
Playwright runner. Setup, daemon mechanics, and accessibility navigation are
not user actions. Every passing journey must record a user action, a visible
rendered assertion, and a canonical store/API assertion; sleep cannot determine
success.

The Browser plugin was not available in the session, so the manifest and report
record the approved deterministic Playwright fallback using installed Chrome.
The runner listens for actual `securitypolicyviolation` events and captures
console warnings/errors, page errors, CSP violations, and requests outside the
disposable loopback origins. Raw health retains the exact expected HTTP 401
console line from the deliberate wrong-password recovery branch. No other
health entry is allowed.

## Measured baseline

The 44 viewport executions recorded:

- 162 clicks;
- 34 keypresses;
- 40 typed-field occurrences;
- eight branch/decision points;
- maximum modal depth one; and
- seven recovery steps.

These are observations for human review, not universal usability scores or
automatic redesign triggers. No redesign item was opened. Five unrun journeys
remain honest: GUI import/export and batch organization are unsupported in the
current client; editor find/replace belongs to the embedded browser editor;
native shell selection requires Wails; and destructive restore is unsupported
by design.

## Defects exposed and corrected

Real execution found two existing contract failures:

- the Sync Center close button measured 36×36 CSS pixels and now meets the
  documented 44-pixel target; and
- preview HTML left remote HTTP(S) image sources loadable by the browser,
  bypassing the server-side domain, SSRF, quarantine, hash, MIME, and size
  policy. Remote sources are now inert metadata until localization rewrites an
  admitted resource URI.

A focused frontend test pins remote-media inertness. Documentation was also
corrected rather than manufacturing unsupported flows: the current Sync Center
does not control import/export jobs, and the v0.6 batch API has no current GUI
multi-select organizer surface.

## Evidence and mutation gates

`performance/v0.7-g18e/REPORT.json` contains 49 rows: 44 passed viewport
executions and five unrun records, with zero journey and health failures. The
dependency-free validator requires exact manifest/report counts, viewport
coverage, runtime flags, non-vacuous metrics and assertions, unique health
identities, exact transient screenshot paths, matching unrun reasons, and no
unexpected browser-health entry. Nine mutation tests reject empty or changed
labels, missing results or metrics, zero assertions, wrong viewports, duplicate
health identities, external requests, and corrupted unrun reasons.

Transient screenshots at `/tmp/notrios-g18e-desktop.png` and
`/tmp/notrios-g18e-narrow-sync.png` were visually reviewed for page identity,
framework overlays, clipping, responsive dialog bounds, readable navigation,
and the 44-pixel close target. They contain generated fixture content only and
are neither committed nor used as assertions.

## Validation evidence

- Live usage preflight after the weekly reset: 99% five-hour and 100% weekly
  remaining; the earlier 15% reserve pause is recorded in the attempt log.
- Two consecutive final real-browser passes: identical per-journey action and
  assertion records, 49 report rows, 44 passed executions, five unrun rows,
  zero journey failures, and zero health failures.
- G18e manifest/source-anchor target: nine Python mutations, 37 journeys, 52
  production anchors, and one check anchor pass.
- Frontend advisory checks, clean install, typecheck, 172-test suite, and
  production build pass.
- Full Go, vet, documentation/offline, scaffold, required-file, JSON, and
  release-package validation pass.

No real peer, cloud account, private corpus or path, external network fixture,
destructive restore, mobile build, remote, evidence reserve, ISO, push, or
physical burn was used. No unsupported mobile application claim is made.
