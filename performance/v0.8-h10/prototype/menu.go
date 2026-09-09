package main

import "github.com/wailsapp/wails/v3/pkg/application"

// buildMenu reproduces the production shell's menu, accelerators and all.
//
// The menu is where v2 and v3 differ most in shape: v2 builds a tree with
// menu.NewMenu() and attaches it through options.App.Menu, while v3 builds one
// through the application and sets it. What matters for the migration is not
// which is prettier but whether every item, role, separator and accelerator
// survives — including F11, which is the one an automated desktop run uses to
// get in without clicking a pixel.
func buildMenu(app *application.App, window *application.WebviewWindow) {
	appMenu := app.Menu.New()

	file := appMenu.AddSubmenu("File")
	file.Add("Reload").SetAccelerator("CmdOrCtrl+r").OnClick(func(*application.Context) {
		window.Reload()
	})
	file.Add("Import and export…").SetAccelerator("CmdOrCtrl+i").OnClick(func(*application.Context) {
		window.ExecJS(`window.__notriosOpenTransfer = Date.now(); window.dispatchEvent(new CustomEvent("notrios:open-transfer"));`)
	})
	file.AddSeparator()
	file.Add("Quit").SetAccelerator("CmdOrCtrl+q").OnClick(func(*application.Context) {
		app.Quit()
	})

	appMenu.AddRole(application.EditMenu)

	view := appMenu.AddSubmenu("View")
	view.Add("Toggle Fullscreen").SetAccelerator("F11").OnClick(func(*application.Context) {
		if window.IsFullscreen() {
			window.UnFullscreen()
		} else {
			window.Fullscreen()
		}
	})

	help := appMenu.AddSubmenu("Help")
	help.Add("About Notrios").OnClick(func(*application.Context) {
		window.ExecJS(`window.__notriosOpenAbout = Date.now(); window.dispatchEvent(new CustomEvent("notrios:open-about"));`)
	})
	help.Add("Notrios Help").OnClick(func(*application.Context) {
		window.ExecJS(`window.__notriosOpenHelp = Date.now(); window.dispatchEvent(new CustomEvent("notrios:open-help"));`)
	})

	app.Menu.Set(appMenu)
}
