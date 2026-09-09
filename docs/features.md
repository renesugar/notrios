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

Write a note, change it, add to the top or bottom of it, and throw it away. Deleting puts a note in the Trash, which is somewhere you can look and take things back out of rather than a countdown — nothing is destroyed until you empty it.

All four surfaces write notes. At a terminal, `notes create` takes a body from a file or a pipe, so a note can be the end of a pipeline, and `notes append` and `notes prepend` add to a note without rewriting it — which matters because no surface can patch a range of a body. Deleting and restoring are deliberately on the same surface: a delete whose undo lives somewhere else is a poor boundary.

*Available on the desktop app, the command line, the REST API and MCP.*

### Read a note and its structure

Open a note whole, or take just the part you need: the body, an outline of its headings, one of its blocks, a range of lines, or an earlier revision. The structured views exist so a program can work on part of a note without parsing the rest.

The command line prints a note as Markdown with front matter, so another application can read it as a file, and `--json` gives the fields instead; `notes outline`, `notes resources` and `notes links` answer what a note is made of. Blocks, line ranges and revisions are on REST and MCP only — they are for programs working inside a note, and a terminal has `notes show` and a text editor.

*Available on the desktop app, the command line, the REST API and MCP.*

### Search your notes

Find notes by text, tag, notebook, date and the rest of the query language — one language, whether you type it into the search box or hand it to a command. You can search a whole library or inside a single note.

A search is what turns a question into something you can act on: every hit carries the note's identifier, which is what `notes show`, `notes move`, `tags add` and the batch operations take. `--count` answers how many without paging through them, and `--links` prints the `notrios://` form for pasting into another machine's library. A search spans every collection unless `collection:` narrows it, and leaves the Trash out unless the query says `is:trashed`.

*Available on the desktop app, the command line, the REST API and MCP.*

### Notebooks that are really saved searches

Save a query as a notebook, so a view you would otherwise retype becomes something you open.

At a terminal this is `notebooks create --query`. In the interface it is offered on a search that has already run and returned something, rather than as a form to fill in: a notebook built from a query nobody has watched work opens empty as easily as it opens right. The query is shown and is not editable there, which is what keeps it from becoming a second search box.

*Available on the desktop app, the command line, the REST API and MCP.*

### Organise notes into notebooks

Make notebooks, nest them inside each other, move notes between them, and see what deleting a notebook would take with it before you agree to it.

`notebooks list` prints a table of names and identifiers, with `--json` for the structured form — the identifier is what a query and the note commands take, so finding it should not mean reading a JSON object at a terminal. Deleting a notebook shows what it holds first, on every surface that offers it.

*Available on the desktop app, the command line, the REST API and MCP.*

### Tag and untag a note

Put a tag on a note, take one off, and see what a note carries. Tags are hierarchical: `field/dusk` sits under `field`, and a search for `field` finds both.

*Available on the desktop app, the command line, the REST API and MCP.*

### See and rename tags

See the tags a library uses, ask whether one exists, and rename a tag — or a whole branch of them — everywhere at once.

`tags show` exits non-zero when a tag does not exist, so a script can test for one without parsing anything, and `--prefix` asks for one branch of the hierarchy rather than the whole vocabulary. A rename shows what it would change before it changes it, and a change described as a merge means the destination already exists and the two tags become one. Listing the notes that carry a tag is a search — `tag:todo` — rather than a second thing this reports.

*Available on the desktop app, the command line, the REST API and MCP.*

### See where notes came from

Notes you import stay marked with where they came from, so material brought in from somewhere else is distinguishable from what you wrote yourself — and you can search within one source or across all of them.

A collection is provenance rather than a place notes live: an import files notes into a notebook, which is what you browse, and marks them with a collection identifier alongside. A search spans every collection and `collection:` narrows it to one; `notriosctl collections list` says which exist and how many notes name each. Collections are made by an import naming one rather than by a command of their own, and the note inspector shows the row only when a note came from somewhere, because printing `default` on everything you wrote here would say nothing. A collection's name and description can be edited over REST, since renaming a label changes no provenance.

