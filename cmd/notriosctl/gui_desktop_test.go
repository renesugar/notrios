package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDesktopImportThroughTheNativeMenu drives the real Wails application.
//
// It exists because the browser crawl cannot reach these controls, and cannot
// be made to. Importing, exporting and taking a snapshot go through the native
// bridge, which is bound only when the window's own process owns the store, so
// a browser correctly finds the control disabled -- which is the right product
// behaviour and a dead end for a browser-driven test. The measurement gap was
// recorded as a limitation until it turned out that Xvfb, openbox and xdotool
// are all available here, and that the desktop application runs headlessly
// under them.
//
// What this proves is deliberately not a screenshot. A picture shows that a
// window rendered, not that anything worked. This drives a real import from the
// native menu and then asks the running service, over its own HTTP surface,
// whether the note arrived -- so it covers menu, event, modal, bridge, importer
// and store, and fails if any link in that chain is broken.
//
// Two harnesses, two jobs. Playwright reads the DOM and answers what controls
// exist; this reads the database afterwards and answers whether the ones a
// browser cannot use actually work.
func TestDesktopImportThroughTheNativeMenu(t *testing.T) {
	if os.Getenv("NOTRIOS_GUI_DESKTOP_RUN") != "1" {
		t.Skip("set NOTRIOS_GUI_DESKTOP_RUN=1 to drive the desktop application under Xvfb")
	}
	for _, tool := range []string{"Xvfb", "openbox", "xdotool", "xclip", "scrot"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required to drive the desktop application: %v", tool, err)
		}
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	desktop := filepath.Join(repoRoot, "bin", "notrios")
	if _, err := os.Stat(desktop); err != nil {
		t.Fatalf("build the desktop binary first with `make gui`: %v", err)
	}
	webDir := filepath.Join(repoRoot, "web", "dist")
	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		t.Fatalf("the desktop application serves built web assets from %s: %v", webDir, err)
	}

	cli := buildCLI(t)
	replica := newSyncReplica(t, cli, "gui-desktop", t.TempDir())
	replica.run(t, "init")
	source := seedJoplinFixture(t, filepath.Join(t.TempDir(), "joplin-export"))

	address := unusedLoopbackAddress(t)
	config := g18eConfig(t, replica, address, "", true, webDir)
	display := startVirtualDisplay(t)

	application := exec.Command(desktop, "-config", config)
	application.Env = append(os.Environ(), "DISPLAY="+display,
		// Echo the interface's own transcript into the log stream. Without it
		// this test can only measure pixels, and a percentage of the screen is
		// a poor way to say whether an import was requested.
		"NOTRIOS_UI_LOG=1",
		// WebKit's sandbox and GPU paths are the usual cause of a headless
		// window that never appears. Both are disabled rather than debugged,
		// because what is under test is Notrios, not WebKit's renderer choice.
		"WEBKIT_DISABLE_COMPOSITING_MODE=1", "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1")
	// Captured as well as forwarded: the transcript is what the assertions below
	// read, and the forwarding keeps a failing run readable in the test output.
	transcript := &liveLog{}
	application.Stdout = io.MultiWriter(os.Stderr, transcript)
	application.Stderr = application.Stdout
	if err := application.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = application.Process.Kill()
		_, _ = application.Process.Wait()
	})

	window := waitForWindow(t, display, "Notrios")
	t.Logf("desktop window %s on %s", window, display)
	xdo(t, display, "windowactivate", "--sync", window)

	// The window exists long before the webview has painted anything, and
	// under software rendering the gap is around ten seconds. The first run of
	// this test typed into a blank window and then waited ninety seconds for an
	// import that had never been asked for; the screenshot it left is what said
	// so. Waiting for the window is therefore not enough -- this waits for the
	// application to be on the screen.
	shots := t.TempDir()
	waitForPaintedWindow(t, display, shots)

	// Before anything is driven, find out what is running. Twice while this was
	// being written a `go build` executed from the wrong directory and the run
	// went ahead against a stale binary, failing in ways that looked like
	// product defects. About answers it in words rather than pixels, and this
	// is the only channel by which the application can tell the harness a fact.
	assertRunningBuild(t, display, shots, repoRoot)

	xdo(t, display, "key", "--clearmodifiers", "ctrl+i")
	// A named event rather than a percentage of changed pixels. The dialog says
	// it opened; nothing has to infer it from how much of the screen repainted.
	transcript.waitFor(t, "the import and export dialog opened",
		"the import and export dialog never opened after Ctrl+I")

	// Reached by keyboard, not by coordinates. The dialog puts focus on its
	// close button, so one Tab lands on the first path field; the three that
	// follow step over the chooser and the scan button to reach Import. A
	// pixel-driven click would have to know where the native menu bar ends,
	// which is a property of the window manager rather than of Notrios.
	dialog := filepath.Join(shots, "dialog.png")
	capture(t, display, dialog)
	xdo(t, display, "key", "--clearmodifiers", "Tab")
	xdo(t, display, "type", "--delay", "20", source)
	waitForVisualChange(t, display, shots, dialog, typedPixelFloor,
		"the source folder never reached the first field")
	for i := 0; i < 3; i++ {
		xdo(t, display, "key", "--clearmodifiers", "Tab")
	}
	xdo(t, display, "key", "--clearmodifiers", "Return")
	transcript.waitFor(t, "joplin-apply requested", "Import was never requested")
	transcript.waitFor(t, "joplin-apply finished", "the import never finished")

	// A picture of whatever the window showed, whether this passes or not. On a
	// failure it is the only account of what actually happened -- twice while
	// this was being written it was the screenshot rather than the message that
	// identified the cause, and both times the message alone had pointed at the
	// wrong thing.
	evidence := filepath.Join(repoRoot, "performance", "v0.8-h15", "DESKTOP_IMPORT.png")
	t.Cleanup(func() { capture(t, display, evidence) })

	waitForImportedNote(t, "http://"+address, "Joplin Fixture")

	// Taken after the import has landed rather than immediately after the
	// keystroke, so the committed image shows a finished operation with its
	// report rather than a dialog that has only just been asked to start one.
	// The cleanup above still overwrites it on the way out; this makes the
	// passing image the interesting one by giving the report a moment to
	// render first.
	time.Sleep(1 * time.Second)
}

