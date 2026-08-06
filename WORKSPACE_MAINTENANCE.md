# Workspace Maintenance Features

These features come from the Foam/Obsidian/networked-notes design space and are not required for the first MVP unless explicitly pulled into `PLAN.md`.

## Embedded query blocks

Support safe embedded dashboard/query blocks later. They must be structured and permission-controlled, not arbitrary SQL or JavaScript.

Example future syntax:

```markdown
```note-query
filter:
  links_to: "$current"
select: [title, tags, backlink_count]
sort: backlink_count DESC
limit: 20
```
```

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

## Trash-first deletion

Default delete should move documents to trash or soft-delete. Permanent deletion and resource garbage collection require explicit commands and retention policy checks.

## Shortest-unique resolver

Human-facing tools should accept stable IDs, titles, aliases, source IDs, or shortest-unique paths. Ambiguous resolution must return candidates rather than guessing.

## Hierarchical tag rename

Support dry-run tag rename, including child tags:

```json
{"from":"project","to":"work","include_children":true,"dry_run":true}
```
