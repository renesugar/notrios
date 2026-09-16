// Command docexamples generates the published command-line examples for a
// document from a tracked example set, and keeps the execution registry's hash
// in step with what it wrote.
//
// # Why the example is generated rather than written in the page
//
// A command line in documentation is a promise, and the whole of v1.0 J13
// exists because 54 of them had never been kept. Executing a fence where it
// sits -- which internal/docexec already does -- proves the promise on the day
// somebody registers it. What it cannot do is stop the text and the intent
// drifting apart: the fence is the source, so "these three keys must be set
// together" lives only in the prose beside it, and a fourth key added to the
// fence changes what the example asserts with nothing to notice.
//
// So the tracked set names the *intent* -- the keys that must be set together,
// and the command that checks them -- and this renders the fence from it. The
// example then sets exactly the keys the source names by construction rather
// than by inspection, and internal/docexec still checks the product's real
// behaviour, which is the part no generator can fake.
//
// # Why it is not part of cmd/docgen
//
// docgen renders Go doc comments into documentation slots. This needs a
// document's example set, the execution registry, and the shape of the fixture
// those examples run in. The two share a destination and nothing else, and
// J13-C settled it by finding that an example's needs come from the fixture --
// cmd/docgen has no access to one.
//
//	go run ./cmd/docexamples            # check the pages and the registry agree
//	go run ./cmd/docexamples --write    # rewrite both
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const Schema = "notrios.docexamples.v1"

// Sets is every tracked example set, in the order they are rendered. One file
// per document rather than one file for all of them: a single file every page
// depends on is a merge conflict with a schedule, and J13's plan says so.
var Sets = []string{"docs/docexamples/configuration.json"}

// Setting is one configuration key an example sets.
//
// Exactly one of Value and List carries the value. A list is separate rather
// than encoded in a string because the renderer has to emit YAML for it and
// because `config show` does not report list membership -- a difference the
// example's own record should make visible.
type Setting struct {
	Key   string   `json:"key"`
	Value string   `json:"value,omitempty"`
	List  []string `json:"list,omitempty"`
}

// Example is one use case: what somebody is trying to do, the keys it takes,
// and the command that shows it worked.
type Example struct {
	Section  string    `json:"section"`
	Ordinal  int       `json:"ordinal"`
	UseCase  string    `json:"use_case"`
	File     string    `json:"file,omitempty"`
	Settings []Setting `json:"settings"`
	// Verify is the command that proves the settings took effect. Empty means
	// no command can, and Unverifiable must then say why -- a published example
	// with no check and no explanation is the defect this item removes.
	Verify        string `json:"verify"`
	Postcondition string `json:"postcondition,omitempty"`
	Unverifiable  string `json:"unverifiable,omitempty"`
}

// ID is the identifier internal/docaudit gives this example's fence, derived
// the same way: document stem, section slug, ordinal within the section.
func (e Example) ID(document string) string {
	stem := strings.TrimSuffix(strings.TrimPrefix(document, "docs/"), ".md")
	return fmt.Sprintf("%s-%s-example-%d", strings.ReplaceAll(stem, "/", "-"), e.Section, e.Ordinal)
}

type Set struct {
	Schema   string    `json:"schema"`
	Document string    `json:"document"`
	Note     string    `json:"note"`
	Examples []Example `json:"examples"`
}

// Render produces the fenced block exactly as it appears in the document.
//
// An example with a verification command writes its configuration with a
// heredoc and then runs the command, because that is how a person writes a
// configuration file -- J13-C put `cat` on the fixture's PATH rather than
// contort the published example into `printf` calls to suit the harness. An
// example with no command renders as a bare yaml fragment, since a bash fence
// with nothing to run would be a lie about what the reader can do.
func (e Example) Render() (string, error) {
	settings, err := e.renderSettings()
	if err != nil {
		return "", err
	}
	if e.Verify == "" {
		return "```yaml\n" + settings + "```", nil
	}
	if e.File == "" {
		return "", fmt.Errorf("%s: a verified example needs a file to write", e.UseCase)
	}
	return "```bash\ncat > " + e.File + " <<'YAML'\n" + settings + "YAML\n" + e.Verify + "\n```", nil
}