// seedJoplinFixture writes the smallest RAW export an import can consume.
func seedJoplinFixture(t *testing.T, root string) string {
	t.Helper()
	files := map[string]string{
		"folder.md": "Imported Notes\n\nid: folder-fixture\ntype_: 2\n",
		"note.md":   "Joplin Fixture\n\nA searchable imported note.\n\nid: joplin-fixture\nparent_id: folder-fixture\ntype_: 1\n",
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// assertRunningBuild reads the About report off the clipboard and checks it.
//
// The menu is driven by keyboard rather than by clicking where the items are
// drawn: F10 focuses a GTK menu bar, the arrows walk it, and none of that
// depends on the window manager's decorations. The earlier draft of this used
// hard-coded pixel positions taken from one openbox theme.
//
// About deliberately has no accelerator -- a shortcut for an About box is
// clutter -- which makes this the one path that exercises the native menu
// itself rather than only the accelerator, and so the one that would notice a
// menu item registered but never rendered.
func assertRunningBuild(t *testing.T, display, shots, repoRoot string) {
	t.Helper()
	closed := filepath.Join(shots, "before-about.png")
	capture(t, display, closed)

	xdo(t, display, "key", "--clearmodifiers", "F10")
	time.Sleep(600 * time.Millisecond)
	xdo(t, display, "key", "--clearmodifiers", "Right")
	xdo(t, display, "key", "--clearmodifiers", "Right")
	xdo(t, display, "key", "--clearmodifiers", "Down")
	xdo(t, display, "key", "--clearmodifiers", "Return")
	waitForVisualChange(t, display, shots, closed, dialogPixelFloor,
		"the About dialog never appeared from Help > About")

	// Copy holds focus when the dialog opens, so the report is one key away for
	// a person and for this.
	xdo(t, display, "key", "--clearmodifiers", "Return")
	report := waitForClipboard(t, display, repoRoot)
	t.Logf("running build:\n%s", report)

	if !strings.Contains(report, "Mode: desktop application") {
		t.Fatalf("About did not report a desktop build; the native bridge is not bound:\n%s", report)
	}
	head := gitHead(t, repoRoot)
	if head != "" && !strings.Contains(report, head) {
		t.Fatalf("the running binary is not built from this checkout.\nHEAD is %s\nAbout says:\n%s\n"+
			"Rebuild with `make gui` before running this.", head, report)
	}

	xdo(t, display, "key", "--clearmodifiers", "Escape")
	time.Sleep(500 * time.Millisecond)
}

// waitForClipboard polls until the copy has landed.
//
// writeText returns a promise, so the clipboard is not populated the instant
// the key is pressed. The first version of this waited a fixed 700ms and
// failed; picking a slightly larger number would have been the same mistake
// with a longer fuse, which is how the two pixel thresholds in this file went
// wrong. xclip exits non-zero while the selection is empty, so that is the
// condition to wait on rather than a duration to guess.
func waitForClipboard(t *testing.T, display, repoRoot string) string {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		read := exec.Command("xclip", "-o", "-selection", "clipboard")
		read.Env = append(os.Environ(), "DISPLAY="+display)
		out, err := read.Output()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return string(out)
		}
		if time.Now().After(deadline) {
			// Kept where it survives the test, because a screenshot inside a
			// temporary directory is deleted exactly when it becomes useful.
			// Deliberately not under performance/: this is a failure artefact
			// for whoever is debugging, not evidence, and evidence directories
			// should not accumulate pictures of things that went wrong.
			capture(t, display, filepath.Join(os.TempDir(), "notrios-about-failed.png"))
			t.Logf("a screenshot of the failing window is in %s",
				filepath.Join(os.TempDir(), "notrios-about-failed.png"))
			t.Fatalf("the About report never reached the clipboard; the last read said %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// gitHead returns the checked-out revision, or "" when that cannot be known.
// An unknown revision is not a failure: the check it feeds is a convenience
// against a stale binary, and refusing to run without git would make the
// harness harder to use than the problem it prevents.
func gitHead(t *testing.T, repoRoot string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = repoRoot
	out, err := command.Output()
	if err != nil {
		t.Logf("the checkout revision is unknown (%v); not comparing it against the running build", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

// waitForPaintedWindow blocks until the application has drawn itself.
//
// Readiness is taken from the screen rather than from a signal the product does
// not have. An empty webview is a single flat colour; the workspace is several
// hundred. Counting distinct colours below the native menu bar separates the
// two without knowing anything about what is being drawn, which is the point:
// a probe that looked for a particular pixel would have to be updated every
// time the interface changed.
func waitForPaintedWindow(t *testing.T, display, directory string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for attempt := 0; ; attempt++ {
		shot := filepath.Join(directory, fmt.Sprintf("paint-%02d.png", attempt))
		if colours := distinctColours(t, display, shot); colours >= paintedColourFloor {
			t.Logf("the window had painted after %d attempts (%d distinct colours)", attempt+1, colours)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the application never painted on %s; the window is there and empty", display)
		}
		time.Sleep(1 * time.Second)
	}
}

// waitForVisualChange blocks until the screen differs from an earlier capture.
//
// This is how a keystroke is confirmed to have done something without a way to
// ask the page. It is deliberately a weak claim -- something changed -- because
// the strong claim is made at the end by asking the service whether the note
// exists. Its job is to fail early and clearly when a keystroke went nowhere,
// rather than to leave that to a ninety-second timeout with no explanation.
func waitForVisualChange(t *testing.T, display, directory, before string, floor float64, complaint string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for attempt := 0; ; attempt++ {
		shot := filepath.Join(directory, fmt.Sprintf("change-%02d.png", attempt))
		capture(t, display, shot)
		if changed := changedFraction(t, before, shot); changed >= floor {
			t.Logf("%.1f%% of the screen changed", changed*100)
			return shot
		}
		if time.Now().After(deadline) {
			t.Fatal(complaint)
		}
		time.Sleep(1 * time.Second)
	}
}

const (
	// An unpainted webview is one flat colour plus the window manager's frame.
	paintedColourFloor = 200
	// A dialog covering the workspace repaints most of the screen.
	dialogPixelFloor = 0.02
	// A path typed into one field does not. It is roughly two hundred pixels by
	// sixteen against a screen of about a million, which is a quarter of one
	// per cent -- so the dialog's floor rejected a field that had filled in
	// correctly, and the run failed claiming the text never arrived. A blinking
	// caret alone is two orders of magnitude smaller than this.
	typedPixelFloor = 0.001
	// The native menu bar is drawn by GTK and is on screen before the webview
	// has anything, so it is excluded from both measurements.
	menuBarHeight = 64
)

func distinctColours(t *testing.T, display, path string) int {
	t.Helper()
	capture(t, display, path)
	frame := decodePNG(t, path)
	if frame == nil {
		return 0
	}
	bounds := frame.Bounds()
	seen := make(map[uint32]struct{})
	for y := bounds.Min.Y + menuBarHeight; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := frame.At(x, y).RGBA()
			seen[r>>8<<16|g>>8<<8|b>>8] = struct{}{}
		}
	}
	return len(seen)
}

func changedFraction(t *testing.T, beforePath, afterPath string) float64 {
	t.Helper()
	before, after := decodePNG(t, beforePath), decodePNG(t, afterPath)
	if before == nil || after == nil || before.Bounds() != after.Bounds() {
		return 0
	}
	bounds := before.Bounds()
	var differing, total int
	for y := bounds.Min.Y + menuBarHeight; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			total++
			if before.At(x, y) != after.At(x, y) {
				differing++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(differing) / float64(total)
}

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	frame, err := png.Decode(file)
	if err != nil {
		return nil
	}
	return frame
}

func capture(t *testing.T, display, path string) {
	t.Helper()
	shot := exec.Command("scrot", "--overwrite", path)
	shot.Env = append(os.Environ(), "DISPLAY="+display)
	if out, err := shot.CombinedOutput(); err != nil {
		t.Logf("no screenshot captured (%v): %s", err, out)
	}
}

// startVirtualDisplay brings up Xvfb and a window manager, and returns the
// display name. openbox is not decoration: without a window manager the window
// never takes focus, and every keystroke below would go nowhere.
func startVirtualDisplay(t *testing.T) string {
	t.Helper()
	display := fmt.Sprintf(":%d", 90+os.Getpid()%9)
	server := exec.Command("Xvfb", display, "-screen", "0", "1400x900x24", "-nolisten", "tcp")
	if err := server.Start(); err != nil {
		t.Fatalf("Xvfb would not start: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Process.Kill()
		_, _ = server.Process.Wait()
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		probe := exec.Command("xdotool", "getdisplaygeometry")
		probe.Env = append(os.Environ(), "DISPLAY="+display)
		if probe.Run() == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Xvfb never became usable on %s", display)
		}
		time.Sleep(200 * time.Millisecond)
	}
	manager := exec.Command("openbox")
	manager.Env = append(os.Environ(), "DISPLAY="+display)
	if err := manager.Start(); err != nil {
		t.Fatalf("openbox would not start: %v", err)
	}
	t.Cleanup(func() {
		_ = manager.Process.Kill()
		_, _ = manager.Process.Wait()
	})
	time.Sleep(500 * time.Millisecond)
	return display
}

func waitForWindow(t *testing.T, display, name string) string {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		search := exec.Command("xdotool", "search", "--onlyvisible", "--name", name)
		search.Env = append(os.Environ(), "DISPLAY="+display)
		out, err := search.Output()
		if err == nil {
			if ids := strings.Fields(string(out)); len(ids) > 0 {
				return ids[len(ids)-1]
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %q window appeared on %s within 60s", name, display)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func xdo(t *testing.T, display string, args ...string) {
	t.Helper()
	command := exec.Command("xdotool", args...)
	command.Env = append(os.Environ(), "DISPLAY="+display)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("xdotool %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// waitForImportedNote asks the running service whether the import landed.
//
// The service is asked rather than the database file, because the application
// holds that file open and because the question is whether a person using this
// window would now find the note -- which is a question about the running
// library, not about a row.
func waitForImportedNote(t *testing.T, baseURL, phrase string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		response, err := http.Get(baseURL + "/api/v1/search?q=" + strings.ReplaceAll(phrase, " ", "+"))
		if err == nil {
			var page struct {
				Hits []struct {
					Title string `json:"title"`
				} `json:"hits"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&page)
			response.Body.Close()
			if decodeErr == nil {
				for _, hit := range page.Hits {
					if strings.Contains(hit.Title, phrase) {
						return
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the import driven through the native menu never produced a note titled %q", phrase)
		}
		time.Sleep(1 * time.Second)
	}
}

// liveLog collects the application's output while it is still running.
//
// The desktop application prints the service's log and, with NOTRIOS_UI_LOG=1,
// the interface's transcript into the same stream in real order. That ordering
// is the useful part: "import requested" followed three lines later by a store
// error is a diagnosis, where either line alone is a guess.
type liveLog struct {
	mu    sync.Mutex
	lines strings.Builder
}

func (l *liveLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lines.Write(p)
}

func (l *liveLog) contains(phrase string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Contains(l.lines.String(), phrase)
}

func (l *liveLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lines.String()
}

// waitFor blocks until the application says it did something.
//
// This replaces measuring how much of the screen changed, which was the
// weakest part of this test and wrong twice: once because a field had
// collapsed to zero width, and once because a path typed into one input
// changes about one per cent of the screen and the threshold had been set for
// a dialog covering all of it. Both times the message was accurate and the
// reason was not. A named event cannot be off by a threshold.
func (l *liveLog) waitFor(t *testing.T, phrase, complaint string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		if l.contains(phrase) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s (never saw %q). The application said:\n%s", complaint, phrase, l.String())
		}
		time.Sleep(500 * time.Millisecond)
	}
}
