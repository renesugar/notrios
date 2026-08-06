# v0.5 E6a — Offline-first frontend assets

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

Evidence: `performance/v0.5-e6a/`.

## Why

A defect, not a feature. Notrios binds to loopback, quarantines remote images
behind a media policy, and refuses to fetch a byte during a static media scan.
Meanwhile the frontend loaded remote **executable JavaScript** from a
third-party CDN, unconditionally, on every launch — in the desktop app too.

Opening one note issued **13 requests to `unpkg.com` totalling 623 kB**: KaTeX
and three fonts, highlight.js and a theme, echarts, cropperjs, and prettier
twice. With the CDN unreachable, `$E = mc^2$` rendered as its own LaTeX source
and code blocks lost highlighting — silently, with no error and no placeholder.

The question that surfaced it was a user asking whether the editor renders math.
It does. It just needed the internet to do it.

## What it does

Everything the editor would fetch is now either bundled or turned off.

| Library | Before | After |
|---|---|---|
| KaTeX | CDN script + css + fonts | bundled instance |
| highlight.js | CDN script + theme css | bundled `lib/common` instance |
| cropperjs | CDN script + css | bundled instance |
| echarts | CDN script | `noEcharts` |
| prettier ×2 | CDN scripts | `noPrettier` |

Plus a `Content-Security-Policy` served with the UI, and a regression check that
fails if any of it comes back.

## Decisions worth recording

**Supplying an instance is what stops the injection.** `md-editor-rt` skips both
the script *and* the stylesheet when `editorExtensions.<lib>.instance` is set —
that is why `editor-assets.ts` imports the CSS explicitly. Getting this half
right and the other half wrong would have left the stylesheet still coming from
unpkg.

**A CSP, not just a test.** A test catches a regression after the fact; a policy
the browser enforces prevents one. `script-src 'self'` is the directive that
matters. `style-src` needs `'unsafe-inline'` because CodeMirror and md-editor-rt
inject `<style>` elements at runtime — external stylesheets are still refused,
which is what a CDN would need. `font-src` needs `data:` because the bundler
inlines the smallest font files; those bytes ship in our own assets.

**Cropper is bundled rather than disabled.** Turning it off requires
`noUploadImg`, which takes image upload with it. Trading one working feature for
another is not what "work offline" was supposed to mean.

**echarts and prettier are turned off rather than bundled.** Notrios has no
charting feature, and a charting library that evaluates a code block's contents
is not something to carry for a feature nobody asked for. Prettier reformats
Markdown, and block identity is content-derived (`PROJECT_DECISIONS.md` 17), so
a reformat would remint every anchor in the note.

**`highlight.js/lib/common`, not the full package.** ~40 languages instead of
~190. An unlisted language degrades to plain text rather than to an error.

**The four dead dependencies are gone.** `remark-gfm`, `remark-math`,
`rehype-katex`, and `rehype-sanitize` were declared and imported nowhere, and
could not be plugged into md-editor-rt, which renders through markdown-it.
Removing `rehype-katex` is also what made `katex` a direct dependency rather than
something present by accident: it was only installed because that dead package
depended on it.

## The regression guard, verified in both directions

`scripts/run_offline_assets_check.sh` drives real headless Chrome with the cache
disabled and every known CDN blocked. It fails on a cross-origin request, an
injected remote script or stylesheet, a CSP violation, or math that did not
render — that last one matters, because "no external requests" also passes on a
completely broken page.

It was run against the pre-fix commit and **failed**, naming all 13 requests and
the missing KaTeX output; and against the fixed tree and **passed**. A check that
cannot fail proves nothing.

## The measurement, including the part that got worse

| | Third-party requests | Third-party bytes | Eager bundle (gzip) | FCP | keystroke p50 |
|---|---:|---:|---:|---:|---:|
| before | 13 | 623.0 kB | 275.4 kB | 1036 ms | 30.0 ms |
| after | 0 | 0 | 426.5 kB | **1452 ms** | 29.7 ms |

First contentful paint is **~400 ms slower** and DOMContentLoaded ~240 ms
slower, consistently across three runs. That is real, it is the cost of parsing
151 kB more JavaScript, and it is reported rather than buried.

Typing is unchanged at p50. The p95 went bimodal (124.3 / 56.3 / 127.1 ms
against a tight 66.8 / 74.3 / 67.4 before) and is recorded as **unresolved**:
something occasionally stalls and three runs cannot say what.

The comparison also flatters the before column, which had a fast working
connection to unpkg.com — the best case for the arrangement being removed.
Offline it does not render math at all; on a slow link 623 kB costs far more
than 400 ms; and on every launch it told a third party that someone opened a
note.

## What is not measured

The Wails webview, for the same reason as E6: WebKitGTK does not speak the
DevTools Protocol these harnesses use. The CSP is delivered by `handleWebApp`,
and the Wails asset server routes every webview request through that same
handler, so the header applies there by construction rather than by measurement.

## Validation

`go vet ./...`, `go test ./...` (including the new CSP header test), required
files, plan-loop, scaffold validation, OpenAPI parse, migration-copy equality,
web typecheck/tests/build, `make gui`, docs-site build, Help reseed, REST/MCP
smoke, performance smoke, `scripts/run_offline_assets_check.sh` in both
directions, and three editor-profile runs.
