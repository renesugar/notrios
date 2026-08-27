# G18b Hugo/Ledger reproducible-site investigation

G18b proves that the 15-page public manual can move to Hugo and the pinned
Ledger theme without changing the canonical Markdown, Help-note bytes, existing
`.html` routes, expected fragments, static Pagefind search, or `/notrios/`
deployment boundary. This is an investigation prototype only: production still
uses `scripts/build_docs_site.sh` until a separately approved migration item.

## Recommendation

Vendor the minimal Ledger runtime source, its Apache-2.0 LICENSE, and exact
upstream commit in the repository. A Git submodule disappears from ordinary
`git archive` source handoffs. A network Hugo Module can use the exact
pseudo-version, and a generated `_vendor` can build offline, but clean initial
resolution needs network/Go/Git/cache state and Hugo's vendor output omitted the
theme LICENSE. Direct vendoring gives the simplest auditable and self-contained
release ZIP.

Keep `uglyURLs = true`, the five explicit Goldmark heading-ID compatibility
mappings, and the two legacy fragment aliases. Keep Pagefind as the only active
search backend and scope it with `data-pagefind-body`; the generated search and
API-section furniture stay out of the index. The custom layouts are an adapter,
not edits to `docs/` or to seeded Help notes.

The current Marked command emits no heading IDs, so 32 source fragment links are
already broken in the current site. The prototype fixes those links and contains
all 199 G18a section IDs. That correction is recorded as a current-pipeline
defect, not counted as a Ledger regression.

## Reproduce

Hugo Extended 0.164.0 and Pagefind 1.5.2 are external tools. With both on PATH:

```bash
python3 performance/v0.7-g18b/validate_evidence.py
python3 -m unittest discover -s performance/v0.7-g18b -p 'test_*.py'
python3 performance/v0.7-g18b/build_prototype.py --output /tmp/notrios-g18b-site
python3 performance/v0.7-g18b/validate_evidence.py --site /tmp/notrios-g18b-site
```

For the executable browser smoke, serve a directory whose `notrios/` child is
the built output, then provide Playwright's module path if it is not installed
in this repository:

```bash
G18B_BASE_URL=http://127.0.0.1:18618/notrios/ \
PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs \
node performance/v0.7-g18b/browser_smoke.mjs
```

`SITE_CONTRACT.json` contains the routes, anchors, search assertions, request
observations, browser results, and size/timing measurements.
`DELIVERY_OPTIONS.json`, `REQUEST_INVENTORY.json`, `LICENSE_BILL.json`, and
`THEME_PROVENANCE.json` contain the option probes and source-closure record.

Two clean builds produced identical Hugo output and the same Pagefind scope,
counts, and known-query result. Pagefind 1.5.2 varied its hashed index/metadata
shard names, so this contract deliberately promises repeatable inputs and
observable behavior—not a byte-identical generated Pagefind directory.
