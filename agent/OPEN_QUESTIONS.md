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
