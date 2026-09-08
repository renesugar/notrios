package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/renesugar/notrios/internal/migrate"
	"github.com/renesugar/notrios/internal/paths"
	"github.com/renesugar/notrios/internal/store"
)

// `notriosctl migrate` moves a pre-H4 library into the resolved roots.
//
// It is a command rather than something startup does, and that is the whole
// design. A binary that relocated a user's library because it recognised the
// shape of a directory would be making an irreversible decision on evidence it
// cannot check -- the directory it happens to be standing in.
//
// So detection reports and the user decides. The report is the useful half: a
// user whose notes have "vanished" is standing in the directory that holds
// them, and nothing in Notrios used to say so.
func runMigrate(args []string) {
	fs := flag.NewFlagSet("notriosctl migrate", flag.ExitOnError)
	from := fs.String("from", "", "the directory to migrate from (default: the working directory)")
	dryRun := fs.Bool("dry-run", false, "write the plan and stop without copying anything")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	dir := *from
	if dir == "" {
		working, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		dir = working
	}

	resolution, err := paths.ForProcess(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	candidate, found := migrate.Detect(dir, resolution, configuredDatabasePath())
	if !found {
		// Every one of these three answers used to print prose whatever
		// `--json` said, and this is the *common* path: most runs have nothing
		// to migrate. A caller that asked for JSON received a sentence, on the
		// branch it was most likely to take.
		nothing := func(reason, detail string, lines ...string) {
			if *asJSON {
				printJSON(map[string]any{
					"migrated": false, "dry_run": *dryRun, "reason": reason, "detail": detail,
				})
				return
			}
			fmt.Println(detail)
			for _, line := range lines {
				fmt.Println(line)
			}
		}
		if resolution.Mode == paths.ModeSource {
			nothing("source_checkout", fmt.Sprintf(
				"Nothing to migrate: this is a source checkout, whose roots are already %s.",
				filepath.Join(".", migrate.LegacyRootName)))
			return
		}
		// "Not found" and "found, and already yours" are different answers, and
		// only one of them should send the reader looking elsewhere.
		here := filepath.Join(dir, migrate.LegacyRootName, migrate.DatabaseName)
		if inUse := configuredDatabasePath(); inUse != "" && sameFile(here, inUse) {
			nothing("already_in_use", fmt.Sprintf(
				"Nothing to migrate: %s is the library this instance already uses.", here),
				"Your configuration names it, so it was never relocated.")
			return
		}
		nothing("no_legacy_library", fmt.Sprintf("Nothing to migrate: no %s was found in %s.",
			filepath.Join(migrate.LegacyRootName, migrate.DatabaseName), dir),
			"If your notes are elsewhere, name that directory with --from.")
		return
	}

	// An empty database at the destination is not a second library, and this is
	// the common case rather than a corner: a user discovers their notes are
	// missing by running the new binary, and `doctor` or the service creates an
	// empty library at the resolved path in the act of looking. Set it aside --
	// renamed, never deleted -- so the real library can arrive.
	setAside := ""
	if candidate.Occupied {
		empty, err := destinationIsEmpty(filepath.Join(candidate.ResolvedRoot, migrate.DatabaseName))
		if err != nil {
			// Unreadable is not empty. A file that cannot be opened as a
			// database is the last thing to displace on the assumption that it
			// holds nothing -- it may be a corrupted library, which is a
			// recovery problem rather than a migration one.
			fmt.Fprintf(os.Stderr, "%s exists but cannot be read as a database (%v);\n",
				filepath.Join(candidate.ResolvedRoot, migrate.DatabaseName), err)
			fmt.Fprintln(os.Stderr, "refusing to move it aside on the assumption that it is empty.")
			empty = false
		}
		if empty {
			moved, err := setAsideUnusedDatabase(candidate.ResolvedRoot, time.Now())
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			setAside = moved
			candidate.Occupied = false
		}
	}

	// The remaining occupied-destination decision belongs to BuildPlan, not
	// here. It is the only place that can tell a foreign library from this
	// migration's own half-finished copy, and duplicating the check here made an
	// interrupted migration unresumable: the retry was refused as a merge with
	// the library it had itself just copied.
	plan, err := migrate.BuildPlan(candidate, resolution, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, migrate.ErrDestinationOccupied) {
			fmt.Fprintln(os.Stderr, "Merging two libraries is a decision rather than a copy; open each with --db and move what you want.")
		}
		os.Exit(1)
	}

	report, err := migrate.Execute(plan, migrate.Options{DryRun: *dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, migrate.ErrInsufficientSpace) {
			fmt.Fprintf(os.Stderr, "Nothing was moved; %s is unchanged.\n", candidate.Root)
		} else {
			fmt.Fprintf(os.Stderr, "%s is unchanged -- the source is never touched until the copy is verified.\n", candidate.Root)
			fmt.Fprintf(os.Stderr, "Run the same command again to resume from %s.\n", plan.JournalPath)
		}
		os.Exit(1)
	}

	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if setAside != "" {
		fmt.Printf("An unused empty library at the destination was set aside as %s.\n\n", setAside)
	}
	printMigrateReport(report, *dryRun)
}

