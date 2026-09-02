package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/migrate"
	"github.com/renesugar/notrios/internal/paths"
	"github.com/renesugar/notrios/internal/store"
)

// `notriosctl paths` answers "where is this instance keeping things?".
//
// It is the first question in any support thread that starts "it cannot find my
// notes", and before this the only way to answer it was to read the resolver.
// `doctor` reported the config file and the database; it did not report the
// roots, and it never reported which *mode* the process resolved -- installed,
// portable, or a source checkout -- which after H4 is the single most useful
// fact when a user asks why a particular database was opened.
//
// It is deliberately not under a `debug` namespace. This is something users
// need, not a developer tool.
func runPaths(args []string) {
	fs := flag.NewFlagSet("notriosctl paths", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
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

	// Redacted by default. These get pasted into issue reports, and a home
	// directory carries a username: a small disclosure, but a gratuitous one,
	// since the reader needs the shape of the layout and not who owns it.
	home := ""
	if !*noRedact {
		if resolved, err := os.UserHomeDir(); err == nil {
			home = resolved
		}
	}

	if *asJSON {
		roots := map[string]string{}
		for name, value := range resolution.Roots {
			roots[name] = paths.Redact(value, home)
		}
		notices := make([]map[string]string, 0, len(resolution.Notices))
		for _, notice := range resolution.Notices {
			notices = append(notices, map[string]string{
				"code": notice.Code, "message": paths.RedactAll(notice.Message, home),
			})
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		payload := map[string]any{
			"mode": string(resolution.Mode), "roots": roots, "notices": notices,
			"redacted": home != "",
		}
		if backups := preMigrationBackupRoot(); backups != "" {
			payload["pre_migration_backups"] = paths.Redact(backups, home)
		}
		if candidate, found := detectLegacyLayout(resolution); found {
			payload["legacy_layout"] = map[string]any{
				"root":     paths.Redact(candidate.Root, home),
				"database": paths.Redact(candidate.DatabasePath, home),
				"occupied": candidate.Occupied,
			}
		}
		if err := encoder.Encode(payload); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	fmt.Print(resolution.Diagnostics(home))
	printPreMigrationBackupRoot(home)
	printLegacyLayoutNotice(resolution, home)
}

// preMigrationBackupRoot is where a schema migration keeps its copy of the
// database.
//
// It is reported here because the whole value of that backup is being findable.
// A user needs it after a migration went wrong or was refused, which is exactly
// when they are least able to go looking through the source for a path.
func preMigrationBackupRoot() string {
	database := configuredDatabasePath()
	if strings.TrimSpace(database) == "" {
		return ""
	}
	return store.PreMigrationBackupRoot(database)
}

func printPreMigrationBackupRoot(home string) {
	root := preMigrationBackupRoot()
	if root == "" {
		return
	}
	fmt.Println("pre-migration backups:")
	fmt.Printf("  %s\n", paths.Redact(root, home))
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		fmt.Printf("  %d kept\n", len(entries))
	} else {
		fmt.Println("  none yet; one is written before any schema migration")
	}
}

// detectLegacyLayout looks for a pre-0.8 library beside the process.
//
// The working directory is the only place worth looking, and it is not a guess:
// the pre-0.8 defaults were relative to it, so a user standing where they
// always stood is standing on their library. Nothing is read from it and
// nothing is loaded from it -- it is noticed and named.
func detectLegacyLayout(resolution paths.Resolution) (migrate.Candidate, bool) {
	working, err := os.Getwd()
	if err != nil {
		return migrate.Candidate{}, false
	}
	return migrate.Detect(working, resolution, configuredDatabasePath())
}

// configuredDatabasePath is the database this process would actually open.
//
// It is what stops the notice firing on a library that is in use. A
// configuration saying `directory: ./data` resolves against the working
// directory, so without this the command would tell a user their own current
// library was stranded and offer to move it out from under the configuration
// naming it. An unreadable configuration returns "", which reports rather than
// suppresses: being unable to read a config is not evidence that a library is
// in use.
func configuredDatabasePath() string {
	cfg, err := config.LoadDefault()
	if err != nil {
		return ""
	}
	return cfg.Data.DatabasePath
}

// printLegacyLayoutNotice tells a user their notes are where they left them.
//
// Without this the failure is silent in the worst way: `paths` prints an
// impeccable list of empty native roots while the library sits in the directory
// the command was run from, and the user concludes their notes are gone.
func printLegacyLayoutNotice(resolution paths.Resolution, home string) {
	candidate, found := detectLegacyLayout(resolution)
	if !found {
		return
	}
	fmt.Println("pre-0.8 layout:")
	fmt.Printf("  a database from an older Notrios is at %s\n", paths.Redact(candidate.DatabasePath, home))
	fmt.Println("  this instance is not using it; the roots above are what it reads and writes")
	if candidate.Occupied {
		fmt.Println("  the resolved location already holds a database, so migration would be a merge and is refused")
		fmt.Println("  open either one explicitly with --db")
		return
	}
	fmt.Println("  run `notriosctl migrate --dry-run` to see what moving it would do")
}
