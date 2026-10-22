package j8

// J8-B: what an MCP client can reach.
//
// Most of this slice's checklist is already proven by shipped tests inside
// internal/httpapi, and a review that re-proves them adds nothing. What those
// tests establish, read rather than assumed:
//
//   - TestEveryMCPToolIsClassified: a registered tool with no scope entry fails
//     the tests, so a tool cannot ship unclassified.
//   - TestEachScopeListsExactlyItsTools: each scope lists exactly its tools.
//   - TestHiddenToolsAreRefusedWhenCalledDirectly: a tool hidden from the
//     active scope is refused when called anyway, the refusal names the scope
//     needed, and an unknown tool is refused as unknown.
//   - TestG15SyncScopeIsExplicitAndOrthogonal: the sync scope is separate from
//     the MCP scope, so a wide MCP scope grants no sync control.
//   - TestDeprecatedProfileKey: a half-migrated configuration can only narrow.
//   - TestNoScopeReachesWholeLibraryOperations: no scope exposes lint, fix,
//     garbage collection, archive export or restore, publication, tag rename,
//     notebook deletion, document purge, graph report or export, or job
//     start, cancel, import or export.
//
// Two implementation facts this review checked by reading, because they decide
// whether the list above means anything:
//
//   - `toolsInScope` is applied inside `mcpTools()` (internal/httpapi/mcp.go),
//     so `tools/list` and the info endpoint return the filtered set rather than
//     the whole registry. A candidate finding that they did not was disproved.
//   - `handleMCPToolCall` calls `mcpScopeAllows` before its dispatch switch, so
//     the check covers every tool rather than the ones with a test.
//
// What is left is the number a deployment actually runs with: the scope an
// operator gets when they configure nothing. This file states it from the
// package's exported surface, which is all a probe outside internal/ may use —
// J8 changes no product code, so it cannot add an accessor to read more.

import (
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/httpapi"
)

// TestJ8TheScopeLadderIsWrittenDown records the scopes and their order, and the
// default an operator gets when they configure nothing.
func TestJ8TheScopeLadderIsWrittenDown(t *testing.T) {
	scopes := httpapi.MCPScopes()
	if len(scopes) == 0 {
		t.Fatal("no MCP scopes are declared")
	}
	t.Logf("scopes, narrowest first: %s", strings.Join(scopes, " -> "))

	// The ladder's ends are what a reader needs: the narrowest rung, and the
	// widest anything reachable over MCP can be.
	if scopes[0] != httpapi.MCPScopeSearchOnly {
		t.Errorf("the narrowest scope is %q, not %q", scopes[0], httpapi.MCPScopeSearchOnly)
	}
	if widest := scopes[len(scopes)-1]; widest != httpapi.MCPScopeOrganizer {
		t.Errorf("the widest scope is %q, not %q", widest, httpapi.MCPScopeOrganizer)
	}

	// The default. `effectiveMCPScope("", "")` returns read-only, and the
	// constant's own documentation says so; both are inside the package, so the
	// probe states the value it expects and the review records where it is set.
	t.Logf("default when mcp.default_scope is unset: %s (internal/httpapi/mcp_scopes.go, effectiveMCPScope)", httpapi.MCPScopeReadOnly)
	t.Logf("no administrator scope exists: destructive whole-library operations are not reachable over MCP at any scope")

	for _, scope := range scopes {
		if strings.TrimSpace(scope) == "" {
			t.Error("a scope name is empty")
		}
	}
}
