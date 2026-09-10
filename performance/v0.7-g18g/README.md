# G18g production docs-site acceptance evidence

G18g is the bounded production migration and QA gate following the G18b Ledger
investigation. It does not change `docs/` or Help notes. The implemented
production builder consumes `docs-site/` using
`scripts/build_docs_site.sh <output>`. The validator is deliberately strict:
it freezes the exact 44-file G18b selected theme snapshot, upstream commit,
Apache-2.0 LICENSE, route/fragment compatibility, Pagefind scope, local
runtime assets, and raw-document/Help equivalence contract.

Run the source and mutation evidence checks:

```bash
python3 performance/v0.7-g18g/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18g -p 'test_*.py'
```

After a production build, pass its output as the first positional site check:

```bash
bash scripts/build_docs_site.sh /tmp/notrios-g18g-site
python3 performance/v0.7-g18g/validate_evidence.py --site /tmp/notrios-g18g-site
```

The Browser plugin was absent for this evidence slice. `browser_smoke.mjs` is
the documented Playwright fallback. Set `G18G_BASE_URL`, `PLAYWRIGHT_MODULE`,
and optionally `G18G_SCREENSHOT_DIR` to a caller-owned `/tmp` directory;
screenshots are never written to the repository. The script covers desktop
1440x960 and mobile 390x844 routes, landmarks, labels, keyboard focus, theme,
Pagefind `Argon2id`, drawer/Escape/44px/no-overflow, console/page/CSP errors,
and external requests.

`REPORT.json` records the production measurements and the frozen G18b baseline.
Two clean builds produced byte-identical Hugo output and identical Pagefind
scope/counts. As G18b established, Pagefind 1.5.2 varies hashed metadata/index
shard names; the reproducibility contract is pinned inputs plus executable
semantic checks, not a byte-identical Pagefind directory. `ROUTES.json`,
`BUILD_CONTRACT.json`, and `MUTATION_MATRIX.json` are the reviewable contracts.

## Known limitation: relative directory links

`local_target` resolves a **relative** directory link to a directory rather
than to its `index.html`, and reports it as a broken link that is not broken:

| link | from | resolves to | |
| --- | --- | --- | --- |
| `/search/` | `index.html` | `search/index.html` | correct |
| `./search/` | `index.html` | `search` | a directory |
| `../search/` | `api/mcp.html` | `api/../search` | a directory, and unnormalised |

`(Path(route).parent / path).as_posix()` normalises the trailing slash away
before the `endswith("/")` test that would have appended `index.html`. The third
row shows a second half to the same problem: `..` is not resolved either. A fix
needs both — remember the trailing slash before normalising, *and* resolve the
path lexically, or the route strings will not match the keys the parsed-page map
is indexed by, even where the filesystem would resolve them happily. Absolute
links are unaffected: they skip that branch and keep their slash.

Nothing emits relative links today, so it is latent. It surfaced in v0.9 I10,
when `relativeURLs = true` was tried so one build could serve both
`https://notrios.com/` and the `https://renesugar.github.io/notrios/` fallback:
the validator reported 54 broken links against a site whose pages all existed.
The site was fine; the check was not.

`test_validate_evidence.py` carries the test that would prove it fixed, skipped
rather than deleted — asserting today's behaviour would record a defect as
intended and fail the moment somebody repaired it. Whoever fixes it removes one
decorator.

**This matters before enabling relative URLs**, which remains an open option for
dual-address compatibility. Doing that without fixing this would either bury the
change under phantom failures or invite someone to weaken the link check to make
them go away.
