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

// GUILocator names an element. For a browser-driven journey only three kinds
// are allowed: a role and an accessible name, a test id, or a CSS selector.
// Anything looser -- an nth-child path, a coordinate -- would describe today's
// markup rather than the thing being pointed at, and would go stale silently.
//
// Native is the fourth kind and it exists for the journeys a browser cannot
// reach at all. Importing, exporting, taking a snapshot and publishing name a
// folder on this machine, so they live behind the native bridge and their
// controls are correctly disabled in a browser -- which is what stopped them
// being photographed until the desktop harness could do it. There is no DOM to
// query from xdotool, so a desktop step names the thing a person would name
// ("the File menu's Import and export…") and is driven by keystrokes.
//
// That would be a weaker claim on its own, and it is not left as one: every
// desktop step that acts declares the line it expects in the application's own
// transcript, so a keystroke that lands somewhere else fails the journey
// instead of producing a confident picture of the wrong place. That is the
// property the DOM locator provides for the browser half, obtained a different
// way for the half that has no DOM.
type GUILocator struct {
	Role   string `json:"role,omitempty"`
	Name   string `json:"name,omitempty"`
	TestID string `json:"testid,omitempty"`
	CSS    string `json:"css,omitempty"`
	Native string `json:"native,omitempty"`
}

type GUIStep struct {
	ID        string     `json:"id"`
	Narrative string     `json:"narrative"`
	Locator   GUILocator `json:"locator"`
	Action    string     `json:"action"`
	Value     string     `json:"value,omitempty"`
	// Keys is the key sequence a desktop step sends before typing Value, as
	// xdotool names them and separated by spaces ("Tab Tab Return").
	Keys string `json:"keys,omitempty"`
	// Expect lists phrases that must appear, in order, in the application's
	// transcript after this step acts. It is what makes a keyboard-driven step
	// falsifiable: the interface says what it did, rather than the harness
	// inferring it from how much of the screen changed. The phrases name the
	// operation *and* the path it ran against, so a keystroke that lands in
	// the wrong field cannot satisfy them.
	//
	// They are also the shutter release. A desktop step is photographed after
	// it acts and after the application says the act finished, because the
	// picture worth printing beside "type the folder here" is the one with the
	// folder in it.
	Expect []string `json:"expect,omitempty"`
	// Confirm marks a step whose control asks the person to confirm before it
	// acts. It is declared per step rather than accepted for every step,
	// because a capture that silently agreed to every dialog would be a
	// capture that could delete something no journey meant to touch -- and
	// because the confirmation is part of what the journey has to show.
	Confirm bool `json:"confirm,omitempty"`
}

type GUIJourney struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Goal    string `json:"goal"`
	Feature string `json:"feature"`
	// Driver is "browser" (the default, and what the Playwright capture runs)
	// or "desktop" for a journey only the real application can perform.
	Driver        string    `json:"driver,omitempty"`
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
		if !journey.browser() && journey.Driver != DesktopDriver {
			return GUICatalogue{}, fmt.Errorf("%s names an unknown driver %q", journey.ID, journey.Driver)
		}
		for _, step := range journey.Steps {
			if step.ID == "" || step.Narrative == "" {
				return GUICatalogue{}, fmt.Errorf("%s has a step with no id or narrative", journey.ID)
			}
			if step.Locator == (GUILocator{}) {
				return GUICatalogue{}, fmt.Errorf("%s/%s has no locator", journey.ID, step.ID)
			}
			// The two halves may not borrow each other's vocabulary. A native
			// locator in a browser journey would never be looked up, and a DOM
			// locator in a desktop one would claim a precision the keyboard
			// does not have -- both would read as checked and be nothing of
			// the kind.
			if journey.browser() && step.Locator.Native != "" {
				return GUICatalogue{}, fmt.Errorf("%s/%s is browser-driven and cannot use a native locator", journey.ID, step.ID)
			}
			if !journey.browser() {
				if step.Locator.Native == "" {
					return GUICatalogue{}, fmt.Errorf("%s/%s is desktop-driven and must name what it drives natively", journey.ID, step.ID)
				}
				if step.Action != "drive" {
					return GUICatalogue{}, fmt.Errorf("%s/%s has action %q; every desktop step drives something", journey.ID, step.ID, step.Action)
				}
				// A step with nothing to press and nothing to type would
				// photograph whatever the window happened to be showing, which
				// is the one thing a documentation screenshot must never be.
				if step.Keys == "" && step.Value == "" {
					return GUICatalogue{}, fmt.Errorf("%s/%s drives nothing", journey.ID, step.ID)
				}
			}
		}
	}
	return catalogue, nil
}

// DesktopDriver marks a journey the real application performs, rather than a
// browser pointed at the same service.
const DesktopDriver = "desktop"

// browser reports whether this journey is the Playwright capture's to run. An
// empty driver means browser, so every journey written before the desktop half
// existed keeps its meaning.
func (j GUIJourney) browser() bool { return j.Driver == "" || j.Driver == "browser" }

// Desktop reports whether the real application has to perform this journey.
func (j GUIJourney) Desktop() bool { return !j.browser() }

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

// Each task below lists its steps, with a picture of every one.
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
		title := journey.Title
		if journey.Desktop() {
			// Said in the catalogue rather than in a paragraph above it,
			// because a reader arrives at one task and not at the list.
			title += " (desktop app only)"
		}
		lines = append(lines, fmt.Sprintf("**%s** — %s", title, journey.Goal))
		for _, step := range journey.Steps {
			lines = append(lines, fmt.Sprintf("  - %s\n\n    ![%s](images/journeys/%s-%s.png)",
				step.Narrative, step.ID, journey.ID, step.ID))
		}
	}
	return lines
}
