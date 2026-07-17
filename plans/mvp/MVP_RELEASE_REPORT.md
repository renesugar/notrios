# MVP Release Report — v0.1.0 Candidate

## Status

The v0.1 MVP implementation plan is complete as of PLAN.md task 10.

## Implemented capability summary

- local `notesd` service with REST API;
- SQLite canonical store for collections, documents, revisions, resources, links, FTS5 search, and status reporting;
- built-in React/Vite UI using `md-editor-rt`;
- UI create/search/open/save workflows;
- content-addressed resource upload, attachment, listing, and download;
- preview routing for `document://` and `resource://` links;
- Markdown link/backlink parsing and graph endpoint;
- read-only MCP MVP endpoint;
- Joplin RAW importer MVP;
- Obsidian vault importer MVP;
- smoke, generated-dataset, and package verification scripts.

## Validation commands run during task 10

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp-check.zip
python3 scripts/check_release_zip.py /tmp/notes-companion-v0.1.0-mvp-check.zip
```

## Deliberately deferred

- remote-media localization and policy engine enforcement;
- perceptual hash moderation database;
- sist2 sidecar integration;
- Quartz publishing;
- Twitter/X, ChatGPT, and Claude importers;
- official MCP Go SDK integration;
- CodeMirror 6/unified editor migration;
- sync/checkpointing with go-git or Fossil;
- native C++/Qt client.

## Release recommendation

This ZIP is suitable as an initial Gitea/GitHub repository baseline and local MVP candidate. It should not be deployed as a public network service.
