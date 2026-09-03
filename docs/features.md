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
- Write and edit notes — Create a note, change it, add to either end of it, and delete it. Deleting moves a note to Trash, and Trash is a place you can look in and take things back out of, not a countdown. (REST 10, MCP 6, GUI 3) No command line: notriosctl imports and maintains a library rather than authoring in it. Writing happens in the GUI or over REST/MCP.
- Read a note and its structure — Fetch a note whole, or just its body, its outline, its blocks, a line range, or an earlier revision. The structured views exist so a tool can work on part of a note without re-parsing all of it. (REST 7, MCP 5, GUI 1) No command line: reading a note is what the GUI and the API are for.
- Search your notes — Find notes by text, tag, notebook, date and the rest of the query language, across the library or within one note. (REST 3, MCP 2, GUI 1) No command line search command; `notriosctl export archive --query` applies the same language to an export instead.
- Notebooks that are really saved searches — Save a query as a notebook, so a view that would otherwise be retyped becomes something you open. (REST 3, MCP 1) No command line and no dedicated GUI journey yet; the sidebar lists them alongside ordinary notebooks.
- Organise notes into notebooks — Make notebooks, nest them, move notes between them, and see what a notebook deletion would take with it before agreeing to it. (CLI 1, REST 9, MCP 4, GUI 2)
- Tag and untag a note — Attach a tag to a note, or take one off. (REST 2, MCP 2)
- See and rename tags — List the tags in a library, see a note's tags, and rename a whole tag hierarchy at once. (CLI 1, REST 3, MCP 1) The GUI shows tags as sidebar navigation and does not rename them.
- Group libraries into collections — Collections sit above notebooks and are how an import keeps its material together. (REST 4, MCP 1) No command line or GUI journey; imports set the collection with --collection.
- Attach and manage files — Add files to notes, read them back, see what references what, and find attachments nothing points at any more. (CLI 1, REST 8, MCP 2) No GUI journey for attachment management yet.
- Bring remote images into the library — Find images a note points at on the web, check them against the domain policy, and copy the allowed ones in so the note stops depending on somebody else's server. (CLI 1, REST 4, MCP 3) No GUI journey; localizing is a maintenance action rather than an editing one.
- Link notes to each other — Get a stable link to a note or one of its sections, open one, check the links in a draft before saving, and register Notrios as the handler for notrios:// links. (CLI 3, REST 4, MCP 1, GUI 1)
- See the shape of the link graph — Look at what surrounds a note, find a path between two notes, and get a report on orphans, isolates and hubs. (CLI 2, REST 4, MCP 3, GUI 1)
- Templates and tasks — Keep note templates and make new notes from them, and see the tasks across a library. (REST 4, MCP 3) No command line or GUI journey.
- Live query blocks inside a note — Put a query in a note and have its results render where it sits. (REST 1, MCP 1) No command line; the GUI renders them in place.
- Do many organiser operations at once — Send a batch of moves, tags and notebook changes as one request, so a large reorganisation is one reviewable action. (REST 1, MCP 1) No command line or GUI journey; batching is for tools.
- Import from another application — Bring in a Joplin raw export, an Obsidian vault, a Twitter or X archive, a ChatGPT or Claude export, or a Notrios archive. Every importer dry-runs first. (CLI 6) Importing is command line only: it reads directories on this machine, which a browser cannot do and an API should not.
- Export your library — Write a portable archive of everything or a chosen subset, check one for compatibility, verify one, and restore one with an explicit intent. (CLI 5) Command line only, for the same reason as importing.
- Back up and restore the whole library — Take a physical snapshot of the database and its attachments, verify it, and restore it — replacing this library or adopting the snapshot as a new replica. (CLI 3) Command line only: a snapshot names paths on this machine, which is native-host work rather than a web request.
- Pair two of your own libraries — Enrol a library for synchronization, issue a single-use code, and spend it from the other side so the two learn each other's keys. (CLI 6, REST 8, GUI 1)
- Synchronize with a replica — Exchange changes with a paired replica, directly over an authenticated connection or through a folder you both can reach — a second drive, or a cloud folder mapped locally. (CLI 3, REST 9, MCP 3) No GUI journey runs an exchange; the sync center configures and reports rather than transferring.
- See and end trust between replicas — List the replicas a library trusts, revoke one's key, and retire a peer after reviewing exactly what retiring it means. (CLI 1, REST 4) The GUI shows peers in the sync center; retirement is reviewed there but has no registered journey yet.
- Recover a replica and resolve conflicts — Fetch a peer-verified backup, inspect one without installing it, work through sync conflicts, and decide what a shared attachment should do. (CLI 1, REST 9, MCP 1) No registered GUI journey; the sync center presents conflicts.
- Choose where sync keys are kept — An installed Notrios keeps the key that protects your sync material in the operating system's credential store. Move existing keys between that and the owner-only development file, in either direction. (CLI 1) Deliberately command line only: this item forbids a credential-management REST or MCP surface.
- Watch and steer long-running work — Imports, exports and syncs run as jobs you can list, inspect, cancel, retry and reset. (CLI 5, REST 5, MCP 4) No GUI journey; progress appears inline where the work was started.
- Keep separate libraries — Run more than one library on a machine — personal notes, work notes, a blog — each with its own database, its own settings and its own address. (CLI 7) Command line only by design: a profile registry is about this machine, and a service answering for one profile should not be able to reach another.
- Publish a subset of your notes — Choose what leaves the library, review exactly what a publication would include and withhold, save that choice as a profile, and publish only after agreeing to the reviewed plan. (CLI 4, REST 1, MCP 1) No GUI journey; publishing is reviewed at the command line.
- Keep a library healthy — Find what has rotted, fix what can be fixed mechanically, reclaim space, and check that this installation is set up the way you think it is. (CLI 8, REST 4, MCP 1) No GUI journey; maintenance is command line work.
- Move a pre-0.8 library into place — A library that lived in ./data next to the program is relocated into the directories an installed Notrios uses, after showing you the plan. (CLI 1) Command line only: it moves directories on this machine, and it reports rather than deciding for you.
- Let an AI assistant use your library — Notrios speaks MCP, so an assistant can read and, within a scope you grant, change your notes. (REST 2) The endpoint itself has no command line or GUI; the tools it exposes are listed against the features above.
<!-- notrios:generated:user:what-you-can-do:end -->


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
