// Package docgen renders source-adjacent documentation fragments into existing
// Markdown sections. Generation is deterministic and preserves bytes outside
// its own marker blocks.
package docgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/docaudit"
)

const Schema = "notrios.docgen.templates.v1"
const BeginPrefix = "<!-- notrios:generated:"
const EndMarker = " -->"

type TemplateFile struct {
	Schema string `json:"schema"`
	Pages  []Page `json:"pages"`
}
type Page struct {
	Path     string    `json:"path"`
	Sections []Section `json:"sections"`
}
type Section struct {
	Slug  string `json:"slug"`
	Slots []Slot `json:"slots"`
}
type Slot struct {
	ID       string `json:"id"`
	Audience string `json:"audience"`
}

// Resolver supplies finite, already-authorized enumeration values for a source
// directive. A nil resolver renders no enumeration rows.
type Resolver func(anchor string) ([]string, error)
type Generator struct {
	Resolver     Resolver
	TemplatePath string
}

func (g Generator) Generate(root, audience string) (map[string][]byte, error) {
	if audience != "user" && audience != "api" {
		return nil, fmt.Errorf("audience must be user or api")
	}
	t, err := g.loadTemplate(root)
	if err != nil {
		return nil, err
	}
	frags, err := docaudit.ScanFragments(root)
	if err != nil {
		return nil, err
	}
	byID := map[string]docaudit.Fragment{}
	for _, f := range frags {
		if _, ok := byID[f.ID]; ok {
			return nil, fmt.Errorf("duplicate fragment %q", f.ID)
		}
		byID[f.ID] = f
	}
	out := map[string][]byte{}
	templated := map[string]bool{}
	for _, p := range t.Pages {
		for _, s := range p.Sections {
			for _, slot := range s.Slots {
				if slot.Audience == audience {
					templated[slot.ID] = true
				}
			}
		}
	}
	for id, f := range byID {
		if f.Audience == audience && !templated[id] {
			return nil, fmt.Errorf("untemplated fragment %q", id)
		}
	}
	for _, p := range t.Pages {
		path, err := safeDocPath(root, p.Path)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, s := range p.Sections {
			rendered, err := g.renderSection(string(b), s, audience, byID)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", p.Path, s.Slug, err)
			}
			b = []byte(rendered)
		}
		out[p.Path] = b
	}
	return out, nil
}

func (g Generator) Write(root, audience string) error {
	generated, err := g.Generate(root, audience)
	if err != nil {
		return err
	}
	for p, b := range generated {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), b, 0644); err != nil {
			return err
		}
	}
	return nil
}
func (g Generator) Check(root, audience string) ([]string, error) {
	generated, err := g.Generate(root, audience)
	if err != nil {
		return nil, err
	}
	var drift []string
	for p, b := range generated {
		actual, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(actual, b) {
			drift = append(drift, p)
		}
	}
	sort.Strings(drift)
	return drift, nil
}

func (g Generator) loadTemplate(root string) (TemplateFile, error) {
	path := g.TemplatePath
	if path == "" {
		path = "docs/docgen/templates.json"
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return TemplateFile{}, err
	}
	var t TemplateFile
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&t); err != nil {
		return t, fmt.Errorf("template: %w", err)
	}
	if err = d.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing JSON values")
		}
		return t, fmt.Errorf("template: %w", err)
	}
	if t.Schema != Schema {
		return t, fmt.Errorf("template schema = %q, want %q", t.Schema, Schema)
	}
	if len(t.Pages) == 0 {
		return t, fmt.Errorf("template has no pages")
	}
	seen := map[string]bool{}
	seenSlots := map[string]bool{}
	for _, p := range t.Pages {
		if _, err := safeDocPath(root, p.Path); err != nil {
			return t, err
		}
		if seen[p.Path] {
			return t, fmt.Errorf("duplicate path %q", p.Path)
		}
		seen[p.Path] = true
		for _, s := range p.Sections {
			if s.Slug == "" {
				return t, fmt.Errorf("empty section slug")
			}
			ids := map[string]bool{}
			for _, slot := range s.Slots {
				if slot.ID == "" || ids[slot.ID] || seenSlots[slot.ID] {
					return t, fmt.Errorf("duplicate or empty slot in %s", s.Slug)
				}
				if slot.Audience != "user" && slot.Audience != "api" {
					return t, fmt.Errorf("invalid slot audience %q", slot.Audience)
				}
				ids[slot.ID] = true
				seenSlots[slot.ID] = true
			}
		}
	}
	return t, nil
}

func safeDocPath(root, p string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(p))
	if !strings.HasPrefix(clean, "docs/") || strings.Contains(clean, "../") || clean == "docs" {
		return "", fmt.Errorf("unsafe document path %q", p)
	}
	return filepath.Join(root, filepath.FromSlash(clean)), nil
}

