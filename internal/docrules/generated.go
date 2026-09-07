package docrules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The markers around the generated inventory table in AGENTS.md.
const (
	Begin  = "<!-- notrios:generated:agents:documents:begin -->"
	Finish = "<!-- notrios:generated:agents:documents:end -->"
)

// AgentsPath is the file the table is written into.
const AgentsPath = "AGENTS.md"

// Render returns AGENTS.md with its inventory table replaced by the registry's.
//
// Rendering and checking are the same code so that "the table is stale" and
// "the table is wrong" cannot become two different answers -- the check is
// simply whether rendering changes anything.
func (r Registry) Render(root string) (string, error) {
	path := filepath.Join(root, AgentsPath)
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	body := string(contents)
	block := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(Begin) + `.*?` + regexp.QuoteMeta(Finish))
	if !block.MatchString(body) {
		return "", fmt.Errorf("%s has no generated markers for the documents table", AgentsPath)
	}
	lines := strings.Join(r.TableLines(), "\n")
	return block.ReplaceAllLiteralString(body, Begin+"\n"+lines+"\n"+Finish), nil
}

// CheckTable reports whether the table in AGENTS.md still matches the registry.
func CheckTable(root string) []string {
	registry, err := Load(root)
	if err != nil {
		return []string{err.Error()}
	}
	rendered, err := registry.Render(root)
	if err != nil {
		return []string{err.Error()}
	}
	current, err := os.ReadFile(filepath.Join(root, AgentsPath))
	if err != nil {
		return []string{err.Error()}
	}
	if string(current) != rendered {
		return []string{"the documents table in AGENTS.md is stale; run `go run ./cmd/docrules --write`"}
	}
	return nil
}
