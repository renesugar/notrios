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
  -remote http://127.0.0.1:8080  # local service or the local end of a tunnel
```

`-gui-only` runs the GUI as a pure REST client — exactly the way a third-party
client (C++/Qt, Rust/Tauri, another Wails app) uses the service. Without
`-remote` it targets the configured listener. Ordinary GUI/REST routes are
loopback-only even when the service also listens for authenticated peers; to
connect across machines, point `-remote` at the local end of an
access-controlled tunnel. Read the [network exposure
notes](service.md#network-exposure) first.

## First launch

`make gui` builds both the binary and the interface it renders. The binary finds
`web/dist/` by looking at `--web-dir`, then `server.web_dir` in the
configuration, then `web/dist` under the working directory, the executable's own
directory, and the executable's parent — so `./bin/notrios` from the checkout
root and `./notrios` from inside `bin/` both work. Move the binary elsewhere and
you must copy `web/dist/` next to it or pass `--web-dir`. If it finds nothing it
**refuses to start and prints every directory it tried**, rather than opening a
window containing an error. See [where the interface files have to
be](installation.md#where-the-interface-files-have-to-be). In the default mode the embedded service also listens on `server.listen_addr` (default `127.0.0.1:8080`), so MCP clients and browsers can connect while the window is open. Configuration and flags are shared with the service: `-config`, `-addr`, `-db` — see [configuration](service.md#configuration-reference). Data lands wherever the config points (default `./data/` under the working directory).

On first launch you'll see the "All notes" view (empty until you create or
[import](import-export.md) notes) with the notebooks sidebar on the left. A
fresh database always bootstraps protected **All notes**, **Notes**,
**Reports**, **Help**, and **Trash** entries.

## Verifying the embedded service

```sh
curl http://127.0.0.1:8080/healthz            # -> ok
curl http://127.0.0.1:8080/api/v1/status | jq  # storage, schema, Recoll backlog/reconciliation
```

If the window opens but shows a JSON error, the build predates v0.5 E11; current
builds refuse to start and print the directories they searched. Run `make web`
and launch from the checkout root, or pass `--web-dir`.

## Diagrams

A fenced ```mermaid block is drawn as a diagram. The source stays available
under every diagram behind a "Diagram source" disclosure, and if a diagram
cannot be drawn the block keeps its source with a line above it saying why —
malformed, too large, too slow, or too many diagrams in one note.

Diagrams are drawn locally. Nothing is fetched while one renders, and a diagram
cannot run script, embed a page, or load a remote image, whatever its source
says.

A `click` directive may link a node to one of your own notes:

```
flowchart LR
    A[Design notes] --> B[Open questions]
    click A "notrios://databases/db_abc/documents/doc_xyz"
```

Links to remote sites are deliberately not clickable inside a diagram. Put the
URL in the note itself, where you can read it before deciding to follow it, and
where it opens in your browser.

## Links that leave Notrios

A remote `http`/`https` link (and `mailto:`) in a note opens in your normal
browser. In a browser tab the link opens a new tab by itself; in the desktop
window Notrios hands the URL to the system browser, because a webview does not
open new windows on its own and the click would otherwise do nothing.

The desktop hand-off runs `xdg-open`, so the browser you get is the one your
desktop is configured to use for URLs — not the `BROWSER` environment variable.
If a link appears to do nothing in the desktop window, check that `xdg-open`
opens a URL from your terminal.

Links to your own notes (`notrios://`), documents, and attachments are always
resolved inside Notrios and never handed to a browser. `javascript:`, `data:`,
and `file:` links are refused outright, both when the note is rendered and again
before anything is opened.