// destinationIsEmpty reports whether the database at path holds no documents.
//
// It opens the store but never bootstraps it, so an older real library is
// answered for without its schema being migrated -- which matters, because
// migrating a library's schema before it has been copied is exactly the risk
// this command exists to avoid.
func destinationIsEmpty(path string) (bool, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	st, err := store.OpenSQLite(path)
	if err != nil {
		return false, err
	}
	defer st.Close()
	return st.LibraryIsEmpty(context.Background())
}

// setAsideUnusedDatabase renames an empty destination library out of the way,
// with its sidecars, and reports where it went. Nothing is deleted.
func setAsideUnusedDatabase(root string, now time.Time) (string, error) {
	stamp := now.UTC().Format("20060102T150405Z")
	base := filepath.Join(root, migrate.DatabaseName)
	target := base + ".unused-" + stamp
	if err := os.Rename(base, target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("setting aside the unused library at %s: %w", base, err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Rename(base+suffix, target+suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("setting aside %s: %w", base+suffix, err)
		}
	}
	return target, nil
}

func printMigrateReport(report migrate.Report, dryRun bool) {
	if dryRun {
		fmt.Printf("Plan for %s (nothing was copied):\n", report.Plan.LegacyRoot)
	} else {
		fmt.Printf("Migrated %s:\n", report.Plan.LegacyRoot)
	}
	for _, item := range report.Plan.Items {
		switch {
		case !item.Present:
			continue
		case item.Action == migrate.ActionCopy:
			fmt.Printf("  copy     %s -> %s (%d file(s), %d bytes)\n", item.Source, item.Destination, item.Files, item.Bytes)
		case item.Action == migrate.ActionRebuild:
			fmt.Printf("  rebuild  %s is derived data and regenerates; it is not copied\n", item.Source)
		case item.Action == migrate.ActionDiscard:
			fmt.Printf("  discard  %s is work in flight and does not travel\n", item.Source)
		}
	}
	fmt.Printf("  %d bytes to copy; %d bytes free\n", report.Plan.CopyBytes, report.Plan.FreeBytes)
	if len(report.Resumed) > 0 {
		fmt.Printf("  resumed, already complete: %v\n", report.Resumed)
	}
	if dryRun {
		fmt.Printf("\nPlan written to %s. Run without --dry-run to perform it.\n", report.Plan.PlanPath)
		return
	}
	fmt.Printf("\nThe old layout was renamed to %s rather than deleted.\n", report.RetiredTo)
	fmt.Println("It is a complete copy of what you had before, so it is also your backup if the")
	fmt.Println("first open migrates the schema. Check your notes, then remove it yourself.")
}

// sameFile compares two paths as the filesystem would see them.
func sameFile(a, b string) bool {
	left, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	right, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
