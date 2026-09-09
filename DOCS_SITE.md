# Documentation Site and Help Notebook

Implemented in plan task R15.

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

## Content

User and reference documentation for the `notriosd` service, `notriosctl` CLI, REST/MCP API, query language, and the built-in GUI lives as Markdown under `docs/`. It is authored once and consumed twice:

1. **GitHub Pages site** — built from `docs/` by `scripts/build_docs_site.sh` with pinned Hugo Extended 0.164.0, the exact repository-owned Ledger snapshot, and local Pagefind 1.5.2; `.github/workflows/docs.yml` publishes it on pushes to `main`.
2. **Built-in "Help" notebook** — `notriosctl seed-help [docs-dir]` mirrors the same Markdown into the protected, read-only Help notebook (deterministic note IDs; updates in place; removes notes whose file disappeared), so documentation is available offline inside the app (`notebook:help` searches it; see `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`). REST and MCP mutations of Help notes return 403/errors.

## Site search

The Pages site uses **PageFind** (https://github.com/pagefind/pagefind): a post-build index step over the generated static HTML, producing a fully static search UI with no server component — consistent with the local-first ethos.

## Documentation-integrity and Hugo production pipeline

`PLAN.md` G18a-G18g completed the replacement of the bespoke site builder and
the integrity pipeline shared by the site and Help content:

- investigate Go/TS/TSX source-adjacent `notrios:` doc anchors and report every
  topic as executed, generated, claimed, or unverified;
- execute copyable CLI/config/REST/MCP examples with behavioral postconditions,
  and execute documented GUI steps in desktop/mobile browser journeys while
  reporting their user-action length;
- generate template-ordered user/API fragments and fail deterministic freshness
  drift;
- run blind code explanation followed by supported/contradicted/not-determinable
  review as calibrated advisory evidence—never an embedding-similarity or CI
  truth oracle—and generate actionable commands/config/API/GUI attempts from
  prose for human review;
- migrate the site to a pinned, Apache-2.0 `hugo-theme-ledger` snapshot with
  Hugo Extended and static Pagefind, preserving `/notrios/` URLs/fragments,
  `googleFonts=false`, zero third-party runtime requests, clean release-ZIP
  builds, and the raw Markdown Help source.

G18b selected a repository-owned 44-file/173,947-byte minimal Ledger snapshot
at commit `f9d28ea297427890ecffa31fa74caa9ee385d9f5`, including its LICENSE and a
deterministic manifest. Its prototype preserves all 15 `.html` routes, all 199
G18a section IDs, two legacy aliases, static Pagefind, and zero third-party
runtime requests. It also records Pagefind 1.5.2's semantically stable but
non-byte-identical hashed shard output. The local checkout is not a build
dependency. G18g materialized that exact snapshot under `docs-site/`, wired it
into production/CI/release packaging, and added strict evidence under
`performance/v0.7-g18g/`.

G18c adds `make docaudit`. The repository-only audit resolves the frozen Go and
TypeScript declaration anchors, checks the typed claim/test registry, binds
source fragments to the Markdown template, and accounts for every
executable-shaped fence and proposed GUI journey. Every unit is graded
executed, generated, claimed, or unverified. Manual sections stay independently
unverified until a later slice moves or generates them; an adjacent source
claim never proves stale prose.

**Run `make docaudit` for the current totals.** They are not written down here:
this paragraph carried G18c's first report — 351 units, none executed — long
after later slices had made seventy-one of them executed. G18c's own frozen
report stays under `performance/v0.7-g18c/`, where a number that describes one
moment belongs.

## Structure

```text
docs/
  index.md              # overview + quick start
  installation.md       # prerequisites, building, output paths, local install
  service.md            # configuration reference, security, sidecar, backup/restore
  cli.md                # notriosctl subcommand reference
  api/rest.md           # REST usage guide with curl examples (OpenAPI remains api/openapi.yaml)
  api/mcp.md            # MCP tools, profiles, client configuration
  query-language.md     # user-facing form of SEARCH_QUERY_LANGUAGE.md
  selection-planning.md # archive/subset/publication dry-run privacy boundary
  archive-v2.md         # native archive v2 identity/admission safety contract
  stable-links.md       # notrios:// links, profile registry, desktop handler
  publishing.md         # publication profiles, review-then-publish, link handling
  gui.md                # built-in GUI: building, modes, layout, themes, errors
  import-export.md      # per-source import workflows, archive export/import
  operations.md         # media, resources/GC, importer resume, sidecar status
  troubleshooting.md    # verified symptoms and fixes
```

The site navigation in `docs-site/layouts/` lists every user page. The build
uses the GitHub Pages project base path (`/notrios/`) and indexes only elements
marked `data-pagefind-body`, excluding generated API/search furniture.

## Rules

- Docs in `docs/` are user-facing; design documents in the repo root remain contributor-facing and are not published to the site.
- The Help-notebook seeding step must be repeatable (re-import replaces the Help notes deterministically).
- The site build and PageFind indexing run in CI; the workflow must not require secrets beyond the default Pages deployment token.
