# 010 — MVP Markdown Link Parser and Graph

## Status

Completed.

## Goal

Leave the project in a working state where managed Markdown notes produce durable outgoing link records, backlinks can be queried, and a small graph slice can be returned through REST.

## Implemented

- Conservative Markdown/Obsidian/app URI link parser in `internal/markdownlinks`.
- Transactional link rebuild on create/update/restore and outgoing-link cleanup on soft delete.
- Link resolution for document URIs, resource URIs, exact note titles, exact resource filenames, external links, and unresolved links.
- REST endpoint for outgoing links and backlinks.
- REST graph-slice endpoint.
- UI link/backlink display for opened notes.
- Tests for parser, store link extraction/backlinks/rebuild, HTTP link endpoint, and graph endpoint.
- Schema version 4.

## Working state

The repo validates with Go tests, required-file checks, scaffold validation, and web typecheck/build. The service can still create/read/update/delete/search notes and upload/list/download resources.

## Next task

Proceed to PLAN.md task 6: Built-in React UI MVP, especially `md-editor-rt` preview link handling for `document://` and `resource://`.
