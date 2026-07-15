# System Architecture

## Design goal

Build a local-first companion service for very large note and document collections, supporting hundreds of thousands of notes plus embedded images, PDFs, archives, and imported conversations.

## Core boundary

```text
Built-in Wails GUI / Web UI / CLI / MCP client / third-party clients (C++/Qt, Rust/Tauri, …)
        │
        ▼
Notrios service (notriosd)
├── REST API
├── MCP adapter
├── Document service
├── Notebook/tag service
├── Resource service
├── Search service (query-language adapter)
├── Link graph service
├── Import service
├── Publish service
├── Media policy service
└── Recoll adapter (optional sidecar)
        │
        ├── SQLite + FTS5 canonical store
        ├── content-addressed asset store
        ├── managed filesystem projection
        └── derived Recoll/Xapian indexes
```

## Canonical storage

SQLite is the canonical application database. It stores:

- collections;
- notebooks (nested, emoji icons), tags, and search-notebook queries;
- documents;
- current document state;
- saved revisions;
- resources and blobs;
- document-resource references;
- document links and backlinks;
- source provenance and conversation threads (author, author ID, thread ID, reply-to, source URL);
- block anchors;
- media policy decisions;
- indexing outbox state.

SQLite FTS5 provides immediate search over managed documents. Recoll is an optional derived sidecar for front-matter field search, expensive extraction over arbitrary files, and broad filesystem search (see `RECOLL_INTEGRATION.md`); the service degrades gracefully to FTS5 when Recoll is absent.

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

Importers should be separate commands but shared code. They should write through the document service, not directly into any search index. Joplin RAW Export Directory is the preferred Joplin bulk-import format. Obsidian vaults, Twitter/X archives, ChatGPT exports, and Claude exports are normalized into notebooks/collections with source provenance rows (including thread recovery for Twitter/X and conversation exports).

## Built-in GUI

The built-in GUI is a Go/Wails application (`notrios`) embedding the service; the React frontend hosts inside the Wails window. Modes: default (GUI + local service), `-no-gui` (headless service, for users running a different client), `-gui-only` (pure REST client, usable against a remote service and for testing the API the way a third-party client would). Layout, themes, and notebook sidebar behavior are specified in `UI_DESIGN.md`.

The GUI owns link interception, resource upload/download, preview sanitization, and routing to document/resource URIs. A later migration to CodeMirror 6 + unified/remark/rehype is reserved for deeper editor-pane behavior and AST-aware features.

## REST and MCP

REST and MCP are adapters over the same service layer, and together must be complete enough that a full-featured third-party note client can be built on them alone. MCP must not expose raw SQL, arbitrary filesystem operations, or unrestricted writes. MCP tools should return snippets and resource links first, requiring explicit document/resource retrieval for larger content.

## Recoll integration

The service owns canonical notes and resources. It writes a managed filesystem projection and queues changes through a durable indexing outbox. Recoll (user-installed, optional, external process — GPL licensing boundary in `RECOLL_INTEGRATION.md`) indexes the projection through a generated config and an enhanced from-scratch front-matter handler, providing field search, derived metadata, and arbitrary-file search. The query-language adapter (`SEARCH_QUERY_LANGUAGE.md`) translates user queries for FTS5 and Recoll.

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
