package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/linkreport"
)

func runLinks(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl links report ...")
		os.Exit(2)
	}
	switch args[0] {
	case "report":
		runLinksReport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown links subcommand %q (want report)\n", args[0])
		os.Exit(2)
	}
}

// runLinksReport prints every ambiguous link with the notes it might mean, and
// optionally writes the same report into the Reports notebook (v1.0 J40-D).
//
// Its output quotes links and titles, so it is for the owner: it is printed
// here and written into the owner's own library, and nowhere else. It reads
// the whole library, so it runs when asked and never on a schedule.
func runLinksReport(args []string) {
	fs := flag.NewFlagSet("notriosctl links report", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "", "narrow to one collection (default: every collection)")
	detailLimit := fs.Int("detail-limit", linkreport.DefaultDetailLimit,
		fmt.Sprintf("notes shown, and links shown per note (max %d); counts are always complete", linkreport.MaxDetailLimit))
	candidateLimit := fs.Int("candidate-limit", linkreport.DefaultCandidateLimit,
		fmt.Sprintf("candidates shown per link (max %d)", linkreport.MaxCandidateLimit))
	writeNote := fs.Bool("write-note", false, "overwrite the read-only report note in the Reports notebook")
	quiet := fs.Bool("quiet", false, "print nothing; useful with --write-note")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl links report [--write-note] [--detail-limit N] [--candidate-limit N]")
		fs.PrintDefaults()
		os.Exit(2)
	}
	for _, limit := range []struct {
		name           string
		value, ceiling int
	}{{"--detail-limit", *detailLimit, linkreport.MaxDetailLimit}, {"--candidate-limit", *candidateLimit, linkreport.MaxCandidateLimit}} {
		if limit.value <= 0 || limit.value > limit.ceiling {
			fmt.Fprintf(os.Stderr, "%s must be between 1 and %d\n", limit.name, limit.ceiling)
			os.Exit(2)
		}
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	req := linkreport.Request{CollectionID: strings.TrimSpace(*collectionID), DetailLimit: *detailLimit, CandidateLimit: *candidateLimit}
	if *writeNote {
		doc, report, err := linkreport.WriteNote(context.Background(), st, req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !*quiet {
			printJSON(map[string]any{"document_id": doc.ID, "uri": doc.URI, "title": doc.Title, "report": report})
		}
		return
	}
	report, err := linkreport.Generate(context.Background(), st, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !*quiet {
		printJSON(report)
	}
}
