# Initial Agent Prompt

You are working in the Notrios repository (formerly Notes Companion). Continue from the repository files only; do not rely on prior chat history. This prompt applies to any coding agent (Codex, Claude, aider, etc.).

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

Read `AGENTS.md` first, and follow the reading list in its "Before coding"
section. That list is not repeated here: this file carried its own copy for
months, and the two had already drifted -- one named `agent/OPEN_QUESTIONS.md`
and the other named `CODING_STANDARDS.md` and `ENVIRONMENT_SETUP.md`, and
neither reader could tell which was short.

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

Important constraints are in `AGENTS.md` under "During each task" and
"Security constraints", including the license boundary around GPL tools, the
untrusted-input rule, and what may never enter the repository.
