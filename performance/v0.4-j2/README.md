# v0.4 J2 real-export dry-run evidence

These reports are private-safe aggregate evidence from the production Joplin
RAW dry-run planner. They contain no source paths, filenames, note titles,
bodies, resource bytes, databases, or warning text.

Reproduce a report with a read-only RAW export:

```bash
bash scripts/run_real_joplin_profile.sh <label> <raw-export-dir> <output.json>
```

The harness runs the full planner, records Go memory and process peak RSS, and
checks that the source directory metadata is unchanged. It writes its temporary
SQLite database and Go build cache outside the source. The CLI likewise writes
no import configuration unless `--write-config` is explicitly supplied.

`recipe-export.json` covers 1,237,553 parsed items. Its 382,206 Joplin notes
exactly match the independently counted Markdown files in the corresponding
Obsidian recipe vault. `attachment-export.json` covers 111,330 items, including
763 resource records. Five resource records had no content file, producing the
five aggregate warnings. The planner rewrote 2,420 resolvable links and created
766 unique note/resource relationship plans; repeated links to the same
resource in one note do not multiply relationships.

The relationship planner scans each note body once and uses direct ID-map
lookups. It does not enumerate every resource for every note. Focused sanitized
fixtures cover all observed supported types (1 note, 2 folder, 4 resource,
5 tag, and 6 note-tag), explicit malformed/unsupported counts, unresolved
targets, deduplication, escaped targets, inline code, fenced code, idempotent
re-import, search readiness, rewritten note links, and attached resources.

These timings describe this machine and filesystem cache, not a product SLO.
J3 remains responsible for complete million-note transactional writes,
interruption/resume, no-op re-import, and any manifest/spool work warranted by
the measured 3.6 GiB dry-run peak RSS at the 1.24-million-item tier.
