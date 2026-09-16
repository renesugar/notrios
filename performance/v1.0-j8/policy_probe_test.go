// Package j8 holds the security review's probes (v1.0 J8).
//
// J8 records findings and changes no product code (owner decision,
// 2026-09-16), so these probes only observe: each one states what it attempted
// and prints what the shipped code did. A probe that finds a gap does not fix
// it; the finding is written up in README.md and deferred to its own plan item.
//
// This file is the half that needs no network: URL policy evaluation, which is
// documented as performing no I/O, not even DNS.
package j8

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/media"
)

// reviewPolicy is the shipped default posture: private networks refused, no
// domain lists, and "review" as the default action.
func reviewPolicy() *media.Policy {
	return media.NewPolicy(config.RemoteMediaConfig{
		DefaultAction:        media.ActionReview,
		BlockedSchemes:       []string{"file", "data", "javascript", "ftp"},
		AllowPrivateNetworks: false,
	})
}

// TestJ8AddressLiteralsThePrivateCheckRefuses walks the address literals an
// SSRF attempt would reach for, and records which the static policy refuses as
// private. The claim under test is `SECURITY_AND_MEDIA_POLICY.md`: "Block
// private network and link-local addresses by default."
//
// It asserts only the ranges the policy names. Everything else is reported, so
// the review states what is and is not covered rather than assuming.
func TestJ8AddressLiteralsThePrivateCheckRefuses(t *testing.T) {
	policy := reviewPolicy()
	type probe struct {
		host          string
		what          string
		namedByPolicy bool // named by the written policy: private or link-local
	}
	probes := []probe{
		{"127.0.0.1", "IPv4 loopback", true},
		{"10.0.0.1", "RFC 1918 private", true},
		{"172.16.0.1", "RFC 1918 private", true},
		{"192.168.1.1", "RFC 1918 private", true},
		{"169.254.169.254", "IPv4 link-local (cloud metadata)", true},
		{"[::1]", "IPv6 loopback", true},
		{"[fc00::1]", "IPv6 unique-local", true},
		{"[fe80::1]", "IPv6 link-local", true},
		{"[::ffff:127.0.0.1]", "IPv4-mapped IPv6 loopback", true},
		{"[::ffff:10.0.0.1]", "IPv4-mapped IPv6 private", true},
		{"0.0.0.0", "unspecified", true},
		{"localhost", "loopback by name", true},
		// Reserved ranges the written policy does not name. Reported, not
		// asserted: whether they belong in the refusal set is a finding for the
		// owner, not a decision this probe makes.
		{"100.64.0.1", "carrier-grade NAT (RFC 6598)", false},
		{"198.18.0.1", "benchmarking (RFC 2544)", false},
		{"192.0.0.1", "IETF protocol assignments (RFC 6890)", false},
		{"240.0.0.1", "reserved for future use (RFC 1112)", false},
		{"0.1.2.3", "this network (RFC 1122 0.0.0.0/8)", false},
		{"[fec0::1]", "IPv6 site-local, deprecated (RFC 3879)", false},
		{"[64:ff9b::7f00:1]", "NAT64 well-known prefix (RFC 6052)", false},
		{"224.0.0.2", "IPv4 multicast", false},
		{"255.255.255.255", "IPv4 broadcast", false},
	}

	refused, allowed := []string{}, []string{}
	for _, p := range probes {
		action, reason := policy.Evaluate("https://" + p.host + "/photo.png")
		isPrivateRefusal := action == media.ActionBlock && strings.Contains(reason, "private, loopback, or link-local")
		line := fmt.Sprintf("%-22s %-40s -> %s (%s)", p.host, p.what, action, reason)
		if isPrivateRefusal {
			refused = append(refused, line)
		} else {
			allowed = append(allowed, line)
		}
		if p.namedByPolicy && !isPrivateRefusal {
			t.Errorf("the policy names this and it was not refused as private: %s", line)
		}
	}
	sort.Strings(allowed)
	t.Logf("refused as private (%d):\n  %s", len(refused), strings.Join(refused, "\n  "))
	t.Logf("NOT refused as private (%d):\n  %s", len(allowed), strings.Join(allowed, "\n  "))
}

