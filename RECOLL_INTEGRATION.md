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
- Real-time incremental indexing and proven behavior beyond 100k documents.
- No separate server process to operate (vs. Meilisearch).

## Components to build

1. **Enhanced Markdown/front-matter handler** — a Python input handler with full YAML/TOML front-matter extraction, configured in Recoll's handler table for Markdown. It must be written **from scratch** (Recoll's shipped `rclmd.py` is GPL; see Licensing below) and emits fields as handler metadata/`<meta>` elements.
2. **Query adapter** — translates the Notrios query language to Recoll query syntax (see `SEARCH_QUERY_LANGUAGE.md`), including `since:`/`until:` → `publishedts:lower..upper` integer ranges.
3. **SQLite changes** — the projection/outbox tables designed for sist2 are reused; sist2-specific naming (config keys, capability flags, collection kinds) is renamed to sidecar-neutral or Recoll naming in plan task R2/R7.
4. **Adapter process management** — generate a dedicated Recoll config directory (fields declarations, `underscoreasletter`, watched projection dir), then drive `recollindex` and query via `recollq` or the Recoll Python API **in a separate process**.

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
