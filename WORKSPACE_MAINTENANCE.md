# Workspace Maintenance Features

These features come from the Foam/Obsidian/networked-notes design space and are not required for the first MVP unless explicitly pulled into `PLAN.md`.

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

## Embedded query blocks

Implemented in v0.5 E7. A fenced block declares a Q1 query plus a typed
selection of fields, a sort, and a limit — never SQL and never JavaScript:

````markdown
```note-query
query: tag:todo -tag:done
fields: notebook, updated
sort: updated
limit: 20
```
````

Four keys exist — `query` (required), `fields`, `sort`, `limit` — and an unknown
one is an error naming the keys that work. The block is parsed **server-side**
by the same Q1 parser every search surface uses, through
`POST /api/v1/note-queries/run`, so a block can express nothing its author could
not type into the search box.

The earlier sketch here used nested YAML with `links_to: "$current"`. Neither
survived. The service carries no YAML parser and four directives do not justify
adding one; a line-oriented `key: value` form needs none, and everything the
sketch expressed as nested filtering is already in the query language.
`links_to` needs a link operator in Q1, which touches the expression tree
SQLite and Recoll both compile — its own slice, not a side effect of a rendering
feature.

A malformed block renders its error inside the block and leaves the note alone;
truncation is always visible; and a publication carries the block's text rather
than a materialized result, so a published note cannot leak what the query
matched at export time.

## Lint and auto-fix

Lint is implemented in v0.5 E2 as a read-only report (`notriosctl lint`,
`GET /api/v1/admin/lint/report`) covering broken document/resource links,
ambiguous wikilinks, unresolved block anchors, duplicate external identities,
missing titles, unlocalized remote media, missing alt text, unreferenced
resources, and projection backlog. Heading anchors joined the list in v0.5 E1a,
once schema v15 gave a heading anchor a stored slug to compare against.

Auto-fix arrived in v0.5 E3 as `notriosctl fix`: dry run by default, single-note
and revision-preconditioned, every repair an ordinary revision
(`PROJECT_DECISIONS.md` 18). It covers non-canonical link targets (default),
alt text from the resource filename (opt-in), and remote-media localization
through the media policy (opt-in). Bulk operations remain the v0.6 organizer's.

Two repairs the plan listed are deliberately absent. Stale link reference
definitions are not fixed because they are not detected: Markdown reference
definitions are outside the link extractor, so the check and its repair belong
together in a later slice rather than fixing something lint cannot find.
Wikilinks are not rewritten into canonical Markdown links, because that replaces
the syntax the author chose rather than repairing it.

Workspace lint should detect:

- missing titles/frontmatter IDs;
- duplicate IDs;
- broken document/resource links;
- ambiguous Wikilinks;
- stale link reference definitions;
- unlocalized remote images;
- unreferenced resources;
- missing alt text;
- unresolved block references;
- projection/index drift.

Auto-fix must support dry-run mode and use revision preconditions.

## Outlines and blocks

Long documents, imported conversations, and research notes need outline and block APIs:

```text
GET /api/v1/documents/{id}/outline
GET /api/v1/documents/{id}/blocks
```

Block anchors became first-class addressable objects in v0.5 E1: schema-v14
`document_blocks`, `GET /api/v1/documents/{id}/blocks`, and anchor resolution
for `document://` and `notrios://` links. Identity is content-based, so an
anchor survives a block moving and breaks when the block's text changes. E1a
added heading anchors on the same footing: schema-v15 slugs, resolution by slug
or heading text, and a `unresolved_heading_anchor` lint check.

## Graph traversal and the orphan/hub report

v0.5 E4 turned the MVP graph slice into bounded traversal:
`POST /api/v1/graph` follows links to a requested depth, `POST /api/v1/graph/path`
finds a shortest path between two notes from both ends, and
`GET /api/v1/graph/report` lists orphans, isolates, and in-degree hubs from one
ordered scan.

