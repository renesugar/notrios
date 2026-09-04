# GUI journeys

The [GUI guide](gui.md) describes what the GUI has in it. This page shows
you doing things with it: a task, the steps, and a picture of each step with the
place to click marked.

The same tasks are shown for the command line in
[Command-line journeys](journeys-cli.md). Where a task can only be done on one
of the two, both pages say so.

## Reading the pictures

Each screenshot carries two marks. The **thin outline** is the thing you are
clicking — how much of it is clickable. The **red circle** is the point the
click lands on.

The library in these pictures is a throwaway one made for the purpose. None of
it is anybody's notes.

## What the GUI does not do
<!-- notrios:generated:user:what-the-gui-does-not-do:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docfeatures#(Registry).WithoutGUILines -->
Everything here is command-line or API work today. Where a capability is
absent for a reason rather than for want of doing it, the reason is given.

- **See and rename tags** — List the tags in a library, see a note's tags, and rename a whole tag hierarchy at once. The GUI shows tags in the sidebar and on the note, and does not rename a hierarchy; `tags rename` is command line only.
- **Group libraries into collections** — Collections sit above notebooks and are how an import keeps its material together. No command line or GUI journey; imports set the collection with --collection.
- **Templates and tasks** — Keep note templates and make new notes from them, and see the tasks across a library. No command line or GUI journey.
- **Live query blocks inside a note** — Put a query in a note and have its results render where it sits. No command line; the GUI renders them in place.
- **Do many organiser operations at once** — Send a batch of moves, tags and notebook changes as one request, so a large reorganisation is one reviewable action. No command line or GUI journey; batching is for tools.
- **Choose where sync keys are kept** — An installed Notrios keeps the key that protects your sync material in the operating system's credential store. Move existing keys between that and the owner-only development file, in either direction. Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- **Watch and steer long-running work** — Imports, exports and syncs run as jobs you can list, inspect, cancel, retry and reset. No GUI journey; progress appears inline where the work was started. Not settled by the H15 crawl either: job rows only render once a job exists and nothing seeded one, so this row is unmeasured rather than known absent.
- **Publish a subset of your notes** — Choose what leaves the library, review exactly what a publication would include and withhold, save that choice as a profile, and publish only after agreeing to the reviewed plan. Not in the GUI yet. Publishing writes to a local directory, so the desktop app could do it and the browser mode could not.
- **Keep a library healthy** — Find what has rotted, fix what can be fixed mechanically, reclaim space, and check that this installation is set up the way you think it is. No GUI journey; maintenance is command line work.
- **Move a pre-0.8 library into place** — A library that lived in ./data next to the program is relocated into the directories an installed Notrios uses, after showing you the plan. Command line only, and genuinely so: it relocates the directories the running program uses, which is not something the program can sensibly do to itself while serving them.
- **Let an AI assistant use your library** — Notrios speaks MCP, so an assistant can read and, within a scope you grant, change your notes. The endpoint itself has no command line or GUI; the tools it exposes are listed against the features above.
<!-- notrios:generated:user:what-the-gui-does-not-do:end -->

This list is derived from the capability registry rather than written out, so
it cannot claim the interface is missing something it has. The paragraph it
replaced said tags were read-only here for as long as it took somebody to
notice that tagging had been built, and said importing was command-line work
after the desktop app had grown an import dialog. Neither was caught by a gate,
because prose is not a claim the gates check.


## The journeys
<!-- notrios:generated:user:the-journeys:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docjourneys#(GUICatalogue).GUILines -->
Each task below lists its steps, with a picture of every one.

- **Find your way around** — See what the GUI is made of before changing anything in it.
  - The sidebar on the left lists your notebooks. Help is one of them: the documentation is seeded into your library as ordinary read-only notes, so you can search it alongside everything else.

    ![the-sidebar](images/journeys/find-your-way-around-the-sidebar.png)
- **Write a note, and read it back** — Create a note and get something into it.
  - Click New note. The note is created immediately and opens for editing; there is no dialog to fill in first.

    ![new-note](images/journeys/write-a-note-new-note.png)
  - Type into the editor. What you write is Markdown, and the preview beside it renders as you go.

    ![the-editor](images/journeys/write-a-note-the-editor.png)
- **Choose which notebook a note goes in** — File a note somewhere other than where it landed.
  - Start from a new note.

    ![new-note](images/journeys/choose-a-notebook-new-note.png)
  - Open the notebook picker. It sits with the note rather than in a menu, because which notebook a note belongs to is part of the note.

    ![notebook-picker](images/journeys/choose-a-notebook-notebook-picker.png)
- **Tag a note** — Put a tag on the note you are writing, and take one off.
  - Start from a note. Tags belong to a note, so there has to be one open.

    ![new-note](images/journeys/tag-a-note-new-note.png)
  - Open the tag control. It sits in the editor toolbar beside the notebook control, because both answer where this note belongs and both belong where you are typing. The label shows how many tags the note carries.

    ![open-tags](images/journeys/tag-a-note-open-tags.png)
  - Type the tag. Tags are hierarchical, so `field/dusk` sits under `field`. Press Enter or use Add.

    ![type-a-tag](images/journeys/tag-a-note-type-a-tag.png)
  - Add it. Each tag then appears as a chip with its own remove button, rather than as a comma-separated line you have to edit carefully.

    ![add-it](images/journeys/tag-a-note-add-it.png)
