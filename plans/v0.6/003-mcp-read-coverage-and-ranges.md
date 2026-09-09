# v0.6 F3 — MCP read coverage, resource reads, and HTTP Range

Status: complete on 2026-08-07.

Model: Claude Opus 5 (Claude Code).

## What it does

Seven read-shaped surfaces became MCP tools under the `read-only` scope:
`get_document_blocks`, `get_graph`, `find_graph_path`, `get_graph_report`,
`run_note_query`, `get_lint_report`, and `read_resource`. REST gained HTTP
`Range` on resource content, which is what makes bounded resource reads possible
without a second mechanism.

## Decisions worth recording

**Every withheld surface was decided, not inherited.** The deliverable was the
list itself. What stays off, and the reason for each, is now a table in
`docs/api/mcp.md`: fix, tag rename, notebook deletion and its preview, garbage
collection, archive export/verify/restore, publication, and purge. The pattern
underneath is one sentence — *anything that writes outside the note model,
deletes permanently, or acts on the whole library at once stays a deliberate act
on the command line* — and batch organizing is the exception that proves it,
being bounded to an explicit ID list, revision-preconditioned, and gated behind
`organizer`.

**Range came from `http.ServeContent`, not from an implementation.** It already
handles `Range`, `If-Range`, multi-range refusal, `Content-Range`, and `416`
correctly. The handler was doing `io.Copy`; it now type-asserts the store's
`io.ReadCloser` to an `io.ReadSeeker` — it is an `*os.File` in practice — and
delegates, keeping `OpenResourceContent`'s interface unchanged. A non-seekable
source falls back to a full copy without ranges, which is the honest outcome
rather than a wrong one.

The test asserts `Content-Length` describes the *slice*, because the handler
sets the full length before delegating and the assertion is what proves the
delegation overrides it.

**`read_resource` returns metadata by default and bytes only on request.**
Streaming attachment bytes into a model's context by accident is exactly what
the bulk-versus-control-plane split exists to prevent. Bytes come back only for
text-like MIME types — an allowlist (`text/*` plus JSON, XML, YAML, TOML, SQL,
JavaScript, SVG) rather than a "not binary" guess — and only within
`mcp.max_document_bytes`.

**A slice that ends mid-character is trimmed back.** Otherwise a range read
hands a model a replacement character it will treat as content. The reader takes
one byte more than the limit so `truncated` can distinguish "there is more" from
"that was exactly all of it", which a caller needs in order to know whether to
ask for the next slice.

**Lint reaches MCP; fix does not.** Lint findings are content-free by
construction — a document ID, a line and column, a reason code, and a SHA-256 of
the offending target, never the target text, because a broken wikilink's raw
text is frequently the title of a private note. A model may see what is broken
without being able to rewrite notes in bulk.

## Two things the tests do that matter

**The scope test caught the new tools before I updated it.** `TestEachScopeListsExactlyItsTools`
asserts each scope's tools as a *whole set*, so adding seven tools failed it
immediately with a diff. That is the guard F2 built working on its first real
use — a spot-check on membership would have passed silently.

**The lint leak test asserts a finding exists first.** Checking only that the
report omits the offending target would pass on an empty report, which proves
nothing. It now asserts a `broken_document_link` finding with a `target_sha256`
is present *and* that the raw target is absent.

## Validation

`go vet ./...`, `go test ./...`, required-file, plan-loop, and scaffold checks,
OpenAPI parse, migration-copy equality, web typecheck/tests/build, `make gui`,
docs-site build, Help reseed, REST/MCP smoke (extended with a range read, a 416,
and `read_resource` returning no bytes unasked), performance smoke, and the
offline-assets check.

No scale profile: every new tool reaches a Store operation that already carries
one, and `Range` is a seek plus a bounded copy.
