# What Notrios can do

Notrios is a note application that keeps everything on your own machine: the
notes, their history, the files attached to them and the search index all live
in one SQLite database and one asset store you can copy, back up with ordinary
tools, and open years from now. There is no account and no server to sign into.
Nothing leaves the machine unless you export it, publish it, or synchronize it
with a replica you have paired yourself.

This page answers one question — what can you actually do with it? — and it is
written against a machine-checked list. Every command, every REST operation and
every MCP tool Notrios offers is claimed by exactly one of the capabilities
below, and the build fails if any of them stops being claimed, so a capability
cannot quietly exist without appearing here. What the check cannot decide is
whether the description is any good; that part is written by a person and is
worth telling us about when it is wrong.

Read this page to find out *whether* Notrios does a thing. The
[CLI guide](cli.md), the [GUI guide](gui.md), the [REST API](api/rest.md) and
the [MCP tools](api/mcp.md) tell you *how*, and the two journey catalogues —
[command line](journeys-cli.md) and [interface](journeys-gui.md) — show the
common tasks step by step.

## Four surfaces, and why they differ

The same library is reachable four ways, and they are deliberately not
equivalent.

The **desktop app** is where a person works: writing, reading, searching,
organising, and reviewing anything consequential before it happens. It is also
the only surface that can name a folder on this machine, which is why importing,
exporting, snapshots and publishing live there and nowhere else.

The **command line** owns everything that touches the machine itself or that
should be scriptable: importing somebody else's export, writing an archive,
taking a snapshot, registering the `notrios://` handler, moving a library
between locations, and choosing where sync keys are kept. A web request must not
be able to read an arbitrary path on your disk, so those capabilities have no
HTTP route at all.

**REST** and **MCP** are how other programs work with a library — including an
AI assistant, within a scope you grant. They carry the reading and writing of
notes, and deliberately not the operations that name paths or manage
credentials.

The **shared library** is the fifth column in the table below and is empty
today. Notrios has a C ABI, and a future client — a mobile app, say — would
reach the same capabilities through it. Nothing claims that surface yet, and an
empty column is the honest way to say so.

Where a capability is missing from a surface, its own section says why. Those
reasons are worth reading: most are boundaries somebody chose, a few are simply
work not done, and the page distinguishes them rather than dressing the second
up as the first.

## What you can do
<!-- notrios:generated:user:what-you-can-do:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docfeatures#Registry -->
Registry is the feature catalogue: what a person can do with Notrios, and
which surfaces offer each capability.

### Write and edit notes

Create a note, change it, add to either end of it, and delete it. Deleting moves a note to Trash, and Trash is a place you can look in and take things back out of, not a countdown.

The command line writes, edits, files, trashes and restores a note; appending and prepending arrived with v0.8 H27, because placing an attachment link otherwise meant rewriting the whole note. `notes create` exists so a note can be the end of a pipeline, and `notes delete` ships with `notes restore` because a delete whose undo lives on another surface is a poor boundary. This entry said editing and deleting happened only in the GUI or over REST/MCP for five days after both commands shipped, because neither appeared in `notriosctl help` and the coverage check reads the help.

*Available on the desktop app, the command line, the REST API and MCP.*

### Read a note and its structure

Fetch a note whole, or just its body, its outline, its blocks, a line range, or an earlier revision. The structured views exist so a tool can work on part of a note without re-parsing all of it.

The command line prints a note as Markdown with the front matter the Recoll projection writes, so another application can read it, and `--json` gives the fields instead; `notes outline`, `notes resources` and `notes links` answer what a note is made of. Blocks, line ranges and revisions stay on REST and MCP. This entry read "No command line: reading a note is what the GUI and the API are for" until v0.8 H21, which was wrong twice over -- `notes show` already read a note, and nobody had decided a terminal should not. An absent adapter tends to acquire a justification.

*Available on the desktop app, the command line, the REST API and MCP.*

### Search your notes

Find notes by text, tag, notebook, date and the rest of the query language, across the library or within one note.

The command line searches with the same query language the search box parses, and returns each hit's identifier -- which is what makes every other command usable from a terminal: nothing at a terminal produced note ids before v0.8 H19, so `notes show`, `notes move` and `tags add` could only be used on an id somebody already had. `--count` runs a counting query over the same compiled predicate rather than paging the results and tallying them, and `--links` gives the notrios:// form for pasting into another machine's library. A search spans every collection; `collection:` narrows it to one provenance, and `notriosctl collections list` says which ones exist. Trash is excluded unless the query says `is:trashed`. The documented workaround used to be `export archive --query`, which applies the query language to a file export -- an answer to a different question.

