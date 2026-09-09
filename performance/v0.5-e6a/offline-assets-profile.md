# v0.5 E6a — offline-first frontend assets

Date: 2026-08-06. Generated data only; the profile note is synthetic.

Reference machine: Intel Core i5-9300H, Linux/amd64, Chrome 151.0.7922.75
(headless, `--disable-gpu`), Node 26.3.0.

## What changed

`md-editor-rt` does not bundle its optional libraries: its default config points
at `https://unpkg.com/...` and injects `<script>`/`<link>` tags at runtime.
KaTeX, highlight.js, and cropper are now supplied as local instances, echarts
and prettier are turned off, and the service serves a Content-Security-Policy.

## The trade, measured both ways

| | Third-party requests | Third-party bytes | Eager bundle (gzip) |
|---|---:|---:|---:|
| before | 13 | 623.0 kB | 275.4 kB |
| after | **0** | **0** | 426.5 kB |

The bundle grew by **151 kB gzipped** and the page stopped downloading
**623 kB** from `unpkg.com`. Net bytes went down, and every remaining byte comes
from the machine the note is stored on.

Where the eager growth went, from the packages themselves:

| | raw |
|---|---:|
| `katex.min.js` | 272.5 kB |
| `katex.min.css` | 23.8 kB |
| `cropper.min.js` | 37.4 kB |
| `highlight.js/lib/common` | ~40 languages, entry 2.3 kB plus grammars |
| KaTeX woff2 fonts | 259.8 kB in 20 files, **lazy** — the browser fetches only the faces a formula uses |

## First paint regressed; typing did not

Three runs per arm, 206,549-character note. "before" is commit `c623307`.

| Arm | first-paint | FCP | DOMContentLoaded | keystroke p50 | keystroke p95 |
|---|---:|---:|---:|---:|---:|
| before | 204 ms | 1036 ms | 614 ms | 30.0 ms | 67.4 ms |
| after | 180 ms | **1452 ms** | **853 ms** | 29.7 ms | 124.3 ms |

Medians of three; individual runs are in the committed JSON.

**First contentful paint is ~400 ms slower and DOMContentLoaded ~240 ms slower.**
Both moved consistently across all three runs, so unlike most numbers in this
repository's profiles these are signal rather than noise. That is the cost of
parsing 151 kB more JavaScript, and it is the honest price of the fix.

**Typing is unaffected at p50** (30.0 → 29.7 ms). The p95 column is *not*
reported as a regression, because the three runs were 124.3, 56.3, and 127.1 ms
— bimodal, against a tight 66.8/74.3/67.4 before. Something occasionally stalls
and three runs cannot say what. It is recorded as unresolved rather than
averaged into a conclusion.

## Why the comparison flatters the "before" column

The before measurement ran with a **fast, working connection to unpkg.com**.
That is the best case for the arrangement being replaced, and it is not the case
the change was made for:

- offline, the before column does not render math at all — `window.katex` is
  undefined and `$E = mc^2$` displays as its own LaTeX source, silently;
- on a slow link, 623 kB of third-party download costs far more than 400 ms;
- on every launch, the before column told `unpkg.com` that someone opened a note.

400 ms of local parse against that is the trade this slice makes deliberately.

## The regression guard

`scripts/run_offline_assets_check.sh` drives real headless Chrome with the cache
disabled and every known CDN blocked, then fails on any cross-origin request,
any injected remote script or stylesheet, any CSP violation, or math that did
not render.

It was verified to fail on the code it was written to catch. Against commit
`c623307` it reports:

```
FAIL:
  third-party requests: …unpkg.com/katex@0.16.33/dist/katex.min.js, … (10 URLs)
  remote scripts injected: … (6)
  remote stylesheets injected: … (3)
  no KaTeX output — math did not render offline
```

and against the fixed tree:

```
OK: no third-party requests, no CSP violations, math rendered offline.
```

A check that cannot fail proves nothing, which is why both directions are
recorded here.

## What is not measured

The Wails webview, for the same reason as E6: WebKitGTK does not speak the
DevTools Protocol these harnesses use. `make gui` builds, and the webview loads
the identical bundle through the service handler — but the CSP is delivered by
`handleWebApp`, and the Wails asset server routes webview requests through that
same handler, so the header applies there too by construction rather than by
measurement.

Committed JSON: `editor-before-e6a-{1,2,3}.json`, `editor-after-e6a-{1,2,3}.json`.
