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
