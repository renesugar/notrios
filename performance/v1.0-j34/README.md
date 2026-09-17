# v1.0 J34: stop the preview loading remote images through media elements

J29 found, while designing its scanner, that the GUI preview's sanitizer
neutralised remote `img` sources and nothing else that loads media. This item
closes that gap. By owner direction (2026-09-17), inline `data:image/…` images
and inline SVG keep rendering.

## Before: measured on the real UI

`run_preview_probe.sh` does the following:
1. Builds the web UI and starts `notriosd` on an in-memory library with
   isolated roots.
2. Serves a stand-in "remote" origin on another port that logs every request.
3. Creates a note with every J29-covered form (Markdown and HTML `img` with
   `srcset`, `picture`/`source`, `video` with `src`, `poster`, `source` and
   `track`, `audio`, `embed`, `object`, SVG `image`) plus two inline base64
   PNGs.
4. Opens the note in headless **Chromium and WebKit** through Playwright
   (`preview_probe.mjs`), and records requests to the remote origin, CSP
   violations, the rendered elements, and whether the inline images loaded.

`BEFORE.json`, on the code before this change:

| | Chromium | WebKit |
|---|---|---|
| requests the stand-in server received | `/video-poster.png`, `/svg-image.png` | the same |
| CSP violations | `media-src` for `video` and `audio` `src` | the same |
| inline images rendered | 2 of 2 | 2 of 2 |

It settled the two questions J34 left open:
- **The renderer passes raw HTML through** to the sanitizer, and it renders.
- **WebKit enforces the CSP header.** Remote audio and video were refused only
  through the `default-src 'self'` fallback. `poster` and SVG `image` are image
  loads, which `img-src` admits, and they were fetched.

The desktop app's own WebKitGTK webview was not driven. WebKit is the closest
engine available.

## What changed

- **`web/src/preview-utils.tsx`.** A single `neutralizeMediaURL` holds the rule
  `img` used, and is applied to:
  - `img` `src`;
  - `video` and `audio` `src` and `poster`;
  - `track` `src`;
  - `href` and `xlink:href` on every SVG element except `a`. That is J29's
    `image`, and also `feImage` and `use`, which load the same way.

  The rule:
  - `resource://` is resolved to the local content URL;
  - `data:image/…` is kept;
  - `http:` and `https:` move to `data-remote-<attribute>` and are removed;
  - anything else is removed.

  `img` behaves exactly as before. `srcset`, `source`, `embed` and `object` are
  still removed.
- **The CSP** (`internal/httpapi/server.go`) states `media-src 'self'`
  explicitly rather than relying on the fallback. No directive was relaxed.
- **Documentation.**
  - `SECURITY_AND_MEDIA_POLICY.md`'s preview rule names every covered
    attribute, the rule, and the explicit `media-src`.
  - `SECURITY_REVIEW.md` no longer says only that `img-src` admits remote
    images "because the preview is permitted to display them".
  - `UI_DESIGN.md` gains the preview rule.

## Proof

**Unit tests** (`web/src/__tests__/preview-media.test.ts`, six tests):
- no remote URL is left on any media element or attribute;
- each remote source is kept as inert metadata;
- inline images and inline SVG keep rendering;
- `resource://` media resolves;
- other schemes (`javascript:`, `file:`, `ftp:`) are removed;
- SVG links keep the link rule.

**The first, second, fourth and fifth fail on the code before this change.**
The third and sixth pass before and after, as the guards that inline content
and links did not regress. The whole web suite passes (31 files, 300 tests),
and `npm audit` found 0 vulnerabilities, so there was nothing to fix.

**The CSP test** (`TestWebAppServesAContentSecurityPolicy`) now requires
`media-src 'self'` and refuses a remote `media-src`.

**The rendered note after the change**, from `AFTER.json` with the UI rebuilt:

| | Chromium | WebKit |
|---|---|---|
| requests the stand-in server received | **none** | **none** |
| CSP violations | **none**: nothing was attempted | **none** |
| inline images rendered | 2 of 2 | 2 of 2 |
| `video` | `data-remote-src`, `data-remote-poster` | the same |
| SVG `image` | `data-remote-href` | the same |

**Why the probe is not in `go test` or `npm test`.** Like G18g's browser smoke,
it needs a Playwright module and browsers this repository does not depend on.
It is run as:

```sh
PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs CHROME_PATH=/usr/bin/google-chrome \
  bash performance/v1.0-j34/run_preview_probe.sh
```

The unit tests are the regression guard that runs everywhere.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry. `validate-scaffold.sh`, `make g18g-validate`
and `scripts/run_offline_assets_check.sh` ("no third-party requests, no CSP
violations") pass.

Scaffold validation first refused `run_preview_probe.sh` for running Python
without `PYTHONDONTWRITEBYTECODE`. The export was added, and the probe re-run
afterwards produced a report byte-identical to `AFTER.json`.
