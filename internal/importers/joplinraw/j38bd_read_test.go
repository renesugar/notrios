package joplinraw

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func j38dItem(t *testing.T, dir, id, body string) inventoryItem {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	raw := fmt.Sprintf("%s\n\nid: %s\nparent_id: folder-00\ntitle: %s\ntype_: 1\n", body, id, id)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(raw))
	built := inventoryItem{
		parsedItem:   parsedItem{ID: id, Type: "1"},
		RelativePath: id + ".md",
		Fingerprint:  hex.EncodeToString(sum[:]),
		SizeBytes:    int64(len(raw)),
	}
	built.Path = path
	return built
}

// TestReadInventoryItemReusesItsBuffer covers what a reused read buffer can get
// wrong: a shorter item after a longer one must not see the longer one's tail,
// and every item must parse to itself (v1.0 J38-D).
func TestReadInventoryItemReusesItsBuffer(t *testing.T) {
	dir := t.TempDir()
	items := []inventoryItem{
		j38dItem(t, dir, "note-long", strings.Repeat("a long body with plenty of text\n", 300)),
		j38dItem(t, dir, "note-short", "short"),
		j38dItem(t, dir, "note-medium", strings.Repeat("medium ελληνικά\n", 40)),
		j38dItem(t, dir, "note-tiny", "x"),
	}
	var buffer []byte
	for _, item := range items {
		parsed, kept, err := readInventoryItem(item, buffer)
		if err != nil {
			t.Fatalf("%s: %v", item.ID, err)
		}
		buffer = kept
		if parsed.ID != item.ID {
			t.Fatalf("%s: parsed id %q", item.ID, parsed.ID)
		}
		// The body must be this item's own, with no tail from a longer one.
		if strings.Contains(parsed.Body, "a long body") && item.ID != "note-long" {
			t.Fatalf("%s: read a previous item's bytes: %q", item.ID, parsed.Body)
		}
	}
}

// TestReadInventoryItemRefusesAChangedItem keeps the conflict the fingerprint
// check gives: an item edited between the scan and the read is not imported as
// though it were what the scan saw.
func TestReadInventoryItemRefusesAChangedItem(t *testing.T) {
	dir := t.TempDir()
	item := j38dItem(t, dir, "note-one", "original body")
	if err := os.WriteFile(item.Path, []byte("changed body\n\nid: note-one\ntype_: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readInventoryItem(item, nil); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected a conflict, got %v", err)
	}
}

// TestRetainedItemBufferIsBoundedByTheItemLimit states the cap: a buffer up to
// the size one item may reach is kept, and the budget is that limit.
func TestRetainedItemBufferIsBoundedByTheItemLimit(t *testing.T) {
	if maxRetainedItemBytes != maxRAWItemSize {
		t.Errorf("the retained buffer should be bounded by the item limit: %d against %d",
			maxRetainedItemBytes, maxRAWItemSize)
	}
	dir := t.TempDir()
	item := j38dItem(t, dir, "note-small", "small body")
	_, kept, err := readInventoryItem(item, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cap(kept) == 0 {
		t.Fatal("an ordinary item's buffer should be kept for the next item")
	}
}
