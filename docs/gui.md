# The built-in GUI

The `notrios` desktop app is one binary containing the GUI and the service. It is built and tested on **Ubuntu Linux only**; macOS and Windows are not currently built or supported.

## Building

Exact build dependencies (the same packages CI installs for its GUI compile check):

```sh
sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev libsqlite3-dev build-essential pkg-config
make gui          # builds web/dist (installing frontend deps on first run) then bin/notrios
```

`make gui` compiles with the required build tags (`gui desktop production webkit2_41`); a plain `go build ./cmd/notrios` produces a binary whose GUI mode only prints instructions (the `-no-gui` service mode still works), which keeps CI and headless machines free of GUI libraries.

## Modes

```sh
./bin/notrios                 # GUI + local service
./bin/notrios -no-gui         # service only (use any client you like)
./bin/notrios -gui-only \
  -remote http://host:8080    # GUI against a service on another machine
```

`-gui-only` runs the GUI as a pure REST client — exactly the way a third-party client (C++/Qt, Rust/Tauri, another Wails app) would use the service. Without `-remote` it targets `http://<listen_addr>` from the configuration. Connecting across machines requires the remote service to listen on a reachable address — read the [network exposure notes](service.md#network-exposure) first (there is no authentication).

## First launch

Run from the repository checkout (the GUI loads its interface from `web/dist/` relative to the working directory — `make gui` builds it). In the default mode the embedded service also listens on `server.listen_addr` (default `127.0.0.1:8080`), so MCP clients and browsers can connect while the window is open. Configuration and flags are shared with the service: `-config`, `-addr`, `-db` — see [configuration](service.md#configuration-reference). Data lands wherever the config points (default `./data/` under the working directory).

On first launch you'll see the "All notes" view (empty until you create or
[import](import-export.md) notes) with the notebooks sidebar on the left. A
fresh database always bootstraps protected **All notes**, **Notes**, **Help**,
and **Trash** entries.

## Verifying the embedded service

```sh
curl http://127.0.0.1:8080/healthz            # -> ok
curl http://127.0.0.1:8080/api/v1/status | jq  # storage, schema, Recoll backlog/reconciliation
```

If the window opens but shows a JSON `web_ui_not_built` error, the binary can't find `web/dist/` in its working directory — run `make web` and launch from the checkout root.

## Common startup errors

| Symptom | Cause / fix |
|---|---|
| `Wails applications will not build without the correct build tags.` | the binary was built without the tags — use `make gui` |
| `this notrios binary was built without the GUI; rebuild with ...` | same, from the stub build; rebuild or run `-no-gui` |
| `package gtk+-3.0 was not found` / `webkit2gtk-4.1 was not found` at build time | install `libgtk-3-dev` / `libwebkit2gtk-4.1-dev` |
| `libEGL warning: DRI3 error` / renderer warnings at launch | harmless software-rendering fallback (common under VMs and Xvfb) |
| window opens with `web_ui_not_built` JSON | build `web/dist` (`make web`) and launch from the checkout |
| `service listener stopped: listen tcp ... address already in use` in the log | another notriosd/notrios owns the port; stop it or change `-addr` (the GUI itself keeps working against its in-process handler) |

## Layout

Below the menu bar the window is four side-by-side panes, always in this order:

1. **Sidebar** — notebooks and tags. The builtin **All notes** view is always at the top and **Trash** is always at the bottom, with **Help** immediately above Trash; between them are your notebooks (nested, with optional emoji icons) and your saved search notebooks. Below the notebooks is the tag list with live note counts. Click anything to search it.
2. **Search panel** — the search box plus results. Its inline hint covers
   uppercase OR, implicit AND, prefix `-`, grouping, and `category:`/
   `notebook:` fields; the complete grammar is in [Search query
   language](query-language.md). On startup the "All notes" view loads, and
   results page in incrementally as you scroll (a keyboard-accessible "Load
   more" button covers the same path), so even huge databases start instantly.
   Read-only notes show a 🔒 marker.
3. **Markdown editor** — title, toolbar, and source editor. Note details (metadata, uploads, links, backlinks, resources) live in a collapsible **Note info** inspector at the bottom of this pane, not in a pane of their own. When a note references remote images or media, the inspector also lists each URL with its server-side policy decision (allow/block/review) — a static scan; nothing is downloaded. For editable notes with allowed media, a **Localize allowed media** button downloads those files through the server's quarantine pipeline, stores them as local resources, and rewrites the note to `resource://` links in a new revision (blocked URLs are never fetched).
4. **Markdown preview** — rendered Markdown. `document://` links open the target note; `resource://` links download attachments; images can be pasted/uploaded in the editor and become local resources.

Each pane scrolls on its own; the window itself never scrolls.

### Link help while you write

The **Note info** inspector holds two link assists for editable notes.

**Insert link to note** searches your notes by title after two characters and
inserts a canonical `[Title](document://…)` link at the cursor. It inserts the
URI rather than the title on purpose: a link written as a title breaks the day
the note is renamed.

You can also stay in the text: typing `[[` followed by two or more characters
opens the same search inline, and choosing a note replaces the `[[` with the
finished link.

Below it, the links in what you have typed are checked a moment after you stop
typing — before you save, and without saving. Anything that would not open is
listed with its line, column, and the reason: no such target, more than one
match, or the note exists but the section inside it does not. A heading you just
typed counts immediately, because the check reads the text in front of you
rather than the last saved version.

Links that will not open are also underlined in red where they sit, and the
underline follows the text as you keep typing. **Ctrl-click** (⌘-click on macOS)
a link in the editor to open its target.

The list is kept alongside the underlines rather than replaced by them: an
underline only helps where you happen to be looking, while the list tells you
how many links are broken in the whole note.

All of it is quiet when the service cannot be reached: you get no suggestions,
no underlines, and no list — not an error interrupting your typing.

### Pasting a table

Paste an HTML table — from a spreadsheet, a web page, or a document — and it
becomes a Markdown table. That matters beyond tidiness: a table left as raw HTML
is invisible to link checking, to the note graph, and to anything you later
export the note to.

Tables a Markdown pipe table cannot represent are pasted exactly as they
arrived: merged cells, rows of differing length, a cell containing a list or a
nested table, a cell with more than one line, or a page excerpt that merely
contains a table. Nothing you paste is ever discarded — if it cannot be
converted faithfully, it is left alone.

### Live query blocks

A fenced `note-query` block turns into a live list of matching notes when the
preview renders:

````markdown
```note-query
query: tag:todo -tag:done
fields: notebook, updated
sort: updated
limit: 20
```
````

`query:` is the ordinary [search query language](query-language.md) — a block
can find exactly what you could type into the search box, and nothing more.
`fields:` picks from `title`, `notebook`, `tags`, `updated`, and `snippet`
(title is always shown), `sort:` is `updated` or `relevance`, and `limit:` caps
the list at 100. When more notes match than the limit shows, the block says so.

A block with a mistake in it shows the mistake in place — the rest of the note
renders normally, and the message names what to fix. Blocks are evaluated after
the note appears, so a slow query never delays reading.

Publishing a note keeps the block's text, not its results. A published note
therefore cannot leak the notes a query happened to match when you exported it.

### Math and code blocks

Write `$E = mc^2$` inline or `$$…$$` on its own lines and the preview renders it
with KaTeX. Fenced code blocks are syntax-highlighted for about forty common
languages; anything else renders as plain text.

Both work with no network at all. Everything the editor needs ships with the
application — nothing is fetched from a CDN at runtime, and the app makes no
third-party requests. Search indexes the LaTeX you typed rather than the
rendered formula, so searching `mc^2` finds the note.

### Resizing panes

Three splitters separate the panes. Drag one with the mouse, or focus it with **Tab** and press **←/→** (16 px), **Shift+←/→** (64 px), or **Home/End**; double-click restores the default layout. Pane sizes persist across restarts.

Panes always fill the window. When you resize the window, the sidebar and search panel keep their widths and the editor and preview re-split the remaining space **equally**; drag a splitter afterwards to make them unequal again. The layout is designed down to a window about 970 px wide — narrower than that, every pane sits at its minimum width and the workspace scrolls horizontally.

The menu bar offers File (Reload/Quit), Edit, View (fullscreen), and Help — Help searches the built-in **Help notebook**, which holds this documentation offline (`notebook:help` finds it too).

## Deleting and restoring notes

Deleting a note in Notrios moves it to the **Trash**. Nothing is lost at that
point, and the Trash view in the sidebar is where you go to look at it again.

**Move to Trash** sits in the editor toolbar for any note you can edit. It asks
first, and the question says what happens rather than "are you sure": the note
stays in the Trash until you restore it or delete it permanently.

Open a note from the Trash and it looks different. The title is greyed out, an
**In the Trash** badge replaces the Save button, and two actions appear:

- **Restore** — the note becomes editable again, in whichever notebook it is
  currently assigned to.
- **Delete forever** — the note and every revision of it are removed. This one
  cannot be undone, and the confirmation says so.

A trashed note is still readable, which is the point: you get to look at it
before deciding. It is not searchable from ordinary queries — `is:trashed` is
the only way to reach it, and the Trash view is that query.

Notes that came from an import (Joplin, Obsidian, Twitter/X, a conversation
export) cannot be permanently deleted. They stay in the Trash, out of search,
rather than losing the record that they were ever imported.

### Deleting a notebook

Notebook rows show a 🗑 when you hover over them or reach them with the
keyboard, except for the ones that cannot be deleted (**All notes**, **Trash**,
**Help**, and the default **Notes** notebook).

Clicking it asks the service what would happen and shows you that answer, not a
generic warning:

```text
Delete the notebook “Work”?

3 notebooks are removed, including Reports, Drafts.
12 note(s) are not deleted: they move to the Trash, where you can restore them.
Those notes move to “Notes”, so restoring one later has somewhere to put it.
```

That last line is the part worth reading. The notes in a deleted notebook are
re-homed to the default notebook on their way to the Trash — otherwise restoring
one later would have nowhere to put it.

## Themes

The 🌙/☀️ button toggles between your light and dark themes; the 🎨 button opens theme settings. You can create custom themes (cloned from the current one, with per-color editing) and choose any theme — builtin or custom — as the one used for light mode and for dark mode.

## Protected items

"All notes" and "Trash" cannot be deleted. The "Help" notebook cannot be deleted
and its notes are read-only. The default "Notes" notebook cannot be deleted
(restored notes land there if their original notebook is gone).

A read-only Help note and a trashed note both refuse edits, and the GUI
distinguishes them on purpose: a Help note shows **Read-only Help note** and
will never be editable, while a trashed note shows **In the Trash** and is one
click from being editable again.
