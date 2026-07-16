//go:build gui

package main

import (
	"context"
	"net/http"

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
func runGUI(handler http.Handler) error {
	var appCtx context.Context

	appMenu := menu.NewMenu()
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Reload", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		if appCtx != nil {
			runtime.WindowReloadApp(appCtx)
		}
	})
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
	helpMenu.AddText("Notrios Help", nil, func(_ *menu.CallbackData) {
		if appCtx != nil {
			// The Help notebook holds the offline documentation. Set a flag
			// before dispatching so a click that lands before the React app
			// has mounted is picked up on mount instead of being lost.
			runtime.WindowExecJS(appCtx, `window.__notriosOpenHelp = Date.now(); window.dispatchEvent(new CustomEvent("notrios:open-help"));`)
		}
	})

	return wails.Run(&options.App{
		Title:  "Notrios",
		Width:  1400,
		Height: 900,
		Menu:   appMenu,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
		},
	})
}
