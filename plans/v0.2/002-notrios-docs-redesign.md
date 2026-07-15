# v0.2 Task R1 — Notrios documentation redesign and rebrand pass

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5), Claude Code on the user's local machine.

## Scope

Docs-only slice implementing the redesign kickoff (`notrios_claude_kickoff` prompt): rebrand to Notrios, generalize the Codex handoff to all coding agents, replace sist2 with Recoll in the design, introduce the notebooks/search-notebooks/query-language/docs-site design docs, and replace `PLAN.md` with the R1–R16 redesign plan.

## Changes

- Git initialized: baseline commit on `main`, work on `develop`.
- `PLAN.md` → v0.2 Notrios redesign plan (R1–R16); former media-hardening draft re-sequenced to the v0.3 roadmap milestone.
- Renames: `CODEX_HANDOFF.md` → `CODING_CLIENT_HANDOFF.md`, `skills/codex-handoff` → `skills/agent-handoff`, `prompts/start_codex_from_handoff.md` → `prompts/start_agent_from_handoff.md`; added `CLAUDE.md` → `AGENTS.md` pointer.
- New design docs: `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `RECOLL_INTEGRATION.md`, `DOCS_SITE.md`.
- Updated: `README.md`, `AGENTS.md`, `ROADMAP.md`, `SYSTEM_ARCHITECTURE.md`, `API_SPEC.md`, `DATABASE_SCHEMA.md`, `UI_DESIGN.md`, `FEATURE_MATRIX.md`, `IMPORT_EXPORT_POLICY.md`, `CONTEXT_MAP.md`, `PROMPT.md`, `LICENSE_PENDING.md`, `ENVIRONMENT_SETUP.md`, `PACKAGING.md`, `PROJECT_DECISIONS.md`, `VERSIONING_AND_SYNC_POLICY.md`, `scripts/check_required_files.py`, agent status files.
- Historical reports intentionally keep old names as records.

## Validation

`go test ./...`, `python3 scripts/check_required_files.py`, and `bash scripts/validate-scaffold.sh` passed (docs-only change).

Commit: "R1: Notrios redesign plan and documentation rebrand pass".
