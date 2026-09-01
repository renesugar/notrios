// Package paths is the single answer to "where does this file go?".
//
// Before H4 the question was answered in twenty-five places, and H3 found that
// the interesting defects were all disagreements between two of them rather
// than mistakes inside any one: `internal/profiles` and `internal/synckeys`
// resolved the config root by different rules and gave different answers when
// XDG_CONFIG_HOME was relative. One resolver exists so that cannot recur.
//
// The contract is H3's, recorded in performance/v0.8-h3/LAYOUT.json and its
// generated RESOLUTION_TABLE.json. TestReproducesH3ResolutionTable asserts this
// package reproduces that table exactly, which is the plan's binding
// requirement on H4 and the reason the table is a file rather than a document.
//
// Precedence, restated here because H4 is bound by it:
//
//	explicit  a path given on the command line or in an explicitly named config
//	portable  a marker file beside the executable
//	installed the native per-OS roots
//	source    a checkout detected beside the executable or working directory
//
// Portable mode is never inferred — not from the current directory, not from
// that directory being writable, not from installed roots being absent.
// Guessing portability from writability makes a USB stick and a home directory
// the same decision, and the wrong guess writes a library somewhere the user
// will not find it.
package paths

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Mode is which of the four layouts was selected.
type Mode string

const (
	ModeInstalled Mode = "installed"
	ModePortable  Mode = "portable"
	ModeSource    Mode = "source"
)

// Root names. They are strings rather than an enum because they are also keys
// in H3's JSON evidence, and one spelling shared by both sides is worth more
// than type safety on this particular axis.
const (
	RootConfig        = "config"
	RootData          = "data"
	RootState         = "state"
	RootCache         = "cache"
	RootRuntime       = "runtime"
	RootProgramAssets = "program_assets"
)

// RootNames lists every root, in the order a human would read them.
var RootNames = []string{RootConfig, RootData, RootState, RootCache, RootRuntime, RootProgramAssets}

// PortableMarkerName is the file whose presence beside the executable selects
// portable mode. Nothing else selects it.
const PortableMarkerName = "notrios-portable.txt"

// Notice is something the resolver decided that the user should be able to see.
//
// Notices carry a Code as well as a Message because they are compared across
// implementations: H3's model is Python and this is Go, and comparing English
// prose between them would fail on the quoting convention rather than on the
// behaviour. The code is the contract; the message is for a person.
type Notice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Notice codes.
const (
	NoticeXDGRelativeIgnored   = "xdg_relative_ignored"
	NoticeXDGIgnoredOnPlatform = "xdg_ignored_on_platform"
	NoticeRuntimeDirUnset      = "runtime_dir_unset"
	NoticeRuntimeDirRelative   = "runtime_dir_relative"
	NoticeRuntimeDirNotPrivate = "runtime_dir_not_private"
	NoticePortableSelected     = "portable_selected"
	NoticeSourceSelected       = "source_selected"
	NoticeExplicitOverride     = "explicit_override"
)

// Resolution is the answer: which layout, which roots, and what the user should
// be told about how it was decided.
type Resolution struct {
	Mode    Mode              `json:"mode"`
	Roots   map[string]string `json:"roots"`
	Notices []Notice          `json:"notices"`
}

// Root returns one resolved root, or "" if the name is not a root.
func (r Resolution) Root(name string) string { return r.Roots[name] }

// Codes returns the notice codes in order, which is what cross-implementation
// comparisons assert against.
func (r Resolution) Codes() []string {
	codes := make([]string, 0, len(r.Notices))
	for _, notice := range r.Notices {
		codes = append(codes, notice.Code)
	}
	return codes
}

// Options describes the environment to resolve against. The zero value resolves
// against this process on this machine; every field exists so a test can ask
// about a machine it is not running on.
type Options struct {
	// Env is consulted instead of the process environment when non-nil. A nil
	// map means "read the real environment"; an empty non-nil map means "there
	// are no variables", which is a different and testable situation.
	Env map[string]string
	// GOOS is the target operating system; empty means runtime.GOOS.
	GOOS string
	// ExecutableDir is the directory holding the running binary; empty means
	// ask the operating system.
	ExecutableDir string
	// PortableMarker forces portable mode. Callers normally leave this false
	// and let DetectPortable look for the marker file.
	PortableMarker bool
	// SourceCheckout selects source mode.
	SourceCheckout bool
	// Explicit maps root names to paths given by the user. These win over
	// everything, and a relative one is refused rather than resolved.
	Explicit map[string]string
	// RuntimeDirIsPrivate reports whether XDG_RUNTIME_DIR is owner-only. Nil
	// means do not check, which is what the pure-model comparisons want.
	RuntimeDirIsPrivate func(string) bool
}

func (o Options) lookup(key string) string {
	if o.Env != nil {
		return strings.TrimSpace(o.Env[key])
	}
	return strings.TrimSpace(os.Getenv(key))
}

