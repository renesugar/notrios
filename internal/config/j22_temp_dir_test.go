package config

import (
	"os"
	"path/filepath"
	"testing"
)

// J22: data.temp_dir is an instance's own temp directory. Unstated, it sits
// under the instance's own cache directory, so a profile config written before
// the key existed still gets one of its own.

func j22WriteConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJ22TempDirIsDerivedFromTheInstancesOwnCacheDir(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "profiles", "p1", "cache")
	cfg, origins, err := LoadWithOrigins(j22WriteConfig(t, "data:\n  cache_dir: \""+cache+"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cache, "tmp"); cfg.Data.TempDir != want {
		t.Fatalf("unstated temp_dir = %q, want %q under the instance's own cache_dir", cfg.Data.TempDir, want)
	}
	if origins["data.temp_dir"] != OriginResolved {
		t.Fatalf("a derived temp_dir must report origin %q, got %q", OriginResolved, origins["data.temp_dir"])
	}

	stated := filepath.Join(dir, "elsewhere")
	cfg, origins, err = LoadWithOrigins(j22WriteConfig(t, "data:\n  cache_dir: \""+cache+"\"\n  temp_dir: \""+stated+"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.TempDir != stated || origins["data.temp_dir"] != OriginFile {
		t.Fatalf("a stated temp_dir must be used as written: %q (%s)", cfg.Data.TempDir, origins["data.temp_dir"])
	}
}

func TestJ22OneDataDirectoryHoldsTheTempDirToo(t *testing.T) {
	library := filepath.Join(t.TempDir(), "library")
	cfg, err := Load(j22WriteConfig(t, "data:\n  directory: \""+library+"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(library, "tmp"); cfg.Data.TempDir != want {
		t.Fatalf("with one data directory, temp_dir = %q, want %q", cfg.Data.TempDir, want)
	}
	if err := EnsureDirectories(cfg); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(cfg.Data.TempDir); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("EnsureDirectories must create temp_dir owner-only: %v %v", info, err)
	}

	moved := filepath.Join(t.TempDir(), "moved")
	UseDataDirectory(&cfg, moved, nil)
	if want := filepath.Join(moved, "tmp"); cfg.Data.TempDir != want {
		t.Fatalf("UseDataDirectory must move temp_dir with the rest: %q, want %q", cfg.Data.TempDir, want)
	}
}
