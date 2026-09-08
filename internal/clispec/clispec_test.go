package clispec_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/clispec"
)

func load(t *testing.T) clispec.Registry {
	t.Helper()
	registry, err := clispec.Load()
	if err != nil {
		t.Fatalf("loading the command registry: %v", err)
	}
	return registry
}

func TestEveryCommandSaysWhatItIsFor(t *testing.T) {
	registry := load(t)
	for _, command := range registry.Commands {
		if strings.TrimSpace(command.Purpose) == "" && command.Exempt == "" {
			t.Errorf("%s has no purpose, so its help says only how to type it", command.Name())
		}
		if len(command.Path) == 0 {
			t.Error("a command has no path")
		}
	}
}

func TestEveryLevelAnswersARequestForHelp(t *testing.T) {
	registry := load(t)
	cases := [][]string{
		{},
		{"notes"},
		{"notes", "show"},
		{"doctor"},
		{"publish", "profile"},
		{"publish", "profile", "list"},
	}
	for _, path := range cases {
		var out bytes.Buffer
		if !registry.Help(&out, path...) {
			t.Errorf("no help for %q", strings.Join(path, " "))
			continue
		}
		if out.Len() == 0 {
			t.Errorf("help for %q was empty", strings.Join(path, " "))
		}
		if !strings.Contains(out.String(), registry.Program) {
			t.Errorf("help for %q does not name the program", strings.Join(path, " "))
		}
	}
}

// TestHelpForAGroupListsItsSubcommands is the behaviour a group did not have:
// asking one for help reported an unknown subcommand and exited 2.
func TestHelpForAGroupListsItsSubcommands(t *testing.T) {
	registry := load(t)
	var out bytes.Buffer
	registry.Help(&out, "notes")
	for _, want := range []string{"notes create", "notes show", "notes delete", "notes restore"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("group help does not list %q:\n%s", want, out.String())
		}
	}
}

// TestHelpForACommandNamesItsArguments guards what the flag package cannot
// know. `import obsidian --help` never mentioned that a vault directory is
// required, which is the one thing its reader needs.
func TestHelpForACommandNamesItsArguments(t *testing.T) {
	registry := load(t)
	var out bytes.Buffer
	registry.Help(&out, "import", "obsidian")
	if !strings.Contains(out.String(), "<vault-dir>") {
		t.Errorf("help for `import obsidian` does not name its positional argument:\n%s", out.String())
	}
}

func TestHelpRefusesAPathThatIsNotACommand(t *testing.T) {
	registry := load(t)
	var out bytes.Buffer
	if registry.Help(&out, "notes", "incinerate") {
		t.Error("help was produced for a command that does not exist")
	}
}

func TestUsageFormsAreTheSurfaceInventory(t *testing.T) {
	registry := load(t)
	forms := registry.UsageForms()
	if len(forms) != len(registry.Offered()) {
		t.Errorf("%d usage forms for %d offered commands", len(forms), len(registry.Offered()))
	}
	seen := map[string]bool{}
	for _, form := range forms {
		if !strings.HasPrefix(form, registry.Program+" ") {
			t.Errorf("usage form does not start with the program name: %q", form)
		}
		if seen[form] {
			t.Errorf("duplicate usage form: %q", form)
		}
		seen[form] = true
	}
}

func TestAnExemptCommandIsNotOffered(t *testing.T) {
	registry := load(t)
	for _, command := range registry.Commands {
		if command.Exempt == "" {
			continue
		}
		for _, form := range registry.UsageForms() {
			if strings.HasPrefix(form, registry.Program+" "+command.Name()) {
				t.Errorf("%s is exempt but appears in the surface inventory", command.Name())
			}
		}
	}
}

func TestHelpRequestedStopsAtASeparator(t *testing.T) {
	if !clispec.HelpRequested([]string{"--document", "x", "--help"}) {
		t.Error("a trailing --help was not recognised")
	}
	if clispec.HelpRequested([]string{"--", "--help"}) {
		t.Error("--help after a -- separator was treated as a request for help")
	}
	if clispec.HelpRequested([]string{"--document", "x"}) {
		t.Error("a command line with no help request was treated as one")
	}
}
