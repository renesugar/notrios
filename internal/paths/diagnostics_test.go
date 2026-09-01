package paths_test

import (
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/paths"
)

func TestRedactReplacesTheHomePrefixOnly(t *testing.T) {
	cases := []struct{ path, home, want string }{
		{"/home/alice/.config/notrios", "/home/alice", "~/.config/notrios"},
		{"/home/alice", "/home/alice", "~"},
		{"/home/alicia/.config", "/home/alice", "/home/alicia/.config"}, // prefix, not path-prefix
		{"/srv/notrios", "/home/alice", "/srv/notrios"},
		{"/home/alice/x", "", "/home/alice/x"},
		{`C:\Users\alice\AppData`, `C:\Users\alice`, `~\AppData`},
		{"/home/alice/x", "/home/alice/", "~/x"}, // trailing separator tolerated
	}
	for _, c := range cases {
		if got := paths.Redact(c.path, c.home); got != c.want {
			t.Errorf("Redact(%q, %q) = %q, want %q", c.path, c.home, got, c.want)
		}
	}
}

// A diagnostic is printed at startup and pasted into issue reports. It must
// show the shape of the layout without naming the user.
func TestDiagnosticsRedactsTheUsernameEverywhereItAppears(t *testing.T) {
	resolved, err := paths.Resolve(paths.Options{
		GOOS: paths.Linux,
		Env:  map[string]string{"HOME": "/home/alice", "XDG_CONFIG_HOME": "not/absolute"},
	})
	if err != nil {
		t.Fatal(err)
	}

	report := resolved.Diagnostics("/home/alice")
	if strings.Contains(report, "alice") {
		t.Fatalf("the username survived redaction:\n%s", report)
	}
	// Redaction must not have eaten the content: the reader still needs the
	// shape of the layout and the reason a variable was ignored.
	for _, want := range []string{"mode: installed", "~/.config/notrios", "~/.local/share/notrios",
		"xdg_relative_ignored", "runtime_dir_unset"} {
		if !strings.Contains(report, want) {
			t.Errorf("diagnostics lost %q:\n%s", want, report)
		}
	}
}

// RedactAll is for sentences with paths inside them. The distinction from
// Redact matters: the first Diagnostics redacted the root listing and left the
// username sitting in the notice that explained why a variable was ignored.
func TestRedactAllHandlesEmbeddedPathsWithoutOverreaching(t *testing.T) {
	cases := []struct{ text, home, want string }{
		{"using /home/alice/.local/state rather than /tmp", "/home/alice", "using ~/.local/state rather than /tmp"},
		{"a and /home/alice/b and /home/alice/c", "/home/alice", "a and ~/b and ~/c"},
		{"/home/alicia/x is someone else", "/home/alice", "/home/alicia/x is someone else"},
		{"ends at /home/alice", "/home/alice", "ends at ~"},
		{"nothing to do", "/home/alice", "nothing to do"},
		{"anything", "", "anything"},
	}
	for _, c := range cases {
		if got := paths.RedactAll(c.text, c.home); got != c.want {
			t.Errorf("RedactAll(%q, %q) = %q, want %q", c.text, c.home, got, c.want)
		}
	}
}

func TestDiagnosticsReportsEveryRoot(t *testing.T) {
	resolved, err := paths.Resolve(paths.Options{
		GOOS: paths.Linux,
		Env:  map[string]string{"HOME": "/home/u", "XDG_RUNTIME_DIR": "/run/user/1000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	report := resolved.Diagnostics("/home/u")
	for _, name := range paths.RootNames {
		if !strings.Contains(report, name) {
			t.Errorf("diagnostics omitted the %s root:\n%s", name, report)
		}
	}
}
