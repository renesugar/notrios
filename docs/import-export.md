# Import and export

`notriosctl` imports notes from five sources and round-trips a native archive format. Every importer:

- supports `--dry-run` (scan and report **without writing anything** — no notes, no notebooks, no resources);
- prints a JSON report to stdout and exits non-zero on failure;
- is **idempotent**: re-running an import leaves unchanged notes alone, updates changed ones as new revisions, and never duplicates;
- **never resurrects a note you moved to the Trash** — re-imports only refresh its hidden metadata;
- records *provenance* (source system, original IDs, author, timestamps, URLs), which is why imported notes cannot be permanently deleted from the Trash — they can only be hidden — and why conversation threads stay recoverable.

Both command forms work identically — from source or a built binary:

<!-- notrios:generated:example:import-export-import-and-export-example-1:begin -->
```sh
go run ./cmd/notriosctl import joplin-raw --dry-run "/path/to/export dir"
./bin/notriosctl   import joplin-raw --dry-run "/path/to/export dir"
```
<!-- notrios:generated:example:import-export-import-and-export-example-1:end -->

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

<!-- notrios:generated:example:import-export-2-dry-run-example-1:begin -->
```sh
go run ./cmd/notriosctl import joplin-raw --dry-run "/path/to/joplin-export"
```
<!-- notrios:generated:example:import-export-2-dry-run-example-1:end -->

The dry run uses the same deterministic inventory and action planner as the
real import. It reports per-type inventory totals, malformed/unsupported items,
unresolved links, missing resource content, and note/resource/notebook/tag
creates, updates, and skips. The suggested rename configuration is included in
the JSON report; add `--write-config /path/to/import-config.json` to save it.
Without that explicit flag, the source tree is never written. No notes,
resources, notebooks, tags, checkpoints, or source-bundle bytes are written
(opening the store does create an empty database file with its schema if none
existed).

### 3. Import

<!-- notrios:generated:example:import-export-3-import-example-1:begin -->
```sh
go run ./cmd/notriosctl import joplin-raw \
  --db ./data/notes.sqlite \
  --asset-store ./data/assets \
  --batch-size 100 \
  --import-config "/path/to/joplin-export/import-config.json" \
  "/path/to/joplin-export"
```
<!-- notrios:generated:example:import-export-3-import-example-1:end -->

The import runs in bounded batches (1–500, default 100). A durable checkpoint
records the inventory fingerprint, phase, next item, cumulative report, and
per-item fingerprints. Re-run the same command after an interruption: it
resumes the next durable batch if the source inventory is unchanged. If the
export changed, a new plan begins and unchanged item fingerprints are skipped.
Batch progress is printed to stderr; the final machine-readable report remains
the only stdout output.

For large exports, inventory rows and note-tag joins are spooled into a
temporary indexed SQLite manifest rather than retained with note bodies in
memory. Each canonical note batch commits documents, revisions, FTS5 rows,
provenance, tags, resource references, projection outbox jobs, item
fingerprints, and its checkpoint in one transaction. After all notes exist, a
bounded final pass resolves links whose targets were created in later batches.
The temporary manifest is removed on normal completion.

Add `--preserve-source` when an exact archival copy matters. Notrios then
stores every classified RAW item byte-for-byte under the source-bundle asset
namespace and records its original relative path, SHA-256, byte size, unknown
properties, and property order. This is separate from canonical note content
and ordinary resource garbage collection.

### What the report means

