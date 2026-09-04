package docjourneys

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const GUISchema = "notrios.docjourneys.gui.v1"
const ImageSchema = "notrios.docjourneys.gui-images.v1"

// GUILocator names an element. Only these three kinds are allowed: a role and
// an accessible name, a test id, or a CSS selector. Anything looser -- an
// nth-child path, a coordinate -- would describe today's markup rather than the
// thing being pointed at, and would go stale silently.
type GUILocator struct {
	Role   string `json:"role,omitempty"`
	Name   string `json:"name,omitempty"`
	TestID string `json:"testid,omitempty"`
	CSS    string `json:"css,omitempty"`
}

type GUIStep struct {
	ID        string     `json:"id"`
	Narrative string     `json:"narrative"`
	Locator   GUILocator `json:"locator"`
	Action    string     `json:"action"`
	Value     string     `json:"value,omitempty"`
}

type GUIJourney struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Goal          string    `json:"goal"`
	Feature       string    `json:"feature"`
	Steps         []GUIStep `json:"steps"`
	Postcondition struct {
		Narrative string     `json:"narrative"`
		Locator   GUILocator `json:"locator"`
	} `json:"postcondition"`
}

type GUICatalogue struct {
	Schema   string `json:"schema"`
	Viewport struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"viewport"`
	Journeys []GUIJourney `json:"journeys"`
}

// ImageRecord is one captured step: which element it pointed at, and the bytes
// that were produced.
type ImageRecord struct {
	Journey string     `json:"journey"`
	Step    string     `json:"step"`
	Image   string     `json:"image"`
	Locator GUILocator `json:"locator"`
	SHA256  string     `json:"sha256"`
}

type ImageManifest struct {
	Schema string        `json:"schema"`
	Images []ImageRecord `json:"images"`
}

func LoadGUI(path string) (GUICatalogue, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return GUICatalogue{}, err
	}
	var catalogue GUICatalogue
	if err := json.Unmarshal(contents, &catalogue); err != nil {
		return GUICatalogue{}, err
	}
	if catalogue.Schema != GUISchema {
		return GUICatalogue{}, fmt.Errorf("unexpected GUI journey schema %q", catalogue.Schema)
	}
	for _, journey := range catalogue.Journeys {
		if journey.ID == "" || journey.Title == "" || journey.Goal == "" || journey.Feature == "" {
			return GUICatalogue{}, fmt.Errorf("GUI journey %q is incomplete", journey.ID)
		}
		for _, step := range journey.Steps {
			if step.ID == "" || step.Narrative == "" {
				return GUICatalogue{}, fmt.Errorf("%s has a step with no id or narrative", journey.ID)
			}
			if step.Locator == (GUILocator{}) {
				return GUICatalogue{}, fmt.Errorf("%s/%s has no locator", journey.ID, step.ID)
			}
		}
	}
	return catalogue, nil
}

func LoadImages(path string) (ImageManifest, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ImageManifest{}, err
	}
	var manifest ImageManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return ImageManifest{}, err
	}
	if manifest.Schema != ImageSchema {
		return ImageManifest{}, fmt.Errorf("unexpected image manifest schema %q", manifest.Schema)
	}
	return manifest, nil
}

// VerifyImages checks the committed screenshots against the manifest.
//
// This is the half of the design that runs without a browser, and it is what
// makes a screenshot in the documentation falsifiable. A picture that no longer
// matches what the interface does is worse than no picture: it is a confident
// claim, and nothing about a stale PNG announces itself.
func VerifyImages(docsDir string, catalogue GUICatalogue, manifest ImageManifest) []string {
	problems := []string{}
	recorded := map[string]ImageRecord{}
	for _, image := range manifest.Images {
		recorded[image.Journey+"/"+image.Step] = image
	}

	for _, journey := range catalogue.Journeys {
		for _, step := range journey.Steps {
			key := journey.ID + "/" + step.ID
			image, ok := recorded[key]
			if !ok {
				problems = append(problems, key+" has no captured screenshot")
				continue
			}
			// The locator is recorded with the image so a step that starts
			// pointing somewhere else cannot keep the old picture.
			if image.Locator != step.Locator {
				problems = append(problems, key+" points at a different element than the one photographed")
			}
			path := filepath.Join(docsDir, filepath.FromSlash(image.Image))
			contents, err := os.ReadFile(path)
			if err != nil {
				problems = append(problems, key+" is missing its image file: "+err.Error())
				continue
			}
			if digest := hex.EncodeToString(sha256Sum(contents)); digest != image.SHA256 {
				problems = append(problems, key+" image does not match the hash recorded when it was captured")
			}
			delete(recorded, key)
		}
	}
	for key := range recorded {
		problems = append(problems, key+" has a screenshot but no step; it is left over from a journey that changed")
	}
	return problems
}

func sha256Sum(contents []byte) []byte {
	sum := sha256.Sum256(contents)
	return sum[:]
}

// GUILines renders the GUI catalogue for the generated fragment.
//
//notrios:doc user gui-journey-catalogue
//notrios:help journeys-gui the-journeys
//notrios:enumerates go:github.com/renesugar/notrios/internal/docjourneys#GUICatalogue
func (c GUICatalogue) GUILines() []string {
	// Steps, their sentences and their pictures are emitted here rather than
	// written into the page, so the caption beside a screenshot is the same
	// string the runner used when it took it. Keeping them in two places would
	// let a description drift from the image next to it.
	lines := []string{}
	for _, journey := range c.Journeys {
		lines = append(lines, fmt.Sprintf("**%s** — %s", journey.Title, journey.Goal))
		for _, step := range journey.Steps {
			lines = append(lines, fmt.Sprintf("%s ![%s](images/journeys/%s-%s.png)",
				step.Narrative, step.ID, journey.ID, step.ID))
		}
	}
	return lines
}
