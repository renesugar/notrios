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

// The markers around the generated root-document list in the atlas.
const (
	AtlasBegin = "<!-- notrios:generated:atlas:documents:begin -->"
	AtlasEnd   = "<!-- notrios:generated:atlas:documents:end -->"
)

// AtlasLines renders the root documents for the atlas.
//
// The atlas listed them by hand and the inventory listed them again, which is
// two lists of the same thing: the hand-written one had drifted, missing
// several documents and describing the plan's progress in a document whose
// subject is where code lives.
func (r Registry) AtlasLines() []string {
	lines := []string{
		fmt.Sprintf("Every root document, from `%s`. What each is the home for, and the rules for",
			RegistryPath),
		"keeping it current, are in `AGENTS.md`.",
		"",
	}
	for _, doc := range r.Documents {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", doc.Path, doc.Home))
	}
	lines = append(lines, "", "Feature contracts, which describe one capability rather than the project:", "")
	for _, exempt := range r.Exempt {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", exempt.Path, exempt.Reason))
	}
	return lines
}

// RenderAtlas returns the atlas with its generated root-document list replaced.
func (r Registry) RenderAtlas(root string) (string, error) {
	path := filepath.Join(root, r.Atlas.Document)
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	body := string(contents)
	block := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(AtlasBegin) + `.*?` + regexp.QuoteMeta(AtlasEnd))
	if !block.MatchString(body) {
		return "", fmt.Errorf("%s has no generated markers for the root-document list", r.Atlas.Document)
	}
	lines := strings.Join(r.AtlasLines(), "\n")
	return block.ReplaceAllLiteralString(body, AtlasBegin+"\n"+lines+"\n"+AtlasEnd), nil
}

// CheckAtlasBlock reports whether the atlas's generated list is current.
func CheckAtlasBlock(root string) []string {
	registry, err := Load(root)
	if err != nil {
		return []string{err.Error()}
	}
	rendered, err := registry.RenderAtlas(root)
	if err != nil {
		return []string{err.Error()}
	}
	current, err := os.ReadFile(filepath.Join(root, registry.Atlas.Document))
	if err != nil {
		return []string{err.Error()}
	}
	if string(current) != rendered {
		return []string{"the root-document list in " + registry.Atlas.Document +
			" is stale; run `go run ./cmd/docrules --write`"}
	}
	return nil
}

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
