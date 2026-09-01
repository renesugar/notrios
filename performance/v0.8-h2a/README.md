# v0.8 H2a — Mermaid renderer and security investigation

Investigation evidence. **No production dependency, configuration, or feature
status changed.** Mermaid remains disabled (`noMermaid: true` in
`web/src/editor-assets.ts`), and `web/package.json` is untouched. Every install
in this investigation happened in a disposable directory outside `web/`.

## Contents

| File | What it is |
|---|---|
| `REPORT.json` | The machine-checked decision record and recommendation |
| `validate_evidence.py` | Offline validator; re-checks that the conclusions follow from the measurements |
| `probe-results.json` | Raw browser results: CSP violations, requests, per-fixture outcome and SVG audit |
| `probe2-results.json` | Supplementary: dense-graph attempt, both themes, 390px layout |
| `fixtures/` | Thirteen representative and adversarial diagram sources |
| `click-probe-results.json` | Link behaviour under `strict` and `loose` |
| `run_click_probe.mjs` | Harness for the link-scheme comparison |
| `probe_main.js` | The probe's Mermaid integration under test |
| `run_probe.mjs`, `run_probe2.mjs`, `measure_bytes.mjs` | Playwright harnesses |
| `serve.py` | Static server that applies the app's **verbatim** production CSP |

## Reproducing

The probe is disposable by design and is not wired into any gate that installs
packages. To repeat it, create a scratch directory outside the repository,
`npm install mermaid vite playwright`, copy `probe_main.js` in as `src/main.js`,
build with Vite, serve `dist/` with `serve.py`, and run the harnesses.

Validate the archived evidence at any time, offline:

```bash
python3 performance/v0.8-h2a/validate_evidence.py
```

## The short version

Mermaid 11.17.2 renders under the application's exact CSP with **zero
violations** and needs no `unsafe-eval`: the two `new Function` sites in the
dependency tree (`d3-dsv`, `es-toolkit`'s template compiler) are unreachable
from Mermaid and do not survive the Vite bundle.

With Mermaid's **default** HTML labels, a diagram label fetched a remote image
— a tracking-pixel vector — and every rendered diagram emitted `foreignObject`,
the construct the G18 contract asks to refuse. Setting `htmlLabels: false`
alongside `securityLevel: 'strict'` removed both: zero `foreignObject`, zero
cross-origin requests, no script execution, no event-handler attributes.

One vector survives even then: a `click` directive's remote `https` href stays
in the SVG. `javascript:` URLs are already neutralised, but a remote href is not.

**Link navigation is inverted by default.** Under `strict`, a `notrios://` note
link is stripped while a remote `https://` link is kept — backwards for a
local-first application. The cause is DOMPurify's default `ALLOWED_URI_REGEXP`,
which admits no custom scheme; Mermaid's own URL sanitiser passes `notrios://`
through fine. `loose` restores custom schemes but also re-admits `javascript:`
URLs into a clickable href, so it is not usable. H2 should allowlist the
`notrios` scheme and neutralise remote ones, so a diagram navigates to the
user's own notes and a remote URL is reached from the note that contains it.

The cost is real. Mermaid adds about 0.90 MB gzipped against a current bundle
of 0.85 MB, roughly doubling it. Code splitting softens this: one flowchart
transferred 772 KiB uncompressed across 25 chunks rather than the whole
3.21 MB.

Enabling it also trips the project's own fail-closed licence gate three times,
none of which is a genuine licence problem — see `REPORT.json`.

## What was not tested

Recorded in `REPORT.json` under `untested`, and enforced by the validator so it
cannot be quietly dropped:

- **The 2000 ms render deadline was never exercised by a genuinely slow
  successful render.** Mermaid's own `maxEdges: 500` guard refuses large graphs
  first, so every attempt to build a slow diagram was rejected before layout.
  The deadline policy is therefore reasoned, not measured.
- No Wails v2 webview smoke: it needs a GUI build and a display.
- No worker-based rendering or cancellation prototype.
- No keyboard or screen-reader assessment of the rendered preview.
- The revised link policy was verified at the sanitiser level and in the
  rendered SVG, but no in-app `notrios://` click was driven end to end.
