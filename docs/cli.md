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

Prints the version string (currently `0.4.0`) and exits 0. No flags.

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
ok    database         ./data/notes.sqlite (schema version 13)
info  web ui           web/dist missing here; run `make web` or serve API-only
doctor: required checks passed
```

## import

Six variants; all support `--dry-run` and the shared `--config/--db/--asset-store/--collection` flags. Full workflows, source-application export steps, report-field explanations, and preserved-metadata tables are in [import & export](import-export.md).

### import joplin-raw

```sh
notriosctl import joplin-raw [shared flags] [--batch-size 100] [--preserve-source] \
  [--dry-run] [--write-config path] [--import-config path] <raw-export-dir>
```

Positional argument: the Joplin **RAW export directory** (not a `.jex` file).
The importer restores nested notebooks and real tags. `--batch-size` is
bounded to 1–500 (default 100), and durable checkpoints resume an interrupted
run when the deterministic source fingerprint still matches.
`--preserve-source` stores exact RAW item bytes, unknown fields, and property
order in a separate content-addressed source bundle.

`--dry-run` uses the real action planner and returns its suggested configuration
inside the JSON report. It writes that configuration only when
`--write-config path` is supplied. Resolve any suggested notebook-path renames,
then pass the file to the real import with `--import-config`. Dry run creates no
source file, import checkpoint, note, resource, notebook, tag, or source-bundle
object.

### import obsidian

```sh
notriosctl import obsidian [shared flags] [--batch-size 100] [--preserve-source]
  [--dry-run] [--write-config path] [--import-config path] <vault-dir>
```

Positional argument: the vault directory. Folder hierarchy becomes nested
notebooks. `--batch-size` is bounded to 1–500 (default 100); matching durable
checkpoints resume interrupted imports. `--preserve-source` captures exact
Markdown/frontmatter and non-Markdown bytes with their relative paths.

Dry run uses the real action classifiers and writes a folder-conflict
configuration (default `<vault>/.notrios/import-config.json`). Apply suggested
path-scoped renames with `--import-config`. Dry run creates no canonical rows,
checkpoint, or source-bundle object.

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

Writes a native archive **v1** directory: query-scoped, human-readable interchange. `--query` (default empty = all non-trashed notes) scopes the export with the [query language](query-language.md). The report lists `notes`, `notebooks`, `resources`, and any `warnings`. v1 is not a lossless backup — use `export archive-v2` for that.

## export archive-v2

```sh
notriosctl export archive-v2 [shared flags] [--target full_archive|subset_transfer]
    [--notebooks id,id] [--tags a,b] [--query "tag:todo"] [--documents id,id]
    [--match any|all] [--max-documents N] [--records-per-object N]
    [--pack] [--pack-bytes N] [--overwrite] [--no-verify] <out-dir>
```

Writes the lossless [native archive v2](archive-v2.md) snapshot: immutable SHA-256 objects with `manifest.json` published last, verified before the command reports success. `--target full_archive` (the default) is the complete-backup mode and takes no selectors; `--target subset_transfer` requires at least one selector and is never described as a backup. Selectors go through the shared [selection/privacy planner](selection-planning.md), and the manifest binds that plan's digest.

Re-running the command over an interrupted export reuses already-published objects and prunes objects the new manifest does not list; `--overwrite` is required to replace an archive that already has a manifest. `--no-verify` skips the post-publication verification pass (not recommended for backups). The JSON report includes `full_backup`, `commit_sha256`, typed `counts`, object and byte totals, `reused_objects`, `cleared_link_targets`, and `warnings`.

`--pack` writes objects into a few large pack files instead of one file per object (`--pack-bytes` sets the target size, default 256 MiB). On a real 382,206-note library that is 46 files instead of 382,447, and about 1.29× faster, at roughly 11% more disk. Loose storage remains the default because it deduplicates and resumes through the object tree; an interrupted packed export restarts instead.

## verify archive-v2

```sh
notriosctl verify archive-v2 <archive-dir>
```

Reads an archive **read-only** and prints what it contains: format/version, snapshot and database identity, capabilities, typed record counts, object and byte totals. It fails on an absent manifest, a missing or damaged object, checksum drift, unknown required capabilities, count or reference inconsistencies, unsafe paths, symlinks, and extra files. Nothing is written and no database is opened.

## restore archive-v2

```sh
notriosctl restore archive-v2 [shared flags] --intent replace|adopt|merge|fork
    [--new-database-id id] [--batch-size N] <archive-dir>
