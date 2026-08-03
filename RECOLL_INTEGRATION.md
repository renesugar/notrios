# Recoll Integration

Recoll (https://framagit.org/medoc92/recoll, features: https://www.recoll.org/pages/features.html) replaces sist2 as the derived extraction/search sidecar in the Notrios design. This document is design-normative for plan task R7.

## Division of responsibility

SQLite remains the canonical store; the Recoll/Xapian index is reconstructible search data, never primary storage.

```text
Markdown notes (canonical: SQLite)
    │
    ├── SQLite
    │     • stable note IDs, notebooks, tags
    │     • links/backlinks, thread/reply graph
    │     • attachment relationships, import state
    │
    └── managed filesystem projection (outbox-driven)
          │
          ▼
    enhanced Markdown front-matter handler
          • body, title, author/authorid
          • tags (field + general terms), aliases
          • publishedts / timestamps, noteid, threadid, replyto
          │
          ▼
    Recoll/Xapian index ── query adapter (SEARCH_QUERY_LANGUAGE.md)
```

Link-graph operations (direct replies, backlinks, thread ordering, orphan detection, rename-safe links, parent/child traversal) stay in SQLite. Recoll provides full-text search, phrases/proximity, field searches, boolean expressions, stemming, wildcards, spelling suggestions, custom metadata fields, custom numeric ranges, incremental/real-time indexing, snippets, previews, and indexing of PDFs/office documents/attachments alongside notes.

## Why Recoll (vs. sist2, vs. Meilisearch)

- Native field searches (`author:`, `title:`, quoted phrase values), implicit AND, custom fields, and integer range fields cover the target query language with only a thin adapter; only `since:`/`until:` timestamps need translation (Recoll's `date:` operator has no time-of-day support).
- v0.4 Q1 implements application-owned `OR`/negation/grouping and the
  `category:` alias. Recoll compiles the same bounded expression tree as
  SQLite rather than receiving unchecked backend-native query strings.
  Negation is lowered to leaf exclusions with De Morgan's laws; notebook
  ancestors and exact standalone-emoji keys are included in the derived
  projection. Unsupported shapes fall back explicitly to exact SQLite results.
- Real-time incremental indexing and proven behavior beyond 100k documents.
- No separate server process to operate (vs. Meilisearch).

Recoll is not the answer to SQLite cursor scaling: the canonical All Notes path
uses SQLite keyset pagination and matching indexes. When optional Recoll adds
sidecar-only hits, the service freezes the merged order in an immutable,
generation-labelled snapshot for every page (1,000-hit explicit cap, ten-minute
TTL, bounded cache). Sidecar failure still degrades to live FTS5 keysets.

## Components to build (implemented in task R7)

1. **Enhanced Markdown/front-matter handler** — `internal/recoll/notrios_md_handler.py`, a from-scratch (Apache-licensed, not derived from Recoll's GPL `rclmd.py`) Python `exec` filter with YAML/TOML front-matter extraction that emits `<meta>` fields; PyYAML is used when installed with a built-in fallback for the flat subset, TOML via stdlib `tomllib`. The generated config installs it for `text/markdown`.
2. **Query adapter** — `internal/recoll.CompileQuery` translates the bounded
   Notrios expression tree to fully parenthesized Recoll syntax, including
   boolean fields, leaf exclusions, `since:`/`until:` → integer ranges,
   recursive notebook/category metadata, and standalone emoji; `recollq -F`
   output is parsed back to document IDs via projection filenames.
3. **SQLite changes** — document mutations enqueue `index_outbox` jobs
   transactionally. `internal/projection` drains due jobs in bounded batches;
   failures persist exponential retry eligibility instead of spinning or
   blocking later jobs.
4. **Adapter process management** — `internal/recoll.Sidecar` generates the
   config directory and invokes user-installed `recollindex`/`recollq` only as
   cancellable argument arrays. Output, errors, fields, UTF-8, projection URLs,
   snippets, and result count are bounded. Startup performs exact projection
   reconciliation plus indexing; a 30-second worker drains bounded batches and
   a 10-minute pass repairs missing/stale/orphaned files. Status is exposed at
   `/api/v1/status.search_sidecar`.
5. **Merged search** — canonical FTS5 traversal and Recoll results are
   deduplicated by document ID, stale IDs are revalidated in 500-item batches,
   and contributing engines are returned in `sources`. Any Recoll contribution
   freezes the bounded merged order and attribution in the existing 1,000-hit
   snapshot for all cursor pages.

v0.3 H10 reuses the independently developed `recollwebui-go`
**behavioral lessons**, not its product code: argument arrays instead of shell
strings; strict bounded base64 field parsing; exact-count/query-drift checks;
bounded batch exports; hostile snippet-to-plain-text handling; subprocess
cancellation; stale result revalidation; accessible bounded result DOM; and
real Recoll/native Wails evidence on a generated 100k corpus. These behaviors
are implemented independently; its page-number offset model and product code
are deliberately not reused. Reproducible evidence lives under
`performance/v0.3-h10/`.

## Front-matter field mapping

```yaml
---
id: post-189734
title: Search engine architecture
author: Alice Smith
author_id: alice@example.social
published: 2026-07-13T18:42:07Z
tags:
  - search engines
  - note taking
thread_id: thread-918
reply_to: post-189700
---
```

| Front-matter property | Recoll field | Treatment |
|---|---|---|
| `title` | `title` | indexed, stored, generally searchable |
| `author` | `author` | indexed and stored |
| `author_id` | `authorid` | canonical identifier |
| `published` | `published` | human-readable stored value |
| `published` | `publishedts` | integer Unix timestamp for ranges |
| `tags` | `tag` + `keywords` | repeated searchable values, also fed to general terms |
| `aliases` | `alias` + `keywords` | same dual treatment |
| `thread_id` | `threadid` | exact/canonical field |
| `reply_to` | `replyto` | exact/canonical field |
| `id` | `noteid` | stable note identifier |

## Licensing boundary

Notrios code targets an MIT or Apache-2.0 license. Recoll and Xapian are GPL. Therefore:

- Recoll is an **optional, user-installed external tool**, invoked as a subprocess; it is never linked into Notrios binaries, vendored, or redistributed in Notrios release artifacts.
- The front-matter handler and Recoll config we generate are original Notrios code (no code derived from Recoll's handlers).
- When Recoll is absent, Notrios degrades gracefully to SQLite FTS5 search; no feature of the canonical store may depend on Recoll.

## Bluge boundary

Bluge is an Apache-2.0 embeddable Go search library with BM25, fuzzy/prefix/
phrase/range queries, custom sorting, highlights, aggregations/facets, and
search-after. It is not a Recoll replacement in the canonical application:
it does not provide Recoll's external extraction ecosystem, creates another
derived index, and the reviewed upstream has had no commit since 2022.

v0.4 may evaluate it behind the generated-site search adapter, where a
self-contained server can be valuable. Adoption requires a maintenance fork
plan, dependency/security review, 100k/500k build/query measurements, stable
cursor tests, and comparison with FTS5 and an external Recoll service.