var headingRE = regexp.MustCompile(`^(#{1,3})\s+(.+?)\s*$`)

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	return b.String()
}

var markerRE = regexp.MustCompile(`<!-- notrios:generated:(user|api):([a-z0-9][a-z0-9-]*):(begin|end) -->`)

type generatedBlock struct {
	start int
	end   int
	bytes string
}

func (g Generator) renderSection(document string, section Section, audience string, frags map[string]docaudit.Fragment) (string, error) {
	lines := strings.SplitAfter(document, "\n")
	start, end := -1, -1
	for i, l := range lines {
		m := headingRE.FindStringSubmatch(strings.TrimSuffix(l, "\n"))
		if m != nil && slug(m[2]) == section.Slug {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("missing section")
	}
	end = len(lines)
	for i := start + 1; i < len(lines); i++ {
		if m := headingRE.FindStringSubmatch(strings.TrimSuffix(lines[i], "\n")); m != nil {
			end = i
			break
		}
	}
	body := strings.Join(lines[start+1:end], "")
	blocks, manual, err := splitGeneratedBlocks(body, section.Slug)
	if err != nil {
		return "", err
	}
	begin := fmt.Sprintf("<!-- notrios:generated:%s:%s:begin -->", audience, section.Slug)
	finish := fmt.Sprintf("<!-- notrios:generated:%s:%s:end -->", audience, section.Slug)
	var block string
	selected := 0
	for _, slot := range section.Slots {
		if slot.Audience == audience {
			selected++
		}
	}
	if selected > 0 {
		var b strings.Builder
		b.WriteString(begin + "\n")
		for _, slot := range section.Slots {
			if slot.Audience != audience {
				continue
			}
			f, ok := frags[slot.ID]
			if !ok {
				return "", fmt.Errorf("missing fragment %q", slot.ID)
			}
			if f.Audience != audience {
				return "", fmt.Errorf("fragment %q has wrong audience", slot.ID)
			}
			b.WriteString("<!-- source: " + f.Anchor + " -->\n" + f.Prose + "\n")
			if f.Enumeration != "" {
				if g.Resolver == nil {
					return "", fmt.Errorf("fragment %q requires enumeration resolver", slot.ID)
				}
				vals, err := g.Resolver(f.Enumeration)
				if err != nil {
					return "", err
				}
				if len(vals) == 0 {
					return "", fmt.Errorf("enumeration %q returned no values", f.Enumeration)
				}
				for _, v := range vals {
					b.WriteString("- " + v + "\n")
				}
			}
		}
		b.WriteString(finish + "\n")
		block = b.String()
	}
	if block == "" {
		delete(blocks, audience)
	} else {
		blocks[audience] = generatedBlock{bytes: block}
	}
	body = blocks["user"].bytes + blocks["api"].bytes + manual
	return strings.Join(append(append(append([]string{}, lines[:start+1]...), body), lines[end:]...), ""), nil
}

func splitGeneratedBlocks(body, section string) (map[string]generatedBlock, string, error) {
	blocks := map[string]generatedBlock{}
	matches := markerRE.FindAllStringSubmatchIndex(body, -1)
	openAudience := ""
	openStart := 0
	var ranges []generatedBlock
	for _, match := range matches {
		audience := body[match[2]:match[3]]
		slugValue := body[match[4]:match[5]]
		kind := body[match[6]:match[7]]
		if slugValue != section {
			return nil, "", fmt.Errorf("generated marker for section %q appears in %q", slugValue, section)
		}
		switch kind {
		case "begin":
			if openAudience != "" || blocks[audience].bytes != "" {
				return nil, "", fmt.Errorf("nested or duplicate markers")
			}
			openAudience, openStart = audience, match[0]
		case "end":
			if openAudience != audience {
				return nil, "", fmt.Errorf("unpaired or crossed markers")
			}
			blockEnd := match[1]
			if blockEnd < len(body) && body[blockEnd] == '\n' {
				blockEnd++
			}
			value := generatedBlock{start: openStart, end: blockEnd, bytes: body[openStart:blockEnd]}
			blocks[audience] = value
			ranges = append(ranges, value)
			openAudience = ""
		}
	}
	if openAudience != "" {
		return nil, "", fmt.Errorf("unpaired markers")
	}
	if len(matches) != len(ranges)*2 {
		return nil, "", fmt.Errorf("unpaired markers")
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	var manual strings.Builder
	position := 0
	for _, value := range ranges {
		manual.WriteString(body[position:value.start])
		position = value.end
	}
	manual.WriteString(body[position:])
	return blocks, manual.String(), nil
}
