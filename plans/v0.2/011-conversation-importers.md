# v0.2 Tasks R10 + R11 — ChatGPT and Claude conversation importers

Completed: 2026-07-15 (single slice; user approved combining). Model: Claude Fable 5 (claude-fable-5).

## Design decision

One note per conversation (not per message): a chat reads naturally as a single Markdown document with `## Role — timestamp` sections; the provenance row's thread ID is the conversation ID, keeping thread queries consistent with the Twitter importer.

## Changes

- `internal/importers/chatgpt` (refs: temnoon/openai_export_parser, slyubarskiy/chatgpt-conversation-extractor): parses the mapping tree, renders the **current-node main path** oldest-first (abandoned edit/regeneration branches excluded; falls back to create-time order when current_node is absent), skips system/tool and empty messages, extracts `parts` string content, dated fallback titles, 🤖 "ChatGPT" notebook (`nb_chatgpt`).
- `internal/importers/claude`: flat `chat_messages` with `text` or `content[].text` blocks, human/assistant headings, dated fallback titles, ✳️ "Claude" notebook (`nb_claude`).
- Shared upsert semantics (mirroring R9): deterministic doc IDs, unchanged/update detection with revision preconditions, user-trashed conversations never resurrected (provenance refresh only), purge protection via document_sources.
- `notriosctl import chatgpt|claude [--notebook] [--dry-run] <conversations.json|export-dir>` (accepts the file or its containing directory).
- `testdata/schemas/{chatgpt,claude}-conversations.schema.json` derived from the synthetic fixtures with `uvx genson`.

## Validation

`go test ./...` (4 new fixture tests covering main-path ordering, branch exclusion, role headings, fallback titles, provenance/thread rows, idempotent re-import, trash rules, purge protection, dry run), `check_required_files`, `validate-scaffold`, `mvp_smoke.sh` — all passing.
