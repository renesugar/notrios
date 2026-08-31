# v0.7 G19 — archive-v2 compatibility bridge

Date: 2026-08-30
Models: GPT-5 (exact serving variant unavailable); Luna workers for bounded schema and fixture materialization, with parent architectural design/review and replacement of incomplete artifacts
Working state: complete

## Goal and boundaries

Publish the portable archive-v2 consumer contract after the sync-era container
freeze. This slice does not change the archive format, canonical schema, sync
wire, physical snapshot format, REST/MCP surface, or MoveNotes repositories. It
does not claim an external consumer that does not exist and does not publish a
SQLite image, private export, source corpus, or secret.

## Compatibility preflight

`notriosctl compatibility archive-v2` performs bounded declaration-only
admission against `current-v2` or the frozen `previous-loose-v2` capability
profile. It strictly decodes a regular, non-symlink manifest under archive
limits, checks archive version/schema bounds, closed required capabilities, and
bounded optional capabilities, and emits structured JSON. Accept exits 0 and
explicitly requires full verification; capability/format refusal exits 1;
usage/profile errors exit 2.

The historical loose profile is pinned to commit
`5ae93df8e14880a6f83cf20014bddcb4e9079f1a` but is a capability profile, not a
claim that an old binary ran. It accepts the base five capabilities and refuses
`objects.pack.v1`. Unknown required capabilities refuse; an unknown bounded
optional capability is ignored. Invalid limits, schema/reader bounds, duplicate
or unsorted capability declarations, and manifest symlinks refuse.

The classifier recognizes `notrios-sqlite-image` v1 only under its exact
application, same-schema, capability, and semantic-fallback declaration. It
always refuses that physical format without opening `notes.sqlite`. An invalid
or forged fallback is not echoed as trusted guidance.

## Published contract and fixtures

`contracts/archive-v2/` contains a machine-readable capability/version/limit
registry and five strict Draft 2020-12 schemas for semantic manifests, index
entries, all twelve record payloads, pack trailers, and production-shaped
physical-manifest identification. JSON Schema is explicitly structural;
duplicate-key safety, ordering, capability semantics, hashes, paths, totals,
references, and filesystem completeness stay with the verifier.

Three complete, sanitized, deterministic goldens use one invented corpus with
all twelve records and three blobs:

- loose schema 12;
- packed schema 12;
- packed schema 27 from the sync era.

Tests construct all three independently of `Export`, compare every committed
byte, and pass each through `VerifyDirectory`. Narrow probes cover the previous
explicit-zero `record_counts` spelling, unknown required/optional capabilities,
and the physical refusal. The physical fixture has a valid production manifest
and commit over fake descriptors but no database or pack payload.

G9's invented NCB1/NEV1 vectors and inputs are copied byte-for-byte under
`contracts/archive-v2/sync-wire/` while remaining explicitly separate protocol
1.0 evidence. NAR1 is not called deterministic because production sealing uses
fresh entropy. The schema-27 archive proves synchronization did not introduce
archive-v3 or sync-wire record types into archive-v2.

The Hugo/Ledger builder publishes all 27 contract files byte-for-byte under
`/notrios/contracts/archive-v2/`. Existing rendered-route, internal-link,
Pagefind-scope, CSP, and offline gates continue to pass.

## External consumer audit

Read-only source, branch, and local-history inspection found no
`notrios2sql.py`, Notrios archive reference, or archive-v2 consumer in
`/home/renes/projects/movenotes-v3` or its predecessor. The checked
`movenotes-v3` head was
`659aa41594b00e1c9d1152b34b227b00cdc28b34`. No external repository was edited,
and no cross-repository test is claimed. The exact audit is
`performance/v0.7-g19/external-consumer-audit.json`.

## Validation

The durable `make g19-validate` gate passes five schema compilations,
structural samples, deterministic fixture reproduction, three complete archive
verifications, registry/production-constant parity, 12 current/previous matrix
cases, CLI JSON/exit behavior, physical refusal, and G9 byte equality. The G19
evidence validator passes.

The regular completion pass also passed `go vet ./...`, unrestricted `go test
./...` (loopback tests require the approved host capability), `make
g18g-validate`, documentation generation/audit, web typecheck, 172 frontend
tests, production web build, scaffold checks, and zero-vulnerability web and
docs dependency audits. A deliberately parallel frontend run initially timed
out two 5-second tests under contention; the isolated canonical run passed all
172 without a code change.

No GitHub push, evidence-reserve write, ISO, physical burn, private data,
external repository mutation, or format/schema migration occurred.
