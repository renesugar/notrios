# What Notrios can do

This page answers one question: what can you actually do with this application?

It is written against a machine-checked list. Every command, every REST
operation and every MCP tool Notrios offers is claimed by exactly one of the
capabilities below, and a build check fails if any of them stops being claimed —
so a capability cannot quietly exist without appearing here. What that check
cannot do is decide whether the description is any good; that part is written by
a person and is worth telling us about when it is wrong.

Read this page to find out *whether* Notrios does a thing. The
[CLI guide](cli.md), the [GUI guide](gui.md), the [REST API](api/rest.md) and
the [MCP tools](api/mcp.md) tell you *how*.

## Not every capability is on every surface

Notrios has four surfaces and they are not equivalent, on purpose.

The **command line** owns anything that touches this machine: importing a
directory of somebody else's export, writing an archive, taking a snapshot,
registering a URL handler, moving a library between locations, and choosing
where sync keys are kept. A web request should not be able to read an arbitrary
path on your disk, so those capabilities do not appear over HTTP.

The **GUI** owns writing. Creating and editing notes, searching and reading, and
the sync center's review screens are where a person works.

**REST** and **MCP** are how other programs work with a library — including an
AI assistant, within a scope you grant.

Where a capability is missing from a surface, the entry below says so. One of
those gaps is not deliberate: **tagging a note is available over REST and MCP
and from neither the command line nor the GUI.** That is recorded as a gap
rather than a design, and it is why this page exists in the form it does.

## What you can do
<!-- notrios:generated:user:what-you-can-do:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docfeatures#Registry -->
Registry is the feature catalogue: what a person can do with Notrios, and
which surfaces offer each capability.

