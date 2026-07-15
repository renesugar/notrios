---
name: rest-mcp-adapter
description: Expose service-layer behavior through REST and MCP without duplicating logic or widening permissions.
---

# REST/MCP Adapter Skill

Use this skill when the active task matches the description.

## Steps

1. Define service-layer request/response structs first.
2. Make REST and MCP call the same service methods.
3. Use conservative MCP limits and snippets.
4. Return resource/document links rather than huge payloads by default.
5. Do not add raw SQL, unrestricted paths, or unscoped write tools.

## Working-state checks

- REST and MCP parity tests for the same service method.
- MCP output-size limit tests.
