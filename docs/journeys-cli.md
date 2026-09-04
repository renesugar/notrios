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
- **Delete a note, and change your mind** — Send a note to Trash and bring it back. Deleting moves a note to Trash. Nothing is destroyed until Trash is emptied, and the note stays visible in the meantime.
  - Make a note to delete.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk."`
  - Delete it. The command prints the exact line that undoes it, so you do not have to go and look that up.

    `notriosctl notes delete --document <note>`
  - Look at it anyway. A note in Trash is still there and still readable; `notes show` reports when it was trashed.

    `notriosctl notes show --document <note>`
  - Bring it back.

    `notriosctl notes restore --document <note>`
- **Bring in an Obsidian vault** — Import a folder of Markdown notes without finding out afterwards what it did. Joplin, Twitter, ChatGPT and Claude exports import the same way, with `import joplin-raw`, `import twitter`, `import chatgpt` and `import claude`. Note that tags written in a vault's front matter do not become Notrios tags: add them afterwards with `tags add`.
  - Point the importer at your vault directory. A directory of Markdown files is all an Obsidian vault is, as far as this is concerned. *(you do this yourself)*
  - Dry-run first. Every importer reports what it would do before doing it, and reading that is cheaper than undoing an import.

    `notriosctl import obsidian --dry-run <vault>`
  - Run it for real.

    `notriosctl import obsidian <vault>`
- **Find notes by tag, and exclude one** — Narrow a set down by what a note is tagged with, and by what it is not. A leading minus excludes. `tag:field -tag:private` means tagged field and not tagged private.
  - Make two notes and tag them differently.

    `notriosctl notes create --title "Reed beds" --body dusk`
  - Tag the first.

    `notriosctl tags add --document <note> --tag field`
  - Select by tag. The same query language the search box takes also selects what an export contains.

    `notriosctl export archive --query tag:field <out>`
- **Find notes by title, date and plain words** — Use the rest of the query language: titles, time ranges, phrases and regular expressions. The pieces combine, and bare words are matched as text: `notebook:"Work" tag:todo quarterly` means all three. Quote anything containing a space. `since:` and `until:` take a date, a timestamp, or a time of day. `re:` takes a regular expression and `is:trashed` finds what is in Trash.
  - Make a note to find.

    `notriosctl notes create --title "Quarterly review" --body "budget notes"`
  - Match on the title. Use quotes when the value has a space in it.

    `notriosctl export archive --query "title:\"Quarterly review\"" <out>`
  - Match on a time range. `since:` and `until:` bound when a note was written.

    `notriosctl export archive --query since:2000-01-01 <out2>`
- **Back up the library, and check the backup** — Take a physical snapshot of the database and its attachments, and confirm it is sound. Restoring is `snapshot restore --intent replace` to overwrite this library, or `--intent adopt` to make the snapshot into a second replica. The intent is required because those are very different acts and neither should be the default.
  - Make something worth backing up.

    `notriosctl notes create --title "Reed beds" --body dusk`
  - Take the snapshot. This copies the database and the attachment store, not just the text.

    `notriosctl snapshot create <out>`
  - Verify it. A backup you have not verified is one you are guessing about.

    `notriosctl snapshot verify <out>`
- **See whether Recoll is being used** — Find out if the optional search sidecar is installed and switched on. Recoll is optional. Notrios searches with SQLite FTS5 on its own; Recoll adds indexing of attachment contents. It is enabled with `search_sidecar.enabled` in the configuration, and `doctor` reports both whether the binary is present and whether the configuration asks for it.
  - Ask doctor. It reports whether `recollindex` is on this machine, and separately whether this configuration turns the sidecar on -- which are different questions.

    `notriosctl doctor`
- **Synchronize through a folder on another drive** — Exchange changes with another of your libraries through a folder you both can reach. The folder can be a second drive, a USB stick, or a cloud folder mapped locally. Everything written there is encrypted and signed, and everything in it also exists in the library that published it, so deleting the folder loses nothing. The other replica runs the same command against the same folder; pairing them first is the `sync init`/`invite`/`join` ceremony.
  - Choose or mount the folder both libraries can reach. *(you do this yourself)*
  - Enrol this library for synchronization, if it is not already.

    `notriosctl sync init`
  - Make something to exchange.

    `notriosctl notes create --title "Reed beds" --body dusk`
  - Run one exchange against the folder. Polling or running this by hand is the mechanism; no filesystem watcher is needed for correctness.

    `notriosctl sync once --carrier <carrier>`
- **Make a notebook, and one that fills itself** — Create a notebook, file a note into it, and make a notebook whose contents come from a query. A query notebook has no parent: what is in it is decided by the query rather than by where you put things.
  - Make a notebook. An emoji is optional and shows before the name in the sidebar.

    `notriosctl notebooks create --name "Field notes" --icon 🌿`
  - Nest another inside it, naming the parent. A name that matches more than one notebook is refused rather than guessed, because names are unique only among siblings.

    `notriosctl notebooks create --name Dusk --parent "Field notes"`
  - File a note into it by name.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk." --notebook "Field notes"`
  - Make a notebook that fills itself. Anything matching the query appears in it; nothing is filed there by hand.

    `notriosctl notebooks create --name Todo --query tag:todo`
<!-- notrios:generated:user:the-journeys:end -->


## Where these come from

Each journey above is run against a throwaway library before this page is
published, and checked by looking at what changed rather than at whether the
command exited cleanly. A command can succeed and do nothing, and it can succeed
and do the opposite of what a page said.

If a step here does not work for you, that is a bug in Notrios or in this page,
and worth reporting either way.