*Available on the desktop app, the command line, the REST API and MCP.*

### Notebooks that are really saved searches

Save a query as a notebook, so a view that would otherwise be retyped becomes something you open.

Made with `notebooks create --query` at the command line. In the GUI it is an action on a search that has already run and returned something, rather than a form: the query is one you have watched work, and a notebook made from an unrun query opens empty as easily as it opens right. The query is shown when keeping it and is not editable there, which is what keeps this from being a query form in another place.

*Available on the desktop app, the command line, the REST API and MCP.*

### Organise notes into notebooks

Make notebooks, nest them, move notes between them, and see what a notebook deletion would take with it before agreeing to it.

`notebooks list` prints a table of ids and names, with `--json` for the structured form. It printed JSON only until v0.8 H22, which is the right answer for a script and the wrong one for the person the command exists to help: a query names a notebook by id, and finding the id meant reading a JSON object at a terminal.

*Available on the desktop app, the command line, the REST API and MCP.*

### Tag and untag a note

Attach a tag to a note, take one off, and see what a note carries. Tags are hierarchical: `field/dusk` sits under `field`.

*Available on the desktop app, the command line, the REST API and MCP.*

### See and rename tags

List the tags in a library, see a note's tags, and rename a whole tag hierarchy at once.

Asking about one tag used to mean fetching every tag: listing returned the whole vocabulary with no filter and no limit on all three surfaces, and `tags list --tag <name>` was accepted and ignored, returning everything. v0.8 H26 added narrowing to the command line, REST and MCP together, because no surface had it -- `tags show` exits 1 when a tag does not exist so a script can test for one without parsing anything, `--prefix` takes a branch of the hierarchy, and a bounded answer says whether it truncated. Listing the notes carrying a tag is a search, `tag:todo`, rather than a second thing this reports.

*Available on the desktop app, the command line, the REST API and MCP.*

### Group libraries into collections

Collections sit above notebooks and are how an import keeps its material together.

A collection is provenance, not a place notes live: notes are imported into a notebook, which is what a person browses, and carry a collection identifier saying where they came from. Search spans every collection and `collection:` narrows it -- it used to be pinned to one collection chosen by the caller and defaulted to `default`, which made imported notes unfindable and the field meaningless. The inspector names the collection only when it is not `default`, since a row saying so on every note written here would say nothing. The command line lists collections and shows one, with how many notes name each; REST and MCP list them without that count, which H16 should decide about along with the rest of the collection shape. Creating and reconfiguring a collection is still not on the command line: an import creates one silently when `--collection` names a new id, and this entry claimed the opposite for months. H16 removed `kind` and `capabilities` from the API, the OpenAPI schema and `list_collections`, and stopped writing the capabilities array into archive-v2: the schema recorded neither, and both were constants the code reported about itself. A collection's name and description stay editable through `PATCH /api/v1/collections/{collection_id}`, because renaming a label changes no provenance while the identifier is untouched.

*Available on the desktop app, the command line, the REST API and MCP.*

### Attach and manage files

Add files to notes, read them back, see what references what, and find attachments nothing points at any more.

Measured by the H15 control crawl, which found the upload field in the note inspector and the Attachments tab. The command line lists what a note carries and writes one attachment's bytes to a file; attaching a file is still REST, MCP or the GUI, because uploading is a write and this half was added as the reading half. Attaching a file is on the command line as of v0.8 H27: the reasoning that kept it off was right about placement and wrong to stop there, since the product already separates the resource, the reference to it, and the link in the body. `resources add` adds the first two and prints the `resource://` URI for the author to place with `notes append` or `notes prepend`, so it guesses at nothing and never writes to a body. Those two writes are on the command line now for the same reason: no surface can patch a range of a note body, so placing a link would otherwise mean rewriting the whole note.

*Available on the desktop app, the command line, the REST API and MCP.*

### Bring remote images into the library

Find images a note points at on the web, check them against the domain policy, and copy the allowed ones in so the note stops depending on somebody else's server.

Measured by the H15 control crawl on a note seeded with an image on an allowed domain. The earlier note here said localizing was a maintenance action rather than an editing one; it is in the editor.

*Available on the desktop app, the command line, the REST API and MCP.*

### Link notes to each other

