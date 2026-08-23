# v0.7 G14b — Full-corpus baseline and physical snapshot selection — complete

Status: **complete**

Date: 2026-08-22

Model: GPT-5 (exact serving variant unavailable)

## Goal and boundaries

Select the scalable physical whole-library snapshot representation from
full-corpus evidence. This slice contains investigation/prototype code,
aggregate-only evidence, tests, and documentation. It changes no production
format, schema, dependency, default, encryption, or catch-up behavior. Private
source paths, filenames, content, hashes, databases, repositories, and detailed
logs remain outside the repository.

## Decision

Select option B: G14c must implement a required, versioned
`sqlite-image+packed-assets.v1` capability for compatible same-schema
whole-library backup and synchronization catch-up. It binds a consistent SQLite
Online Backup image, exact schema/application and snapshot-vector bounds,
database identity, and bounded external resource/source-bundle packs. Packed
semantic archive-v2 remains first-class for subset export, merge,
schema-independent interchange, and fallback recovery. Existing loose and
packed readers remain compatible.

G14c adds no compressor by default. Deterministic stored packs close before
256 MiB payload or 65,536 entries, except that one larger resource occupies its
own declared streaming pack. A pack publishes through a private partial file,
fsync, verification, and atomic rename; the manifest publishes last. Resume is
only from complete verified pack boundaries, while interrupted SQLite image
creation restarts. Physical admission is limited to the exact declared schema/
application range; incompatible images refuse and use semantic archive-v2
rather than silently migrating. Restore verifies the full capability before
writes, makes an emergency snapshot, installs through durable staging,
preserves database identity, rotates replica identity, clears reviewed local
state, and rebuilds invalid derived state. Secret keys never enter the image.

## Full-corpus evidence

The equivalent Joplin and Obsidian recipe views each contained 382,206
documents and 451,964,499 visible body bytes. Their canonical semantic
title/body/deletion multisets matched after the documented source projection;
their raw databases and source-specific metadata were deliberately not called
identical.

Both selected candidates recovered the exact canonical aggregate and passed
integrity, corruption refusal, time, memory, and bounded-shape gates:

| Complete boundary | Packed semantic archive-v2 | SQLite image + packed assets |
|---|---:|---:|
| Local create through verified restore | 4,384.0 s | 1,884.2 s |
| Including distinct Google Drive copy/reread | 4,457.6 s | 2,245.1 s |
| Sealed bytes | 1.309 GB | 6.040 GB |
| Restore writes | 28.4 GB | 6.04 GB |

The physical image was 2.33x faster locally and 1.99x faster with provider
evidence, but the semantic artifact was 78.3% smaller. That tradeoff is why the
formats remain complementary.

The attachment workload contained 103,349 documents, 758 resource records,
731 deduplicated blobs, and 111,330 preserved source-bundle items. The image
bundle restored the exact aggregate from one packed payload. Loose recipe and
loose-asset layouts failed the non-object-per-file gate.

Restic and Borg data checks, unchanged snapshots, and exact restores completed
for canonical state and the raw 1,237,553-file source tree. Their repositories
bounded stored entries, but raw create/traversal/restore repeatedly exceeded
the 512 MiB desktop gate; raw restore took 4.13x and 5.39x the image restore.
They remain useful external references, not substitutes for application-level
compatibility, asset binding, identity rotation, or semantic fallback.

The imports exposed separate debt: recipe Joplin and Obsidian exceeded the
two-hour gate and all three full imports exceeded 512 MiB RSS. Post-snapshot
incremental replay converged but reached 3.22 GiB RSS. These findings do not
change the physical-format selection; importer work needs a separately planned
slice and G14d owns bounded replay.

## Validation

- 57 external aggregate phase rows passed the harness result validator.
- `python3 performance/v0.7-g14b/validate_evidence.py`
- `python3 -m unittest performance/v0.7-g14b/test_harness.py`
- full repository validation is recorded in `agent/ATTEMPT_LOG.jsonl`

**Outcome (2026-08-22).** G14b is complete. Option B and the exact G14c
capability/default, pack, resume, compatibility, migration, local-state, and
fallback decisions are durable in `PLAN.md` and
`performance/v0.7-g14b/SQLITE_IMAGE_CAPABILITY.md`. Fifty-seven sanitized phase
rows and the reviewed findings are committed; all private evidence remains
external. G14c is next and approval-gated.
