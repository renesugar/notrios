# Scaffold Review Report — Step 2

## Scope

This review repaired the Step 1 scaffold against the copied conversation context. No product feature implementation was attempted; the goal was to make the scaffold more complete and easier for Codex to continue from.

## Findings

The Step 1 scaffold already captured the core architecture:

- Go companion REST/MCP service.
- SQLite/FTS5 as canonical storage and immediate search.
- sist2 as a derived sidecar, not source of truth.
- React/Vite built-in UI with `md-editor-rt` initially.
- Joplin RAW and Obsidian importers.
- Media policy, remote-media localization, and plan-loop tracking.

Missing or under-specified areas were promoted into first-class documents:

- Feature-to-version ownership matrix.
- Built-in UI/editor decision detail.
- Quartz publishing policy and privacy dry-run rules.
- Versioning/sync policy for SQLite revisions, go-git, and Fossil.
- Foam-style workspace maintenance and query/dashboard features.

## Files added

- `FEATURE_MATRIX.md`
- `UI_DESIGN.md`
- `PUBLISHING_POLICY.md`
- `VERSIONING_AND_SYNC_POLICY.md`
- `WORKSPACE_MAINTENANCE.md`
- `skills/quartz-publishing/SKILL.md`
- `skills/ui-editor-preview/SKILL.md`
- `skills/workspace-maintenance/SKILL.md`
- `prompts/review_feature_against_roadmap.md`
- `plans/v0.1/002-scaffold-review-repair.md`

## Files updated

- `README.md`
- `AGENTS.md`
- `PLAN.md`
- `ROADMAP.md`
- `SYSTEM_ARCHITECTURE.md`
- `API_SPEC.md`
- `SECURITY_AND_MEDIA_POLICY.md`
- `IMPORT_EXPORT_POLICY.md`
- `CONTEXT_MAP.md`
- `SCAFFOLD_CREATION_PLAN.md`
- `agent/PLAN_STATUS.md`
- `agent/ATTEMPT_LOG.jsonl`
- `agent/MODEL_LOG.jsonl`
- `scripts/check_required_files.py`

## Validation

Run after Step 2 changes:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
```

## Next step

Proceed to Scaffold Creation Plan Step 3: expand API and schema detail. That step should make `API_SPEC.md`, `api/openapi.yaml`, and `migrations/0001_initial.sql` precise enough for Codex to implement the first persistence slice.
