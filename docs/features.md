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
- Notebooks that are really saved searches — Save a query as a notebook, so a view that would otherwise be retyped becomes something you open. (CLI 1, REST 3, MCP 1) Made with `notebooks create --query`, since a query notebook is a notebook whose contents are whatever matches. The GUI lists them in the sidebar and does not create them yet.
- Organise notes into notebooks — Make notebooks, nest them, move notes between them, and see what a notebook deletion would take with it before agreeing to it. (CLI 3, REST 9, MCP 4, GUI 2)
- Tag and untag a note — Attach a tag to a note, take one off, and see what a note carries. Tags are hierarchical: `field/dusk` sits under `field`. (CLI 3, REST 2, MCP 2, GUI 4)
- See and rename tags — List the tags in a library, see a note's tags, and rename a whole tag hierarchy at once. (CLI 1, REST 3, MCP 1) The GUI shows tags in the sidebar and on the note, and does not rename a hierarchy; `tags rename` is command line only.
- Group libraries into collections — Collections sit above notebooks and are how an import keeps its material together. (REST 4, MCP 1) No command line or GUI journey; imports set the collection with --collection.
- Attach and manage files — Add files to notes, read them back, see what references what, and find attachments nothing points at any more. (CLI 1, REST 8, MCP 2, GUI 2) Measured by the H15 control crawl, which found the upload field in the note inspector and the Attachments tab. This was recorded as absent from the GUI until the crawl opened a note; no journey covers it yet.
- Bring remote images into the library — Find images a note points at on the web, check them against the domain policy, and copy the allowed ones in so the note stops depending on somebody else's server. (CLI 1, REST 4, MCP 3, GUI 2) Measured by the H15 control crawl on a note seeded with an image on an allowed domain. The earlier note here said localizing was a maintenance action rather than an editing one; it is in the editor.
- Link notes to each other — Get a stable link to a note or one of its sections, open one, check the links in a draft before saving, and register Notrios as the handler for notrios:// links. (CLI 3, REST 4, MCP 1, GUI 1)
- See the shape of the link graph — Look at what surrounds a note, find a path between two notes, and get a report on orphans, isolates and hubs. (CLI 2, REST 4, MCP 3, GUI 1)
- Templates and tasks — Keep note templates and make new notes from them, and see the tasks across a library. (REST 4, MCP 3) No command line or GUI journey.
- Live query blocks inside a note — Put a query in a note and have its results render where it sits. (REST 1, MCP 1) No command line; the GUI renders them in place.
- Do many organiser operations at once — Send a batch of moves, tags and notebook changes as one request, so a large reorganisation is one reviewable action. (REST 1, MCP 1) No command line or GUI journey; batching is for tools.
- Import from another application — Bring in a Joplin raw export, an Obsidian vault, a Twitter or X archive, a ChatGPT or Claude export, or a Notrios archive. Every importer dry-runs first. (CLI 6) Not in the GUI yet. The constraint is the *browser* mode, which cannot read arbitrary local paths; the desktop app can, and native directory pickers are part of the planned client architecture. This is a gap rather than a boundary.
- Export your library — Write a portable archive of everything or a chosen subset, check one for compatibility, verify one, and restore one with an explicit intent. (CLI 5) Not in the GUI yet. The constraint is the *browser* mode, which cannot read arbitrary local paths; the desktop app can, and native directory pickers are part of the planned client architecture. This is a gap rather than a boundary.
- Back up and restore the whole library — Take a physical snapshot of the database and its attachments, verify it, and restore it — replacing this library or adopting the snapshot as a new replica. (CLI 3) Not in the GUI yet. The constraint is the *browser* mode, which cannot read arbitrary local paths; the desktop app can, and native directory pickers are part of the planned client architecture. This is a gap rather than a boundary.
- Pair two of your own libraries — Enrol a library for synchronization, issue a single-use code, and spend it from the other side so the two learn each other's keys. (CLI 6, REST 8, GUI 1)
- Synchronize with a replica — Exchange changes with a paired replica, directly over an authenticated connection or through a folder you both can reach — a second drive, or a cloud folder mapped locally. (CLI 3, REST 9, MCP 3, GUI 1)
- See and end trust between replicas — List the replicas a library trusts, revoke one's key, and retire a peer after reviewing exactly what retiring it means. (CLI 1, REST 4, GUI 2)
- Recover a replica and resolve conflicts — Fetch a peer-verified backup, inspect one without installing it, work through sync conflicts, and decide what a shared attachment should do. (CLI 1, REST 9, MCP 1, GUI 2)
- Choose where sync keys are kept — An installed Notrios keeps the key that protects your sync material in the operating system's credential store. Move existing keys between that and the owner-only development file, in either direction. (CLI 1) Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- Watch and steer long-running work — Imports, exports and syncs run as jobs you can list, inspect, cancel, retry and reset. (CLI 5, REST 5, MCP 4) No GUI journey; progress appears inline where the work was started. Not settled by the H15 crawl either: job rows only render once a job exists and nothing seeded one, so this row is unmeasured rather than known absent.
- Keep separate libraries — Run more than one library on a machine — personal notes, work notes, a blog — each with its own database, its own settings and its own address. (CLI 7, GUI 1) Switching between profiles is in the GUI; creating, registering and forgetting them is not. A profile registry is about this machine, which the desktop app also runs on, so the rest is a gap rather than a boundary; the browser mode is the part that genuinely cannot manage it.
- Publish a subset of your notes — Choose what leaves the library, review exactly what a publication would include and withhold, save that choice as a profile, and publish only after agreeing to the reviewed plan. (CLI 4, REST 1, MCP 1) Not in the GUI yet. Publishing writes to a local directory, so the desktop app could do it and the browser mode could not.
- Keep a library healthy — Find what has rotted, fix what can be fixed mechanically, reclaim space, and check that this installation is set up the way you think it is. (CLI 8, REST 4, MCP 1) No GUI journey; maintenance is command line work.
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
- Live query blocks inside a note — neither the command line nor the GUI; reachable only over REST or MCP. No command line; the GUI renders them in place.
- Templates and tasks — neither the command line nor the GUI; reachable only over REST or MCP. No command line or GUI journey.
- See and rename tags — command line only. The GUI shows tags in the sidebar and on the note, and does not rename a hierarchy; `tags rename` is command line only.
- Export your library — command line only. Not in the GUI yet. The constraint is the *browser* mode, which cannot read arbitrary local paths; the desktop app can, and native directory pickers are part of the planned client architecture. This is a gap rather than a boundary.
- Import from another application — command line only. Not in the GUI yet. The constraint is the *browser* mode, which cannot read arbitrary local paths; the desktop app can, and native directory pickers are part of the planned client architecture. This is a gap rather than a boundary.
- Watch and steer long-running work — command line only. No GUI journey; progress appears inline where the work was started. Not settled by the H15 crawl either: job rows only render once a job exists and nothing seeded one, so this row is unmeasured rather than known absent.
- Keep a library healthy — command line only. No GUI journey; maintenance is command line work.
- Publish a subset of your notes — command line only. Not in the GUI yet. Publishing writes to a local directory, so the desktop app could do it and the browser mode could not.
- Notebooks that are really saved searches — command line only. Made with `notebooks create --query`, since a query notebook is a notebook whose contents are whatever matches. The GUI lists them in the sidebar and does not create them yet.
- Back up and restore the whole library — command line only. Not in the GUI yet. The constraint is the *browser* mode, which cannot read arbitrary local paths; the desktop app can, and native directory pickers are part of the planned client architecture. This is a gap rather than a boundary.
- Choose where sync keys are kept — command line only. Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- Move a pre-0.8 library into place — command line only. Command line only, and genuinely so: it relocates the directories the running program uses, which is not something the program can sensibly do to itself while serving them.
- Read a note and its structure — GUI only. No command line: reading a note is what the GUI and the API are for.
- Search your notes — GUI only. No command line search command; `notriosctl export archive --query` applies the same language to an export instead.
- Attach and manage files — both surfaces, and neither journey is written yet
- See the shape of the link graph — both surfaces, and neither journey is written yet
- Link notes to each other — both surfaces, and neither journey is written yet
- Bring remote images into the library — both surfaces, and neither journey is written yet
- See and end trust between replicas — both surfaces, and neither journey is written yet
- Recover a replica and resolve conflicts — both surfaces, and neither journey is written yet
- Keep separate libraries — both, but only the command-line journey is written
- Synchronize with a replica — both, but only the command-line journey is written
- Pair two of your own libraries — both, but only the command-line journey is written
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
