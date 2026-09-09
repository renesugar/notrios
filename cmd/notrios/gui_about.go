//go:build gui

package main

import (
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/renesugar/notrios/internal/version"
)

// BuildInfo describes the binary a person is actually running.
//
// It exists for the ordinary reason an About box exists: when somebody reports
// that something is wrong, the first useful question is which build they have,
// and "0.7.0" does not answer it for anyone running between releases. The
// revision, the build time and whether the tree was modified do.
//
// Nothing here is read from configuration or from the service. It is compiled
// in, by the toolchain, from the repository the binary was built in --
// debug.ReadBuildInfo reports vcs.revision, vcs.time and vcs.modified for any
// ordinary `go build`, so this needs no linker flags and cannot fall out of
// step with a Makefile.
//
// It is on the native bridge rather than the HTTP API deliberately. This
// describes *this application*, and in -gui-only mode the window is showing a
// service on another machine whose binary it knows nothing about. A REST field
// would have to answer for the service instead, which is a different question
// and not the one an About box is asked.
type BuildInfo struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	BuiltAt   string `json:"built_at"`
	Modified  bool   `json:"modified"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Tags      string `json:"tags"`
}

// About reports the build. The interface asks for it when the About dialog
// opens, and an automated desktop run reads the same values off the clipboard
// through that dialog's Copy button -- which is the only channel by which the
// running application can tell a test harness a fact in words rather than in
// pixels, and the one thing that would have caught a stale binary being tested
// twice while this was written.
func (b *NativeUIBridge) About() BuildInfo {
	built := BuildInfo{
		Version:   version.Version,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return built
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			built.Revision = setting.Value
		case "vcs.time":
			built.BuiltAt = setting.Value
		case "vcs.modified":
			built.Modified = setting.Value == "true"
		case "-tags":
			built.Tags = setting.Value
		}
	}
	if built.Revision == "" && info.Main.Version != "" {
		// A binary built outside a checkout still reports a module version,
		// which is less useful but better than an empty line in a bug report.
		built.Revision = strings.TrimSpace(info.Main.Version)
	}
	return built
}