*Available on the desktop app, the command line, the REST API and MCP.*

### Attach and manage files

Attach files to notes, read them back out, see what references what, and find attachments nothing points at any more.

Where the link goes in your prose is yours, and the two surfaces differ because only one of them knows where you are typing. In the interface, attaching a file inserts the Markdown at the cursor and leaves the note unsaved. At a terminal, `resources add` puts the file in the library and prints its `resource://` URI without touching the body; `notes append` or `notes prepend` place it once you have decided where it belongs. The bytes are stored once and addressed by their content, so the same file attached twice is stored once.

*Available on the desktop app, the command line, the REST API and MCP.*

### Bring remote images into the library

A note that points at an image on somebody else's server stops working when that server does. Find those images, see what the policy decided about each one, and copy the allowed ones in so the note stops depending on anyone.

Localizing is offered where you are editing, because it rewrites the note. It is deliberately not a general downloader: every fetch goes through the domain policy, quarantine, hashing and size limits, so a note cannot be made to pull an arbitrary address into your library. The scan that lists them downloads nothing — each decision is made from the address alone.

*Available on the desktop app, the command line, the REST API and MCP.*

### Link notes to each other

Get a stable link to a note or to one of its sections, follow one, check that a draft's links still resolve before you save it, and register Notrios as the program that opens `notrios://` links.

*Available on the desktop app, the command line, the REST API and MCP.*

### See the shape of the link graph

See what surrounds a note, find the path between two of them, and get a report on the notes nothing links to, the ones that link to nothing, and the few that everything runs through.

*Available on the desktop app, the command line, the REST API and MCP.*

### Templates and tasks

Keep notes that are shapes for other notes, make new ones from them, and see the tasks scattered across a library in one list.

Both are ordinary Markdown rather than tables in a database: a template is a note carrying a note-template block, and a task is a checkbox line in a note tagged `task` or `todo`. So nothing creates a task — writing `- [ ] chase the permit` in a note is how one comes to exist — and what the command line adds is the other half: asking what remains, and filling in a template. A placeholder you leave out is refused rather than quietly left blank, so a template that gains a field fails the scripts that do not know about it. The tag is required because a checkbox is ordinary Markdown and turns up in quoted examples; `--untagged` asks for those too. Not in the interface yet.

*Available on the command line, the REST API and MCP.*

### Live query blocks inside a note

Put a query inside a note and have its results render where it sits, so a note can show what currently matches instead of a list somebody has to keep up to date.

A query block is a rendering rather than a command: the note carries a fenced query and the interface shows what it matches in place, which is a line rather than a gap. At a terminal the same question is `notriosctl search`, and formatting the answer is a template tool's job. A command that ran a block's query would be a second way to run a query.

*Available on the desktop app, the REST API and MCP.*

### Reorganise many notes at once

Reorganise a lot of notes in one go: move, tag, untag, duplicate, trash or restore everything a search finds, as one reviewable action.

At a terminal the set is named by a search rather than by a list of identifiers: `--query` on `notes move`, `notes delete`, `notes restore`, `notes duplicate`, `tags add` and `tags remove`. Nothing happens until you add `--apply` — without it the command shows the notes it would act on and stops — and a selection larger than the 500-note ceiling is refused rather than trimmed, because acting on the first five hundred of a thousand matches looks exactly like success. In the interface you tick the notes in the results list and the editor is replaced by the operations that apply to a set, because with several notes chosen there is no one note to read. Both send the same request, and both report every note rather than a total: a note that was skipped — a tag it already had, a notebook it was already in — is not a note that failed.

*Available on the desktop app, the command line, the REST API and MCP.*

### Import from another application

Bring in what you already have elsewhere: a Joplin export, an Obsidian vault, a Twitter or X archive, a ChatGPT or Claude export, or another Notrios archive. Every importer shows you what it would do before it does it.

Importing names a folder on the machine running the library, so it belongs to the desktop application and the command line and has no REST route at all; in a browser the control is shown and disabled with that reason rather than hidden. The desktop application handles Joplin, Obsidian and Notrios archives, and merges only — replace, fork and adopt stay at the command line because they change which library this is. Twitter, ChatGPT and Claude imports are command line only.

