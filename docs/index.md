# Notrios

Notrios is a local-first note-taking, search, import, and publishing system for very large Markdown and document collections. Your notes live in a SQLite database on your machine; search, notebooks, tags, revisions, and attachments all work offline.

## Highlights

- **Local first** — one SQLite file is the canonical store; full-text search (FTS5) is built in, and the optional [Recoll sidecar](service.md#search-sidecar) adds field search over front matter and attached files.
- **Notebooks and tags** — nested notebooks with emoji icons, tags with live counts, and query-backed *search notebooks* ("All notes", "Trash", and your own saved searches like `tag:todo`).
- **A real query language** — `notebook:"Work"`, `tag:"shopping mall"`, `author:"Alice Smith"`, `since:2026-07-01`, quoted phrases, and more. See the [query language](query-language.md).
- **Importers** — Joplin RAW, Obsidian vaults, Twitter/X archives (with conversation threads), ChatGPT exports, and Claude exports. See [import & export](import-export.md).
- **Open APIs** — a REST API and an MCP endpoint complete enough to build a full third-party client. See the [REST](api/rest.md) and [MCP](api/mcp.md) guides.
- **Built-in GUI** — a Go/Wails desktop app with a notebooks sidebar, incremental search, Markdown editor/preview, and light/dark/custom themes. See the [GUI guide](gui.md).
- **Safe maintenance** — explicit remote-media localization, read-only
  resource reports, retention-aware garbage collection, resumable imports,
  and observable Recoll repair. See [data safety and maintenance](operations.md).
- **Privacy-reviewed handoffs** — dry-run full archive, subset, or publication
  boundaries before any files are written. See [selection and privacy
  planning](selection-planning.md).
- **Verified archive identity** — native archive v2 defines manifest-last,
  SHA-256 object admission and explicit replace/merge/fork/adopt semantics.
  See the [archive v2 safety contract](archive-v2.md).

## Getting started

Notrios is built from source on Ubuntu Linux (the only tested platform); there are no prebuilt binaries yet. The [installation guide](installation.md) covers prerequisites, every build target, output paths, and optional local installation. The short version:

```bash
git clone https://github.com/renesugar/notrios.git
cd notrios
make build web
./bin/notriosd -config config/config.example.yaml   # http://127.0.0.1:8080

make gui && ./bin/notrios                           # desktop GUI
```

The [service guide](service.md) covers configuration; the [CLI guide](cli.md) covers imports and exports; [troubleshooting](troubleshooting.md) covers common build and startup errors.

## License

Notrios is licensed under the Apache License 2.0.
