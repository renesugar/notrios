package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/docjourneys"
)

// TestDesktopJourneyCapture photographs the journeys a browser cannot perform.
//
// Importing, exporting, taking a snapshot and publishing name a folder on the
// machine that owns the library. They go through the native bridge, no REST
// route starts them, and the Import/Export control is therefore correctly
// disabled in a browser -- so the Playwright capture, which is a browser, could
// not reach the dialog to photograph it. That was recorded as a limitation of
// the pictures for as long as the desktop harness could only assert. It can
// drive the real application now, so it can also capture it.
//
// What is different about a desktop journey, and it is not a shortcut. There is
// no DOM to query from xdotool, so no locator is looked up and no click marker
// is drawn -- and there is nothing for a marker to point at, because every step
// here is a keystroke rather than a place. The falsifiability that a locator
// provides in the browser half is obtained instead from the application's own
// transcript: each acting step declares the line it expects, and those lines
// name both the operation and the path it ran against. A keystroke that lands
// on the wrong field produces a different line, or none, and the journey fails
// rather than producing a confident picture of the wrong thing.
func TestDesktopJourneyCapture(t *testing.T) {
	root := repositoryRoot(t)
	catalogue, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
	if err != nil {
		t.Fatalf("reading the GUI journey catalogue: %v", err)
	}
	wanted := []docjourneys.GUIJourney{}
	for _, journey := range catalogue.Journeys {
		if journey.Desktop() {
			wanted = append(wanted, journey)
		}
	}
	if len(wanted) == 0 {
		t.Skip("no desktop-driven journeys in the catalogue")
	}

	captured := []docjourneys.ImageRecord{}
	for _, journey := range wanted {
		journey := journey
		t.Run(journey.ID, func(t *testing.T) {
			captured = append(captured, captureDesktopJourney(t, root, journey)...)
		})
	}
	if len(captured) == 0 {
		return // every journey skipped: nothing ran, so nothing is rewritten
	}
	assertPicturesDiffer(t, captured)
	mergeCapturedImages(t, root, catalogue, captured)
}

// captureDesktopJourney drives one journey in the real window and returns what
// it photographed.
func captureDesktopJourney(t *testing.T, root string, journey docjourneys.GUIJourney) []docjourneys.ImageRecord {
	t.Helper()
	app := launchDesktopAppOn(t, "capture-"+journey.ID, 1280, 800, true)
	// Sized to the catalogue's viewport so a desktop picture sits beside a
	// browser one at the same width. The window opens larger than the screen
	// it is given, and openbox is happy to leave it that way.
	xdo(t, app.display, "windowsize", "--sync", app.window, "1280", "800")
	xdo(t, app.display, "windowmove", "--sync", app.window, "0", "0")
	xdo(t, app.display, "windowactivate", "--sync", app.window)

	replace := strings.NewReplacer(
		"{joplin}", seedJoplinFixture(t, filepath.Join(t.TempDir(), "joplin-export")),
		"{empty}", t.TempDir(),
	)
	imageDir := filepath.Join(root, "docs", "images", "journeys")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	records := []docjourneys.ImageRecord{}
	for _, step := range journey.Steps {
		file := fmt.Sprintf("%s-%s.png", journey.ID, step.ID)
		absolute := filepath.Join(imageDir, file)
		for _, key := range strings.Fields(step.Keys) {
			xdo(t, app.display, "key", "--clearmodifiers", key)
		}
		if value := replace.Replace(step.Value); value != "" {
			xdo(t, app.display, "type", "--delay", "20", value)
		}
		// Every phrase, in order. The first names the operation and the path it
		// ran against, so a Tab that stopped one control short cannot satisfy
		// it; the last says the work finished, which is what makes the picture
		// below a picture of a result rather than of a spinner.
		for _, phrase := range step.Expect {
			want := replace.Replace(phrase)
			app.transcript.waitFor(t, want,
				fmt.Sprintf("%s/%s: the application never reported %q", journey.ID, step.ID, want))
		}
		// Photographed after acting, because the picture worth printing beside
		// "type the folder here" is the one with the folder in it. A step whose
		// picture showed the state before it acted would show the previous
		// step's result, and three journeys that open the same dialog would
		// ship the same photograph three times -- which is how this was found.
		settleForCapture()
		capture(t, app.display, absolute)

		contents, err := os.ReadFile(absolute)
		if err != nil {
			t.Fatalf("%s/%s: no screenshot was written: %v", journey.ID, step.ID, err)
		}
		digest := sha256.Sum256(contents)
		records = append(records, docjourneys.ImageRecord{
			Journey: journey.ID,
			Step:    step.ID,
			Image:   "images/journeys/" + file,
			Locator: step.Locator,
			SHA256:  hex.EncodeToString(digest[:]),
		})
	}
	return records
}

