# GUI journeys

The [GUI guide](gui.md) describes what the GUI has in it. This page shows
you doing things with it: a task, the steps, and a picture of each step with the
place to click marked.

The same tasks are shown for the command line in
[Command-line journeys](journeys-cli.md). Where a task can only be done on one
of the two, both pages say so.

## Reading the pictures

Most screenshots carry two marks. The **thin outline** is the thing you are
clicking — how much of it is clickable. The **red circle** is the point the
click lands on.

The journeys marked **desktop app only** have neither, and not by oversight:
they are driven from the keyboard, and a keystroke has no place on the screen to
point at. They also cannot be photographed in a browser at all — importing,
exporting, snapshots and publishing name a folder on the computer running the
library, so those controls are disabled there — which is why the pictures for
them come from the real application instead.

The library in these pictures is a throwaway one made for the purpose. None of
it is anybody's notes.

## What the GUI does not do
<!-- notrios:generated:user:what-the-gui-does-not-do:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docfeatures#(Registry).WithoutGUILines -->
Everything here is command-line or API work today. Where a capability is
absent for a reason rather than for want of doing it, the reason is given.

**Let an AI assistant use your library** — Notrios speaks MCP, so an assistant can read your library and, within a scope you grant, change it. What it may touch is yours to decide, and establishing trust between replicas and managing credentials are outside every scope. This is a surface Notrios serves rather than something you run: software with an MCP client connects to it, so there is nothing to demonstrate at a terminal or in a window — a journey would document the client rather than this program. The two REST operations listed here are how the endpoint is reached.
**Remove your data, on purpose** — Delete this library's notes, configuration, state and cache, after a backup that is written and verified before anything is removed. It shows you what it would delete before it deletes it, tells you where the backup is, and refuses rather than guessing when nothing can answer the question. Removing the program is a separate act -- `apt remove` or `make uninstall` -- and both leave your notes exactly where they are. Command line only, deliberately. There is no REST or MCP surface for deleting a library: those exist to be called by something else, and the one operation that cannot be undone should be reached by a person who typed it.
**Choose where sync keys are kept** — An installed Notrios keeps the key protecting your sync material in the operating system's credential store. Move existing keys between that and the owner-only development file, in either direction. Credentials are managed from the command line and nowhere else, firmly: a remote route that can move a key is a remote route that can take one. An installed Notrios never silently falls back to a plaintext file, and nothing here prints, logs or exports a secret.
**Templates and tasks** — Keep notes that are shapes for other notes, make new ones from them, and see the tasks scattered across a library in one list. Both are ordinary Markdown rather than tables in a database: a template is a note carrying a note-template block, and a task is a checkbox line in a note tagged `task` or `todo`. So nothing creates a task — writing `- [ ] chase the permit` in a note is how one comes to exist — and what the command line adds is the other half: asking what remains, and filling in a template. A placeholder you leave out is refused rather than quietly left blank, so a template that gains a field fails the scripts that do not know about it. The tag is required because a checkbox is ordinary Markdown and turns up in quoted examples; `--untagged` asks for those too. Not in the interface yet.
**Move a pre-0.8 library into place** — A library that lived in a `./data` folder next to the program moves into the directories an installed Notrios uses — after showing you the plan. This one is genuinely command-line work: it relocates the directories the running program is using, which is not something a program can sensibly do to itself while serving them.
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

**Find your way around** — See what the GUI is made of before changing anything in it.
  - The sidebar on the left lists your notebooks. Help is one of them: the documentation is seeded into your library as ordinary read-only notes, so you can search it alongside everything else.

    ![the-sidebar](images/journeys/find-your-way-around-the-sidebar.png)
**Write a note, and read it back** — Create a note and get something into it.
  - Click New note. The note is created immediately and opens for editing; there is no dialog to fill in first.

    ![new-note](images/journeys/write-a-note-new-note.png)
  - Type into the editor. What you write is Markdown, and the preview beside it renders as you go.

    ![the-editor](images/journeys/write-a-note-the-editor.png)
