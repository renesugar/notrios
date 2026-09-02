package main

import (
	"os"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// Every command named in a recovery message must still exist.
//
// Recovery messages tell a user what to type at the moment they have least room
// to improvise: their library has just been rolled back, or a migration has
// refused. A message naming a command that has since been renamed is worse than
// no message at all — it sends them somewhere that does not exist and costs them
// the trust they need to follow the rest of it.
//
// Nothing else catches this. The strings live in internal/store, the command
// registry lives here, and no compiler or documentation gate connects them:
// `printHelp` could drop `export archive-v2` tomorrow and every test would still
// pass. So the two are compared directly.
func TestRecoveryMessagesNameCommandsThatStillExist(t *testing.T) {
	help := capturedHelp(t)
	commands := store.RecoveryCommands()
	if len(commands) == 0 {
		t.Fatal("no recovery commands are registered; the guard would pass vacuously")
	}
	for _, command := range commands {
		// The usage line carries flags and placeholders after the command, so
		// the command itself is a prefix of a line rather than a whole one.
		if !strings.Contains(help, command) {
			t.Errorf("recovery messages name %q, which no longer appears in `notriosctl help`", command)
		}
	}
}

// And the registry has to describe the messages, not just any commands. A list
// nobody keeps current is the failure mode this guard is meant to prevent, so
// the strings actually used are checked against it.
func TestRegisteredRecoveryCommandsAppearInTheMessages(t *testing.T) {
	marker := store.MigrationMarker{FromVersion: 19, ToVersion: 27, BackupDir: "/tmp/backup"}
	message := store.MigrationRolledBackMessage(marker, "/tmp/notes.sqlite")
	for _, command := range store.RecoveryCommands() {
		if !strings.Contains(message, command) {
			t.Errorf("%q is registered as a recovery command but no recovery message uses it; "+
				"the registry has drifted from the text it is supposed to guard", command)
		}
	}
}

func capturedHelp(t *testing.T) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = write
	printHelp()
	os.Stdout = stdout
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	buffer := make([]byte, 4096)
	for {
		n, err := read.Read(buffer)
		builder.Write(buffer[:n])
		if err != nil {
			break
		}
	}
	return builder.String()
}

// Nothing may name a command without registering it.
//
// The two checks above guard the registry in both directions, but only for
// commands that are in it. A message that mentions `notriosctl reindex` without
// anyone adding it to RecoveryCommands would escape them entirely, and would
// then be free to go stale — which is the whole failure this is meant to
// prevent. So the message is read back and every command it names has to be
// accounted for.
func TestEveryCommandNamedInARecoveryMessageIsRegistered(t *testing.T) {
	marker := store.MigrationMarker{FromVersion: 19, ToVersion: 27, BackupDir: "/tmp/backup"}
	message := store.MigrationRolledBackMessage(marker, "/tmp/notes.sqlite")

	registered := store.RecoveryCommands()
	named := commandsNamedIn(message)
	if len(named) == 0 {
		t.Fatal("no commands were found in the recovery message; the scan is not working")
	}
	for _, command := range named {
		accounted := false
		for _, known := range registered {
			if strings.HasPrefix(command, known) {
				accounted = true
				break
			}
		}
		if !accounted {
			t.Errorf("the recovery message names %q, which is not in store.RecoveryCommands(); "+
				"add it there so it is checked against `notriosctl help`", command)
		}
	}
}

// commandsNamedIn pulls `notriosctl ...` invocations out of prose. Tokens are
// taken until one looks like a flag, a path, or a placeholder, which is where
// the command name ends and its arguments begin.
func commandsNamedIn(message string) []string {
	found := []string{}
	for _, line := range strings.Split(message, "\n") {
		rest := line
		for {
			index := strings.Index(rest, "notriosctl")
			if index < 0 {
				break
			}
			rest = rest[index:]
			fields := strings.Fields(rest)
			command := []string{}
			for _, field := range fields {
				field = strings.Trim(field, "`.,:;")
				if field == "" {
					break
				}
				if len(command) > 0 && (strings.HasPrefix(field, "-") ||
					strings.HasPrefix(field, "/") || strings.HasPrefix(field, "<")) {
					break
				}
				command = append(command, field)
			}
			if len(command) > 1 {
				found = append(found, strings.Join(command, " "))
			}
			rest = rest[len("notriosctl"):]
		}
	}
	return found
}
