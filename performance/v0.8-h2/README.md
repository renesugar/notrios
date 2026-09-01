# v0.8 H2 — Bounded offline Mermaid enablement

Evidence for the implementation of H2a's accepted containment design.

| File | What it is |
|---|---|
| `browser-results.json` | Real-browser results for the production renderer |
| `run.mjs` | Playwright harness |
| `probe_main.ts` | Probe entry point; imports `web/src/mermaid-render.ts` unchanged |
| `serve.py` | Static server applying the app's verbatim production CSP |

## Why a browser probe exists at all

**Mermaid cannot render under jsdom** — it needs layout APIs no DOM shim
provides. Every call fails there, which would make each fallback test pass for
the wrong reason. The unit tests therefore inject a stub renderer and cover this
project's decisions (limits, timeout, sanitisation, fallback, cancellation),
while the probe runs the real `mermaid-render.ts` in Chromium under the exact
production CSP.

## Results

Zero CSP violations, zero cross-origin requests, no script execution. Ordinary
flowchart and sequence diagrams render. Malformed and over-limit sources keep
their fenced source. A label carrying `<img onerror>` and a `<script>` renders
with no `foreignObject`, no script, and no remote fetch.

## The finding the unit tests could not have caught

The first probe run showed `note_link` rendering with **no link at all**.

Mermaid drops a `notrios://` href before the sanitiser here ever sees the SVG:
`formatUrl` passes the URL through, but the renderer's own DOM pass keeps only
the schemes it recognises. A remote `https` link survives that pass and the
user's own note link does not, which is backwards for this product and is what
H2a measured. The allowlist in `sanitizeDiagramSVG` was therefore a no-op for
the one scheme it existed to permit.

A DOMPurify hook on the shared singleton did not reach mermaid's pass. The fix
is to reattach note links from the diagram source after sanitisation: mermaid
wraps a linked node in an `<a>` containing `<g id="{diagramID}-{type}-{node}-{n}">`,
so the declaring node is recoverable. Only `click <node> "<uri>"` directives are
read, only URIs that parse as stable links are kept, and the link becomes
`data-app-uri` with `href="#"` — the same in-app routing every other note link
uses — rather than a real href, because a stable link may name another database
and only the service can resolve it.

The second run confirms it: `app=["notrios://databases/db_a/documents/doc_b"]`
with the remote `https` link absent.

## Reproducing

The probe is disposable and installs nothing into `web/`. Create a scratch
directory, `npm install mermaid@11.17.2 vite typescript playwright`, copy
`web/src/mermaid-render.ts` and `web/src/stable-links.ts` beside `probe_main.ts`
as `src/`, build with Vite, serve `dist/` with `serve.py`, and run `run.mjs`.

## Not covered here

- No Wails v2 webview smoke: it needs a GUI build and a display. The G18
  contract's enablement gate asks for one before claiming desktop support.
- No worker-based rendering; the timeout bounds wall-clock time but a pathological
  diagram still occupies the main thread until mermaid returns.
- No screen-reader assessment. The diagram carries `role="img"` and an
  `aria-label`, and the source is reachable through a `<details>` element, but
  that is a design choice rather than a tested outcome.
