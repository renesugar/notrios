# v0.7 G1 archive — representative divergence and revision-delta workload

Status: completed 2026-08-11

Model: GPT-5 (exact variant not exposed to the agent)

## Goal and boundaries

Choose G7 body delta and three-way merge behavior from aggregate corpus shape
and deterministic divergent-edit evidence. G1 did not implement a production
merge engine, synchronization path, schema migration, protocol encoding, or
dependency. Static imported corpora were not treated as independent-device
history and no human conflict frequency was inferred.

## Completed scope

- Profiled body and non-body file-size distributions across three complete
  available corpora using an aggregate-only output schema.
- Generated separate one-hour, one-day, one-week, and thirty-day edit chains.
- Compared complete bodies, line edit scripts, Unicode-aware word edit scripts,
  and a single byte-span delta over ordinary, Unicode, Markdown, valid
  control-bearing, long-line, and 1 MiB generated cases.
- Classified line/word/byte three-way merges for independent paragraphs,
  disjoint and overlapping same-line edits, Unicode neighbors, Markdown
  overlap, and a repeated-token long line.
- Covered title/body, delete/edit, move/rename, tag add/remove, and notebook
  cycle semantics in the frozen scenario matrix.
- Rejected missing/wrong bases, wrong result hash, cursor overrun, operation and
  inserted-byte limits, and invalid UTF-8 output.
- Probed a permissive maintained Go merge candidate at an exact upstream commit
  without adding it to the project module.

## Evidence and result

Evidence is under `performance/v0.7-g1/`. The structural validator reports
2,063,061 aggregate-only bodies, 112 delta records, eighteen merge records, and
seven rejected malformed/untrusted patches. The private sources themselves,
paths, filenames, titles, bodies, source-content hashes, databases, and
resources are absent.

G7's selected model is:

1. Every immutable revision binds a complete UTF-8 result object/hash and its
   parent IDs.
2. A named-parent line-token delta is optional and retained only when bounded
   generation succeeds, exact base/result verification succeeds, and the
   encoded delta is materially smaller than the complete object.
3. Three-way merge is bounded line-first. A line conflict may receive
   Unicode-aware word/punctuation/whitespace refinement only within a bounded
   conflict region.
4. Same-token, delete/edit, malformed, or over-limit overlap becomes a durable
   typed conflict holding base/local/remote revision references and content.
5. Complete-object fetch is the fallback for every missing/refused delta. A
   best-effort or partially verified patch never reaches canonical state.

`github.com/epiclabs-io/diff3` at commit
`3b1669897fb1aa7c1fb2699a3c6a45bbb46e9ec1` is the preferred G7 candidate. The
MIT package has no runtime external imports, supports generic token slices and
inspectable conflicts, and passed upstream plus G1 Unicode/conflict probes. It
has no observed tag/release, so G7 must recheck maintenance, pin an exact
pseudo-version, and keep Notrios-owned conformance/property tests before
adoption. G1 added no dependency.

Concurrent add/remove of the same tag remains the material G6 LWW-per-element
case: protocol order chooses one result, while both operations remain in the
journal/audit trail. The static corpus cannot establish how often it occurs and
did not justify an unbounded observed-remove dot set.

## Validation

Passed on 2026-08-11:

- `python3 performance/v0.7-g1/validate_evidence.py` — 2,063,061 bodies, 112
  delta records, eighteen merge records, seven rejected patches;
- `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s
  performance/v0.7-g1 -p 'test_*.py' -v` — four focused tests;
- upstream candidate `go test ./...` at the recorded commit, including the
  temporary G1 UTF-8/conflict probe;
- `go vet ./...`;
- `go test ./...`;
- `python3 scripts/check_required_files.py` — 65 required files;
- `python3 scripts/check_plan_loops.py`;
- `bash scripts/validate-scaffold.sh`;
- frontend `npm run typecheck`;
- frontend `npm test -- --run` — 15 files, 155 tests;
- frontend `npm run build` — passed with the existing large-chunk warning;
- `bash scripts/build_docs_site.sh` — 15 pages;
- `git diff --check`.

Release packaging and copied-ZIP readback are post-commit handoff steps and are
reported with the completed artifact.

## Working state and next item

Product version remains 0.6.0 and canonical schema remains v18. G2 is next and
is not approved. It may refine numeric object/envelope limits but must preserve
the complete-result recovery invariant and may not make a transfer delta
canonical state.
