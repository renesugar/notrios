# Publishing Policy

Publishing is not backup/export. Publishing produces a sanitized public subset of the note database. Backup/export preserves enough information to restore private application state.

## Quartz as first-class target

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
- Optional simple static HTML export.
