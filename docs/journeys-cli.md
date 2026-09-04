# Command-line journeys

The [CLI reference](cli.md) tells you what each command and flag does. This page
is the other half: tasks people actually set out to do, with the steps that do
them and how you can tell each one worked.

The same tasks are shown for the interface in [Interface journeys](journeys-gui.md).
Where a task can only be done on one of the two, both pages say so.

Some steps are yours rather than Notrios' — writing a file, plugging in a drive.
Those are marked, because a list that skipped them would make a task look
shorter than it is.

## What the command line does not do

`notriosctl` writes, reads and maintains a library. It is not an editor: there
is no interactive editing session, no preview, and no live search. Long-form
writing belongs in the [interface](gui.md).

Two narrower limits are worth knowing before you start.

**The Obsidian importer cannot choose a notebook.** It takes `--collection`, not
`--notebook`, so notes imported that way land where the importer puts them and
have to be moved afterwards with `notes move`.

**`--collection` will not create a collection.** Naming one that does not exist
fails with `FOREIGN KEY constraint failed`, which is the database talking rather
than Notrios. Use `default`, or a collection that already exists.

## The journeys
<!-- notrios:generated:user:the-journeys:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docjourneys#(Catalogue).Lines -->
Each task below lists the steps that do it, in order.

- **Write a note, read it back, and change it** — Create a note, see what it holds, and edit it without losing the rest.
  - Write the note. The body can be an argument, a file, or standard input, so a note can be the end of a pipeline.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk."`
  - Read it back. `notes show` reports the title, notebook, tags, revision and timestamps; add `--body` when you want the text as well, which is left out by default because a note can be long.

    `notriosctl notes show --document <note>`
  - Change the title. Anything you do not pass keeps its current value, so editing a title never empties a body, and the edit is recorded as a new revision.

    `notriosctl notes edit --document <note> --title "Reed beds at dusk"`
- **Add, change and remove a note's tags** — Put tags on a note, swap one for another, and take one off. Tags are hierarchical: `field/dusk` sits under `field`, and renaming `field` with `tags rename --include-children` carries it along.
  - Make a note to tag.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk."`
  - Add a tag. The command prints the note's tags afterwards, so you can see the result without running anything else.

    `notriosctl tags add --document <note> --tag field/dusk`
  - Add a second one. A note carries as many tags as you give it.

    `notriosctl tags add --document <note> --tag birds`
  - Change a tag by taking the old one off. There is no rename-on-one-note: `tags rename` changes a tag everywhere, which is a different thing from correcting one note.

    `notriosctl tags remove --document <note> --tag field/dusk`
  - Check what the note carries now. Asking for a note that does not exist is refused rather than answered with an empty list.

    `notriosctl tags list --document <note>`
- **Find out where your notes actually live** — See which library this installation is using before doing anything to it.
  - Ask Notrios where it resolved its directories. This is the first thing to run when notes seem to have gone missing: more often than not they are in a different library from the one the command you just ran was talking to.

    `notriosctl paths`
  - Check the installation itself. doctor creates the database if it is not there yet, so running it also tells you the library is usable.

    `notriosctl doctor`
- **Find notes with the query language** — Select a subset of notes by tag, notebook or text, and see exactly which ones matched. The command line has no search command; it applies the same query language to an export instead, which is how you see a result set as files.
  - Export only the notes a query matches. The query language is the same one the GUI search box takes.

    `notriosctl export archive --query tag:field <out>`
- **Export the library and check the export is sound** — Write a portable copy of everything and confirm it is complete before trusting it.
  - Write a native archive. This is the format that preserves identity, revisions and attachments rather than just the text.

    `notriosctl export archive-v2 <out>`
  - Verify it. An export you have not verified is a backup you are guessing about.

    `notriosctl verify archive-v2 <out>`
- **Keep work notes and personal notes apart** — Run more than one library on this machine, each with its own database and settings.
  - Create a profile. It gets its own database, its own configuration and its own address, so nothing in it can reach anything in another.

    `notriosctl profile create --name work_notes`
  - List what this machine now knows about.

    `notriosctl profile list`
- **Prepare a library to synchronize** — Turn on synchronization for a library and see where its keys are kept.
  - Enrol the library. Nothing before this point writes a single sync record, and the command tells you which store holds the key material.

    `notriosctl sync init`
  - Check what it reports afterwards.

    `notriosctl sync status`
- **Find what has gone stale in a library** — See broken links, orphaned attachments and other rot, and fix what can be fixed mechanically.
  - Run the workspace lint. It reports rather than changes anything.

    `notriosctl lint`
  - See what a mechanical fix would do. `fix` dry-runs by default, which is the safe order: read the plan, then apply it.

    `notriosctl fix`
- **Read the documentation inside your own library** — Get the Notrios guides into the library as notes, so they are searchable like anything else.
  - Seed the Help notebook. The pages become ordinary read-only notes, which is the point: help you can search alongside your own notes rather than a separate place to go.

    `notriosctl seed-help <docs>`
- **Move sync keys into the operating system's keychain** — Take key material out of the development file and put it where the OS keeps secrets. Nothing about this command is guessable: the subcommand name, the direction flag and the confirmation are all specific to Notrios, which is what makes it able to measure a page.
  - Enrol the library first, choosing the development file so there is something to move.

    `notriosctl sync init`
  - See the plan without moving anything. The command reports which store holds the keys now and which would hold them afterwards, and writes nothing.

    `notriosctl sync migrate-credentials --to native --dry-run`
<!-- notrios:generated:user:the-journeys:end -->


## Where these come from

Each journey above is run against a throwaway library before this page is
published, and checked by looking at what changed rather than at whether the
command exited cleanly. A command can succeed and do nothing, and it can succeed
and do the opposite of what a page said.

If a step here does not work for you, that is a bug in Notrios or in this page,
and worth reporting either way.
