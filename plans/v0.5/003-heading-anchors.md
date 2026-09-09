# v0.5 E1a — Heading anchors in stable links

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## Why

E1 made blocks addressable; E2 had to leave heading anchors unchecked and said
so. A heading anchor is a slug, and block rows deliberately store a content hash
rather than heading text, so there was nothing to compare against. This closes
that gap: a `notrios://` or `document://` link can now name a section.

## What Obsidian does, checked

Verified on 2026-08-06 against `obsidian.md/help/links` and the Obsidian URI
help page:

- headings are named by **text**: `[[Note#Heading Text]]`, with nested
  subheading paths and a Markdown form `[Section](Example.md#Details)`;
- blocks use `[[Note#^block-id]]`; block IDs are Latin letters, numbers, dashes;
- `obsidian://open` supports both by percent-encoding the anchor onto the file
  parameter — `file=Note%23Heading`, `file=Note%23%5EBlock`, with `%23`, `%5E`,
  `%20`, `%2F`.

Two claims in the source material could **not** be verified and were not relied
on: an Alt/Option-modified "Copy obsidian URI" context-menu item, and explicit
`heading=`/`block=` parameters in the Advanced URI plugin — that plugin's schema
page documents only the general form and `vault`.

## What Notrios adopts, and what it does not

The model transfers: a heading is addressable by name, a block by ID, both from
outside the app. The percent-encoding does not. P5 decided the stable-link
parser refuses percent-escapes rather than decoding them, because an identifier
needing escapes is not one this application minted and decoding lets two
spellings name one target. So a `notrios://` heading anchor is the **slug**:

```text
notrios://databases/db_…/documents/doc_…#getting-started
```

Resolution normalizes whatever the caller wrote, so `#Install & Setup` and
`#install-setup` reach the same heading. Precedence is author marker, then block
ID, then heading slug — the name the author chose still wins.

## What it does

- Schema v15 `document_blocks.heading_slug` with a partial index, derived from
  heading text (lowercase, spaces to hyphens, non-alphanumerics dropped, Unicode
  preserved) and disambiguated by occurrence (`notes`, `notes-1`).
- Heading-anchor resolution in `notrios://` and `document://` links, with
  `stale_anchor` when a heading is renamed.
- The `unresolved_heading_anchor` lint check E2 could not implement.
- `heading_slug` on `GET /api/v1/documents/{id}/blocks`, and per-anchor backlink
  counts that fold both spellings onto one heading.
- `notriosctl link --anchor` (slug, heading text, `^marker`, or block ID) and
  `--list-anchors`. An anchor that does not resolve is refused rather than
  printed: a stable link is meant to be pasted somewhere permanent, and one that
  never worked is worse than no link.

## Decisions worth recording

**A heading that slugs to nothing gets no slug.** An emoji-only or
punctuation-only heading is reachable by its block ID; inventing a name would
make two unrelated headings collide.

**Repeated headings stay addressable.** Obsidian resolves a duplicate heading to
the first match and offers no way to name the second; a numbered suffix is the
convention Markdown renderers already use and costs nothing.

**The heading check runs in Go, not SQL.** Matching an anchor means slugifying
it with the parser's Unicode rules. The check rides E2's single link scan and
looks up each target's slugs through a bounded 512-document cache, so a note
linked from a hundred places costs one query and the pass still holds nothing
proportional to the library.

## What a test taught me about Markdown

A fixture written as `[x](document://…#Install & Setup)` did not behave: in
Markdown a space ends an unquoted URL, so the anchor truncated to `Install`. The
heading-text spelling belongs in a wikilink (`[[Note#Install & Setup]]`), which
is exactly the form Obsidian uses. That is independent support for the slug
decision — the slug form is the one that works in every syntax — and it is now
documented for users rather than left as a surprise.

## Measurements

`performance/v0.5-e1a/`, 100,000 notes. Adding heading anchors cost nothing
measurable: lint 3.20 s → 3.05 s, save 4.17 ms → 4.16 ms, database 114 MB
unchanged, peak RSS 87 → 89 MB. Anchor resolution moved 0.276 → 0.404 ms p95 on
a sub-millisecond note-scoped lookup that gained a third `OR` branch; both that
and the lint number are close to run noise and are reported as measured rather
than re-run until they read tidily.

## Validation

- parser fixtures: slug derivation across punctuation, Unicode, snake case,
  repeated spaces, the length bound, and headings that slug to nothing;
  occurrence disambiguation; renaming changing both slug and block ID;
- store fixtures: slugs on headings only, slug and heading text resolving to one
  block, a renamed heading ceasing to resolve, marker-before-ID-before-slug
  precedence, backlink counts folding both spellings;
- migration fixture: v14 → v15 without losing rows, without inventing slugs, and
  filled in by rebuild;
- REST fixtures: exposed slug, `stale_anchor` after a rename;
- CLI fixture on the real binary: `--anchor` by slug, by heading text, by
  marker; refusal of an unresolvable anchor; the anchored link opening;
- lint fixture: a heading anchor that no longer resolves;
- `go vet ./...`, `go test ./...`, required-file, scaffold, OpenAPI parse,
  migration-copy equality, docs site, `npm run typecheck`/`test`/`build`,
  `make gui`, `scripts/mvp_smoke.sh`, `scripts/run_performance_smoke.sh`;
- generated 100k profile under `performance/v0.5-e1a/`.

## Next task

E3, workspace fix, requires user approval.
