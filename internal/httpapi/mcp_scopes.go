package httpapi

import (
	"fmt"
	"sort"
	"strings"
)

// MCP tool scopes (v0.6 F2).
//
// A scope says what an MCP connection may do. It is **not authorization**:
// Notrios is single-user, so there is no second principal to authorize against
// and the scope is chosen by the same configuration file the operator controls.
// It is the user narrowing what their own agent may do — a seatbelt, not a
// lock. `SECURITY_REVIEW.md` says so at length, because a later reader must not
// cite this as an access-control boundary.
//
// The word is "scope" and not "profile" because "profile" already means two
// other shipped things: a named local database (`notriosctl profile register`)
// and a saved publication selection (`notriosctl publish profile save`). Those
// two are both "a saved named configuration", which is coherent; a permission
// tier wearing the same word is not.
//
// There are four, not the five the roadmap once listed. `administrator` is
// absent by decision: destructive whole-library operations — garbage
// collection, purge, archive restore, publication — are not reachable over MCP
// at all, so a scope naming them would cover an empty set.

// Scope names, narrowest first. The order is the authority for "narrower":
// `scopeRank` uses it, and the deprecated-alias rule below depends on it.
const (
	// MCPScopeSearchOnly can find notes and see the shape of the library. It
	// cannot read a note's body — search snippets are the most it gets.
	MCPScopeSearchOnly = "search-only"
	// MCPScopeReadOnly adds reading note content. This is the default.
	MCPScopeReadOnly = "read-only"
	// MCPScopeEditor adds single-note writes, every one of them
	// revision-preconditioned where it replaces content.
	MCPScopeEditor = "editor"
	// MCPScopeOrganizer adds bounded batch operations over an explicit note
	// list. It is the widest scope reachable over MCP.
	MCPScopeOrganizer = "organizer"
)

// MCPScopes lists every scope, narrowest first.
func MCPScopes() []string {
	return []string{MCPScopeSearchOnly, MCPScopeReadOnly, MCPScopeEditor, MCPScopeOrganizer}
}

// scopeRank returns a scope's position, or -1 when it is not a scope.
func scopeRank(scope string) int {
	for i, known := range MCPScopes() {
		if strings.EqualFold(strings.TrimSpace(scope), known) {
			return i
		}
	}
	return -1
}

// mcpToolScopes maps every tool to the narrowest scope that may call it.
//
// Scopes are cumulative: a tool available at `read-only` is available at
// `editor` and `organizer` too. That is what makes the list readable — each
// entry answers "how much trust does this need", not "which tiers include it".
//
// **Every registered tool must appear here.** `TestEveryMCPToolIsClassified`
// fails if one does not, so a tool cannot be added without someone deciding how
// much trust it needs. Defaulting an unclassified tool to the narrowest scope
// would be the dangerous kind of safe: it would ship silently.
var mcpToolScopes = map[string]string{
	// search-only: find things, and see the structure needed to search well.
	// None of these returns a note body.
	"list_collections":      MCPScopeSearchOnly,
	"list_notebooks":        MCPScopeSearchOnly,
	"get_notebook_tree":     MCPScopeSearchOnly,
	"list_tags":             MCPScopeSearchOnly,
	"list_search_notebooks": MCPScopeSearchOnly,
	"search_documents":      MCPScopeSearchOnly,

	// read-only: everything that returns note content or reads across notes.
	"get_document":            MCPScopeReadOnly,
	"get_documents":           MCPScopeReadOnly,
	"get_document_outline":    MCPScopeReadOnly,
	"get_note_line_range":     MCPScopeReadOnly,
	"search_in_note":          MCPScopeReadOnly,
	"get_notebook_notes":      MCPScopeReadOnly,
	"list_document_links":     MCPScopeReadOnly,
	"list_document_resources": MCPScopeReadOnly,
	"scan_remote_media":       MCPScopeReadOnly,
	"plan_selection":          MCPScopeReadOnly,

	// editor: single-note writes.
	"create_note":           MCPScopeEditor,
	"update_note":           MCPScopeEditor,
	"append_to_note":        MCPScopeEditor,
	"prepend_to_note":       MCPScopeEditor,
	"edit_note":             MCPScopeEditor,
	"delete_note":           MCPScopeEditor,
	"move_note_to_notebook": MCPScopeEditor,
	"localize_remote_media": MCPScopeEditor,

	// organizer: many notes at once.
	"run_batch": MCPScopeOrganizer,
}

