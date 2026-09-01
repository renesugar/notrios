# v0.8 H2a — Mermaid renderer and security investigation

Date: 2026-09-01
Status: complete investigation; recommendation issued, no production change
Model: Claude Opus 5 (`claude-opus-5`) via Claude Code 2.1.252, parent-owned
throughout; no subagents were used.

## Outcome

**Recommendation: enable Mermaid in H2, under a specific containment design.**
The premise that motivated the caution holds only for Mermaid's *default*
configuration; a constrained configuration meets every guarantee the G18
contract asks for, with one residual vector H2 must close explicitly and a
bundle cost that is real and quantified.

Mermaid remains disabled. `web/package.json` and `web/src/editor-assets.ts` are
untouched; every install happened in a disposable directory outside `web/`.

## The decisive finding: no CSP widening is needed

The dependency tree contains dynamic code construction — `d3-dsv` builds a CSV
row accessor with `new Function`, and `es-toolkit`'s lodash-style template
compiler does the same. Either would require `script-src 'unsafe-eval'`, which
the recommendation criteria forbid.

Neither is reachable. `es-toolkit/compat/string/template` is not referenced by
Mermaid at all, `d3-dsv` arrives only through the `d3` umbrella package, and
**zero `new Function` occurrences survive the Vite bundle**. Rendering every
fixture in Chromium under the application's verbatim production CSP produced
**zero violations**.

This was worth measuring rather than assuming in either direction: the sources
really do contain the construct, and only the built artifact and a real browser
settle whether it matters.

## The security finding: default configuration is not acceptable

With Mermaid's default HTML labels:

- a diagram label containing `<img src="https://…">` **fetched the remote
  image** — two cross-origin requests from the fixture alone. The application's
  CSP permits `img-src https:` deliberately, because the Markdown preview may
  display remote images, so CSP does not stop this. The remote-media policy
  governs Markdown images, not diagram labels, so nothing else stops it either;
- every rendered diagram emitted `foreignObject`, which the G18 contract lists
  among the constructs to refuse. Mermaid uses it legitimately for rich text
  labels, so "refuse foreignObject" and "render with default labels" are
  contradictory requirements.

Setting `htmlLabels: false` alongside `securityLevel: 'strict'` resolved both:
zero `foreignObject`, **zero cross-origin requests**, no script execution, no
event-handler attributes, no iframes. Labels become SVG `<text>`.

**One vector survives.** A `click` directive's remote `https` href stays in the
rendered SVG. `javascript:` URLs are already neutralised by Mermaid's bundled
`@braintree/sanitize-url`, but a remote href is not, so H2 must strip or
neutralise every `href` and `xlink:href` after rendering.

Mermaid also crashed with an internal `TypeError` on one dense graph rather
than raising a clean error. The integration must therefore catch **any** throw
and fall back to source, not merely handle known error types.

## Built-in guards

Mermaid enforces `maxEdges: 500` and `maxTextSize: 50000` itself, refuses
over-limit input with a clear message, and **cannot be reconfigured from inside
the diagram source**. H2 lowers these to the product bound rather than building
complexity limits from scratch.

## Cost

| | chunks | raw | gzip |
|---|---:|---:|---:|
| Current production bundle | 114 | 2.45 MB | 0.85 MB |
| Mermaid bundle | 91 | 3.21 MB | 0.90 MB |

Adding Mermaid roughly doubles the shipped bundle. Code splitting softens the
practical cost: rendering one flowchart transferred 662 KiB to load the module
graph and 772 KiB in total across 25 chunks, uncompressed, rather than the whole
3.21 MB. H2 should load Mermaid only when a diagram is present.

## Licence gate

Enabling Mermaid trips the project's own fail-closed G20 licence gate three
times. None is a genuine licence problem, and all three need a reviewed
decision before H2 can pass its gates:

- **`khroma@2.1.0`** ships an MIT licence file but omits `license` from its
  `package.json`, so a metadata scanner reports `UNKNOWN` and the gate refuses
  it. Fix: a reviewed override keyed to the licence file hash
  (`66b333b0…961b49`).
- **`dompurify@3.4.14`** declares `(MPL-2.0 OR Apache-2.0)`. Both branches are
  individually allowed, but the checker does exact set membership rather than
  SPDX expression resolution. Fix: teach it to resolve a disjunction when every
  branch is allowed.
- **`robust-predicates@3.0.3`** declares `Unlicense`, a public-domain
  equivalent absent from the allowed set — though the project already accepts
  public domain for SQLite. Fix: add it as a reviewed decision.

113 transitive packages in total: 70 MIT, 33 ISC, 6 BSD-3-Clause, 1 Apache-2.0,
plus the three above.

## Recommended H2 design

- Pin `mermaid@11.17.2` exactly and bundle it locally; no CDN, no runtime fetch.
- Configure `securityLevel: 'strict'`, `htmlLabels: false` everywhere,
  `startOnLoad: false`, `suppressErrorRendering: true`, and lowered `maxEdges`
  and `maxTextSize`.
- Sanitise the produced SVG before insertion; refuse `script`, `iframe`,
  `foreignObject`, and `on*` attributes; strip every `href` and `xlink:href`.
- Catch every throw, including internal `TypeError`s, and always fall back to
  the visible fenced source. Never an empty box.
- Load Mermaid by dynamic import only when a diagram is present.
- Rollback is restoring `noMermaid: true`. No data, schema, or stored content
  is involved.

## Validation and evidence

Evidence is under `performance/v0.8-h2a/`; validate offline with
`python3 performance/v0.8-h2a/validate_evidence.py`. The validator re-checks
that the recommendation still follows from the recorded measurements, and was
verified to fail both on a falsified CSP claim and on a quietly deleted caveat.

Ten representative and adversarial fixtures were exercised in Chromium under
the verbatim production CSP. Malformed source and an over-limit graph both
correctly failed to render. Both themes render; a 390px viewport does not
overflow, because Mermaid emits `width="100%"`.

## What was not tested

Recorded in `REPORT.json` and enforced by the validator so it cannot be quietly
dropped:

- **The 2000 ms render deadline was never exercised by a genuinely slow
  successful render.** Mermaid's own `maxEdges` guard refuses large graphs
  before layout, so every attempt to construct a slow diagram was rejected
  first. The deadline policy is reasoned, not measured; H2 should treat it as
  unvalidated until a real slow render is found.
- No Wails v2 webview smoke; it needs a GUI build and a display. The G18
  contract's enablement gate requires this before `noMermaid` changes.
- No worker-based rendering or cancellation prototype, so the abort story is
  designed but unproven.
- No keyboard or screen-reader assessment of the rendered preview.

## Open decision, resolved

- **Which renderer and containment design should H2 implement?** Resolved as a
  recommendation, non-blocking for H2a and blocking for H2: Mermaid 11.17.2
  with the containment design above. H2a explicitly considered recommending
  that Mermaid stay disabled and did not, because the measured configuration
  meets the offline, CSP, and sanitisation guarantees. Approving H2 also means
  accepting the roughly doubled bundle and the three licence-gate decisions.
