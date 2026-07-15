# System Architecture

## Design goal

Build a local-first companion service for very large note and document collections, supporting hundreds of thousands of notes plus embedded images, PDFs, archives, and imported conversations.

## Core boundary

```text
GUI / Web UI / CLI / MCP client
        │
        ▼
Companion service
├── REST API
├── MCP adapter
├── Document service
├── Resource service
├── Search service
├── Link graph service
├── Import service
├── Publish service
├── Media policy service
└── sist2 adapter
        │
        ├── SQLite + FTS5 canonical store
        ├── content-addressed asset store
        ├── managed filesystem projection
        └── derived sist2 indexes
```

## Canonical storage

SQLite is the canonical application database. It stores:

- collections;
- documents;
- current document state;
- saved revisions;
- resources and blobs;
- document-resource references;
- document links and backlinks;
- block anchors;
- import provenance;
- media policy decisions;
- indexing outbox state.

SQLite FTS5 provides immediate search over managed documents. sist2 is a derived sidecar for expensive extraction over arbitrary files, OCR, thumbnails, archive traversal, and broad filesystem search.

## Document identity

Documents use stable application IDs independent of titles, filenames, folders, or source-system IDs. External source IDs are preserved in metadata:

```text
document://<collection>/documents/<document-id>
resource://<collection>/resources/<resource-id>
```

Exports can rewrite these into relative Markdown paths.

## Resource model

Resources are logical attachments or embedded media. Blobs are exact bytes addressed by hash. Multiple resources may share the same blob. Multiple documents may reference the same resource.

Remote resources must pass through media policy, quarantine, hash checks, and provenance recording before admission.

## Import model

Importers should be separate commands but shared code. They should write through the companion document service, not directly into sist2. Joplin RAW Export Directory is the preferred Joplin bulk-import format. Obsidian vaults, Twitter/X archives, ChatGPT exports, and Claude exports are normalized into collections.

## Built-in UI

The built-in UI is a React/Vite app. Initial editor choice is `md-editor-rt` for a polished split edit/preview experience. The UI must own link interception, resource upload/download, preview sanitization, and routing to document/resource URIs.

A later migration to CodeMirror 6 + unified/remark/rehype is reserved for deeper editor-pane behavior and AST-aware features.

## REST and MCP

REST and MCP are adapters over the same service layer. MCP must not expose raw SQL, arbitrary filesystem operations, or unrestricted writes. MCP tools should return snippets and resource links first, requiring explicit document/resource retrieval for larger content.

## sist2 integration

The companion service owns canonical notes and resources. It writes a managed filesystem projection for sist2 and queues changes through a durable indexing outbox. sist2 scans/indexes the projection in batches and provides derived metadata, OCR, thumbnails, and arbitrary-file search.

## Publishing

Publishing is not the same as backup. Quartz publishing profiles select a public subset of notes/resources, sanitize metadata, rewrite private links safely, and emit a Quartz-compatible content tree.

## Optional derived systems

- go-git or Fossil can checkpoint projections but should not replace SQLite revisions.
- LadybugDB can be added later as a derived graph backend if advanced traversal/analytics are needed.
- Bleve can be added later if fuzzy/faceted search requirements exceed FTS5.

## Cross-cutting design references

- Use `FEATURE_MATRIX.md` to decide whether a feature is MVP, soon, later, or optional.
- Use `UI_DESIGN.md` for built-in UI/editor behavior.
- Use `PUBLISHING_POLICY.md` for Quartz and public-subset publishing.
- Use `VERSIONING_AND_SYNC_POLICY.md` for SQLite revisions, go-git, Fossil, and external-vault sync.
- Use `WORKSPACE_MAINTENANCE.md` for Foam-style query blocks, lint/fix, outlines, and block anchors.
