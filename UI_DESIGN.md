# Built-in UI Design

The built-in UI is a basic browser client for local note taking, searching, resource management, and MCP-adjacent workflows. It is not the only future client: a native C++/Qt client can be developed later against the same REST API.

## UI stack decision

Use **React + Vite** for the built-in UI.

Use **`md-editor-rt` initially** for a polished Joplin-like editor/preview experience. Wrap it behind an application-owned adapter so the editor can later migrate to `React + CodeMirror 6 + unified/remark/rehype` if deeper source-position and editor-pane behavior is required.

## Required MVP behavior

- Create, edit, save, and search Markdown notes.
- Display split edit/preview panes.
- Intercept preview links:
  - `document://...` opens the target note in the UI.
  - `resource://...` opens or downloads the resource through REST.
- Upload/paste images and attach PDFs/resources through the companion REST API.
- Show outgoing links, backlinks, unresolved links, and resources for the active note.
- Show revision/conflict state using revision IDs or ETags.
- Sanitize preview HTML.

## Why `md-editor-rt` is enough for MVP

The MVP only requires link handling in the preview pane, not rich link widgets inside the raw Markdown editor. A custom preview component or event delegation can intercept rendered `<a>` links and route them through application actions.

Move to CodeMirror 6 + unified when requirements include:

- Ctrl-click inside the editor pane.
- Broken-link markers while typing.
- Inline resource widgets in source editing mode.
- AST/source-range safe edits.
- Rich Obsidian/Joplin link autocomplete tied to source positions.

## LeafWiki-inspired layout

LeafWiki is a useful UI reference, but not a storage/backend reference. The companion UI should adopt the pattern:

```text
left: collections/folders/search
center: editor + preview or viewer
right: links/backlinks/resources/revisions/properties
bottom/status: saved/index/projected/sist2 state
```

## Remote media in preview

The preview can detect remote images and show a warning/action, but it must not silently localize remote media. Localization is a server operation using media policy, quarantine, hash checks, MIME validation, and revision-safe note rewriting.

## Resource downloads

Resource links should call REST endpoints such as:

```text
GET /api/v1/resources/{resource_id}/content?download=1
```

The server must set appropriate `Content-Type` and `Content-Disposition` headers. The UI should not expose raw storage paths.
