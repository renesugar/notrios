# v1.0 J45: the web UI's dependency advisories, cleared

J44's archive build stopped at `npm audit` in `web/`. Six advisories had been
published since J42's archive, which was the last time the release gate ran
clean. `docs-site/` had none. Nothing was skipped to get past the gate. The gate
is unchanged, and it now passes because there is nothing left to report.

## J45-A: what a lockfile change could clear

`npm audit fix`, without `--force`, moved four packages and nothing else.
`package.json` is unchanged.

| package | before | after | how the UI reaches it |
|---|---|---|---|
| undici | 7.29.0 | 7.30.1 | it does not: only through `jsdom`, a test dependency |
| source-map-js | 1.2.1 | 1.2.2 | it does not: through `postcss` under `vite`, and `css-tree` under `jsdom` |
| markdown-it | 14.3.0 | 14.3.3 | the preview's renderer. `md-editor-rt` builds it with `linkify: true`, which is the configuration the advisory needs |
| dompurify | 3.4.14 | 3.4.17 | bundled with mermaid |

**DOMPurify's `IN_PLACE` mode was never in use.** Both DOMPurify advisories are
about it. mermaid 11.17.2 and 12.2.0, each read in its installed build, call
DOMPurify with the `dompurifyConfig` they were given, which Notrios does not set,
with `FORBID_TAGS: ['style']` alone, or with no options at all, for titles and
tooltip text. Nothing in `web/src` calls DOMPurify.

## J45-B: katex 0.18.11 and mermaid 12.2.0

**katex** goes from 0.16.47 to 0.18.11. The advisory affects versions before
0.18.2. mermaid 12.2.0 asks for `^0.18.0`, so 0.18.11 is a single copy shared
with mermaid. 0.19.0 would have meant two copies, and it is no further outside
the advisory's range. `src/__tests__/preview-math.test.tsx` renders inline and
display math through the real preview pane, its sanitiser and the bundled KaTeX.
It passes on 0.16.47 and on 0.18.11, and before this nothing tested math at all.

**mermaid** goes from 11.17.2 to 12.2.0. It was listed only because its own
`katex` range admitted no fixed version. 12.2.0 was published on 2026-10-13, six
days before it was pinned here. Nothing in the repository sets a minimum age.

### What the new major version brought

- **elkjs 0.9.3, which is EPL-2.0.** It was bundled as a 1.47 MB chunk loaded for
  any diagram using an ELK layout. mermaid 11 shipped no ELK at all. EPL-2.0 is
  weak copyleft, and the rule is MIT/Apache-2.0 compatible, so the owner decided
  on 2026-10-19 that it stays out.

  **It is replaced at install.** `web/elk-refused/` is a local Apache-2.0 package
  named `elkjs`, whose `layout` rejects. `package.json` depends on it, and an
  npm override (`"elkjs": "$elkjs"`) makes mermaid's dependency resolve to it.
  A clean `npm ci` installs the stub, and no EPL code is downloaded, installed
  or bundled. The ELK chunk left in the build, 34,848 bytes, is mermaid's own MIT
  adapter calling the stub. Neither elkjs's licence header nor its
  `org.eclipse.elk` package names appear anywhere in `dist/`.
  `src/__tests__/elk-refused.test.ts` checks three things: that the installed
  `elkjs` is the stub; that every layout is refused; and that mermaid imports
  elkjs by that one path, so a mermaid that moved the import fails the test
  instead of reaching past the stub.

  **A first approach was abandoned.** A Vite alias that pointed mermaid's import
  at a stub kept ELK out of the bundle, and its test was shown to fail without
  the alias. It still left elkjs in `node_modules` and in the lockfile, though,
  and G20's licence gate refused it there. That was the gate working: it checks
  every lockfile package against its allowlist. The scaffold validation caught
  it; the web tests alone could not.
- **The licence gate learned what a link is.** The lockfile records the
  replacement as a link entry, `node_modules/elkjs`, with no licence field,
  pointing at the `elk-refused` entry, which has one. The gate refused the link
  for having no licence. `check_dependency_licenses.py` now checks a link
  through its target, under the link's own name, and fails a link whose target
  the lockfile does not list. This is a rule about links, not an exemption: the
  allowlist is unchanged, and the stub is checked like any other package. Three
  new tests cover it: a GPL target is refused, a dangling link is refused, and
  the link is checked under its own name rather than passing because its target
  was. A link is also no longer counted as a package, because it isn't one.
- **ELK became the default layout.** mermaid 12's defaults are `layout: 'elk'`,
  and again for state diagrams. With ELK stubbed, the first probe run rendered
  only the sequence diagram: every flowchart was refused. `mermaid-render.ts`
  now sets `layout: 'dagre'` and `state: { layout: 'dagre' }`, which is what
  mermaid 11 used. `src/__tests__/mermaid-config.test.ts` pins the whole
  configuration H2a measured, with these two keys added. A diagram that asks for
  ELK itself fails like any other diagram that will not render: its source stays
  visible.

Every other key the preview sets is still declared in mermaid 12's
configuration type, so the type check would have caught a dropped one. The probe
shows three of them still take effect:
- `securityLevel: 'strict'`: the click results are identical.
- `htmlLabels: false`: no `foreignObject` anywhere. In 12 the root-level key
  takes precedence over the per-diagram ones it deprecates, and the root-level
  key is the one set.
