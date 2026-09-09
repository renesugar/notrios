package main

import (
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// The exit-code contract, designed rather than accreted. A script that cannot
// tell "not finished" from "failed" will either poll forever or give up early,
// so every state a caller can observe maps to a distinct code.
func TestJobExitCodesAreDistinctPerState(t *testing.T) {
	want := map[string]int{
		store.JobSucceeded:   0,
		store.JobFailed:      1,
		store.JobRunning:     3,
		store.JobQueued:      3,
		store.JobCancelled:   4,
		store.JobInterrupted: 6,
	}
	for state, code := range want {
		if got := jobExitCode(state); got != code {
			t.Fatalf("jobExitCode(%q) = %d, want %d", state, got, code)
		}
	}
	// Interrupted is deliberately not the failure code: the work may have
	// completed a great deal before the process died and the importers resume
	// from their own checkpoints, so "run it again" is right — which it is not
	// for a failure.
	if jobExitCode(store.JobInterrupted) == jobExitCode(store.JobFailed) {
		t.Fatal("interrupted and failed call for different responses and must not share a code")
	}
	// 2 is reserved for usage across every command here, so no state may take
	// it — otherwise a typo would read as a job outcome.
	for state := range want {
		if jobExitCode(state) == exitJobUsage {
			t.Fatalf("state %q collides with the usage exit code", state)
		}
	}
}

// The command is rendered from stored parameters, never from stored argv: argv
// would have captured whatever was on the command line, including any secret,
// and a stored string cannot improve when a flag is renamed.
func TestRenderJobCommandFromStoredParameters(t *testing.T) {
	command := renderJobCommand(store.Job{
		Kind: store.JobKindImportObsidian,
		Parameters: []store.JobParameter{
			{Name: "collection", Value: "default"},
			{Name: "batch-size", Value: "100"},
			{Name: "preserve-source", Value: "true"},
			// An empty and a false parameter carry no information; printing
			// them would make the command harder to read than the one someone
			// actually typed.
			{Name: "import-config", Value: ""},
			{Name: "localize-media", Value: "false"},
			{Name: "_vault-dir", Value: "/home/someone/My Vault", Path: true},
		},
	})
	want := `notriosctl import obsidian --collection default --batch-size 100 --preserve-source '/home/someone/My Vault'`
	if command != want {
		t.Fatalf("rendered:\n  %s\nwant:\n  %s", command, want)
	}
}

// A path is pasted back into a shell, so it has to survive being pasted.
func TestRenderedCommandQuotesHostileValues(t *testing.T) {
	command := renderJobCommand(store.Job{
		Kind: store.JobKindExportArchiveV2,
		Parameters: []store.JobParameter{
			{Name: "query", Value: "tag:todo OR tag:later"},
			{Name: "_out-dir", Value: "/tmp/it's here; rm -rf /", Path: true},
		},
	})
	if !strings.Contains(command, `'tag:todo OR tag:later'`) {
		t.Fatalf("a value with spaces must be quoted: %s", command)
	}
	// The apostrophe is closed and reopened rather than escaped inside single
	// quotes, which is the only form a POSIX shell accepts, and the semicolon
	// stays inside the quotes rather than becoming a second command.
	if !strings.Contains(command, `'/tmp/it'\''s here; rm -rf /'`) {
		t.Fatalf("a hostile path must not break out of its quotes: %s", command)
	}
}

// A kind this build cannot run gets a comment, not a command that would fail.
func TestRenderJobCommandRefusesToGuess(t *testing.T) {
	command := renderJobCommand(store.Job{Kind: "import_something_later"})
	if !strings.HasPrefix(command, "#") || !strings.Contains(command, "import_something_later") {
		t.Fatalf("expected a comment naming the unknown kind, got %q", command)
	}
}

// Every kind the store knows how to record has a command the CLI knows how to
// render. Without this, adding a kind would produce job records that
// `jobs show --command` could only apologise for.
func TestEveryJobKindRendersACommand(t *testing.T) {
	for _, kind := range store.JobKinds() {
		command := renderJobCommand(store.Job{Kind: kind})
		if strings.HasPrefix(command, "#") {
			t.Fatalf("job kind %q has no command mapping in jobCommands", kind)
		}
		if !strings.HasPrefix(command, "notriosctl ") {
			t.Fatalf("job kind %q rendered %q", kind, command)
		}
	}
}
