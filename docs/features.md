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

The command line writes a note and files it, and does no more than that: editing, appending and deleting happen in the GUI or over REST/MCP. `notes create` exists so a note can be the end of a pipeline.

*Available on the desktop app, the command line, the REST API and MCP.*

### Read a note and its structure

Fetch a note whole, or just its body, its outline, its blocks, a line range, or an earlier revision. The structured views exist so a tool can work on part of a note without re-parsing all of it.

No command line: reading a note is what the GUI and the API are for.

*Available on the desktop app, the REST API and MCP.*

### Search your notes

Find notes by text, tag, notebook, date and the rest of the query language, across the library or within one note.

No command line search command; `notriosctl export archive --query` applies the same language to an export instead. A search now spans every collection; `collection:"id"` narrows it to one provenance.

*Available on the desktop app, the REST API and MCP.*

### Notebooks that are really saved searches

Save a query as a notebook, so a view that would otherwise be retyped becomes something you open.

Made with `notebooks create --query` at the command line. In the GUI it is an action on a search that has already run and returned something, rather than a form: the query is one you have watched work, and a notebook made from an unrun query opens empty as easily as it opens right. The query is shown when keeping it and is not editable there, which is what keeps this from being a query form in another place.

*Available on the desktop app, the command line, the REST API and MCP.*

### Organise notes into notebooks

Make notebooks, nest them, move notes between them, and see what a notebook deletion would take with it before agreeing to it.

*Available on the desktop app, the command line, the REST API and MCP.*

### Tag and untag a note

Attach a tag to a note, take one off, and see what a note carries. Tags are hierarchical: `field/dusk` sits under `field`.

*Available on the desktop app, the command line, the REST API and MCP.*

### See and rename tags

List the tags in a library, see a note's tags, and rename a whole tag hierarchy at once.

Renaming is offered beside the tag, because it is decided while looking at the tag and the count next to it is half the reason for deciding. The dry run is not a prediction: the service performs the rename in a transaction and rolls it back, so the report and the apply cannot disagree. Applying is unreachable until a report of that exact rename has been shown, and editing the name or the children option withdraws it.

*Available on the desktop app, the command line, the REST API and MCP.*

### Group libraries into collections

Collections sit above notebooks and are how an import keeps its material together.

A collection is provenance, not a place notes live: notes are imported into a notebook, which is what a person browses, and carry a collection identifier saying where they came from. Search spans every collection and `collection:` narrows it -- it used to be pinned to one collection chosen by the caller and defaulted to `default`, which made imported notes unfindable and the field meaningless. The inspector names the collection only when it is not `default`, since a row saying so on every note written here would say nothing. Creating and reconfiguring collections stays on the command line and over REST; H16 decides what a collection's `kind` and capabilities mean, because the schema records neither.

*Available on the desktop app, the REST API and MCP.*

### Attach and manage files

Add files to notes, read them back, see what references what, and find attachments nothing points at any more.

Measured by the H15 control crawl, which found the upload field in the note inspector and the Attachments tab. This was recorded as absent from the GUI until the crawl opened a note; no journey covers it yet.

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