```json
{
  "source_dir": "...", "source_key": "...",
  "collection_id": "default", "dry_run": false,
  "resumed": false, "checkpoint_status": "completed",
  "batches_completed": 8,
  "canonical_document_batches": 2,
  "link_rebuild_batches": 2,
  "temporary_manifest_bytes": 262144,
  "notes_seen": 120,          // note items found in the export
  "notes_imported": 118,      // created this run
  "notes_updated": 0,         // existed with different content; new revision written
  "notes_unchanged": 2,       // identical, or deliberately left in your Trash
  "notebooks_seen": 12,
  "notebooks_created": 10,
  "notebooks_updated": 0,
  "notebooks_skipped": 2,
  "tags_seen": 25,
  "tags_created": 20,
  "tags_updated": 0,
  "tags_skipped": 5,
  "tags_applied": 96,
  "tags_removed": 0,
  "resources_seen": 40,
  "resources_imported": 39,   // stored content-addressed (deduplicated by hash)
  "resources_updated": 0,
  "resources_existing": 0,    // already present from an earlier run
  "resources_skipped": 1,     // no content file in resources/ — see warnings
  "source_bundle_items": 0,   // nonzero with --preserve-source
  "source_bundle_bytes": 0,
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
| Notebook (folder) membership and hierarchy | restored as nested Notrios notebooks with original names; also recorded in front matter |
| Tags | restored as stable real Notrios tags (including unassigned source tags); also recorded in front matter |
| Created/updated/user timestamps, `source_url`, author | preserved in front matter; the creation time also becomes the provenance published-time, so `since:`/`until:` queries work |
| Original Joplin IDs | preserved (`joplin_id` front matter; deterministic Notrios IDs `doc_joplin_<id>` / `res_joplin_<id>` keep re-imports idempotent) |
| Attachments | imported into the content-addressed asset store and attached to their notes |
| Unknown properties and exact RAW layout | retained only when `--preserve-source` is enabled; exact bytes and property order remain in the source bundle |

If a Joplin notebook name collides with a builtin or another source-bound
notebook, the real import refuses before writing. Dry run suggests a
path-scoped rename in `import-config.json`. A same-named plain local notebook
is merged deliberately and reported as such.

### Edge cases (implementation-verified)

- **Canonical RAW titles:** Joplin's first physical line becomes the Notrios
  title and is removed from the canonical Markdown body. The legacy
  metadata-first shape remains accepted for compatibility.
- **PDF OCR controls:** metadata is split only on CR/LF physical endings, so
  vertical tab, form feed, file/record separators, and NEL remain inside one
  `ocr_text` value. Future and duplicate property keys retain their source
  order when `--preserve-source` is enabled.
- **Encoding:** an optional UTF-8 BOM is accepted; invalid UTF-8 is rejected
  with an input error instead of being silently replaced.
- **Malformed/unsupported items:** files that cannot be classified are skipped
  but counted explicitly; unsupported parsed `type_` values are also counted.
- **Missing resource files:** counted in `resources_skipped` with a warning naming the resource; the import completes.
- **Duplicates / re-import:** deterministic IDs make re-runs safe — unchanged notes count as `notes_unchanged`, notes edited in Joplin become `notes_updated` (a new revision; the previous text stays in revision history).
- **Interrupted imports:** completed batches and their cumulative report are
  durable. Re-running resumes at the stored phase/index without duplicating
  notes, tags, notebooks, or resources.
- **Notes you deleted in Notrios:** stay in the Trash; the importer will not bring them back.

### Verify after importing

1. Search for a phrase you know is in a Joplin note — in the UI, or `curl -s -X POST localhost:8080/api/v1/search -H 'Content-Type: application/json' -d '{"query":"<phrase>","limit":5}'`.
2. Open an imported note; check that internal links navigate and images render.
3. Compare `notes_seen` with your expectation from Joplin, and read every `warnings` entry.

---

## Obsidian (vault directory)

Point the importer at your vault directory (the folder containing your `.md` files; no export step is needed in Obsidian):

<!-- notrios:generated:example:import-export-obsidian-vault-directory-example-1:begin -->
```sh
go run ./cmd/notriosctl import obsidian --dry-run \
  --write-config "/path/to/vault/.notrios/import-config.json" "/path/to/vault"
go run ./cmd/notriosctl import obsidian --preserve-source \
  --import-config "/path/to/vault/.notrios/import-config.json" "/path/to/vault"
