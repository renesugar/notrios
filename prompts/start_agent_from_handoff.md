# Prompt: Start a Coding Agent From Handoff

You are continuing the Notrios project from a repository snapshot. This prompt applies to any coding agent (Codex, Claude, aider, etc.).

Read these files first:

1. `AGENTS.md`
2. `CODING_CLIENT_HANDOFF.md`
3. `README.md`
4. `PLAN.md`
5. `ROADMAP.md`
6. `agent/PLAN_STATUS.md`
7. `agent/ATTEMPT_LOG.jsonl`
8. `agent/MODEL_LOG.jsonl`
9. `agent/OPEN_QUESTIONS.md`

Then do the following:

1. Identify the next incomplete task in `PLAN.md`.
2. Record the current model/session in `agent/MODEL_LOG.jsonl` if known.
3. Append a `started` entry to `agent/ATTEMPT_LOG.jsonl`.
4. Implement one narrow working-state slice only.
5. Run validation.
6. Update status files, commit with git, and archive the completed plan slice.
7. Stop and ask before starting the next task.

Preserve these constraints:

- SQLite is canonical managed-note storage.
- Recoll is a derived, optional, external-process sidecar (GPL — never linked or vendored).
- Project code stays MIT/Apache-2.0 compatible.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown/resources are untrusted.
- Remote media must go through policy and quarantine.
- Preview HTML must be sanitized.
- Leave the repository in a working state after every task.
