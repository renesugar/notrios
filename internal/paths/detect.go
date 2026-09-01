package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mode detection. These are the only functions in the package that look at the
// filesystem, and they look for exactly two things: a marker file, and a
// checkout. Neither is inferred from anything else.

// ExecutableDir returns the directory holding the running binary, with symlinks
// resolved. A symlinked launcher in ~/.local/bin pointing at an installed tree
// should find the installed tree's assets, not the launcher's directory.
func ExecutableDir() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	return filepath.Dir(executable), nil
}

// DetectPortable reports whether the portable marker sits beside the
// executable.
//
// Presence of the file is the entire test. It is deliberately not "is this
// directory writable", "is there no installed layout", or "does this look like
// removable media": a USB stick and a home directory are both writable, and
// guessing wrong writes a user's library somewhere they will not find it. If
// someone wants portable mode they say so by creating one file.
func DetectPortable(executableDir string) bool {
	if strings.TrimSpace(executableDir) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(executableDir, PortableMarkerName))
	return err == nil && !info.IsDir()
}

// SourceMarkers are the files whose presence together identify this repository
// as opposed to any other Go checkout. go.mod alone would match every Go
// project on the machine.
var SourceMarkers = []string{"go.mod", "PLAN.md", "AGENTS.md"}

// DetectSourceCheckout reports whether dir is a Notrios checkout.
//
// It requires the module path inside go.mod to match, so a different project
// that happens to have a PLAN.md is not mistaken for this one.
func DetectSourceCheckout(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	for _, marker := range SourceMarkers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
			return false
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), "module github.com/renesugar/notrios")
}

// FindSourceCheckout looks for a checkout at dir and then upward, stopping at
// the filesystem root. Running `go run ./cmd/notriosd` from a subdirectory is
// normal and should still be source mode.
func FindSourceCheckout(dir string) (string, bool) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", false
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if DetectSourceCheckout(absolute) {
			return absolute, true
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			return "", false
		}
		absolute = parent
	}
}

// DetectMode decides which layout applies for a running process, in the
// precedence H3 fixed and H4 is bound by. Explicit paths are applied on top by
// Resolve and are not a mode.
func DetectMode(executableDir, workingDir string) (Mode, string) {
	if DetectPortable(executableDir) {
		return ModePortable, executableDir
	}
	if root, ok := FindSourceCheckout(executableDir); ok {
		return ModeSource, root
	}
	if root, ok := FindSourceCheckout(workingDir); ok {
		return ModeSource, root
	}
	return ModeInstalled, ""
}

// ForProcess resolves roots for the running process: the common entry point.
func ForProcess(explicit map[string]string) (Resolution, error) {
	executableDir, err := ExecutableDir()
	if err != nil {
		// Not fatal. A process that cannot locate its own binary can still be
		// resolved for installed mode, which needs only the environment.
		executableDir = ""
	}
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = ""
	}
	mode, _ := DetectMode(executableDir, workingDir)
	return Resolve(Options{
		ExecutableDir:       executableDir,
		PortableMarker:      mode == ModePortable,
		SourceCheckout:      mode == ModeSource,
		Explicit:            explicit,
		RuntimeDirIsPrivate: RuntimeDirIsPrivate,
	})
}

// Root resolves one root for the running process.
//
// Consumers call this rather than reading environment variables themselves.
// That is the whole point of the package: before H4, internal/profiles and
// internal/synckeys each derived the config root by their own rules and
// disagreed about the answer whenever XDG_CONFIG_HOME was relative.
func Root(name string) (string, error) {
	if !isRootName(name) {
		return "", fmt.Errorf("unknown root %q", name)
	}
	resolution, err := ForProcess(nil)
	if err != nil {
		return "", err
	}
	return resolution.Root(name), nil
}

// ConfigRoot is the directory holding user-authored settings: the service
// configuration, the profile registry, generated profile configs, sync keys.
func ConfigRoot() (string, error) { return Root(RootConfig) }
