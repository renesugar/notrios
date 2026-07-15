# Plan Archive: MVP Document CRUD and Revisions

## Original goal

Implement `PLAN.md` task 2: document CRUD and revisions.

## Final status

Completed.

## Implementation summary

- Added optimistic concurrency with `base_revision_id` and `If-Match`.
- Implemented document update, patch, soft delete, revision listing, revision read, and revision restore.
- Added revision writes for create, update, delete, and restore.
- Updated FTS5 rows transactionally as documents are changed, deleted, or restored.
- Updated web UI save behavior to create a new revision when editing an opened note.
- Updated API, schema, status, context, and handoff documentation.

## Validation evidence

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

## Model/session notes

Implemented by GPT-5.5 Thinking in the scaffold continuation session.

## Follow-up tasks

- Implement `PLAN.md` task 4 resource store.
- Later add trash browsing, permanent deletion, revision diffs, and AST-aware patching.
