# Publishing Policy

Publishing is not backup/export. Publishing produces a sanitized public subset of the note database. Backup/export preserves enough information to restore private application state.

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

## Shared publish planner

Every handoff target consumes one neutral, deterministic plan containing selected note
IDs, stable output paths, rewritten link decisions, reachable resource hashes,
metadata-removal decisions, and privacy warnings. Planning must not depend on a
particular site generator. Full archive, subset transfer, and publication
handoff therefore share the hard security and selection work.

P1 implements this neutral layer as `PlanSelection`,
`POST /api/v1/selection/plan`, and the read-only MCP `plan_selection` tool.
The live planner is content-free and read-only: it returns stable IDs, hashes,
counts, link/privacy decisions, exclusions, metadata decisions, and a complete
manifest digest, never note bodies, source metadata JSON, raw resource bytes,
or local paths. See `SELECTION_AND_PRIVACY_PLANNER.md`.

P2 binds that complete planner digest into the strict archive-v2 snapshot
manifest. The format verifier admits only immutable hash-addressed objects,
bounded typed records, safe relative source-bundle paths, and consistent
schema/capability/MIME/count/reference metadata. P3 streams only the records and
bytes the plan authorizes.

P7 implements the publication itself (`notriosctl publish`). A publication
handoff is a projection, not an archive of canonical state: current revisions
only, no trashed notes, no provenance, no exact source bundles, no saved
searches, and stripped revision metadata. It is the one export that rewrites
note content — links to withheld or unresolved targets become plain text or a
redaction placeholder — and the corresponding link records are dropped rather
than published, because a record carries the withheld note's ID, its raw
target, and a context excerpt of the surrounding sentence. Retained links keep
byte offsets adjusted onto the published body. Content-rewriting link actions
are refused for `full_archive`: a backup's promise is that what comes out is
what went in.

## Publishing implementation boundary

Notrios plans and emits a scoped, sanitized native-archive-v2 handoff. The
separately maintained MIT-licensed `movenotes-v3` toolkit consumes that handoff
through a planned `notrios2sql.py` importer and owns the downstream projections:

- Obsidian vault output for interoperability and Quartz;
- Hugo project generation using `hugo-theme-ledger`;
- Pagefind for bounded static sites and Bluge for large server-backed archives.

Notrios owns:

- selection of public notes/resources;
- private/draft/confidential exclusion;
- resource reachability analysis;
- link rewriting for private/missing targets;
- metadata stripping;
- dry-run privacy warnings;
- writing a checksum-verified, explicitly subset-scoped publication handoff.

Notrios does not duplicate the portable-vault, Quartz, Hugo/Ledger, Pagefind, or
Bluge implementations. It may invoke a user-installed compatible movenotes
command as an external process after explicit plan approval, but never links,
vendors, or silently downloads it.

Quartz remains the curated/smaller-library target through movenotes' Obsidian
projection. Hugo/Ledger with Bluge is the measured large-library route: fixed
bounded navigation and server-side search, with Pagefind only as a bounded
static fallback.

## Publishing profile example

Profiles are saved with `notriosctl publish profile save` into
`<data-dir>/publish-profiles.json` (owner-only). They record selection and
privacy decisions only — never an output path, a command, or anything derived
from note content. The shape below is the equivalent of the implemented flags:

```yaml
profiles:
  public-research:
    target: publication_handoff
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
- Run a dry-run plan before publishing. Execution requires the digest that plan
  printed: Notrios re-plans at publication time and refuses when the library no
  longer matches the reviewed decision.
- Never execute note content. A profile cannot name a command and publishing
  runs no build step; conversion happens in a separate toolkit, invoked by the
  user.

## Dry-run output

A publish dry run must report:

- included notes count;
- included resources count;
- excluded notes/resources and reasons, including
  `read_only_notebook:<id>` for Notrios' own Help and Reports notes;
- links to private or missing targets;
- remote media decisions;
- oversized resources;
- metadata-stripping warnings.

The live P1 report additionally classifies internal/private/broken/external
links, reports exact source-bundle availability/inclusion, and caps detail
arrays while keeping complete counts and `manifest_sha256`.

## Downstream publishing targets

- Obsidian/Quartz for curated subsets, produced by `movenotes-v3`.
- Hugo with `hugo-theme-ledger` and Bluge for large archives, produced by
  `movenotes-v3`; Pagefind remains the static fallback.
- Optional Foam-style query/dashboard materialization after the archive bridge.

## Evidence and compatibility

`movenotes-v3` and `hugo-theme-ledger` contain measured 10k/100k/500k build and
search evidence, bounded taxonomy/navigation rules, neutral streamed JSONL,
and backend-specific unsupported-operator reporting. Notrios consumes those as
behavioral and integration evidence, not as code to copy. Each repository
retains its own tests, license, and release.

The compatibility contract that pins archive schemas, capabilities, and privacy
outcomes across the two repositories is **deferred to v0.7**, gated on the
snapshot/change container slice: that slice extends the container the contract
would pin, and `movenotes-v3` has not started the `notrios2sql.py` importer.
Until then Notrios emits a checksum-verified, explicitly subset-scoped
archive-v2 handoff admitted by its own verifier, and does not claim an external
consumer.
