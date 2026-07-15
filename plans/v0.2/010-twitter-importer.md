# v0.2 Task R9 — Twitter/X archive importer

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Changes

- `internal/importers/twitter`: parses an **extracted** Twitter/X archive (`data/tweets.js` or `tweet.js`, `account.js`, `tweets_media/`; `window.YTD.*.part0 =` wrappers stripped). Reference: doggy8088/x-archive-parser.
- Thread recovery: in-reply-to chains walked among archived tweets; a tweet's thread ID is its highest reachable archived ancestor; replies to unarchived tweets start their own thread but keep the external `reply_to`.
- Notes: deterministic IDs (`doc_twitter_<id>`), first-line titles, Markdown bodies with t.co URLs expanded, media embedded as `resource://` images (deduplicated by preferred resource IDs), and a "View on Twitter/X" footer link; notes land in a 🐦 "Twitter" notebook (`nb_twitter`, reused case-insensitively if a same-named notebook exists).
- Hashtags become tags; media files become content-addressed resources attached as `embedded`.
- Provenance: `source_system=twitter`, external tweet ID, display-name author + `@handle` author_id, thread/reply IDs, post URL, published time (Ruby-date parsed) — enabling `ListThreadDocuments` ordering and Trash purge protection.
- Re-import semantics: unchanged/updated detection; **user-trashed tweets are never resurrected** (provenance lookup guards recreation; only their provenance is refreshed); media/attachment counters only count real work.
- `notriosctl import twitter` with `--notebook` and `--dry-run` (dry run writes nothing, not even the notebook).
- `testdata/schemas/twitter-{tweets,account}.schema.json` derived from synthetic samples with `uvx genson` (per json-schema.org; no real exports in the repo).

## Validation

`go test ./...` (fixture test covering threads, media, tags, notebook, query-language reachability, purge protection, idempotent re-import incl. trashed notes, dry run), `check_required_files`, `validate-scaffold`, `mvp_smoke.sh` — all passing.
