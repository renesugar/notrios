---
name: joplin-raw-import
description: Import Joplin RAW Export Directory into canonical documents and a Markdown/frontmatter projection.
---

# Joplin RAW Import Skill

Use this skill when the active task matches the description.

## Steps

1. Inventory RAW items before writing records; use the indexed temporary
   manifest for notes/note-tag joins at large scale.
2. Keep only bounded routing maps for notebooks, resources, tags, and stable
   note IDs; page manifest records with keysets after a resume-offset lookup.
3. Preserve original Joplin IDs.
4. Rewrite Joplin `:/id` links only after target classification.
5. Keep RAW source immutable and generate projections separately.
6. Commit each canonical Store batch together with its item states and
   checkpoint; rebuild later-batch links in a final bounded pass.
7. Make import idempotent by source-system/source-object ID and skip identical
   provenance/state rewrites only after both fingerprint and source mapping
   match.

## Working-state checks

- Small RAW fixture imports without duplicate documents.
- Links/resources resolve or are recorded as unresolved.
- Interruption resumes after the last atomic batch, and a complete no-op adds
  no revisions.
- Run 100/10k/100k generated tiers; use aggregate-only real-corpus evidence
  for million-item claims.
