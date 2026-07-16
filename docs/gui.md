# The built-in GUI

The `notrios` desktop app is one binary containing the GUI and the service.

```bash
make gui          # builds bin/notrios (needs GTK3 + WebKit2GTK dev packages on Linux)
./bin/notrios                 # GUI + local service
./bin/notrios -no-gui         # service only (use any client you like)
./bin/notrios -gui-only \
  -remote http://host:8080    # GUI against a service on another machine
```

`-gui-only` runs the GUI as a pure REST client — exactly the way a third-party client (C++/Qt, Rust/Tauri, another Wails app) would use the service.

## Layout

- **Left sidebar** — notebooks first: the builtin **All notes** view is always at the top and **Trash** is always at the bottom; between them are your notebooks (nested, with optional emoji icons) and your saved search notebooks. Below the notebooks is the tag list with live note counts. Click anything to search it.
- **Search panel** — the search box plus results. On startup the "All notes" view loads incrementally ("Load more" pages through cursors), so even huge databases start instantly.
- **Editor and preview** — a split Markdown editor/preview. `document://` links in the preview open the target note; `resource://` links download attachments; images can be pasted/uploaded and become local resources.

The menu bar offers File (Reload/Quit), Edit, View (fullscreen), and Help — Help searches the built-in **Help notebook**, which holds this documentation offline (`notebook:help` finds it too).

## Themes

The 🌙/☀️ button toggles between your light and dark themes; the 🎨 button opens theme settings. You can create custom themes (cloned from the current one, with per-color editing) and choose any theme — builtin or custom — as the one used for light mode and for dark mode.

## Protected items

"All notes" and "Trash" cannot be deleted. The "Help" notebook cannot be deleted and its notes are read-only. The default "Notes" notebook cannot be deleted (restored notes land there if their original notebook is gone).
