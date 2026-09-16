package media

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestJ28EveryProductionCallerPassesTheAddressRanges guards the wiring: a
// production call that builds a policy, fetcher or localizer without
// WithAddressRanges would silently apply the default set instead of the one
// the configuration states. Test files and the constructors' own forwarding
// (opts...) are exempt.
func TestJ28EveryProductionCallerPassesTheAddressRanges(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`\b(media\.NewPolicy|media\.NewFetcher|localize\.New|NewPolicy|NewFetcher)\(`)
	checked := 0
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for number, line := range strings.Split(string(body), "\n") {
				trimmed := strings.TrimSpace(line)
				if !call.MatchString(trimmed) || strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "//") {
					continue
				}
				checked++
				if !strings.Contains(trimmed, "WithAddressRanges(") && !strings.Contains(trimmed, "opts...") && !strings.Contains(trimmed, "options...") {
					relative, _ := filepath.Rel(root, path)
					t.Errorf("%s:%d builds remote-media policy without WithAddressRanges: %s", relative, number+1, trimmed)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 6 {
		t.Fatalf("found only %d call sites; the guard is not seeing the code", checked)
	}
}
