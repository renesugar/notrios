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
// its access control: in -gui-only mode the window may be showing a service on
// another machine, where a directory chosen here would name the wrong
// filesystem, so `local` is nil, nothing is bound, and the frontend finds no
// bridge at all. See gui_transfer.go for why import, export and snapshots have
// to come this way rather than over REST.
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
	bridge := &NativeUIBridge{local: local, actions: &actionLog{enabled: verboseUILog()}}

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
	}
	if local != nil {
		app.Bind = []interface{}{bridge}
	}
	return wails.Run(app)
}