```

Admits a verified archive into a database. `--intent` is required — each choice has a different consequence for the logical database universe and Notrios never guesses one:

| Intent | Target | Result |
|---|---|---|
| `adopt` | must be empty | archive's logical database ID, new replica ID |
| `replace` | existing database | contents replaced; archive's database ID, new replica ID |
| `merge` | existing database | records imported; target keeps its identity and replica ID |
| `fork` | any | new logical database ID from `--new-database-id` |

Verification completes in full before the first canonical write, so a damaged archive leaves the target untouched. Both object layouts are read; every note body, resource, and source bundle is re-hashed as it is used, and attachments are re-sniffed through the ordinary resource admission path rather than trusted from archive metadata.

An interrupted restore leaves a durable marker naming the snapshot it was applying. That library is neither empty nor complete: `adopt`, `merge`, and `fork` refuse it and only `--intent replace` recovers it. The JSON summary reports the intent, resulting database/replica IDs, and applied record counts.

## link

```sh
notriosctl link [--config config.yaml] [--db path] [--asset-store path]
    [--anchor slug|text|^marker] [--list-anchors] <document-id>
```

Prints the [stable link](stable-links.md) for a note: the portable
`notrios://databases/{database_id}/documents/{document_id}` form that survives
moving the database or renaming the profile. The report also carries the note's
title and internal `document://` URI.

`--anchor` points the link at a heading or a block. Give a heading slug, the
heading's text (normalized to the slug), a `^marker`, or a block ID; an anchor
that does not resolve is refused rather than printed. `--list-anchors` prints
what the note offers, with backlink counts.

## open

```sh
notriosctl open [--registry path] [--profile name] [--db path] [--launch] <notrios-uri>
```

Resolves a stable link on this machine. Without `--db` the [profile
registry](stable-links.md) decides which database answers, and only when the
answer is unambiguous. Exit `0` means the link named a note this machine can
open, `1` means it could not be resolved (unregistered database, several
candidate profiles, or a note that no longer exists), and `2` means the link
was malformed — the codes matter because the desktop protocol handler runs this
without a terminal.

`--profile` settles ambiguity between profiles holding clones of one database;
it cannot redirect a link into a different database. `--launch` opens the
resolved note in the local web UI with `xdg-open`.

## profile

```sh
notriosctl profile register --name <profile> [--db path] [--asset-store path] [--registry path]
notriosctl profile list [--registry path]
notriosctl profile forget --name <profile> [--registry path]
```

Manages the local registry (default `~/.config/notrios/profiles.json`, override
with `--registry` or `NOTRIOS_PROFILE_REGISTRY`) that maps a logical database ID
to a database on this machine. `register` reads the database ID out of the
database itself — it cannot be asserted on the command line. `forget` edits the
registry only and never touches the database it named.

## register-url-handler

```sh
notriosctl register-url-handler [--apply] [--binary path] [--dir path]
```

Ubuntu/XDG protocol-handler registration for `notrios://`. It prints the
desktop entry and changes nothing unless `--apply` is given, because installing
it changes what happens when you click such a link anywhere on the machine. The
entry claims `x-scheme-handler/notrios` and nothing else.

## publish

