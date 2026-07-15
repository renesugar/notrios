# Prompt: Start Codex From Handoff

You are continuing the Notes Companion project from a repository snapshot.

Read these files first:

1. `AGENTS.md`
2. `CODEX_HANDOFF.md`
3. `README.md`
4. `PLAN.md`
5. `ROADMAP.md`
6. `SCAFFOLD_STEP6_REPORT.md`
7. `agent/PLAN_STATUS.md`
8. `agent/ATTEMPT_LOG.jsonl`
9. `agent/MODEL_LOG.jsonl`
10. `agent/OPEN_QUESTIONS.md`

Then do the following:

1. Identify the next incomplete task in `PLAN.md`.
2. Record the current model/session in `agent/MODEL_LOG.jsonl` if known.
3. Append a `started` entry to `agent/ATTEMPT_LOG.jsonl`.
4. Implement one narrow working-state slice only.
5. Run validation.
6. Update status files and archive the completed plan slice.
7. Stop and ask before starting the next task.

Preserve these constraints:

- SQLite is canonical managed-note storage.
- sist2 is a derived sidecar.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown/resources are untrusted.
- Remote media must go through policy and quarantine.
- Preview HTML must be sanitized.
- Leave the repository in a working state after every task.