- **Change a note you already wrote** — Open an existing note, change it, and save the change.
  - All notes is every note in the library, whichever notebook it is filed in. Start here when you know what a note is called but not where you put it.

    ![open-all-notes](images/journeys/update-a-note-open-all-notes.png)
  - Click a note in the results list to open it. The editor fills with the note as it stands; nothing is changed by opening it.

    ![open-the-note](images/journeys/update-a-note-open-the-note.png)
  - The title is an ordinary field at the top of the editor. Change it the way you would change any text.

    ![change-the-title](images/journeys/update-a-note-change-the-title.png)
  - Saving writes a new revision rather than overwriting the old one, so the version you just replaced is still there.

    ![save-it](images/journeys/update-a-note-save-it.png)
- **Delete a note** — Move a note to the Trash and confirm it arrived there.
  - Find the note first. Deleting is something you do to an open note, not to a row in a list, so that it is always clear which note is about to go.

    ![open-all-notes](images/journeys/delete-a-note-open-all-notes.png)
  - Open the note you mean to delete and read it once. This is the last screen on which its text is in front of you.

    ![open-the-note](images/journeys/delete-a-note-open-the-note.png)
  - Deleting asks you to confirm, and names the note it is about to move so that a mis-clicked row cannot become a deleted note. Deleting moves it to the Trash; it is not destroyed, and nothing here empties the Trash on a timer.

    ![delete-it](images/journeys/delete-a-note-delete-it.png)
  - Trash is a place you can open and look inside, not a countdown. The note is in it.

    ![look-in-the-trash](images/journeys/delete-a-note-look-in-the-trash.png)
- **Take a note back out of the Trash** — Restore a deleted note and confirm it is back among your notes.
  - Open the Trash to see what is in it. Everything here was deleted and none of it is gone.

    ![open-the-trash](images/journeys/restore-a-note-open-the-trash.png)
  - Open the note you want back. A trashed note opens read-only, with a badge saying why.

    ![open-the-trashed-note](images/journeys/restore-a-note-open-the-trashed-note.png)
  - Restoring returns the note to the notebook it was filed in, not to some general inbox.

    ![restore-it](images/journeys/restore-a-note-restore-it.png)
  - All notes again, to see it where it belongs.

    ![back-to-all-notes](images/journeys/restore-a-note-back-to-all-notes.png)
- **Search for a note** — Find notes by what they say, using the same query language the command line takes.
  - The search box takes the query language, not just a word. Bare words are combined with an implicit AND; an uppercase OR widens the search instead.

    ![type-a-query](images/journeys/search-your-notes-type-a-query.png)
  - Run the search. Results are ordered by relevance and page as you scroll, so a large library does not have to be loaded to be searched.

    ![run-it](images/journeys/search-your-notes-run-it.png)
- **Narrow a search by tag, and exclude what you do not want** — Use a field query and a negation together, which is what most real searches turn out to be.
  - tag: matches a tag rather than the note's text, and a leading minus excludes. Here that is everything tagged field/dusk except the documentation.

    ![type-a-field-query](images/journeys/search-by-tag-and-exclude-type-a-field-query.png)
  - The same box runs it. Every query-language feature works here exactly as it does in notriosctl search, because it is the same parser.

    ![run-it](images/journeys/search-by-tag-and-exclude-run-it.png)
- **Keep a search as a notebook** — Turn a search you have just watched work into a notebook in the sidebar.
  - Search for whatever you want the notebook to hold. Any query works: this is the same language the rest of the interface takes.

    ![run-a-search](images/journeys/keep-a-search-as-a-notebook-run-a-search.png)
  - Run it and read the results. This is the point of keeping a search here rather than typing one into a form: you are deciding about a query you have watched work, not one you hope is right.

    ![see-it-work](images/journeys/keep-a-search-as-a-notebook-see-it-work.png)
  - Keep this search appears once a search has returned something. A query that found nothing is not offered, because a notebook made from it would open empty.

    ![keep-it](images/journeys/keep-a-search-as-a-notebook-keep-it.png)
  - The query is suggested as the name and is usually not what you want to read in a sidebar. The query itself is shown below the field but cannot be edited here.

    ![name-it](images/journeys/keep-a-search-as-a-notebook-name-it.png)
  - Keeping it adds a notebook to the sidebar whose contents are whatever the query matches, now and later.

    ![save-it](images/journeys/keep-a-search-as-a-notebook-save-it.png)
<!-- notrios:generated:user:the-journeys:end -->


## Where these come from

Every picture above was taken by driving the real GUI. Nothing is a
mockup, and nothing was placed by hand: each mark is drawn from where the thing
being clicked actually was, so if a button moves the mark moves with it.

If a step here does not match what you see, that is a bug in Notrios or in this
page, and worth reporting either way.