*Available on the desktop app and the command line.*

### Export your library

Write a portable archive of the whole library or a chosen part of it, check one against this version before trusting it, verify one, and restore one when you mean to.

Exporting names a folder on the machine running the library, so it belongs to the desktop application and the command line and has no REST route; in a browser the control is shown and disabled with that reason. The interface writes a complete archive; choosing a subset by notebook, tag or query stays at the command line, because choosing a subset means seeing what it selects first.

*Available on the desktop app and the command line.*

### Back up and restore the whole library

Take a copy of everything — the database, the attachments and the history — check that it is sound, and put it back later, either over this library or as a new replica beside it.

Snapshots name a folder on the machine running the library, so they are desktop and command line only. Taking one is in the interface; restoring one is not, because a restore replaces the library the window is showing. Restoring asks which you mean — replace this library, or adopt the snapshot as a separate replica — rather than choosing for you.

*Available on the desktop app and the command line.*

### Pair two of your own libraries

Teach two of your own libraries to trust each other: enrol one, issue a single-use code, and spend it from the other side.

No MCP tools, deliberately: an assistant can watch a synchronization run and cannot decide who is trusted. Pairing also reads and writes files you name, which is why it has no general REST route either. Pairing from a phone would go through the shared library rather than through MCP, and that surface carries no synchronization yet.

*Available on the desktop app, the command line and the REST API.*

### Synchronize with a replica

Exchange changes with a library you have paired: directly over an authenticated connection, or through a folder you can both reach — a second drive, or a cloud folder mapped into this machine.

*Available on the desktop app, the command line, the REST API and MCP.*

### See and end trust between replicas

See which replicas a library trusts, take one's key away, and retire a peer once you have read exactly what retiring it means.

No MCP tools, under the same rule as pairing: an assistant can watch a synchronization and cannot decide who is trusted. Retiring a peer is a permanent identity decision recorded with a signature, and revoking a key with `--advance-epoch` changes what every other replica will accept — both show you what they mean before they happen.

*Available on the desktop app, the command line and the REST API.*

### Recover a replica and resolve conflicts

When a replica is lost, or two of them disagree: fetch a backup a peer has verified, look inside one without installing it, work through the conflicts a synchronization could not settle, and decide what a shared attachment should do.

*Available on the desktop app, the command line, the REST API and MCP.*

### Choose where sync keys are kept

An installed Notrios keeps the key protecting your sync material in the operating system's credential store. Move existing keys between that and the owner-only development file, in either direction.

Credentials are managed from the command line and nowhere else, firmly: a remote route that can move a key is a remote route that can take one. An installed Notrios never silently falls back to a plaintext file, and nothing here prints, logs or exports a secret.

*Available on the command line.*

### Watch and steer long-running work

Imports, exports and synchronizations run as jobs rather than as a window you have to keep open. List them, look at one, cancel it, retry it, or reset one that is stuck.

The interface has the sync centre's list of recent synchronization jobs, with Cancel on one still running and Retry on one that failed or was cancelled — a retry resumes from the last verified checkpoint rather than from the beginning. Import, export and snapshot jobs do not appear there, and there is no general job list, no inspect and no reset in the interface yet; the command line, REST and MCP have all of it.

*Available on the desktop app, the command line, the REST API and MCP.*

### Keep separate libraries

Run more than one library on a machine — personal notes, work notes, a blog — each with its own database, its own settings and its own address.

Switching between profiles is in the interface; making, registering and forgetting them is command-line work, and a browser cannot manage them at all. The absence from REST and MCP is deliberate rather than unfinished: `profile create` names a directory to keep a library in and `profile start` launches a process, so a route that accepted those would let a caller point the service at any directory and start something.

*Available on the desktop app and the command line.*

### Publish a subset of your notes

Publish a chosen part of your library and nothing else. Review exactly what would be included and what would be held back, save that choice as a profile, and publish only after agreeing to the plan you were shown.

