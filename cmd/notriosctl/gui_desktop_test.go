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
	return startVirtualDisplaySized(t, 1400, 900)
}

func startVirtualDisplaySized(t *testing.T, width, height int) string {
	t.Helper()
	display := fmt.Sprintf(":%d", 90+os.Getpid()%9)
	server := exec.Command("Xvfb", display, "-screen", "0",
		fmt.Sprintf("%dx%dx24", width, height), "-nolisten", "tcp")
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

// TestDesktopCloseAsksBeforeDiscardingUnsavedWork drives the one path the
// frontend cannot protect.
//
// Saving in Notrios is deliberate rather than continuous, because every save
// writes a revision that replicates and is never pruned. The editor can
// therefore hold text that is in no store, and the interface asks before
// anything replaces it -- except the window's own close button, which does not
// go through `beforeunload` at all. `OnBeforeClose` is where that question has
// to be asked, in Go, which means the window has to have told Go what it is
// holding (cmd/notrios/gui_window_state.go).
//
// None of that is observable from a browser: there is no window to close, no
// binding to receive the report, and no native dialog to answer. This drives
// the real GTK close request with xdotool and reads the application's own
// transcript for what it decided, rather than inferring it from pixels.
func TestDesktopCloseAsksBeforeDiscardingUnsavedWork(t *testing.T) {
	app := launchDesktopApp(t, "gui-close")

	app.typeIntoTheEditor(t, "unsaved words")

	// A named event rather than a percentage of changed pixels: the window
	// itself says it told the shell, which is the fact under test. If this line
	// never arrives the binding was never injected or never called, and no
	// amount of screen comparison would have said which.
	app.transcript.waitFor(t, "editor: unsaved changes present",
		"the window never told the shell it was holding unsaved work")

	// The title bar's close button, as a person would use it. Alt+F4 is
	// openbox's Close, which sends WM_DELETE_WINDOW -- the GTK delete-event
	// Wails turns into its quit message. Not xdotool's `windowclose`, which
	// destroys the window behind the client's back: the first run of this test
	// used it and got "GdkWindow unexpectedly destroyed" and no dialog, because
	// nothing had asked the application anything.
	xdo(t, app.display, "key", "--clearmodifiers", "alt+F4")

	dialog := waitForWindow(t, app.display, "Unsaved changes")
	t.Logf("the shell asked before closing: dialog window %s", dialog)
	capture(t, app.display, filepath.Join(app.shots, "close-dialog.png"))

	// Escape dismisses a GTK message dialog without answering it, which is the
	// case that must fail closed: an unanswered question is not permission to
	// throw the work away.
	xdo(t, app.display, "windowactivate", "--sync", dialog)
	xdo(t, app.display, "key", "--clearmodifiers", "Escape")
	app.transcript.waitFor(t, "close refused: unsaved changes",
		"the window closed, or refused to close without saying so")
	assertWindowStillThere(t, app.display, "Notrios",
		"the application closed while it was holding unsaved work")

	// The other half, and the one a regression here would break most rudely: a
	// window with nothing unsaved must close without asking anything. Undo puts
	// the note back to what it was loaded with, which is what the frontend
	// compares against, and the transcript says when that has happened.
	xdo(t, app.display, "windowactivate", "--sync", app.window)
	app.clickIntoTheEditor(t)
	undoUntilClean(t, app.display, app.transcript)

	// Through File -> Quit's accelerator this time, which is the other way into
	// the same handler: openbox's Close arrives as the delete-event, and this
	// arrives through runtime.Quit. A guard that only covered one of them would
	// leave the other able to discard the work.
	xdo(t, app.display, "key", "--clearmodifiers", "ctrl+q")
	waitForWindowGone(t, app.display, "Notrios")
}

// TestDesktopCloseProceedsWhenAskedTo drives the answer the other test does not.
//
// A guard that can refuse but never accept is worse than no guard: the window
// becomes impossible to close while it holds unsaved work, with no way out but
// killing the process. The answer comes back from GTK as a string, so what
// counts as yes is a fact about the toolkit rather than a choice this code
// gets to make -- which is exactly the kind of assumption that has to be run
// rather than read.
//
// The dialog promises the changes will be waiting next time rather than
// offering to discard them, because that is what the storage does;
// TestDesktopKeepsTheDraftAcrossACrash is where that promise is checked.
func TestDesktopCloseProceedsWhenAskedTo(t *testing.T) {
	app := launchDesktopApp(t, "gui-close-yes")
	app.typeIntoTheEditor(t, "work that outlives the window")
	app.transcript.waitFor(t, "editor: unsaved changes present",
		"the window never told the shell it was holding unsaved work")

	xdo(t, app.display, "key", "--clearmodifiers", "alt+F4")
	dialog := waitForWindow(t, app.display, "Unsaved changes")

	// Alt+Y is the mnemonic on GTK's stock Yes button. Pressing Return instead
	// would depend on which button the dialog gave focus to, which is a
	// property of the theme rather than of anything under test.
	xdo(t, app.display, "windowactivate", "--sync", dialog)
	xdo(t, app.display, "key", "--clearmodifiers", "alt+y")

	waitForWindowGone(t, app.display, "Notrios")
	if !app.transcript.contains("closed with unsaved changes, kept for the next start") {
		t.Fatalf("the window closed without recording that it was asked to. It said:\n%s",
			app.transcript.String())
	}
}

// TestDesktopKeepsTheDraftAcrossACrash checks the promise the manual makes.
//
// docs/gui.md tells the reader that if the application or the machine goes away
// without asking, the text they had not saved is there when they start again.
// That rests on the webview's storage surviving a restart, which is a property
// of WebKitGTK's default web context rather than of anything in this codebase
// -- exactly the kind of thing that should be run rather than read. The
// application is killed outright, with no chance to save anything on the way
// out, and started again against the same library and the same storage.
//
// The signal is the window reporting unsaved work moments after a cold start.
// Nothing else makes a fresh window dirty: it opens on the new-note template,
// which is what the draft check compares against.
func TestDesktopKeepsTheDraftAcrossACrash(t *testing.T) {
	app := launchDesktopApp(t, "gui-draft-crash")
	app.typeIntoTheEditor(t, "notes typed but never saved")
	app.transcript.waitFor(t, "editor: unsaved changes present",
		"the window never told the shell it was holding unsaved work")

	// Killing the moment the window reports the change tests nothing about
	// recovery: the draft is written on a short delay and WebKit flushes its
	// storage on its own schedule, so the first version of this test killed the
	// application before the text had ever been written and concluded that
	// nothing survives. What has to be true before a kill is that the draft is
	// on the disk, so that is what is waited for -- not a duration somebody
	// guessed.
	waitForDraftOnDisk(t, app)

	app.crash(t)
	app.start(t)

	app.transcript.waitFor(t, "editor: unsaved changes present",
		"the unsaved note did not come back after the application was killed")
	capture(t, app.display, filepath.Join(app.shots, "restored-draft.png"))
}

// waitForDraftOnDisk blocks until the webview has written the draft out.
//
// WebKitGTK keeps localStorage in a SQLite database under the data directory.
// This looks for the key's bytes in that file and its write-ahead log rather
// than opening it: the question is only whether the write has left the
// process, and a reader that understood the format would be a second thing to
// keep right for no more answer than this gives.
func waitForDraftOnDisk(t *testing.T, app *desktopApp) {
	t.Helper()
	storage := filepath.Join(app.dataHome, "notrios", "localstorage")
	deadline := time.Now().Add(60 * time.Second)
	for {
		entries, _ := filepath.Glob(filepath.Join(storage, "*.localstorage*"))
		for _, entry := range entries {
			content, err := os.ReadFile(entry)
			if err == nil && strings.Contains(string(content), "notrios.draft.v1") {
				t.Logf("the draft reached %s", entry)
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the draft never reached the webview storage under %s", storage)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// desktopApp is a running Wails application on a virtual display.
type desktopApp struct {
	display    string
	window     string
	shots      string
	transcript *liveLog
	binary     string
	config     string
	// The webview's own storage, which is where the unsaved draft lives.
	// WebKitGTK's default context keeps website data under the user's data
	// directory, so a test that reads or writes a draft would otherwise share
	// -- and change -- the storage of the person running it.
	dataHome string
	// Extra environment the application is started with, for state that lives
	// outside the library. Publication profiles are the case: they are a file
	// beside the library rather than rows in it, and a journey that photographs
	// the publish panel needs one to exist.
	extraEnv []string
	process  *exec.Cmd
}

// launchDesktopApp starts the real application on its own display and waits
// until it has painted. Every check that made the import test trustworthy is
// here: the tools, the built binary, the built assets, and a window that has
// actually drawn something rather than one that merely exists.
func launchDesktopApp(t *testing.T, name string) *desktopApp {
	t.Helper()
	return launchDesktopAppOn(t, name, 1400, 900, false)
}

// launchDesktopAppOn is the same with the screen size named and the library
// optionally filled. The journey capture wants both: the same width as a
// browser picture, and something in the library -- an export report reading
// "documents: 0" is a photograph of the feature working on nothing, which is
// how this argument came to exist.
func launchDesktopAppOn(t *testing.T, name string, width, height int, seed bool) *desktopApp {
	t.Helper()
	if os.Getenv("NOTRIOS_GUI_DESKTOP_RUN") != "1" {
		t.Skip("set NOTRIOS_GUI_DESKTOP_RUN=1 to drive the desktop application under Xvfb")
	}
	for _, tool := range []string{"Xvfb", "openbox", "xdotool", "scrot"} {
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
	replica := newSyncReplica(t, cli, name, t.TempDir())
	replica.run(t, "init")
	if seed {
		// The same notes the browser capture uses, so a reader moving between
		// the two halves of the catalogue sees one library rather than two.
		if help := runCLI(t, cli, "seed-help", "--db", replica.db, "--asset-store", replica.assets,
			filepath.Join(repositoryRoot(t), "docs")); help.exitCode != 0 {
			t.Fatalf("seed-help exited %d: %s", help.exitCode, help.stderr)
		}
		seedJourneyFixtures(t, cli, replica)
	}
	app := &desktopApp{
		binary:   desktop,
		config:   g18eConfig(t, replica, unusedLoopbackAddress(t), "", true, webDir),
		display:  startVirtualDisplaySized(t, width, height),
		shots:    t.TempDir(),
		dataHome: t.TempDir(),
	}
	if seed {
		// One profile, so the panel chooses it and the journey does not have to
		// drive a dropdown by keystroke. Its selection is a tag the fixtures
		// use, so the review has something to count and something to withhold.
		profiles := filepath.Join(t.TempDir(), "publication-profiles.json")
		saved := runCLIInEnv(t, t.TempDir(), cli, []string{"NOTRIOS_PUBLISH_PROFILES=" + profiles},
			"publish", "profile", "save", "--name", "Field notes",
			"--tags", "field/dusk", "--private-tags", "place/hide",
			"--description", "Dusk field notes, without the hide locations.")
		if saved.exitCode != 0 {
			t.Fatalf("publish profile save exited %d: %s", saved.exitCode, saved.stderr)
		}
		app.extraEnv = append(app.extraEnv, "NOTRIOS_PUBLISH_PROFILES="+profiles)
	}
	app.start(t)
	return app
}

// start runs the application and waits until its window has drawn something.
// The window exists long before the webview has painted, and under software
// rendering the gap is around ten seconds: the first run of the import test
// typed into a blank window and waited ninety seconds for something that had
// never been asked for.
func (a *desktopApp) start(t *testing.T) {
	t.Helper()
	application := exec.Command(a.binary, "-config", a.config)
	application.Env = append(os.Environ(), "DISPLAY="+a.display, "NOTRIOS_UI_LOG=1",
		"XDG_DATA_HOME="+a.dataHome,
		"WEBKIT_DISABLE_COMPOSITING_MODE=1", "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1")
	application.Env = append(application.Env, a.extraEnv...)
	// A transcript per run. Reusing one would make a line from before a restart
	// answer a question asked after it, which is the same mistake as asserting
	// on a screenshot taken too early.
	a.transcript = &liveLog{}
	application.Stdout = io.MultiWriter(os.Stderr, a.transcript)
	application.Stderr = application.Stdout
	if err := application.Start(); err != nil {
		t.Fatal(err)
	}
	a.process = application
	t.Cleanup(func() {
		_ = application.Process.Kill()
		_, _ = application.Process.Wait()
	})

	a.window = waitForWindow(t, a.display, "Notrios")
	xdo(t, a.display, "windowactivate", "--sync", a.window)
	waitForPaintedWindow(t, a.display, a.shots)
}

// crash kills the application the way a power cut or an OOM would: without
// giving it a chance to run anything on the way out. Nothing that follows may
// depend on the application having been asked to stop.
func (a *desktopApp) crash(t *testing.T) {
	t.Helper()
	if err := a.process.Process.Kill(); err != nil {
		t.Fatalf("could not stop the application: %v", err)
	}
	_, _ = a.process.Process.Wait()
	waitForWindowGone(t, a.display, "Notrios")
}

// clickIntoTheEditor puts the caret in the Markdown editor, which is the third
// of the four panes. Clicked rather than reached by keyboard because synthetic
// Tab does not dispatch a DOM keydown under xdotool -- see
// gui_capture_test.go, which records what that costs. The point is well inside the pane and
// below its toolbar, so it lands in the text area at any of the widths the
// layout settles on.
func (a *desktopApp) clickIntoTheEditor(t *testing.T) {
	t.Helper()
	xdo(t, a.display, "mousemove", "--sync", "760", "520")
	xdo(t, a.display, "click", "1")
}

func (a *desktopApp) typeIntoTheEditor(t *testing.T, text string) {
	t.Helper()
	a.clickIntoTheEditor(t)
	xdo(t, a.display, "type", "--delay", "30", text)
}

// undoUntilClean presses undo until the window reports nothing is unsaved.
//
// A count of undos would be a guess: an editor may group a typed phrase into
// one history entry or into several, and the number is a property of
// CodeMirror's history rather than of anything this test is about. The window
// says when it is clean, so that is what is waited for.
func undoUntilClean(t *testing.T, display string, transcript *liveLog) {
	t.Helper()
	for attempt := 0; attempt < 40; attempt++ {
		if transcript.contains("editor: no unsaved changes") {
			return
		}
		xdo(t, display, "key", "--clearmodifiers", "ctrl+z")
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("the editor never returned to its saved state after 40 undos. The application said:\n%s",
		transcript.String())
}

func assertWindowStillThere(t *testing.T, display, name, complaint string) {
	t.Helper()
	search := exec.Command("xdotool", "search", "--onlyvisible", "--name", name)
	search.Env = append(os.Environ(), "DISPLAY="+display)
	out, err := search.Output()
	if err != nil || len(strings.Fields(string(out))) == 0 {
		t.Fatal(complaint)
	}
}

func waitForWindowGone(t *testing.T, display, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		search := exec.Command("xdotool", "search", "--onlyvisible", "--name", name)
		search.Env = append(os.Environ(), "DISPLAY="+display)
		out, err := search.Output()
		if err != nil || len(strings.Fields(string(out))) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the %q window was still open 30s after a close request with nothing unsaved", name)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
