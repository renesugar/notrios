# Data safety and maintenance

This guide covers the workflows that keep a library healthy: finding what has
rotted, repairing what can be repaired mechanically, reading the shape of the
link graph, renaming a tag hierarchy, localizing remote media, auditing and
collecting attachments, planning and resuming imports, and watching the optional
Recoll sidecar.

Everything here follows the same two rules. **Reports never write**, so they are
safe to run on a library you have not backed up — which is usually exactly when
you want to know what is broken. **Anything that writes is a dry run by
default** and needs an explicit `--apply` (or `"dry_run": false`). Before a
large import or a garbage-collection apply, make a verified backup as described
in the [service guide](service.md#backup-and-restore).


## Finding what has rotted (workspace lint)

```sh
notriosctl lint                       # JSON report; exit 1 if anything was found
notriosctl lint --quiet               # exit code only, for a script or a hook
notriosctl lint --checks broken_document_link,unreferenced_resource
notriosctl lint --list-checks
```

Lint reads and never writes, so it is safe to run on a library you have not
backed up — which is usually when you want to know what is broken. Fixing is a
separate, explicitly confirmed operation.

| Check | What it means |
|---|---|
| `broken_document_link` | a link whose target note does not resolve |
| `broken_resource_link` | a `resource://` link with no such resource |
| `ambiguous_link` | a wikilink matching more than one note |
| `unresolved_block_anchor` | an anchor naming a block the target note no longer has |
| `unresolved_heading_anchor` | an anchor naming a heading the target note no longer has |
| `duplicate_source_id` | two notes claiming the same external identity |
| `missing_title` | a note with an empty title |
| `unlocalized_remote_media` | an image still fetched from the network on every preview |
| `missing_alt_text` | an embedded image with no alt text |
| `unreferenced_resource` | an attachment no note references |
| `projection_backlog` | search-projection jobs pending or retrying |

The report gives you a document ID with a line and column, a stable reason code,
and a SHA-256 fingerprint of the offending target — not the target itself. A
broken wikilink's raw text is frequently the title of a private note, so the
location is what crosses the API and the text stays in your note. Open the note
at that line to see it.

Counts always describe the whole library. `--detail-limit` caps how many
examples are listed, never what is counted, and `report_sha256` covers every
finding including the ones the cap hid — so two runs over an unchanged library
produce the same digest, and a changed digest means something really changed.

`GET /api/v1/admin/lint/report` returns the same report. Like the
garbage-collection report, there is deliberately no apply endpoint.

## Fixing what can be fixed mechanically

```sh
notriosctl fix --list-kinds       # what can be repaired, and what is opt-in
notriosctl fix                    # dry run: prints the exact edits, changes nothing
notriosctl fix --apply            # writes them, one note at a time
notriosctl fix --kinds missing_alt_text --apply
notriosctl fix --kinds unlocalized_remote_media --apply
```

Most of what lint reports cannot be repaired without deciding what you meant —
a broken link needs a target, an ambiguous wikilink needs a choice. Those stay
reported. Fix handles the small, boring subset:

| Kind | What it does | Default |
|---|---|---|
| `non_canonical_link_target` | rewrites a link that resolved by title or filename into the canonical `document://`/`resource://` URI it already points at, so a later rename cannot break it | on |
| `missing_alt_text` | fills an image's empty alt text from the resource filename | off |
| `unlocalized_remote_media` | downloads the image through the media policy and rewrites the note to a local `resource://` link | off |

Alt text is off by default because a filename is a starting point for a
description, not a description. Remote-media localization is off by default
because it reaches the network; when you ask for it, it runs the same engine as
`notriosctl localize`, with the same domain rules, private-address blocking,
size caps, MIME sniffing, and hash checks — never a plain fetch.

Three properties hold for every fix:

- **Dry run is the default.** The plan shows the exact `before` and `after` for
  each edit. Read it, then run it.
- **Each note is repaired against the revision the plan was made from.** If the
  note changed in between — you edited it, another run touched it — that note
  fails and the others still proceed. Nothing is overwritten silently.
- **Every fix is an ordinary revision.** It appears in the note's history and
  you can revert it by restoring the previous revision.

Wikilinks are left alone: rewriting `[[Kitchen]]` into a Markdown link changes
the syntax you chose rather than repairing it.

Lint reads the whole library, so it is a command you run deliberately rather
than on every save: about 3 seconds at 100,000 notes and 19 seconds at 500,000
on the reference machine. Each check's own time is in the report, so you can see
where it went on your library.

Heading anchors are checked from v0.5 E1a onward: block rows now store a heading
slug, so `#section-title` has something to compare against. On a library
upgraded from an older schema, notes nobody has edited since have no slugs yet —
saving a note fills them in.

## Seeing the shape of the link graph

`GET /api/v1/graph/report` reads the whole collection once and reports what the
links look like from above:

```sh
curl -s "http://127.0.0.1:8080/api/v1/graph/report?limit=20" | jq
```

- `orphan_count` — notes nothing links to;
- `isolated_count` — the subset that also links to nothing;
- `hubs` — the notes most linked *to*.

None of this is a defect, which is why it is a report and not a lint check.
Plenty of notes are entry points nobody links to. What the report is good for is
finding the parts of a library that have drifted out of the graph, and finding
the notes everything else hangs off.

`limit` caps the example lists only; the counts always describe the whole
collection. Like lint, this is a whole-library read — about 4 seconds at 100,000
notes on the reference machine — so run it deliberately.

To walk outward from one note instead, `POST /api/v1/graph` takes a `depth`
(maximum 5), and `POST /api/v1/graph/path` finds a shortest route between two
notes. A path search that comes back `depth_exhausted` or `budget_exhausted`
did not prove the notes are unconnected — it ran out of hops or visits, and
raising `max_depth` or `max_visits` may change the answer. Only `no_path` means
the search covered everything reachable. See the
[REST guide](api/rest.md) for the full shapes.

## Renaming a tag hierarchy

Tags nest with `/`: `project/alpha` is a child of `project`. Renaming the parent
can carry the children with it.

```sh
notriosctl tags rename --from project --to work                     # dry run
notriosctl tags rename --from project --to work --include-children  # dry run
notriosctl tags rename --from project --to work --include-children --apply
```

The dry run is the default, and it is not a guess. Notrios performs the rename
inside a transaction and rolls it back, so what the dry run prints is what an
apply does — including the parts that are hard to predict by hand:

```json
{
  "from": "project", "to": "work", "dry_run": true,
  "changes": [
    {"from": "project",       "to": "work",       "action": "rename", "notes": 12, "notes_gained": 12},
    {"from": "project/alpha", "to": "work/alpha", "action": "merge",  "notes": 5,  "notes_gained": 2}
  ],
  "notes": 15,
  "warnings": ["1 saved search(es) mention \"project\" and are not rewritten: Project work"]
}
```

`action: "merge"` means a tag with the destination name already existed, so the
notes join it and the old tag disappears. `notes` is how many notes carried the
old tag; `notes_gained` is how many actually change — the difference is the
notes that already had both. Read the merges before applying: the CLI exits `1`
on a dry run whose plan contains one, so a script cannot combine two hierarchies
by accident.

Three things a rename deliberately does not do:

- **It does not match part of a name.** `projects` is not a child of `project`,
  because the hierarchy is compared segment by segment.
- **It does not touch your note text.** Tags in Notrios are stored alongside
  notes, not inside them; a `#project` you wrote in a sentence is a sentence.
- **It does not rewrite saved searches.** A search notebook whose query mentions
  the old name is *reported*, as above, and left alone. Deciding which
  occurrences of a word were the tag is a guess, and a wrong guess silently
  changes what a saved search means.

A rename is bounded at 500 tags and refuses beyond that rather than doing half
of it. `POST /api/v1/tags/rename` is the same operation over REST, with the same
`dry_run` default of `true`.

## Deleting a notebook without surprises

Deleting a notebook does not delete the notes in it. They move to the Trash —
and they are re-homed to the default notebook on the way, so restoring one later
has somewhere to land. Ask before you do it:

```sh
curl -s http://127.0.0.1:8080/api/v1/notebooks/$NB/deletion-preview | jq
```

```json
{
  "notebook_id": "nb_...", "name": "Work",
  "notebooks": 3, "descendant_names": ["Reports", "Drafts"],
  "notes": 12, "trashed_notes": 1,
  "rehome_notebook_id": "nb_notes",
  "deletable": true
}
```

The preview is read-only and deletes nothing. A notebook that cannot be deleted
answers `deletable: false` with a `reason` rather than an error — "what would
happen" has an answer even when the answer is "nothing". The desktop GUI builds
its confirmation from this response; see the
[GUI guide](gui.md#deleting-a-notebook).

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

Joplin's canonical note batch also commits revisions, FTS5, link state,
provenance, tags/resources, projection outbox jobs, item fingerprints, and the
checkpoint atomically. Large inventories use a temporary indexed manifest; a
final bounded pass resolves links after all targets exist. For private-safe
capacity baselines, run:

```sh
scripts/run_full_joplin_import_profile.sh <label> /path/to/raw /tmp/joplin-full.json
```

The output contains aggregate counts, timings, SQLite settings/size, and peak
RSS only. Do not commit a report until you have confirmed it contains no source
paths, titles, bodies, resource bytes, or database files.

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