To use the interface in a browser instead of a window, run the service on its
own — `make serve` from source, or `./bin/notriosd` — and open it. **Which
address depends on which instance you started:** from a checkout that is
`http://127.0.0.1:8099`, and an installed Notrios uses `http://127.0.0.1:8080`.
See [the two default addresses](service.md#the-two-default-addresses).

## Common startup errors

| Symptom | Cause / fix |
|---|---|
| `Wails applications will not build without the correct build tags.` | the binary was built without the tags — use `make gui` |
| `this notrios binary was built without the GUI; rebuild with ...` | same, from the stub build; rebuild or run `-no-gui` |
| `package gtk+-3.0 was not found` / `webkit2gtk-4.1 was not found` at build time | install `libgtk-3-dev` / `libwebkit2gtk-4.1-dev` |
| `libEGL warning: DRI3 error` / renderer warnings at launch | harmless software-rendering fallback (common under VMs and Xvfb) |
| `notrios cannot start the GUI: the built web interface was not found` | build it with `make web`, run from the checkout, or pass `--web-dir /path/to/web/dist`; the message lists every directory tried |
| `service listener stopped: listen tcp ... address already in use` in the log | another notriosd/notrios owns the port; stop it or change `-addr` (the GUI itself keeps working against its in-process handler) |

## Layout
<!-- notrios:generated:user:layout:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docjourney#Manifest -->
Manifest is the finite documented GUI-procedure catalog. Each generated row
preserves the procedure's reviewed label and whether G18e executed it or
retained an explicit unverified reason.

- Open a search result — executed (search-open)
- Page through search results — executed (search-page)
- Create a note in a notebook — executed (new-note-in-notebook)
- Edit and save a note — executed (edit-save)
- Move a note to another notebook — executed (move-note)
- Trash and restore a note — executed (trash-restore)
- Permanently purge a trashed note — executed (purge-note)
- Review notebook deletion — executed (delete-notebook-review)
- Respect protected Help items — executed (protected-items)
- Change the application theme — executed (themes)
- Insert a stable note link — executed (insert-and-check-link)
- Open links from preview — executed (preview-links)
- Upload and attach a resource — executed (upload-resource)
- Review remote-media policy before localization — executed (localize-remote-media)
- Inspect the local link graph — executed (inspect-local-graph)
- Paste a table into the editor — executed (paste-table)
- Use a live query block — executed (live-query-block)
- Render math and code blocks — executed (math-and-code)
- Resize workspace panes — executed (resize-panes)
- Open Sync Center — executed (open-sync-center)
- Configure synchronization — executed (sync-setup)
- Pair a synchronization peer — executed (sync-pairing)
- Start and monitor a sync job — executed (sync-jobs)
- Request a lazy resource — executed (sync-lazy-resource)
- Review and resolve a sync conflict — executed (sync-conflict)
- Create a password backup — executed (sync-backup-create)
- Inspect a password backup — executed (sync-backup-inspect)
- Review sync recovery — executed (sync-recovery-review)
- Review synchronization retention — executed (sync-retention)
- Review peer retirement — executed (sync-retirement)
- Inspect synchronization repairs — executed (sync-repairs)
- Confirm destructive actions — executed (destructive-confirmations)
- Run an import/export job in the GUI — unverified (import-export-job-gui)
- Batch-organize notes in the GUI — unverified (batch-organizer-gui)
- Find and replace in the editor — unverified (editor-find-replace)
- Use native shell menu actions — unverified (native-shell-actions)
- Apply destructive sync restore — unverified (destructive-restore-apply)
<!-- notrios:generated:user:layout:end -->

Below the menu bar the window is four side-by-side panes, always in this order:

1. **Sidebar** — notebooks and tags. The builtin **All notes** view is always at the top and **Trash** is always at the bottom, with **Reports** and then **Help** immediately above Trash; between them are your notebooks (nested, with optional emoji icons) and your saved search notebooks. Below the notebooks is the tag list with live note counts. Click anything to search it.
2. **Search panel** — the search box plus results. Its inline hint covers
   uppercase OR, implicit AND, prefix `-`, grouping, and `category:`/
   `notebook:` fields; the complete grammar is in [Search query
   language](query-language.md). On startup the "All notes" view loads, and
   results page in incrementally as you scroll (a keyboard-accessible "Load
   more" button covers the same path), so even huge databases start instantly.
   Read-only notes show a 🔒 marker. Above the results, **New note** starts a
   draft and names where it will be filed — see [choosing the
   notebook](#choosing-which-notebook-a-note-goes-in).
3. **Markdown editor** — title on its own line, then a row holding the note's
   state and the actions that apply to it; below those, the editor toolbar and
   the source editor. Narrow the pane and that action row stacks vertically as a
   unit rather than wrapping a button at a time. Every control there acts on the
   note in front of you — starting a *new* note lives in the search panel,
   because it does not. Note details (metadata, uploads, links, backlinks, resources) live in a collapsible **Note info** inspector at the bottom of this pane, not in a pane of their own. When a note references remote images or media, the inspector also lists each URL with its server-side policy decision (allow/block/review) — a static scan; nothing is downloaded. For editable notes with allowed media, a **Localize allowed media** button downloads those files through the server's quarantine pipeline, stores them as local resources, and rewrites the note to `resource://` links in a new revision (blocked URLs are never fetched).
4. **Markdown preview** — rendered Markdown. `document://` links open the target note; `resource://` links download attachments; images can be pasted/uploaded in the editor and become local resources.

Each pane scrolls on its own; the window itself never scrolls.

### Sync Center
<!-- notrios:generated:user:sync-center:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/httpapi#(*Server).handleSyncUIRetirePeer -->
handleSyncUIRetirePeer requires the GUI review flow to echo the exact peer
identity before it records the signed, destructive retirement decision.
<!-- notrios:generated:user:sync-center:end -->

The **↻ Sync** button opens the local Sync Center. Its header always names the
active profile, logical library, and replica so setup cannot silently target a
different copy. Sections cover overview/jobs, profile and transport setup,
peers, lazy attachments, body conflicts, backup/recovery, and deterministic
repair reports.

Transport changes are saved to the active profile and require restart; they do
not pair or start work. The desktop's local-service mode offers **Choose
folder…** for a shared-directory carrier. Browsers retain an absolute-path
field, and `-gui-only` deliberately has no native chooser because its local
filesystem is not the remote service's filesystem. REST inbound access is a
separate explicit checkbox, and pairing still needs a short-lived code.

Allowing ordinary sync does not authorize a peer to receive a complete library
snapshot. Use **Allow catch-up snapshot** on that peer only when intended.
Catch-up downloads and verifies into private staging; reset and restore remain
blocked on a separate destructive review. Password backups never remember the
password. Wrong-password retry and Cancel change no library data, and a
successful upload review still does not apply the backup.

At narrow/mobile-sized viewports the Sync Center becomes a full-screen,
touch-sized layout. This is responsive design only; it is not a supported
mobile build.

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

### Seeing what is around a note

The **Note info** inspector ends with **Nearby notes**: the notes one or two
link hops from the one you have open, grouped by distance, in either direction.
Click one to open it. Resources and links that do not resolve appear as plain
entries rather than as something to click, since there is nothing to open.

There is deliberately no whole-library graph picture. Past a few thousand notes
a global graph becomes an unreadable tangle, while the neighbourhood of *one*
note stays exactly as readable as it ever was — its size is set by the note, not
by the library. For the shape of the whole library there is the [graph
report](#the-graph-report) instead, which is a ranked list, and a ranked list
reads the same at any size.

If a ceiling stops the expansion, the panel says so rather than showing a
partial neighbourhood as though it were the whole one.

### The graph report

The builtin **Reports** notebook, just above Help, holds notes Notrios writes
about your library. Today that is the **Library graph report**: your most-linked
notes, your orphans, and the totals, with the time it was generated.

It is regenerated when you ask and not before — it reads every note, so nothing
does it on a timer or on save:

```sh
notriosctl graph report --write-note
```

The note is read-only and overwritten in place, so a link to it keeps working
and it cannot silently drift from the truth. Nothing in Reports or Help is
measured by the report, so writing it does not change what it says, and neither
notebook travels in a publication.

To hand the graph to software built for graph analysis:

```sh
notriosctl graph export ~/graph      # nodes.csv and edges.csv
```

Gephi, Cytoscape, NetworkX, and igraph all read those. Notrios does not try to
do centrality or community detection itself.

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

## Choosing which notebook a note goes in

Select a notebook in the sidebar and it becomes the target: the sidebar
highlights it, and the **New note** button in the search panel says where the
note will be filed — "New note in **Work**". This is the same model Joplin uses,
and the sidebar highlight is the main cue.

Selecting **All notes**, the **Trash**, the **Help** notebook, or a saved search
falls back to the default **Notes** notebook. Those name a view rather than a
place, and a note has to be filed somewhere.

The editor's own toolbar carries a notebook control showing where the open note
lives. It is the second cue, and it is also the correction: pick another
notebook and the note is filed there immediately, with a message naming the
destination. That matters because the usual way notes end up misfiled is
noticing several of them at once, long after the sidebar moved.

Two details worth knowing:

- The control shows the **open note's** notebook, not the sidebar's selection.
  Reaching a note from "All notes" shows you where that note actually lives.
- On a read-only or trashed note it is visible but disabled, so a note's
  notebook is still legible where it cannot be changed. **Help** and **Reports**
  are never offered as destinations — the service refuses notes moved into or
  out of either.

To move a note from the command line:

```sh
notriosctl notes move --document doc_01H... --notebook nb_01H...
notriosctl notes move --document doc_01H... --notebook Work
```

A notebook name is accepted when it identifies one notebook. Names are unique
only among siblings, so if you have both `Contacts/Work` and `Personal/Work` the
command refuses the name and asks for the ID rather than guessing.

Moving several notes at once is not available yet.

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
