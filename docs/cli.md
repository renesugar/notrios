# The notriosctl CLI

`notriosctl` handles imports, exports, diagnostics, and maintenance against the same SQLite database the service uses. Build it with `make build-cli` (output `bin/notriosctl`) or run any command from source with `go run ./cmd/notriosctl <command>`.

General behavior:

- successful commands exit `0`; runtime failures print to stderr and exit `1`; usage mistakes (unknown command/flag, wrong argument count) print usage and exit `2`;
- commands that touch the database accept `--config`, `--db`, and `--asset-store` (overrides win over the config file, which wins over built-in defaults; relative paths resolve against your working directory);
- importers and `export archive` print a JSON report to stdout;
- `notriosctl help` (or any unknown command) prints the full usage summary.

## version

```sh
notriosctl version
```

Prints the version string (currently `0.2.0`) and exits 0. No flags.

## doctor

```sh
notriosctl doctor [--config config.yaml] [--db path] [--asset-store path]
```

Environment and configuration diagnostics. Checks, in order:

| Check | Kind | What it verifies |
|---|---|---|
| `go runtime` | required | which Go the binary was built with |
| `config` | required | the config file loads (shows which file, or "built-in defaults") |
| `directories` | required | storage directories exist or can be created |
| `database` | required | the SQLite database opens, migrates, and reports its schema version |
| `asset store` | required | the attachment directory is writable |
| `web ui` | informational | whether `web/dist/index.html` exists in the working directory (the browser UI needs it; the API does not) |
| `recoll` | informational | whether the optional `recollindex` binary is on `PATH` |

Exit `0` when all required checks pass, `1` otherwise. Note that doctor *creates* missing storage directories and an empty database if none exists (the same bootstrap the service performs) — point it at your real config to diagnose your real setup.

```text
ok    config           config/config.example.yaml
ok    database         ./data/notes.sqlite (schema version 6)
info  web ui           web/dist missing here; run `make web` or serve API-only
doctor: required checks passed
```

## import

Six variants; all support `--dry-run` and the shared `--config/--db/--asset-store/--collection` flags. Full workflows, source-application export steps, report-field explanations, and preserved-metadata tables are in [import & export](import-export.md).

### import joplin-raw

```sh
notriosctl import joplin-raw [shared flags] [--dry-run] <raw-export-dir>
```

Positional argument: the Joplin **RAW export directory** (not a `.jex` file). No source-specific flags. Imported notes land in the default "Notes" notebook with Joplin notebook/tag metadata preserved in front matter.

### import obsidian

```sh
notriosctl import obsidian [shared flags] [--dry-run] <vault-dir>
```

Positional argument: the vault directory. No source-specific flags.

### import twitter

```sh
notriosctl import twitter [shared flags] [--notebook Twitter] [--dry-run] <extracted-archive-dir>
```

Positional argument: the **extracted** archive directory (the one containing `data/`). `--notebook` (default `Twitter`) names/creates the destination notebook.

### import chatgpt / import claude

```sh
notriosctl import chatgpt [shared flags] [--notebook ChatGPT] [--dry-run] <conversations.json|export-dir>
notriosctl import claude  [shared flags] [--notebook Claude]  [--dry-run] <conversations.json|export-dir>
```

Positional argument: the export's `conversations.json`, or a directory containing it. `--notebook` defaults to `ChatGPT` / `Claude`.

### import archive

```sh
notriosctl import archive [shared flags] [--dry-run] [--write-config path] [--import-config path] <archive-dir>
```

Imports a native Notrios archive. `--dry-run` analyzes notebook-name conflicts and writes an import configuration file (default `<archive-dir>/import-config.json`; override with `--write-config`) instead of importing. A real import optionally takes `--import-config` with notebook renames; it refuses (exit 1) before writing anything if a name still conflicts with a source-bound notebook.

## export archive

```sh
notriosctl export archive [shared flags] [--query "tag:todo"] <out-dir>
```

Writes a native Notrios archive directory. `--query` (default empty = all non-trashed notes) scopes the export with the [query language](query-language.md). The report lists `notes`, `notebooks`, `resources`, and any `warnings`.

## seed-help

```sh
notriosctl seed-help [--config config.yaml] [--db path] [--asset-store path] [docs-dir]
```

Mirrors a documentation directory (default `docs`) into the built-in read-only **Help** notebook: unchanged notes are kept, changed files update in place, and notes whose source file disappeared are removed. Deterministic note IDs make it repeatable. Report: `files_seen`, `notes_created/updated/kept/removed`.

```json
{"files_seen": 10, "notes_created": 10, "notes_updated": 0, "notes_kept": 0, "notes_removed": 0}
```

## localize

```sh
notriosctl localize [--config config.yaml] [--db path] [--asset-store path] [--dry-run] [--allow-review] [--base-revision rev] <document-id>
```

Downloads the note's policy-**allowed** remote media through the quarantine pipeline (domain and redirect-hop checks, private-address blocking, size caps, MIME sniffing, exact hashes), stores it as local content-addressed resources, and rewrites the note's Markdown to `resource://` links in a new revision. `--dry-run` prints the per-URL policy decisions without fetching a byte or writing anything. `--allow-review` also localizes URLs whose decision is `review`; blocked URLs are never fetched. `--base-revision` guards against concurrent edits (default: the note's current revision). The JSON report lists `localized`, `blocked`, `review`, `failed`, and the new `revision_id`.

The `import joplin-raw` and `import obsidian` commands accept `--localize-media` to run the same engine over every imported note after the import completes.
