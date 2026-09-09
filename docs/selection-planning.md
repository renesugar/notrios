# Selection and privacy planning

Notrios can dry-run the exact note/resource boundary for a future backup,
subset transfer, or publication handoff without writing files or changing the
database.

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/selection/plan \
  -H 'Content-Type: application/json' \
  -d '{
    "target":"publication_handoff",
    "selection":{
      "notebook_ids":["nb_public"],
      "include_notebook_descendants":true,
      "tags":["publish"],
      "match":"all"
    },
    "policy":{"max_resource_bytes":25000000},
    "detail_limit":100
  }' | jq
```

The three targets have different defaults:

| Target | Default behavior |
| --- | --- |
| `full_archive` | All current and trashed notes; preserve provenance, private metadata, source bundles, and links. |
| `subset_transfer` | Requires selectors; current notes; preserve provenance/source bundles; report cross-boundary links. |
| `publication_handoff` | Requires selectors; exclude `private`, `draft`, `confidential`; strip provenance/private metadata/source bundles; plan private/broken links as plain text. |

Selectors can use recursive notebook IDs, tags, the normal [query
language](query-language.md), and up to 1,000 explicit note IDs. Values in one
selector type are ORed. Set `match` to `all` when the populated selector types
must all match.

The response contains content-free IDs, revision IDs, exact resource hashes,
link classifications, source-bundle hashes allowed by policy, exclusions,
metadata decisions, warnings, complete counts, and `manifest_sha256`. It never
contains note bodies, resource bytes, source metadata JSON, source URLs, raw
source-bundle keys, or local storage paths. Bundle source/item keys are hashed.
Detail arrays are capped; when `truncated` is true, the
counts and digest still describe the complete bounded plan.

Planning is read-only. It does not create archive v2, publish a site, rewrite
links, copy resources, or invoke `movenotes-v3`; those are later explicit
steps after reviewing this report.
