//go:build !gui

package main

import (
	"errors"
	"net/http"
)

// runGUI without the gui build tag: keep the binary buildable everywhere
// (CI, headless servers) while pointing users at the real build.
func runGUI(http.Handler) error {
	return errors.New("this notrios binary was built without the GUI; rebuild with `make gui` (go build -tags \"gui desktop production webkit2_41\" ./cmd/notrios) (Linux needs libgtk-3-dev and libwebkit2gtk dev packages) or run with -no-gui")
}
