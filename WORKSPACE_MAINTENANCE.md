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

Block anchors should become first-class addressable objects in a later milestone.

## Trash-first deletion

Default delete should move documents to trash or soft-delete. Permanent deletion and resource garbage collection require explicit commands and retention policy checks.

## Shortest-unique resolver

Human-facing tools should accept stable IDs, titles, aliases, source IDs, or shortest-unique paths. Ambiguous resolution must return candidates rather than guessing.

## Hierarchical tag rename

Support dry-run tag rename, including child tags:

```json
{"from":"project","to":"work","include_children":true,"dry_run":true}
```
