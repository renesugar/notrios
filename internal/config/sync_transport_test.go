package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheTransportPolicyRefusesRatherThanWarns is the check that keeps v0.7's
// security posture true: enabling the sync surface is the first thing that can
// put any part of Notrios on a network, and a misconfiguration there is
// invisible until something is already exposed.
func TestTheTransportPolicyRefusesRatherThanWarns(t *testing.T) {
	workspace := t.TempDir()
	certificate := filepath.Join(workspace, "cert.pem")
	key := filepath.Join(workspace, "key.pem")
	for _, path := range []string{certificate, key} {
		if err := os.WriteFile(path, []byte("not a real certificate"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for name, testCase := range map[string]struct {
		listen  string
		enabled bool
		cert    string
		key     string
		refused bool
	}{
		"off is always fine":               {listen: "0.0.0.0:8080", enabled: false},
		"loopback plaintext is allowed":    {listen: "127.0.0.1:8080", enabled: true},
		"localhost plaintext is allowed":   {listen: "localhost:8080", enabled: true},
		"every interface without TLS":      {listen: "0.0.0.0:8080", enabled: true, refused: true},
		"a routable address without TLS":   {listen: "192.168.1.10:8080", enabled: true, refused: true},
		"an empty host binds everywhere":   {listen: ":8080", enabled: true, refused: true},
		"every interface with TLS":         {listen: "0.0.0.0:8443", enabled: true, cert: certificate, key: key},
		"a certificate without its key":    {listen: "0.0.0.0:8443", enabled: true, cert: certificate, refused: true},
		"a key without its certificate":    {listen: "0.0.0.0:8443", enabled: true, key: key, refused: true},
		"TLS material that does not exist": {listen: "0.0.0.0:8443", enabled: true, cert: "/nope/cert.pem", key: "/nope/key.pem", refused: true},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Default()
			cfg.Server.ListenAddr = testCase.listen
			cfg.Sync.REST.Enabled = testCase.enabled
			cfg.Sync.REST.TLSCertFile = testCase.cert
			cfg.Sync.REST.TLSKeyFile = testCase.key
			err := ValidateSyncTransport(cfg)
			if testCase.refused && err == nil {
				t.Fatal("the configuration was accepted")
			}
			if !testCase.refused && err != nil {
				t.Fatalf("the configuration was refused: %v", err)
			}
			if testCase.refused && !strings.Contains(err.Error(), "sync.rest") {
				t.Fatalf("the refusal does not name the setting to change: %v", err)
			}
		})
	}
}

func TestSyncRESTDefaultsAreOffAndStrict(t *testing.T) {
	cfg := Default()
	if cfg.Sync.REST.Enabled {
		t.Fatal("the sync surface defaults to on")
	}
	if !cfg.Sync.REST.RequireTLS {
		t.Fatal("TLS is not required by default")
	}
	if err := ValidateSyncTransport(cfg); err != nil {
		t.Fatalf("the default configuration is invalid: %v", err)
	}
}

func TestSyncRESTSettingsParseFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`sync:
  target: rest
  rest:
    enabled: true
    require_tls: false
    tls_cert_file: /etc/notrios/cert.pem
    tls_key_file: /etc/notrios/key.pem
    max_body_bytes: 4MB
    requests_per_minute: 42
    burst: 7
    failures_per_minute: 3
    key_file: /home/user/.config/notrios/sync-keys.json
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	rest := cfg.Sync.REST
	if !rest.Enabled || rest.RequireTLS {
		t.Fatalf("enabled/require_tls = %t/%t", rest.Enabled, rest.RequireTLS)
	}
	if rest.TLSCertFile != "/etc/notrios/cert.pem" || rest.TLSKeyFile != "/etc/notrios/key.pem" {
		t.Fatalf("TLS paths: %+v", rest)
	}
	if rest.MaxBodyBytes != 4*1000*1000 && rest.MaxBodyBytes != 4<<20 {
		t.Fatalf("max_body_bytes = %d", rest.MaxBodyBytes)
	}
	if rest.RequestsPerMinute != 42 || rest.Burst != 7 || rest.FailuresPerMinute != 3 {
		t.Fatalf("limits: %+v", rest)
	}
	if rest.KeyFile == "" {
		t.Fatal("key_file did not parse")
	}
}

func TestLoopbackDetection(t *testing.T) {
	for address, want := range map[string]bool{
		"127.0.0.1:8080": true,
		"localhost:8080": true,
		"[::1]:8080":     true,
		"0.0.0.0:8080":   false,
		":8080":          false,
		"192.168.1.5:80": false,
	} {
		got, err := IsLoopbackListenAddr(address)
		if err != nil {
			t.Fatalf("IsLoopbackListenAddr(%q): %v", address, err)
		}
		if got != want {
			t.Fatalf("IsLoopbackListenAddr(%q) = %t, want %t", address, got, want)
		}
	}
	if _, err := IsLoopbackListenAddr("not-an-address"); err == nil {
		t.Fatal("an unparsable listen address was accepted")
	}
}
