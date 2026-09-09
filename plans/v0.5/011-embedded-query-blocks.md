# v0.5 E7 — Embedded query blocks

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## What it does

A fenced ```` ```note-query ```` block in a note renders as a live list of
matching notes:

````markdown
```note-query
query: tag:todo -tag:done
fields: notebook, updated
sort: updated
limit: 20
```
````

`POST /api/v1/note-queries/run` parses the block and runs it through the same
bounded Store search the search box, REST, and MCP already use.

## Decisions worth recording

**The block format is `key: value` lines, not YAML.**
`WORKSPACE_MAINTENANCE.md` sketched a nested YAML shape years ago. The service
carries no YAML parser and adding one for four directives is not a trade worth
making — and every filtering idea the sketch expressed is already expressible in
the Q1 query language `query:` accepts. Four keys exist: `query` (required),
`fields`, `sort`, `limit`.

**An unknown key is an error, and the error names the keys that work.** A block
that silently does less than it says is worse than one that refuses, and the
message has to be actionable from inside the note where it is rendered.

**Parsing is server-side.** The block reaches the same Q1 parser as every other
search surface, so a block can express nothing its author could not already type
into the search box, and there is no second grammar for a client and a server to
disagree about. That is the lesson E5 recorded about Markdown links, applied
again.

**A malformed block is a `200` carrying `error`, not a `4xx`.** The note has to
render; only the block should show a problem. Every failure in the parser and
the query language is a value on the result, not a returned error — a returned
error is reserved for a real storage or transport fault.

**Fields are opt-in and `title` is always present.** A block that asks for
nothing gets titles and URIs. A query block is not a way to pull note bodies
into a page that only wanted a list.

**E7 needed an explicit sort, so `SearchRequest` grew one.** Before this, the
order was implied by the query's shape: a positive text-only query ranked by
relevance and anything else traversed chronologically, with no way to ask. A
block saying `sort: updated` over a text query would silently have got relevance.
`SearchRequest.Sort` now forces the chronological path for a text query too, and
it is part of the cursor fingerprint — replaying a relevance cursor against a
chronological request would decode the wrong boundary. An empty `Sort` keeps the
old behaviour exactly, so nothing else changed.

**Truncation comes from the search's own cursor, not from over-fetching by
one.** `NormalizeSearchRequest` clamps a limit at 100, so asking for one extra
row would have been silently ignored at exactly the block ceiling — the case
where knowing there is more matters most.

**Truncation is always visible.** A list that silently stops is a list that lies
about the library.

**Nothing is built from note content as markup.** Every value the client renders
is written with `textContent` or as an element attribute. The block's source
travels to the placeholder on a `data-` attribute, so the serializer escapes it
and no note text is ever concatenated into HTML. A fixture asserts a title of
`<img src=x onerror=…>` renders as text and sets nothing on `window`.

**A block never blocks the note.** The note renders first with the block showing
"Running query…"; results arrive afterwards. An unreachable service leaves a
message inside the block. Navigating away aborts anything in flight, so a late
reply cannot write into a different note's preview.

## Export inertness

A publication carries the block's *text*, never a materialized result. A test
asserts it directly: a published note whose block queries `tag:private` — the
exact boundary a publication exists to protect — comes out byte-identical, fence
intact, with no trace of the withheld note's title.

That is the property that makes a block safe to publish. A materialized result
would leak what the query matched at export time and go stale immediately.

## What is deliberately not here

**`links_to: "$current"`.** The old sketch in `WORKSPACE_MAINTENANCE.md`
included a backlinks-of-this-note query. The Q1 language has no link operator,
and adding one touches the shared expression tree that SQLite and Recoll both
compile — that is its own slice, not a side effect of a rendering feature. E7's
scope is "the Q1 expression language plus a bounded, typed selection of fields,
sorts, and limits", and that is what shipped.

**Materialization on export.** The plan allows a profile to ask for it. No
profile does, and inventing the option before something needs it would add a way
to publish query results without a caller who wants one.

## Validation

`go vet ./...`, `go test ./...`, required files, plan-loop, scaffold validation,
OpenAPI parse, migration-copy equality, web typecheck/tests (107)/build,
`make gui`, docs-site build, Help reseed, REST/MCP smoke, performance smoke, and
the E6a offline-assets check.

No scale profile: a block is bounded to 100 rows through the search path that
already carries 10k/100k/500k evidence, so the measurement that matters was
taken in H7 and Q1.