// TestJ8HostFormsCannotDisguiseADestination records how the policy reads hosts
// that are written to look like something else. The destination that matters is
// the one the fetch will connect to.
func TestJ8HostFormsCannotDisguiseADestination(t *testing.T) {
	policy := media.NewPolicy(config.RemoteMediaConfig{
		DefaultAction:        media.ActionReview,
		BlockedSchemes:       []string{"file", "data", "javascript", "ftp"},
		AllowedDomains:       []string{"images.example.org", "*.cdn.example.org"},
		BlockedDomains:       []string{"blocked.example.org"},
		AllowPrivateNetworks: false,
	})
	cases := []struct {
		url  string
		what string
		want string // the action the destination deserves
		why  string
		// finding names the J8 finding a difference belongs to. A difference
		// with a finding is recorded, not failed: J8 records and defers, and a
		// red test in the repository would misstate the item rather than
		// describe the gap.
		finding string
	}{
		{"https://images.example.org/a.png", "the allowed host itself", media.ActionAllow, "listed", ""},
		{"https://images.example.org@evil.test/a.png", "allowed host as userinfo", media.ActionReview, "connects to evil.test", ""},
		{"https://sub.cdn.example.org/a.png", "wildcard subdomain", media.ActionAllow, "listed", ""},
		{"https://cdn.example.org/a.png", "wildcard apex", media.ActionReview, "the apex is not the wildcard", ""},
		{"https://notcdn.example.org/a.png", "suffix that is not a subdomain", media.ActionReview, "must not match *.cdn.example.org", ""},
		{"https://IMAGES.EXAMPLE.ORG/a.png", "uppercase host", media.ActionAllow, "hosts are case-insensitive", ""},
		{"https://images.example.org./a.png", "trailing dot", media.ActionAllow, "the same host in DNS", "J8-F1"},
		{"https://blocked.example.org/a.png", "blocked host", media.ActionBlock, "listed as blocked", ""},
		{"https://blocked.example.org./a.png", "blocked host, trailing dot", media.ActionBlock, "the same host in DNS", "J8-F1"},
	}
	for _, c := range cases {
		action, reason := policy.Evaluate(c.url)
		status := "as expected"
		if action != c.want {
			status = fmt.Sprintf("DIFFERS: wanted %s because %s", c.want, c.why)
			if c.finding != "" {
				status = fmt.Sprintf("%s [%s]", status, c.finding)
			}
		}
		t.Logf("%-46s %-34s -> %-6s %s (%s)", c.url, c.what, action, status, reason)
		// A difference this review has already written up as a finding is
		// recorded here and deferred; anything else that widens access fails,
		// because it would be a gap nobody has decided about.
		if c.finding != "" {
			continue
		}
		if c.want != media.ActionAllow && action == media.ActionAllow {
			t.Errorf("%s (%s) was allowed; %s", c.url, c.what, c.why)
		}
		if c.want == media.ActionBlock && action != media.ActionBlock {
			t.Errorf("%s (%s) was not blocked; %s", c.url, c.what, c.why)
		}
	}
}

// TestJ8ScanBodyFindsWhatAPreviewWouldFetch checks that the scanner sees the
// remote references a renderer would load, since a URL the scan misses is never
// evaluated by any later stage.
func TestJ8ScanBodyFindsWhatAPreviewWouldFetch(t *testing.T) {
	body := strings.Join([]string{
		"![markdown image](https://images.example.org/a.png)",
		`<img src="https://images.example.org/b.png">`,
		`<img   SRC = 'https://images.example.org/c.png' >`,
		`<IMG src="https://images.example.org/d.png"/>`,
		"[a plain link to a png](https://images.example.org/e.png)",
		"![data uri](data:image/png;base64,AAAA)",
		"![file uri](file:///etc/passwd)",
		`<video src="https://images.example.org/f.mp4"></video>`,
		`<img src=https://images.example.org/unquoted.png>`,
		"<img\n  src=\"https://images.example.org/newline.png\">",
	}, "\n\n")

	found := map[string]bool{}
	for _, decision := range reviewPolicy().ScanBody(body) {
		found[decision.URL] = true
	}
	seen := make([]string, 0, len(found))
	for url := range found {
		seen = append(seen, url)
	}
	sort.Strings(seen)
	t.Logf("scanned %d references:\n  %s", len(seen), strings.Join(seen, "\n  "))

	for _, required := range []string{
		"https://images.example.org/a.png",
		"https://images.example.org/b.png",
		"https://images.example.org/e.png",
		"data:image/png;base64,AAAA",
		"file:///etc/passwd",
	} {
		if !found[required] {
			t.Errorf("the scan missed %s, so no later stage evaluates it", required)
		}
	}
	for _, reported := range []string{
		"https://images.example.org/c.png",
		"https://images.example.org/d.png",
		"https://images.example.org/f.mp4",
		"https://images.example.org/unquoted.png",
		"https://images.example.org/newline.png",
	} {
		if !found[reported] {
			t.Logf("NOT scanned: %s", reported)
		}
	}
}