- Write and edit notes — Create a note, change it, add to either end of it, and delete it. Deleting moves a note to Trash, and Trash is a place you can look in and take things back out of, not a countdown. (CLI 1, REST 10, MCP 6, GUI 6) The command line writes a note and files it, and does no more than that: editing, appending and deleting happen in the GUI or over REST/MCP. `notes create` exists so a note can be the end of a pipeline.
- Read a note and its structure — Fetch a note whole, or just its body, its outline, its blocks, a line range, or an earlier revision. The structured views exist so a tool can work on part of a note without re-parsing all of it. (REST 7, MCP 5, GUI 2) No command line: reading a note is what the GUI and the API are for.
- Search your notes — Find notes by text, tag, notebook, date and the rest of the query language, across the library or within one note. (REST 3, MCP 2, GUI 1) No command line search command; `notriosctl export archive --query` applies the same language to an export instead.
- Notebooks that are really saved searches — Save a query as a notebook, so a view that would otherwise be retyped becomes something you open. (CLI 1, REST 3, MCP 1, GUI 1) Made with `notebooks create --query` at the command line. In the GUI it is an action on a search that has already run and returned something, rather than a form: the query is one you have watched work, and a notebook made from an unrun query opens empty as easily as it opens right. The query is shown when keeping it and is not editable there, which is what keeps this from being a query form in another place.
- Organise notes into notebooks — Make notebooks, nest them, move notes between them, and see what a notebook deletion would take with it before agreeing to it. (CLI 3, REST 9, MCP 4, GUI 2)
- Tag and untag a note — Attach a tag to a note, take one off, and see what a note carries. Tags are hierarchical: `field/dusk` sits under `field`. (CLI 3, REST 2, MCP 2, GUI 4)
- See and rename tags — List the tags in a library, see a note's tags, and rename a whole tag hierarchy at once. (CLI 1, REST 3, MCP 1, GUI 4) Renaming is offered beside the tag, because it is decided while looking at the tag and the count next to it is half the reason for deciding. The dry run is not a prediction: the service performs the rename in a transaction and rolls it back, so the report and the apply cannot disagree. Applying is unreachable until a report of that exact rename has been shown, and editing the name or the children option withdraws it.
- Group libraries into collections — Collections sit above notebooks and are how an import keeps its material together. (REST 4, MCP 1) No command line or GUI journey; imports set the collection with --collection.
- Attach and manage files — Add files to notes, read them back, see what references what, and find attachments nothing points at any more. (CLI 1, REST 8, MCP 2, GUI 2) Measured by the H15 control crawl, which found the upload field in the note inspector and the Attachments tab. This was recorded as absent from the GUI until the crawl opened a note; no journey covers it yet.
- Bring remote images into the library — Find images a note points at on the web, check them against the domain policy, and copy the allowed ones in so the note stops depending on somebody else's server. (CLI 1, REST 4, MCP 3, GUI 2) Measured by the H15 control crawl on a note seeded with an image on an allowed domain. The earlier note here said localizing was a maintenance action rather than an editing one; it is in the editor.
- Link notes to each other — Get a stable link to a note or one of its sections, open one, check the links in a draft before saving, and register Notrios as the handler for notrios:// links. (CLI 3, REST 4, MCP 1, GUI 1)
- See the shape of the link graph — Look at what surrounds a note, find a path between two notes, and get a report on orphans, isolates and hubs. (CLI 2, REST 4, MCP 3, GUI 1)
- Templates and tasks — Keep note templates and make new notes from them, and see the tasks across a library. (REST 4, MCP 3) No command line or GUI journey.
- Live query blocks inside a note — Put a query in a note and have its results render where it sits. (REST 1, MCP 1, GUI 1) No command line. The GUI renders them where they sit: the preview finds each ```note-query block, runs it through the same parser the search box uses, and fills the block with the result. This row read as a gap for as long as the control crawl was the only measurement, and the crawl cannot see it and never could -- a query block renders content, not a control, and the crawl enumerates interactive elements. Its evidence is web/src/note-query.ts and its tests instead.
- Do many organiser operations at once — Send a batch of moves, tags and notebook changes as one request, so a large reorganisation is one reviewable action. (REST 1, MCP 1) No command line or GUI journey; batching is for tools.
- Import from another application — Bring in a Joplin raw export, an Obsidian vault, a Twitter or X archive, a ChatGPT or Claude export, or a Notrios archive. Every importer dry-runs first. (CLI 6, GUI 3) In the desktop app only, through the native bridge: these operations name a folder on the machine running the library, and no REST route starts them. In a browser the Import/Export control is shown and disabled with that reason rather than hidden. Joplin and Obsidian can be scanned first without writing. Importing from Notrios verifies the archive without opening a database, and merges only: replace, fork and adopt stay on the command line because they change which library this is. Twitter, ChatGPT and Claude imports remain command line only.
- Export your library — Write a portable archive of everything or a chosen subset, check one for compatibility, verify one, and restore one with an explicit intent. (CLI 5, GUI 2) In the desktop app only, through the native bridge: these operations name a folder on the machine running the library, and no REST route starts them. In a browser the Import/Export control is shown and disabled with that reason rather than hidden. The GUI writes a complete archive; exporting a subset by notebook, tag or query stays on the command line, because choosing a subset means seeing what it selects first.
- Back up and restore the whole library — Take a physical snapshot of the database and its attachments, verify it, and restore it — replacing this library or adopting the snapshot as a new replica. (CLI 3, GUI 1) In the desktop app only, through the native bridge: these operations name a folder on the machine running the library, and no REST route starts them. In a browser the Import/Export control is shown and disabled with that reason rather than hidden. Creating a snapshot is in the GUI; restoring one is not, because a restore replaces the library the window is showing.
- Pair two of your own libraries — Enrol a library for synchronization, issue a single-use code, and spend it from the other side so the two learn each other's keys. (CLI 6, REST 8, GUI 1)
- Synchronize with a replica — Exchange changes with a paired replica, directly over an authenticated connection or through a folder you both can reach — a second drive, or a cloud folder mapped locally. (CLI 3, REST 9, MCP 3, GUI 1)
- See and end trust between replicas — List the replicas a library trusts, revoke one's key, and retire a peer after reviewing exactly what retiring it means. (CLI 1, REST 4, GUI 2)
- Recover a replica and resolve conflicts — Fetch a peer-verified backup, inspect one without installing it, work through sync conflicts, and decide what a shared attachment should do. (CLI 1, REST 9, MCP 1, GUI 2)
- Choose where sync keys are kept — An installed Notrios keeps the key that protects your sync material in the operating system's credential store. Move existing keys between that and the owner-only development file, in either direction. (CLI 1) Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- Watch and steer long-running work — Imports, exports and syncs run as jobs you can list, inspect, cancel, retry and reset. (CLI 5, REST 5, MCP 4, GUI 3) Measured by the H15 control crawl once jobs were seeded, which is what the row had been waiting for: every job control is gated on state, so an interface holding only completed jobs shows none of them. What the GUI has is the sync centre's list of recent *sync* jobs, with Cancel on one still running or queued and Retry on one that failed or was cancelled. Import, export and snapshot jobs appear nowhere: the sync centre lists four sync kinds and only those, and the import dialog shows a final report rather than a job. There is no general job list, no inspect and no reset.
- Keep separate libraries — Run more than one library on a machine — personal notes, work notes, a blog — each with its own database, its own settings and its own address. (CLI 7, GUI 1) Switching between profiles is in the GUI; creating, registering and forgetting them is not. A profile registry is about this machine, which the desktop app also runs on, so the rest is a gap rather than a boundary; the browser mode is the part that genuinely cannot manage it.
- Publish a subset of your notes — Choose what leaves the library, review exactly what a publication would include and withhold, save that choice as a profile, and publish only after agreeing to the reviewed plan. (CLI 4, REST 1, MCP 1) Not in the GUI yet. Publishing writes to a local directory, so the desktop app could do it and the browser mode could not.
- Keep a library healthy — Find what has rotted, fix what can be fixed mechanically, reclaim space, and check that this installation is set up the way you think it is. (CLI 8, REST 4, MCP 1, GUI 3) The GUI reports both, and repairs. Repairing goes through the native bridge because the lint surface is deliberately read-only over REST -- that is a remote, programmatic surface an agent reaches, and repairs to somebody's notes should not be one call away from one -- which says nothing about a person pressing a button in the program that owns the library. Each repair writes a revision against a required base revision, so a note edited since the plan refuses rather than being repaired against text nobody read, and refusals are reported per note. Collecting stays `notriosctl gc --apply`: it deletes blobs, and there is no revision to go back to. doctor, paths, config show and seed-help remain command line.
- Move a pre-0.8 library into place — A library that lived in ./data next to the program is relocated into the directories an installed Notrios uses, after showing you the plan. (CLI 1) Command line only, and genuinely so: it relocates the directories the running program uses, which is not something the program can sensibly do to itself while serving them.
- Let an AI assistant use your library — Notrios speaks MCP, so an assistant can read and, within a scope you grant, change your notes. (REST 2) The endpoint itself has no command line or GUI; the tools it exposes are listed against the features above.
<!-- notrios:generated:user:what-you-can-do:end -->


## Where the surfaces disagree
<!-- notrios:generated:user:where-the-surfaces-disagree:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/doccompare#Lines -->
Each capability below is offered on some surfaces and not others.

- Do many organiser operations at once — neither the command line nor the GUI; reachable only over REST or MCP. No command line or GUI journey; batching is for tools.
- Group libraries into collections — neither the command line nor the GUI; reachable only over REST or MCP. No command line or GUI journey; imports set the collection with --collection.
- Let an AI assistant use your library — neither the command line nor the GUI; reachable only over REST or MCP. The endpoint itself has no command line or GUI; the tools it exposes are listed against the features above.
- Templates and tasks — neither the command line nor the GUI; reachable only over REST or MCP. No command line or GUI journey.
- Publish a subset of your notes — command line only. Not in the GUI yet. Publishing writes to a local directory, so the desktop app could do it and the browser mode could not.
- Choose where sync keys are kept — command line only. Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- Move a pre-0.8 library into place — command line only. Command line only, and genuinely so: it relocates the directories the running program uses, which is not something the program can sensibly do to itself while serving them.
- Live query blocks inside a note — GUI only. No command line. The GUI renders them where they sit: the preview finds each ```note-query block, runs it through the same parser the search box uses, and fills the block with the result. This row read as a gap for as long as the control crawl was the only measurement, and the crawl cannot see it and never could -- a query block renders content, not a control, and the crawl enumerates interactive elements. Its evidence is web/src/note-query.ts and its tests instead.
- Read a note and its structure — GUI only. No command line: reading a note is what the GUI and the API are for.
- Search your notes — GUI only. No command line search command; `notriosctl export archive --query` applies the same language to an export instead.
- Attach and manage files — both surfaces, and neither journey is written yet
- See the shape of the link graph — both surfaces, and neither journey is written yet
- Watch and steer long-running work — both surfaces, and neither journey is written yet
- Link notes to each other — both surfaces, and neither journey is written yet
- Bring remote images into the library — both surfaces, and neither journey is written yet
- See and end trust between replicas — both surfaces, and neither journey is written yet
- Recover a replica and resolve conflicts — both surfaces, and neither journey is written yet
- Export your library — both, but only the command-line journey is written
- Import from another application — both, but only the command-line journey is written
- Keep separate libraries — both, but only the command-line journey is written
- Back up and restore the whole library — both, but only the command-line journey is written
- Synchronize with a replica — both, but only the command-line journey is written
- Pair two of your own libraries — both, but only the command-line journey is written
- See and rename tags — both, but only the GUI journey is written
- Notebooks that are really saved searches — both, but only the GUI journey is written
<!-- notrios:generated:user:where-the-surfaces-disagree:end -->

This list is computed, not written. It compares what each capability claims
against which journeys exist, and reports two different kinds of disagreement.

A **capability** difference is a thing one surface can do and the other cannot.
Most are deliberate and say why beside them. One is not, and it is called out in
bold: tagging a note is reachable over REST and MCP and from neither the command
line nor the GUI — the capability exists and you cannot reach it without
writing a program.

A **documentation** difference is a capability both surfaces offer where only
one has a journey written. Nothing is missing from Notrios there; something is
missing from these pages.


## How this page is kept honest

The list above is generated from `docs/docfeatures/FEATURES.json`, which records
each capability's title and summary alongside the exact surfaces that offer it.
Two checks run against it.

The first is coverage: every CLI usage form, REST operation and MCP tool must be
claimed by some feature. A new capability added without an entry here fails the
build, because a capability nobody can discover is a documentation defect even
when every other gate is green.

The second runs the other way: a feature may not claim a surface that does not
exist. That catches a page describing something that was removed — which no
consistency check can see, because such a page is perfectly consistent with
itself. It caught three invented endpoints the first time it ran.