**Choose which notebook a note goes in** — File a note somewhere other than where it landed.
  - Start from a new note.

    ![new-note](images/journeys/choose-a-notebook-new-note.png)
  - Open the notebook picker. It sits with the note rather than in a menu, because which notebook a note belongs to is part of the note.

    ![notebook-picker](images/journeys/choose-a-notebook-notebook-picker.png)
**Tag a note** — Put a tag on the note you are writing, and take one off.
  - Start from a note. Tags belong to a note, so there has to be one open.

    ![new-note](images/journeys/tag-a-note-new-note.png)
  - Open the tag control. It sits in the editor toolbar beside the notebook control, because both answer where this note belongs and both belong where you are typing. The label shows how many tags the note carries.

    ![open-tags](images/journeys/tag-a-note-open-tags.png)
  - Type the tag. Tags are hierarchical, so `field/dusk` sits under `field`. Press Enter or use Add.

    ![type-a-tag](images/journeys/tag-a-note-type-a-tag.png)
  - Add it. Each tag then appears as a chip with its own remove button, rather than as a comma-separated line you have to edit carefully.

    ![add-it](images/journeys/tag-a-note-add-it.png)
**Link one note to another** — Put a link to another note into the one you are writing, without leaving it to find the address.
  - Open the note you want to link from. Everything that follows is in this note; the one you link to is never opened.

    ![open-the-note](images/journeys/insert-a-link-open-the-note.png)
  - Note info holds what the note is rather than what it says: where it came from, what it links to, what is attached to it, and the tools for changing those. It is closed by default because a note you are reading is usually a note you are reading.

    ![open-note-info](images/journeys/insert-a-link-open-note-info.png)
  - Type part of the other note’s title. The suggestions come from a search over titles, so you do not need the identifier and you do not need the note open in another window.

    ![find-the-other-note](images/journeys/insert-a-link-find-the-other-note.png)
  - Choosing a suggestion writes a stable `document://` link into the body at the cursor. It is stable in the sense that matters: it names the note, so renaming the note later does not break it.

    ![choose-it](images/journeys/insert-a-link-choose-it.png)
  - The link is in the body and the note is unsaved — the badge above the editor says so. Nothing was written for you: move the link to where it belongs in your prose first, then save.

    ![save-when-ready](images/journeys/insert-a-link-save-when-ready.png)
**Change a note you already wrote** — Open an existing note, change it, and save the change.
  - All notes is every note in the library, whichever notebook it is filed in. Start here when you know what a note is called but not where you put it.

    ![open-all-notes](images/journeys/update-a-note-open-all-notes.png)
  - Click a note in the results list to open it. The editor fills with the note as it stands; nothing is changed by opening it.

    ![open-the-note](images/journeys/update-a-note-open-the-note.png)
  - The title is an ordinary field at the top of the editor. Change it the way you would change any text.

    ![change-the-title](images/journeys/update-a-note-change-the-title.png)
  - Saving writes a new revision rather than overwriting the old one, so the version you just replaced is still there.

    ![save-it](images/journeys/update-a-note-save-it.png)
**Delete a note** — Move a note to the Trash and confirm it arrived there.
  - Find the note first. Deleting is something you do to an open note, not to a row in a list, so that it is always clear which note is about to go.

    ![open-all-notes](images/journeys/delete-a-note-open-all-notes.png)
  - Open the note you mean to delete and read it once. This is the last screen on which its text is in front of you.

    ![open-the-note](images/journeys/delete-a-note-open-the-note.png)
  - Deleting asks you to confirm, and names the note it is about to move so that a mis-clicked row cannot become a deleted note. Deleting moves it to the Trash; it is not destroyed, and nothing here empties the Trash on a timer.

    ![delete-it](images/journeys/delete-a-note-delete-it.png)
  - Trash is a place you can open and look inside, not a countdown. The note is in it.

    ![look-in-the-trash](images/journeys/delete-a-note-look-in-the-trash.png)