No command line. The GUI renders them where they sit: the preview finds each ```note-query block, runs it through the same parser the search box uses, and fills the block with the result. This row read as a gap for as long as the control crawl was the only measurement, and the crawl cannot see it and never could -- a query block renders content, not a control, and the crawl enumerates interactive elements. Its evidence is web/src/note-query.ts and its tests instead.

*Available on the desktop app, the REST API and MCP.*

### Do many organiser operations at once

Send a batch of moves, tags and notebook changes as one request, so a large reorganisation is one reviewable action.

No command line or GUI journey; batching is for tools.

*Available on the REST API and MCP.*

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

*Available on the desktop app, the command line and the REST API.*

### Synchronize with a replica

Exchange changes with a paired replica, directly over an authenticated connection or through a folder you both can reach — a second drive, or a cloud folder mapped locally.

*Available on the desktop app, the command line, the REST API and MCP.*

### See and end trust between replicas

List the replicas a library trusts, revoke one's key, and retire a peer after reviewing exactly what retiring it means.

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

Switching between profiles is in the GUI; creating, registering and forgetting them is not. A profile registry is about this machine, which the desktop app also runs on, so the rest is a gap rather than a boundary; the browser mode is the part that genuinely cannot manage it.

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

The endpoint itself has no command line or GUI; the tools it exposes are listed against the features above.

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
| Write and edit notes | 6 | 1 | 10 | 6 | — |
| Read a note and its structure | 2 | — | 7 | 5 | — |
| Search your notes | 1 | — | 3 | 2 | — |
| Notebooks that are really saved searches | 1 | 1 | 3 | 1 | — |
| Organise notes into notebooks | 2 | 3 | 9 | 4 | — |
| Tag and untag a note | 4 | 3 | 2 | 2 | — |
| See and rename tags | 4 | 1 | 3 | 1 | — |
| Group libraries into collections | 2 | — | 4 | 1 | — |
| Attach and manage files | 2 | 1 | 8 | 2 | — |
| Bring remote images into the library | 2 | 1 | 4 | 3 | — |
| Link notes to each other | 1 | 3 | 4 | 1 | — |
| See the shape of the link graph | 1 | 2 | 4 | 3 | — |
| Templates and tasks | — | 3 | 4 | 3 | — |
| Live query blocks inside a note | 1 | — | 1 | 1 | — |
| Do many organiser operations at once | — | — | 1 | 1 | — |
| Import from another application | 3 | 6 | — | — | — |
| Export your library | 2 | 5 | — | — | — |
| Back up and restore the whole library | 1 | 3 | — | — | — |
| Pair two of your own libraries | 1 | 6 | 8 | — | — |
| Synchronize with a replica | 1 | 3 | 9 | 3 | — |
| See and end trust between replicas | 2 | 1 | 4 | — | — |
| Recover a replica and resolve conflicts | 2 | 1 | 9 | 1 | — |
| Choose where sync keys are kept | — | 1 | — | — | — |
| Watch and steer long-running work | 3 | 5 | 5 | 4 | — |
| Keep separate libraries | 1 | 7 | — | — | — |
| Publish a subset of your notes | 3 | 4 | 1 | 1 | — |
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

- Do many organiser operations at once — neither the command line nor the GUI; reachable only over REST or MCP. No command line or GUI journey; batching is for tools.
- Let an AI assistant use your library — neither the command line nor the GUI; reachable only over REST or MCP. The endpoint itself has no command line or GUI; the tools it exposes are listed against the features above.
- Choose where sync keys are kept — command line only. Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- Templates and tasks — command line only. A template is an ordinary note carrying a note-template block, and a task is a checkbox line in a note tagged `task` or `todo`; neither is a table, and both are read back from the Markdown. So nothing creates a task -- writing `- [ ] chase the permit` into a note is how one comes to exist, which `notes create` already does -- and the command line covers the other half: asking what remains, and instantiating a template, which is the repeatable version of the same workflow. A missing placeholder is refused rather than left blank, so a template that gains a field fails the scripts that do not know about it. The tag is required because a checkbox is ordinary Markdown and appears in quoted examples; `--untagged` asks for those too. No GUI journey yet.
- Move a pre-0.8 library into place — command line only. Command line only, and genuinely so: it relocates the directories the running program uses, which is not something the program can sensibly do to itself while serving them.
- Group libraries into collections — GUI only. A collection is provenance, not a place notes live: notes are imported into a notebook, which is what a person browses, and carry a collection identifier saying where they came from. Search spans every collection and `collection:` narrows it -- it used to be pinned to one collection chosen by the caller and defaulted to `default`, which made imported notes unfindable and the field meaningless. The inspector names the collection only when it is not `default`, since a row saying so on every note written here would say nothing. Creating and reconfiguring collections stays on the command line and over REST; H16 decides what a collection's `kind` and capabilities mean, because the schema records neither.
- Live query blocks inside a note — GUI only. No command line. The GUI renders them where they sit: the preview finds each ```note-query block, runs it through the same parser the search box uses, and fills the block with the result. This row read as a gap for as long as the control crawl was the only measurement, and the crawl cannot see it and never could -- a query block renders content, not a control, and the crawl enumerates interactive elements. Its evidence is web/src/note-query.ts and its tests instead.
- Read a note and its structure — GUI only. No command line: reading a note is what the GUI and the API are for.
- Search your notes — GUI only. No command line search command; `notriosctl export archive --query` applies the same language to an export instead. A search now spans every collection; `collection:"id"` narrows it to one provenance.
- Attach and manage files — both surfaces, and neither journey is written yet
- See the shape of the link graph — both surfaces, and neither journey is written yet
- Watch and steer long-running work — both surfaces, and neither journey is written yet
- Link notes to each other — both surfaces, and neither journey is written yet
- Bring remote images into the library — both surfaces, and neither journey is written yet
- See and end trust between replicas — both surfaces, and neither journey is written yet
- Recover a replica and resolve conflicts — both surfaces, and neither journey is written yet
- Keep separate libraries — both, but only the command-line journey is written
- Synchronize with a replica — both, but only the command-line journey is written
- Pair two of your own libraries — both, but only the command-line journey is written
- See and rename tags — both, but only the GUI journey is written
- Publish a subset of your notes — both, but only the GUI journey is written
- Notebooks that are really saved searches — both, but only the GUI journey is written
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
