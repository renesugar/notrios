# v0.8 H2 — Bounded offline Mermaid enablement

Date: 2026-09-01
Status: complete
Model: Claude Opus 5 (`claude-opus-5`) via Claude Code 2.1.252, parent-owned
throughout; no subagents were used.

## Outcome

Mermaid diagrams render in the GUI, locally and bounded, under the containment
H2a recommended. `mermaid@11.17.2` is pinned exactly and bundled; nothing is
fetched at render time; a diagram cannot run script, embed a page, or load a
remote image; and every failure leaves the reader the fenced source.

## Where the rendering happens, and why

`md-editor-rt` keeps `noMermaid: true`. Diagrams are rendered by
`web/src/mermaid-render.ts` in a post-pass over the already-rendered preview,
the same way `note-query.ts` fills embedded query blocks.

That placement is the design. Handing an instance to the editor would have made
the library's own choices the product's security contract; rendering afterwards
means this file owns the limits, the sanitisation, the cancellation, and the
fallback. It also makes "preserve the source on failure" the default state
rather than an error path: the fenced block is already on the page, and a
diagram replaces it only on success.

## Containment

Configured: `securityLevel: 'strict'`, `htmlLabels: false` everywhere,
`startOnLoad: false`, `suppressErrorRendering: true`, `maxEdges: 500`,
`maxTextSize: 50000`.

Sanitised afterwards regardless, because configuration is not a contract:
forbidden elements (`script`, `iframe`, `object`, `embed`, `foreignObject`,
`link`, `base`, `meta`, `animate`, `set`, `handler`), event handlers in any
casing, `style` elements or attributes that can reach the network via `url()` or
`@import`, media elements pointing anywhere remote, and every link destination
that is not one of the user's own notes.

Bounded: 20 diagrams per note, 65,536 source bytes, a 2,000 ms wall-clock
budget. Mermaid's own `maxEdges` and `maxTextSize` are lowered rather than
reinvented, and neither can be raised from inside a diagram.

## Two defects found while building it

**Sanitiser ordering.** The attribute pass stripped a remote `href` before the
media pass ran, so the media pass saw an element with nothing to judge it by and
left an empty `<image>` behind. Element removal now runs first. A unit test
caught this.

**Mermaid drops note links before the sanitiser sees them.** The first browser
run showed a `click A "notrios://…"` directive producing no link at all.
`formatUrl` passes the URL through, but the renderer's own DOM pass keeps only
schemes it recognises: a remote `https` link survives and the user's own note
link does not — backwards for this product, and exactly what H2a measured. The
allowlist in `sanitizeDiagramSVG` was therefore a no-op for the one scheme it
existed to permit. A DOMPurify hook on the shared singleton did not reach
mermaid's pass.

Note links are now reattached from the diagram source. Mermaid wraps a linked
node in an `<a>` containing `<g id="{diagramID}-{type}-{node}-{n}">`, so the
declaring node is recoverable. Only `click <node> "<uri>"` directives are read,
only URIs that *parse* as stable links are kept — `isStableLink` alone accepts a
malformed `notrios://`, which would give the reader a link that fails on click —
and the result becomes `data-app-uri` with `href="#"` rather than a real href,
because a stable link may name another database and only the service can resolve
it. That is the same routing every other in-app link already uses.

**The unit tests could not have caught this**, which is the more useful lesson:
see below.

## Mermaid cannot render under jsdom

It needs layout APIs no DOM shim provides, so every call fails there. Left
alone, that would make each fallback test pass for the wrong reason — the same
vacuous-test failure mode H1 hit.

The renderer is therefore injectable. Unit tests pass a stub and cover this
project's decisions: limits, timeout, sanitisation, fallback, cancellation,
double-render protection, and an internal `TypeError` of the kind H2a saw
mermaid throw. Real rendering is verified separately in Chromium under the
verbatim production CSP.

## Licence gate

The three decisions H2a identified are implemented in
`performance/v0.7-g20/check_dependency_licenses.py` and recorded in
`DEPENDENCY_LICENSES.json`:

- `Unlicense` joins the allowed set as public-domain equivalent; the project
  already redistributes public-domain code in the vendored SQLite amalgamation.
- An SPDX `OR` expression passes when every branch is allowed, since a
  redistributor may pick any branch. `AND` is deliberately not resolved.
- `khroma@2.1.0` ships an MIT licence file but declares nothing in metadata. The
  review is recorded with the file's SHA-256, and the gate **re-hashes the
  installed file**, so the exemption cannot widen and a changed licence forces a
  new review.

Each is a specific judgement about a specific package rather than a looser
matcher, so the next genuine problem still fails.

## Validation

- 211 frontend tests, up from 184. Typecheck, production build, and `npm audit`
  with zero vulnerabilities pass.
- Regression checks: disabling the href allowlist fails two tests; discarding
  the source on failure fails another.
- Browser probe under the production CSP: **zero CSP violations, zero
  cross-origin requests, no script execution, no `foreignObject`**. Flowchart and
  sequence render; malformed and over-limit keep their source; a label carrying
  `<img onerror>` and `<script>` renders inert; the note link appears as
  `data-app-uri` with the remote link absent.
- `go test ./...`, `go vet`, scaffold validation, and the licence gate pass.
- Evidence under `performance/v0.8-h2/`.

## Cost

The bundle went from 0.85 MB to 1.68 MB gzipped, close to H2a's estimate of
roughly double. Mermaid loads by dynamic import, so a reader who never opens a
note containing a diagram does not download it.

## Rollback

Remove the `renderMermaidBlocks` call from `PreviewPane`. `noMermaid: true` is
already set, so the editor path needs no change, and no data, schema, or stored
content is involved. Removing the dependency additionally means reverting the
`package.json` pin and the licence-gate inventory.

## Not covered

- **No Wails v2 webview smoke.** It needs a GUI build and a display. The G18
  contract's enablement gate asks for one before desktop support is claimed, so
  desktop rendering is expected to work but is unverified.
- **No worker-based rendering.** The timeout bounds wall-clock time, but a
  pathological diagram still occupies the main thread until mermaid returns.
  H2a's "bounded abort" is therefore partial: the reader is never left waiting,
  but the work is not actually cancelled.
- **No screen-reader assessment.** The diagram carries `role="img"` and an
  `aria-label`, and the source is reachable through `<details>`, but that is a
  design choice rather than a tested outcome.
- The 2,000 ms budget remains the reasoned figure H2a could not validate,
  because mermaid's own `maxEdges` guard refuses large graphs before layout.
