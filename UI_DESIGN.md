# Built-in GUI Design

The built-in GUI is a **Go + Wails v2** desktop application
(https://github.com/wailsapp/wails) named `notrios` (implemented in task R13;
built with `make gui`, i.e. `-tags "gui desktop production webkit2_41"`; plain
builds include a stub so headless/CI builds need no GUI system libraries),
shipped in the first released version. It is not the only client: the REST/MCP
API must stay complete enough for third-party native clients.

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
│  Help      │               │               │               │
│  🗑 Trash   │               │               │               │
│ Tags       │               │               │               │
│  tag (n)   │               │               │               │
└────────────┴───────────────┴───────────────┴───────────────┘
```

- **Left sidebar:** notebooks tree above, tags (with note counts) below. Builtin "All notes" search notebook is always first; "Trash" is always last, with the builtin "Help" notebook immediately above it; none of these is deletable. Ordering keys off the stable builtin IDs (`snb_all_notes`, `nb_help`, `snb_trash`), never off display names. Notebooks show an optional emoji icon before their name. Notebooks nest like Joplin (e.g. `Contacts` → `Plumbers`, `Electricians`, `Carpenters`) so the experience is smooth for Joplin users. See `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`.
- **First panel:** search box and query results. On startup the "All notes" search runs; the search API returns incremental results as the user scrolls (cursor paging via an IntersectionObserver sentinel, with a keyboard-accessible "Load more" fallback), so startup never retrieves hundreds of thousands of notes at once.
- **Next two panels:** Markdown editor and Markdown preview. Note details (metadata, uploads, links/backlinks, resources) are a collapsible inspector inside the editor pane, not a fifth region.

Implementation notes (task R13 GUI-fix pass):

- The three pane boundaries are draggable splitters (Pointer Events) that are also keyboard-operable (`role="separator"`, Arrow / Shift+Arrow steps, Home/End, double-click resets). Widths persist in `localStorage` under a versioned key; invalid stored values fall back to defaults.
- The four panes fill the window and scroll individually; the document body never scrolls. On a **window resize** the sidebar and search widths are kept and the space after them is re-split **equally** between the editor and the preview (they always come out the same width and height after a resize); splitter drags may then set individual sizes. Minimum pane widths give a minimum workspace width of ~966 px, below which every pane holds its minimum and the workspace scrolls horizontally.
- Whether a note is editable is a **server-provided capability** (`editable` on documents and search hits — false for Help-notebook and trashed notes), never inferred from names in the client. Read-only notes render a visible badge, a read-only editor, and no save/title/upload affordances, while the preview still works.

Trash-first deletion (implemented in v0.5 E8):

- The editor toolbar offers **Move to Trash** on an editable note. It asks
  first, and the question says what happens ("it stays in the Trash until you
  restore it") rather than "are you sure". The delete carries the revision the
  note was opened at, so a note edited elsewhere fails the precondition instead
  of being deleted out from under the other writer.
- A trashed note opens with an **In the Trash** badge and two offers: **Restore**
  and **Delete forever**. It does not share the Help note's "read-only" badge —
  both are uneditable, but only one can be brought back, and calling a trashed
  note read-only would hide the one thing its reader can act on.
- Deleting a note or restoring one updates the results list in place rather than
  re-running the search, so the pane keeps its contents and scroll position.
- Sidebar notebook rows carry a delete affordance, revealed on hover or focus,
  and only where the service would allow it (never builtin or default
  notebooks). Clicking it asks
  `GET /api/v1/notebooks/{id}/deletion-preview` first and confirms with **that**
  answer: how many notebooks go, that the notes are not deleted, and which
  notebook they are re-homed to so a later restore has a destination. The
  re-homing rule is a store rule the GUI surfaces, not one it invents.
- A new note is created into the **selected notebook**, and the target is shown
  before it is saved; selecting a search notebook (All notes, Trash, a saved
  search), or selecting nothing, falls back to the default "Notes" notebook. The
  selection is tracked by notebook **ID**, never derived from the sidebar row's
  `notebook:"<name>"` query — notebook names are unique only among siblings, so
  two notebooks under different parents can share a name and a query. A
  notebook dropdown in the **editor's own toolbar** is both a second visual cue
  and the correction: it shows the open note's notebook and changing it moves
  the note. It lives in that toolbar rather than in a row of its own because
  `md-editor-rt` accepts custom toolbar items (`defToolbars` plus a numeric
  entry in `toolbars`, with an exported `DropdownToolbar`) and its toolbar
  scrolls horizontally instead of wrapping — a separate control above it would
  reintroduce exactly the wrapping this section otherwise forbids. Help is never
  an available destination, and for a read-only or trashed note the dropdown is
  disabled rather than hidden so the note's notebook stays visible. Moving
  several notes at once is a batch operation and belongs with the rest of them.
  (Planned as v0.6 F0; before it, the GUI created every note in "Notes" and
  offered no way to move one.)
- The editor toolbar holds **only actions that apply to the open note** — save,
  move to Trash, restore, delete forever. Starting a *new* note is not one of
  them: it discards the editor's contents rather than acting on the note, and
  putting it among per-note controls is what made it appear in a read-only Help
  or Trash toolbar as an apparent offer to create something there. It belongs in
  the search pane, which is the list context and is present whatever is open —
  and it has to stay reachable there, because it is the only path back to a
  blank draft. (Planned as v0.5 E10 part 2.)
- The editor toolbar is a **title row plus an action row**, not one wrapping
  line. The title occupies its own row; the state chip and the note's actions
  sit beneath it on a single row, and when the pane is too narrow for that row
  they stack vertically **together** rather than wrapping one item at a time.
  The trigger is the *pane's* width, not the window's — the panes are
  splitter-resized independently, so a viewport media query would measure the
  wrong box. (Planned as v0.5 E10; before it, a trashed note's toolbar went
  ragged at every supported width and the chip never left the title's row.)
- Opening a trashed note works because `GET /api/v1/documents/{id}` returns one,
  with `deleted_at` set and `editable: false`. Before E8 it returned 404, which
  made the Trash unusable and left the `trashed` branch in stable-link routing
  dead. The remote-media scan is skipped for a trashed note: localization writes
  a revision it cannot take.
- Tag rename has no GUI surface; v0.5 E8 scoped it to Store/REST/CLI.

## Themes (implemented, task R14)

- A light/dark toggle (🌙/☀️) sits on the main window header; the mode persists.
- Themes are named sets of CSS custom properties (`web/src/themes.ts`); every color in `styles.css` reads a token, and the editor/preview follows the theme's light/dark base.
- Users can create custom themes (cloned from the current theme, per-token color editing, deletable) and select any theme — builtin or custom — as the one used for light mode and for dark mode; the toggle then switches between those two selections.
- Themes and selections persist in localStorage, so they behave identically in the Wails webview and a browser.

## Frontend implementation

The existing React frontend is the basis of the Wails webview UI, using `md-editor-rt` behind an application-owned adapter. A migration to `CodeMirror 6 + unified/remark/rehype` was weighed in v0.5 E6 and declined.

**There is no migration to make: `md-editor-rt` is CodeMirror 6.** v0.5 E5
recorded that the editor exposed no caret position and accepted no inline
widgets. That was wrong, and E6 corrected it. `md-editor-rt` 6.5.3 depends on
`@codemirror/{view,state,autocomplete,commands,language,search}` 6.x and exposes
them — `completions` feeds `@codemirror/autocomplete`, `codeMirrorExtensions`
accepts arbitrary extensions, `getEditorView()` returns the `EditorView`, and
`domEventHandlers` is CodeMirror's own handler map.

E6 therefore implemented the features rather than planning a migration to reach
them: `[[` autocomplete inside the editor, wavy underlines on broken links that
move with their text, and Ctrl-click to open a target. They cost 1.3 kB gzipped
and nothing measurable in typing latency, because the library was already in the
bundle. See `PROJECT_DECISIONS.md` 20.

The adapter still earns its place — it is what made that decision cheap to
reach. The remaining argument for owning the editor outright is
`@codemirror/language-data`, which md-editor-rt pulls in for code-block
highlighting and which contributes 113 lazy chunks totalling 1.32 MB. Those load
only when a fenced block names their language, so they cost distribution size
rather than first paint.

## What the preview renders today (verified 2026-08-06)

Recorded because the `remark`/`rehype` dependencies that used to be declared
here invited the wrong inference. They were unused and E6a removed them:
`md-editor-rt` renders through **markdown-it**, not unified.

Checked in a real browser against a note containing each case:

| Input | Result |
|---|---|
| `[text](document://…)` | `<a data-app-uri="document://…" href="#">`, click intercepted and routed |
| `[text](https://…)` | `<a target="_blank" rel="noreferrer">` |
| Markdown pipe table | rendered as a `<table>` |
| Pasted raw `<table>` HTML | rendered as a `<table>` — raw HTML is enabled. Since v0.5 E6b a simple pasted table is converted to a Markdown table in the source instead, so it is only raw HTML when the converter refused it. |
| `<script>`, `onerror=`, `style=` | removed |

Sanitization is Notrios' own `normalizePreviewHTML` (DOMParser-based), passed to
`md-editor-rt` as its `sanitize` prop, on top of that library's built-in `xss`.
`rehype-sanitize` is not involved and could not be: it belongs to a unified
pipeline this application does not run.

**The frontend is offline-capable as of v0.5 E6a.** It had not been: KaTeX,
highlight.js, echarts, cropperjs, and prettier were fetched from `unpkg.com` at
runtime — 13 requests and 623 kB on every launch — and math silently rendered as
raw LaTeX without a network. Those are bundled or disabled now
(`web/src/editor-assets.ts`), and the service serves a Content-Security-Policy
with `script-src 'self'` so a future dependency cannot reintroduce the problem
quietly. The rule is: anything the editor would fetch is either bundled or
turned off.

## Required behavior (carried over from the web-UI MVP)

- Create, edit, save, and search Markdown notes.
- Intercept preview links: `document://…` opens the target note; `resource://…` opens/downloads through REST (`GET /api/v1/resources/{id}/content?download=1`); never expose raw storage paths.
- Upload/paste images and attach PDFs/resources through REST.
- Show outgoing links, backlinks, unresolved links, and resources for the active note.
- Show revision/conflict state using revision IDs or ETags.
- Sanitize preview HTML.
- Remote images in preview may show a warning/action but must never be silently localized; localization is a server operation under media policy.
- Multi-select organizer actions (planned v0.6): move, duplicate, trash,
  tag/untag, and copy stable Markdown links. The client calls the bounded batch
  API and displays per-item/atomic outcomes; query-scoped export is not required
  merely to organize a selection.
- External `notrios://` links (implemented in v0.4 P5) are routed, never
  followed: the preview hands the URI to `POST /api/v1/links/resolve`, opens the
  note when this database owns it, and otherwise says the link belongs to
  another database or names a note that no longer exists. The desktop handler
  resolves the database through the local registry and opens the local UI at
  `#document=<id>`. Multi-profile switching inside one window remains future
  work; ambiguity is never guessed.
- Sync UI (planned v0.7) exposes target `none`, REST/folder/rclone job status,
  pending/corrupt objects, behind/retired peers, body conflicts, and
  notebook-tree repairs.

## Mobile investigation

Wails v3 documents reuse of one `main.go` and frontend on desktop, iOS, and
Android, but v3 is pre-release and mobile support experimental. Keep Wails v2
as the release shell until an approved migration spike passes desktop
regression plus real Android validation. The UI will need a mobile layout (not
four simultaneous panes), safe-area handling, lifecycle/background transfer,
sandboxed file picking/export, touch targets, and memory tests. Sync/library
interfaces must remain UI-framework independent so mobile work can proceed
without redesigning the protocol.
