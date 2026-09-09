// The Wails v3 spike lives in its own module on purpose.
//
// H10's boundary is that the production shell does not migrate and the
// production build does not gain a v3 dependency. A nested module is how that
// is enforced rather than promised: `go build ./...` and `go test ./...` in the
// repository root do not see this directory, and the root go.mod never learns
// that wails/v3 exists.
//
// The module path stays under github.com/renesugar/notrios/ because Go's
// internal-package rule is about import paths rather than modules, so a
// prototype named this way can import internal/service and serve the real
// handler. A spike that served a stub would be measuring a stub.
module github.com/renesugar/notrios/performance/v0.8-h10/prototype

go 1.25.0

require (
	github.com/renesugar/notrios v0.0.0
	github.com/wailsapp/wails/v3 v3.0.0-beta.18
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/zalando/go-keyring v0.2.8 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
)

replace github.com/renesugar/notrios => ../../..
