package paths

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Redact replaces the user's home directory prefix with "~".
//
// Resolved paths are printed at startup, in `notriosctl doctor`, and in error
// messages, and they reach issue reports and support threads. A home directory
// contains a username, which is a small disclosure but a gratuitous one: the
// reader of a diagnostic needs the shape of the path, not who owns it.
//
// This redacts a home directory, not a secret. Nothing in this package ever
// prints key material, and no path here contains any; sync keys are named by
// path, and the path is what gets redacted.
func Redact(path, home string) string {
	home = strings.TrimRight(strings.TrimSpace(home), "/\\")
	if home == "" || path == "" {
		return path
	}
	if path == home {
		return "~"
	}
	for _, separator := range []string{"/", `\`} {
		if strings.HasPrefix(path, home+separator) {
			return "~" + path[len(home):]
		}
	}
	return path
}

// RedactAll redacts every occurrence of the home directory inside free text.
//
// Redact handles a path, where home can only be a prefix. A notice is a
// sentence with paths embedded in it, so prefix replacement misses them --
// which is exactly what happened: the first version of Diagnostics redacted the
// root listing and left the username in the notice explaining why a variable
// had been ignored.
//
// It is not a naive substring replacement. Home must be followed by a separator
// or end there, so /home/alice does not redact the /home/alicia of a different
// user who appears in the same message.
func RedactAll(text, home string) string {
	home = strings.TrimRight(strings.TrimSpace(home), "/\\")
	if home == "" || text == "" {
		return text
	}
	for _, separator := range []string{"/", `\`} {
		text = strings.ReplaceAll(text, home+separator, "~"+separator)
	}
	if strings.HasSuffix(text, home) {
		text = strings.TrimSuffix(text, home) + "~"
	}
	return text
}

// Diagnostics renders the resolution for a human, with home redacted.
func (r Resolution) Diagnostics(home string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "mode: %s\n", r.Mode)

	names := make([]string, 0, len(r.Roots))
	for name := range r.Roots {
		names = append(names, name)
	}
	sort.Strings(names)

	width := 0
	for _, name := range names {
		if len(name) > width {
			width = len(name)
		}
	}
	for _, name := range names {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, name, Redact(r.Roots[name], home))
	}

	if len(r.Notices) > 0 {
		b.WriteString("notices:\n")
		for _, notice := range r.Notices {
			fmt.Fprintf(&b, "  [%s] %s\n", notice.Code, RedactAll(notice.Message, home))
		}
	}
	return b.String()
}

// RuntimeDirIsPrivate reports whether an XDG_RUNTIME_DIR is safe to stage
// plaintext backup material in: it must exist, be a directory, and be
// accessible to nobody but its owner.
//
// Ownership is checked by the caller's ability to stat it plus the mode bits.
// A directory that is group- or world-anything fails, because backup staging
// briefly holds decrypted library contents.
func RuntimeDirIsPrivate(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o077 == 0
}
