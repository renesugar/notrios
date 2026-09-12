package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/paths"
	"github.com/renesugar/notrios/internal/purge"
)

// `notriosctl purge` exists because a packaged installation could not reach any
// of this.
//
// `make purge` takes a verified backup before it deletes anything, refuses when
// nothing can answer a prompt, never follows a symlink out of a profile, and
// keeps sync key material out of the backup it writes. All of that lived in
// scripts/lifecycle.py, which the .deb does not ship -- so a user who installed
// the package was deleting directories by hand with no backup taken for them,
// and v0.9 I9's installation page had to say so.
//
// It deletes data, not program files. Removing the program is `apt remove`, or
// `make uninstall`, and both leave the data alone by design; this is the other
// half, and the half that is irreversible.
func runPurge(args []string) {
	fs := flag.NewFlagSet("notriosctl purge", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "show exactly what would be deleted and delete nothing")
	confirm := fs.Bool("confirm", false, "proceed without asking; required when nothing can answer")
	noBackup := fs.Bool("no-backup", false, "delete without copying anything anywhere first")
	backupDir := fs.String("backup-dir", "", "where to write the backup (default: beside the state root)")
	asJSON := fs.Bool("json", false, "print the plan as JSON instead of a report")
	noRedact := fs.Bool("no-redact", false, "print the home directory instead of ~")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	resolution, err := paths.ForProcess(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	home := ""
	if !*noRedact {
		home, _ = os.UserHomeDir()
	}
	show := func(path string) string {
		if home == "" {
			return path
		}
		return paths.Redact(path, home)
	}

	// paths.RootNames, not resolution.Codes(): Codes reports the notice codes a
	// resolution emitted, which is a different list entirely and happened to be
	// empty here -- leaving purge with no roots and a confusing complaint about
	// the state root rather than an obvious one about having nothing to do.
	roots := map[string]string{}
	for _, name := range paths.RootNames {
		if value := resolution.Root(name); value != "" {
			roots[name] = value
		}
	}
	userHome, _ := os.UserHomeDir()
	steps := purge.Plan(roots, purge.Environment{Home: userHome})

	destination := ""
	if !*noBackup {
		if *backupDir != "" {
			destination = *backupDir
		} else if destination, err = purge.BackupDestination(roots, time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// The backup must not land anywhere this same run would delete. H3
		// proved the property for the default location; this asserts it on
		// every run rather than trusting the layout -- which matters more here
		// than it does for the Make target, because --backup-dir lets the user
		// name a destination, and the obvious wrong answer is somewhere inside
		// the library they are about to remove.
		owned := make([]string, 0, len(roots))
		for _, path := range roots {
			owned = append(owned, path)
		}
		verdict := purge.Decide(destination, purge.Environment{OwnedRoots: owned, Home: userHome})
		if verdict.Verdict != purge.Refuse {
			fmt.Fprintf(os.Stderr,
				"the backup destination %s is not refused by the purge oracle (%s),\n"+
					"which means this run could delete its own backup. Refusing.\n",
				destination, verdict)
			os.Exit(1)
		}
	}

	if *asJSON {
		printJSON(map[string]any{
			"dry_run": *dryRun, "backup": destination, "no_backup": *noBackup,
			"steps": steps, "redacted": home != "",
		})
		if *dryRun {
			return
		}
	} else {
		describePurge(steps, destination, *noBackup, show)
	}

	if *dryRun {
		fmt.Println("\ndry run: nothing was deleted.")
		return
	}

	if !anythingToDelete(steps) {
		fmt.Println("\nNothing to delete: no data root resolved to something removable.")
		return
	}

	if !confirmPurge(*confirm) {
		os.Exit(1)
	}

	if !*noBackup {
		fmt.Printf("\nwriting backup to %s\n", show(destination))
		manifest, err := purge.CreateBackup(steps, destination)
		if err != nil {
			fmt.Fprintf(os.Stderr, "the backup failed, so nothing was deleted: %v\n", err)
			os.Exit(1)
		}
		ok, detail := purge.VerifyBackup(destination)
		if !ok {
			fmt.Fprintf(os.Stderr, "the backup could not be verified, so nothing was deleted: %s\n", detail)
			os.Exit(1)
		}
		fmt.Printf("backup verified: %s\n", detail)
		if len(manifest.ExcludedKeyMaterial) > 0 {
			fmt.Printf("sync key material excluded from the backup: %d file(s)\n",
				len(manifest.ExcludedKeyMaterial))
		}
	}

	removed, err := purge.Remove(steps)
	for _, path := range removed {
		fmt.Printf("deleted %s\n", show(path))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Said last, where it is read, and in absolute form even when the rest is
	// redacted: a backup the user cannot find is not a backup.
	if !*noBackup {
		fmt.Printf("\nYour data is in %s until you remove it. Nothing deletes it for you.\n", destination)
	}
}

func anythingToDelete(steps []purge.Step) bool {
	for _, step := range steps {
		if step.Action == "backup_then_delete" || step.Action == "dispose" {
			return true
		}
	}
	return false
}

// confirmPurge asks before deleting, and fails closed when nobody can answer.
//
// A non-interactive run without --confirm refuses rather than hanging on a
// prompt no one will see. --confirm skips the question, never the backup.
func confirmPurge(confirmed bool) bool {
	if confirmed {
		return true
	}
	// Decided by reading, not by inspecting the file mode.
	//
	// The obvious check -- os.Stdin.Stat() and ModeCharDevice -- calls
	// /dev/null a terminal, because /dev/null *is* a character device. A purge
	// run with stdin redirected therefore printed a prompt nobody could see and
	// reported an answer nobody gave. It failed closed, which is the direction
	// to be wrong in, but it told the user the wrong thing: that they had
	// declined, rather than that nothing could ask them.
	//
	// A terminal blocks for input; a closed or empty stream returns EOF at
	// once. That distinction is the one that matters, and it needs no extra
	// dependency to make. The prompt goes to stderr so that a caller capturing
	// stdout gets the report rather than the question.
	fmt.Fprint(os.Stderr, "\nType 'delete my notes' to continue: ")
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && strings.TrimSpace(answer) == "" {
		fmt.Fprintln(os.Stderr,
			"\n\nNothing was deleted. Nothing could answer that question -- this is not an\n"+
				"interactive terminal. Run it interactively, or pass --confirm if you are\n"+
				"automating it deliberately: --confirm skips the question, never the backup.")
		return false
	}
	if strings.TrimSpace(answer) != "delete my notes" {
		fmt.Fprintln(os.Stderr, "\nNothing was deleted.")
		return false
	}
	return true
}

func describePurge(steps []purge.Step, destination string, noBackup bool, show func(string) string) {
	fmt.Println("Mutable roots:")
	for _, step := range steps {
		label := map[string]string{
			"backup_then_delete": "BACK UP AND DELETE",
			"dispose":            "DELETE WITHOUT BACKUP",
			"keep":               "KEEP   ",
			"refuse":             "REFUSED",
		}[step.Action]
		detail := step.Reason
		if step.Action == "backup_then_delete" && detail == "" {
			detail = fmt.Sprintf("%d files, %d bytes", step.Files, step.Bytes)
		}
		if step.Action == "dispose" && detail == "" {
			detail = "rebuildable"
		}
		fmt.Printf("  %-22s %s  (%s; %s)\n", label, show(step.Path), step.Category, detail)
	}
	fmt.Println("\nBackup:")
	if noBackup {
		fmt.Println("  NONE. --no-backup was passed, so nothing will be copied anywhere")
		fmt.Println("  before it is deleted. Run it again without --no-backup if that was not")
		fmt.Println("  what you meant.")
		return
	}
	fmt.Printf("  %s\n", show(destination))
	fmt.Println("  written and verified before anything is deleted")
	if parent := filepath.Dir(destination); parent != "" {
		_ = parent
	}
}