Get a stable link to a note or one of its sections, open one, check the links in a draft before saving, and register Notrios as the handler for notrios:// links.

*Available on the desktop app, the command line, the REST API and MCP.*

### See the shape of the link graph

Look at what surrounds a note, find a path between two notes, and get a report on orphans, isolates and hubs.

*Available on the desktop app, the command line, the REST API and MCP.*

### Templates and tasks

Keep note templates and make new notes from them, and see the tasks across a library.

A template is an ordinary note carrying a note-template block, and a task is a checkbox line in a note tagged `task` or `todo`; neither is a table, and both are read back from the Markdown. So nothing creates a task -- writing `- [ ] chase the permit` into a note is how one comes to exist, which `notes create` already does -- and the command line covers the other half: asking what remains, and instantiating a template, which is the repeatable version of the same workflow. A missing placeholder is refused rather than left blank, so a template that gains a field fails the scripts that do not know about it. The tag is required because a checkbox is ordinary Markdown and appears in quoted examples; `--untagged` asks for those too. No GUI journey yet.

*Available on the command line, the REST API and MCP.*

### Live query blocks inside a note

Put a query in a note and have its results render where it sits.

No command line, and that is the line rather than a gap: a query block is a rendering inside a note -- the note carries a fenced query and the interface shows what it matches in place. At a terminal the same question is `notriosctl search`, and formatting the answer is a template tool's job. A command that ran a block's query would be a second way to run a query.

*Available on the desktop app, the REST API and MCP.*

### Do many organiser operations at once

Send a batch of moves, tags and notebook changes as one request, so a large reorganisation is one reviewable action.

The command line names the set with a search rather than a list of ids: `--query` on `notes move`, `notes delete`, `notes restore`, `notes duplicate`, `tags add` and `tags remove` runs the same batch, showing the selection unless `--apply` is given. `notes duplicate` is listed here because it is the only one of the six with no other home; the rest are listed under the capability they belong to. No GUI surface yet.

*Available on the command line, the REST API and MCP.*

### Import from another application

Bring in a Joplin raw export, an Obsidian vault, a Twitter or X archive, a ChatGPT or Claude export, or a Notrios archive. Every importer dry-runs first.

In the desktop app only, through the native bridge: these operations name a folder on the machine running the library, and no REST route starts them. In a browser the Import/Export control is shown and disabled with that reason rather than hidden. Joplin and Obsidian can be scanned first without writing. Importing from Notrios verifies the archive without opening a database, and merges only: replace, fork and adopt stay on the command line because they change which library this is. Twitter, ChatGPT and Claude imports remain command line only. The journey is photographed by the desktop harness rather than by the browser capture, because the dialog it lives in is correctly disabled in a browser: the real application is driven under Xvfb, and each step declares the line it expects in the application's own transcript, so a keystroke that lands elsewhere fails the capture instead of producing a confident picture of the wrong thing.

*Available on the desktop app and the command line.*

### Export your library

Write a portable archive of everything or a chosen subset, check one for compatibility, verify one, and restore one with an explicit intent.

In the desktop app only, through the native bridge: these operations name a folder on the machine running the library, and no REST route starts them. In a browser the Import/Export control is shown and disabled with that reason rather than hidden. The GUI writes a complete archive; exporting a subset by notebook, tag or query stays on the command line, because choosing a subset means seeing what it selects first. The journey is photographed by the desktop harness rather than by the browser capture, because the dialog it lives in is correctly disabled in a browser: the real application is driven under Xvfb, and each step declares the line it expects in the application's own transcript, so a keystroke that lands elsewhere fails the capture instead of producing a confident picture of the wrong thing.

*Available on the desktop app and the command line.*

### Back up and restore the whole library

Take a physical snapshot of the database and its attachments, verify it, and restore it — replacing this library or adopting the snapshot as a new replica.

In the desktop app only, through the native bridge: these operations name a folder on the machine running the library, and no REST route starts them. In a browser the Import/Export control is shown and disabled with that reason rather than hidden. Creating a snapshot is in the GUI; restoring one is not, because a restore replaces the library the window is showing. The journey is photographed by the desktop harness rather than by the browser capture, because the dialog it lives in is correctly disabled in a browser: the real application is driven under Xvfb, and each step declares the line it expects in the application's own transcript, so a keystroke that lands elsewhere fails the capture instead of producing a confident picture of the wrong thing.

*Available on the desktop app and the command line.*

