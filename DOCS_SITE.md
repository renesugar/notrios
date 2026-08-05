# Documentation Site and Help Notebook

Implemented in plan task R15.

## Content

User and reference documentation for the `notriosd` service, `notriosctl` CLI, REST/MCP API, query language, and the built-in GUI lives as Markdown under `docs/`. It is authored once and consumed twice:

1. **GitHub Pages site** — built from `docs/` by `scripts/build_docs_site.sh` (marked → HTML with a shared template, then a PageFind index) and published by `.github/workflows/docs.yml` on pushes to `main`.
2. **Built-in "Help" notebook** — `notriosctl seed-help [docs-dir]` mirrors the same Markdown into the protected, read-only Help notebook (deterministic note IDs; updates in place; removes notes whose file disappeared), so documentation is available offline inside the app (`notebook:help` searches it; see `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`). REST and MCP mutations of Help notes return 403/errors.

## Site search

The Pages site uses **PageFind** (https://github.com/pagefind/pagefind): a post-build index step over the generated static HTML, producing a fully static search UI with no server component — consistent with the local-first ethos.

## Structure (initial)

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
  gui.md                # built-in GUI: building, modes, layout, themes, errors
  import-export.md      # per-source import workflows, archive export/import
  operations.md         # media, resources/GC, importer resume, sidecar status
  troubleshooting.md    # verified symptoms and fixes
```

The site nav in `scripts/build_docs_site.sh` must list every page (it currently does); the same script prefixes the GitHub Pages project base path (`/notrios/`).

## Rules

- Docs in `docs/` are user-facing; design documents in the repo root remain contributor-facing and are not published to the site.
- The Help-notebook seeding step must be repeatable (re-import replaces the Help notes deterministically).
- The site build and PageFind indexing run in CI; the workflow must not require secrets beyond the default Pages deployment token.
