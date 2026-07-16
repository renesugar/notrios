# Import and export

## Exporting

```bash
notriosctl export archive --query 'tag:todo' ./my-archive
```

Exports write a **Notrios archive**: a directory with a manifest, the notebook structure (paths and emojis), every note as Markdown with front matter, and the attached resource files. The `--query` flag scopes the export using the [query language](query-language.md) — export a single notebook, a tag, or any search-notebook query instead of the whole database. Notes marked deleted are excluded.

## Importing an archive

```bash
# 1. See what would happen; writes my-archive/import-config.json
notriosctl import archive --dry-run ./my-archive

# 2. (Optionally) edit the generated renames, then import
notriosctl import archive --import-config ./my-archive/import-config.json ./my-archive
```

The dry run classifies each top-level notebook name from the archive:

- **creates** — the notebook doesn't exist yet and will be created (nesting and emoji preserved);
- **merges** — a plain notebook with that name exists; archive notes join it;
- **conflicts** — the name collides with a notebook *bound to another data source* (a builtin notebook, or one holding imported Twitter/ChatGPT/Claude/Joplin/Obsidian notes). Conflicts must be renamed; the generated `import-config.json` prefills suggestions like `"Twitter": "Twitter (imported)"`, and the real import re-checks every name before writing anything.

Re-imported notes are **plain local notes** — not references to their original source — so they behave like notes you wrote yourself (including permanent deletion from the Trash).

## Trash rules

Deleting a note moves it to the Trash search notebook; restore brings it back. Only notes stored purely in the local database can be permanently deleted — imported notes marked deleted simply disappear from queries, results, and exports.
