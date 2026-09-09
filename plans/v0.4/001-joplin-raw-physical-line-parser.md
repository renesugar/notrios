# v0.4 J1 — Joplin RAW physical-line parser and canonical title/body compatibility

Status: complete

Date: 2026-08-02

Model: GPT-5 (Codex)

## Review basis

Compared:

- `internal/importers/joplinraw/joplinraw.go`, `scalable.go`, and tests;
- `movenotes-v3/joplin2sql.py`, `notesdb.py`, preservation/OCR tests, and
  canonical sample RAW items;
- the read-only corpus shapes supplied in the follow-up prompt (aggregate file
  counts only; no private content was copied or committed);
- Notrios' publishing/search roadmap against `movenotes-v3` and
  `hugo-theme-ledger` behavior and measured scale documentation.

## Findings

1. Canonical Joplin RAW stores an item title on the first physical source line.
   Notrios' synthetic fixtures commonly invented a `title:` property. Real notes
   could therefore receive their Joplin ID as the Notrios title while retaining
   the source title in the Markdown body.
2. The old parser split on CR/LF rather than Python-style Unicode line
   boundaries, but it had no OCR-control regression and used `TrimSpace` on
   whole property lines/values. That could discard significant delimiter
   whitespace or leading/trailing control/Unicode whitespace. The independent
   property-order parser could also drift from the effective-field parser.
3. Property names were restricted to an ASCII identifier pattern even though
   loss-preserving future fields should accept any nonempty single-line key
   without a colon. Effective maps necessarily use the last duplicate value,
   but source property order must retain every duplicate.
4. Byte-to-string conversion did not reject invalid UTF-8 or explicitly handle
   a BOM.
5. Remaining performance/correctness work is substantial and separately
   planned: attachment discovery is currently proportional to notes × all
   resources, complete imports still call per-document canonical transactions,
   whole inventories/maps grow with the source, and H8's largest committed tier
   profiles dry run rather than a complete 100k/1M write.

## Implemented slice

- Added one CR/LF-only physical-line splitter and one ordered-property parser.
- Canonical trailing-metadata items derive titles from their first source line;
  note bodies omit that title and its separator. The legacy metadata-first form
  remains accepted as a compatibility path.
- Property parsing consumes at most one delimiter space, retains other value
  whitespace/control characters, accepts future key spellings, records duplicate
  keys in order, and uses the same parse for source-bundle property order.
- Added UTF-8 BOM support and invalid-UTF-8 rejection.
- Replaced the primary importer and resource-reference fixtures with canonical
  first-line-title RAW shapes.
- Added focused tests for CRLF, OCR vertical-tab/form-feed/file-separator/
  record-separator/NEL data, duplicate/future keys, whitespace, terminal blank
  lines, BOMs, invalid UTF-8, source title/body separation, source bundles, and
  idempotent resources.

## Planning decisions

- J2 validates real read-only exports and makes relationship planning
  proportional to actual links; J3 adds true bounded canonical batch writes and
  million-note full-import evidence.
- Q1 adds `OR`, implicit `AND`, prefix negation, grouping, phrases, and
  `category:`/All-notes semantics through one bounded FTS5/Recoll expression
  tree.
- Notrios will export archive v2 and own publication selection/privacy.
  A separately maintained `movenotes-v3/notrios2sql.py` importer will own
  Obsidian, Quartz, and Hugo/Ledger+Bluge output. No publishing code was copied
  or linked across repositories.

## Validation evidence

- `go test ./internal/importers/joplinraw`
- `go test ./...`
- `go vet ./...`
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- `cd web && npm run typecheck && npm test && npm run build` (38 tests)
- `bash scripts/build_docs_site.sh`
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
- `bash scripts/run_joplin_import_profile.sh 100 /tmp/notrios-v0.4-j1-profile.json`
  (100 notes; dry-run + interrupted/resumed real import; 100 notes and 20
  resources imported)
- Dry run and real import of `movenotes-v3/sample/source2`: 6 notes, 2 nested
  notebooks, 1 tag, 1 resource, 1 attachment reference; zero ID-derived note
  titles.

The release ZIP and smoke/profile results are recorded in the completion entry
in `agent/ATTEMPT_LOG.jsonl`.
