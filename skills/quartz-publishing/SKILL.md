---
name: quartz-publishing
description: Plan and implement privacy-safe Quartz publishing profiles for selected note subsets.
---

# Quartz Publishing Skill

Use this skill when implementing or reviewing publishing/export features.

## Steps

1. Treat publishing as separate from backup/export.
2. Select notes by profile: notebooks, folders, tags, explicit IDs, and recursive include options.
3. Exclude private/draft/confidential notes and resources.
4. Compute reachable public resources from selected notes.
5. Rewrite links to private/missing targets according to profile policy.
6. Strip private metadata.
7. Produce dry-run counts and warnings before writing output.
8. Emit a Quartz-compatible `content/` tree.

## Checks

- No private resources copied because they are merely present in the source vault.
- Dry run reports private-link warnings.
- Output can be deleted and regenerated from canonical database state.