### Pair two of your own libraries

Enrol a library for synchronization, issue a single-use code, and spend it from the other side so the two learn each other's keys.

No MCP tools, by a decision recorded in `SYNCHRONIZATION.md`: "Enrollment, peer retirement, purge, backup export/restore, destructive recovery, and cancelling another actor's catch-up remain outside MCP", and MCP sync authority is separately configured as `disabled|status|control` and "never reaches catch-up/restore preparation, enrollment, keys, backup/restore, retirement, purge, or reset". So MCP has the running of sync -- `start_sync`, `plan_sync`, `get_sync_status`, `retry_sync_job`, `cancel_sync_job` -- and none of the establishing of trust. `sync accept` and `sync enroll` also read and write files the caller names, which is why import and export have no REST route either. A mobile client does not want these as MCP tools: it is a client of the shared C ABI, whose operation set is `abi.info`, `note.*`, `search` and `resource.get` and carries no sync at all. Pairing from a phone is an ABI question, not an MCP one, and is unanswered.

*Available on the desktop app, the command line and the REST API.*

### Synchronize with a replica

Exchange changes with a paired replica, directly over an authenticated connection or through a folder you both can reach — a second drive, or a cloud folder mapped locally.

*Available on the desktop app, the command line, the REST API and MCP.*

### See and end trust between replicas

List the replicas a library trusts, revoke one's key, and retire a peer after reviewing exactly what retiring it means.

No MCP tools, under the same recorded decision as pairing: `SYNCHRONIZATION.md` places retirement and revocation outside MCP whatever the configured sync authority. An assistant can watch a sync and cannot decide who is trusted. Retiring a peer is a permanent identity decision recorded with a signature, and `sync revoke --advance-epoch` changes what every other replica will accept.

*Available on the desktop app, the command line and the REST API.*

### Recover a replica and resolve conflicts

Fetch a peer-verified backup, inspect one without installing it, work through sync conflicts, and decide what a shared attachment should do.

*Available on the desktop app, the command line, the REST API and MCP.*

### Choose where sync keys are kept

An installed Notrios keeps the key that protects your sync material in the operating system's credential store. Move existing keys between that and the owner-only development file, in either direction.

Deliberately command line only: this item forbids a credential-management REST or MCP surface.

*Available on the command line.*

### Watch and steer long-running work

Imports, exports and syncs run as jobs you can list, inspect, cancel, retry and reset.

Measured by the H15 control crawl once jobs were seeded, which is what the row had been waiting for: every job control is gated on state, so an interface holding only completed jobs shows none of them. What the GUI has is the sync centre's list of recent *sync* jobs, with Cancel on one still running or queued and Retry on one that failed or was cancelled. Import, export and snapshot jobs appear nowhere: the sync centre lists four sync kinds and only those, and the import dialog shows a final report rather than a job. There is no general job list, no inspect and no reset.

*Available on the desktop app, the command line, the REST API and MCP.*

### Keep separate libraries

Run more than one library on a machine — personal notes, work notes, a blog — each with its own database, its own settings and its own address.

Switching between profiles is in the GUI; creating, registering and forgetting them is not. A profile registry is about this machine, which the desktop app also runs on, so the rest is a gap in the GUI rather than a boundary; the browser mode is the part that genuinely cannot manage it. The absence from REST and MCP is a boundary, and a firmer one than the GUI's: `profile create` takes `--data-dir`, `--registry` and `--public-url`, and `profile start` launches a binary. A route that accepted those would let a caller point the service at a directory of their choosing and start a process -- strictly worse than the local paths import and export already refuse to accept over HTTP.

*Available on the desktop app and the command line.*

### Publish a subset of your notes

Choose what leaves the library, review exactly what a publication would include and withhold, save that choice as a profile, and publish only after agreeing to the reviewed plan.

The review is in the GUI, because reading what a publication lets out is the point of the feature rather than a step before it: the counts lead with what is withheld and which links get rewritten, not with what is sent. Planning goes through the native bridge rather than over REST for a correctness reason, not a path one -- `publish run` re-plans and refuses unless the digest matches what was reviewed, so the plan shown must be the plan the run recomputes, and an equivalent selection rebuilt over REST would match only by luck. Profiles are read and never written here: a profile carries a target, a selection and a privacy policy, and a form that quietly defaulted one is how something private gets published, so authoring stays `notriosctl publish profile save`. The journey is photographed by the desktop harness rather than by the browser capture, because the dialog it lives in is correctly disabled in a browser: the real application is driven under Xvfb, and each step declares the line it expects in the application's own transcript, so a keystroke that lands elsewhere fails the capture instead of producing a confident picture of the wrong thing.

