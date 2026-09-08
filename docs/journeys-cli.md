# Command-line journeys

The [CLI reference](cli.md) tells you what each command and flag does. This page
is the other half: tasks people actually set out to do, with the steps that do
them and how you can tell each one worked.

The same tasks are shown for the GUI in [GUI journeys](journeys-gui.md).
Where a task can only be done on one of the two, both pages say so.

Some steps are yours rather than Notrios' — writing a file, plugging in a drive.
Those are marked, because a list that skipped them would make a task look
shorter than it is.

## What the command line does not do

`notriosctl` writes, reads and maintains a library. It is not an editor: there
is no interactive editing session, no preview, and no live search. Long-form
writing belongs in the [GUI](gui.md).

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

**Write a note, read it back, and change it** — Create a note, see what it holds, and edit it without losing the rest.
  - Write the note. The body can be an argument, a file, or standard input, so a note can be the end of a pipeline.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk."`
  - Read it back. `notes show` reports the title, notebook, tags, revision and timestamps; add `--body` when you want the text as well, which is left out by default because a note can be long.

    `notriosctl notes show --document <note>`
  - Change the title. Anything you do not pass keeps its current value, so editing a title never empties a body, and the edit is recorded as a new revision.

    `notriosctl notes edit --document <note> --title "Reed beds at dusk"`
**Add, change and remove a note's tags** — Put tags on a note, swap one for another, and take one off. Tags are hierarchical: `field/dusk` sits under `field`, and renaming `field` with `tags rename --include-children` carries it along.
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
**Find out where your notes actually live** — See which library this installation is using before doing anything to it.
  - Ask Notrios where it resolved its directories. This is the first thing to run when notes seem to have gone missing: more often than not they are in a different library from the one the command you just ran was talking to.

    `notriosctl paths`
  - Check the installation itself. doctor creates the database if it is not there yet, so running it also tells you the library is usable.

    `notriosctl doctor`
**Find notes with the query language** — Select a subset of notes by tag, notebook or text, and see exactly which ones matched. The command line has no search command; it applies the same query language to an export instead, which is how you see a result set as files.
  - Export only the notes a query matches. The query language is the same one the GUI search box takes.

    `notriosctl export archive --query tag:field <out>`
**Export the library and check the export is sound** — Write a portable copy of everything and confirm it is complete before trusting it.
  - Write a native archive. This is the format that preserves identity, revisions and attachments rather than just the text.

    `notriosctl export archive-v2 <out>`
  - Verify it. An export you have not verified is a backup you are guessing about.

    `notriosctl verify archive-v2 <out>`
**Keep work notes and personal notes apart** — Run more than one library on this machine, each with its own database and settings.
  - Create a profile. It gets its own database, its own configuration and its own address, so nothing in it can reach anything in another.

    `notriosctl profile create --name work_notes`
  - List what this machine now knows about.

    `notriosctl profile list`
**Prepare a library to synchronize** — Turn on synchronization for a library and see where its keys are kept.
  - Enrol the library. Nothing before this point writes a single sync record, and the command tells you which store holds the key material.

    `notriosctl sync init`
  - Check what it reports afterwards.

    `notriosctl sync status`
**Find what has gone stale in a library** — See broken links, orphaned attachments and other rot, and fix what can be fixed mechanically.
  - Run the workspace lint. It reports rather than changes anything.

    `notriosctl lint`
  - See what a mechanical fix would do. `fix` dry-runs by default, which is the safe order: read the plan, then apply it.

    `notriosctl fix`
**Read the documentation inside your own library** — Get the Notrios guides into the library as notes, so they are searchable like anything else.
  - Seed the Help notebook. The pages become ordinary read-only notes, which is the point: help you can search alongside your own notes rather than a separate place to go.

    `notriosctl seed-help <docs>`
**Move sync keys into the operating system's keychain** — Take key material out of the development file and put it where the OS keeps secrets. Nothing about this command is guessable: the subcommand name, the direction flag and the confirmation are all specific to Notrios, which is what makes it able to measure a page.
  - Enrol the library first, choosing the development file so there is something to move.

    `notriosctl sync init`
  - See the plan without moving anything. The command reports which store holds the keys now and which would hold them afterwards, and writes nothing.

    `notriosctl sync migrate-credentials --to native --dry-run`
**Delete a note, and change your mind** — Send a note to Trash and bring it back. Deleting moves a note to Trash. Nothing is destroyed until Trash is emptied, and the note stays visible in the meantime.
  - Make a note to delete.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk."`
  - Delete it. The command prints the exact line that undoes it, so you do not have to go and look that up.

    `notriosctl notes delete --document <note>`
  - Look at it anyway. A note in Trash is still there and still readable; `notes show` reports when it was trashed.

    `notriosctl notes show --document <note>`
  - Bring it back.

    `notriosctl notes restore --document <note>`
