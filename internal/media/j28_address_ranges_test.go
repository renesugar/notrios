package media

import (
	"net"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

// J28: one refused-address set, applied by one helper at the static URL check
// and at the connect-time dial check.

// j28Measured is the 44-address measurement recorded in PLAN.md J28 before
// the change, with whether the default set refuses each one. Only
// ::ffff:8.8.8.8 and ORCHIDv2 stay reachable.
var j28Measured = []struct {
	addr    string
	refused bool
}{
	{"127.0.0.1", true}, {"10.1.2.3", true}, {"172.16.0.1", true}, {"192.168.1.1", true},
	{"169.254.169.254", true}, {"0.0.0.0", true}, {"0.1.2.3", true}, {"100.64.0.1", true},
	{"192.0.0.1", true}, {"192.0.2.1", true}, {"198.51.100.1", true}, {"203.0.113.1", true},
	{"198.18.0.1", true}, {"224.0.0.2", true}, {"239.1.1.1", true}, {"233.252.0.1", true},
	{"240.0.0.1", true}, {"255.255.255.255", true}, {"192.88.99.1", true},
	{"::1", true}, {"::", true}, {"fe80::1", true}, {"fc00::1", true}, {"fd00::1", true},
	{"ff02::1", true}, {"ff05::1", true}, {"ff0e::1", true},
	{"::ffff:127.0.0.1", true}, {"::ffff:10.0.0.1", true}, {"::ffff:100.64.0.1", true}, {"::ffff:8.8.8.8", false},
	{"64:ff9b::7f00:1", true}, {"64:ff9b:1::a00:1", true}, {"::7f00:1", true}, {"2002:7f00:1::1", true},
	{"2001:0:4136:e378:8000:63bf:80ff:fffe", true}, {"100::1", true}, {"2001:2::1", true},
	{"2001:db8::1", true}, {"3fff::1", true}, {"fec0::1", true}, {"5f00::1", true},
	{"2001:10::1", true}, {"2001:20::1", false},
}

// urlFor writes an address literal as a URL host.
func urlFor(addr string) string {
	if strings.Contains(addr, ":") {
		return "https://[" + addr + "]/photo.png"
	}
	return "https://" + addr + "/photo.png"
}

func j28Fetcher(t *testing.T, cfg config.RemoteMediaConfig, opts ...Option) *Fetcher {
	t.Helper()
	cfg.QuarantineDir = t.TempDir()
	fetcher, err := NewFetcher(cfg, nil, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return fetcher
}

// bothChecks reports the static verdict and the dial verdict for one address.
func bothChecks(policy *Policy, fetcher *Fetcher, addr string) (staticRefused bool, staticReason string, dialErr error) {
	action, reason := policy.Evaluate(urlFor(addr))
	return action == ActionBlock, reason, fetcher.checkDialAddress(net.JoinHostPort(addr, "443"))
}

func TestJ28BTheDefaultSetAtBothChecks(t *testing.T) {
	cfg := config.Default().RemoteMedia
	policy := NewPolicy(cfg)
	fetcher := j28Fetcher(t, cfg)
	if len(j28Measured) != 44 {
		t.Fatalf("the measurement has %d addresses, want 44", len(j28Measured))
	}
	for _, c := range j28Measured {
		staticRefused, reason, dialErr := bothChecks(policy, fetcher, c.addr)
		if staticRefused != c.refused {
			t.Errorf("static check: %s refused=%t (%s); want %t", c.addr, staticRefused, reason, c.refused)
		}
		if (dialErr != nil) != c.refused {
			t.Errorf("dial check: %s err=%v; want refused=%t", c.addr, dialErr, c.refused)
		}
		if c.refused && (!strings.Contains(reason, "is in refused range") || !strings.Contains(dialErr.Error(), "is in refused range")) {
			t.Errorf("%s: both refusals must name the range: %q / %v", c.addr, reason, dialErr)
		}
	}
}

func TestJ28BBothChecksAgreeOnTheSameReason(t *testing.T) {
	cfg := config.Default().RemoteMedia
	policy, fetcher := NewPolicy(cfg), j28Fetcher(t, cfg)
	for _, addr := range []string{"64:ff9b::7f00:1", "::ffff:169.254.169.254", "100.64.0.1"} {
		_, reason, dialErr := bothChecks(policy, fetcher, addr)
		if dialErr == nil || dialErr.Error() != "resolved "+reason {
			t.Errorf("%s: static %q, dial %v; one helper must give one reason", addr, reason, dialErr)
		}
	}
}

func TestJ28BEmbeddedPublicAddressesStayReachable(t *testing.T) {
	cfg := config.Default().RemoteMedia
	policy, fetcher := NewPolicy(cfg), j28Fetcher(t, cfg)
	for _, addr := range []string{"64:ff9b::808:808", "::ffff:8.8.8.8", "8.8.8.8", "2606:4700::1111"} {
		if staticRefused, reason, dialErr := bothChecks(policy, fetcher, addr); staticRefused || dialErr != nil {
			t.Errorf("%s must stay reachable (D4): static %s, dial %v", addr, reason, dialErr)
		}
	}
}

func TestJ28BAStatedSetAndItsExceptionsApplyAtBothChecks(t *testing.T) {
	cfg := config.Default().RemoteMedia
	security := config.RemoteMediaSecurityConfig{
		RefusedAddressRanges:   []string{"192.168.0.0/16", "127.0.0.0/8", "203.0.113.0/24"},
		PermittedAddressRanges: []string{"192.168.1.0/24"},
	}
	policy := NewPolicy(cfg, WithAddressRanges(security))
	fetcher := j28Fetcher(t, cfg, WithAddressRanges(security))
	cases := []struct {
		addr    string
		refused bool
	}{
		{"192.168.1.5", false},        // the exception
		{"::ffff:192.168.1.5", false}, // its mapped form (D9)
		{"64:ff9b::c0a8:105", false},  // its NAT64 form (D9)
		{"192.168.2.5", true},         // outside the exception
		{"127.0.0.1", true},           // stated
		{"10.0.0.1", false},           // the stated set replaced the default (D2)
		{"100.64.0.1", false},
		{"203.0.113.9", true},
	}
	for _, c := range cases {
		staticRefused, reason, dialErr := bothChecks(policy, fetcher, c.addr)
		if staticRefused != c.refused || (dialErr != nil) != c.refused {
			t.Errorf("%s: static refused=%t (%s), dial err=%v; want refused=%t", c.addr, staticRefused, reason, dialErr, c.refused)
		}
	}
}

func TestJ28BAllowPrivateNetworksSwitchesBothChecksOff(t *testing.T) {
	cfg := config.Default().RemoteMedia
	cfg.AllowPrivateNetworks = true
	policy, fetcher := NewPolicy(cfg), j28Fetcher(t, cfg)
	for _, addr := range []string{"127.0.0.1", "::1", "64:ff9b::7f00:1", "100.64.0.1"} {
		if staticRefused, reason, dialErr := bothChecks(policy, fetcher, addr); staticRefused || dialErr != nil {
			t.Errorf("%s with allow_private_networks=true: static %s, dial %v (D5)", addr, reason, dialErr)
		}
	}
	if action, _ := policy.Evaluate("http://localhost/a.png"); action == ActionBlock {
		t.Error("allow_private_networks=true switches off the localhost-name check too, as before J28")
	}
}

func TestJ28BRangesThatDoNotParseRefuseEverything(t *testing.T) {
	cfg := config.Default().RemoteMedia
	cfg.AllowedDomains = []string{"images.example.org"}
	security := config.RemoteMediaSecurityConfig{PermittedAddressRanges: []string{"127.0.0.1"}}
	policy := NewPolicy(cfg, WithAddressRanges(security))
	if action, reason := policy.Evaluate("https://images.example.org/a.png"); action != ActionBlock || !strings.Contains(reason, "address ranges are invalid") {
		t.Errorf("Evaluate = %s (%s); a policy whose ranges did not parse must refuse every URL", action, reason)
	}
	cfg.QuarantineDir = t.TempDir()
	if _, err := NewFetcher(cfg, nil, WithAddressRanges(security)); err == nil {
		t.Error("NewFetcher must refuse ranges that do not parse")
	}
}

func TestJ28BLocalhostNamesStayRefused(t *testing.T) {
	policy := NewPolicy(config.Default().RemoteMedia, WithAddressRanges(config.RemoteMediaSecurityConfig{RefusedAddressRanges: []string{}}))
	for _, raw := range []string{"http://localhost/a.png", "http://media.localhost./a.png"} {
		if action, reason := policy.Evaluate(raw); action != ActionBlock || !strings.HasPrefix(reason, "localhost name ") {
			t.Errorf("Evaluate(%q) = %s (%s); localhost names are refused whatever the address set says", raw, action, reason)
		}
	}
	if action, _ := policy.Evaluate("http://127.0.0.1/a.png"); action == ActionBlock {
		t.Error("a stated empty set refuses no address literal")
	}
}