```
<!-- notrios:generated:example:import-export-obsidian-vault-directory-example-1:end -->

Behavior:

- Scans once, in deterministic relative-path order; skips `.obsidian/`,
  `.notrios/`, VCS, Trash, and dependency folders. Symlinks are refused.
- Restores the vault folder hierarchy as nested notebooks. A collision with a
  builtin or source-bound sibling is reported by dry run with a path-scoped
  rename; a same-named plain local notebook can be merged deliberately.
- Resolves note filenames, frontmatter aliases, vault-root and note-relative
  paths. Wikilinks, Markdown links, note/resource embeds, heading anchors, and
  block references are canonicalized to stable Notrios URIs; unresolved or
  ambiguous source syntax remains in the canonical note with a warning.
- Preserves frontmatter as canonical metadata and augments it with
  `source_system`, `obsidian_path`, and `obsidian_folder`. A frontmatter
  `title:` wins over the first heading.
- `--preserve-source` keeps each original Markdown file (therefore its exact
  frontmatter bytes and line endings), relative path, and every discovered
  non-Markdown file byte-for-byte in the source-bundle store. Canonical parsing
  never replaces that source representation.
- Imports local assets content-addressed, attaches referenced assets, and
  updates changed bytes behind the same deterministic resource ID.
- Fingerprints and bounded 1–500 item batches make re-runs idempotent and
  resumable. Dry run uses the same create/update/unchanged classifiers as the
  real import and writes no import checkpoint or content.

Limitations: Obsidian canvases and plugin-specific syntax import as ordinary
non-Markdown source files; frontmatter `tags:` are not yet turned into sidebar
tags. Test first on a copy of a real vault and inspect warnings.

---

## Twitter/X archive

Request your archive from X (Settings → download your data) and download the ZIP. **Give the importer the ZIP as it downloaded**: there is no need to extract it. The importer reads it in place and finds everything it needs itself. An archive you have already extracted works too: pass the folder containing `data/`.

<!-- notrios:generated:example:import-export-twitter-x-archive-example-1:begin -->
```sh
go run ./cmd/notriosctl import twitter --dry-run "/path/to/twitter-archive.zip"
go run ./cmd/notriosctl import twitter --notebook Twitter "/path/to/twitter-archive.zip"
```
<!-- notrios:generated:example:import-export-twitter-x-archive-example-1:end -->

What it reads, for reference:

```text
data/
  account.js            # window.YTD.account.part0 = [...]
  tweets.js             # window.YTD.tweets.part0 = [...]  (older archives: tweet.js)
  tweets-part1.js       # a large archive continues its posts in part files,
  tweets-part2.js       #   read in order after tweets.js
  community-tweet.js    # posts to X Communities, imported like any post
  deleted-tweets.js     # posts you deleted: counted, not imported
  tweet-headers.js      # one line per post, used to check nothing was missed
  tweets_media/         # media files named <tweetid>-<name>.<ext>