// Resolve computes the roots.
//
// It never creates a directory and never touches the filesystem except through
// RuntimeDirIsPrivate. Deciding where things go and making them are separate
// acts: a diagnostic command has every right to ask the first question without
// causing the second.
func Resolve(options Options) (Resolution, error) {
	goos := options.GOOS
	if strings.TrimSpace(goos) == "" {
		goos = runtime.GOOS
	}
	normalized, ok := NormalizeOS(goos)
	if !ok {
		return Resolution{}, fmt.Errorf("unsupported operating system %q", goos)
	}
	goos = normalized

	result := Resolution{Mode: ModeInstalled, Roots: map[string]string{}, Notices: []Notice{}}

	switch {
	case options.PortableMarker:
		resolvePortable(goos, options, &result)
	case options.SourceCheckout:
		if err := resolveSource(goos, options, &result); err != nil {
			return Resolution{}, err
		}
	default:
		if err := resolveInstalled(goos, options, &result); err != nil {
			return Resolution{}, err
		}
	}

	if err := applyExplicit(goos, options, &result); err != nil {
		return Resolution{}, err
	}
	return result, nil
}

func (r *Resolution) note(code, format string, args ...any) {
	r.Notices = append(r.Notices, Notice{Code: code, Message: fmt.Sprintf(format, args...)})
}

func resolvePortable(goos string, options Options, result *Resolution) {
	result.Mode = ModePortable
	executable := options.ExecutableDir
	base := Join(goos, executable, "..", "notrios-data")
	for _, name := range RootNames {
		result.Roots[name] = Join(goos, base, name)
	}
	result.Roots[RootProgramAssets] = Join(goos, executable, "..", "share", "notrios")
	result.note(NoticePortableSelected,
		"portable mode: selected by the %s marker beside %s", PortableMarkerName, executable)
}

// resolveSource is a checkout: a developer's working copy.
//
// Source mode moves the program's own files and the developer's scratch data
// into the checkout. It deliberately does *not* move the config root.
//
// That distinction was a correction: the first version overrode config too, and
// the profile registry promptly relocated into the checkout's config/
// directory. A checkout is not a different user. The registry and sync keys are
// the developer's identity across every build they run, they have always lived
// in the user config root, and writing them into the source tree would put a
// file naming every local database path one `git add -A` away from being
// committed. NOTRIOS_PROFILE_REGISTRY already exists for anyone who does want
// an isolated registry.
func resolveSource(goos string, options Options, result *Resolution) error {
	// The native roots are resolved first because config is taken from them.
	if err := resolveInstalled(goos, options, result); err != nil {
		return err
	}
	result.Mode = ModeSource
	for _, name := range []string{RootData, RootState, RootCache, RootRuntime} {
		result.Roots[name] = Join(goos, ".", "data", name)
	}
	result.Roots[RootProgramAssets] = Join(goos, ".", "web", "dist")
	result.note(NoticeSourceSelected,
		"source mode: a checkout was detected, so data and assets are checkout-relative; config stays in the user config root")
	return nil
}

// xdg applies the XDG rules to one variable.
//
// Empty is unset. Relative is invalid: the specification says to ignore it, and
// ignoring it is also the only safe reading, because the alternative is
// resolving a user's database location against whatever directory the process
// happened to start in. Go's own os.UserConfigDir refuses a relative value for
// the same reason; before H4, internal/profiles accepted one.
func xdg(goos string, options Options, variable, fallback string, result *Resolution) string {
	raw := options.lookup(variable)
	if raw == "" {
		return fallback
	}
	if !IsAbs(goos, raw) {
		result.note(NoticeXDGRelativeIgnored,
			"%s is %q, which is relative; the specification requires an absolute path, so it was ignored and %s used instead",
			variable, raw, fallback)
		return fallback
	}
	return raw
}

var xdgVariables = []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"}

