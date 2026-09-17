# v1.0 J35: the browser smoke runs again, and drift is caught

J31 found that `performance/v0.7-g18g/browser_smoke.mjs` did not finish: it
timed out clicking the theme menu's `light` option, which Playwright reported
as not visible. It failed the same way on the theme from before J31's refresh,
so it was not the theme.

## What was actually wrong (J35-A)

**The theme control works.** A probe of the built site found one
`[data-ledger-theme-button]`, visible, `aria-expanded="false"`, with the three
`[data-theme-value]` options present but hidden inside `ledger-theme-menu`.
Clicking the button made all three visible. There was no regression to fix.

**The smoke was being run at an address the build does not serve.** Its default
base was `http://127.0.0.1:18618/notrios/`, a GitHub Pages project path. The
site is built for the apex (`baseURL = "https://notrios.com/"`), so its pages
ask for root-absolute `/js/…`. Served under `/notrios/`, every asset 404s, the
theme's JavaScript never loads, and the button that opens the menu does
nothing. The control was not broken; it was inert.

Run at the root, the same unmodified script got past the theme and failed on
its last hard-coded subpath: `if (!href?.startsWith('/notrios/'))`, which
refused a correct result at `/publishing.html`.

**Owner context (2026-09-17).** The subpath is a leftover from GitHub Pages;
the site is served at `notrios.com` once changes reach `main`. Comments lag
because a local build cannot test that deployment.
`https://renesugar.github.io/notrios/` now answers **301** to
`https://notrios.com/`, so nothing is served at the subpath at all.

## The corrections (J35-B)

| file | was | now |
|---|---|---|
| `performance/v0.7-g18g/browser_smoke.mjs` | default base `…:18618/notrios/`; result prefix hard-coded `/notrios/` | default base `…:18618/`; the expected prefix is `new URL(base).pathname`, so either build shape can be checked |
| `performance/v0.7-g18g/README.md` | told the reader to set a subpath base | says the default is the root, why the subpath failed, and that a project-path build would pass its own base |
| `DOCS_SITE.md` (twice) | "preserving `/notrios/` URLs", "uses the GitHub Pages project base path" | the site's URLs; the `baseURL` in `docs-site/hugo.toml`, apex since v0.9 I10 |
| `FEATURE_MATRIX.md` | "preserve `/notrios/` links" | "preserve the site's links" |
| `CONTEXT_MAP.md` | "`/notrios/` route adapters" | "route adapters for the configured `baseURL`" |
| `TESTING_POLICY.md` | "Pagefind results under `/notrios/`" | "under the base it is run at" |

`ROADMAP.md` needed nothing: it already records the I10 move, that a build is
correct only for the `baseURL` it was built with, the preview trick for each
shape, and the stale-`_site` trap.

**The smoke now runs with no environment variable but `PLAYWRIGHT_MODULE`**
(`SMOKE.json`): 15 routes, theme contrast 15.0 / 15.07 / 21 for light, dark and
contrast — so the menu opened three times — the `Argon2id` search, the mobile
drawer with a 44-pixel target and no overflow, **0 external requests, 0 CSP
violations, 0 problems**.

## The check that catches the next one (J35-C)

`scripts/check_site_base_url.py` compares what tracked files say against the
address the build is configured for.

- The served address comes from `docs-site/hugo.toml`'s `baseURL`, the one file
  that decides it; the repository name from the `origin` remote; the custom
  domain from `docs-site/static/CNAME`.
- For an apex build it flags address-shaped mentions of the Pages address, of
  any `http(s)` URL whose path starts with the repository name, and of a quoted
  bare `/<repo>/`. For a project-path build it flags the custom domain
  presented as what is served. **The mirror case is tested**, so the check is
  not written only for today's shape.
- A repository name inside a filesystem path — `~/.config/notrios/`,
  `cmd/notrios/`, an evidence ZIP path — is not an address and is not flagged.
  The first draft did flag them, which is what the narrowing is for.
- Historical mentions are listed with a reason: archived plans, v0.7 and v0.9
  evidence, the roadmap's record of the move, and this check's own text. A live
  tool inside an evidence directory is **not** covered by that: the G18g smoke
  was corrected rather than listed.
- It runs in `scripts/validate-scaffold.sh`, so drift fails in `make validate`
  rather than at packaging time. It reads configuration and never reaches the
  network: it can say the repository agrees with itself, not what the live site
  serves.

`scripts/test_check_site_base_url.py`, six tests: a document naming the subpath
as current fails; the same document listed as history passes; all three
address shapes are found; filesystem paths are not addresses; a project-path
build is checked the other way; and the check passes on this repository.

**On the code before this change** the check reported **27 contradicting
mentions**. Six were live claims and are corrected above; the rest were records
of their time and are listed.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry. `validate-scaffold.sh` — which now runs the new
check — and `make g18g-validate` pass.
