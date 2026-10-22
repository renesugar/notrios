package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// J28: config show reports the effective address sets, their origin, and the
// warnings service start would log; a malformed block fails.

func TestJ28ConfigShowReportsAddressSetsAndWarnings(t *testing.T) {
	binary := buildCLI(t)

	defaults := t.TempDir()
	plantConfig(t, defaults, "remote_media:\n  default_action: review\n")
	result := runCLIIn(t, defaults, binary, "config", "show")
	if result.exitCode != 0 {
		t.Fatalf("config show exited %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "31 ranges (listed below)") || !strings.Contains(result.stdout, "\n  100.64.0.0/10\n") {
		t.Errorf("the default set must be counted and listed:\n%s", result.stdout)
	}
	if strings.Contains(result.stderr, "warning:") {
		t.Errorf("the default posture warns about nothing: %s", result.stderr)
	}

	stated := t.TempDir()
	plantConfig(t, stated, "security:\n  remote_media:\n    refused_address_ranges: [\"127.0.0.0/8\"]\n    permitted_address_ranges: []\n")
	result = runCLIIn(t, stated, binary, "config", "show", "--json")
	if result.exitCode != 0 {
		t.Fatalf("config show --json exited %d: %s", result.exitCode, result.stderr)
	}
	var decoded struct {
		Settings  []struct{ Key, Value, Origin string }
		Addresses struct {
			Refused   []string `json:"refused"`
			Permitted []string `json:"permitted"`
			Active    bool     `json:"active"`
		} `json:"remote_media_addresses"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, result.stdout)
	}
	if strings.Join(decoded.Addresses.Refused, ",") != "127.0.0.0/8" || decoded.Addresses.Permitted == nil || !decoded.Addresses.Active {
		t.Errorf("remote_media_addresses = %+v", decoded.Addresses)
	}
	origins := map[string]string{}
	for _, row := range decoded.Settings {
		origins[row.Key] = row.Origin
	}
	if origins["security.remote_media.refused_address_ranges"] != "file" {
		t.Errorf("origins = %v; the stated set is from the file", origins)
	}
	if len(decoded.Warnings) != 1 || !strings.Contains(decoded.Warnings[0], "omits 30 default range(s)") {
		t.Errorf("warnings = %v", decoded.Warnings)
	}

	malformed := t.TempDir()
	plantConfig(t, malformed, "security:\n  remote_media:\n    refused_address_ranges:\n      - 10.0.0.1/8\n")
	result = runCLIIn(t, malformed, binary, "config", "show")
	if result.exitCode == 0 || !strings.Contains(result.stderr, `"10.0.0.1/8" has host bits set`) {
		t.Errorf("a malformed entry must fail and name itself: exit %d, %s", result.exitCode, result.stderr)
	}
}
