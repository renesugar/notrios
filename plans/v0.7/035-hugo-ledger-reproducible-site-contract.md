# v0.7 G18b — Hugo/Ledger migration and reproducible site contract

Date: 2026-08-27
Model: GPT-5 (exact serving variant unavailable)
Working state: complete investigation; production site unchanged

## Goal and boundary

Prove one reproducible way to apply the local Apache-2.0
`hugo-theme-ledger` reference to Notrios' public documentation without changing
the canonical Markdown/Help bytes, existing `/notrios/` URLs and fragments,
static search, offline runtime, or release-ZIP closure. This slice does not
switch `scripts/build_docs_site.sh` or `.github/workflows/docs.yml`, mutate
`docs/` or Help notes, add a product dependency, enable a Bluge service, alter
the separate note-publication pipeline, or implement G18c-G18g.

## Selected integration

G18g should vendor the minimal runtime source from exact Ledger commit
`f9d28ea297427890ecffa31fa74caa9ee385d9f5`: `assets/`, `layouts/`,
`theme.toml`, and the upstream Apache-2.0 LICENSE. The G18b prototype contains
44 files/173,947 bytes and commits a deterministic aggregate SHA-256 manifest.
The only tracked upstream files omitted from those runtime directories are two
`*.test.js` fixtures; their implementation modules remain exact upstream bytes.
It requires only documented external build tools, not the sibling checkout,
Git metadata, a network, `node_modules`, or a search server.

The URL contract uses Hugo `uglyURLs = true` to preserve the 15 existing
`.html` routes. Five explicit render-hook mappings preserve Goldmark headings
whose slugs differ from G18a's frozen IDs. Two zero-content aliases preserve
stale source links while retaining the canonical headings:

- `service.html#configuration` aliases `#configuration-reference`;
- `stable-links.html#linking-to-a-block-not-just-a-note` aliases
  `#linking-to-a-section-or-a-block`.

Static Pagefind remains the only active backend. Exactly the 15 documentation
articles carry `data-pagefind-body`; the generated search and API section pages
are furniture and remain unindexed. `googleFonts=false` works with Hugo 0.164;
the prototype emitted no font request. A local favicon data URL avoids an
implicit browser request, and a 44x44 minimum fixes the only measured mobile
target-size adapter issue.

## Delivery-option evidence

### Minimal vendored source — selected

The payload includes exact upstream runtime bytes, LICENSE, and provenance.
Ordinary release ZIPs contain it, clean/offline source resolution needs no
network, and upgrades become explicit reviewable diffs. It is the smallest
operationally complete option despite requiring deliberate update work.

### Git submodule — rejected

A temporary repository pinned a gitlink to the exact commit. Its ordinary
`git archive` was 10,240 bytes and contained only `.gitmodules` and empty theme
directory entries—no theme files. Recursive clone/update and a separate release
materialization step would therefore be mandatory.

### Network Hugo Module — rejected

The exact pseudo-version
`v0.0.0-20260731032527-f9d28ea29742` resolved with module sum
`h1:2nWTjFIOrm79Rot74+7dARHq9VtX7i5W8mJWq9GxV3s=`. Initial resolution required
network plus Go/Git/cache state. After `hugo mod vendor`, a build with
`GOPROXY=off` and an empty Hugo cache passed, but the 47-file/172,528-byte
vendor tree included `modules.txt` and omitted the upstream LICENSE. Copying
the license and committing `_vendor` would collapse into a less intentional
form of direct vendoring. Hugo documents that Modules can be vendored, while
`uglyURLs` is the setting that emits file-style HTML URLs:
<https://gohugo.io/hugo-modules/use-modules/> and
<https://gohugo.io/configuration/ugly-urls/>.

## Route, Help, and search findings

The builder verifies every `docs/**/*.md` SHA-256 against G18a, stages exactly
15 files, and byte-copies each file. That is the same raw source consumed by
`internal/helpdocs.Seed`; the prototype adds only its own generated search
page. All 15 current routes, all 199 frozen G18a section IDs, every internal
target, and both aliases validate. No `.md` link or `/notrios/` escape remains.

The present Marked command emits no heading IDs. Its generated output therefore
has 32 broken internal fragment links. Hugo/Goldmark plus the compatibility
hook repairs this measured current-pipeline defect; it is not a Ledger
regression.

Pagefind 1.5.2 reports 15 indexed pages and 3,191 words. The browser query
`Argon2id` returns results whose URLs stay under `/notrios/`. Pagefind's
documented `data-pagefind-body` behavior excludes pages without the marker:
<https://pagefind.app/docs/indexing/>.

## Reproducibility qualification and measurements

On the measured host, Hugo Extended 0.164.0 plus Pagefind 1.5.2 built the
repository-owned prototype in 1.44 seconds wall at 82,560 KiB maximum RSS.
One observed output had 52 files/1,372,946 bytes: 496,194 HTML bytes, 828,491
Pagefind bytes, 22,463 non-Pagefind CSS bytes, and 25,798 non-Pagefind JavaScript
bytes. These are local observations, not acceptance limits.

Two clean builds produced byte-identical Hugo HTML/CSS/JavaScript. Pagefind
produced the same 15-page/3,191-word scope and passed the same query, but varied
hashed metadata/index shard names and total size by a few bytes. The frozen
contract is therefore reproducible source/tool pinning plus executable semantic
checks. It intentionally does not promise a byte-identical generated Pagefind
directory.

The current builder's observed 39.72-second run at 108,164 KiB RSS included 15
separate `npx --yes marked` resolutions plus Pagefind; a restricted-network
attempt failed at the npm registry before an authorized retry. This is useful
dependency/request evidence, not a product performance comparison.

## Validation evidence

- G18b static evidence validator and four unit tests pass.
- Clean repository-owned Hugo/Pagefind build passes the built-site validator:
  15 routes, 199 expected section IDs, two aliases, no broken/Markdown/outside-
  base links, 15 scoped pages, excluded generated furniture, no remote font,
  and a local Pagefind bundle.
- Executable Playwright/Chrome smoke passes 15 route responses, 15 sidebar
  links, `Argon2id`, light/dark/high-contrast prose samples (15.00/15.07/21.00),
  theme keyboard controls, mobile drawer/Escape, a 44x44 menu target, zero
  horizontal overflow, zero third-party requests, and zero console problems.
  Contrast figures are samples, not a complete WCAG audit.
- Two clean prototype builds establish the reproducibility qualification above.
- Full repository validation, the G17b host-side evidence gate, release ZIP
  verification, and a build from the extracted source ZIP passed at completion.

Evidence lives under `performance/v0.7-g18b/`. No generated site is committed.
No production workflow/site, canonical docs/Help content, schema, dependency,
remote, external reserve, or physical medium changed; no push occurred.