```

Extra flag: `--notebook` (default `Twitter`) names the notebook the tweets are placed in; it is created with a 🐦 icon if missing (an existing notebook with the same name is reused).

Behavior:

- **Conversation threads are recovered** by following in-reply-to chains among your archived tweets; thread queries return them in chronological order. Replies to other people's (unarchived) tweets keep the external `reply_to` ID.
- `t.co` links are expanded to their real URLs; media becomes embedded resources; hashtags become **real sidebar tags**; each note ends with a link to the original post.
- Provenance records your display name and `@handle` (searchable via `author:"..."` / `authorid:@...`), the tweet ID, thread ID, and posting time (`since:`/`until:` filters work).
- **Every post is imported**, however many part files the archive splits them across. Posts you deleted on X are not brought back: they are counted in the report and left out.
- **The count is checked.** When the archive has `tweet-headers.js`, the report compares the posts found with the number it lists, and warns if they differ.
- **A hostile archive is refused, not trusted.** Entries whose names escape the archive are skipped and counted. A data file, a media file or an entry count over the import's limits stops that file or the import with a clear error, and nothing is extracted to disk.
- Report: `source_format` (`zip` or `directory`), `tweet_files`, `posts_in_tweet_files`, `tweet_headers`, `tweets_seen`, `community_posts_seen`, `deleted_posts_skipped`, `duplicate_posts_skipped`, `archive_entries_rejected`, `notes_*`, `threads_recovered`, `media_files_in_archive`, `media_unmatched` (media files naming no imported post), `media_imported`/`media_missing`, `tags_applied`, `attachments_linked`, `warnings`.

---

## ChatGPT (data export, and the OpenAI Privacy Portal export)

ChatGPT is exported in two shapes, and the importer takes either **as the ZIP you downloaded** — there is no need to unzip anything:

<!-- notrios:generated:example:import-export-chatgpt-data-export-and-the-openai-privacy-portal-export-example-1:begin -->
```sh
go run ./cmd/notriosctl import chatgpt --dry-run "/path/to/chatgpt-export.zip"
go run ./cmd/notriosctl import chatgpt "/path/to/OpenAI-export.zip"
```
<!-- notrios:generated:example:import-export-chatgpt-data-export-and-the-openai-privacy-portal-export-example-1:end -->

An extracted folder, or a path straight to a `conversations.json`, works too.

| What you downloaded | What it holds |
|---|---|
| **from ChatGPT** (Settings → Data controls → Export) | `conversations.json` beside `chat.html` and the asset files, named `file-<id>-<name>.<ext>` or `file_<hash>-<name>.<ext>` |
| **from the OpenAI Privacy Portal** | an outer ZIP whose `User Online Activity/` folder holds **ZIPs of its own**: `Conversations__….zip` (with `conversations-000.json`, `conversations-001.json`, … and assets stripped to `file-<id>.dat`) and `Files__….zip` (your ChatGPT file library) |

Extra flag: `--notebook` (default `ChatGPT`, created with a 🤖 icon).

Behaviour:

- **Every conversation becomes its own note**, and every `conversations*.json` shard is read, in order.
- **Nested ZIPs are read in place.** Nothing is extracted; a large nested archive is spooled into the instance's own temp directory (`data.temp_dir`) and removed afterwards.
- **`.dat` assets get their real names back**, from `conversation_asset_file_names.json`, then `library_files.json` (which also carries the MIME type), and otherwise from the file's own bytes. The report counts which source named how many. `chat.html` shows the `.dat` names in a Privacy Portal export; the importer reads the JSON instead, so notes name files properly.
- **Attachments become resources** and are embedded in the note; file IDs match under either the `file-` or `file_` prefix.
- **The file library is imported.** A file a conversation references is attached to that note; one nothing references gets a note of its own in a **ChatGPT Files** notebook, so it stays searchable.
- **Code and output are rendered** as fenced code blocks, and browsing results as text. The model's thinking, reasoning recaps and tool plumbing are **not** imported; the report counts them.
- Report: `source_format`, `archive_files`, `nested_archives`, `conversations_seen`, `notes_*`, `messages_*`, `code_blocks`, `machinery_skipped`, `assets_referenced`/`assets_imported`/`assets_missing`, `asset_names_from_map`/`asset_names_from_library`/`asset_names_sniffed`, `library_files_seen`/`library_files_imported`/`library_stub_notes`, `archive_entries_rejected`, `warnings`.

---

## Claude (data export)

Give the importer the ZIP as you downloaded it. A large export arrives as several `...-batch-0000.zip`, `...-batch-0001.zip` files: point at the folder holding them and they import as one export.

<!-- notrios:generated:example:import-export-claude-data-export-example-1:begin -->
```sh
go run ./cmd/notriosctl import claude --dry-run "/path/to/claude-data-export.zip"
go run ./cmd/notriosctl import claude "/path/to/export-folder"
```
<!-- notrios:generated:example:import-export-claude-data-export-example-1:end -->

Extra flag: `--notebook` (default `Claude`, created with a ✳️ icon).

Behaviour:

- **Every conversation becomes its own note.** Messages come from each conversation's flat message list, using the text content blocks when the plain-text field is empty.
- **Each project becomes a note too**, with its description, prompt template and each of its docs.
- **Attachments keep their text.** A Claude export carries an attachment's extracted text rather than its bytes, so the note holds that text under the file's name. A file the export only names is shown as not in the archive.
- **Thinking and tool blocks are not imported**, and are counted in the report.
- Report: `source_format`, `archive_files`, `batch_archives`, `conversations_seen`, `notes_*`, `messages_*`, `code_blocks`, `attachments_seen`, `files_referenced`, `machinery_skipped`, `projects_seen`/`projects_imported`/`project_docs`, `archive_entries_rejected`, `warnings`.

---

## Exporting a Notrios archive

<!-- notrios:generated:example:import-export-exporting-a-notrios-archive-example-1:begin -->
```sh
go run ./cmd/notriosctl export archive ./my-archive                      # everything
go run ./cmd/notriosctl export archive --query 'tag:todo' ./todo-archive # query-scoped
```
<!-- notrios:generated:example:import-export-exporting-a-notrios-archive-example-1:end -->

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
revision, or all provenance.

## Exporting a native archive v2 snapshot

<!-- notrios:generated:example:import-export-exporting-a-native-archive-v2-snapshot-example-1:begin -->
```sh
# Complete database backup.
go run ./cmd/notriosctl export archive-v2 ./notrios-backup

# Explicitly scoped subset transfer.
go run ./cmd/notriosctl export archive-v2 --target subset_transfer \
  --notebooks nb_research --tags shared ./research-transfer