**Bring in an Obsidian vault** — Import a folder of Markdown notes without finding out afterwards what it did. Twitter, ChatGPT and Claude exports import the same way, with `import twitter`, `import chatgpt` and `import claude`. Joplin has its own journey below. Note that tags written in a vault's front matter do not become Notrios tags: add them afterwards with `tags add`.
  - Point the importer at your vault directory. A directory of Markdown files is all an Obsidian vault is, as far as this is concerned. *(you do this yourself)*
  - Dry-run first. Every importer reports what it would do before doing it, and reading that is cheaper than undoing an import.

    `notriosctl import obsidian --dry-run <vault>`
  - Run it for real.

    `notriosctl import obsidian <vault>`
**Find notes by tag, and exclude one** — Narrow a set down by what a note is tagged with, and by what it is not. A leading minus excludes. `tag:field -tag:private` means tagged field and not tagged private.
  - Make two notes and tag them differently.

    `notriosctl notes create --title "Reed beds" --body dusk`
  - Tag the first.

    `notriosctl tags add --document <note> --tag field`
  - Select by tag. The same query language the search box takes also selects what an export contains.

    `notriosctl export archive --query tag:field <out>`
**Find notes by title, date and plain words** — Use the rest of the query language: titles, time ranges, phrases and regular expressions. The pieces combine, and bare words are matched as text: `notebook:"Work" tag:todo quarterly` means all three. Quote anything containing a space. `since:` and `until:` take a date, a timestamp, or a time of day. `re:` takes a regular expression and `is:trashed` finds what is in Trash.
  - Make a note to find.

    `notriosctl notes create --title "Quarterly review" --body "budget notes"`
  - Match on the title. Use quotes when the value has a space in it.

    `notriosctl export archive --query "title:\"Quarterly review\"" <out>`
  - Match on a time range. `since:` and `until:` bound when a note was written.

    `notriosctl export archive --query since:2000-01-01 <out2>`
**Back up the library, and check the backup** — Take a physical snapshot of the database and its attachments, and confirm it is sound. Restoring is `snapshot restore --intent replace` to overwrite this library, or `--intent adopt` to make the snapshot into a second replica. The intent is required because those are very different acts and neither should be the default.
  - Make something worth backing up.

    `notriosctl notes create --title "Reed beds" --body dusk`
  - Take the snapshot. This copies the database and the attachment store, not just the text.

    `notriosctl snapshot create <out>`
  - Verify it. A backup you have not verified is one you are guessing about.

    `notriosctl snapshot verify <out>`
**See whether Recoll is being used** — Find out if the optional search sidecar is installed and switched on. Recoll is optional. Notrios searches with SQLite FTS5 on its own; Recoll adds indexing of attachment contents. It is enabled with `search_sidecar.enabled` in the configuration, and `doctor` reports both whether the binary is present and whether the configuration asks for it.
  - Ask doctor. It reports whether `recollindex` is on this machine, and separately whether this configuration turns the sidecar on -- which are different questions.

    `notriosctl doctor`
**Synchronize through a folder on another drive** — Exchange changes with another of your libraries through a folder you both can reach. The folder can be a second drive, a USB stick, or a cloud folder mapped locally. Everything written there is encrypted and signed, and everything in it also exists in the library that published it, so deleting the folder loses nothing. The other replica runs the same command against the same folder; pairing them first is the `sync init`/`invite`/`join` ceremony.
  - Choose or mount the folder both libraries can reach. *(you do this yourself)*
  - Enrol this library for synchronization, if it is not already.

    `notriosctl sync init`
  - Make something to exchange.

    `notriosctl notes create --title "Reed beds" --body dusk`
  - Run one exchange against the folder. Polling or running this by hand is the mechanism; no filesystem watcher is needed for correctness.

    `notriosctl sync once --carrier <carrier>`
**Make a notebook, and one that fills itself** — Create a notebook, file a note into it, and make a notebook whose contents come from a query. A query notebook has no parent: what is in it is decided by the query rather than by where you put things.
  - Make a notebook. An emoji is optional and shows before the name in the sidebar.

    `notriosctl notebooks create --name "Field notes" --icon 🌿`
  - Nest another inside it, naming the parent. A name that matches more than one notebook is refused rather than guessed, because names are unique only among siblings.

    `notriosctl notebooks create --name Dusk --parent "Field notes"`
  - File a note into it by name.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk." --notebook "Field notes"`
  - Make a notebook that fills itself. Anything matching the query appears in it; nothing is filed there by hand.

    `notriosctl notebooks create --name Todo --query tag:todo`
