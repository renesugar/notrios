// Command docaudit audits repository documentation anchors and claim coverage.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/renesugar/notrios/internal/docaudit"
)

func main() {
	root := flag.String("root", ".", "repository root")
	inventory := flag.String("inventory", "performance/v0.7-g18a/INVENTORY.json", "G18a inventory path")
	registry := flag.String("registry", "docs/docaudit/registry.json", "claim/executable registry path")
	listExamples := flag.Bool("list-executables", false, "print detected executable fences without auditing")
	flag.Parse()
	if *listExamples {
		examples, err := docaudit.DetectExecutableExamples(*root, *inventory)
		finish(examples, err)
		return
	}
	report, err := docaudit.Audit(docaudit.Options{
		Root: *root, InventoryPath: *inventory, RegistryPath: *registry,
		TSResolver: resolveTypeScript,
	})
	finish(report, err)
}

func resolveTypeScript(root string, anchors []string) error {
	sort.Strings(anchors)
	arguments := []string{filepath.Join(root, "scripts/docaudit_ts.mjs"), "--root", root}
	for _, anchor := range anchors {
		arguments = append(arguments, "--anchor", anchor)
	}
	command := exec.Command("node", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("TypeScript compiler-API resolver: %w: %s", err, output)
	}
	return nil
}

func finish(value any, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "docaudit:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, "docaudit:", err)
		os.Exit(1)
	}
}