*Available on the desktop app, the command line, the REST API and MCP.*

### Keep a library healthy

Find what has rotted, fix what can be fixed mechanically, reclaim space, and check that this installation is set up the way you think it is.

The GUI reports both, and repairs. Repairing goes through the native bridge because the lint surface is deliberately read-only over REST -- that is a remote, programmatic surface an agent reaches, and repairs to somebody's notes should not be one call away from one -- which says nothing about a person pressing a button in the program that owns the library. Each repair writes a revision against a required base revision, so a note edited since the plan refuses rather than being repaired against text nobody read, and refusals are reported per note. Collecting stays `notriosctl gc --apply`: it deletes blobs, and there is no revision to go back to. doctor, paths, config show and seed-help remain command line.

*Available on the desktop app, the command line, the REST API and MCP.*

### Move a pre-0.8 library into place

A library that lived in ./data next to the program is relocated into the directories an installed Notrios uses, after showing you the plan.

Command line only, and genuinely so: it relocates the directories the running program uses, which is not something the program can sensibly do to itself while serving them.

*Available on the command line.*

### Let an AI assistant use your library

Notrios speaks MCP, so an assistant can read and, within a scope you grant, change your notes.

No command line, because this is a served surface rather than something a person runs: software with an MCP client connects to it. A command-line journey would document the client rather than this program.

*Available on the REST API.*

<!-- notrios:generated:user:what-you-can-do:end -->

## Where each capability lives
<!-- notrios:generated:user:where-each-capability-lives:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/docfeatures#(Registry).SurfaceTable -->
SurfaceTable is the capability-by-adapter table: one row per capability, one
column per surface, and a number saying how many operations that surface
spends on it.

It answers a question the prose above cannot answer quickly -- "can I do this
from the command line?" -- and it is generated from the same registry, so it
cannot drift from the sentences beside it. An empty cell is not an oversight;
it is a capability that surface does not offer, and the reason is in that
capability's own section.

| Capability | Desktop app | Command line | REST | MCP | Shared library |
|---|---|---|---|---|---|
| Write and edit notes | 6 | 6 | 10 | 6 | — |
| Read a note and its structure | 2 | 4 | 7 | 5 | — |
| Search your notes | 1 | 1 | 3 | 2 | — |
| Notebooks that are really saved searches | 1 | 1 | 3 | 1 | — |
| Organise notes into notebooks | 2 | 3 | 9 | 4 | — |
| Tag and untag a note | 4 | 3 | 2 | 2 | — |
| See and rename tags | 4 | 3 | 3 | 1 | — |
| Group libraries into collections | 2 | 2 | 4 | 1 | — |
| Attach and manage files | 2 | 4 | 8 | 2 | — |
| Bring remote images into the library | 2 | 1 | 4 | 3 | — |
| Link notes to each other | 1 | 4 | 4 | 1 | — |
| See the shape of the link graph | 1 | 2 | 4 | 3 | — |
| Templates and tasks | — | 3 | 4 | 3 | — |
| Live query blocks inside a note | 1 | — | 1 | 1 | — |
| Do many organiser operations at once | — | 1 | 1 | 1 | — |
| Import from another application | 3 | 6 | — | — | — |
| Export your library | 2 | 5 | — | — | — |
| Back up and restore the whole library | 1 | 3 | — | — | — |
| Pair two of your own libraries | 1 | 7 | 8 | — | — |
| Synchronize with a replica | 1 | 5 | 9 | 3 | — |
| See and end trust between replicas | 2 | 3 | 4 | — | — |
| Recover a replica and resolve conflicts | 2 | 1 | 9 | 1 | — |
| Choose where sync keys are kept | — | 1 | — | — | — |
| Watch and steer long-running work | 3 | 5 | 5 | 4 | — |
| Keep separate libraries | 1 | 7 | — | — | — |
| Publish a subset of your notes | 3 | 5 | 1 | 1 | — |
| Keep a library healthy | 3 | 8 | 4 | 1 | — |
| Move a pre-0.8 library into place | — | 1 | — | — | — |
| Let an AI assistant use your library | — | — | 2 | — | — |
<!-- notrios:generated:user:where-each-capability-lives:end -->

