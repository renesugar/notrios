# v0.7 G1a — pure-Go xdelta/VCDIFF feasibility

G1a is an investigation-only prototype for deterministic transfer deltas over
arbitrary bytes. It is not production synchronization code and it adds no
runtime dependency, cgo code, schema, or wire-format commitment.

The pure-Go matcher uses non-overlapping 64-byte source blocks, a rolling
pseudo-Adler checksum, byte confirmation, and forward/backward match extension
in the style documented by Apache Subversion. The same `ADD`/source-`COPY`/`RUN`
operation stream is serialized two ways:

- a constrained RFC 3284 VCDIFF default-code-table profile; and
- `NXD1`, a minimal private comparison container that exists only to measure
  what portability costs.

The VCDIFF emitter produces one bounded window and only opcode 0 (`RUN`), 1
(`ADD`), and 19 (SELF-mode `COPY`). The decoder additionally accepts the
single-operation fixed-size ADD and SELF COPY opcodes and implements overlapping
target-copy semantics. It deliberately rejects custom code tables, secondary
compression, `VCD_TARGET` windows, non-SELF modes, compound opcodes, excess
sections/windows/operations, overlong varints, excessive expansion, and deep
delta chains. A future production implementation would still verify the named
base and exact reconstructed-result hashes in the outer Notrios object
contract.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g1a-gocache \
  go test ./performance/v0.7-g1a/...

env GOCACHE=/tmp/notrios-g1a-gocache \
  go test ./performance/v0.7-g1a/xdelta -run '^$' -parallel=1 \
  -fuzz '^FuzzDecodeVCDIFFNeverPanics$' -fuzztime=10s

env GOCACHE=/tmp/notrios-g1a-gocache \
  go test ./performance/v0.7-g1a/xdelta -run '^$' -parallel=1 \
  -fuzz '^FuzzDecodePrivateNeverPanics$' -fuzztime=10s

env GOCACHE=/tmp/notrios-g1a-gocache \
  go run ./performance/v0.7-g1a/cmd/evidence \
  -out /tmp/g1a-benchmark-results.json

PYTHONDONTWRITEBYTECODE=1 \
  python3 performance/v0.7-g1a/validate_evidence.py
```

For the interoperability run, build the pinned Apache-2.0 test oracles listed
in `PROVENANCE.md`, pass their executable paths to `cmd/interop`, and point
`-fixtures` at the optional oracle directory produced by `cmd/evidence`.
Neither executable is a Notrios build or runtime dependency.

## Evidence

- `benchmark-results.json` — 14 G1-compatible text and seven generated binary
  fixtures, seven-run CPU/wall/allocation measurements, peak RSS, sizes,
  operation counts, hashes, and determinism;
- `interoperability.json` — bidirectional compatibility probes against pinned
  xdelta3 and open-vcdiff builds on three representative fixtures;
- `vectors.json` — golden vectors, hostile cases, declared bounds, and the
  recorded single-worker fuzz runs;
- `FINDINGS.md` — interpretation and recommendation;
- `PROVENANCE.md` — source/specification roles, exact commits, and licenses;
- `xdelta/` — bounded investigation implementation and tests;
- `cmd/evidence` and `cmd/interop` — reproducible evidence generators;
- `validate_evidence.py` — completeness, privacy, and invariant validator.

All fixtures are deterministic synthetic bytes. No private corpus content,
paths, or content hashes are committed. These desktop measurements do not
establish mobile-safe numeric limits; G2 owns those bounds.