v0.6 F5 added the presentation around it — a local neighbourhood view in the
GUI, the report written as a read-only note in the builtin Reports notebook
(`POST /api/v1/graph/report/note`, `notriosctl graph report --write-note`), and
CSV node/edge export for tools built for graph analysis — plus the filters that
make the numbers mean what they claim: trashed notes and system-authored notes
are neither measured nor counted at either end of a link. The trashed-source
half of that was a pre-existing defect, since soft delete deliberately leaves
`document_links` intact so a restore can use them.

The orphan report and lint's `unreferenced_resource` check answer different
questions and both belong here: lint finds a resource nothing points at, while
the graph report finds a *note* nothing points at. A note nobody links to is not
a defect — plenty of notes are entry points — so it is a report rather than a
lint finding.

Ceilings are refused rather than clamped, and a traversal stopped by one names
it. A shortest-path search reports `no_path`, `depth_exhausted`, and
`budget_exhausted` separately, because only the first is a statement about the
library.

## Trash-first deletion

Trash-first deletion has been the store's rule since the MVP: `DELETE` on a
document soft-deletes it, `GET /api/v1/trash` lists what is there,
`POST /api/v1/trash/{id}/restore` brings it back, and
`DELETE /api/v1/trash/{id}` purges — behind an object-specific confirmation
header, and never for an externally-sourced note.

v0.5 E8 made it reachable without the CLI. The editor pane offers **Move to
Trash** on an editable note; a trashed note opens with an "In the Trash" badge
and two offers, **Restore** and **Delete forever**. A trashed note and a Help
note are both uneditable, but only one of them can be brought back, so they do
not share a badge — calling a trashed note "read-only" would hide the one thing
its reader can act on.

Deleting a notebook is trash-first too, and what it does to the notes inside is
not guessable: they move to the Trash **and are re-homed to the default
notebook**, so a later restore always has a destination.
`GET /api/v1/notebooks/{id}/deletion-preview` reports that before it happens —
notebooks removed, notes affected, and where they land — and the GUI confirms
with the service's answer rather than a generic "are you sure". A protected
notebook previews as `deletable: false` with a reason, so the sidebar can omit
an affordance that would only ever fail.

Permanent deletion and resource garbage collection still require explicit
commands and retention policy checks.

## Shortest-unique resolver

Human-facing tools should accept stable IDs, titles, aliases, source IDs, or shortest-unique paths. Ambiguous resolution must return candidates rather than guessing.

## Hierarchical tag rename

Implemented in v0.5 E8 as `POST /api/v1/tags/rename`,
`store.RenameTag`, and `notriosctl tags rename`:

```json
{"from":"project","to":"work","include_children":true,"dry_run":true}
```

`dry_run` defaults to **true** on both the REST and CLI surfaces. A caller that
forgets it gets a report, and the only way to change the library is to say so.

The report is not a prediction. The service runs the rename inside a
transaction and rolls it back for a dry run, so the numbers come from the
statements that would do the work. A rename with cascading merges is exactly
where a predictor and an applier drift apart, and it is the case where being
wrong is expensive.

Hierarchy is matched by path segment: `project/alpha` is a child of `project`
and `projects` is not. Renaming onto a name that already exists is a **merge**,
reported per tag with `notes` (how many carried the source) and `notes_gained`
(how many actually change). Renaming a tag into its own subtree is refused,
because the result would depend on the order tags happened to come out in.

Two things it deliberately does not do. It does not rewrite note bodies: tags in
Notrios are relational, and a `#project` in someone's prose is prose. It does
not rewrite saved searches that mention the old name — it names them in
`warnings` instead, because guessing which occurrences of a word are the tag is
the kind of guess that silently changes what a search means.

The ceiling is 500 tags per rename, refused rather than truncated: a
half-renamed hierarchy is worse than no rename. The GUI does not offer tag
rename; E8 scoped it to Store/REST/CLI.
