// Package docjourney validates the structured GUI journey inventory used by
// the documentation acceptance tests.
package docjourney

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const Schema = "notrios.docjourney.manifest.v1"

type Manifest struct {
	Schema        string        `json:"schema"`
	BrowserPolicy BrowserPolicy `json:"browser_policy"`
	Viewports     []Viewport    `json:"viewports"`
	Journeys      []Journey     `json:"journeys"`
}

type BrowserPolicy struct {
	Preferred string `json:"preferred"`
	Observed  string `json:"observed"`
	Fallback  string `json:"fallback"`
	Reason    string `json:"reason"`
}

type Viewport struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Journey struct {
	ID             string       `json:"id"`
	Path           string       `json:"path"`
	Section        string       `json:"section"`
	Label          string       `json:"label"`
	State          string       `json:"state"`
	UIOwner        string       `json:"ui_owner"`
	ActionOwners   []string     `json:"action_owners"`
	GOwners        []string     `json:"go_owners"`
	CheckAnchor    string       `json:"check_anchor"`
	Case           string       `json:"case"`
	Viewports      []string     `json:"viewports"`
	Postconditions []string     `json:"postconditions"`
	UnrunReason    *UnrunReason `json:"unrun_reason,omitempty"`
}

type UnrunReason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Owner  string `json:"owner"`
}

var journeyIDRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var tsOwnerRE = regexp.MustCompile(`^ts:[^\s#]+#[^\s#]+$`)
var goOwnerRE = regexp.MustCompile(`^go:[^\s#]+#[^\s#]+$`)

var allowedReasons = map[string]bool{
	"unsupported-current": true, "native-shell-only": true,
	"browser-owned": true, "unsupported-by-design": true,
}

var requiredLegacyIDs = []string{
	"search-open", "new-note-in-notebook", "edit-save", "trash-restore",
	"delete-notebook-review", "insert-and-check-link", "inspect-local-graph",
	"open-sync-center", "editor-find-replace",
}

// LoadAndValidate loads a journey manifest relative to root and validates its
// contract, including that every source section still exists in the tree.
func LoadAndValidate(root, path string) (Manifest, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, err
	}
	manifestPath := path
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(absRoot, manifestPath)
	}
	var m Manifest
	if err := readStrict(manifestPath, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: %w", err)
	}
	if err := validate(absRoot, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func readStrict(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON values")
		}
		return err
	}
	return nil
}

