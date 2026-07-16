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
  index.md              # overview + install
  service.md            # notriosd configuration, flags (-no-gui, -gui-only on the GUI binary)
  cli.md                # notriosctl commands, importers
  api/rest.md           # REST usage guide (OpenAPI remains api/openapi.yaml)
  api/mcp.md            # MCP tools guide
  query-language.md     # user-facing form of SEARCH_QUERY_LANGUAGE.md
  gui.md                # built-in GUI: layout, notebooks, tags, themes
  import-export.md      # importers, export, dry-run import configuration
```

## Rules

- Docs in `docs/` are user-facing; design documents in the repo root remain contributor-facing and are not published to the site.
- The Help-notebook seeding step must be repeatable (re-import replaces the Help notes deterministically).
- The site build and PageFind indexing run in CI; the workflow must not require secrets beyond the default Pages deployment token.
