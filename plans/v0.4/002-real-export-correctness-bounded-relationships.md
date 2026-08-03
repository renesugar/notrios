# v0.4 J2 — Real-export correctness and bounded relationship planning

Status: complete

Date: 2026-08-02

Model: GPT-5 (Codex)

## Implemented slice

- Added explicit aggregate inventory reporting for every parsed `type_`, total
  metadata files/items, malformed Markdown metadata, unsupported item types,
  ignored non-Markdown files, missing resource content, and unresolved links.
- Kept the observed supported Joplin shapes explicit: type 1 notes, type 2
  folders, type 4 resources, type 5 tags, and type 6 note-tag relations.
  Sanitized fixtures also exercise malformed and future unsupported types.
- Replaced regex rewriting plus notes-by-all-resources discovery with one
  Markdown-aware note scan and direct ID-map lookups. It returns deduplicated
  source/target resource pairs for attachment planning and never searches every
  resource for every note.
- Preserved escaped `:/id` targets and targets in inline code, backtick fences,
  and tilde fences. Unresolved targets remain unchanged and are counted.
- Stopped retaining property-order slices twice (or at all when exact source
  preservation is disabled), reducing measured real-export planning time and
  memory.
- Changed Joplin CLI dry runs so omitted `--write-config` never creates a file
  in the source. The suggested configuration remains in the JSON report and an
  explicitly selected path still writes a loadable configuration.
- Added an opt-in private-safe real-export harness and committed only aggregate
  evidence under `performance/v0.4-j2/`.

## Real-format evidence

The production dry-run planner processed both user-supplied read-only exports:

- Recipe export: 1,237,553 items — 382,206 notes, 781 folders, 11,753 tags,
  and 842,813 note-tag relations. Zero malformed/unsupported items and zero
  warnings. The 382,206 notes exactly match the independently counted Markdown
  files in the paired Obsidian recipe vault. Full planning took 327.560 seconds
  with 3,769,552 KiB peak RSS.
- Attachment export: 111,330 items — 103,349 notes, 6 folders, 763 resources,
  322 tags, and 6,890 note-tag relations. Zero malformed/unsupported items;
  five resource records lacked content files and produced five warnings. The
  planner rewrote 2,420 links, retained 95 unresolved targets, and produced 766
  unique note/resource relationships. Full planning took 55.190 seconds with
  580,712 KiB peak RSS.

Both runs reported unchanged source-directory metadata. Reports contain no
source paths, filenames, titles, bodies, resource bytes, databases, or warning
text. The measured 1.24-million-item memory footprint is recorded as input to
J3's indexed manifest/spool decision; J2 does not claim full-write throughput.

## Correctness evidence

Focused tests cover:

- all five observed supported item types plus malformed, unsupported, and
  ignored input reporting;
- duplicate resource links producing one relationship;
- note and resource URI rewriting, unresolved preservation, escapes, inline
  code, backtick fences, and tilde fences;
- canonical title/body parsing, nested notebooks, real tags, exact source
  bundles, stable resource refresh, interruption/resume, idempotent re-import,
  search readiness, rewritten links, and attached resources;
- omitted versus explicit CLI dry-run configuration output.

## Validation evidence

- `go test ./internal/importers/joplinraw ./cmd/notriosctl`
- `bash scripts/run_real_joplin_profile.sh` equivalent full dry runs over both
  supplied corpora (the first measurements used the same test under
  `/usr/bin/time -v`; the checked-in harness adds peak RSS automatically)
- independent aggregate count of the paired Obsidian recipe vault
- full repository, frontend, docs, smoke, generated-profile, and release gates
  listed in the completion entry in `agent/ATTEMPT_LOG.jsonl`

J3 is the next incomplete plan task and requires user approval.
