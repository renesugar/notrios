# MVP Task 6 Report: Built-in React UI MVP

## Status

Completed.

## Implemented

- Replaced the textarea placeholder with `md-editor-rt` for Markdown editing and preview.
- Added a custom preview renderer that intercepts app-owned links:
  - `document://.../documents/{id}` opens the target note in the current UI without navigating away.
  - `resource://.../resources/{id}` downloads the resource through `/api/v1/resources/{id}/content?download=1`.
- Added preview HTML normalization for the MVP:
  - removes script/style/iframe/object/embed/form/input/button/meta/link elements;
  - removes inline event handlers and inline styles;
  - rewrites resource image sources to local REST content URLs;
  - removes unsafe non-app/non-HTTP link schemes.
- Wired `md-editor-rt` image upload to the content-addressed resource store and document-resource attachment API.
- Preserved existing UI workflows: create, search, open, save revision, upload/attach/list/download resources, and show outgoing links/backlinks.
- Made resolved links in the sidebar actionable.
- Added static serving of `web/dist` from `notesd` when a production UI build exists, while preserving the Vite dev-server workflow.

## Working state

A developer can now:

1. run `go run ./cmd/notesd` from the repository root;
2. either open the built UI from `http://127.0.0.1:8080/` after `cd web && npm run build`, or use `cd web && npm run dev` during development;
3. create notes and save revisions;
4. paste or type Markdown links to `document://` and `resource://` URIs;
5. click a `document://` link in preview to open another note;
6. click a `resource://` link in preview to download the stored local resource;
7. upload local images from the editor image tool into the resource store.

## Deliberate limitations

- The preview sanitizer is an MVP browser-side sanitizer. It is safer than raw preview HTML, but Codex should evaluate replacing or supplementing it with a pinned sanitizer package and explicit allowlist tests.
- `md-editor-rt` pulls in a large production bundle with many language/highlight chunks. Codex should evaluate lazy loading, toolbar reductions, or a later migration to CodeMirror 6 + unified/remark/rehype when deeper editor control is needed.
- The custom preview only handles app URI navigation in the rendered preview, not Ctrl-click behavior inside the source editor pane.
- Obsidian/Foam-style wikilinks are parsed by the backend link graph, but `md-editor-rt` does not render them as clickable preview links yet.
- UI tests are still type/build checks; browser automation with Playwright or equivalent remains a future task.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

All commands passed during this task. A manual smoke test also confirmed that `notesd` serves `web/dist/index.html` from `/` when the UI has been built.
