# v0.8 H2b — Desktop external-link opening

Date: 2026-09-01
Status: complete
Model: Claude Opus 5 (`claude-opus-5`) via Claude Code 2.1.252, parent-owned
throughout; no subagents were used.

## Outcome

A remote `http`, `https`, or `mailto:` link in a note now opens the user's
browser from the Wails desktop window. Previously the click was silently
swallowed and the only way to follow the link was to copy the URL out by hand.

The browser-tab path is unchanged: there the anchor's own `target="_blank"`
already opens a tab, and intercepting would have replaced working behaviour
with our own.

## Why the click did nothing

WebKitGTK raises its `create` signal for a `target="_blank"` click. Wails
v2.13.0's Linux webview connects handlers for script messages, context menu,
button press and release, load-changed, drag, and window delete, but not
`create` and not `decide-policy`. With no handler, the default returns NULL and
nothing happens. `cmd/notrios/gui_wails.go` added no link handling of its own,
and the repository made no `BrowserOpenURL` call anywhere.

## What was implemented

`web/src/desktop.ts` holds the whole integration:

- `isDesktopShell()` feature-detects `window.runtime.BrowserOpenURL` rather than
  sniffing the user agent. A WebKit webview and a Safari tab report the same
  engine; the question is not which engine but whether a host exists that can
  open a browser.
- `isExternalLink()` admits exactly `http://`, `https://`, and `mailto:` —
  deliberately the same set `normalizePreviewHTML` already keeps on an anchor.
  This item changes how an already-permitted link is followed and must never
  widen what is followable.
- `openExternalURL()` hands the URL to the host and reports whether it took it.

`PreviewPane`'s existing click handler, which already routes `document://`,
`resource://`, and `notrios://`, gained one branch at the end. It suppresses the
default only when the desktop shell took the link.

Suppressing the default is half the fix. A webview that followed the link in
place would replace the running application with a website, which is worse than
the silent no-op it replaces, so a test asserts `defaultPrevented` in both
directions: true on the desktop, false in a browser tab.

## Validation

- 12 new frontend tests; the full suite is 184 tests, up from 172. Typecheck and
  production build pass.
- Both guards were verified to fail against deliberate regressions: removing
  `event.preventDefault()` fails the desktop assertion, and widening
  `isExternalLink` to accept any non-empty string fails three scheme tests.
- The GUI still builds under `gui desktop production webkit2_41`.
- The Go leg was verified end to end in isolation: `browser.OpenURL` on Linux
  runs `xdg-open <url>`, confirmed by shadowing `xdg-open` on `PATH` and
  observing the exact URL arrive.
- `go test ./...`, `go vet`, gofmt, and scaffold validation pass.

## The verification chain, and its one gap

1. The click handler calls `window.runtime.BrowserOpenURL(href)` — **verified by
   test**.
2. The Wails runtime marshals that to `window.WailsInvoke("BO:" + url)` — read
   from the compiled desktop runtime, not independently exercised. It is the
   same bridge every other runtime call in the application already uses.
3. Go's `Frontend.BrowserOpenURL` validates and calls `browser.OpenURL` — read
   from Wails source.
4. `browser.OpenURL` runs `xdg-open <url>` — **verified empirically**.

**No click in a running GUI was observed.** Driving a click inside a WebKitGTK
webview is not something this environment can script reliably. Steps 1 and 4 are
proven, 2 and 3 are read from source, and the honest summary is that the chain
is sound but unwitnessed end to end.

A manual check on a machine with a display: build with `make gui`, open a note
containing a remote link, click it, and confirm the browser opens and the
Notrios window still shows the note. Record the desktop environment and
WebKitGTK version, because the behaviour being fixed is webview-specific.

## Security notes

Nothing was widened. Two independent layers still refuse dangerous schemes:
`normalizePreviewHTML` removes the `href` from anything that is not
`http`/`https`/`mailto`/`#` when the note is rendered, and Wails'
`ValidateAndSanitizeURL` refuses `javascript`, `data`, `file`, `ftp`, and empty
schemes before opening anything. `isExternalLink` is a third check in front of
both, and tests cover `javascript:`, `data:`, `file:`, `vbscript:`, and a scheme
hidden later in the string.

In-app schemes never reach the browser: `notrios://`, `document://`, and
`resource://` are matched earlier in the handler and a test asserts a note link
routes in-app with no host call.

## Decision taken

- **Should leaving the application be confirmed first? — No prompt**, the
  documented non-blocking default. The browser UI already follows these links
  without asking, so a desktop-only dialog would be an inconsistency rather than
  a protection. If a confirmation is ever wanted, it belongs in preferences, not
  per click.

## Documentation

`docs/gui.md` gained "Links that leave Notrios", which also records the fact
most likely to confuse someone debugging this: the hand-off runs `xdg-open`, so
the browser is the desktop's configured URL handler and **not** the `BROWSER`
environment variable. `pkg/browser` ignores `BROWSER` on Linux entirely.

`UI_DESIGN.md`'s link table now names the desktop behaviour beside the browser
behaviour.

## Follow-up

This closes the dependency H2a recorded for its diagram link policy: de-linking
a remote URL in a diagram is now coherent, because the reader can reach it from
the note that contains it.
