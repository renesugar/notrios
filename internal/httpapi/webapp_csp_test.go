package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// The built-in UI is served with a Content-Security-Policy because v0.5 E6a
// found the editor loading KaTeX, highlight.js, and four other libraries from
// unpkg.com at runtime. Those are bundled now; this header is what stops the
// next dependency from quietly reintroducing a remote script.
func TestWebAppServesAContentSecurityPolicy(t *testing.T) {
	s := NewServer()
	// web/dist is not built in a unit-test environment, so the handler answers
	// `web_ui_not_built` — the headers are set before that branch on purpose,
	// since a policy that only applies on the success path is not a policy.
	rr := doJSON(t, s, http.MethodGet, "/", "")

	policy := rr.Header().Get("Content-Security-Policy")
	if policy == "" {
		t.Fatalf("no Content-Security-Policy on the web app response (status %d)", rr.Code)
	}
	for _, directive := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"font-src 'self' data:",
		"connect-src 'self'",
		"object-src 'none'",
	} {
		if !strings.Contains(policy, directive) {
			t.Fatalf("policy is missing %q: %s", directive, policy)
		}
	}
	// The directive that does the work: no third-party origin may supply script.
	if strings.Contains(policy, "script-src 'self' http") || strings.Contains(policy, "script-src 'self' https") {
		t.Fatalf("script-src admits a remote origin: %s", policy)
	}
	// CodeMirror and md-editor-rt inject <style> elements at runtime, so inline
	// styles are allowed — but a remote stylesheet still is not.
	if !strings.Contains(policy, "style-src 'self' 'unsafe-inline'") {
		t.Fatalf("style-src should allow inline styles and nothing remote: %s", policy)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing nosniff on the web app response")
	}
	if rr.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("missing Referrer-Policy on the web app response")
	}
}
