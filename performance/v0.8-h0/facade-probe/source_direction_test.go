package facadeprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep the public facade probe free of transport and storage implementation
// imports in its API-facing file. This catches accidental HTTP/SQL leakage.
func TestFacadeSourceDirection(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("facade.go"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, forbidden := range []string{"net/http", "httpapi", "database/sql", "filepath", "os.Open"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("facade public surface contains %q", forbidden)
		}
	}
}