**Take a note back out of the Trash** — Restore a deleted note and confirm it is back among your notes.
  - Open the Trash to see what is in it. Everything here was deleted and none of it is gone.

    ![open-the-trash](images/journeys/restore-a-note-open-the-trash.png)
  - Open the note you want back. A trashed note opens read-only, with a badge saying why.

    ![open-the-trashed-note](images/journeys/restore-a-note-open-the-trashed-note.png)
  - Restoring returns the note to the notebook it was filed in, not to some general inbox.

    ![restore-it](images/journeys/restore-a-note-restore-it.png)
  - All notes again, to see it where it belongs.

    ![back-to-all-notes](images/journeys/restore-a-note-back-to-all-notes.png)
**Search for a note** — Find notes by what they say, using the same query language the command line takes.
  - The search box takes the query language, not just a word. Bare words are combined with an implicit AND; an uppercase OR widens the search instead.

    ![type-a-query](images/journeys/search-your-notes-type-a-query.png)
  - Run the search. Results are ordered by relevance and page as you scroll, so a large library does not have to be loaded to be searched.

    ![run-it](images/journeys/search-your-notes-run-it.png)
**Narrow a search by tag, and exclude what you do not want** — Use a field query and a negation together, which is what most real searches turn out to be.
  - tag: matches a tag rather than the note's text, and a leading minus excludes. Here that is everything tagged field/dusk except the documentation.

    ![type-a-field-query](images/journeys/search-by-tag-and-exclude-type-a-field-query.png)
  - The same box runs it. Every query-language feature works here exactly as it does in notriosctl search, because it is the same parser.

    ![run-it](images/journeys/search-by-tag-and-exclude-run-it.png)
**Keep a search as a notebook** — Turn a search you have just watched work into a notebook in the sidebar.
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
**Rename a tag, and everything under it** — Change a tag's name across every note that carries it, after seeing what that will do.
  - Tags are listed in the sidebar with the number of notes carrying each one. The pencil beside a tag renames it; the name itself still runs a search for it.

    ![find-the-tag](images/journeys/rename-a-tag-find-the-tag.png)
  - A rename reaches every note carrying the tag. Leaving “rename everything under it” ticked also moves the tags nested below this one.

    ![type-the-new-name](images/journeys/rename-a-tag-type-the-new-name.png)
  - Nothing has changed yet. The report is not a guess: the service performs the rename inside a transaction and rolls it back, so what you are reading and what applying would do cannot disagree. A change described as a merge means the destination already exists and the two tags become one.

    ![see-what-changes](images/journeys/rename-a-tag-see-what-changes.png)
  - Rename applies exactly what the report described. Editing the name or the children option first withdraws the report, so you cannot agree to one rename and apply another.

    ![agree-to-it](images/journeys/rename-a-tag-agree-to-it.png)
**Check what has rotted, and repair what can be** — See the library's problems, repair the mechanical ones, and know why the rest are commands.
  - Health reads two reports about this library. Nothing on the screen changes anything: both reports are read-only, and one of them says so in the API itself.

    ![open-health](images/journeys/check-library-health-open-health.png)
  - Lint finds broken links, unresolved anchors, notes without titles, remote media nobody has localized, attachments nothing points at. Counts are for the whole library; where a list is capped it says so, because a capped list read as the whole problem undercounts it.

    ![what-has-rotted](images/journeys/check-library-health-what-has-rotted.png)
  - The collector plans, and never deletes. Deleting is notriosctl gc --apply, and the absence of a button here is deliberate: reclaiming space should be an explicit local act rather than something a window does.

    ![what-could-be-reclaimed](images/journeys/check-library-health-what-could-be-reclaimed.png)
  - Some findings can be corrected without judgement. Planning shows how many edits across how many notes and changes nothing; repairing applies them, each against the revision it was computed from, so a note you edited in the meantime refuses rather than being repaired against text nobody read. Collecting unreferenced attachments is not offered here: it deletes, and a repair only writes a new revision.

    ![what-can-be-repaired](images/journeys/check-library-health-what-can-be-repaired.png)
