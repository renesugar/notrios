// A Wails v3 shell for Notrios, built to answer one question: can the real
// window move to v3 without giving anything up?
//
// It is deliberately a mirror of cmd/notrios/gui_wails.go rather than a fresh
// application. Every capability the production shell uses is reproduced here on
// the v3 API — the same menu with the same accelerators, the same two bound
// objects, the same http.Handler serving the real frontend and the real
// /api/v1, the same close veto over unsaved work — because what the spike has
// to measure is the migration, not v3's sample app.
//
// It is disposable. Nothing in the repository builds it, imports it, or
// depends on it, and deleting the directory is the whole of the rollback.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowState is the production shell's unsaved-work guard, reproduced. The
// frontend calls SetUnsavedChanges; the shell asks before the window closes.
type WindowState struct{ unsaved bool }

func (w *WindowState) SetUnsavedChanges(dirty bool) { w.unsaved = dirty }
func (w *WindowState) HasUnsavedChanges() bool      { return w.unsaved }

// NativeUIBridge is the other bound object: the work that cannot go over HTTP
// because it names a folder on this machine.
type NativeUIBridge struct{ app *application.App }

func (b *NativeUIBridge) ChooseDirectory(purpose string) (string, error) {
	dialog := b.app.Dialog.OpenFile()
	dialog.SetTitle("Choose a folder for " + purpose)
	dialog.CanChooseDirectories(true)
	dialog.CanChooseFiles(false)
	return dialog.PromptForSingleSelection()
}

// askAboutUnsavedWork is the production shell's close veto on the v3 API, and
// it is the piece that does not port across as a rename.
//
// v2's runtime.MessageDialog *returns the button that was pressed*, so
// OnBeforeClose reads the answer and returns true to keep the window open. v3's
// MessageDialog.Show() returns nothing: each button carries an OnClick, so the
// answer arrives on a callback after Show has returned. A veto that has to
// decide now therefore has to wait for that callback itself.
//
// It fails closed, exactly as production does: no answer means the window
// stays, because the cost of that is one more click and the cost of the other
// choice is somebody's work.
func askAboutUnsavedWork(app *application.App) bool {
	answered := make(chan bool, 1)
	dialog := app.Dialog.Question()
	dialog.SetTitle("Unsaved changes")
	dialog.SetMessage("This note has changes that have not been saved.\n\n" +
		"Close Notrios anyway? The changes will be waiting when you open it again.")
	yes := dialog.AddButton("Yes")
	yes.OnClick(func() { answered <- true })
	no := dialog.AddButton("No")
	no.OnClick(func() { answered <- false })
	dialog.SetDefaultButton(no)
	dialog.SetCancelButton(no)
	dialog.Show()
	select {
	case allow := <-answered:
		return allow
	case <-time.After(2 * time.Minute):
		return false
	}
}

func main() {
	assets := flag.String("assets", "", "directory holding the built frontend (web/dist)")
	dataDir := flag.String("data", "", "disposable library directory for the spike")
	probe := flag.String("probe", "", "write a capability report to this file and exit before the window opens")
	flag.Parse()

	state := &WindowState{}
	bridge := &NativeUIBridge{}

	// The same handler production uses, not a stand-in. A nested module under
	// the same import-path prefix can reach internal/, because Go's internal
	// rule is about paths rather than modules -- so the spike serves the real
	// frontend and the real /api/v1 through the real service, and what it
	// measures is the migration rather than a sample app.
	//
	// v2 took this as assetserver.Options{Handler}; v3 takes it as
	// AssetOptions{Handler}. The field moved and the type did not.
	cfg := config.Default()
	cfg.Data.Directory = *dataDir
	cfg.Data.DatabasePath = filepath.Join(*dataDir, "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(*dataDir, "assets")
	cfg.Server.WebDir = *assets
	svc, err := service.New(cfg)
	if err != nil {
		log.Fatalf("the spike could not open a library: %v", err)
	}
	defer svc.Close()
	handler := svc.Handler

	var globalApp *application.App
	app := application.New(application.Options{
		Name:        "Notrios",
		Description: "Wails v3 migration spike",
		Services: []application.Service{
			application.NewService(state),
			application.NewService(bridge),
		},
		Assets: application.AssetOptions{Handler: handler},
		// v2 had no equivalent. Two Notrios profiles are two processes today
		// and this would make that a decision rather than an accident.
		SingleInstance: nil,
		ShouldQuit: func() bool {
			// The production shell vetoes a close over unsaved work. In v2 this
			// is OnBeforeClose returning true to keep the window open; here it
			// is ShouldQuit returning false -- and the dialog it asks first has
			// a different shape, which askAboutUnsavedWork explains.
			if !state.HasUnsavedChanges() {
				return true
			}
			return askAboutUnsavedWork(globalApp)
		},
	})
	bridge.app = app
	globalApp = app

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Notrios",
		Width:  1400,
		Height: 900,
	})

	buildMenu(app, window)

	if *probe != "" {
		report := fmt.Sprintf("capabilities: services=%d menu=yes assets=handler shouldquit=yes window=%dx%d\n",
			2, 1400, 900)
		if err := os.WriteFile(filepath.Clean(*probe), []byte(report), 0o644); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