```
<!-- notrios:generated:example:import-export-exporting-a-native-archive-v2-snapshot-example-1:end -->

Archive **v2** is the lossless format: every saved revision, trashed notes,
notebooks and tags, links, provenance, resources, exact source bundles, and
logical database/replica identity, all as immutable SHA-256 objects under a
manifest published last and verified before the command succeeds. See the
[archive v2 safety contract](archive-v2.md) for the modes, the interruption and
resume behavior, and the current object-count bound.

Selection goes through the same [selection/privacy
planner](selection-planning.md) you can dry-run first, and the manifest binds
that plan's digest, so a reviewed dry run and the archive it produced can be
matched afterwards.

## Verifying and restoring an archive v2 snapshot

<!-- notrios:generated:example:import-export-verifying-and-restoring-an-archive-v2-snapshot-example-1:begin -->
```sh
# Read-only integrity/contents check.
go run ./cmd/notriosctl verify archive-v2 ./notrios-backup

# Restore into a fresh database. The intent is mandatory.
go run ./cmd/notriosctl restore archive-v2 --intent adopt \
  --db ./restored/notes.sqlite ./notrios-backup
```
<!-- notrios:generated:example:import-export-verifying-and-restoring-an-archive-v2-snapshot-example-1:end -->

`adopt` restores into an empty database and keeps the archive's logical database
ID; `replace` restores over an existing one; `merge` imports records into an
existing database that keeps its own identity; `fork` creates a new logical
database with `--new-database-id`. Verification completes before the first write,
and an interrupted restore leaves a marker that only `--intent replace` can
recover. See the [archive v2 safety contract](archive-v2.md).

## Fast same-schema whole-library snapshots

For a complete local library on the current schema, the frozen G14c-G14e path
provides the physical snapshot representation selected and accepted against the
full corpora:

<!-- notrios:generated:example:import-export-fast-same-schema-whole-library-snapshots-example-1:begin -->
```bash
go run ./cmd/notriosctl snapshot create --db data/notes.sqlite \
  --asset-store data/assets ./notrios-physical-snapshot
go run ./cmd/notriosctl snapshot verify ./notrios-physical-snapshot
go run ./cmd/notriosctl snapshot restore --intent replace \
  --db data/notes.sqlite --asset-store data/assets \
  ./notrios-physical-snapshot
```
<!-- notrios:generated:example:import-export-fast-same-schema-whole-library-snapshots-example-1:end -->

This is not a subset export and cannot be merged. It binds one consistent
SQLite Online Backup image to deterministic bounded packs of every
database-declared local resource and preserved source bundle. Restore requires
the service to be stopped and an explicit `replace` (same database ID) or
`adopt` intent. It creates and verifies an emergency physical snapshot, records
each cutover stage durably, mints a new replica ID, and leaves the installed
replica ready to re-enroll and replay operations after the snapshot floor.
Use packed archive-v2 for portable interchange, selective transfer, merge, and
fallback when schema compatibility is not exact.

## Importing a Notrios archive

<!-- notrios:generated:example:import-export-importing-a-notrios-archive-example-1:begin -->
```sh
# 1. Analyze; writes my-archive/import-config.json (or --write-config <path>)
go run ./cmd/notriosctl import archive --dry-run ./my-archive

# 2. Review/edit the renames in import-config.json, then import
go run ./cmd/notriosctl import archive --import-config ./my-archive/import-config.json ./my-archive
```
<!-- notrios:generated:example:import-export-importing-a-notrios-archive-example-1:end -->

The dry run classifies each top-level archive notebook name (case-insensitively):

- **creates** — will be created, nesting and emoji preserved;
- **merges** — an existing plain notebook with that name absorbs the archive's notes;
- **conflicts** — the name collides with a notebook **bound to another data source** (a builtin notebook, or one containing imported notes). Conflicts must be renamed: the generated config prefills suggestions like `"Twitter": "Twitter (imported)"`, and the real import re-validates every post-rename name **before writing anything**, refusing with an error if a conflict remains.

Re-imported archive notes are **plain local notes** — not references to their original data source. They behave like notes you wrote yourself, including being permanently deletable from the Trash. Tags and resource attachments from the archive are restored; imports are idempotent.

Report: `notes_seen/imported/updated/unchanged`, `notebooks_created`, `resources_created`, `tags_applied`, plus `conflicts`/`creates`/`merges` on dry runs.

## Trash rules recap

Deleting any note moves it to the Trash search notebook; restoring brings it back. Only purely local notes (including re-imported archive notes) can be permanently deleted from the Trash — source-imported notes marked deleted simply disappear from queries, results, and exports.