**Import a Joplin export (desktop app only)** — Bring a Joplin RAW export into this library from the desktop app.
  - Open **File ▸ Import and export…**, or press Ctrl+I. Everything that reads or writes a folder on this machine is here, and it is here only in the desktop app: a browser cannot open a folder, so these controls are shown there and disabled with that reason rather than hidden.

    ![open-import-and-export](images/journeys/import-from-joplin-in-the-app-open-import-and-export.png)
  - Name the Joplin RAW export folder — the one Joplin wrote, with a Markdown file per note. **Choose folder…** opens a directory chooser if you would rather point at it. Nothing below is available until a folder is named.

    ![name-the-folder](images/journeys/import-from-joplin-in-the-app-name-the-folder.png)
  - Press **Import**, and the report says what arrived. **Scan without importing** sits beside it and writes nothing, which is the safer first move on an export you have not opened. Imported notes are ordinary editable notes: a migration, not a read-only attachment.

    ![the-report](images/journeys/import-from-joplin-in-the-app-the-report.png)
**Export the whole library (desktop app only)** — Write a portable archive of everything, from the desktop app.
  - Open **File ▸ Import and export…**, or press Ctrl+I. Everything that reads or writes a folder on this machine is here, and it is here only in the desktop app: a browser cannot open a folder, so these controls are shown there and disabled with that reason rather than hidden.

    ![open-import-and-export](images/journeys/export-this-library-open-import-and-export.png)
  - Name the folder to write the archive into, and let it be an empty one: an archive is a directory of files rather than a single file.

    ![name-the-destination](images/journeys/export-this-library-name-the-destination.png)
  - Press **Export everything**, and the report names what was written. Exporting a subset — by notebook, tag or query — stays on the command line, because choosing a subset means seeing what it selects before it is written. The same dialog reads an archive back: **Import from Notrios** verifies one without opening a database.

    ![the-archive](images/journeys/export-this-library-the-archive.png)
**Take a snapshot of the library (desktop app only)** — Write a verified snapshot image you can restore from.
  - Open **File ▸ Import and export…**, or press Ctrl+I. Everything that reads or writes a folder on this machine is here, and it is here only in the desktop app: a browser cannot open a folder, so these controls are shown there and disabled with that reason rather than hidden.

    ![open-import-and-export](images/journeys/take-a-snapshot-open-import-and-export.png)
  - Name the folder to write the snapshot into. A snapshot is the whole library at one moment, and it is the fastest way to copy one that is going back into the same schema.

    ![name-the-destination](images/journeys/take-a-snapshot-name-the-destination.png)
  - Press **Create snapshot**. The image is verified as it is written, and one that fails verification is never recorded as a snapshot. Restoring one is a command-line operation, because a restore replaces the library this window is showing.

    ![the-snapshot](images/journeys/take-a-snapshot-the-snapshot.png)
**Publish a subset of your notes (desktop app only)** — Review exactly what a publication would let out, then write it.
  - Open **File ▸ Import and export…**, or press Ctrl+I. Publishing is at the bottom, under the operations that read and write whole libraries, because it is the one that hands notes to somebody else.

    ![open-import-and-export](images/journeys/publish-a-subset-open-import-and-export.png)
  - Choose a saved profile and press **Review what this would publish**. The counts lead with what is withheld and which links are rewritten, because that is the question — a publication carries current versions only, no trashed notes, no history and no provenance. Profiles are made with `notriosctl publish profile save` and only read here: a form that quietly defaulted a privacy choice is how something private gets published.

    ![review-it](images/journeys/publish-a-subset-review-it.png)
  - Name an empty folder — the caret is already in the field, because naming one is what the review just asked for — and press **Publish this review**. The run re-plans and refuses unless the plan still hashes to the one on screen, so what is published is what was read, not something rebuilt a moment later from the same profile.

    ![publish-it](images/journeys/publish-a-subset-publish-it.png)
  - The report names how many notes were written and where. A publication is not a backup: restoring from one is not a thing you can do, which is why the archive above exists.

    ![the-publication](images/journeys/publish-a-subset-the-publication.png)
