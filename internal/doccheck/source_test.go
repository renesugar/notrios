package doccheck

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/docaudit"
)

func TestResolveGoSourceSliceWithholdsClaimAndBoundsCallees(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "internal", "fixture")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package fixture

// Root claims something that the blind explanation must not see.
//notrios:doc user root-claim
func Root() int { return helper() }

// helper rationale is also source-adjacent prose.
func helper() int { return 27 }
`
	if err := os.WriteFile(filepath.Join(directory, "fixture.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	anchor := "go:github.com/renesugar/notrios/internal/fixture#Root"
	slice, err := ResolveSourceSlice(context.Background(), root, anchor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if slice.Root != anchor || len(slice.DirectCallees) != 1 || !strings.HasSuffix(slice.DirectCallees[0], "#helper") {
		t.Fatalf("unexpected slice identity: %+v", slice)
	}
	if !strings.Contains(slice.Source, "func Root() int") || !strings.Contains(slice.Source, "func helper() int") {
		t.Fatalf("declarations missing:\n%s", slice.Source)
	}
	if strings.Contains(slice.Source, "claims something") || strings.Contains(slice.Source, "notrios:doc") || strings.Contains(slice.Source, "helper rationale") {
		t.Fatalf("source-adjacent prose leaked into blind source:\n%s", slice.Source)
	}
}

func TestRepositoryUserSourceSlicesResolveWithinModelBudget(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
	fragments, err := docaudit.ScanFragments(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range fragments {
		if fragment.Audience != "user" {
			continue
		}
		slice, err := ResolveSourceSlice(context.Background(), root, fragment.Anchor, nil)
		if err != nil {
			t.Errorf("%s: %v", fragment.ID, err)
			continue
		}
		// Qwen's 8192-token context needs room for instructions and output.
		// A conservative byte ceiling prevents a syntactically valid source
		// slice from being silently truncated by llama.cpp.
		if len(slice.Source) > 24<<10 {
			t.Errorf("%s source slice = %d bytes, exceeds 24 KiB model budget", fragment.ID, len(slice.Source))
		}
		t.Logf("%s: %d bytes", fragment.ID, len(slice.Source))
	}
}

func TestResolveGoSourceSliceRejectsCrossPackageExplicitCallee(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "internal", "fixture")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "fixture.go"), []byte("package fixture\nfunc Root() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveSourceSlice(context.Background(), root,
		"go:github.com/renesugar/notrios/internal/fixture#Root",
		[]string{"go:github.com/renesugar/notrios/internal/store#Store"})
	if err == nil || !strings.Contains(err.Error(), "outside root package") {
		t.Fatalf("cross-package callee error = %v", err)
	}
}
