// Command docjourney validates the G18e GUI journey manifest and all of its
// G18a-format source and test anchors.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/renesugar/notrios/internal/docaudit"
	"github.com/renesugar/notrios/internal/docjourney"
)

func main() {
	root := flag.String("root", ".", "repository root")
	manifestPath := flag.String("manifest", "performance/v0.7-g18e/JOURNEYS.json", "journey manifest path")
	flag.Parse()
	manifest, err := docjourney.LoadAndValidate(*root, *manifestPath)
	if err == nil {
		err = docaudit.ValidateAnchors(*root, manifest.ProductionAnchors(), false, resolveTypeScript)
	}
	if err == nil {
		err = docaudit.ValidateAnchors(*root, manifest.CheckAnchors(), true, resolveTypeScript)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "docjourney:", err)
		os.Exit(1)
	}
	result := map[string]any{
		"schema":             "notrios.docjourney.validation.v1",
		"journeys":           len(manifest.Journeys),
		"production_anchors": len(manifest.ProductionAnchors()),
		"check_anchors":      len(manifest.CheckAnchors()),
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "docjourney:", err)
		os.Exit(1)
	}
}

func resolveTypeScript(root string, anchors []string) error {
	arguments := []string{filepath.Join(root, "scripts/docaudit_ts.mjs"), "--root", root}
	for _, anchor := range anchors {
		arguments = append(arguments, "--anchor", anchor)
	}
	output, err := exec.Command("node", arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("TypeScript compiler-API resolver: %w: %s", err, output)
	}
	return nil
}
