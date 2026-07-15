# Open Questions

1. Which license should the project use?
2. Should the first SQLite driver be Cgo-based or pure Go?
3. Which MCP Go SDK version should be pinned when MCP implementation begins?
4. Should the repo name and Go module be changed from `example.com/notes-companion` before first commit?

## Step 2 open questions

- Which final license should the repository use?
- Should the first implementation pin `md-editor-rt` to a known version before the web UI is implemented?
- Which SQLite driver should be selected for the Go service: Cgo-backed or pure Go?
- Should the MVP include a minimal Quartz dry-run planner, or defer all publishing to v0.4?


## Step 6 handoff questions

- Should Codex replace the temporary local cgo SQLite wrapper before implementing more persistence features, or keep it until the MVP slice is further along?
- What should the public repository/module path be?
- Which license should be selected before the first public GitHub push?
- Should configuration parsing use YAML with a dependency, JSON/TOML, or a small local parser for the first MVP slice?

## MVP release questions

- Has the user reviewed and approved the v0.1 MVP ZIP for first Gitea/GitHub commit?
- Should `PLAN.md` v0.2 start with policy config, or should Codex first replace the SQLite/MCP MVP adapters in a less restricted environment?
- Should UI bundle size be reduced before the first public release by pruning md-editor/highlight language imports or adding code splitting?

## Notrios redesign questions (2026-07-15)

1. **License (blocks R2):** MIT, Apache-2.0, or dual "MIT OR Apache-2.0"? All three satisfy the "compatible with MIT or Apache 2.0" requirement; Apache-2.0 adds a patent grant, MIT is simplest.
2. Should `notesctl` become `notriosctl` (assumed yes in `PLAN.md` R2), or keep its name?
3. When a regular notebook is deleted, its notes currently are specified to move to Trash (`NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`) — confirm, or should they move to the parent/default notebook?
4. Should FTS5 remain the always-on baseline with Recoll optional (current design), or should Recoll become required for full query-language support?
