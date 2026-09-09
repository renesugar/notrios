# v0.7 G1a archive — pure-Go xdelta/VCDIFF feasibility and dependency removal

Status: completed 2026-08-11

Model: GPT-5 (exact variant not exposed to the agent)

## Goal and boundaries

Determine whether a Notrios-owned pure-Go implementation can produce and apply
deterministic bounded transfer deltas over arbitrary text and binary bytes,
without adding an external runtime dependency or confusing binary delta
compression with conflict-aware three-way merge.

G1a is investigation code only. It did not add a production sync path, schema,
wire-format commitment, cgo/C/C++ code, project module dependency, private
content, or mobile-safety claim. The prototype remains under
`performance/v0.7-g1a/`.

## Completed scope

- Implemented an independently written Subversion-style matcher using
  non-overlapping 64-byte source blocks, rolling pseudo-Adler checksums, byte
  confirmation, and forward/backward match extension.
- Serialized the resulting `ADD`/source-`COPY`/`RUN` operations as both a
  constrained RFC 3284 default-code-table VCDIFF stream and the minimal `NXD1`
  private comparison container.
- Added bounded byte-slice and `io.Reader`/`io.Writer` encode/decode APIs.
- Bounded source, target, delta, window, section, operation, window,
  checksum-candidate, varint, expansion-ratio, and chain-depth dimensions and
  checked context cancellation.
- Tested exact output, target-copy overlap, every truncated prefix, invalid
  ranges/opcodes/windows/compression, unused/trailing bytes, every declared
  limit, short writers, 100 randomized seeds, and two fuzz targets.
- Measured fourteen G1-compatible text cases and seven deterministic generated
  binary cases with seven-run CPU/wall/allocation samples and peak RSS.
- Compared all text cases with G1 line-JSON sizes and all cases with complete
  bytes and the private container.
- Built pinned Apache-2.0 xdelta3 and open-vcdiff executables in `/tmp` as
  external oracles and recorded bidirectional compatibility on three fixtures.
- Recorded exact source revisions, licenses, and the specification/reference/
  oracle role of each upstream project without vendoring or linking it.

## Evidence and decisions

Evidence under `performance/v0.7-g1a/` contains 21 exact deterministic round
trips. VCDIFF was smaller than complete bytes in 19 cases and smaller than G1's
line JSON in all fourteen text comparisons. The two non-beneficial cases were
meaningful: empty output costs a five-byte header, and unrelated 256 KiB random
bytes cost 22 bytes more than the complete target. Complete-object fallback and
a material-size gate therefore remain mandatory.

The private container was normally ten bytes smaller and at most thirteen bytes
smaller on this matrix. That saving does not justify a Notrios-only format.
Both xdelta3 and open-vcdiff decoded all three representative Go-emitted
fixtures exactly, so G1a recommends the constrained RFC 3284 default-table
profile. This is not Subversion svndiff byte compatibility. It is also not a
claim of a general VCDIFF decoder: the strict Go decoder accepted two of six
external encodes and rejected valid compound opcodes outside the selected
profile.

Implementation provenance remains independent and spec-first. Apache
Subversion at `3a113b9b5e8050df2dbb32283b21c00934976711` is an attributed
behavioral reference, not a claimed line translation. No GPL source was used.
Pinned open-vcdiff `868f459a8d815125c2457f8c74b12493853100f9`
and xdelta3 `9822b17313263d458b80511b08124971fc0e04fa` builds were
external test oracles only.

Production promotion is explicitly refused in G1a. G2 must establish numeric
bounds; later G7/G8 must separately approve a rewrite or deliberate promotion,
require a real immutable named parent and exact outer base/result hashes, and
retain complete-object fallback. G7's line-first/word-region three-way merge
remains a separate Notrios-owned implementation.

## Validation

Passed on 2026-08-11:

- `go test ./performance/v0.7-g1a/...`;
- `go test -race ./performance/v0.7-g1a/...`;
- `go test ... -fuzz FuzzDecodeVCDIFFNeverPanics -fuzztime=10s
  -parallel=1` — 59,709 executions;
- `go test ... -fuzz FuzzDecodePrivateNeverPanics -fuzztime=10s
  -parallel=1` — 59,593 executions;
- `python3 performance/v0.7-g1a/validate_evidence.py` — 21 fixtures, 19
  beneficial VCDIFFs, six exact external decodes, two of six constrained
  inbound accepts;
- `go vet ./...`;
- `go test ./...` — passed with normal loopback-listener permission. The first
  restricted-network run failed only because four existing `httptest` suites
  could not bind `[::1]:0`;
- `python3 scripts/check_required_files.py` — 65 required files;
- `python3 scripts/check_plan_loops.py`;
- `bash scripts/validate-scaffold.sh`;
- frontend `npm run typecheck`;
- frontend `npm test -- --run` — 15 files, 155 tests;
- frontend `npm run build` — passed with the existing large-chunk warning;
- `bash scripts/build_docs_site.sh` — 15 pages;
- `git diff --check`.

`npm audit` continues to report the pre-existing one moderate and two high
development-tree advisories in PostCSS, nanoid, and undici. G1a changed no npm
or Go dependency.

Release packaging and copied-ZIP readback are post-commit handoff steps and are
reported with the completed artifact.

## Working state and next item

Product version remains 0.6.0 and canonical schema remains v18. G2 is next and
is not approved. Its first narrow slice is to regenerate bounded envelope and
resource fixtures at 100/10k/100k-operation tiers, use G1a's decoder dimensions
as candidate inputs, and measure before fixing protocol numbers. It may not
promote the G1a codec or infer ancestry between unrelated resources.
