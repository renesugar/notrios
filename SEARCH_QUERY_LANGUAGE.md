# Search Query Language

Notrios exposes one user-facing query language across the GUI search box, REST search, MCP search tools, and search-notebook queries. A **query adapter** in the service translates it to the active backends: SQLite FTS5 (always available) and Recoll/Xapian (optional sidecar, see `RECOLL_INTEGRATION.md`). The adapter lives in the application — not in the Markdown parser and not as a Recoll core modification.

## Operators

| Syntax | Meaning | FTS5 | Recoll |
|---|---|---|---|
| `keyword1 keyword2` | implicit AND over body, title, and tags | native | native |
| `"multiple word phrase"` | phrase search | native | native |
| `title:keyword`, `title:"multiple words"` | title field | title column | native `title` |
| `author:alice`, `author:"Alice Smith"` | author display name | metadata filter | native `author` |
| `authorid:alice@example.social` | canonical account identity | metadata filter | custom field |
| `tag:toys`, `tag:"shopping mall"` | tag match | `note_tags` join | custom `tag` field |
| `notebook:"name"` | limit to a notebook (case-insensitive) | notebook join | adapter-side filter |
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

## Reserved internal operators

- The empty query means "all non-deleted notes" (backs the "All notes" search notebook).
- `is:trashed` is reserved for the builtin Trash search notebook; deleted notes never appear in any other query.

## Result behavior

- Implicit AND, phrases, stemming, wildcards, and boolean expressions follow the backend's native behavior.
- Query weighting (title above body) is a later adapter feature; Recoll supports per-element weights natively.
- All search endpoints support cursor-based incremental results so a GUI can populate "All notes" lazily while scrolling (limits and cursor rules in `API_SPEC.md`).
