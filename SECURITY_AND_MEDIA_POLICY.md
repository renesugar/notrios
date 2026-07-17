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
action, and the importers' `--localize-media` flag. Perceptual-hash hooks
are task H5.

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
- future perceptual-hash hooks for moderated content and near-duplicate review.

Perceptual hashes must not be used to silently deduplicate bytes; exact hashes deduplicate blobs, perceptual hashes raise similarity/moderation signals.
