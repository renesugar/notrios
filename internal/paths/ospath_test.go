package paths_test

import (
	"testing"

	"github.com/renesugar/notrios/internal/paths"
)

// The path arithmetic is host-independent so the Windows and macOS rules are
// testable before there is a Windows or macOS build. These cases run on Linux
// and are about the target, not the host.

func TestCleanResolvesDotAndDotDot(t *testing.T) {
	posix := []struct{ in, want string }{
		{"/a/b/../c", "/a/c"},
		{"/a/./b", "/a/b"},
		{"/a//b", "/a/b"},
		{"a/b/../..", "."},
		{"a/../../b", "../b"}, // a relative path may climb above itself
		{"/..", "/"},          // an absolute path may not
		{"/a/b/../../../c", "/c"},
		{"", "."},
		{"/", "/"},
		{"./data/config", "data/config"},
	}
	for _, c := range posix {
		if got := paths.Clean(paths.Linux, c.in); got != c.want {
			t.Errorf("Clean(linux, %q) = %q, want %q", c.in, got, c.want)
		}
	}

	windows := []struct{ in, want string }{
		{`C:\a\b\..\c`, `C:\a\c`},
		{`C:\a\.\b`, `C:\a\b`},
		{`C:\a\\b`, `C:\a\b`},
		{`C:\..`, `C:\`},
		{`a\b\..`, `a`},
		{`C:\Program Files\Notrios`, `C:\Program Files\Notrios`},
	}
	for _, c := range windows {
		if got := paths.Clean(paths.Windows, c.in); got != c.want {
			t.Errorf("Clean(windows, %q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsAbsFollowsTargetRules(t *testing.T) {
	cases := []struct {
		goos, path string
		want       bool
	}{
		{paths.Linux, "/a", true},
		{paths.Linux, "a", false},
		{paths.Linux, "./a", false},
		{paths.Linux, `C:\a`, false}, // a Windows path is not absolute on Linux
		{paths.Windows, `C:\a`, true},
		{paths.Windows, `C:a`, false}, // drive-relative, and genuinely not absolute
		{paths.Windows, `\a`, true},
		{paths.Windows, `a`, false},
		{paths.Darwin, "/Users/u", true},
	}
	for _, c := range cases {
		if got := paths.IsAbs(c.goos, c.path); got != c.want {
			t.Errorf("IsAbs(%s, %q) = %v, want %v", c.goos, c.path, got, c.want)
		}
	}
}

// Join must clean. H3's model did not, and produced roots containing ".." that
// H3's own purge oracle refuses as not being in normal form -- so a portable
// installation could not have been purged. This is that defect, pinned.
func TestJoinCleansSoResolvedRootsAreInNormalForm(t *testing.T) {
	got := paths.Join(paths.Linux, "/media/stick/notrios/bin", "..", "notrios-data", "cache")
	if want := "/media/stick/notrios/notrios-data/cache"; got != want {
		t.Fatalf("Join = %q, want %q", got, want)
	}
	if got := paths.Join(paths.Darwin, "/Applications/N.app/Contents/MacOS", "..", "Resources"); got != "/Applications/N.app/Contents/Resources" {
		t.Fatalf("macOS bundle assets = %q", got)
	}
	if got := paths.Join(paths.Windows, `C:\Program Files\Notrios`, "..", "share"); got != `C:\Program Files\share` {
		t.Fatalf("windows = %q", got)
	}
}

func TestNormalizeOSAcceptsBothSpellingsAndRefusesTheRest(t *testing.T) {
	for _, name := range []string{"darwin", "macos", "MacOS", "  Darwin "} {
		if got, ok := paths.NormalizeOS(name); !ok || got != paths.Darwin {
			t.Errorf("NormalizeOS(%q) = %q, %v", name, got, ok)
		}
	}
	// An unknown OS must refuse rather than fall through to the Linux branch,
	// which would put a user's library in ~/.config on a platform that does not
	// use it.
	for _, name := range []string{"plan9", "freebsd", ""} {
		if _, ok := paths.NormalizeOS(name); ok {
			t.Errorf("NormalizeOS(%q) unexpectedly succeeded", name)
		}
	}
	if _, err := paths.Resolve(paths.Options{GOOS: "plan9", Env: map[string]string{"HOME": "/home/u"}}); err == nil {
		t.Error("Resolve accepted an unsupported operating system")
	}
}
