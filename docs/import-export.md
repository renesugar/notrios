# Import and export

`notriosctl` imports notes from five sources and round-trips a native archive format. Every importer:

- supports `--dry-run` (scan and report **without writing anything** — no notes, no notebooks, no resources);
- prints a JSON report to stdout and exits non-zero on failure;
- is **idempotent**: re-running an import leaves unchanged notes alone, updates changed ones as new revisions, and never duplicates;
- **never resurrects a note you moved to the Trash** — re-imports only refresh its hidden metadata;
- records *provenance* (source system, original IDs, author, timestamps, URLs), which is why imported notes cannot be permanently deleted from the Trash — they can only be hidden — and why conversation threads stay recoverable.

Both command forms work identically — from source or a built binary:

```sh
go run ./cmd/notriosctl import joplin-raw --dry-run "/path/to/export dir"
./bin/notriosctl   import joplin-raw --dry-run "/path/to/export dir"
```

Quote any path containing spaces.

## Shared options

These flags are accepted by **every** `import` subcommand and by `export archive`:

| Flag | Default | Meaning |
|---|---|---|
| `--config` | none | YAML config file; its `data` section supplies database/asset paths |
| `--db` | `./data/notes.sqlite` (or the config value) | SQLite database path override |
| `--asset-store` | `./data/assets` (or the config value) | attachment storage directory override |
| `--collection` | `default` | collection namespace for imported items |

Precedence: `--db`/`--asset-store` override the config file, which overrides the built-in defaults. Relative paths resolve against your working directory — run from the repository root, or pass absolute paths. The CLI creates missing directories and the database (with migrations) automatically, and writes to the **same** database the service uses, so imported notes appear in a running service/GUI immediately (search again or reload to see them).

**Can the service stay open during an import?** Yes for normal use: the database runs in WAL mode with a 5-second lock timeout, so the CLI and the service can share it. For very large imports, avoid heavy simultaneous editing; if you ever see a `database is locked` error, stop the service, re-run the import (it resumes idempotently), and restart.

