# Publishing Policy

Publishing is not backup/export. Publishing produces a sanitized public subset of the note database. Backup/export preserves enough information to restore private application state.

## Shared publish planner

Every target consumes one neutral, deterministic plan containing selected note
IDs, stable output paths, rewritten link decisions, reachable resource hashes,
metadata-removal decisions, and privacy warnings. Planning must not depend on a
particular site generator. Portable-vault, Quartz, and large-library targets
therefore share the hard security and selection work.

## Quartz as curated target

Support Quartz publish profiles for users who want to publish selected notebooks/folders/subfolders/tags without exporting the full database.

The companion service owns:

- selection of public notes/resources;
- private/draft/confidential exclusion;
- resource reachability analysis;
- link rewriting for private/missing targets;
- metadata stripping;
- dry-run privacy warnings;
- writing a Quartz-compatible `content/` tree.

Quartz owns static-site rendering.

Quartz is the first curated/smaller-library target, not an unmeasured promise
for a 100k-note public archive. Large publish sets require a separate profile
with bounded/fixed navigation and server-side search.

## Publishing profile example

```yaml
profiles:
  public-research:
    target: quartz
    output: /sites/research-quartz/content
    include:
      notebooks:
        - Research/Public
      folders:
        - papers/public
      tags:
        - publish
    include_descendants: true
    exclude:
      tags: [private, draft, confidential]
    link_policy: include_public_targets_only
    unresolved_link_policy: strip_or_plain_text
    resources:
      include_linked_resources: true
      include_unreferenced_resources: false
      localize_remote_images: true
      apply_media_policy: company-default
```

## Privacy rules

- Never rely on static-site-generator private-page filters alone to protect resources.
- Copy only resources reachable from selected public notes.
- Strip private metadata such as source IDs, import errors, moderation decisions, local paths, and private tags unless explicitly allowed.
- Convert links to private notes into plain text, redacted placeholders, or warnings according to profile policy.
- Run a dry-run plan before publishing.

## Dry-run output

A publish dry run must report:

- included notes count;
- included resources count;
- excluded notes/resources and reasons;
- links to private or missing targets;
- remote media decisions;
- oversized resources;
- metadata-stripping warnings.

## Future publishing targets

- Portable Markdown vault export.
- Quartz publishing.
- Optional Foam-style query/dashboard materialization.
- A scalable Hugo/Relearn-style or equivalent archive site: streamed
  generation, fixed sidebar rather than the entire note tree, and a small
  server-side search service.
- Static PageFind remains appropriate for the small documentation site and may
  be offered as a bounded archive fallback; it is not the default for a
  hundreds-of-thousands-of-notes site.

## Search backend spike

The scalable-site search boundary requires text/phrase/prefix queries, stable
pagination, highlighting, typed fields, facets/aggregations, deterministic
rebuilds, bounded memory, and an Apache-2.0/MIT-compatible dependency story.

Bluge currently exposes BM25 search, fuzzy/prefix/phrase/range queries,
highlights, facets, custom sorts, and search-after. It is Apache-2.0, but the
reviewed upstream has seen no commit since 2022. Recoll/Xapian is more capable
for extraction but remains an external GPL process. SQLite FTS5 is already
present but offers fewer site-search features. v0.4 must benchmark and assess
maintenance/security before selecting an adapter; the publish manifest and
frontend must not bind to one engine.

The `movenotes-v3` implementation is a useful behavioral reference for exact
source preservation, streamed JSONL, fixed navigation, deterministic/bucketed
tag metadata, bounded indexing batches, and small-vs-large site profiles. It is
not a source from which to copy code.
