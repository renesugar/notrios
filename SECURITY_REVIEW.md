# Security Review — MVP

This MVP is designed as a local-first single-user application. It should still be treated as a network service because it exposes REST and MCP endpoints on an HTTP listener.

## Reviewed areas

### Resource download endpoint

Implemented controls:

- resources are stored under content-addressed SHA-256 paths, not user filenames;
- logical resource filenames are metadata only;
- resource content responses set `X-Content-Type-Options: nosniff`;
- `Content-Disposition` is generated with `mime.FormatMediaType` and sanitized filenames;
- resource deletion refuses referenced resources with `409 Conflict`;
- exact duplicate bytes reuse existing blob files.

Remaining work:

- configurable upload-size limits;
- media MIME allow/review/block policy enforcement;
- resource garbage-collection retention windows;
- antivirus/malware scanning hooks for enterprise deployments.

### Markdown preview sanitization

Implemented controls:

- the built-in UI removes scripts, styles, iframes, objects, embeds, forms, inputs, buttons, metadata tags, event-handler attributes, and inline styles from preview HTML;
- `document://` preview links are intercepted and routed inside the application;
- `resource://` preview links are rewritten to local REST resource-content URLs;
- non-HTTP, non-mailto, non-fragment external links are stripped from anchors;
- non-local image sources are restricted to `http(s)` or `data:image/` in the MVP sanitizer.

Remaining work:

- replace the MVP DOM sanitizer with a pinned, reviewed sanitizer package and explicit allowlist;
- implement remote-media policy warnings in preview;
- block or proxy remote images according to organization policy;
- add browser-driven UI tests using Playwright or equivalent.

### MCP endpoint

Implemented controls:

- MCP is read-only in v0.1;
- raw SQL and arbitrary filesystem access are unavailable;
- document bodies returned through MCP are size-limited;
- returned document bodies are marked as untrusted data;
- tool result limits are clamped by configuration.

Remaining work:

- replace or wrap the MVP adapter with the official Go MCP SDK where practical;
- add authentication/profile gates before write tools are introduced;
- add audit logs for MCP tool calls;
- add prompt-injection guidance to all MCP resource/tool output documentation.

### Importers

Implemented controls:

- Joplin RAW and Obsidian importers use deterministic IDs to support idempotent re-runs;
- imports write through the companion store instead of bypassing SQLite/resource ownership;
- local resources become content-addressed blobs.

Remaining work:

- dry-run diffs for large imports;
- import quarantine for suspicious or disallowed media;
- loop/stall-aware import checkpoints;
- support for larger real-world test archives.

## Recommended deployment posture for v0.1

Use the default loopback listener:

```yaml
server:
  listen_addr: "127.0.0.1:8080"
```

Do not bind to `0.0.0.0` or expose the service on a LAN until authentication, CSRF protection, CORS policy, and profile-based authorization are implemented.

## Release-blocking issues

No release-blocking issue is known for local-only MVP testing. Public or multi-user deployment is out of scope for v0.1.
