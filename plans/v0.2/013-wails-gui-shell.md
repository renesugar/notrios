# v0.2 Task R13 — Wails GUI shell

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5). Wails v2.13, libgtk-3-dev, and webkit2gtk-4.1 were present on the dev machine; the real GUI was built and launched under xvfb (window opened, embedded service answered /healthz).

## Changes

- `internal/service`: shared startup extracted from notriosd (directories → store → handler → Recoll sidecar loop with clean shutdown); notriosd refactored onto it.
- `cmd/notrios`: one executable containing GUI and service.
  - default: starts the local service (also listening on its TCP address for MCP/third-party clients) and opens the GUI on the in-process handler;
  - `-no-gui`: headless service, same behavior as notriosd;
  - `-gui-only [-remote http://host:port]`: GUI as a pure REST client via a reverse proxy — tests the GUI exactly as a third-party client would and supports remote services.
- Wails integration: the asset server's `Handler` receives every webview request, so the unmodified React frontend (relative `/api/v1` fetches) works identically in the webview and a browser. Menu bar: File (Reload/Quit), Edit (roles), View (fullscreen), Help (dispatches a `notebook:help` search event to the frontend).
- Build tags: `gui desktop production webkit2_41` compile the webview (`make gui`); plain `go build ./...` uses a stub that keeps CI/headless builds free of GUI system dependencies and prints how to get the GUI. go.mod gains wails v2 (compiled only under the tags).
- React UI layout per `UI_DESIGN.md`: left sidebar with search notebooks ("All notes" always first, "Trash" always last), the nested notebook tree with emoji icons and indentation, and the tag list with live counts below; clicking runs the corresponding query-language search (`notebook:"..."` / `tag:"..."` / saved queries). Startup runs the "All notes" view with limit 25 and a cursor-based "Load more" button. Sidebar refreshes after saves.

## Validation

`go test ./...`, `go vet` (plain + gui tag builds), scaffold checks, `mvp_smoke.sh`, web typecheck+build; live xvfb launch of the GUI binary with the embedded service verified. All passing.