The table is generated from the same registry as the sections above, so it
cannot disagree with them. Each number is how many operations that surface
spends on the capability — a rough measure of how completely it is offered
there, not a score. A dash means that surface does not offer it, and the
capability's own section says why.

## Where the surfaces disagree
<!-- notrios:generated:user:where-the-surfaces-disagree:begin -->
<!-- source: go:github.com/renesugar/notrios/internal/doccompare#Lines -->
Each capability below is offered on some surfaces and not others.

- Let an AI assistant use your library — neither the command line nor the GUI; reachable only over REST or MCP. No command line, because this is a served surface rather than something a person runs: software with an MCP client connects to it. A command-line journey would document the client rather than this program.
- Do many organiser operations at once — command line only. The command line names the set with a search rather than a list of ids: `--query` on `notes move`, `notes delete`, `notes restore`, `notes duplicate`, `tags add` and `tags remove` runs the same batch, showing the selection unless `--apply` is given. `notes duplicate` is listed here because it is the only one of the six with no other home; the rest are listed under the capability they belong to. No GUI surface yet.
- Choose where sync keys are kept — command line only. Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- Templates and tasks — command line only. A template is an ordinary note carrying a note-template block, and a task is a checkbox line in a note tagged `task` or `todo`; neither is a table, and both are read back from the Markdown. So nothing creates a task -- writing `- [ ] chase the permit` into a note is how one comes to exist, which `notes create` already does -- and the command line covers the other half: asking what remains, and instantiating a template, which is the repeatable version of the same workflow. A missing placeholder is refused rather than left blank, so a template that gains a field fails the scripts that do not know about it. The tag is required because a checkbox is ordinary Markdown and appears in quoted examples; `--untagged` asks for those too. No GUI journey yet.
- Move a pre-0.8 library into place — command line only. Command line only, and genuinely so: it relocates the directories the running program uses, which is not something the program can sensibly do to itself while serving them.
- Live query blocks inside a note — GUI only. No command line, and that is the line rather than a gap: a query block is a rendering inside a note -- the note carries a fenced query and the interface shows what it matches in place. At a terminal the same question is `notriosctl search`, and formatting the answer is a template tool's job. A command that ran a block's query would be a second way to run a query.
- See and end trust between replicas — both surfaces, and neither journey is written yet
- Recover a replica and resolve conflicts — both surfaces, and neither journey is written yet
- Attach and manage files — both, but only the command-line journey is written
- Group libraries into collections — both, but only the command-line journey is written
- See the shape of the link graph — both, but only the command-line journey is written
- Watch and steer long-running work — both, but only the command-line journey is written
- Link notes to each other — both, but only the command-line journey is written
- Keep separate libraries — both, but only the command-line journey is written
- Bring remote images into the library — both, but only the command-line journey is written
- Synchronize with a replica — both, but only the command-line journey is written
- Pair two of your own libraries — both, but only the command-line journey is written
<!-- notrios:generated:user:where-the-surfaces-disagree:end -->

This list is computed, not written. It compares what each capability claims
against which journeys exist, and reports two different kinds of disagreement.

A **capability** difference is a thing one surface can do and another cannot.
Most are deliberate and say why beside them: importing names a folder and a
browser cannot open one; credential migration is refused a remote surface on
purpose; a batch is for tools rather than for a person.

A **documentation** difference is a capability both surfaces offer where only
one has a journey written. Nothing is missing from Notrios there; something is
missing from these pages, and the entry says which half.

## How this page is kept honest

Everything generated above comes from `docs/docfeatures/FEATURES.json`, which
records each capability's title, summary and the exact surfaces that offer it.
Three checks run against it.

The first is coverage: every CLI usage form, REST operation and MCP tool must be
claimed by some feature. A new capability added without an entry here fails the
build, because a capability nobody can discover is a documentation defect even
when every other gate is green.

The second runs the other way: a feature may not claim a surface that does not
exist. That catches a page describing something that was removed — which no
consistency check can see, because such a page is perfectly consistent with
itself. It caught three invented endpoints the first time it ran.

The third is newer and narrower: a claim on the shared library is refused
outright, because that surface publishes no inventory to check a claim against.
The column can only be filled once the ABI says what it offers.

None of that makes the prose true. This page has been wrong while every gate was
green — it once said tagging was unreachable from the command line and the
interface for as long as it took somebody to notice that both had been built.
The paragraphs are written by hand and are the part worth reading sceptically.
