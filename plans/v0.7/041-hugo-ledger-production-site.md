# v0.7 G18g — pinned Hugo/Ledger production documentation site

Date: 2026-08-30
Models: GPT-5 (exact serving variant unavailable); bounded Luna workers for site materialization, Help regression coverage, and evidence scaffolding
Working state: complete

## Goal and boundaries

Replace the bespoke per-page Marked build with the G18b-selected, repository-
owned Hugo/Ledger integration while keeping `docs/**/*.md` as the sole raw
source for both the public manual and protected offline Help notebook. This
slice does not add Bluge, change application search, alter note publication,
publish design documents, vendor `node_modules` or binaries, use remote runtime
assets, push Git, or burn physical media.

## Production integration

`docs-site/` now owns the Hugo configuration, minimal Notrios layout adapters,
local styling, search page, exact 44-file/173,947-byte Ledger snapshot, upstream
Apache-2.0 LICENSE, and provenance. The snapshot is byte-identical to G18b's
reviewed source at commit `f9d28ea297427890ecffa31fa74caa9ee385d9f5`; its
sorted SHA-256 manifest is
`2897566a3861c1a92f6f83d2606d8605c085d4b63d3f67adbef4954c55880025`.

The build pins Hugo Extended 0.164.0, Node 26.3.0, and Pagefind 1.5.2. It stages
exactly the 15 canonical Markdown files in a temporary directory, renders
file-style routes under `/notrios/`, and generates the local Pagefind bundle.
The Pages workflow, `make docs`, CI source checks, and release packager all use
the same builder. A content-security-policy meta tag confines runtime resources
to the site/data URLs and forbids objects, remote connections, and foreign form
targets.

## Raw Help and publication adapter

Hugo's safe Goldmark renderer interpreted the generated CLI registry's angle-
bracket positional placeholders as raw HTML and dropped them. The production
page adapter escapes angle placeholders only in the exact generated CLI block
before rendering. It does not edit `docs/`, loosen raw-HTML handling, or affect
ordinary Markdown. A static built-site gate now requires the visible
`<raw-export-dir>` placeholder plus semantic code and table output.

`internal/helpdocs/g18g_docs_test.go` independently seeds all 15 raw files into
an in-memory store and proves deterministic IDs, Help notebook assignment,
derived titles, byte-for-byte bodies, and idempotent reseeding with 15 kept and
zero creates, updates, or removals.

## Compatibility, search, and browser evidence

The strict evidence bundle under `performance/v0.7-g18g/` requires all 15
legacy `.html` routes, all 199 G18a section IDs, both compatibility aliases,
valid internal targets/fragments, exactly 15 Pagefind-scoped articles, excluded
API/search furniture, local assets, and exact production/frozen-theme equality.
Five mutation classes reject theme-pin, route/fragment, scope, Help-contract,
and runtime-offline drift.

The Browser plugin was unavailable, so the repository's documented Playwright
fallback drove Chrome at 1440×960 and 390×844. All 15 routes, the `Argon2id`
search result under `/notrios/`, keyboard focus, mobile drawer/Escape behavior,
44×44 target, zero overflow, and light/dark/high-contrast samples
(15.00/15.07/21.00) passed. Visual inspection found the desktop search and
mobile manual states coherent. There were zero console/page/HTTP errors, CSP
violations, third-party requests, or framework overlays. These checks are an
executable rendered sweep, not a complete WCAG audit.

## Reproducibility and measurements

Two clean builds produced byte-identical Hugo HTML/CSS/JavaScript and the same
15-page Pagefind scope/search behavior. As G18b already measured, Pagefind
1.5.2 varies hashed metadata/index shard names; the frozen contract is pinned
source/tools plus executable semantic checks, not a byte-identical Pagefind
tree.

One local production run took 2.71 seconds wall at 82,432 KiB maximum RSS and
produced 53 files/1,419,897 bytes: 530,458 HTML bytes and 841,178 Pagefind
bytes, indexing 15 pages/3,438 words. The G18b current-builder baseline was
39.72 seconds and 1,109,026 output bytes. These are observations, not
acceptance limits.

## Validation

Focused completion checks passed: G18g source/site validator, four Python
evidence tests, five mutation classes, production-theme equality, the raw Help
seed/reseed test, two clean builds, G18b compatibility validator, shell/Node
syntax, and rendered desktop/mobile browser QA. The final repository gate and
verified source ZIP are recorded in the completion handoff.

No canonical note/database data, private corpus, remote font/CDN, Bluge/Recoll
linkage, GitHub push, reserve write, ISO, or physical burn was used.
