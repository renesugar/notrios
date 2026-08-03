# Selection and privacy planner

The v0.4 P1 planner is the shared, read-only decision layer for future native
archive v2, subset transfer, and publication handoff. It selects canonical
SQLite records and reports what a later writer may emit; it never creates an
archive, invokes a publisher, reads resource bytes, or mutates notes.

## Typed request

Every plan requires one target:

- `full_archive` — all current and trashed notes by default, preserving
  provenance, private metadata, exact source bundles, and link syntax;
- `subset_transfer` — an explicitly selected current-note subset, preserving
  provenance/source bundles but reporting cross-boundary links and stripping
  private source metadata by default;
- `publication_handoff` — an explicitly selected current-note subset with
  secure defaults: exclude `private`, `draft`, and `confidential`, omit
  provenance/source bundles/private metadata, and convert private/broken links
  to plain text in the later writer.

Selectors are typed: recursive notebook IDs, tag names, one bounded canonical
query, and up to 1,000 explicit document IDs. Values inside a selector type are
ORed; populated selector types combine with `match: any` or `match: all`.
Notebook recursion defaults on. Subset/publication requests with no selector
are rejected so an omitted field cannot accidentally expose the whole library.

Policy overrides are explicit booleans and enums, not generic maps. They cover
excluded/private tags, link action (`retain`, `report`, `plain_text`, or
`redact`), provenance, exact source bundles, private metadata, Trash, and an
oversized-resource threshold.

## Deterministic report

The response contains:

- selected document IDs/URIs/current revision IDs and inclusion reasons;
- exact reachable resource IDs, SHA-256 values, MIME types, and sizes;
- internal, private/cross-boundary, broken, and external link counts plus
  bounded policy decisions;
- associated exact source-bundle content hashes plus SHA-256 fingerprints of
  source/item keys when policy permits (raw keys may be paths and never cross
  planner APIs);
- document exclusions with stable reason codes;
- metadata preserve/strip/include/exclude decisions and affected counts;
- warnings and a SHA-256 digest over the complete normalized manifest.

Detail arrays are independently capped. `truncated: true` means the visible
examples were capped; complete counts and `manifest_sha256` still cover the
bounded selection. The plan has no generation timestamp, so identical
canonical state and normalized inputs produce identical output.

## Privacy boundary

Planner APIs never return note titles/bodies, revision bodies, resource bytes,
source `metadata_json`, source URLs, raw broken-link targets/context, raw
source-bundle keys, asset storage paths, or arbitrary filesystem paths. Source
and item keys are represented only by SHA-256 fingerprints. Imported content is therefore
not placed into MCP context merely by planning a handoff.

Reachability starts only from selected documents. A resource referenced solely
by an excluded note is not included. A link from a selected note to an
excluded/unselected note is classified `private`; unresolved, ambiguous,
invalid, and deleted targets are `broken`. P1 reports the later writer action
but does not rewrite content.

## Limits and scalability

- 100 notebook selectors and 100 tag/policy selectors;
- 1,000 explicit document IDs;
- 512 UTF-8 bytes per selector value;
- Q1 query limits: 4,096 bytes, 256 tokens, 16 grouping levels;
- direct Store planner: at most 1,000,000 selected documents;
- REST/MCP planner: at most 100,000 selected documents;
- REST detail cap: 1,000 items per detail array; MCP uses its configured result
  cap (default 10, maximum 50).

The Store mutex supplies a consistent in-process read snapshot. The planner
loads only document identity/revision state and processes resources, links,
provenance, and source-bundle metadata in 400-ID batches. It never selects a
note body or resource/source-bundle byte stream. P2 will stream the same
content-free manifest contract into archive objects.

## Interfaces

- Store: `PlanSelection(context.Context, SelectionPlanRequest)`
- REST: `POST /api/v1/selection/plan`
- MCP read-only tool: `plan_selection`

REST and MCP call the same Store method. Neither accepts SQL, output paths, nor
archive/publisher commands.
