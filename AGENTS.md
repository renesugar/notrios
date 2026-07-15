# AGENTS.md

This file provides project instructions for all coding agents (Codex, Claude, aider, swival.dev, etc.). AGENTS.md is intended as a predictable place for agent guidance, similar to a README for agents. `CLAUDE.md` points here.

## Prime directive

Leave the repository in a working state after every task. If interrupted, the next agent should be able to resume using only repository files.

## Before coding

Start with the handoff: `CODING_CLIENT_HANDOFF.md` is the current compressed state of the project and the recommended next task.


1. Read `CODING_CLIENT_HANDOFF.md`, `README.md`, `PLAN.md`, `ROADMAP.md`, `SYSTEM_ARCHITECTURE.md`, `API_SPEC.md`, `DATABASE_SCHEMA.md`, `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `RECOLL_INTEGRATION.md`, `CODING_STANDARDS.md`, `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`, and `CONTEXT_MAP.md`.
2. Read `agent/PLAN_STATUS.md`, `agent/ATTEMPT_LOG.jsonl`, and `agent/MODEL_LOG.jsonl`.
3. Identify the next incomplete task in `PLAN.md`.
4. If the task is ambiguous, write the question into `agent/OPEN_QUESTIONS.md` and ask the user before implementing.

## During each task

- Work in the smallest coherent slice that can be validated.
- Add or update tests with the change.
- Run the relevant validation commands.
- Do not rewrite large areas without preserving working behavior.
- Do not introduce raw SQL or unrestricted filesystem access in MCP tools.
- Do not make Recoll (or any search sidecar) the canonical document store; Recoll is a derived, optional extraction/search sidecar (see `RECOLL_INTEGRATION.md`).
- Keep all project code and dependencies compatible with an MIT or Apache-2.0 license. GPL tools such as Recoll/Xapian may only be invoked as external processes — never linked, vendored, or redistributed, and never used as a source to derive code from.
- Do not put secrets, medical data, private user exports, or proprietary datasets in the repository.


## Handoff and environment limitations

The scaffold was created in a restricted container. Before large implementation work, read `CODING_CLIENT_HANDOFF.md` and review the documented limitations. In particular:

- the current SQLite adapter is a small local cgo wrapper chosen to avoid external Go module downloads during scaffold creation;
- the Go module path is `github.com/renesugar/notrios` and the license is Apache-2.0 (both set in plan task R2);
- Recoll, Quartz, MCP SDK, and browser automation are documented but not integrated;
- real private import datasets were intentionally not included;
- UI validation has been build/typecheck level only, not Playwright-level browser testing.

In a less restricted environment, consider resolving the SQLite driver, npm dependency audit, MCP SDK pin, and browser test stack before deep feature work. Do not make these changes all at once unless the active plan calls for a cleanup slice.

## Progress and loop detection

For every attempt, append one JSON object to `agent/ATTEMPT_LOG.jsonl`:

```json
{"timestamp":"2026-07-08T00:00:00Z","task_id":"mvp-01","task":"Service persistence foundation","model":"unknown","status":"started","notes":"..."}
```

On completion, append another object with `status: "completed"` or `status: "blocked"`.

If a task has three blocked attempts with no meaningful file changes or no passing validation improvement:

1. Stop broad attempts.
2. Split the task into smaller subtasks in `PLAN.md`.
3. Add a loop-detection note to `agent/LOOP_DETECTION.md`.
4. Ask the user to review the obstacle.

## Model tracking

At the start of a session, record the model if known in `agent/MODEL_LOG.jsonl`. If the model appears downgraded or performance changes abruptly, note it. Do not assume the model is stable across sessions.

## Plan archival

Implemented plans must be archived under:

```text
plans/v<major>.<minor>/<NNN>-<slug>.md
```

Use the active product version/milestone. Include validation evidence and the models used. After the current `PLAN.md` completes, create a new `PLAN.md` from `ROADMAP.md` and ask the user before starting it.

## Git workflow

- Work on `develop` until MVP is ready.
- Use small commits aligned with completed working-state slices.
- Do not commit generated build artifacts, private data, or local databases.
- Before committing, run `go test ./...` and any task-specific tests.

## Skills

Reusable workflows live under `skills/<skill-name>/SKILL.md`. If you discover a repeatable process, add or update a skill after the task succeeds. Keep skills short and progressively disclosed.

## Security constraints

- Treat imported notes and downloaded resources as untrusted.
- Remote-media localization must go through domain policy, quarantine, exact hashes, perceptual-hash policy hooks, MIME sniffing, size limits, and SSRF protections.
- Preview HTML must be sanitized.
- MCP tools should default to read-only and context-limited outputs.

## User interaction

When a plan step completes, produce a ZIP snapshot and ask whether to proceed to the next step.

## Feature triage

When a requested feature is not already in the active `PLAN.md`, consult `FEATURE_MATRIX.md` and `ROADMAP.md` before coding. If the feature is not MVP, document it or create a small planning slice; do not silently expand the MVP.

Use `prompts/review_feature_against_roadmap.md` for features that may alter milestone scope.

## Release packaging rule

When producing a ZIP for user handoff or repository import, use `scripts/package_release.sh` and verify with `scripts/check_release_zip.py`. Do not hand off a ZIP that lacks `web/dist/`, and do not include `web/node_modules/`, runtime `data/`, `.git/`, or SQLite database files. See `skills/release-packaging/SKILL.md`.