```sh
notriosctl publish profile save --name <profile> [--target publication_handoff|subset_transfer]
    [--notebooks id,id] [--tags a,b] [--query "..."] [--documents id,id] [--match any|all]
    [--exclude-tags a,b] [--private-tags a,b] [--link-action plain_text|redact|report|retain]
    [--include-provenance] [--include-source-bundles] [--max-resource-bytes N] [--profiles path]
notriosctl publish profile list [--profiles path]
notriosctl publish profile delete --name <profile> [--profiles path]
notriosctl publish plan --profile <profile> [--detail-limit 100]
notriosctl publish run --profile <profile> --reviewed-plan <sha256> [--overwrite] <out-dir>
```

Saves, reviews, and runs a [publication handoff](publishing.md): a scoped,
sanitized archive-v2 directory containing only the notes you selected, with
current text only — no revision history, trashed notes, provenance, exact source
bundles, or saved searches.

`publish plan` writes nothing and prints the notes, resources, link decisions,
metadata decisions, warnings, and a `manifest_sha256`. `publish run` requires
that digest and re-plans before writing: if anything about the library changed
since the review, it refuses. `--target full_archive` is refused at save time —
a backup must not be reachable through a name that sounds like publishing.

Profiles live in `<data-dir>/publish-profiles.json` (override with `--profiles`
or `NOTRIOS_PUBLISH_PROFILES`) and are written owner-only.

## lint

```sh
notriosctl lint [--config config.yaml] [--db path] [--asset-store path] [--collection default]
    [--checks a,b] [--detail-limit 100] [--quiet] [--list-checks]
```

Read-only [workspace lint](operations.md#finding-what-has-rotted-workspace-lint):
broken document and resource links, ambiguous wikilinks, unresolved block
anchors, duplicate external identities, missing titles, unlocalized remote
media, missing alt text, unreferenced resources, and projection backlog.

Exit `0` when the library is clean and `1` when findings exist, so
`notriosctl lint --quiet` works in a hook. `--detail-limit` caps the examples
printed per check, never the counts, and `report_sha256` covers every finding —
the same library produces the same digest. Nothing is written; fixing is a
separate operation.

## fix

```sh
notriosctl fix [--config config.yaml] [--db path] [--asset-store path] [--collection default]
    [--kinds a,b] [--document id] [--max-documents 1000] [--apply] [--allow-review] [--list-kinds]
```

Repairs the [mechanically safe subset](operations.md#fixing-what-can-be-fixed-mechanically)
of what `notriosctl lint` reports. Dry run is the default and prints the exact
`before`/`after` of every edit; `--apply` writes them one note at a time, each
against the revision its plan was computed from, so a note edited in between
fails while the rest proceed. Every fix writes an ordinary revision.

`non_canonical_link_target` runs by default. `missing_alt_text` and
`unlocalized_remote_media` are opt-in — the first because a filename is not a
description, the second because it reaches the network (through the full media
policy, exactly as `notriosctl localize` does). Findings outside this set stay
reported and unfixed rather than guessed at.

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

## resources report

```sh
notriosctl resources report [--config config.yaml] [--db path] [--asset-store path]
```

Prints a read-only JSON report of logical resources sharing an exact SHA-256
blob, blobs with no document references, and per-notebook resource usage.
References held by trashed notes still count, so the unreferenced list is safe
input for the retention-aware garbage collector. The perceptual section
is empty by default because Notrios ships no perceptual algorithm; an installed
hook may add review-only policy matches and near-duplicate suggestions.

## gc

```sh
notriosctl gc [--config config.yaml] [--db path] [--asset-store path] [--dry-run | --apply]
```

Plans retention-aware resource garbage collection. With no mode flag—or with
`--dry-run`—it only reports:

- `eligible`: unreferenced logical resources whose configured retention window
  and retention gate are satisfied;
- `retained`: unreferenced resources kept, with the exact reason and eligible
  timestamp;
- `removed`: empty during a dry run;
- physical blob/byte totals and warnings.

Only `--apply` deletes. Apply rechecks every reference inside the SQLite
transaction; referenced resources, including resources referenced only by
notes in Trash, are never eligible. A shared physical blob remains until its
last logical resource is removed.
