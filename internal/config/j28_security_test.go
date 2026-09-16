package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/addressrange"
)

// J28: the security block states the ranges remote media refuses.

func j28Write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJ28AbsentBlockMeansTheDefaultSet(t *testing.T) {
	cfg, origins, err := LoadWithOrigins(j28Write(t, "remote_media:\n  default_action: review\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Security.RemoteMedia.RefusedStated() || cfg.Security.RemoteMedia.RefusedAddressRanges != nil {
		t.Fatalf("an absent block must leave the set unstated, got %v", cfg.Security.RemoteMedia.RefusedAddressRanges)
	}
	rules, err := cfg.Security.RemoteMedia.Rules()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rules.Refused(), ",") != strings.Join(addressrange.Default, ",") {
		t.Errorf("an unstated set must resolve to the default, got %v", rules.Refused())
	}
	if _, ok := origins[RefusedAddressRangesKey]; ok {
		t.Error("an unstated set must not be reported as stated in the file")
	}
	if warnings := cfg.RemoteMediaAddressWarnings(); len(warnings) != 0 {
		t.Errorf("the default posture warns about nothing, got %v", warnings)
	}
	if Default().Security.RemoteMedia.RefusedAddressRanges != nil {
		t.Error("the compiled default leaves the set unstated")
	}
}

func TestJ28BlockAndFlowListsBothStateTheSet(t *testing.T) {
	block := `
security:
  remote_media:
    refused_address_ranges:
      - "127.0.0.0/8"
      - 10.0.0.0/8   # unquoted works too
    permitted_address_ranges:
      - "10.1.0.0/16"
retention:
  sync_history_days: 30
`
	flow := `
security:
  remote_media:
    refused_address_ranges: ["127.0.0.0/8", "10.0.0.0/8"]
    permitted_address_ranges: ["10.1.0.0/16"]
retention:
  sync_history_days: 30
`
	for name, body := range map[string]string{"block": block, "flow": flow} {
		cfg, origins, err := LoadWithOrigins(j28Write(t, body))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sec := cfg.Security.RemoteMedia
		if strings.Join(sec.RefusedAddressRanges, ",") != "127.0.0.0/8,10.0.0.0/8" || strings.Join(sec.PermittedAddressRanges, ",") != "10.1.0.0/16" {
			t.Errorf("%s: refused=%v permitted=%v", name, sec.RefusedAddressRanges, sec.PermittedAddressRanges)
		}
		if origins[RefusedAddressRangesKey] != OriginFile || origins[PermittedAddressRangesKey] != OriginFile {
			t.Errorf("%s: both lists must be reported as stated in the file: %v", name, origins)
		}
		if cfg.Retention.SyncHistoryDays != 30 {
			t.Errorf("%s: the section after the block must still load", name)
		}
		warnings := strings.Join(cfg.RemoteMediaAddressWarnings(), "\n")
		if !strings.Contains(warnings, "omits 29 default range(s)") || strings.Contains(warnings, "127.0.0.0/8,") ||
			!strings.Contains(warnings, "100.64.0.0/10") || !strings.Contains(warnings, "lets remote media reach: 10.1.0.0/16") {
			t.Errorf("%s: warnings must name every omitted default range and the exceptions:\n%s", name, warnings)
		}
	}
}

func TestJ28AStatedEmptySetRefusesNothingAndSaysSo(t *testing.T) {
	cfg, err := Load(j28Write(t, "security:\n  remote_media:\n    refused_address_ranges: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Security.RemoteMedia.RefusedStated() || len(cfg.Security.RemoteMedia.RefusedAddressRanges) != 0 {
		t.Fatalf("[] is a stated empty set, got %#v", cfg.Security.RemoteMedia.RefusedAddressRanges)
	}
	if warnings := cfg.RemoteMediaAddressWarnings(); len(warnings) != 1 || !strings.Contains(warnings[0], "omits 31 default range(s)") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestJ28MalformedBlocksFailLoadingByName(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"not an address": {
			"security:\n  remote_media:\n    refused_address_ranges:\n      - \"not-an-address\"\n",
			`"not-an-address" is not a valid address or CIDR range`},
		"host bits": {
			"security:\n  remote_media:\n    permitted_address_ranges: [\"10.0.5.20/8\"]\n",
			`"10.0.5.20/8" has host bits set`},
		"not a list": {
			"security:\n  remote_media:\n    refused_address_ranges: 10.0.0.0/8\n",
			"security.remote_media.refused_address_ranges must be a list"},
		"no value and no items, before another section": {
			"security:\n  remote_media:\n    refused_address_ranges:\nretention:\n  sync_history_days: 30\n",
			"has no value and no items"},
		"no value and no items, at the end of the file": {
			"security:\n  remote_media:\n    permitted_address_ranges:\n",
			"has no value and no items"},
		"no value and no items, before a sibling": {
			"security:\n  remote_media:\n    refused_address_ranges:\n    permitted_address_ranges: []\n",
			"has no value and no items"},
		"loopback exception": {
			"security:\n  remote_media:\n    permitted_address_ranges: [\"127.0.0.1\"]\n",
			"no exception may permit"},
		"loopback exception through NAT64": {
			"security:\n  remote_media:\n    permitted_address_ranges:\n      - 64:ff9b::7f00:1\n",
			"no exception may permit"},
	}
	for name, c := range cases {
		_, err := Load(j28Write(t, c.body))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v; want it to contain %q", name, err, c.want)
		}
	}
}

func TestJ28UnknownKeysAreIgnoredAsElsewhere(t *testing.T) {
	cfg, err := Load(j28Write(t, `
security:
  remote_media:
    refused_adress_ranges: ["10.0.0.0/8"]
    future_key: 3
  sync:
    refused_address_ranges:
      - "10.0.0.0/8"
  flag: true
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Security.RemoteMedia.RefusedStated() {
		t.Error("a misspelled key, or the same key under another surface, must not state remote media's set")
	}
}

func TestJ28AllowPrivateNetworksSaysTheSetIsInactive(t *testing.T) {
	cfg, err := Load(j28Write(t, "remote_media:\n  allow_private_networks: true\nsecurity:\n  remote_media:\n    refused_address_ranges: [\"10.0.0.0/8\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	warnings := cfg.RemoteMediaAddressWarnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not applied") {
		t.Errorf("warnings = %v; a set switched off by allow_private_networks must say so (D5)", warnings)
	}
}

func TestJ28ACodeBuiltInvalidBlockIsReported(t *testing.T) {
	cfg := Default()
	cfg.Security.RemoteMedia.PermittedAddressRanges = []string{"::1"}
	if warnings := cfg.RemoteMediaAddressWarnings(); len(warnings) != 1 || !strings.Contains(warnings[0], "every remote-media URL is refused") {
		t.Errorf("warnings = %v", warnings)
	}
}