The review is the feature rather than a step before it, so it is in the interface, and the counts lead with what is withheld and which links get rewritten rather than with what is sent. `publish run` re-plans and refuses unless what it computes still matches what you reviewed. Profiles are read here and never written: a profile carries a target, a selection and a privacy policy, and a form that quietly defaulted one is how something private gets published, so authoring stays `notriosctl publish profile save`.

*Available on the desktop app, the command line, the REST API and MCP.*

### Keep a library healthy

Find what has rotted — broken links, attachments nothing points at, notes referring to things that are gone — fix what can be fixed mechanically, reclaim the space, and check that this installation is set up the way you think it is.

Reporting and repairing are in the interface and at the command line. Over REST the lint surface is read-only on purpose: that is the surface a program reaches, and repairs to somebody's notes should not be one call away from one. Every repair writes a revision against the revision it was planned on, so a note edited in between is refused rather than repaired against text nobody read. Reclaiming space stays `notriosctl gc --apply`, because it deletes blobs and there is no revision to go back to. `doctor`, `paths` and `config show` are command line only.

*Available on the desktop app, the command line, the REST API and MCP.*

### Move a pre-0.8 library into place

A library that lived in a `./data` folder next to the program moves into the directories an installed Notrios uses — after showing you the plan.

This one is genuinely command-line work: it relocates the directories the running program is using, which is not something a program can sensibly do to itself while serving them.

*Available on the command line.*

### Let an AI assistant use your library

Notrios speaks MCP, so an assistant can read your library and, within a scope you grant, change it. What it may touch is yours to decide, and establishing trust between replicas and managing credentials are outside every scope.

This is a surface Notrios serves rather than something you run: software with an MCP client connects to it, so there is nothing to demonstrate at a terminal or in a window — a journey would document the client rather than this program. The two REST operations listed here are how the endpoint is reached.

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
| See where notes came from | 2 | 2 | 4 | 1 | — |
| Attach and manage files | 2 | 4 | 8 | 2 | — |
| Bring remote images into the library | 2 | 1 | 4 | 3 | — |
| Link notes to each other | 1 | 4 | 4 | 1 | — |
| See the shape of the link graph | 1 | 2 | 4 | 3 | — |
| Templates and tasks | — | 3 | 4 | 3 | — |
| Live query blocks inside a note | 1 | — | 1 | 1 | — |
| Reorganise many notes at once | 5 | 1 | 1 | 1 | — |
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

- Let an AI assistant use your library — neither the command line nor the GUI; reachable only over REST or MCP. This is a surface Notrios serves rather than something you run: software with an MCP client connects to it, so there is nothing to demonstrate at a terminal or in a window — a journey would document the client rather than this program. The two REST operations listed here are how the endpoint is reached.
- Choose where sync keys are kept — command line only. Credentials are managed from the command line and nowhere else, firmly: a remote route that can move a key is a remote route that can take one. An installed Notrios never silently falls back to a plaintext file, and nothing here prints, logs or exports a secret.
- Templates and tasks — command line only. Both are ordinary Markdown rather than tables in a database: a template is a note carrying a note-template block, and a task is a checkbox line in a note tagged `task` or `todo`. So nothing creates a task — writing `- [ ] chase the permit` in a note is how one comes to exist — and what the command line adds is the other half: asking what remains, and filling in a template. A placeholder you leave out is refused rather than quietly left blank, so a template that gains a field fails the scripts that do not know about it. The tag is required because a checkbox is ordinary Markdown and turns up in quoted examples; `--untagged` asks for those too. Not in the interface yet.
- Move a pre-0.8 library into place — command line only. This one is genuinely command-line work: it relocates the directories the running program is using, which is not something a program can sensibly do to itself while serving them.
- Live query blocks inside a note — GUI only. A query block is a rendering rather than a command: the note carries a fenced query and the interface shows what it matches in place, which is a line rather than a gap. At a terminal the same question is `notriosctl search`, and formatting the answer is a template tool's job. A command that ran a block's query would be a second way to run a query.
- See and end trust between replicas — both, but only the GUI journey is written
- Recover a replica and resolve conflicts — both, but only the GUI journey is written
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
