package paths

import "strings"

// Path arithmetic that does not depend on the host.
//
// The resolver must answer "where would Notrios put this on Windows?" while
// running on Linux, because that is the only way the Windows and macOS rules
// are testable at all before there is a Windows or macOS build. `path/filepath`
// cannot do that: it is compiled for one separator. So joining, cleaning and
// absoluteness are parameterized by target OS here, and the rest of the package
// never touches `filepath`.

const (
	// Linux names every Unix that is not macOS. XDG applies here and only here.
	Linux = "linux"
	// Windows uses the native AppData locations and ignores XDG.
	Windows = "windows"
	// Darwin uses ~/Library and ignores XDG. runtime.GOOS spells it "darwin";
	// H3's tables spell it "macos". NormalizeOS accepts both.
	Darwin = "darwin"
)

// NormalizeOS maps the spellings that reach this package onto the three it
// understands. H3's evidence says "macos" and Go says "darwin"; a resolver that
// silently treated one of them as an unknown OS would be worse than one that
// refuses, because it would fall through to the Linux branch and put a Mac
// user's library in ~/.config.
func NormalizeOS(name string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "linux":
		return Linux, true
	case "windows":
		return Windows, true
	case "darwin", "macos", "mac os x", "osx":
		return Darwin, true
	default:
		return "", false
	}
}

func separator(goos string) string {
	if goos == Windows {
		return `\`
	}
	return "/"
}

// splitVolume returns a Windows drive or UNC prefix and the rest of the path.
// On other systems the volume is always empty.
func splitVolume(goos, p string) (volume, rest string) {
	if goos != Windows {
		return "", p
	}
	if len(p) >= 2 && p[1] == ':' && isASCIILetter(p[0]) {
		return p[:2], p[2:]
	}
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
		// \\server\share is the smallest meaningful UNC root; anything shorter
		// is malformed and is left for the caller's own validation.
		trimmed := strings.TrimLeft(p, `\/`)
		parts := strings.FieldsFunc(trimmed, func(r rune) bool { return r == '\\' || r == '/' })
		if len(parts) >= 2 {
			return `\\` + parts[0] + `\` + parts[1], strings.TrimPrefix(trimmed, parts[0]+`\`+parts[1])
		}
	}
	return "", p
}

func isASCIILetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// IsAbs reports whether p is absolute for the target OS.
//
// The Windows rule follows Python's ntpath, which is what H3's model used: a
// drive prefix is optional, and what makes a path absolute is a leading
// separator after it.
func IsAbs(goos, p string) bool {
	if goos != Windows {
		return strings.HasPrefix(p, "/")
	}
	_, rest := splitVolume(goos, p)
	return strings.HasPrefix(rest, `\`) || strings.HasPrefix(rest, "/")
}

// Clean resolves "." and ".." lexically and collapses repeated separators.
//
// Lexical resolution is the right kind here even though it disagrees with the
// kernel when a component is a symlink: these paths are being *constructed*,
// not inspected, and none of the components exist yet. Containment decisions
// about paths that do exist are the purge oracle's job, and it resolves
// symlinks for exactly that reason.
func Clean(goos, p string) string {
	if p == "" {
		return "."
	}
	sep := separator(goos)
	volume, rest := splitVolume(goos, p)
	rooted := IsAbs(goos, p)

	fields := strings.FieldsFunc(rest, func(r rune) bool {
		return r == '/' || (goos == Windows && r == '\\')
	})

	out := make([]string, 0, len(fields))
	for _, field := range fields {
		switch field {
		case ".":
			// Nothing: "./a" and "a" name the same place.
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else if !rooted {
				// A relative path may legitimately climb above itself.
				out = append(out, "..")
			}
			// An absolute path cannot climb above its root, so ".." there is
			// dropped rather than kept: /.. is /.
		default:
			out = append(out, field)
		}
	}

	joined := strings.Join(out, sep)
	switch {
	case rooted:
		return volume + sep + joined
	case volume != "":
		return volume + joined
	case joined == "":
		return "."
	default:
		return joined
	}
}

// Join concatenates parts with the target separator and cleans the result.
//
// Cleaning here rather than at the call sites is deliberate. H3's model joined
// without cleaning and produced roots containing "..", which H3's own purge
// oracle refuses as not being in normal form — so a portable installation could
// not have been purged. A resolver that can emit a path the rest of the system
// rejects has not finished its job.
func Join(goos string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return Clean(goos, strings.Join(kept, separator(goos)))
}
