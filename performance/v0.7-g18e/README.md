# G18e GUI journey evidence

`JOURNEYS.json` is the strict `notrios.docjourney.manifest.v1` contract for
the GUI documentation acceptance slice: 37 finite journeys, consisting of 32
executed browser journeys and five explicitly unverified entries. Every
executed journey names a real documentation heading, TypeScript declaration
anchors, its dispatcher case, viewport, and both visible and canonical/API
postconditions. The five unverified entries remain counted and carry a reason
code and owner; they are not silently treated as passes.

The browser policy prefers the Browser plugin. It was absent in this run, so
the evidence uses deterministic Playwright against isolated loopback daemons.
Desktop journeys use 1440x960. Sync Center journeys additionally use the
390x844 narrow viewport. This is a responsive desktop-dialog check, not a
mobile build or Android/iOS support claim.

Fixture creation, daemon setup/teardown, seeded data, and accessibility
navigation are evidence mechanics and are excluded from user action metrics.
The measured task counts cover only the user's visible interactions and
recovery decisions. Assertions must observe the rendered result and then a
real daemon/canonical store or API postcondition. Mutating a label, dispatcher
case, or expected postcondition must fail the harness; this prevents
setup-only or assertion-free journeys from being counted.

No real peer, cloud account, private corpus, destructive restore, physical
media, or mobile package is part of this evidence. The manifest is an audit
input, not a product capability declaration.

## Executed baseline

The checked report contains 44 passing viewport executions and five matching
unrun rows. Twenty desktop-only workspace journeys cover search pagination,
create/edit/move/trash/restore/purge, notebook deletion review, Help protection,
themes, links, resource upload and remote-media refusal, graph/table/query/math
preview, pane resize, and destructive cancellation. Twelve Sync Center journeys
run at both viewports and cover entry/setup, pairing, jobs, lazy resources,
conflicts, backup creation/inspection/recovery review, retention, repairs, and
retirement.

Across those executions the user-action baseline is 162 clicks, 34 keypresses,
40 distinct typed-field occurrences, eight decisions/branches, modal depth one,
and seven recovery steps. These totals deliberately exclude fixture creation,
daemon mechanics, and accessibility setup. They are observations for later
human review, not product targets.

Five procedures remain visible and honestly unverified: GUI import/export and
batch organization are unsupported in the current client; editor find/replace
belongs to the embedded browser editor; native shell selection requires Wails;
and automatic destructive restore is unsupported by design.

The run found two contract defects and fixed them in scope. The Sync Center
close button was 36 by 36 CSS pixels and now meets the documented 44-pixel
target. Preview HTML also allowed a remote `<img>` URL to reach the browser
directly; remote sources are now inert metadata until the server-side media
policy localizes them. The final report observed no unexpected console warning
or error, page error, CSP violation, or external request. The deliberate
wrong-password recovery branch emits the exact expected HTTP 401 console line
and is retained in raw health evidence rather than hidden.

## Reproduction

Build `web/dist`, then run the opt-in Go test with the installed Playwright
module and Chrome:

```sh
cd web && npm run build
cd ..
PLAYWRIGHT_MODULE=file:///home/renes/.npm/_npx/9833c18b2d85bc59/node_modules/playwright/index.mjs \
  NOTRIOS_G18E_BROWSER=1 \
  NOTRIOS_G18E_REPORT=performance/v0.7-g18e/REPORT.json \
  GOCACHE=/tmp/notrios-g18e-gocache \
  go test ./cmd/notriosctl -run '^TestG18eBrowserJourneys$' -count=1 -v -timeout 600s
make g18e-validate
```

Screenshots are transient QA artifacts at `/tmp/notrios-g18e-desktop.png` and
`/tmp/notrios-g18e-narrow-sync.png`; they are intentionally not committed or
treated as result assertions.
