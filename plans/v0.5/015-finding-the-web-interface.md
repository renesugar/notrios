# v0.5 E11 — Finding the GUI's own files, and the docs that explain running it

Status: complete on 2026-08-07.

Model: Claude Opus 5 (Claude Code).

## What it does

`notrios` and `notriosd` now find their built interface from a documented,
ordered list of locations; the GUI refuses to start when it cannot, naming every
directory it tried; the notebook control looks like a control; every `make`
target is documented; and `web/README.md` describes the project that exists.

## The defect

`internal/httpapi/server.go` resolved `filepath.Clean("web/dist")` — relative to
the **process working directory**. That works from a checkout root and nowhere
else, so running `bin/notrios` from inside `bin/` looked for `bin/web/dist`,
found nothing, and opened a window containing:

```json
{"error":{"code":"web_ui_not_built","message":"web/dist/index.html not found; run cd web && npm run build or use npm run dev"}}
```

Two things wrong beyond the lookup. The message was **wrong in exactly the case
it fired**: the assets were built, they were simply not where that process
looked, so it sent the reader to rebuild something that already existed. And the
failure surfaced *inside the GUI*, which makes an application look broken rather
than misplaced.

There was no `--web-dir` flag and no configuration field — `config.Config` had
no UI section at all.

## Decisions worth recording

**The search order ends at the executable, not at the working directory.**
`--web-dir`, then `server.web_dir`, then `web/dist` under the working directory,
then under the executable's own directory, then under its parent. The last two
are the fix: they make `./bin/notrios` from the checkout root and `./notrios`
from inside `bin/` both work, because the assets sit beside the binary's parent
either way.

**An explicit path is the only candidate.** Given `--web-dir`, nothing else is
tried. An explicit answer that is wrong should fail loudly rather than fall
through to a directory that happens to work — a fallback there would hide a
typo and serve the wrong build.

**The failure names every directory, absolute and deduplicated.** "It is not
here" and "here is where I looked" are different messages and only the second is
actionable. Deduplication matters because running from the executable's own
directory makes two candidates the same place, and a message listing one
directory twice reads as a bug in the message. Caught by measuring the real
output rather than by trusting the code.

**"Not built" and "not found here" are different problems** and the old message
conflated them. An explicit-path failure says the path is wrong; a search
failure says how to build and how to point. A test asserts the explicit failure
does *not* advise rebuilding.

**Only the GUI refuses to start.** `-no-gui` serves REST and MCP, which work
without an interface, so it starts and says so in the log. `-gui-only` renders
whatever the *remote* service serves, so the local machine needs no assets at
all — checking there would have broken the mode that exercises the API exactly
as a third-party client does.

**The server re-resolves on a miss.** The root is resolved once at construction,
but a request that finds nothing tries again rather than reporting a stale
answer — the ordinary development case is `make serve` running while `make web`
finishes.

## Two things found while verifying

**The notebook control was behind horizontal scroll.** It was appended after
every built-in tool (`[...allToolbar, 0]`), and that toolbar scrolls
horizontally, so at a narrow pane the control was off-screen — exactly when
knowing which notebook you are in matters most. It now leads the toolbar
(`[0, '-', ...allToolbar]`), separator included.

**A Help note's control said "Notes".** The label was looked up in the pickable
options, which deliberately omit builtin notebooks because they are not valid
destinations — so a note *in* one fell through to the default name and the
control claimed the note lived somewhere it did not. The label is now resolved
over the whole notebook tree and passed in separately from the destination list.
Found by opening a Help note in a browser; every unit test passed over it.

## The smaller findings

**The dropdown had no border.** `md-editor-rt`'s toolbar item supplies neither
border nor caret, so the notebook name was bare text floating over the editor. It
now has a border, a rounded background, a `▾` caret, and hover/focus states —
and a *dashed* border when disabled, so a read-only or trashed note still reads
as a control rather than as plain text.

**Three `make` targets were undocumented:** `serve`, `doctor`, `seed-help`.
`docs/installation.md` now carries a table of all seventeen, with a column
marking which reach the network, and `ENVIRONMENT_SETUP.md` lists the everyday
loop including all three. `make serve` mattered most — it is how the interface
is opened in a browser, and the user found it before the documentation mentioned
it.

**`web/README.md` was two milestones stale** — "Notes Companion", `cmd/notesd`
(renamed in v0.2), and a v0.1 feature list. Rewritten as a frontend entry point:
how to run the dev server against `make serve`, what lives where, and the two
rules — note content is untrusted, nothing is fetched at runtime — that a change
there must not break.

**Answered, not changed:** `make clean` does not remove `web/node_modules`;
`make clobber` does. That is deliberate so a clean never forces a network
reinstall, and the target table now says so on the `clean` row where the
question gets asked.

## Validation

`go vet ./...`, `go test ./...`, required-file and scaffold checks, web
typecheck, **141** tests, and production build, `make gui`, docs-site build,
Help reseed (15 kept), REST/MCP smoke, performance smoke, and the offline-assets
check.

Verified by running the binaries, not only by reading them: from `bin/` the
service now reports `serving the web interface from …`; from a temporary
directory with no assets the GUI exits with the searched list; with a wrong
`--web-dir` it says the path is wrong rather than advising a rebuild; and the
headless service starts anyway. The control's border, caret, disabled dashed
border, leading position, and corrected Help label were all confirmed in a
browser in both themes.
