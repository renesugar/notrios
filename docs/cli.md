# The notriosctl CLI

`notriosctl` handles imports, exports, and maintenance against the same database the service uses.

```text
notriosctl import joplin-raw  [options] <raw-export-dir>
notriosctl import obsidian    [options] <vault-dir>
notriosctl import twitter     [options] <extracted-archive-dir>
notriosctl import chatgpt     [options] <conversations.json|export-dir>
notriosctl import claude      [options] <conversations.json|export-dir>
notriosctl import archive     [options] <archive-dir>
notriosctl export archive     [options] <out-dir>
notriosctl seed-help          [options] [docs-dir]
```

Common options: `--config`, `--db`, `--asset-store`, `--collection`, `--dry-run`. Importers print a JSON report and are idempotent — re-running an import updates changed notes, leaves unchanged ones alone, and never resurrects notes you moved to the Trash.

## Import sources

- **Joplin RAW** — the preferred Joplin bulk format; notebooks/tags metadata is preserved in front matter, `:/id` links become Notrios links, and resources are imported.
- **Obsidian** — Markdown and assets from a vault; wikilinks, embeds, and front matter are preserved.
- **Twitter/X** — an extracted archive ZIP; conversation threads are recovered from reply chains, media becomes attachments, hashtags become tags, and every note links back to the original post.
- **ChatGPT / Claude** — `conversations.json` exports; each conversation becomes one note with role/timestamp sections.

See [import & export](import-export.md) for the archive format and the rename-on-import dry run.
