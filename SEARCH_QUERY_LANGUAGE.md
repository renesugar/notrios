# Search Query Language

Notrios exposes one user-facing query language across the GUI search box, REST search, MCP search tools, and search-notebook queries. A **query adapter** in the service translates it to the active backends: SQLite FTS5 (always available; implemented in `internal/query` + the store compiler, task R6) and Recoll/Xapian (optional sidecar, see `RECOLL_INTEGRATION.md`; compilation target for task R7). The adapter lives in the application — not in the Markdown parser and not as a Recoll core modification.

## Operators

| Syntax | Meaning | FTS5 | Recoll |
|---|---|---|---|
| `keyword1 keyword2` | implicit AND over body, title, and tags | native | native |
| `"multiple word phrase"` | phrase search | native | native |
| `title:keyword`, `title:"multiple words"` | title field | title column | native `title` |
| `author:alice`, `author:"Alice Smith"` | author display name | metadata filter | native `author` |
| `authorid:alice@example.social` | canonical account identity | metadata filter | custom field |
| `tag:toys`, `tag:"shopping mall"` | tag match | `note_tags` join | custom `tag` field |
| `notebook:"name"` | limit to a notebook and its sub-notebooks (case-insensitive; same-named notebooks all match) | notebook join | adapter-side filter |
| `since:2026-07-01` | on/after start of that date | timestamp filter | `publishedts:<epoch>..` |
| `until:2026-07-31` | through end of that date (23:59:59) | timestamp filter | `publishedts:..<epoch>` |

## Timestamp rules

- Store and display ISO 8601 (`2026-07-13T18:42:07Z`). Normalize internally to UTC Unix seconds; keep the original offset as stored metadata when it matters.
- Accept both `since:2026-07-13T18:42:07` and `...Z` forms; seconds precision is supported through the integer `publishedts` range field (Recoll's built-in `date:` operator supports calendar dates only, so the adapter never uses it for `since:`/`until:`).
- Date-only `since:` means `00:00:00`, date-only `until:` means `23:59:59`, in the selected timezone.
- Time-only values (`since:14:30:00`) mean *today at that time*. Cross-date time-of-day filtering, if ever added, uses a distinct operator (e.g. `time-after:`) backed by a `timeofday` seconds-since-midnight field — it must not overload chronological `since:`.

## Field indexing rules (Recoll backend)

- Tags are emitted twice: as the dedicated `tag` field (phrase-searchable: `tag:"note taking"`) and into `keywords`/general terms so unqualified searches match tag-only notes. Aliases get the same dual treatment (`alias:` + general terms).
- Multiword tags use the quoted form as canonical syntax. A canonical `tagid` (`shopping_mall`) may be indexed alongside the display value; if underscores become identifier characters, enable Recoll's `underscoreasletter`. Hyphenated canonicalization is avoided because Recoll's dehyphenation makes hyphenated tags unreliable as identifiers.
- `author` (display name) and `authorid` (canonical account) are separate fields so two people with the same display name are never conflated, and so `@`/domain punctuation tokenization never matters — the adapter canonicalizes account IDs before indexing and querying.

## Parsing rules (implemented)

- Unqualified terms are ANDed and search title and body; quoted spans are phrase matches.
- Repeated field filters AND together (`tag:a tag:b` requires both).
- Unknown `word:value` tokens are kept as literal search terms (so `re:invoice` or a pasted URL is never silently dropped).
- `author:` matches the display name case-insensitively (substring/phrase); `authorid:` matches the canonical identity exactly (case-insensitive).
- `since:`/`until:` compare the source `published_ts` when provenance exists, falling back to the note's local creation time.
- `is:trashed` queries search trashed notes with LIKE-based text matching (trashed notes have no FTS rows).
- Cursors are opaque, bound to the query + collection, and reject replay
  against a different search. Version `k2` tokens carry chronological
  `(updated_at, id)` or relevance `(score, id)` boundaries; SQL uses row-value
  keysets and has no offset ceiling.

## Reserved internal operators

- The empty query means "all non-deleted notes" (backs the "All notes" search notebook).
- `is:trashed` is reserved for the builtin Trash search notebook; deleted notes never appear in any other query.

## Result behavior

- Implicit AND, phrases, stemming, wildcards, and boolean expressions follow the backend's native behavior.
- Query weighting (title above body) is a later adapter feature; Recoll supports per-element weights natively.
- All search endpoints support cursor-based incremental results so a GUI can
  populate "All notes" lazily while scrolling (limits and cursor rules in
  `API_SPEC.md`).
- Chronological paging uses a versioned keyset cursor over
  `(updated_at DESC, id DESC)` and matching composite indexes. Relevance paging
  uses a reproducible `(score ASC, id ASC)` FTS5 boundary. Optional merged
  FTS5/Recoll results are frozen in a generation-labelled `m1` snapshot capped
  at 1,000 hits for at most ten minutes; `truncated` makes that bound explicit.
  Cursor version changes invalidate old tokens rather than misinterpreting
  them.
