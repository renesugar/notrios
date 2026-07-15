# Initial Codex Prompt

You are working in the Notes Companion repository. Continue from the repository files only; do not rely on prior chat history.

Read these files first:

1. `AGENTS.md`
2. `CODEX_HANDOFF.md`
3. `README.md`
4. `PLAN.md`
5. `ROADMAP.md`
6. `SCAFFOLD_STEP6_REPORT.md`
7. `SYSTEM_ARCHITECTURE.md`
8. `API_SPEC.md`
9. `DATABASE_SCHEMA.md`
10. `TESTING_POLICY.md`
11. `CONTEXT_MAP.md`
12. `agent/PLAN_STATUS.md`
13. `agent/ATTEMPT_LOG.jsonl`
14. `agent/MODEL_LOG.jsonl`
15. `agent/OPEN_QUESTIONS.md`

Then:

1. Identify the next incomplete task in `PLAN.md`.
2. Record the session/model in `agent/MODEL_LOG.jsonl` if possible.
3. Append a `started` entry to `agent/ATTEMPT_LOG.jsonl`.
4. Implement only one coherent task slice.
5. Add/update tests.
6. Run validation.
7. Update `agent/PLAN_STATUS.md`.
8. Append a `completed` or `blocked` entry to `agent/ATTEMPT_LOG.jsonl`.
9. If the plan step is complete, archive it under `plans/v0.1/` and ask before starting the next task.

Recommended first implementation slice after this scaffold:

```text
Complete PLAN.md task 1: Service persistence foundation.
```

Start with configuration loading, data/resource directory creation, and richer status reporting. Preserve the existing document create/read/search REST behavior and the Step 5 UI workflow.

Important constraints:

- Leave the repository in a working state.
- Do not use sist2 as canonical storage.
- Do not expose raw SQL or arbitrary filesystem operations to MCP.
- Treat imported Markdown/resources as untrusted.
- Keep code changes small and reviewable.
- Review `CODEX_HANDOFF.md` before replacing the temporary SQLite adapter or adding external dependencies.