**Bring in a Joplin library** — Import a Joplin RAW export, after seeing what it would do. It must be a RAW export, not a `.jex` file -- JEX is a tar archive. Point it at the export directory, never at Joplin's profile directory or its synchronization target. Unlike an Obsidian vault, a RAW export carries notebooks and tags as items of their own, so those come across.
  - In the Joplin desktop app, use File and export in the RAW format. You want the directory it produces, which holds one Markdown file per item and a `resources/` folder for attachments. *(you do this yourself)*
  - Dry-run it. The dry run uses the same inventory and planner as the real import and reports what it found, what it could not read, and what it would create. It writes nothing to your library.

    `notriosctl import joplin-raw --dry-run <joplin>`
  - Import it.

    `notriosctl import joplin-raw <joplin>`
**Find the notebooks and collections a query can name** — Discover the identifiers `notebook:` and `collection:` take, before writing a query that uses one. A search spans every collection unless a `collection:` term narrows it. Notes written here are in `default`; notes that arrived from somewhere else carry the identifier their import was given.
  - Import something, so the library has more than one provenance. `--collection` names where the notes came from, and creates that collection if the identifier is new.

    `notriosctl import joplin-raw --collection joplin <joplin>`
  - List the collections. The count is how you tell whether an import landed anywhere: a collection with no notes did not.

    `notriosctl collections list`
  - Look at one on its own.

    `notriosctl collections show --collection joplin`
  - List the notebooks. The identifier is what a query takes; the name is what you recognise. Add `--json` when a script needs to read this.

    `notriosctl notebooks list`
**Find a note, then act on the id the search returned** — Search for a note and use the identifier it hands back with the commands that take one. This is the loop the command line could not close before v0.8 H19: nothing at a terminal produced note identifiers, so every command that takes one could only be used on an id somebody already had.
  - Write a note to find.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk."`
  - Search for it. The query language is the one the search box parses, so `tag:`, `notebook:` and `collection:` mean here what they mean there.

    `notriosctl search dusk`
  - Ask how many notes match rather than which ones. This is a counting query over the same predicate, not the hits fetched and tallied.

    `notriosctl search --count dusk`
  - Read the note the search found, using the identifier it returned.

    `notriosctl notes show --document <note>`
  - Tag it, with the same identifier.

    `notriosctl tags add --document <note> --tag wetland`
**Read a note, and what it is made of** — Print a note as Markdown another application can read, then its headings, attachments and links. `notes show` prints the note itself, with the front matter the Recoll projection writes. The other three answer what the note is made of rather than what it says.
  - Write a note with a heading and a link in it.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk.\n\n## Café notes\n\nMore."`
  - Read it. The default is Markdown with front matter, so another application can consume it; `--json` gives the fields instead.

    `notriosctl notes show --document <note>`
  - List its headings. The anchors are the ones a notrios:// link resolves against, because the outline comes from the same parse that stores them.

    `notriosctl notes outline --document <note>`
  - List what is attached to it. A note with nothing attached reports an empty list rather than failing.

    `notriosctl notes resources --document <note>`
  - List the links out of it. Broken links are reported rather than filtered, because a broken link is the interesting one.

    `notriosctl notes links --document <note>`
**See which tags exist, and rename a branch of them** — Find the tags in a library, ask about one, and rename a branch without touching the notes by hand. Tags nest with `/`. A rename is a dry run until `--apply`, and `--include-children` is what makes it a branch rather than one tag.
  - Write a note and tag it, including a nested tag.

    `notriosctl notes create --title Shopping --body list`
  - Attach a tag.

    `notriosctl tags add --document <note> --tag shopping`
  - And one under it.

    `notriosctl tags add --document <note> --tag shopping/mall`
  - See every tag with how many notes carry it.

    `notriosctl tags list`
  - Narrow to one branch. `shopping` matches `shopping/mall` and never `shoppingcart`, because the separator is what makes a branch.

    `notriosctl tags list --prefix shopping`
  - Ask about one tag. It exits 1 when there is no such tag, so a script can test for one without parsing anything.

    `notriosctl tags show --tag shopping`
  - Rename the branch. Without `--apply` this reports what it would do and changes nothing.

    `notriosctl tags rename --from shopping --to errands --include-children`
  - Do it.

    `notriosctl tags rename --from shopping --to errands --include-children --apply`
**Make a note from a template, and find what is left to do** — List the templates a library holds, create a note from one, and see the checkbox items across notes. A missing placeholder is refused rather than blanked: a template that quietly produced `{{name}}` in a note would be worse than one that would not run.
  - Write a note carrying a template block.

    `notriosctl notes create --title "Weekly review" --body "```note-template\nid: weekly\nplaceholders: [week]\n```\n\n# Week {{week}}\n\n- [ ] read the inbox\n- [x] tidy the desk\n"`
  - Tag it, because `tasks list` reads checkbox items from notes tagged `task` or `todo` rather than from every note.

    `notriosctl tags add --document <note> --tag todo`
  - List the templates and their placeholders.

    `notriosctl templates list`
  - See the checkbox items across the library, with open and done counts.

    `notriosctl tasks list`
