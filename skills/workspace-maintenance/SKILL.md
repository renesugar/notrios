---
name: workspace-maintenance
description: Design and implement lint, query blocks, outlines, block anchors, and graph maintenance.
---

# Workspace Maintenance Skill

Use this skill for Foam/Obsidian-style knowledge-base maintenance.

## Principles

- Prefer structured queries over arbitrary SQL/JavaScript in notes.
- Provide dry-run before auto-fix.
- Track source positions where edits will occur.
- Default deletes to trash/soft delete.
- Keep stable IDs separate from human-readable titles/paths.

## Candidate tools

- `lint_workspace`
- `fix_workspace_issues`
- `get_document_outline`
- `list_document_blocks`
- `rename_tag`
- `resolve_note_reference`
