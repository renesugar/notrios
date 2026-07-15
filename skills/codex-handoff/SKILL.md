# Codex Handoff Skill

Use this skill when preparing the repository for Codex or another coding agent to continue work from files alone.

## Inputs

- `AGENTS.md`
- `PLAN.md`
- `ROADMAP.md`
- `agent/PLAN_STATUS.md`
- `agent/ATTEMPT_LOG.jsonl`
- `agent/MODEL_LOG.jsonl`
- Latest scaffold or task report

## Steps

1. Summarize the current working state in a root handoff document.
2. Identify the next incomplete task and a narrow first implementation slice.
3. Record validation commands and the last known passing validation state.
4. List environment limitations that may have affected implementation choices.
5. List early cleanup tasks for a less restricted environment.
6. Update `PROMPT.md`, `AGENTS.md`, and `agent/PLAN_STATUS.md`.
7. Archive the completed plan slice under `plans/v<major>.<minor>/`.
8. Produce a ZIP snapshot.

## Working-state rules

- Do not depend on chat history for essential instructions.
- Keep next steps concrete and small.
- Preserve loop-detection and model-tracking instructions.
- Do not hide limitations; record them in the repository.