**See the shape of the link graph, and take it elsewhere** — Report which notes are hubs and which are orphans, then export the graph for another tool. A ranked list reads the same at any library size; a global force-directed canvas does not, which is why the report is a list and the canvas is a local view in the GUI.
  - Write a note to link to.

    `notriosctl notes create --title Target --body "the end"`
  - And one that links to it.

    `notriosctl notes create --title Source --body "See [it](document://default/documents/{note})."`
  - Report the shape: how many notes, how many links, which are isolated and which are orphans.

    `notriosctl graph report`
  - Export nodes and edges as CSV, for Gephi, Cytoscape, NetworkX or igraph.

    `notriosctl graph export <out>`
**Get a stable link to a note or one of its sections** — Produce a link that survives a rename, find the anchors a note offers, and resolve one back. A stable link names identity rather than a path, so moving or renaming a note does not break it. Heading anchors are written bare; block anchors keep the caret.
  - Write a note with a heading to anchor at.

    `notriosctl notes create --title "Reed beds" --body "Seen at dusk.\n\n## At dawn\n\nAlso."`
  - Print the stable link for the whole note.

    `notriosctl link <note>`
  - See the anchors it offers. These are the names a `#section` link can use.

    `notriosctl link --list-anchors <note>`
  - Anchor the link at one of them.

    `notriosctl link --anchor at-dawn <note>`
**Make a notebook whose contents come from a query** — Create a saved search that appears in the sidebar as a notebook, and see it beside the ordinary ones. Nothing is filed into a query notebook: what is in it is whatever matches, which is why it has no parent.
  - Write a note and tag it, so the query has something to match.

    `notriosctl notes create --title "Buy milk" --body "- [ ] milk"`
  - Tag it.

    `notriosctl tags add --document <note> --tag todo`
  - Make a notebook defined by a query rather than by what you file into it.

    `notriosctl notebooks create --name Todo --query tag:todo`
  - See it beside the ordinary notebooks, the way the sidebar shows them together.

    `notriosctl notebooks list`
**Watch a long import, and read what it did** — Start an import that records a job, watch it to completion, and read the record afterwards. `jobs status --wait` exits 0 succeeded, 1 failed, 3 running, 4 cancelled, 5 no such job, 6 interrupted, so a script can branch on the outcome without parsing.
  - Import a Joplin export. Long imports record a job rather than only printing at the end.

    `notriosctl import joplin-raw <joplin>`
  - List the jobs this library has run.

    `notriosctl jobs list`
  - Read one in detail, including the command that started it.

    `notriosctl jobs list --limit 1`
**Move a pre-0.8 library into the resolved locations** — See what a migration would move before it moves anything. A dry run is the whole point here: this relocates the directories the program uses, and it backs up and verifies before it moves anything. `--json` reports the same answer for a script, including when there is nothing to migrate.
  - Ask what a migration would do. Nothing is moved, and the report names every path on both sides.

    `notriosctl migrate --dry-run --json`
**See what localizing remote images would do, before it fetches anything** — Find the remote images a note points at and see the policy decision for each, without a byte being downloaded. A dry run performs no network I/O at all -- not even DNS. Every URL is reported as localized, blocked, needing review, or failed, so the policy is visible before it is exercised.
  - Write a note pointing at a remote image.

    `notriosctl notes create --title Remote --body "An image: ![sky](https://example.com/sky.png)"`
  - Ask what localizing would do. Nothing is fetched; a URL no domain rule matched is held for review rather than downloaded on the strength of a default.

    `notriosctl localize --dry-run <note>`
**Review what a publication would contain, then publish exactly that** — Save a publication profile, read the plan it produces, and publish only after the review still matches. `publish run` re-plans and refuses unless the digest of the reviewed plan still matches, so a library that changed between the review and the run stops rather than publishing something nobody read.
  - Write a note to publish, and one to keep back.

    `notriosctl notes create --title "Public note" --body "for the world"`
  - Tag it so a profile can select it.

    `notriosctl tags add --document <note> --tag public`
  - Save a profile: what to publish, and what to do with links that leave the selection.

    `notriosctl publish profile save --name site --tags public --link-action plain_text`
  - List the profiles this library holds.

    `notriosctl publish profile list`
  - Read the plan. This is a read-only privacy review: it changes nothing and prints the digest the run will be held to.

    `notriosctl publish plan --profile site`
<!-- notrios:generated:user:the-journeys:end -->


## Where these come from

Each journey above is run against a throwaway library before this page is
published, and checked by looking at what changed rather than at whether the
command exited cleanly. A command can succeed and do nothing, and it can succeed
and do the opposite of what a page said.

If a step here does not work for you, that is a bug in Notrios or in this page,
and worth reporting either way.
