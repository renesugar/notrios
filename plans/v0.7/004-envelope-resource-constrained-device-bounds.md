# v0.7 G2 archive — envelope, resource, and constrained-device bounds

Status: completed 2026-08-11

Model: GPT-5 (exact variant not exposed to the agent)

## Goal and boundaries

Determine deterministic operation encoding/compression, envelope, pending,
resource chunk, range, and many-small-object pack bounds before G8/G9 make them
an immutable protocol contract.

G2 is investigation code only. It did not add a production sync codec, schema,
cryptography, transport, project dependency, private content, or mobile
implementation. The harness remains under `performance/v0.7-g2/`.

## Completed scope

- Reused aggregate-only G1 recipe/Joplin body and resource distributions and
  archive-v2 P3b/P4 loose/pack measurements. The committed input names no
  private path, filename, content, source hash, database, or resource.
- Implemented canonical JSONL and compact NCB1 comparison codecs in pure Go,
  including count/byte/dependency/payload checks, minimal varints, exact
  round-trip verification, and repeat-byte determinism.
- Specified NCB1 completely for the investigation: fixed raw identifiers,
  shortest unsigned varints, ordered fields, raw SHA-256 dependencies, and one
  bounded per-kind canonical-JSON payload.
- Pinned an evidence gzip/DEFLATE level-6 profile with zero time, OS 255, and no
  optional header fields; documented why G9 still needs cross-toolchain goldens.
- Measured both formats at 100, 10,000, and 100,000 operations with wall/CPU,
  allocation, process peak-RSS, raw/compressed size, exactness, determinism, and
  proposed-envelope splitting.
- Compared nine whole-threshold/fixed-chunk combinations across zero, 64 KiB,
  32 MiB, 107,951,795-byte attachment-heavy, and aggregate corpus proxy cases.
- Reused two real archive-v2 pack A/B runs and separated file-count, apparent
  bytes/disk-block effects, and throughput rather than reducing them to one
  misleading size number.
- Rejected operation-count overflow, oversized payload, compression-ratio
  expansion, multiple gzip members, non-minimal compact varints, and unknown
  JSON fields.
- Added separate v0.8 Android-emulator and post-1.0 physical-device checklists
  with explicit performance, memory, hostile-input, pending, pack, range,
  suspension, and recovery gates.

## Evidence and decisions

At the 10,000-operation tier, canonical JSONL encoded to 3,090,440 bytes and
424,041 gzip bytes. NCB1 encoded to 1,485,497 bytes and 355,032 gzip bytes:
51.9% smaller raw and 16.3% smaller compressed. The recorded desktop run decoded
NCB1 about 6.1 times faster and allocated about one-fifth as much. This is a
material benefit and NCB1 has a canonicalization specification, so G2 invokes
the plan's recorded exception to the JSONL default. The outer envelope manifest
remains a readable canonical-JSON candidate. G9 owns deliberate promotion or
replacement, operation-kind schemas, capabilities, goldens, fuzzing,
signature/AEAD framing, and production admission.

The 100,000-operation workload split into ten candidate envelopes. G9 receives
simultaneous ceilings of 10,000 operations, 16 MiB canonical record bytes, and
4 MiB compressed bytes, plus 1 MiB per record, 512 KiB per payload, 64
dependencies per operation, and 64:1 decompression expansion. Pending data per
peer is bounded at 10,000 operations/64 MiB of disk-backed encoded bytes;
overflow backpressures and requests repair/snapshot catch-up rather than
evicting arbitrary prerequisites.

Resources below 1 MiB stay whole and larger resources use 1 MiB fixed chunks.
That leaves 97.7% of the aggregate recipe resources and 83.8% of the Joplin
resources whole while bounding retry work for the large tail. The inherited 16
GiB resource ceiling permits at most 16,384 chunks. A 1 MiB chunk is the middle
point between the 256 KiB candidate's fourfold object/manifest cost and the 4
MiB candidate's fourfold range/retry cost. FastCDC remains deferred because
static corpora contain no repeated binary-resource edit trace.

Existing packs reduced file count 8,314× and 7,183× in the two real corpora.
Sync carriers therefore reuse the pack capability with provisional smaller 64
MiB/4,096-object targets and a 4 MiB trailer ceiling, while desktop snapshot
archives retain their existing targets. These and every envelope number remain
desktop proxies until the emulator and physical-device gates retain or lower
them.

## Validation

Passed on 2026-08-11:

- `go test ./performance/v0.7-g2/...`;
- `go test -race ./performance/v0.7-g2/...`;
- `go run ./performance/v0.7-g2/cmd/evidence` — six operation tier rows, nine
  resource policies, and six hostile cases;
- `python3 performance/v0.7-g2/validate_evidence.py` — compact NCB1 selected;
- `go vet ./...`;
- `go test ./...`;
- `python3 scripts/check_required_files.py`;
- `python3 scripts/check_plan_loops.py`;
- `bash scripts/validate-scaffold.sh`;
- frontend `npm ci`, `npm run typecheck`, `npm test -- --run`, and
  `npm run build`;
- `bash scripts/build_docs_site.sh`;
- `git diff --check`.

`npm audit` continues to report the pre-existing one moderate and two high
development-tree advisories. G2 changed no npm or Go dependency.

Release packaging and copied-ZIP readback are post-commit handoff steps and are
reported with the completed artifact.

## Working state and next item

Product version remains 0.6.0 and canonical schema remains v18. G3 is next and
is not approved. It owns named runtime profiles, copied-database/replica
identity handling, and multi-instance path/port/process isolation. G2 does not
authorize it.
