# G18c documentation-anchor audit evidence

G18c adds a deterministic, repository-only documentation/source graph audit.
Run the checked contract with:

```bash
make docaudit
python3 performance/v0.7-g18c/validate_evidence.py
go test ./internal/docaudit -count=1
cd web && npm test -- --run src/__tests__/docaudit-ts.test.ts
```

`REPORT.json` is the complete stable JSON emitted by `go run ./cmd/docaudit`.
It accounts for all 199 existing Markdown sections, 131 currently executable-
shaped fences, nine proposed GUI journeys, and the first 12 source-adjacent
fragments. Existing prose is still authoritative and therefore remains
unverified: adding a generated or claimed source fragment does not silently
upgrade the neighbouring manual section.

The first anchor set covers the product and schema versions, configuration
keys and defaults, CLI usage forms, the REST registration surface, MCP tool
names and both ordinary/sync scope registries, exact REST destructive
confirmation, and GUI peer-retirement confirmation. The frozen G18a inventory
continues to expose that OpenAPI currently has 109 operations and zero
`operationId` values, and that the server exposes zero MCP protocol resources.

`MUTATION_MATRIX.json` indexes the deterministic failures exercised by the Go
and TypeScript tests. The TypeScript resolver deliberately uses the compiler
package owned by the web workspace; its TypeScript 7 compatibility path has
real-source regressions for `App`, `EditorPane`, and `SyncCenter`, including
nested template literals.

No manual prose was moved or corrected in this slice. In particular,
`docs/service.md`'s preserved schema-v20 statement remains a visible unverified
manual unit alongside the separately claimed canonical schema-v27 declaration.
G18d subsequently upgraded 63 of the 131 registered examples to executed
contracts. `REPORT.json` remains the frozen G18c before-execution baseline;
`performance/v0.7-g18d/REPORT.json` is the current result-bearing report.
G18e subsequently upgraded eight of the nine frozen GUI journeys to executed
evidence; editor find/replace remains explicitly unverified under the embedded-
browser owner. The current aggregate grades are therefore 71 executed, eight
generated, four claimed, and 268 unverified.
