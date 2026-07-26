# Security and Media Policy

Remote-media localization must not be a blind downloader.

Implementation status (v0.3): the policy configuration (H1), the static
scan (H2), the quarantine fetch pipeline (H3, `internal/media.Fetcher`),
and localization (H4, `internal/localize`) are implemented — URL
validation, scheme/domain checks re-applied per redirect hop, connect-time
private-address blocking (post-DNS, so rebinding is caught), streaming
size caps, MIME sniffing that a lying header cannot override, exact
SHA-256, exact-hash rule checks (`media_hash_rules`) before admission to
the content-addressed store, Markdown rewriting to `resource://` in a new
revision with a base-revision precondition, and per-attempt records in
`media_policy_decisions` (quarantined → admitted/refused). One engine
serves REST, `notriosctl localize`, the MCP editor tool, the GUI inspector
action, and the importers' `--localize-media` flag. H5 adds exact-reference
reports and wires a perceptual-hash hook that is inert by default.

## Required pipeline

```text
remote URL in Markdown
    ↓
normalize and validate URL
    ↓
check scheme/domain stop list
    ↓
fetch into quarantine only
    ↓
apply size, timeout, redirect, MIME, and private-network rules
    ↓
compute exact hash
    ↓
compute perceptual hash where supported
    ↓
check local policy databases
    ↓
if allowed, admit to content-addressed store
    ↓
rewrite note and record provenance
```

## Policy requirements

- Block `file:`, `data:`, and other unsafe schemes by default.
- Block private network and link-local addresses by default.
- Apply domain policy to every redirect hop.
- Keep exact-hash and perceptual-hash policy hooks separate.
- Use exact hashes for deduplication.
- Use perceptual hashes for moderation signals and near-duplicate suggestions; do not silently collapse perceptual matches.
- Store provenance: original URL, final URL, retrieved timestamp, content type, hashes, and policy decision.

## Preview rule

Preview components may detect remote images and offer actions, but the server performs localization. Browser image loading must not be treated as a safe content source.

## Stop lists and hash checks

Remote-media localization must support:

- configured blocked/allowed/review domain patterns;
- redirect-chain policy checks;
- blocked schemes and private-network protections;
- quarantine fetches before admission to the resource store;
- exact content hashes for deduplication and blocking;
- optional perceptual-hash hooks for moderated content and near-duplicate review.

Perceptual hashes must not be used to silently deduplicate bytes; exact hashes deduplicate blobs, perceptual hashes raise similarity/moderation signals.

## Perceptual-hash hook contract

Notrios defines `store.PerceptualHashHook` but ships no pHash, dHash,
blockhash, or other implementation. An embedding application may install one
on `SQLiteStore` before admitting resources:

- `Algorithm` returns a stable lowercase identifier distinct from `sha256`.
- `SupportsMIME` explicitly selects content the hook can parse.
- `Compute` receives a read-only blob stream and returns one canonical
  lowercase hash. Hook errors fail that resource admission rather than silently
  claiming the blob was checked.
- `SuggestNearDuplicates` receives stored hashes for that algorithm and returns
  candidate blob pairs plus an algorithm-specific non-negative distance.

The store computes at most once per exact blob/algorithm, persists the value in
`resource_hashes`, and reuses it for logical resources sharing those exact
bytes. It checks `media_hash_rules` during admission, but perceptual rules may
only yield `review`; `AddMediaHashRule` rejects perceptual `block` rules.
Returned candidate pairs are validated against the supplied blob set,
canonicalized, and deduplicated before appearing in the resource report.

The hook has no store mutation capability and no authority to admit, reject,
merge, rewrite, hide, delete, or garbage-collect data. Exact SHA-256 remains
the only deduplication identity. With no installed hook, admission behavior is
unchanged and the perceptual report is empty with `hook_enabled: false`.

Reports are available from
`GET /api/v1/resources/reports/reference` and
`notriosctl resources report`. The report is advisory and read-only; H6 owns
retention and deletion policy.