// assertPicturesDiffer catches a capture that has stopped photographing the
// thing it names.
//
// The browser runner makes this check because a missing click marker turns
// every step in one app state into the same bytes. The desktop half has no
// marker, so two steps that open the same dialog *should* produce the same
// picture -- and do. What must not happen is two different targets
// photographing identically, which would mean a keystroke went nowhere and the
// screen never changed.
func assertPicturesDiffer(t *testing.T, captured []docjourneys.ImageRecord) {
	t.Helper()
	seen := map[string]docjourneys.ImageRecord{}
	for _, record := range captured {
		if same, ok := seen[record.SHA256]; ok && same.Locator != record.Locator {
			t.Errorf("%s/%s photographed identically to %s/%s despite driving something else; "+
				"a keystroke probably went nowhere", record.Journey, record.Step, same.Journey, same.Step)
		}
		if _, ok := seen[record.SHA256]; !ok {
			seen[record.SHA256] = record
		}
	}
}

// mergeCapturedImages writes the new records into the shared manifest without
// disturbing the browser capture's rows.
//
// Two runners own halves of one file. Rewriting it from either would delete the
// other's rows, and a deleted row looks exactly like a journey that was never
// captured -- which is the failure the manifest exists to make impossible.
func mergeCapturedImages(t *testing.T, root string, catalogue docjourneys.GUICatalogue, captured []docjourneys.ImageRecord) {
	t.Helper()
	path := filepath.Join(root, "docs", "images", "journeys", "MANIFEST.json")
	manifest, err := docjourneys.LoadImages(path)
	if err != nil {
		t.Fatalf("reading the image manifest: %v", err)
	}
	// Replaced by journey rather than by step. Keying on the step leaves a row
	// behind whenever a journey loses one -- the file is deleted, the row is
	// not, and the manifest then claims a screenshot that does not exist.
	// Found exactly that way, by rewriting these three journeys from four steps
	// to three.
	replaced := map[string]bool{}
	for _, record := range captured {
		replaced[record.Journey] = true
	}
	merged := []docjourneys.ImageRecord{}
	for _, record := range manifest.Images {
		if !replaced[record.Journey] {
			merged = append(merged, record)
		}
	}
	merged = append(merged, captured...)
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Journey != merged[j].Journey {
			return merged[i].Journey < merged[j].Journey
		}
		return merged[i].Step < merged[j].Step
	})

	// The viewport is written back as the catalogue declares it, which is what
	// the window was sized to before anything was photographed.
	out := struct {
		Schema   string `json:"schema"`
		Viewport struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"viewport"`
		Images []docjourneys.ImageRecord `json:"images"`
	}{Schema: docjourneys.ImageSchema, Viewport: catalogue.Viewport, Images: merged}
	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d desktop images into %s (%d rows total)", len(captured), path, len(merged))
}

// settleForCapture waits for the window to finish drawing what it was just
// told to do. Under software rendering a screenshot taken in the same
// millisecond as the keystroke catches the frame before it.
func settleForCapture() { time.Sleep(1200 * time.Millisecond) }

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
