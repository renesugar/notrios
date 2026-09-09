package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/importers/joplinraw"
)

func TestJ2JoplinDryRunWritesConfigOnlyWhenRequested(t *testing.T) {
	dir := t.TempDir()
	config := joplinraw.ImportConfig{Version: 1, Renames: map[string]string{"Work": "Work (Joplin)"}}
	if target, err := writeJoplinDryRunConfig(config, ""); err != nil || target != "" {
		t.Fatalf("omitted target wrote config: target=%q err=%v", target, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("omitted target changed source: entries=%v err=%v", entries, err)
	}

	target := filepath.Join(dir, "explicit-config.json")
	written, err := writeJoplinDryRunConfig(config, target)
	if err != nil || written != target {
		t.Fatalf("explicit target: written=%q err=%v", written, err)
	}
	loaded, err := joplinraw.LoadConfig(target)
	if err != nil || loaded.Renames["Work"] != "Work (Joplin)" {
		t.Fatalf("explicit config was not written correctly: config=%#v err=%v", loaded, err)
	}
}