**See where a note came from** — Tell an imported note from one you wrote here, and find the rest of what came in with it.
  - Open a note that arrived in an import. Nothing about the card says where it came from — that is what Note info is for.

    ![open-an-imported-note](images/journeys/see-where-a-note-came-from-open-an-imported-note.png)
  - Note info carries what the note is rather than what it says.

    ![open-note-info](images/journeys/see-where-a-note-came-from-open-note-info.png)
  - Collection is where this note came from. It is shown only when there is something to say: a note written in this library carries the default collection, and a row repeating that on every note would say nothing. The notes are filed in an ordinary notebook — the collection is provenance beside it, not a place they live.

    ![read-the-collection](images/journeys/see-where-a-note-came-from-read-the-collection.png)
**Attach a file to a note** — Put a file into the library and keep it with the note it belongs to.
  - Open the note the file belongs with.

    ![open-the-note](images/journeys/attach-a-file-open-the-note.png)
  - Attachments are added from Note info, beside the rest of what the note is made of.

    ![open-note-info](images/journeys/attach-a-file-open-note-info.png)
  - Choose the file — this one is the Notrios icon, because a journey has to attach something real. It is copied into the library, so the note stops depending on where the file happened to live and a backup or a sync carries it along with everything else.

    ![choose-a-file](images/journeys/attach-a-file-choose-a-file.png)
  - The file is in the library and listed against the note — and a Markdown link to it has been written where your cursor was, which is why the note is now unsaved. The interface can place the link because it knows where you were typing; `notriosctl resources add` prints the `resource://` URI instead and never touches a body, because at a terminal nobody knows where the link belongs. The bytes are stored once and addressed by their content, so attaching the same file to a second note stores nothing twice.

    ![see-it-attached](images/journeys/attach-a-file-see-it-attached.png)
**Bring a note’s images into the library** — Find the images a note is loading from somebody else’s server, and see what Notrios is willing to copy in.
  - Open a note that points at an image on the web.

    ![open-the-note](images/journeys/bring-images-into-the-library-open-the-note.png)
  - Note info lists what the note depends on, including anything it is loading from elsewhere.

    ![open-note-info](images/journeys/bring-images-into-the-library-open-note-info.png)
  - Every remote image is listed with what the policy decided about it and why. This is a scan, not a download: nothing has been fetched, and the decision is made from the address alone.

    ![read-the-decisions](images/journeys/bring-images-into-the-library-read-the-decisions.png)
  - Localize copies in only the images the policy allows, through the quarantine that hashes and sniffs each one, and rewrites the note to point at the local copies. A note that has been localized still renders when the other server is gone.

    ![localize-the-allowed-ones](images/journeys/bring-images-into-the-library-localize-the-allowed-ones.png)
**See what a note connects to** — Follow a note’s links without reading it, and find your way to what it points at.
  - Open a note that links to others.

    ![open-the-note](images/journeys/see-what-a-note-connects-to-open-the-note.png)
  - Nearby notes lives in Note info, under everything else the note is made of.

    ![open-note-info](images/journeys/see-what-a-note-connects-to-open-note-info.png)
  - Notes are grouped by how far away they are: directly linked first, then anything reached through those. Hops widens the view — two hops shows what the neighbours point at.

    ![read-the-neighbourhood](images/journeys/see-what-a-note-connects-to-read-the-neighbourhood.png)
  - A neighbour opens like any other note. Nothing here is a separate graph screen; it is a way out of the note you are in.

    ![follow-one](images/journeys/see-what-a-note-connects-to-follow-one.png)
**Put a live query in a note** — Keep a list inside a note that fills itself in, instead of one you have to maintain.
  - Open a note with a query block in it. In the editor it is an ordinary fenced code block: three backticks, `note-query`, and a `query:` line in the same language the search box takes.

    ![open-the-note](images/journeys/put-a-live-query-in-a-note-open-the-note.png)
  - In the preview the block is replaced by what the query matches right now. Nothing is stored in the note but the query, so the list is never out of date and never needs editing.

    ![read-the-results](images/journeys/put-a-live-query-in-a-note-read-the-results.png)
  - Each result opens the note it names. A block is a way into your notes rather than a report about them.

    ![follow-a-result](images/journeys/put-a-live-query-in-a-note-follow-a-result.png)