// renderSettings groups dotted keys under their section, in the order given.
//
// Order is the author's, not sorted: `default_action` before `allowed_domains`
// reads as "the rule, then the exception", and sorting would reverse that for
// no gain.
func (e Example) renderSettings() (string, error) {
	if len(e.Settings) == 0 {
		return "", fmt.Errorf("%s: an example that sets nothing", e.UseCase)
	}
	var out strings.Builder
	section, group := "", ""
	for _, setting := range e.Settings {
		parts := strings.Split(setting.Key, ".")
		switch {
		case len(parts) < 2:
			return "", fmt.Errorf("%s: %q is not a sectioned key", e.UseCase, setting.Key)
		case len(parts) > 3:
			return "", fmt.Errorf("%s: %q nests deeper than a published example should",
				e.UseCase, setting.Key)
		}
		// Two levels is every section; three is a section's surface block, such
		// as security.remote_media.<key> (J28), and nothing goes deeper.
		head, leaf, indent := parts[0], parts[len(parts)-1], "  "
		if parts[0] != section {
			out.WriteString(head + ":\n")
			section, group = head, ""
		}
		if len(parts) == 3 {
			if parts[1] != group {
				out.WriteString("  " + parts[1] + ":\n")
				group = parts[1]
			}
			indent = "    "
		} else {
			group = ""
		}
		switch {
		case len(setting.List) > 0 && setting.Value != "":
			return "", fmt.Errorf("%s: %q has both a value and a list", e.UseCase, setting.Key)
		case len(setting.List) > 0:
			out.WriteString(indent + leaf + ":\n")
			for _, member := range setting.List {
				out.WriteString(indent + "  - " + member + "\n")
			}
		case setting.Value == "":
			return "", fmt.Errorf("%s: %q has no value", e.UseCase, setting.Key)
		default:
			out.WriteString(indent + leaf + ": " + setting.Value + "\n")
		}
	}
	return out.String(), nil
}

func load(root, path string) (Set, error) {
	var set Set
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return set, err
	}
	if err := json.Unmarshal(raw, &set); err != nil {
		return set, fmt.Errorf("%s: %w", path, err)
	}
	if set.Schema != Schema {
		return set, fmt.Errorf("%s: schema %q, want %q", path, set.Schema, Schema)
	}
	if set.Document == "" {
		return set, fmt.Errorf("%s: names no document", path)
	}
	seen := map[string]bool{}
	for _, example := range set.Examples {
		id := example.ID(set.Document)
		if seen[id] {
			return set, fmt.Errorf("%s: two examples claim %s", path, id)
		}
		seen[id] = true
		if example.Verify == "" && example.Unverifiable == "" {
			return set, fmt.Errorf("%s: %s has no verification command and does not say why",
				path, id)
		}
		if example.Verify != "" && example.Postcondition == "" {
			return set, fmt.Errorf("%s: %s runs a command and declares no postcondition", path, id)
		}
	}
	return set, nil
}

func markers(id string) (string, string) {
	return "<!-- notrios:generated:example:" + id + ":begin -->",
		"<!-- notrios:generated:example:" + id + ":end -->"
}

// renderDocument replaces each marked block with its example's rendering.
func renderDocument(root string, set Set) (string, map[string]string, error) {
	path := filepath.Join(root, set.Document)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	body := string(raw)
	hashes := map[string]string{}
	for _, example := range set.Examples {
		id := example.ID(set.Document)
		rendered, err := example.Render()
		if err != nil {
			return "", nil, err
		}
		begin, finish := markers(id)
		block := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(begin) + `.*?` + regexp.QuoteMeta(finish))
		if !block.MatchString(body) {
			return "", nil, fmt.Errorf("%s has no markers for %s", set.Document, id)
		}
		body = block.ReplaceAllLiteralString(body, begin+"\n"+rendered+"\n"+finish)

		// The hash internal/docaudit will compute for this fence: the body
		// between the fence lines, which is what the registry pins.
		inner := strings.TrimSuffix(strings.SplitN(rendered, "\n", 2)[1], "\n```")
		digest := sha256.Sum256([]byte(strings.TrimSuffix(inner, "\n")))
		hashes[id] = hex.EncodeToString(digest[:])
	}
	return body, hashes, nil
}