func resolveInstalled(goos string, options Options, result *Resolution) error {
	home := options.lookup("HOME")
	if home == "" {
		home = options.lookup("USERPROFILE")
	}
	if home == "" {
		return fmt.Errorf(
			"no home directory is set, so no user root can be resolved; " +
				"pass explicit paths rather than falling back to the working directory")
	}

	switch goos {
	case Linux:
		result.Roots[RootConfig] = Join(goos, xdg(goos, options, "XDG_CONFIG_HOME", Join(goos, home, ".config"), result), "notrios")
		result.Roots[RootData] = Join(goos, xdg(goos, options, "XDG_DATA_HOME", Join(goos, home, ".local", "share"), result), "notrios")
		result.Roots[RootState] = Join(goos, xdg(goos, options, "XDG_STATE_HOME", Join(goos, home, ".local", "state"), result), "notrios")
		result.Roots[RootCache] = Join(goos, xdg(goos, options, "XDG_CACHE_HOME", Join(goos, home, ".cache"), result), "notrios")
		result.Roots[RootProgramAssets] = "/usr/local/share/notrios"
		resolveLinuxRuntime(goos, options, result)

	case Windows:
		roaming := options.lookup("APPDATA")
		if roaming == "" {
			roaming = Join(goos, home, "AppData", "Roaming")
		}
		local := options.lookup("LOCALAPPDATA")
		if local == "" {
			local = Join(goos, home, "AppData", "Local")
		}
		noteXDGIgnored(goos, options, result, "Windows")
		result.Roots[RootConfig] = Join(goos, roaming, "Notrios", "Config")
		result.Roots[RootData] = Join(goos, local, "Notrios", "Data")
		result.Roots[RootState] = Join(goos, local, "Notrios", "State")
		result.Roots[RootCache] = Join(goos, local, "Notrios", "Cache")
		result.Roots[RootRuntime] = Join(goos, local, "Notrios", "Runtime")
		result.Roots[RootProgramAssets] = Clean(goos, options.ExecutableDir)

	case Darwin:
		support := Join(goos, home, "Library", "Application Support")
		noteXDGIgnored(goos, options, result, "macOS")
		result.Roots[RootConfig] = Join(goos, support, "Notrios", "Config")
		result.Roots[RootData] = Join(goos, support, "Notrios", "Data")
		result.Roots[RootState] = Join(goos, support, "Notrios", "State")
		result.Roots[RootCache] = Join(goos, home, "Library", "Caches", "Notrios")
		temporary := options.lookup("TMPDIR")
		if temporary == "" {
			temporary = Join(goos, home, "Library", "Caches")
		}
		result.Roots[RootRuntime] = Join(goos, temporary, "Notrios", "Runtime")
		result.Roots[RootProgramAssets] = Join(goos, options.ExecutableDir, "..", "Resources")
	}
	return nil
}

// noteXDGIgnored says so once when XDG variables are set on a platform that
// does not use them. A user who exports XDG_CONFIG_HOME for another tool has
// not asked Notrios to leave ~/Library, but they deserve to be told that the
// variable they set had no effect here.
func noteXDGIgnored(goos string, options Options, result *Resolution, platform string) {
	for _, variable := range xdgVariables {
		if options.lookup(variable) != "" {
			result.note(NoticeXDGIgnoredOnPlatform,
				"XDG_* variables are ignored on %s; the native locations are authoritative", platform)
			return
		}
	}
}

// resolveLinuxRuntime implements the one XDG root with no specified fallback.
//
// Inventing one in /tmp would put staged plaintext backups in a
// world-traversable directory, so the fallback is <state>/runtime, created
// owner-only, and the substitution is reported rather than silent.
func resolveLinuxRuntime(goos string, options Options, result *Resolution) {
	fallback := Join(goos, result.Roots[RootState], "runtime")
	raw := options.lookup("XDG_RUNTIME_DIR")
	switch {
	case raw == "":
		result.Roots[RootRuntime] = fallback
		result.note(NoticeRuntimeDirUnset,
			"XDG_RUNTIME_DIR is not set and the specification names no fallback; using %s rather than a shared temporary directory",
			fallback)
	case !IsAbs(goos, raw):
		result.Roots[RootRuntime] = fallback
		result.note(NoticeRuntimeDirRelative,
			"XDG_RUNTIME_DIR is %q, which is relative; using %s", raw, fallback)
	case options.RuntimeDirIsPrivate != nil && !options.RuntimeDirIsPrivate(raw):
		result.Roots[RootRuntime] = fallback
		result.note(NoticeRuntimeDirNotPrivate,
			"%s is not owner-only, so staged backup material would be readable by others; using %s", raw, fallback)
	default:
		result.Roots[RootRuntime] = Join(goos, raw, "notrios")
	}
}

// applyExplicit puts caller-given paths on top of whatever was resolved.
//
// A relative explicit path is refused rather than made absolute. The user named
// a path; resolving it against the working directory would mean the same
// command wrote to different places depending on where it was run, which is the
// behaviour H3 found and H4 exists to remove.
func applyExplicit(goos string, options Options, result *Resolution) error {
	for _, name := range RootNames {
		value, ok := options.Explicit[name]
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !IsAbs(goos, value) {
			return fmt.Errorf(
				"the explicit %s path %q is relative; an explicitly given path is never resolved against the working directory",
				name, value)
		}
		result.Roots[name] = Clean(goos, value)
		result.note(NoticeExplicitOverride, "%s was given explicitly as %s", name, result.Roots[name])
	}
	for name := range options.Explicit {
		if !isRootName(name) {
			return fmt.Errorf("unknown root %q", name)
		}
	}
	return nil
}

func isRootName(name string) bool {
	for _, known := range RootNames {
		if known == name {
			return true
		}
	}
	return false
}
