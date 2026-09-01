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
`@braintree/sanitize-url`, but a remote href is not. See the link-navigation
section below for what H2 should do about it, which is not what an earlier draft
of this document said.

Mermaid also crashed with an internal `TypeError` on one dense graph rather
than raising a clean error. The integration must therefore catch **any** throw
and fall back to source, not merely handle known error types.

## Link navigation: the stock behaviour is inverted

A follow-up question asked whether a `click` directive may reach a `notrios://`
note link, so that a diagram navigates to one of the user's own notes and the
user decides about any remote URL from there, having read it in context. The
answer is yes, and finding out why took measuring rather than reasoning.

Under the recommended `securityLevel: 'strict'`:

| `click` target | Result |
|---|---|
| `notrios://databases/…/documents/…` | **href stripped** |
| `https://example.invalid/remote` | **href kept** |
| `javascript:…` | stripped |

Strict mode removes the product's own note links and preserves the remote ones,
which is backwards for a local-first application. Under `securityLevel: 'loose'`
all three survive, including `javascript:window.__pwned=3` sitting in a
clickable href, so `loose` is not usable.

**The cause is DOMPurify, not Mermaid.** Mermaid's own URL sanitiser,
`@braintree/sanitize-url`, passes `notrios://` through unchanged and maps
`javascript:` and `data:` to `about:blank`. DOMPurify's default
`ALLOWED_URI_REGEXP` admits `http`, `https`, `mailto`, `tel` and similar but no
custom scheme, so it is what drops `notrios://` while keeping `https://`.
Adding `notrios` to that expression restores the link, and `javascript:` stays
refused with and without the change. Both behaviours were verified directly
against the sanitisers as well as in the rendered SVG.

The documented alternative of putting an `<a href>` inside a node label needs
`htmlLabels: true`. Under the recommended configuration it emits **no anchor at
all**, which is the same setting that closes the remote-image vector.

This supersedes an earlier draft of the recommendation that said to strip every
`href`. That would have deleted the product's own note links while leaving
nothing useful in their place, and it misattributed the stripping to the wrong
layer.

### The policy depends on note links working, and today they do not

De-linking a remote URL in a diagram is only reasonable because the reader can
reach it from the note that contains it, where they see it in context first.
That flow assumes clicking a remote link in a note opens a browser. It does in
the loopback web UI, where `preview-utils.tsx` sets `target="_blank"` and
`rel="noreferrer"`. **It does not in the Wails desktop window.**

Wails v2.13.0's Linux webview connects signals for script messages, context
menu, button press and release, load-changed, drag, and window delete, but
neither `create` nor `decide-policy`. WebKitGTK emits `create` for a
`target="_blank"` click; with no handler its default returns NULL and the click
is silently swallowed. `cmd/notrios/gui_wails.go` adds no link handling of its
own, and no `BrowserOpenURL` call exists anywhere in the repository.

The mechanism is available: `window.runtime.BrowserOpenURL(url)` is present in
the Wails desktop JS runtime and is implemented in Go through
`github.com/pkg/browser`.

This was traced through the Wails module source and the compiled desktop
runtime, **not** observed in a running GUI, which would need a `make gui` build
and a display.

If it stays unfixed, H2's link policy puts a remote URL two hops from the reader
rather than one, because neither the diagram link nor the note link opens. It is
a desktop defect independent of Mermaid and is tracked separately as `PLAN.md`
H2b, so the fix is not hostage to Mermaid approval.

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
  `foreignObject`, and `on*` attributes.
- **Link policy:** extend the DOM sanitiser's `ALLOWED_URI_REGEXP` to admit the
  `notrios` scheme so a diagram can link to the user's own notes; drop or
  de-link every remote `href` and `xlink:href` so a diagram cannot navigate off
  the machine; keep `javascript:` and `data:` refused by both the URL sanitiser
  and the DOM sanitiser. A remote URL is then reached from the note it is
  written in, where the reader can see it before following it.
- Resolve a `notrios://` click through the in-app stable-link resolver that
  `web/src/api.ts` and `web/src/App.tsx` already use, not browser navigation.
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
- The revised link policy was verified at the sanitiser level and in the
  rendered SVG, but no in-app `notrios://` click was driven end to end through
  the stable-link resolver.
- The desktop external-link finding was traced through Wails source rather than
  observed in a running GUI.

## Open decision, resolved

- **Which renderer and containment design should H2 implement?** Resolved as a
  recommendation, non-blocking for H2a and blocking for H2: Mermaid 11.17.2
  with the containment design above. H2a explicitly considered recommending
  that Mermaid stay disabled and did not, because the measured configuration
  meets the offline, CSP, and sanitisation guarantees. Approving H2 also means
  accepting the roughly doubled bundle and the three licence-gate decisions.
- **May a diagram link to a note? — Answered 2026-09-01, after the first draft.**
  Yes: allowlist the `notrios` scheme in the DOM sanitiser and neutralise remote
  schemes, rather than stripping every link. The stock strict behaviour does the
  opposite of what this product wants, so this is a deliberate configuration
  choice H2 must implement and test, not a default it inherits.
