---
name: project-planning
description: Maintain incremental plans, attempt logs, model logs, and archived plan history for Codex-safe work.
---

# Project Planning Skill

Use this skill when the active task matches the description.

## Steps

1. Read `PLAN.md`, `ROADMAP.md`, and `agent/PLAN_STATUS.md`.
2. Select one coherent task slice.
3. Append attempt start/end records to `agent/ATTEMPT_LOG.jsonl`.
4. Update `agent/PLAN_STATUS.md`.
5. Archive completed plans under `plans/v<major>.<minor>/`.
6. If stalled, update `agent/LOOP_DETECTION.md` and split the task.

## Working-state checks

- `python3 scripts/check_plan_loops.py`
- `python3 scripts/check_required_files.py`
