# Interface journeys

The [GUI guide](gui.md) describes what the interface has in it. This page shows
you doing things with it: a task, the steps, and a picture of each step with the
click marked.

Every picture here was taken by driving the real interface. Nothing is a mockup
and nothing was placed by hand.

## What the marks mean

Each screenshot carries two marks, and they say different things.

The **thin outline** is the element itself — how much of the screen is
clickable. The **red circle** is the point that is actually clicked, which is
the centre of that element.

Both are drawn from the element's own position at the moment the picture was
taken, never from coordinates written into a file. That has a consequence worth
stating: if a button moves, the circle moves with it, and if the thing a step
points at stops existing, the capture fails rather than producing a confident
picture of the wrong place. A screenshot in documentation is otherwise the one
kind of claim that cannot go wrong loudly.

The library in these pictures is a disposable one made for the purpose. None of
it is anybody's notes.

## The journeys
<!-- notrios:generated:user:the-journeys:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docjourneys#(GUICatalogue).GUILines -->
GUILines renders the GUI catalogue for the generated fragment.
- **Find your way around** — See what the interface is made of before changing anything in it. Confirmed when the Help notebook is present in the sidebar.
- The sidebar on the left lists your notebooks. Help is one of them: the documentation is seeded into your library as ordinary read-only notes, so you can search it alongside everything else. ![the-sidebar](images/journeys/find-your-way-around-the-sidebar.png)
- **Write a note** — Create a note and get something into it. Confirmed when the editor is open with the note in it.
- Click New note. The note is created immediately and opens for editing; there is no dialog to fill in first. ![new-note](images/journeys/write-a-note-new-note.png)
- Type into the editor. What you write is Markdown, and the preview beside it renders as you go. ![the-editor](images/journeys/write-a-note-the-editor.png)
- **Choose which notebook a note goes in** — File a note somewhere other than where it landed. Confirmed when the notebook picker is open and offering somewhere to file the note.
- Start from a new note. ![new-note](images/journeys/choose-a-notebook-new-note.png)
- Open the notebook picker. It sits with the note rather than in a menu, because which notebook a note belongs to is part of the note. ![notebook-picker](images/journeys/choose-a-notebook-notebook-picker.png)
<!-- notrios:generated:user:the-journeys:end -->


## How this page is kept honest

The steps, their sentences and their pictures all come from
`docs/docjourneys/GUI_JOURNEYS.json`, so the caption beside a screenshot is the
same text the runner used when it took it. Keeping those in two places would let
a description drift from the image next to it with every gate still green.

Two checks run. The capture itself is opt-in, because it needs a browser — but a
missing screenshot, a stale one, or a step that starts pointing at a different
element all fail an ordinary test run, by comparing the committed images against
the hashes recorded when they were taken. Regenerate them with:

```sh
NOTRIOS_GUI_JOURNEYS=1 go test ./cmd/notriosctl -run TestGUIJourneyCapture
```

Writing this found the failure it was designed to prevent. The first run
produced five screenshots with no marker drawn on any of them and no error
anywhere — the marker function had been passed to the browser as a string,
which constructs it and never calls it. Nothing noticed until two steps pointing
at different elements came out byte-identical. That comparison is now a check
rather than a coincidence.
