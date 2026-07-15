---
name: joplin-raw-import
description: Import Joplin RAW Export Directory into canonical documents and a Markdown/frontmatter projection.
---

# Joplin RAW Import Skill

Use this skill when the active task matches the description.

## Steps

1. Inventory RAW items before writing records.
2. Build maps for notes, notebooks, resources, tags, and note-tag joins.
3. Preserve original Joplin IDs.
4. Rewrite Joplin `:/id` links only after target classification.
5. Keep RAW source immutable and generate projections separately.
6. Make import idempotent by source-system/source-object ID.

## Working-state checks

- Small RAW fixture imports without duplicate documents.
- Links/resources resolve or are recorded as unresolved.
