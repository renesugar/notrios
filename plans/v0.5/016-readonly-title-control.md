# v0.5 E12 — A read-only title you can still read

Status: complete on 2026-08-07.

Model: Claude Opus 5 (Claude Code).

## What it does

The note title in the editor pane is `readOnly` rather than `disabled` when the
note cannot be edited — a Help note or one in the Trash.

## The defect

`disabled` and `readOnly` both refuse edits, and they are not otherwise
interchangeable. **A disabled input leaves the tab order.** It cannot be
focused, so no keyboard interaction reaches it: no caret, no arrow keys, no
Home/End, no select-and-copy. A title longer than the control was therefore
unreadable — the overflow could not be scrolled into view by any means except
resizing the pane.

Measured in a browser against a plain, non-React probe input for contrast:

| | `disabled` | `readOnly` |
|---|---|---|
| focusable | **false** | **true** |
| selectable | true | true |
| scrollable | true | true |

One difference, and it is the one that matters. Everything the user asked for —
reading a long title with the keyboard while being unable to change it — follows
from it.

Confirmed in the running application: the title of a trashed note is
`readOnly: true`, `disabled: false`, appears at position 12 of 53 in the tab
order, and takes focus on click.

## Decisions worth recording

**The CSS selector moved with the attribute.** `.title-input:disabled` became
`.title-input[readonly]`, plus `cursor: text` — the control is read-only, not
inert, so it keeps a text cursor and a focus ring. Styling it as though it were
dead would undo in appearance what the attribute change fixed in behaviour.

**The test types the way a person does.** `fireEvent.change` dispatches
synthetically and sails straight past `readonly`, so an assertion built on it
would have been testing jsdom's laxness rather than the control. The fixture
uses `@testing-library/user-event`, already a dependency, and asserts both that
no handler fires and that the value is unchanged.

**What is asserted is reachability, not caret arithmetic.** Chrome does not
advance `selectionStart` on a read-only input, though the field still scrolls.
That was confirmed to be the engine's behaviour rather than the application's by
reproducing it on a plain input outside React — worth recording, because the
obvious test to write (`press End`, `expect(caret).toBe(length)`) would fail for
a reason that has nothing to do with Notrios.

## Validation

`go vet ./...`, `go test ./...`, required-file checks, web typecheck, **143**
tests, and production build, `make gui`, REST/MCP smoke, and the offline-assets
check.

Browser verification covered the real control on a trashed note (readOnly, not
disabled, focusable, in the tab order) and the `disabled`-versus-`readOnly`
comparison that explains why the change was needed.
