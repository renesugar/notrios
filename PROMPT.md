# Initial Agent Prompt

You are working in the Notrios repository (formerly Notes Companion). Continue from the repository files only; do not rely on prior chat history. This prompt applies to any coding agent (Codex, Claude, aider, etc.).

Read these files first:

1. `AGENTS.md`
2. `CODING_CLIENT_HANDOFF.md`
3. `README.md`
4. `PLAN.md`
5. `ROADMAP.md`
6. `SYSTEM_ARCHITECTURE.md`
7. `API_SPEC.md`
8. `DATABASE_SCHEMA.md`
9. `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `RECOLL_INTEGRATION.md`
10. `TESTING_POLICY.md`
11. `SYNCHRONIZATION.md`
12. `FLUTTER_GO_CLIENT.md`
13. `CONTEXT_MAP.md`
14. `agent/PLAN_STATUS.md`
15. `agent/ATTEMPT_LOG.jsonl`
16. `agent/MODEL_LOG.jsonl`
17. `agent/OPEN_QUESTIONS.md`

Then:

1. Identify the next incomplete task in `PLAN.md`.
2. Record the session/model in `agent/MODEL_LOG.jsonl` if possible.
3. Append a `started` entry to `agent/ATTEMPT_LOG.jsonl`.
4. Implement only one coherent task slice.
5. Add/update tests.
6. Run validation.
7. Update `agent/PLAN_STATUS.md`.
8. Append a `completed` or `blocked` entry to `agent/ATTEMPT_LOG.jsonl`.
9. Commit the working-state slice with git; if the plan step is complete,
   archive it under `plans/v<major>.<minor>/`, create and verify the requested
   evidence ZIP, and ask before starting the next task.

Important constraints:

- Leave the repository in a working state.
- SQLite is canonical storage; Recoll is an optional derived sidecar, never canonical.
- Keep code and dependencies MIT/Apache-2.0 compatible; GPL tools are external processes only.
- Do not expose raw SQL or arbitrary filesystem operations to MCP.
- Treat imported Markdown/resources as untrusted.
- Keep code changes small and reviewable.
- Review `CODING_CLIENT_HANDOFF.md` before replacing the temporary SQLite adapter or adding external dependencies.
