package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

// J27: a host written in a form DNS treats as the same name meets the same
// rules (J8 finding J8-F1), and a host no resolver treats as a name is refused.

func j27Policy() *Policy {
	return NewPolicy(config.RemoteMediaConfig{
		DefaultAction:  ActionReview,
		BlockedSchemes: []string{"file", "data", "javascript", "ftp"},
		BlockedDomains: []string{"blocked.example.org", "*.bad.example.org"},
		AllowedDomains: []string{"images.example.org", "*.cdn.example.org"},
		ReviewDomains:  []string{"review.example.net", "*.queue.example.net"},
	})
}

func TestJ27ATrailingDotHostMeetsTheSameRules(t *testing.T) {
	policy := j27Policy()
	cases := []struct {
		dotted, undotted, want string
	}{
		{"https://blocked.example.org./a.png", "https://blocked.example.org/a.png", ActionBlock},
		{"https://x.bad.example.org./a.png", "https://x.bad.example.org/a.png", ActionBlock},
		{"https://images.example.org./a.png", "https://images.example.org/a.png", ActionAllow},
		{"https://edge.cdn.example.org./a.png", "https://edge.cdn.example.org/a.png", ActionAllow},
		{"https://review.example.net./a.png", "https://review.example.net/a.png", ActionReview},
		{"https://item.queue.example.net./a.png", "https://item.queue.example.net/a.png", ActionReview},
		{"https://BLOCKED.Example.ORG.:8443/a.png", "https://BLOCKED.Example.ORG:8443/a.png", ActionBlock},
		{"https://user@blocked.example.org./a.png", "https://user@blocked.example.org/a.png", ActionBlock},
	}
	for _, c := range cases {
		// The review list is only distinguishable from the default action by
		// its reason, so compare reasons too: both forms must match one rule.
		wantAction, wantReason := policy.Evaluate(c.undotted)
		if wantAction != c.want {
			t.Fatalf("fixture drift: %q = %s, want %s", c.undotted, wantAction, c.want)
		}
		action, reason := policy.Evaluate(c.dotted)
		if action != wantAction || reason != wantReason {
			t.Errorf("Evaluate(%q) = %s (%s); want %s (%s), as for %q", c.dotted, action, reason, wantAction, wantReason, c.undotted)
		}
	}
}

func TestJ27ATrailingDotCannotDisguiseAPrivateHost(t *testing.T) {
	policy := j27Policy()
	for _, raw := range []string{
		"http://localhost./a.png",
		"http://media.localhost./a.png",
		"http://LOCALHOST./a.png",
		"http://127.0.0.1./a.png",
		"http://10.1.2.3.:8080/a.png",
	} {
		if action, reason := policy.Evaluate(raw); action != ActionBlock {
			t.Errorf("Evaluate(%q) = %s (%s); a dotted private host must be blocked like its undotted form", raw, action, reason)
		}
	}

	open := j27Policy()
	open.cfg.AllowPrivateNetworks = true
	if action, reason := open.Evaluate("http://127.0.0.1./a.png"); action == ActionBlock {
		t.Errorf("allow_private_networks=true must not block a dotted loopback literal: %s", reason)
	}
}

func TestJ27AHostWithAnEmptyLabelIsBlockedAsMalformed(t *testing.T) {
	policy := j27Policy()
	for _, raw := range []string{
		"https://./a.png",
		"https://images.example.org../a.png",
		"https://images..example.org/a.png",
		"https://.images.example.org/a.png",
		"https://unlisted.example.com../a.png",
	} {
		action, reason := policy.Evaluate(raw)
		if action != ActionBlock || reason != "malformed host" {
			t.Errorf("Evaluate(%q) = %s (%s); want block (malformed host)", raw, action, reason)
		}
	}
}

func TestJ27ADottedPatternMeansTheUndottedName(t *testing.T) {
	policy := NewPolicy(config.RemoteMediaConfig{
		DefaultAction:  ActionReview,
		BlockedDomains: []string{"blocked.example.org.", "*.bad.example.org."},
		AllowedDomains: []string{"Images.Example.org."},
	})
	cases := []struct{ url, want string }{
		{"https://blocked.example.org/a.png", ActionBlock},
		{"https://blocked.example.org./a.png", ActionBlock},
		{"https://x.bad.example.org/a.png", ActionBlock},
		{"https://images.example.org/a.png", ActionAllow},
	}
	for _, c := range cases {
		if action, reason := policy.Evaluate(c.url); action != c.want {
			t.Errorf("Evaluate(%q) = %s (%s); want %s", c.url, action, reason, c.want)
		}
	}
}

func TestJ27AWildcardKeepsItsMeaning(t *testing.T) {
	policy := j27Policy()
	cases := []struct{ url, want string }{
		{"https://cdn.example.org/a.png", ActionReview},     // the apex is not the wildcard
		{"https://cdn.example.org./a.png", ActionReview},    // nor is its dotted form
		{"https://notcdn.example.org/a.png", ActionReview},  // a suffix that is not a subdomain
		{"https://notcdn.example.org./a.png", ActionReview}, // dotted or not
		{"https://a.b.cdn.example.org./a.png", ActionAllow}, // any depth of subdomain
	}
	for _, c := range cases {
		if action, reason := policy.Evaluate(c.url); action != c.want {
			t.Errorf("Evaluate(%q) = %s (%s); want %s", c.url, action, reason, c.want)
		}
	}
}

// TestJ27AFetchRefusesADottedBlockedHost proves the fix where it matters: a note
// URL, and a redirect hop, naming a blocked host with a trailing dot are refused
// by the blocked rule before any connection. Review URLs are permitted here, so
// on the code before J27 both would have been fetched under the default action.
func TestJ27AFetchRefusesADottedBlockedHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/to-dotted-blocked" {
			http.Redirect(w, r, "http://tracker.example.com./x.png", http.StatusFound)
			return
		}
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()
	fetcher := testFetcher(t, server.URL, nil, nil)

	results := fetcher.Quarantine(context.Background(), QuarantineRequest{
		URLs:        []string{"http://tracker.example.com./pixel.gif", server.URL + "/to-dotted-blocked"},
		AllowReview: true,
	})
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	for _, result := range results {
		if result.Status != StatusRefused || !strings.Contains(result.Reason, "blocked pattern tracker.example.com") {
			t.Errorf("%s: %+v; want refused by the blocked pattern", result.URL, result)
		}
	}
}
