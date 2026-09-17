package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/installreport"
	"github.com/renesugar/notrios/internal/paths"
)

// `notriosctl paths --report` answers "where is *everything*?".
//
// `paths` reports the roots, which is what a user needs when a note cannot be
// found. This reports every directory and every file inside them, plus every
// registered profile's library including the ones kept outside those roots, and
// the program files the installer recorded. Two uses, both concrete: after an
// install it is the record of what a clean installation looks like, and taken
// before a purge it is the oracle `scripts/check_purged.sh` reads afterwards to
// say whether the deletion was complete (v1.0 J11).
//
// It is a flag on `paths` rather than a `paths report` subcommand because a
// subcommand makes its parent a group, and `notriosctl paths` is a command
// users run on its own.

// reportOptions are the flags `paths` parsed for --report.
type reportOptions struct {
	asJSON, asPaths, noRedact bool
	registry, installManifest string
}

// runPathsReport builds and prints the report in the shape the flags asked for.
func runPathsReport(resolution paths.Resolution, options reportOptions) {
	if options.asJSON && options.asPaths {
		fmt.Fprintln(os.Stderr, "choose one of --json and --paths")
		os.Exit(2)
	}
	report, err := installreport.Build(installreport.Options{
		Roots:           resolution.Roots,
		Mode:            string(resolution.Mode),
		RegistryPath:    options.registry,
		InstallManifest: options.installManifest,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// --paths is what a checker reads, and a checker compares real paths, so it
	// is never redacted. Refusing is better than printing `~` to something that
	// would then test for a file called "~".
	if options.asPaths {
		if options.noRedact {
			fmt.Fprintln(os.Stderr, "--paths is already literal; --no-redact would say nothing")
			os.Exit(2)
		}
		for _, line := range report.Paths() {
			fmt.Println(line)
		}
		return
	}

	// Redacted by default, like `paths`, `config show` and `doctor`: this is
	// pasted into issue reports, and a home directory carries a username.
	home := ""
	if !options.noRedact {
		if resolved, err := os.UserHomeDir(); err == nil {
			home = resolved
		}
	}
	if options.asJSON {
		printReportJSON(report, home)
		return
	}
	printReport(report, home)
}

func printReportJSON(report installreport.Report, home string) {
	redacted := report
	redacted.Roots = map[string]string{}
	for name, path := range report.Roots {
		redacted.Roots[name] = paths.Redact(path, home)
	}
	redacted.Structure = redactEntries(report.Structure, home)
	redacted.Manifest = redactEntries(report.Manifest, home)
	redacted.Notes = make([]string, 0, len(report.Notes))
	for _, note := range report.Notes {
		redacted.Notes = append(redacted.Notes, paths.RedactAll(note, home))
	}
	if report.Program != nil {
		program := *report.Program
		program.ManifestPath = paths.Redact(program.ManifestPath, home)
		program.Prefix = paths.Redact(program.Prefix, home)
		redacted.Program = &program
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(map[string]any{
		"schema": redacted.Schema, "mode": redacted.Mode, "roots": redacted.Roots,
		"structure": redacted.Structure, "manifest": redacted.Manifest,
		"program": redacted.Program, "notes": redacted.Notes, "counts": redacted.Counts,
		"redacted": home != "",
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func redactEntries(entries []installreport.Entry, home string) []installreport.Entry {
	out := make([]installreport.Entry, 0, len(entries))
	for _, entry := range entries {
		entry.Path = paths.Redact(entry.Path, home)
		out = append(out, entry)
	}
	return out
}

func printReport(report installreport.Report, home string) {
	show := func(value string) string { return paths.Redact(value, home) }
	fmt.Printf("mode: %s\n\n", report.Mode)

	fmt.Println("structure:")
	names := make([]string, 0, len(report.Roots))
	for name := range report.Roots {
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
		if root := strings.TrimSpace(report.Roots[name]); root != "" {
			fmt.Printf("  %-*s  %s\n", width, name, show(root))
		}
	}
	fmt.Printf("\n  %s, %s, %s\n", count(report.Counts.Directories, "directory", "directories"),
		count(report.Counts.Files, "file", "files"), humanBytes(report.Counts.Bytes))

	if report.Program != nil {
		fmt.Printf("\nprogram files: %d recorded by %s", report.Program.Entries, show(report.Program.ManifestPath))
		if report.Program.Version != "" {
			fmt.Printf(" (version %s)", report.Program.Version)
		}
		fmt.Println()
	}

	external := []installreport.Entry{}
	for _, entry := range report.Manifest {
		if !entry.Owned {
			external = append(external, entry)
		}
	}
	fmt.Printf("\nprofiles: %d registered", report.Counts.Profiles)
	if len(external) == 0 {
		fmt.Println("; none keeps a library outside these roots")
	} else {
		fmt.Printf("; %s outside these roots, which a purge keeps:\n", count(len(external), "path", "paths"))
		for _, entry := range external {
			state := ""
			if entry.Kind == installreport.KindMissing {
				state = " (not present)"
			}
			fmt.Printf("  %-12s %-8s %s%s\n", entry.Profile, entry.Field, show(entry.Path), state)
		}
	}

	if len(report.Notes) > 0 {
		fmt.Println("\nnotes:")
		for _, note := range report.Notes {
			fmt.Printf("  %s\n", paths.RedactAll(note, home))
		}
	}

	fmt.Println("\nThe full manifest is --json, or --paths for one path per line.")
	fmt.Println("Taken before `notriosctl purge`, a --paths manifest is what")
	fmt.Println("scripts/check_purged.sh reads afterwards to confirm the deletion.")
}

// count writes a number with the right noun, because "1 directories" in a
// report a user pastes into an issue reads like a bug in the report.
func count(value int, singular, plural string) string {
	if value == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", value, plural)
}

func humanBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value, exponent := float64(bytes)/unit, 0
	for value >= unit && exponent < 3 {
		value /= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %s", value, []string{"KiB", "MiB", "GiB", "TiB"}[exponent])
}
