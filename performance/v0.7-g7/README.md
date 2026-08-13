# v0.7 G7 — note revision objects, transfer deltas, three-way merge, conflicts

G7 is production code, not an investigation. This directory holds the evidence
that the shipped implementation transfers less than a full body, reconstructs
exactly, merges concurrent edits, and keeps overlapping ones visible.

The harness builds **real replicas** — the production `internal/store` on real
SQLite files — rather than a model. What it measures is therefore what the
service does, including its schema, its journal, and its admission path.

All content is generated. No private corpus, path, note text, or source hash is
read or recorded, and no database or resource leaves the temporary directory the
run creates and deletes.

## What is measured

Four offline intervals, matching the ones G1 selected: 1 hour, 1 day, 1 week,
and 30 days, standing for 1, 4, 12, and 32 edits per side. Two replicas edit the
same 24 generated notes concurrently. Two thirds of the notes receive edits in
disjoint bands of sections; one third receives edits to the same section from
both sides, so the report covers conflicts as well as clean merges.

For each interval:

- `transfer_bytes` — the revision operation payloads that actually travelled;
- `complete_body_bytes` — what the same revisions would have cost with every
  body inline, which is the counterfactual the delta gate exists to beat;
- `delta_revisions` / `inline_revisions` — how often the benefit gate fired;
- `clean_merges`, `conflicts`, `conflict_kinds`, `pending_reasons`;
- `converged_documents` — documents where both replicas agree on the current
  revision **and** the exact body.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g7-gocache \
  go test ./internal/syncbody/... ./internal/syncdelta/... ./internal/store/...

env GOCACHE=/tmp/notrios-g7-gocache \
  go test ./internal/syncdelta -run '^$' -parallel=1 \
  -fuzz '^FuzzDecodeVCDIFFNeverPanics$' -fuzztime=10s

env GOCACHE=/tmp/notrios-g7-gocache \
  go run ./performance/v0.7-g7/cmd/evidence \
  -out performance/v0.7-g7/transfer-results.json

PYTHONDONTWRITEBYTECODE=1 \
  python3 performance/v0.7-g7/validate_evidence.py
```

## Evidence

- `transfer-results.json` — the four intervals, their transfer and complete-body
  byte totals, delta selection counts, merge and conflict outcomes, and
  convergence;
- `FINDINGS.md` — what the numbers say, including the two limits they do not
  remove.

## Provenance

The transfer-delta codec in `internal/syncdelta` is the reviewed G7 promotion of
the G1a investigation prototype. Its complete license and behavioral-reference
record — RFC 3284, Apache Subversion, xdelta3, open-vcdiff — remains
`performance/v0.7-g1a/PROVENANCE.md` and applies unchanged to the promoted code.
No GPL source was consulted, and no external delta or merge library is a
dependency.

## Scope

These are desktop measurements on one host. G2's Android emulator and physical
device checklists still gate any mobile claim, and nothing here makes one.