**Backup first.** Before any large import, stop the service and copy the
consistent database plus `data/assets/`, or use SQLite's online backup API and
copy assets. Native archive v1 is useful for transfer but omits revisions and
some provenance, so it is not the disaster-recovery backup. See
[backup guidance](service.md#backup-and-restore).

---

## Joplin (RAW export)

### 1. Create the export in Joplin

In the Joplin **desktop** app, use the File menu's export function and choose the **RAW** format. Joplin's documentation describes RAW as "the same as the JEX format except that the data is saved to a directory and each item represented by a single file" — you pick a target folder and Joplin fills it with one `.md` file per note/notebook/tag plus a `resources/` directory.

Notrios needs exactly that **directory**:

```text
joplin-export/
  0a1b2c....md        # notes, notebooks, tags, note-tag links (one item per file)
  3d4e5f....md
  resources/
    9f8e7d....png     # attachment bytes
```

Common mistakes:

- A **`.jex` file is not accepted** — JEX is a tar archive; export RAW instead.
- Do **not** point the importer at Joplin's profile directory or its synchronization target — only at a RAW export you created.

### 2. Dry run

```sh
go run ./cmd/notriosctl import joplin-raw --dry-run "/path/to/joplin-export"
```

The dry run parses everything and prints the JSON report with `"dry_run": true` and the would-be `notes_imported` count. No notes, resources, or notebooks are written (opening the store does create an empty database file with its schema if none existed).

### 3. Import

```sh
go run ./cmd/notriosctl import joplin-raw \
  --db ./data/notes.sqlite \
  --asset-store ./data/assets \
  "/path/to/joplin-export"
```

### What the report means

```json
{
  "source_dir": "...", "collection_id": "default", "dry_run": false,
  "notes_seen": 120,          // note items found in the export
  "notes_imported": 118,      // created this run
  "notes_updated": 0,         // existed with different content; new revision written
  "notes_unchanged": 2,       // identical, or deliberately left in your Trash
  "resources_seen": 40,
  "resources_imported": 39,   // stored content-addressed (deduplicated by hash)
  "resources_existing": 0,    // already present from an earlier run
  "resources_skipped": 1,     // no content file in resources/ — see warnings
  "links_rewritten": 57,      // Joplin :/<id> links converted
  "attachments_created": 39,  // note-resource references
  "warnings": ["resource ab12... has no content file"]
}
```

### What is preserved, transformed, or lost

| Joplin data | In Notrios |
|---|---|
| Note title and Markdown body | preserved; body gains a YAML front-matter block |
| Internal `:/<id>` note/resource links | rewritten to `document://` / `resource://` links that work in the UI |
| Notebook (folder) membership and hierarchy | **recorded as front matter only** (`joplin_notebook: "Parent/Child"`); see below |
| Tags | **recorded as front matter only** (`joplin_tags:` list); full-text searchable, but they do **not** appear in the sidebar tag list |
| Created/updated/user timestamps, `source_url`, author | preserved in front matter; the creation time also becomes the provenance published-time, so `since:`/`until:` queries work |
| Original Joplin IDs | preserved (`joplin_id` front matter; deterministic Notrios IDs `doc_joplin_<id>` / `res_joplin_<id>` keep re-imports idempotent) |
| Attachments | imported into the content-addressed asset store and attached to their notes |

> **Notebook placement:** imported Joplin notes currently all land in the default **"Notes"** notebook — the Joplin notebook tree is *not* recreated as Notrios notebooks. The original notebook path is kept in each note's front matter (and is searchable); recreating source notebook hierarchies is planned importer hardening (`ROADMAP.md` v0.3). The Twitter/ChatGPT/Claude importers, by contrast, do create their own notebook.

### Edge cases (implementation-verified)

- **Malformed items:** the parser is tolerant — it accepts both metadata-first and body-first item files and skips files it cannot classify.
- **Missing resource files:** counted in `resources_skipped` with a warning naming the resource; the import completes.
- **Duplicates / re-import:** deterministic IDs make re-runs safe — unchanged notes count as `notes_unchanged`, notes edited in Joplin become `notes_updated` (a new revision; the previous text stays in revision history).
- **Notes you deleted in Notrios:** stay in the Trash; the importer will not bring them back.

### Verify after importing

1. Search for a phrase you know is in a Joplin note — in the UI, or `curl -s -X POST localhost:8080/api/v1/search -H 'Content-Type: application/json' -d '{"query":"<phrase>","limit":5}'`.
2. Open an imported note; check that internal links navigate and images render.
3. Compare `notes_seen` with your expectation from Joplin, and read every `warnings` entry.

---

## Obsidian (vault directory)

Point the importer at your vault directory (the folder containing your `.md` files; no export step is needed in Obsidian):

```sh
go run ./cmd/notriosctl import obsidian --dry-run "/path/to/vault"
go run ./cmd/notriosctl import obsidian "/path/to/vault"
```

Behavior:

- Scans Markdown notes and non-Markdown assets; skips `.obsidian/`, VCS, and dependency folders.
- Preserves your Markdown as-is — Wikilinks `[[Target]]`, embeds `![[...]]`, and unresolved links survive; link/backlink indexes are refreshed after the whole batch so cross-references resolve regardless of import order (`link_indexes_refreshed` in the report).
- Front matter is preserved and augmented with `source_system: obsidian` and `obsidian_path`; an existing front-matter `title:` wins over the first heading.
- Local assets referenced by notes are imported content-addressed and attached.
- Deterministic IDs derive from vault-relative paths, so re-imports are idempotent. Like Joplin, notes land in the default **"Notes"** notebook today; the vault folder structure survives in `obsidian_path`.
- Report fields mirror Joplin's, with `markdown_seen` instead of `notes_seen` and no link-rewriting counter (links are preserved, not rewritten).

Limitations: Obsidian canvases and plugin-specific syntax import as plain text; front-matter `tags:` are not turned into sidebar tags.

---

## Twitter/X (extracted archive)

Request your archive from X (Settings → download your data), download the ZIP, and **extract it**. The importer wants the extracted directory (the one containing `data/`):

```text
twitter-archive/
  data/
    account.js          # window.YTD.account.part0 = [...]
    tweets.js           # window.YTD.tweets.part0 = [...]  (older archives: tweet.js)
    tweets_media/       # media files named <tweetid>-<name>.<ext>
```

```sh
go run ./cmd/notriosctl import twitter --dry-run "/path/to/twitter-archive"
go run ./cmd/notriosctl import twitter --notebook Twitter "/path/to/twitter-archive"
```

Extra flag: `--notebook` (default `Twitter`) names the notebook the tweets are placed in; it is created with a 🐦 icon if missing (an existing notebook with the same name is reused).

Behavior:

- **Conversation threads are recovered** by following in-reply-to chains among your archived tweets; thread queries return them in chronological order. Replies to other people's (unarchived) tweets keep the external `reply_to` ID.
- `t.co` links are expanded to their real URLs; media becomes embedded resources; hashtags become **real sidebar tags**; each note ends with a link to the original post.
- Provenance records your display name and `@handle` (searchable via `author:"..."` / `authorid:@...`), the tweet ID, thread ID, and posting time (`since:`/`until:` filters work).
- Report: `tweets_seen`, `notes_*`, `threads_recovered`, `media_imported`/`media_missing`, `tags_applied`, `attachments_linked`, `warnings`.

---

## ChatGPT (data export)

Request your data export from ChatGPT's settings and extract the ZIP; the importer takes either the extracted directory or the `conversations.json` inside it:

```sh
go run ./cmd/notriosctl import chatgpt --dry-run "/path/to/export/conversations.json"
go run ./cmd/notriosctl import chatgpt "/path/to/export-dir"
```

Extra flag: `--notebook` (default `ChatGPT`, created with a 🤖 icon).

Each conversation becomes **one Markdown note** with `## User — timestamp` / `## Assistant — timestamp` sections. Only the conversation's *current* branch is imported (abandoned edit/regeneration branches are excluded); system/tool and empty messages are skipped (`messages_skipped`). Unnamed conversations get dated titles. The conversation ID becomes the provenance thread ID.

## Claude (data export)

Same shape as ChatGPT — point at the export's `conversations.json` or its directory:

```sh
go run ./cmd/notriosctl import claude --dry-run "/path/to/conversations.json"
go run ./cmd/notriosctl import claude "/path/to/export-dir"
```

Extra flag: `--notebook` (default `Claude`, created with a ✳️ icon). Messages come from each conversation's flat message list (text content blocks are used when the plain-text field is empty).

---

## Exporting a Notrios archive

```sh
go run ./cmd/notriosctl export archive ./my-archive                      # everything
go run ./cmd/notriosctl export archive --query 'tag:todo' ./my-archive  # query-scoped
```

`--query` accepts the full [query language](query-language.md), so you can export a notebook (`--query 'notebook:"Work"'`), a tag, or any search-notebook query instead of the whole database. Notes in the Trash are excluded.

The archive is a plain directory:

```text
my-archive/
  manifest.json        # format marker, version, the query used
  notebooks.json       # notebook paths and emoji, so nesting survives
  notes/<id>.md        # front matter: id, title, notebook path, tags, resources
  resources/<id>__<filename>
```

This is native archive **v1**: query-scoped, human-readable interchange. It is
not lossless and does not preserve database/profile/replica identity, every
revision, or all provenance. v0.4 plans native archive v2 as a checksum-verified
snapshot/backup format and the full-snapshot layer later reused by sync.

## Importing a Notrios archive

```sh
# 1. Analyze; writes my-archive/import-config.json (or --write-config <path>)
go run ./cmd/notriosctl import archive --dry-run ./my-archive

# 2. Review/edit the renames in import-config.json, then import
go run ./cmd/notriosctl import archive --import-config ./my-archive/import-config.json ./my-archive
```

The dry run classifies each top-level archive notebook name (case-insensitively):

- **creates** — will be created, nesting and emoji preserved;
- **merges** — an existing plain notebook with that name absorbs the archive's notes;
- **conflicts** — the name collides with a notebook **bound to another data source** (a builtin notebook, or one containing imported notes). Conflicts must be renamed: the generated config prefills suggestions like `"Twitter": "Twitter (imported)"`, and the real import re-validates every post-rename name **before writing anything**, refusing with an error if a conflict remains.

Re-imported archive notes are **plain local notes** — not references to their original data source. They behave like notes you wrote yourself, including being permanently deletable from the Trash. Tags and resource attachments from the archive are restored; imports are idempotent.

Report: `notes_seen/imported/updated/unchanged`, `notebooks_created`, `resources_created`, `tags_applied`, plus `conflicts`/`creates`/`merges` on dry runs.

## Trash rules recap

Deleting any note moves it to the Trash search notebook; restoring brings it back. Only purely local notes (including re-imported archive notes) can be permanently deleted from the Trash — source-imported notes marked deleted simply disappear from queries, results, and exports.
