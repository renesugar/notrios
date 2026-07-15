---
name: ui-editor-preview
description: Implement built-in Markdown editor and preview behavior with safe links and resources.
---

# UI Editor Preview Skill

Use this skill when working on the React built-in UI.

## Requirements

1. Keep editor behind an adapter interface.
2. Use `md-editor-rt` for MVP unless a task explicitly requires CodeMirror-level behavior.
3. Intercept preview links:
   - `document://` opens a note route.
   - `resource://` opens/downloads via REST.
4. Upload pasted images and attachments through REST resource endpoints.
5. Sanitize Markdown preview HTML.
6. Detect remote images and offer localization, but never make preview image loading perform the server-side download implicitly.
7. Show save/index/projection/sist2 state clearly.

## Checks

- Internal note links navigate in preview.
- Resource links download through service endpoint.
- Remote images are visibly marked or actionable according to policy.
