package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

func writeConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// An installed binary must not take its configuration from whatever directory
// it was launched in. That file decides the database path, the listen address,
// the public base URL and the remote-media policy, including whether private
// networks may be fetched.
//
// H3 observed the old behaviour; this is its inverse.
func TestLoadDefaultIgnoresAWorkingDirectoryConfigWhenInstalled(t *testing.T) {
	compiled := config.Default()

	// An isolated environment with no user config: whatever is found must be
	// the compiled default.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	// A directory that is not a checkout, holding a plausible-looking config.
	elsewhere := t.TempDir()
	writeConfig(t, filepath.Join(elsewhere, config.ExampleRelativePath),
		"server:\n  listen_addr: 127.0.0.1:59999\ndata:\n  database_path: /planted/notes.sqlite\n")
	t.Chdir(elsewhere)

	loaded, err := config.LoadDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server.ListenAddr != compiled.Server.ListenAddr {
		t.Errorf("listen address came from the working directory: %q", loaded.Server.ListenAddr)
	}
	if loaded.Data.DatabasePath != compiled.Data.DatabasePath {
		t.Errorf("database path came from the working directory: %q", loaded.Data.DatabasePath)
	}
}

// In a checkout the example is still read: that is the developer convenience
// the old behaviour existed for, kept where it is correct.
func TestLoadDefaultReadsTheCheckoutExampleInSourceMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	checkout := t.TempDir()
	writeConfig(t, filepath.Join(checkout, "go.mod"), "module github.com/renesugar/notrios\n")
	writeConfig(t, filepath.Join(checkout, "PLAN.md"), "# plan\n")
	writeConfig(t, filepath.Join(checkout, "AGENTS.md"), "# agents\n")
	writeConfig(t, filepath.Join(checkout, config.ExampleRelativePath),
		"server:\n  listen_addr: 127.0.0.1:58888\n")
	t.Chdir(checkout)

	loaded, err := config.LoadDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server.ListenAddr != "127.0.0.1:58888" {
		t.Fatalf("the checkout example was not read: %q", loaded.Server.ListenAddr)
	}
}

// The user's own config outranks the checkout example, so a developer who has
// written one gets theirs rather than the sample.
func TestLoadDefaultPrefersTheUserConfigOverTheCheckoutExample(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	writeConfig(t, filepath.Join(configHome, "notrios", "config.yaml"),
		"server:\n  listen_addr: 127.0.0.1:57777\n")

	checkout := t.TempDir()
	writeConfig(t, filepath.Join(checkout, "go.mod"), "module github.com/renesugar/notrios\n")
	writeConfig(t, filepath.Join(checkout, "PLAN.md"), "# plan\n")
	writeConfig(t, filepath.Join(checkout, "AGENTS.md"), "# agents\n")
	writeConfig(t, filepath.Join(checkout, config.ExampleRelativePath),
		"server:\n  listen_addr: 127.0.0.1:58888\n")
	t.Chdir(checkout)

	loaded, err := config.LoadDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server.ListenAddr != "127.0.0.1:57777" {
		t.Fatalf("the user's own config did not win: %q", loaded.Server.ListenAddr)
	}
}