// updateRegistry writes the rendered examples' hashes into the execution
// registry.
//
// This is the half that was done by hand twice while J13-C was written, both
// times correctly and both times one keystroke from being wrong. A generated
// fence whose registry hash is stale fails internal/docaudit's gate, so the
// tool that changes the fence is the tool that should move the hash.
// updateRegistry writes the rendered examples' hashes into the execution
// registry, by editing the text rather than re-encoding the document.
//
// This is the half that was done by hand twice while J13-C was written, both
// times correctly and both times one keystroke from being wrong. A generated
// fence whose registry hash is stale fails internal/docaudit's gate, so the
// tool that changes the fence is the tool that should move the hash.
//
// The edit is textual on purpose. Decoding to a map and re-encoding sorts every
// object's keys, which would rewrite all 169 entries to change five hashes --
// a diff nobody can review, hiding the change it was made for.
func updateRegistry(root string, hashes map[string]string) (string, []string, error) {
	path := filepath.Join(root, "docs/docaudit/registry.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	body := string(raw)

	missing := []string{}
	for _, id := range sortedKeys(hashes) {
		// The first sha256 after this id, which is the one belonging to it:
		// every entry carries exactly one, and the id precedes it.
		pattern := regexp.MustCompile(`("id": "` + regexp.QuoteMeta(id) +
			`",(?s:.*?)"sha256": ")[0-9a-f]{64}(")`)
		if !pattern.MatchString(body) {
			missing = append(missing, id)
			continue
		}
		body = pattern.ReplaceAllString(body, "${1}"+hashes[id]+"${2}")
	}
	return body, missing, nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func main() {
	root := flag.String("root", ".", "repository root")
	write := flag.Bool("write", false, "rewrite the documents and the registry")
	flag.Parse()

	total := 0
	hashes := map[string]string{}
	for _, source := range Sets {
		set, err := load(*root, source)
		if err != nil {
			fail(err)
		}
		body, documentHashes, err := renderDocument(*root, set)
		if err != nil {
			fail(err)
		}
		for id, digest := range documentHashes {
			hashes[id] = digest
		}
		total += len(set.Examples)

		path := filepath.Join(*root, set.Document)
		current, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		if string(current) != body {
			if !*write {
				fmt.Fprintf(os.Stderr, "docexamples: %s does not match %s; "+
					"run `go run ./cmd/docexamples --write`\n", set.Document, source)
				os.Exit(1)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				fail(err)
			}
		}
	}

	registry, missing, err := updateRegistry(*root, hashes)
	if err != nil {
		fail(err)
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "docexamples: generated but not in docs/docaudit/registry.json: %s\n"+
			"an example nothing registers is an example nothing runs.\n", strings.Join(missing, ", "))
		os.Exit(1)
	}
	registryPath := filepath.Join(*root, "docs/docaudit/registry.json")
	current, err := os.ReadFile(registryPath)
	if err != nil {
		fail(err)
	}
	if string(current) != registry {
		if !*write {
			fmt.Fprintln(os.Stderr, "docexamples: the registry's hashes are stale; "+
				"run `go run ./cmd/docexamples --write`")
			os.Exit(1)
		}
		if err := os.WriteFile(registryPath, []byte(registry), 0o644); err != nil {
			fail(err)
		}
	}

	if *write {
		fmt.Printf("generated %d examples across %d document(s), registry hashes updated\n",
			total, len(Sets))
		return
	}
	fmt.Printf("%d examples across %d document(s) match their tracked set\n", total, len(Sets))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "docexamples:", err)
	os.Exit(1)
}
