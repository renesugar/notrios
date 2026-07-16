# Built-in GUI Design

The built-in GUI is a **Go + Wails** desktop application (https://github.com/wailsapp/wails) named `notrios` (implemented in task R13; built with `make gui`, i.e. `-tags "gui desktop production webkit2_41"`; plain builds include a stub so headless/CI builds need no GUI system libraries), shipped in the first released version. It is not the only client: the REST/MCP API must stay complete enough for third-party native clients (C++/Qt, Rust/Tauri, other Go/Wails apps).

## Executable modes

One executable contains the GUI and the service:

- **default** — start the local service and open the GUI on it.
- **`-no-gui`** — start the service headless, for users who prefer a different client as their GUI.
- **`-gui-only`** — start only the GUI as a pure REST client. This tests the GUI exactly the way a third-party client would use the service, and lets the GUI attach to a service running on another machine via the REST API.

`notriosd` remains the standalone headless service binary. Both binaries share `internal/service`.

Implementation note: the Wails asset server routes **every** webview request through an `http.Handler` — the in-process service handler in the default mode, or a reverse proxy to the remote service in `-gui-only` mode — so the React frontend runs unmodified with relative `/api/v1` fetches, identical to a browser pointed at `notriosd`. In the default mode the service also listens on its TCP address so MCP clients and third-party GUIs can connect while the built-in GUI is open.

## Window layout

Standard desktop menu bar at the top (File, Edit, View, Help, …), then four regions:

```text
┌────────────┬───────────────┬───────────────┬───────────────┐
│ sidebar    │ search box    │ markdown      │ markdown      │
│            │ + results     │ editor        │ preview       │
│ Notebooks  │               │               │               │
│  📥 All    │ (incremental/ │               │               │
│   notes    │  cursor-paged │               │               │
│  Notes     │  scrolling)   │               │               │
│  Bookmarks │               │               │               │
│  Twitter   │               │               │               │
│  …         │               │               │               │
│  🗑 Trash   │               │               │               │
│ Tags       │               │               │               │
│  tag (n)   │               │               │               │
└────────────┴───────────────┴───────────────┴───────────────┘
```

- **Left sidebar:** notebooks tree above, tags (with note counts) below. Builtin "All notes" search notebook is always first; "Trash" is always last; neither is deletable. Notebooks show an optional emoji icon before their name. Notebooks nest like Joplin (e.g. `Contacts` → `Plumbers`, `Electricians`, `Carpenters`) so the experience is smooth for Joplin users. See `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`.
- **First panel:** search box and query results. On startup the "All notes" search runs; the search API returns incremental results as the user scrolls, so startup never retrieves hundreds of thousands of notes at once.
- **Next two panels:** Markdown editor and Markdown preview.

## Themes (implemented, task R14)

- A light/dark toggle (🌙/☀️) sits on the main window header; the mode persists.
- Themes are named sets of CSS custom properties (`web/src/themes.ts`); every color in `styles.css` reads a token, and the editor/preview follows the theme's light/dark base.
- Users can create custom themes (cloned from the current theme, per-token color editing, deletable) and select any theme — builtin or custom — as the one used for light mode and for dark mode; the toggle then switches between those two selections.
- Themes and selections persist in localStorage, so they behave identically in the Wails webview and a browser.

## Frontend implementation

The existing React frontend is the basis of the Wails webview UI, currently using `md-editor-rt` behind an application-owned adapter; a later migration to `CodeMirror 6 + unified/remark/rehype` is reserved for deeper source-position and editor-pane behavior (Ctrl-click in the editor pane, broken-link markers while typing, inline resource widgets, AST-safe edits, rich link autocomplete).

## Required behavior (carried over from the web-UI MVP)

- Create, edit, save, and search Markdown notes.
- Intercept preview links: `document://…` opens the target note; `resource://…` opens/downloads through REST (`GET /api/v1/resources/{id}/content?download=1`); never expose raw storage paths.
- Upload/paste images and attach PDFs/resources through REST.
- Show outgoing links, backlinks, unresolved links, and resources for the active note.
- Show revision/conflict state using revision IDs or ETags.
- Sanitize preview HTML.
- Remote images in preview may show a warning/action but must never be silently localized; localization is a server operation under media policy.
