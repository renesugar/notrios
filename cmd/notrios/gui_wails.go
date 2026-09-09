//go:build gui

package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/renesugar/notrios/internal/service"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// runGUI opens the Wails window. Every request from the webview — the React
// frontend assets and all /api/v1 fetches — routes through the given handler:
// the in-process service in the default mode, or a reverse proxy to a remote
// service in -gui-only mode. The frontend therefore behaves identically to a
// browser pointed at notriosd.
// NativeUIBridge is the part of the interface that does not go over HTTP.
//
// It is bound only when this process owns the service, which is the whole of
// its access control.
//
// The usual case is that it does: the default mode starts the service in this
// process and the window and the store are the same program. -gui-only is the
// exception, and even it points at 127.0.0.1 unless -remote says otherwise --
// so "the service might be on another machine" is the exception to the
// exception and not the reason this matters.
//
// The reason is that in -gui-only mode this process has no store. It is a
// window and a reverse proxy; the library belongs to a separate notriosd.
// Everything below acts on the store directly, so pointing the same window at a
// local service would not help: the work has to happen in the program that owns
// the database. See gui_transfer.go for why it cannot go over REST instead.
type NativeUIBridge struct {
	ctx     context.Context
	local   *service.Service
	actions *actionLog
}

func (b *NativeUIBridge) chooseDirectory(title string) (string, error) {
	if b == nil || b.ctx == nil {
		return "", errors.New("the native window is not ready")
	}
	return runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{Title: title})
}

// runGUI opens the window. A non-nil local service means this process owns the
// store, and is what binds the native bridge.
func runGUI(handler http.Handler, local *service.Service) error {
	var appCtx context.Context
	// The transcript is always collected and only echoed to the log stream when
	// asked for. Collecting it unconditionally is what makes it useful after
	// the fact: a log somebody has to switch on before reproducing a problem is
	// a log that is empty when the problem first happens.
	actions := &actionLog{enabled: verboseUILog()}
	bridge := &NativeUIBridge{local: local, actions: actions}
	// Bound in both modes; see gui_window_state.go for why that is not a hole
	// in the rule above it.
	unsavedWork := &WindowState{actions: actions}

	appMenu := menu.NewMenu()
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Reload", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		if appCtx != nil {
			runtime.WindowReloadApp(appCtx)
		}
	})
	// Import and export are native-only work -- they name a folder on this
	// machine and go through the bridge below rather than over HTTP -- so the
	// native menu is where they belong. The accelerator also gives an automated
	// desktop run a way in that does not depend on clicking a pixel: see
	// performance/v0.8-h15/desktop_journey.sh.
	if local != nil {
		fileMenu.AddText("Import and export…", keys.CmdOrCtrl("i"), func(_ *menu.CallbackData) {
			if appCtx != nil {
				runtime.WindowExecJS(appCtx, `window.__notriosOpenTransfer = Date.now(); window.dispatchEvent(new CustomEvent("notrios:open-transfer"));`)
			}
		})
	}
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		if appCtx != nil {
			runtime.Quit(appCtx)
		}
	})
	appMenu.Append(menu.EditMenu())
	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Toggle Fullscreen", keys.Key("f11"), func(_ *menu.CallbackData) {
		if appCtx == nil {
			return
		}
		if runtime.WindowIsFullscreen(appCtx) {
			runtime.WindowUnfullscreen(appCtx)
		} else {
			runtime.WindowFullscreen(appCtx)
		}
	})
	helpMenu := appMenu.AddSubmenu("Help")
	// Which build is this? "0.7.0" does not answer it between releases, and it
	// is the first thing worth knowing about a bug report. Bound to the same
	// event channel as the other menu items, so it is also the cheapest proof
	// that channel works.
	if local != nil {
		helpMenu.AddText("About Notrios", nil, func(_ *menu.CallbackData) {
			if appCtx != nil {
				runtime.WindowExecJS(appCtx, `window.__notriosOpenAbout = Date.now(); window.dispatchEvent(new CustomEvent("notrios:open-about"));`)
			}
		})
	}
	helpMenu.AddText("Notrios Help", nil, func(_ *menu.CallbackData) {
		if appCtx != nil {
			// The Help notebook holds the offline documentation. Set a flag
			// before dispatching so a click that lands before the React app
			// has mounted is picked up on mount instead of being lost.
			runtime.WindowExecJS(appCtx, `window.__notriosOpenHelp = Date.now(); window.dispatchEvent(new CustomEvent("notrios:open-help"));`)
		}
	})

	app := &options.App{
		Title:  "Notrios",
		Width:  1400,
		Height: 900,
		Menu:   appMenu,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
			bridge.ctx = ctx
		},
		// The last thing standing between unsaved work and a closed window.
		//
		// Saving here is deliberate rather than continuous (see web/src/draft.ts),
		// so the editor can hold text that is in no store, and the interface asks
		// before anything replaces it. Closing the window is the one path the
		// frontend cannot intercept: `beforeunload` covers a reload and a browser
		// tab, and neither the title bar nor File → Quit goes through it.
		//
		// Both of those paths do go through here -- the GTK delete-event becomes
		// the "Q" message, and runtime.Quit calls the same frontend method -- and
		// both call this from a goroutine, which is what makes it safe to block on
		// a dialog: doing so on the GTK main thread would deadlock against the
		// thread that has to run it.
		OnBeforeClose: func(ctx context.Context) bool {
			if !unsavedWork.hasUnsavedChanges() {
				return false
			}
			// A question dialog on Linux is GTK's Yes/No pair: Wails' Buttons
			// option is not used by that implementation, so the message has to
			// be a question those two words answer.
			//
			// It does not offer to discard the work, because that is not what
			// happens. The draft is kept in the webview's storage, and that
			// storage was measured surviving a killed process, so a window
			// closed here opens again holding the same text. Saying "discard"
			// would be a lie the next start would expose -- and clearing the
			// draft from here would be a race against a process on its way out.
			answer, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
				Type:  runtime.QuestionDialog,
				Title: "Unsaved changes",
				Message: "This note has changes that have not been saved.\n\n" +
					"Close Notrios anyway? The changes will be waiting when you open it again.",
			})
			// Fail closed. If the dialog could not be shown or was dismissed
			// without an answer, the window stays open: the cost of that is a
			// second click, and the cost of the other choice is the person's
			// work.
			if err != nil || answer != "Yes" {
				actions.record("window", "close refused: unsaved changes")
				return true
			}
			actions.record("window", "closed with unsaved changes, kept for the next start")
			return false
		},
	}
	app.Bind = []interface{}{unsavedWork}
	if local != nil {
		app.Bind = append(app.Bind, bridge)
	}
	return wails.Run(app)
}
