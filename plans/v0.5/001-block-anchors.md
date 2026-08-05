# v0.5 E1 — Block anchors and block-level addressability

Status: complete on 2026-08-05.

Model: Claude Opus 5 (Claude Code).

## Why

`document_blocks` had been "planned" since the MVP schema. Heading and block
anchors existed only as text on link records, so nothing could answer "what
blocks does this note have" or "what links point at this paragraph". The rest of
v0.5 — lint's unresolved block references, the graph's block-level edges, the
editor's in-buffer markers — all need a block model underneath.

## The decision this rests on

Block identity is **strictly content-based** (user decision, 2026-08-05;
`PROJECT_DECISIONS.md` 17). A block's ID derives from its text, so moving a
block keeps its ID and editing its text mints a new one. An anchor therefore
names exactly the text it was written against, and a rewritten block breaks
links into it rather than silently redirecting them at replaced content.

Three consequences were settled in the plan before implementation:

- **Duplicate text needs a disambiguator.** The hash covers the document ID, the
  block kind, the normalized text, and the occurrence index among identical
  blocks in that note. Two identical paragraphs stay distinguishable without
  position becoming identity.
- **Normalization is part of the contract.** Line endings are normalized and
  trailing whitespace trimmed before hashing, so an editor that tidies a file on
  save does not break every anchor in it. Nothing else is touched: case,
  punctuation, and emphasis are content.
- **Authored `^markers` outrank derived IDs.** Obsidian-style markers already
  arrive through the importer and already resolve as link anchors. They are
  names the author chose, and they survive edits the derived ID does not, so
  resolution tries the marker first.

Document scope is the fourth property, and it is a privacy one: the same
sentence in two notes produces two different block IDs, so a block key never
lets one note's content be recognized in another.

## What it does

- Schema v14 `document_blocks` — ID, document, ordinal, kind, heading level,
  marker, content hash, byte range — rebuilt from the body inside the same
  transaction as the save that produced it, and removed with the document.
  The migration file now creates through v14 and also folds in v13's
  `restore_state`, which had lived only in a shim and would have left a fresh
  database's `user_version` going backwards once v14 was appended.
- `internal/markdownblocks` splits headings, paragraphs, list items, fenced code
  blocks, and tables, bounded at 10,000 blocks per note.
- `GET /api/v1/documents/{id}/blocks` lists a note's blocks with per-block
  backlink counts and returns **no block text**: the response addresses content
  rather than handing it out, and a caller that wants the text reads the body,
  which is already an authorized read.
- `notrios://` and `document://` anchors resolve to blocks. A new `stale_anchor`
  status reports the note being present while the block is not — the visible
  consequence of content-based identity — and still names the note so a client
  can offer to open it.
- `RebuildDocumentBlocks` fills in blocks without writing a revision, which is
  how a database upgraded to v14 gets rows for notes nobody has edited since.

## Measurements

`performance/v0.5-e1/`, at 10k/100k/500k notes.

The generated seeder writes rows with raw SQL and so produces no blocks. The
profile therefore measures the real save path over a bounded 200-note sample,
and separately fills the block table synthetically to library scale to test
index behaviour at real row counts. Those are labelled differently on purpose,
and nothing claims the parser ran at 500,000 notes.

| Tier | Block rows | DB before → after | Anchor p95 at scale |
|---|---:|---|---:|
| 10k | 61,200 | 12 → 25 MB | 0.423 ms |
| 100k | 601,200 | 114 → 245 MB | 0.544 ms |
| 500k | 3,001,200 | 570 → 1,229 MB | 0.307 ms |

Anchor resolution is flat from 61,000 to 3,001,200 rows, because every lookup is
scoped to one document and served by the index. Peak RSS is unchanged. The real
cost is disk: blocks roughly double the database at six blocks per note, which
is what a second index over every note's text costs. Row width was held to what
an anchor needs — a hash and a byte range, never block text.

## Validation

- parser fixtures: the five kinds, unclosed fences, tables, the per-document
  bound, determinism, and the identity decision directly — a moved block keeps
  its ID, an edited one does not, the same text in two notes is two blocks,
  identical blocks are disambiguated by occurrence, a heading and a paragraph
  reading the same are distinct, and CRLF or trailing whitespace changes nothing;
- store fixtures: rebuild inside the save transaction, removal with the
  document, marker-outranks-ID including after an edit that breaks the derived
  ID, backlink counts for either spelling, and `RebuildDocumentBlocks` writing no
  revision;
- REST fixtures: the listing, its 404, the absence of block text in the
  response, and `stale_anchor` reported distinctly while still naming the note;
- `go vet ./...`, `go test ./...`, required-file, scaffold, OpenAPI parse,
  migration-copy equality, docs site, `npm run typecheck`/`test`/`build`,
  `make gui`, `scripts/mvp_smoke.sh`, `scripts/run_performance_smoke.sh`;
- generated 10k/100k/500k profiles under `performance/v0.5-e1/`.

## Next task

E2, workspace lint, requires user approval.
