package httpapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/renesugar/notrios/internal/paths"
)

// Finding the built web interface.
//
// The GUI used to look in exactly one place: `web/dist`, relative to the
// **process working directory**. That works from a checkout root and nowhere
// else, so `bin/notrios` launched from `bin/` opened a window containing a JSON
// error — and the error told the reader to build assets that were already
// built, which is wrong in precisely the case it fired.
//
// Two rules replace it. The search order is explicit and reported, and a caller
// that finds nothing is told every directory that was tried rather than being
// left to guess which one the process considered "here".

// WebRootCandidates lists, in order, the directories a web root is looked for.
//
// `explicit` comes from `--web-dir` or the configuration and, when set, is the
// only candidate — an explicit answer that is wrong should fail loudly rather
// than fall through to a directory that happens to work.
//
// Otherwise: the working directory first, because a developer running from a
// checkout means the checkout they are standing in. Then the executable's own
// directory and its parent, which is what makes `bin/notrios` work whether it
// was invoked as `./bin/notrios` from the root or as `./notrios` from `bin/`.
func WebRootCandidates(explicit string) []string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return []string{filepath.Clean(trimmed)}
	}

	// The installed assets first, then the executable's own tree, and the
	// working directory only in a checkout.
	//
	// The working directory used to come first, unconditionally. That is right
	// for a developer standing in a checkout and wrong once the binary is
	// installed: the HTML, CSS and JavaScript loaded into the application's own
	// window were taken from a web/dist beside wherever the user happened to
	// be, which is a content-injection path gated on nothing but the current
	// directory.
	resolution, err := paths.ForProcess(nil)
	sourceMode := err == nil && resolution.Mode == paths.ModeSource

	candidates := []string{}
	if err == nil && !sourceMode {
		// In an installed layout the program assets root holds the interface
		// in a web/ subdirectory. In source mode that root *is* web/dist, and
		// is added below as the working-directory candidate instead.
		if assets := strings.TrimSpace(resolution.Root(paths.RootProgramAssets)); assets != "" {
			candidates = append(candidates, filepath.Join(assets, "web"))
		}
	}

	if executable, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		dir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(dir, "web", "dist"),
			filepath.Join(filepath.Dir(dir), "web", "dist"),
		)
	}

	if sourceMode {
		candidates = append(candidates, filepath.Clean("web/dist"))
	}
	return candidates
}

// ResolveWebRoot returns the first candidate holding an index.html.
//
// The error names every directory tried, deduplicated: running from the
// executable's own directory makes the working-directory and executable
// candidates the same place, and a message listing one directory twice reads
// as a bug in the message rather than a fact about the search.
//
// "It is not here" and "here is where I looked" are different messages, and
// only the second one is actionable.
func ResolveWebRoot(explicit string) (string, error) {
	searched := []string{}
	seen := map[string]bool{}
	for _, candidate := range WebRootCandidates(explicit) {
		absolute := candidate
		if resolved, err := filepath.Abs(candidate); err == nil {
			absolute = resolved
		}
		if seen[absolute] {
			continue
		}
		seen[absolute] = true
		searched = append(searched, absolute)
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate, nil
		}
	}
	return "", &WebRootNotFoundError{Searched: searched, Explicit: strings.TrimSpace(explicit) != ""}
}

// WebRootNotFoundError reports where the interface was looked for.
type WebRootNotFoundError struct {
	Searched []string
	// Explicit is true when a --web-dir or configured path was given, which
	// changes the advice: the path is wrong, not the build.
	Explicit bool
}

func (e *WebRootNotFoundError) Error() string {
	var b strings.Builder
	b.WriteString("the built web interface was not found.\nLooked in:\n")
	for _, dir := range e.Searched {
		fmt.Fprintf(&b, "  %s\n", dir)
	}
	if e.Explicit {
		b.WriteString("\nThat directory came from --web-dir or server.web_dir. " +
			"Point it at a directory containing index.html, or unset it to search the default locations.")
		return b.String()
	}
	b.WriteString("\nBuild it with `make web` (or `make gui`, which implies it), " +
		"then run from the repository checkout — or pass --web-dir /path/to/web/dist.")
	return b.String()
}
