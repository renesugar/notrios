# Plan Slice 011: MVP Built-in React UI

## Task

Implement `PLAN.md` task 6: integrate `md-editor-rt` into the built-in UI and route preview links for app-owned document/resource URIs.

## Completed work

- Added `md-editor-rt` editor/preview in place of the textarea placeholder.
- Added custom preview link routing for `document://` and `resource://`.
- Added preview HTML normalization and resource image source rewriting.
- Added editor image-upload integration with the REST resource store.
- Preserved create/search/open/save/resource/link-sidebar workflows.
- Served `web/dist` from `notesd` when present.
- Updated docs, agent status, and handoff notes.

## Working state

The UI can create, edit, preview, search, open, and save notes; upload/attach/download local resources; open app-owned note links in preview; and download app-owned resource links in preview.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```
