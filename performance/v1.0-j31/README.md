# v1.0 J31: the vendored Ledger theme takes its Bluge fix

`docs-site/themes/hugo-theme-ledger` was pinned to upstream `f9d28ea`, and
G18g's validator required it to be **byte-identical to the frozen G18b
prototype snapshot**. That made the vendored theme unable to take an upstream
fix without rewriting v0.7's record of what it had reviewed. This item split
the check and refreshed the copy.

## The pin split (J31-A)

`performance/v0.7-g18g/validate_evidence.py` had one function checking both
copies against the same fixed numbers. It now has two:

| | checks |
|---|---|
| `validate_frozen_theme` | the G18b snapshot against the constants v0.7 recorded: 44 files, 173,947 bytes, manifest `2897566a…`. Unchanged. |
| `validate_production_theme` | the vendored copy against `docs-site/THEME_PROVENANCE.json`: a full 40-character upstream commit, and the file count, byte count and manifest hash **of the bytes on disk**. The file *list* must still equal the reviewed runtime set. |

G18g's own evidence bundle still records `f9d28ea`, because that is what G18b
selected and G18g materialized. No v0.7 evidence file changed.

`performance/v0.7-g18g/test_validate_evidence.py` went from 5 tests to 9,
proving the split in both directions:
- a production file that its provenance does not describe fails;
- a production file **updated together with its provenance passes** — the
  point of the item;
- an added or removed vendored file fails, even with matching provenance;
- a provenance without a full commit id fails (missing, empty, `main`, short,
  non-hex);
- a mutated frozen snapshot still fails.

## The refresh (J31-B)

`performance/v1.0-j31/refresh_vendored_theme.sh` clones upstream fresh — never
a local working copy, which may be mid-change — checks out the requested ref,
and copies only the files the vendored snapshot already has.

Run at `origin/main` on 2026-09-17:

```
upstream https://github.com/renesugar/hugo-theme-ledger at origin/main = cf68886beb31ec0bbe7e2f05e538a20fec722e5a
commits between the vendored pin and this one:
  cf68886 Merge pull request #5 from renesugar/develop
  306438f Merge pull request #4 from renesugar/main
  fc6adf8 Merge pull request #3 from renesugar/develop
  85ac1b0 fix: resolve Bluge results under the site subpath
  2b7bbc0 Merge pull request #2 from renesugar/develop
  41b6f41 Merge pull request #1 from renesugar/develop
NOT vendored (upstream has it): assets/js/search/backends/bluge.test.js
NOT vendored (upstream has it): assets/js/search/query.test.js
docs-site/THEME_PROVENANCE.json: 44 files, 174,456 bytes, 64607692c75a…
copied 44 file(s), 0 missing upstream
```

- **One file changed:** `assets/js/search/backends/bluge.js`. 44 files still,
  173,947 → 174,456 bytes.
- **The two upstream test files were reported, not vendored.** The snapshot has
  never carried tests, and the script does not decide that silently.
- `write_provenance.py` rewrote the provenance from the bytes on disk, and it
  now also records the previous pin, the commits between the two, the changed
  file, the upstream files deliberately not vendored, and that the frozen
  review snapshot is unchanged.
- `DOCS_SITE.md` and `docs-site/README.md` name the shipping commit and say the
  G18b figures are v0.7's record. `CODING_CLIENT_HANDOFF.md`'s G18b account and
  `agent/PLAN_STATUS.md` are history and were left alone.

## The site still works (J31-C)

- `make g18g-validate` passes: the site builds and both source and site
  evidence pass with the new bytes.
- **The theme's own tests pass at the pinned commit**: 16 of 16, run from the
  clone with `node --test 'assets/js/search/**/*.test.js'`. They are not
  vendored, so they cannot run from this repository.
- **Search works**, proven by `performance/v1.0-j31/search_probe.mjs`
  (`SEARCH.json`): the query `Argon2id` returns 6 results, every link under the
  served base, with no console errors or warnings, no request leaving the
  origin, and no CSP violation.
- Notrios searches with Pagefind, so no reader sees a different result today.
  The bundled `ledger-search` script does change, because every backend is
  bundled, and its hash is what the provenance now records.

## Found and deferred: J35

`performance/v0.7-g18g/browser_smoke.mjs` does not finish in this environment.
It stops clicking the theme menu's `light` option, which Playwright reports as
not visible until it times out. **It fails the same way on the theme before
this refresh**, so it is not this change's doing, and its documented
`/notrios/` base URL cannot serve this build, whose pages use absolute root
paths. Recorded as **J35** rather than worked around here. J31's own search
probe is narrower than the smoke and is what proves search.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry. `validate-scaffold.sh`, G18b's and G18a's
evidence validators and `make g18g-validate` pass.

## The archive

Built from `d8c67d4` with the usage guard on and no override:
- **Usage guard:** it checked only Claude, 27% of the five-hour window
  remaining, above the 20% reserve, identified by process ancestry (J24).
- **Package:** `package_release.sh` passed, including `go test ./...`,
  `validate-scaffold.sh`, `make g18g-validate` and every evidence validator.
- **ZIP:** `notrios-v1.0-j31-d8c67d4.zip`, 25,673,818 bytes, 2,680 entries.
  `check_release_zip.py` accepts it.
