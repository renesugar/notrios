# Data safety and maintenance

This guide covers the v0.3 workflows that protect imported notes, remote
media, attachments, and the optional Recoll search sidecar. The reporting
commands are safe to run while the service is open. Before a large import or
garbage-collection apply, make a verified backup as described in the
[service guide](service.md#backup-and-restore).

## Localizing remote media

Remote images remain remote until you explicitly localize them. The note
inspector first performs a static policy scan; that scan does not download
anything or resolve DNS. For an editable note, **Localize allowed media** runs
the shared quarantine pipeline:

1. check the URL, domain, scheme, and every redirect;
2. reject private/link-local destinations unless the configuration explicitly
   allows them;
3. stream into quarantine with timeout and size limits;
4. sniff MIME type and compute the exact SHA-256;
5. apply exact-hash policy, admit the resource, and rewrite the note in a new
   revision.

The equivalent CLI workflow is:

```sh
# Report decisions only; no network request and no write
notriosctl localize --dry-run <document-id>

# Localize policy-allowed URLs
notriosctl localize <document-id>

# Also opt in URLs classified as review; blocked URLs remain blocked
notriosctl localize --allow-review <document-id>
```

Joplin RAW and Obsidian imports can run the same engine after import with
`--localize-media`. Notrios never treats a browser preview cache as an admitted
resource. If a policy or network check fails, the original URL remains in the
note and the result explains why.

## Inspecting resource references

Run the read-only reference report before removing anything:

```sh
notriosctl resources report
curl -s http://127.0.0.1:8080/api/v1/resources/reports/reference | jq
```

The report distinguishes:

- logical resources sharing one exact SHA-256 blob;
- physical blobs with no document references;
- current-note usage grouped by notebook;
- optional perceptual-hash review suggestions.

References from notes in Trash still protect a resource. Exact SHA-256 is the
only deduplication identity. No perceptual algorithm ships by default, and an
installed hook can suggest review only—it never merges or deletes resources.

## Garbage collection

Garbage collection is retention-aware and dry-run first:

```sh
notriosctl gc                 # same as --dry-run
notriosctl gc --dry-run
notriosctl gc --apply         # explicit destructive step
curl -s http://127.0.0.1:8080/api/v1/admin/gc/report | jq
```

The REST route is always read-only. The plan separates `eligible` and
`retained` resources and includes the reason and eligibility time. Apply
rechecks references inside the deletion transaction; a shared blob remains
until its final logical resource is removed. Default recovery windows are 30
days after an unattached upload/final detach and 90 days after permanent note
purge, configurable under `retention`.

For immediate administrative deletion of one unreferenced resource, REST
requires the object-specific `X-Notrios-Confirmation` header documented in the
[REST guide](api/rest.md#resources-attachments).

## Planning and resuming imports

Use a dry run first for Joplin RAW and Obsidian:

```sh
notriosctl import joplin-raw --dry-run --write-config /tmp/joplin-plan.json /path/to/raw
notriosctl import obsidian --dry-run --write-config /tmp/obsidian-plan.json /path/to/vault
```

The dry run uses the real deterministic inventory and action classifiers, but
creates no canonical notes, resources, checkpoints, or source bundles. Review
the create/update/unchanged counts and any conflict renames, then pass the
configuration to the real import:

```sh
notriosctl import joplin-raw --batch-size 100 --preserve-source \
  --import-config /tmp/joplin-plan.json /path/to/raw
notriosctl import obsidian --batch-size 100 --preserve-source \
  --import-config /tmp/obsidian-plan.json /path/to/vault
```

Both importers commit bounded batches and persist the source fingerprint,
phase, next item, and applied item fingerprints. Re-run the same command after
an interruption: an unchanged inventory resumes at the next durable batch; a
changed inventory starts a new plan and skips unchanged item fingerprints.
Keep the final JSON report and inspect every warning. `--preserve-source`
stores exact source bytes separately from canonical notes and ordinary
resource garbage collection.

## Monitoring the Recoll sidecar

SQLite/FTS5 remains the always-on search engine. When Recoll is enabled, the
desktop header shows `off`, `unavailable`, `active`, or `degraded`, plus
pending work and the last projection sync. The complete report is:

```sh
curl -s http://127.0.0.1:8080/api/v1/status | jq .search_sidecar
```

Useful fields include pending/retrying/failed jobs, last sync/index/
reconciliation timestamps, the most recent missing/stale/orphaned/repaired
counts, and a bounded last error. Failed projection jobs use durable
exponential backoff while later due jobs continue. Startup and the periodic
reconciler repair missing or stale projections and remove orphaned files.

If Recoll is absent or degraded, searches continue through FTS5. Install or
repair the external Recoll binaries, then restart the service; do not edit the
SQLite database or projection files as a recovery step. The projection and
Recoll index directories are derived and can be regenerated from canonical
storage.