**Choose where synchronization happens** — Point this library at a folder both machines can reach, and start a synchronization.
  - Everything about synchronization is behind one button: setup, pairing, the peers you trust, attachments, conflicts, backups and repairs.

    ![open-the-sync-centre](images/journeys/choose-where-sync-happens-open-the-sync-centre.png)
  - Setup is where a profile says how it reaches the other side.

    ![open-setup](images/journeys/choose-where-sync-happens-open-setup.png)
  - A shared directory is the simplest transport: a second drive, or a cloud folder mapped into this machine. Notrios writes its exchange into that folder and reads what the other library left there. The folder is a carrier, not a copy of your notes.

    ![choose-a-shared-directory](images/journeys/choose-where-sync-happens-choose-a-shared-directory.png)
  - The path is on the machine running this library. A browser cannot open a folder chooser, so the path is typed; the desktop application offers a chooser beside this field.

    ![name-the-folder](images/journeys/choose-where-sync-happens-name-the-folder.png)
  - Saving a transport does not enrol a peer and does not start a synchronization: it records where this profile will look, and takes effect when the profile restarts.

    ![save-it](images/journeys/choose-where-sync-happens-save-it.png)
  - The overview is where a synchronization is started and watched.

    ![back-to-the-overview](images/journeys/choose-where-sync-happens-back-to-the-overview.png)
  - Sync now queues the exchange as a job rather than blocking the window on it. What it sends is the operations this library has that the other one has not acknowledged; what it reads is the same from the other side.

    ![start-one](images/journeys/choose-where-sync-happens-start-one.png)
**Invite another device** — Issue a single-use code so a second library of yours can pair with this one.
  - Everything about synchronization is behind one button: setup, pairing, the peers you trust, attachments, conflicts, backups and repairs.

    ![open-the-sync-centre](images/journeys/invite-another-device-open-the-sync-centre.png)
  - Peers is where trust between libraries is established and ended.

    ![open-peers](images/journeys/invite-another-device-open-peers.png)
  - The label is for you, not for the other machine: it is how this peer will be listed here afterwards.

    ![label-it](images/journeys/invite-another-device-label-it.png)
  - The code is single-use and expires in fifteen minutes. Creating one enrols nobody: it is spent from the other side, and only then do the two libraries learn each other’s keys.

    ![create-a-code](images/journeys/invite-another-device-create-a-code.png)
  - Read the code to the other device over a different channel from the address — a code and an address travelling together are worth exactly as much as each other to anybody who intercepts them.

    ![carry-it-across](images/journeys/invite-another-device-carry-it-across.png)
**See who is on the carrier, without trusting anybody** — Find out which replicas have left something in the shared folder, and confirm that looking is not the same as trusting.
  - Everything about synchronization is behind one button: setup, pairing, the peers you trust, attachments, conflicts, backups and repairs.

    ![open-the-sync-centre](images/journeys/look-for-peers-without-enrolling-one-open-the-sync-centre.png)
  - Peers lists the libraries this one trusts. It starts empty, and it does not fill itself in.

    ![open-peers](images/journeys/look-for-peers-without-enrolling-one-open-peers.png)
  - No peers are enrolled. Pairing is the only thing that adds one, and it takes a code somebody carried across by hand.

    ![read-the-empty-list](images/journeys/look-for-peers-without-enrolling-one-read-the-empty-list.png)
  - Discovery reads what is in the shared folder and reports what it found. It enrols nobody — a replica that has written to the carrier is a candidate, not a peer, and becoming one is a decision you make.

    ![scan-the-carrier](images/journeys/look-for-peers-without-enrolling-one-scan-the-carrier.png)
**Make a backup only you can open** — Write an encrypted copy of this library that a password protects, and know what happens if you lose the password.
  - Everything about synchronization is behind one button: setup, pairing, the peers you trust, attachments, conflicts, backups and repairs.

    ![open-the-sync-centre](images/journeys/make-a-protected-backup-open-the-sync-centre.png)
  - Backup and recovery is where a copy is made and where one is reviewed before it is restored.

    ![open-backup](images/journeys/make-a-protected-backup-open-backup.png)
  - The password wraps a fresh key for this backup. It is used and forgotten — it is not stored, not remembered, and not recoverable.

    ![choose-a-password](images/journeys/make-a-protected-backup-choose-a-password.png)
  - Twice, because there is no way back from a typo here: Notrios cannot open a backup whose password it does not have.

    ![type-it-again](images/journeys/make-a-protected-backup-type-it-again.png)
  - The backup is verified as it is written, then downloaded. Restoring one is a separate act with its own review — creating a backup never replaces anything.

    ![write-it-out](images/journeys/make-a-protected-backup-write-it-out.png)