- `maxEdges`: the same refusal.

`suppressErrorRendering` and `maxTextSize` were not exercised separately.

### H2a's probe, re-run against 12.2.0

These are H2a's harnesses, unchanged, run with Playwright 1.62.0, whose Chromium
build was already installed. The page was served under the production CSP as it
now stands. That CSP has gained `media-src 'self'` since H2a copied it; `serve.py`
here carries the current header. `probe_main.js` is H2a's integration with the
two layout keys added. `probe_package.json` shows the probe installed with the
same stub and override as `web/`. Two fixtures are new: `elk_layout.mmd` and
`state_basic.mmd`. The probe was run twice, once with the abandoned alias and once
with the replacement. The render outcomes and SVG audits are identical, and the
files here are from the second run.

| measure | H2a, mermaid 11.17.2 | J45, mermaid 12.2.0 |
|---|---|---|
| CSP violations | 0 | 0 |
| cross-origin requests | 0 | 0 |
| injected script executed | no | no |
| page errors | 0 | 0 |
| `foreignObject` in any rendered SVG | 0 | 0 |
| event-handler attributes, remote images | none | none |
| click links under `strict` and `loose` | — | identical to H2a's, field for field |
| remote `https` href kept by mermaid | yes | yes, still neutralised afterwards by `sanitizeDiagramSVG` |
| malformed source, edge limit + 1 | refused | refused, same errors |
| dense graph | fails inside mermaid, under 2 s | the same |
| default and dark themes, 390 px width | render, no overflow | the same |
| ELK fixture | — | refused by the stub |
| state diagram | — | renders with dagre |

The raw results are `probe-results.json`, `probe2-results.json` and
`click-probe-results.json`. H2a's own results covered ten fixtures, and the
fixture set has grown since, so the extra rows here have no H2a counterpart. The
"click links" row compares them with H2a's click probe, which did cover them.

### Bundle size

| | before | after | change |
|---|---|---|---|
| JavaScript | 5,740,789 | 6,033,588 | +292,799 |
| CSS | 129,838 | 130,805 | +967 |
| fonts and other files | 1,666,644 | 1,666,644 | 0 |
| total | 7,537,271 | 7,831,037 | +293,766 (+3.9%) |

The total without the stub would have been 9,262,467 bytes, because ELK was
bundled.

## J45-C: the gate passes

`npm audit` finds 0 vulnerabilities in `web/` and in `docs-site/`. With the
audit clean, `scripts/package_release.sh` ran to the end from `4fe08b7`, the
J45-B commit. It ran `npm ci`, `npm audit` and the web build in both packages,
the scaffold validation and the full test suite. The ZIP it produced,
`notrios-v1.0-j44-4fe08b7.zip`, is J44's archive: 26,609,076 bytes and 2,977
entries, accepted by `check_release_zip.py`. That is the run J44's close waited
on.

The ZIP carries the built web UI. Every file in it was searched for elkjs's
licence text and its `org.eclipse.elk` package names, and the only match was this
record, which quotes them.

## Checks

- 305 web tests pass, after a clean `npm ci`, as do the type check and the build.
- `npm audit` finds 0 vulnerabilities in `web/` and `docs-site/`.
- G20's licence gate passes: 39 Go modules and 365 npm packages. The web
  lockfile went from 353 packages to 357. chevrotain and four `@chevrotain`
  packages came with mermaid 12's new parser, all Apache-2.0. katex 0.18 dropped
  `fastdom` and `strictdom`. elkjs is now the stub. Its link is a lockfile entry
  as well, and the gate no longer counts link entries, so the count matches what
  the release SBOM lists. v0.9 I6's record checks its SBOM counts against this
  inventory, so it moves from 361 to 365 npm components, with a dated note. Its
  artifact hashes are those of the set it was built from, which was not rebuilt.
- **J45-A's commit, `270d50c`, does not pass the scaffold validation on its
  own.** It changed `web/package-lock.json` without updating the hash in G20's
  inventory, because only the web checks were run for it. The inventory is
  brought up to date here.

## What this does not prove

- **The gate checks what the lockfile says a licence is.** elkjs was refused
  because its lockfile entry says EPL-2.0. A package that misstated its licence
  in `package.json` would pass, as it always could.
- **The release set's SBOM will list the stub as `pkg:npm/elkjs@0.0.0-notrios`.**
  `performance/v0.9-i6/build_release_set.py` names npm components from the
  lockfile, and no such package exists on npm. Its licence there is correct,
  Apache-2.0. Saying it is a local package is left to the release work.
- **The probe is H2a's integration, not the application.** It mirrors the
  preview's mermaid configuration, the CSP and the install, but
  `sanitizeDiagramSVG`'s handling of the remote href is shown by
  `mermaid-render.test.ts`, as before, not by this browser run. That an ELK
  diagram keeps its source visible is shown the same way: a refused render is an
  `invalid` outcome, and those tests cover what the preview does with one.
- **mermaid 12's other visual changes**, such as the `neo` look and the
  `redux-color` theme default for state diagrams, were not reviewed by eye.
