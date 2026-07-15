---
name: remote-media-policy
description: Implement policy-controlled remote media localization with stop lists, quarantine, hashes, and deduplication.
---

# Remote Media Policy Skill

Use this skill when the active task matches the description.

## Steps

1. Normalize URL and apply scheme/domain/private-network policy before fetch.
2. Fetch only into quarantine.
3. Enforce size, timeout, redirect, MIME, and sniffing rules.
4. Compute exact hashes for deduplication and exact blocklists.
5. Add perceptual-hash hooks for moderation signals.
6. Admit to resource store only after policy allows it.
7. Record provenance and policy decision.

## Working-state checks

- SSRF tests.
- Blocked-domain tests.
- Oversized file tests.
- Deduplication tests.
