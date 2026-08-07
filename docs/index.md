# Notrios

Notrios is a local-first note-taking, search, import, and publishing system for very large Markdown and document collections. Your notes live in a SQLite database on your machine; search, notebooks, tags, revisions, and attachments all work offline.

## Highlights

- **Local first** — one SQLite file is the canonical store; full-text search (FTS5) is built in, and the optional [Recoll sidecar](service.md#search-sidecar) adds field search over front matter and attached files.
- **Notebooks and tags** — nested notebooks with emoji icons, tags with live counts, and query-backed *search notebooks* ("All notes", "Trash", and your own saved searches like `tag:todo`).
- **A real query language** — `notebook:"Work"`, `tag:"shopping mall"`, `author:"Alice Smith"`, `since:2026-07-01`, quoted phrases, and more. See the [query language](query-language.md).
- **Importers** — Joplin RAW, Obsidian vaults, Twitter/X archives (with conversation threads), ChatGPT exports, and Claude exports. See [import & export](import-export.md).
- **Open APIs** — a REST API and an MCP endpoint complete enough to build a full third-party client. See the [REST](api/rest.md) and [MCP](api/mcp.md) guides.
- **Built-in GUI** — a Go/Wails desktop app with a notebooks sidebar, incremental search, Markdown editor/preview, and light/dark/custom themes. See the [GUI guide](gui.md).
- **Addressable sections and blocks** — link to a heading or a paragraph, not
  just a note. Anchors survive a block moving and say so when the text they
  named changes. See [stable links](stable-links.md#linking-to-a-section-or-a-block).
- **Links checked while you type** — inline note search on `[[`, red underlines
  under links that would not open, and Ctrl-click to follow one, all before you
  save. See the [GUI guide](gui.md#link-help-while-you-write).
- **Live query blocks** — a fenced `note-query` block renders as a list of
  matching notes, written in the same query language as the search box. See
  [using a query inside a note](query-language.md#using-a-query-inside-a-note).
- **A graph you can walk** — traverse outward from a note, find a shortest path
  between two, and read which notes are orphans and which are hubs. See
  [seeing the shape of the link graph](operations.md#seeing-the-shape-of-the-link-graph).
- **Safe maintenance** — a read-only report of what has rotted and a dry-run
  repair for the part that can be fixed mechanically, hierarchical tag rename
  with a dry run, explicit remote-media localization, read-only resource
  reports, retention-aware garbage collection, resumable imports, and observable
  Recoll repair. See [data safety and maintenance](operations.md).
- **Trash-first deletion** — deleting a note moves it to the Trash, where you
  can read it and put it back; deleting a notebook tells you exactly what it
  will do first. See [deleting and restoring notes](gui.md#deleting-and-restoring-notes).
- **Privacy-reviewed handoffs** — dry-run full archive, subset, or publication
  boundaries before any files are written. See [selection and privacy
  planning](selection-planning.md).
- **Verified backup and restore** — native archive v2 exports a complete
  snapshot as manifest-last SHA-256 objects, verifies it read-only, and restores
  it under an explicit replace/merge/fork/adopt intent.
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