**Watch a synchronization, and retry one that stopped** — See what synchronization is doing, and start again from where it left off.
  - Everything about synchronization is behind one button: setup, pairing, the peers you trust, attachments, conflicts, backups and repairs.

    ![open-the-sync-centre](images/journeys/watch-a-synchronization-job-open-the-sync-centre.png)
  - Synchronization runs as jobs rather than as a window you have to keep open. The recent ones are listed with what each was doing and how it ended.

    ![read-the-jobs](images/journeys/watch-a-synchronization-job-read-the-jobs.png)
  - A job that failed or was cancelled can be started again. It resumes from its last verified checkpoint rather than from the beginning, so retrying a large transfer does not repeat the part that already arrived.

    ![retry-one](images/journeys/watch-a-synchronization-job-retry-one.png)
**See which library this window is showing** — Keep separate libraries on one machine and know which one you are looking at.
  - Everything about synchronization is behind one button: setup, pairing, the peers you trust, attachments, conflicts, backups and repairs.

    ![open-the-sync-centre](images/journeys/see-which-library-this-is-open-the-sync-centre.png)
  - The header names the profile this window is showing, with the library and this replica’s identifiers beneath it. Two profiles are two databases and two addresses; nothing is shared between them.

    ![read-the-identity](images/journeys/see-which-library-this-is-read-the-identity.png)
  - Switching between profiles is on the Setup tab, above the transport.

    ![open-setup](images/journeys/see-which-library-this-is-open-setup.png)
  - Each profile is a separate local process at its own address, so choosing one opens it there. Making, registering and forgetting profiles is command-line work: `profile create` names a directory to keep a library in and `profile start` launches a process, and neither is something a web request should be able to ask for.

    ![choose-a-profile](images/journeys/see-which-library-this-is-choose-a-profile.png)
**Reorganise several notes at once** — Tag, move or trash a set of notes in one action, instead of opening each one.
  - Every result carries a tick box. Ticking one starts a selection; the note it belongs to is not opened, because you are choosing notes rather than reading one.

    ![tick-the-first](images/journeys/act-on-several-notes-tick-the-first.png)
  - Tick as many as you want. Ctrl-click — Command on a Mac — does the same from the card itself, and shift-click takes everything between the last one you touched and this one.

    ![tick-another](images/journeys/act-on-several-notes-tick-another.png)
  - With more than one note chosen there is no single note to read, so the editor and the preview are replaced by the operations that apply to a set: move to a notebook, add or remove a tag, duplicate, restore, or move to the Trash. The notes you picked are listed above them.

    ![read-the-panel](images/journeys/act-on-several-notes-read-the-panel.png)
  - A tag to put on all of them. The same box removes one, because adding and removing a tag are the same choice made in two directions.

    ![type-a-tag](images/journeys/act-on-several-notes-type-a-tag.png)
  - One request, bounded at five hundred notes and refused rather than trimmed past it. It carries a key that makes a second click safe: if nothing appeared to happen and you press again, the answer comes from the first run rather than doing the work twice.

    ![apply-it](images/journeys/act-on-several-notes-apply-it.png)
  - Every note is reported, not just a total. A note that was skipped — it already carried the tag — is not a note that failed, and the two are counted separately so “nothing to do” cannot look like a fault.

    ![read-the-report](images/journeys/act-on-several-notes-read-the-report.png)
<!-- notrios:generated:user:the-journeys:end -->


## Where these come from

Every picture above was taken by driving the real GUI. Nothing is a
mockup, and nothing was placed by hand: each mark is drawn from where the thing
being clicked actually was, so if a button moves the mark moves with it.

If a step here does not match what you see, that is a bug in Notrios or in this
page, and worth reporting either way.