func validate(root string, m *Manifest) error {
	if m.Schema != Schema {
		return fmt.Errorf("schema = %q, want %q", m.Schema, Schema)
	}
	if m.BrowserPolicy.Preferred != "browser-plugin" || m.BrowserPolicy.Observed != "absent" || m.BrowserPolicy.Fallback != "playwright" || strings.TrimSpace(m.BrowserPolicy.Reason) == "" {
		return fmt.Errorf("invalid browser_policy")
	}
	knownVP := make(map[string]bool, len(m.Viewports))
	for _, v := range m.Viewports {
		if v.ID == "" || v.Width <= 0 || v.Height <= 0 || knownVP[v.ID] {
			return fmt.Errorf("invalid or duplicate viewport %q", v.ID)
		}
		knownVP[v.ID] = true
	}
	if !hasViewport(m.Viewports, "desktop", 1440, 960) || !hasViewport(m.Viewports, "narrow-sync", 390, 844) {
		return fmt.Errorf("required viewports desktop 1440x960 and narrow-sync 390x844 are missing")
	}
	if len(m.Journeys) != 37 {
		return fmt.Errorf("journeys = %d, want 37", len(m.Journeys))
	}
	seen := map[string]bool{}
	executed, unverified := 0, 0
	for _, j := range m.Journeys {
		if !journeyIDRE.MatchString(j.ID) || seen[j.ID] {
			return fmt.Errorf("invalid or duplicate journey id %q", j.ID)
		}
		seen[j.ID] = true
		if strings.TrimSpace(j.Path) == "" || strings.TrimSpace(j.Section) == "" || strings.TrimSpace(j.Label) == "" {
			return fmt.Errorf("journey %q has incomplete identity", j.ID)
		}
		if !tsOwnerRE.MatchString(j.UIOwner) || !ownersValid(j.ActionOwners, tsOwnerRE) || !ownersValid(j.GOwners, goOwnerRE) {
			return fmt.Errorf("journey %q has invalid owner", j.ID)
		}
		if !sourceSectionExists(root, j.Path, j.Section) {
			return fmt.Errorf("journey %q source section %q not found in %q", j.ID, j.Section, j.Path)
		}
		if len(j.Viewports) > 0 {
			journeyViewports := make(map[string]bool, len(j.Viewports))
			for _, vp := range j.Viewports {
				if !knownVP[vp] || journeyViewports[vp] {
					return fmt.Errorf("journey %q references unknown viewport %q", j.ID, vp)
				}
				journeyViewports[vp] = true
			}
		}
		switch j.State {
		case "executed":
			executed++
			if strings.TrimSpace(j.Case) == "" || len(j.ActionOwners) == 0 || len(j.Viewports) == 0 || !nonempty(j.Postconditions) || !goOwnerRE.MatchString(j.CheckAnchor) || j.UnrunReason != nil {
				return fmt.Errorf("executed journey %q has invalid contract", j.ID)
			}
		case "unverified":
			unverified++
			if j.CheckAnchor != "" || j.Case != "" || len(j.Viewports) != 0 || len(j.Postconditions) != 0 || j.UnrunReason == nil || !allowedReasons[j.UnrunReason.Code] || strings.TrimSpace(j.UnrunReason.Detail) == "" || (!tsOwnerRE.MatchString(j.UnrunReason.Owner) && !goOwnerRE.MatchString(j.UnrunReason.Owner)) {
				return fmt.Errorf("unverified journey %q has invalid contract", j.ID)
			}
		default:
			return fmt.Errorf("journey %q has invalid state %q", j.ID, j.State)
		}
	}
	if executed != 32 || unverified != 5 {
		return fmt.Errorf("journey states executed=%d unverified=%d, want 32/5", executed, unverified)
	}
	for _, id := range requiredLegacyIDs {
		if !seen[id] {
			return fmt.Errorf("required legacy journey %q missing", id)
		}
	}
	return nil
}

// ProductionAnchors returns every declared UI, action, Go, and unrun owner.
// The caller resolves them through docaudit's frozen G18a source-symbol rules.
func (m Manifest) ProductionAnchors() []string {
	seen := map[string]bool{}
	var anchors []string
	add := func(anchor string) {
		if anchor != "" && !seen[anchor] {
			seen[anchor] = true
			anchors = append(anchors, anchor)
		}
	}
	for _, journey := range m.Journeys {
		add(journey.UIOwner)
		for _, owner := range journey.ActionOwners {
			add(owner)
		}
		for _, owner := range journey.GOwners {
			add(owner)
		}
		if journey.UnrunReason != nil {
			add(journey.UnrunReason.Owner)
		}
	}
	sort.Strings(anchors)
	return anchors
}

// CheckAnchors returns the unique result-bearing test anchors.
func (m Manifest) CheckAnchors() []string {
	seen := map[string]bool{}
	var anchors []string
	for _, journey := range m.Journeys {
		if journey.CheckAnchor != "" && !seen[journey.CheckAnchor] {
			seen[journey.CheckAnchor] = true
			anchors = append(anchors, journey.CheckAnchor)
		}
	}
	sort.Strings(anchors)
	return anchors
}

func hasViewport(vs []Viewport, id string, w, h int) bool {
	for _, v := range vs {
		if v.ID == id && v.Width == w && v.Height == h {
			return true
		}
	}
	return false
}
func ownersValid(xs []string, pattern *regexp.Regexp) bool {
	for _, x := range xs {
		if !pattern.MatchString(x) {
			return false
		}
	}
	return true
}
func nonempty(xs []string) bool {
	if len(xs) == 0 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return true
}

func sourceSectionExists(root, source, section string) bool {
	clean := filepath.Clean(source)
	if filepath.IsAbs(source) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	b, err := os.ReadFile(filepath.Join(root, clean))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimSpace(strings.TrimLeft(line, "#"))
		text = strings.TrimSpace(strings.TrimRight(text, "#"))
		if slug(text) == section {
			return true
		}
	}
	return false
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// Keep deterministic diagnostics useful to callers that inspect required IDs.
func requiredIDs() []string {
	ids := append([]string(nil), requiredLegacyIDs...)
	sort.Strings(ids)
	return ids
}