// effectiveMCPScope resolves the configured scope, honouring the deprecated
// `mcp.default_profile` key.
//
// When both are set and they disagree, **the narrower wins** and the caller is
// told. A configuration key that silently stops applying is bad in general; for
// this key it would *widen* what an agent may do, which is the one direction
// that must never happen quietly. Taking the narrower of the two makes a
// half-migrated config strictly no more permissive than either half intended.
func effectiveMCPScope(scope, deprecatedProfile string) (string, string) {
	scopeSet := strings.TrimSpace(scope) != ""
	profileSet := strings.TrimSpace(deprecatedProfile) != ""

	switch {
	case !scopeSet && !profileSet:
		return MCPScopeReadOnly, ""
	case scopeSet && !profileSet:
		resolved, warning := normalizeMCPScope(scope, "mcp.default_scope")
		return resolved, warning
	case !scopeSet && profileSet:
		resolved, warning := normalizeMCPScope(deprecatedProfile, "mcp.default_profile")
		note := "mcp.default_profile is deprecated; rename it to mcp.default_scope"
		if warning != "" {
			note = warning + " " + note
		}
		return resolved, note
	}

	fromScope, scopeWarning := normalizeMCPScope(scope, "mcp.default_scope")
	fromProfile, profileWarning := normalizeMCPScope(deprecatedProfile, "mcp.default_profile")
	narrower := fromScope
	if scopeRank(fromProfile) < scopeRank(fromScope) {
		narrower = fromProfile
	}
	note := fmt.Sprintf(
		"both mcp.default_scope (%q) and the deprecated mcp.default_profile (%q) are set; using the narrower %q. Remove mcp.default_profile.",
		fromScope, fromProfile, narrower)
	for _, warning := range []string{scopeWarning, profileWarning} {
		if warning != "" {
			note = warning + " " + note
		}
	}
	return narrower, note
}

// normalizeMCPScope falls back to the default for an unrecognized value, and
// says so. Failing closed to the *narrowest* scope would be defensible, but a
// typo silently disabling reads looks like a broken service; falling back to
// the documented default with a warning is the behaviour an operator can debug.
func normalizeMCPScope(value, key string) (string, string) {
	if scopeRank(value) >= 0 {
		return strings.ToLower(strings.TrimSpace(value)), ""
	}
	return MCPScopeReadOnly, fmt.Sprintf("%s=%q is not a known scope (%s); using %q.",
		key, value, strings.Join(MCPScopes(), ", "), MCPScopeReadOnly)
}

// mcpScope is the active scope for this server.
func (s *Server) mcpScope() string {
	scope, _ := effectiveMCPScope(s.config.MCP.DefaultScope, s.config.MCP.DefaultProfile)
	return scope
}

// mcpScopeAllows reports whether the active scope may call a tool.
//
// An unknown tool is refused. So is a *known* tool with no scope entry — that
// is a programming error, and answering it would be worse than failing.
func (s *Server) mcpScopeAllows(tool string) bool {
	required, classified := mcpToolScopes[tool]
	if !classified {
		return false
	}
	return scopeRank(s.mcpScope()) >= scopeRank(required)
}

// mcpScopeError explains a refusal in terms the caller can act on: which scope
// the tool needs, and which one is active.
func (s *Server) mcpScopeError(tool string) error {
	required, classified := mcpToolScopes[tool]
	if !classified {
		return fmt.Errorf("unknown MCP tool %q", tool)
	}
	return fmt.Errorf("tool %q requires the %q MCP scope or wider; the active scope is %q (set mcp.default_scope)",
		tool, required, s.mcpScope())
}

// mcpToolsForScope filters a tool list to what the active scope may call.
func (s *Server) toolsInScope(tools []mcpTool) []mcpTool {
	allowed := make([]mcpTool, 0, len(tools))
	for _, tool := range tools {
		if s.mcpScopeAllows(tool.Name) {
			allowed = append(allowed, tool)
		}
	}
	return allowed
}

// classifiedMCPToolNames lists every tool that has a scope, sorted. Used by
// tests and by the status endpoint.
func classifiedMCPToolNames() []string {
	names := make([]string, 0, len(mcpToolScopes))
	for name := range mcpToolScopes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
