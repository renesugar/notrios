# v0.7 planning amendment — pure-Go xdelta/VCDIFF investigation

Date: 2026-08-11  
Model: GPT-5 (exact variant not exposed)  
Working state: planning/documentation amendment complete; G1a remains
unapproved and unimplemented

## Request and outcome

The user asked that the synchronization plan investigate a pure-Go port of the
xdelta algorithm used by Subversion instead of taking an external dependency,
with binary as well as text input and RFC 3284/VCDIFF considered.

The active plan now contains a new independently approvable **G1a** before G2.
It is an evidence spike, not authorization to implement a production codec. It
must compare Subversion's xdelta matching algorithm, Subversion's separate
svndiff serialization, and RFC 3284 VCDIFF; prototype a deterministic bounded
pure-Go encoder/decoder; validate arbitrary bytes and hostile inputs; and
recommend a format and provenance path. External implementations may be test
oracles but not runtime dependencies.

G1's measurements and selected conflict behavior remain valid. Its tentative
`github.com/epiclabs-io/diff3` implementation recommendation is superseded.
G7 will own a bounded pure-Go line/word three-way merge because binary delta
encoding and conflict-aware merge solve different problems.

## Primary-source review

The review recorded these distinctions:

- RFC 3284 specifies the VCDIFF portable delta-data format and default code
  table, not one mandatory match-finding algorithm.
- Apache Subversion `libsvn_delta/xdelta.c` computes binary-safe txdelta windows
  using 64-byte blocks, a rolling pseudo-Adler checksum, hash lookup, forward
  and backward match extension, source-copy operations, and new target bytes.
- Subversion `libsvn_delta/svndiff.c` separately serializes txdelta windows in
  the SVN svndiff format. Svndiff bytes are not RFC 3284 VCDIFF bytes.
- `jmacd/xdelta` is an Apache-2.0 C implementation/tool for VCDIFF; it is not
  pure Go. Its older GPL lineage requires care, so G1a forbids deriving code
  from GPL sources.
- `antmicro/go-xdelta` is an Apache-2.0 C fork with a cgo Go interface and
  native build dependencies, not a pure-Go implementation.
- `google/open-vcdiff` is an Apache-2.0 C++ VCDIFF implementation and was
  archived in 2026; it is useful only as an interoperability oracle.

Reviewed source commits:

| Source | Commit |
|---|---|
| Apache Subversion | `3a113b9b5e8050df2dbb32283b21c00934976711` |
| jmacd/xdelta | `9822b17313263d458b80511b08124971fc0e04fa` |
| antmicro/go-xdelta | `9180b718329b756354916b18c6ca834bbc21722c` |
| google/open-vcdiff | `868f459a8d815125c2457f8c74b12493853100f9` |

## Decisions recorded in G1a

- Pure Go and Notrios-owned production implementation; no cgo, vendored C/C++,
  GPL-derived code, or external merge/delta runtime package.
- Binary delta remains a named-base transfer optimization. Complete verified
  objects remain recoverable, and unrelated content-addressed resources do not
  acquire an invented parent relationship.
- Three-way merge remains a separate bounded text operation.
- Non-blocking default: compare a constrained RFC 3284 default-code-table
  profile with a minimal deterministic private operation container.
- Non-blocking default: implement spec-first and use Apache Subversion for
  behavioral comparison/attribution; any direct translation needs an explicit
  Apache license/NOTICE review.
- Non-blocking default: the G1a prototype stays under `performance/` unless a
  later approved G7/G8 slice promotes or rewrites it.

## Documents reconciled

`PLAN.md`, `ROADMAP.md`, `SYNCHRONIZATION.md`,
`VERSIONING_AND_SYNC_POLICY.md`, `FEATURE_MATRIX.md`, `SECURITY_REVIEW.md`,
`TESTING_POLICY.md`, `README.md`, `CONTEXT_MAP.md`,
`CODING_CLIENT_HANDOFF.md`, `agent/PLAN_STATUS.md`, and the living G1 evidence
notes now agree that G1a is next and unapproved. Historical G0/G1 archives were
not rewritten.

## Validation

This slice changes plans and documentation only. Its completion record in
`agent/ATTEMPT_LOG.jsonl` lists the repository checks; the verified release ZIP
is created from the resulting commit and reported in the handoff.
