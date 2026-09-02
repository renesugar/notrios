package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/renesugar/notrios/internal/paths"
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
		if err := encoder.Encode(map[string]any{
			"mode": string(resolution.Mode), "roots": roots, "notices": notices,
			"redacted": home != "",
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	fmt.Print(resolution.Diagnostics(home))
}
