package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

type runResult struct {
	JournalEnabled bool    `json:"journal_enabled"`
	Documents      int     `json:"documents"`
	Operations     int64   `json:"operations"`
	ElapsedMS      float64 `json:"elapsed_ms"`
	DocumentsPerS  float64 `json:"documents_per_second"`
	DatabaseBytes  int64   `json:"database_bytes"`
}

type evidence struct {
	GeneratedAt       string    `json:"generated_at"`
	GoVersion         string    `json:"go_version"`
	GOOS              string    `json:"goos"`
	GOARCH            string    `json:"goarch"`
	DocumentCount     int       `json:"document_count"`
	JournalDisabled   runResult `json:"journal_disabled"`
	JournalEnabled    runResult `json:"journal_enabled"`
	ElapsedOverheadPC float64   `json:"elapsed_overhead_percent"`
	ByteOverheadPC    float64   `json:"database_byte_overhead_percent"`
	ExactSequences    bool      `json:"exact_sequences"`
}

func main() {
	count := flag.Int("count", 100_000, "number of imported documents per run")
	out := flag.String("out", "", "output JSON path")
	flag.Parse()
	if *count != 100_000 {
		fatalf("count must be 100000 for the reviewed G4 evidence tier")
	}
	if *out == "" {
		fatalf("-out is required")
	}
	root, err := os.MkdirTemp("", "notrios-g4-profile-")
	if err != nil {
		fatalf("temporary directory: %v", err)
	}
	defer os.RemoveAll(root)

	disabled := runImport(filepath.Join(root, "disabled"), *count, false)
	enabled := runImport(filepath.Join(root, "enabled"), *count, true)
	report := evidence{
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		GoVersion:         runtime.Version(),
		GOOS:              runtime.GOOS,
		GOARCH:            runtime.GOARCH,
		DocumentCount:     *count,
		JournalDisabled:   disabled,
		JournalEnabled:    enabled,
		ElapsedOverheadPC: percent(enabled.ElapsedMS-disabled.ElapsedMS, disabled.ElapsedMS),
		ByteOverheadPC:    percent(float64(enabled.DatabaseBytes-disabled.DatabaseBytes), float64(disabled.DatabaseBytes)),
		ExactSequences:    enabled.Operations == int64(*count*3),
	}
	if !report.ExactSequences || disabled.Operations != 0 {
		fatalf("journal count invariant failed: disabled=%d enabled=%d want=%d", disabled.Operations, enabled.Operations, *count*3)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatalf("encode evidence: %v", err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fatalf("write evidence: %v", err)
	}
	fmt.Printf("wrote %s\n", *out)
}

func runImport(root string, count int, journal bool) runResult {
	if err := os.MkdirAll(root, 0o755); err != nil {
		fatalf("create run directory: %v", err)
	}
	dbPath := filepath.Join(root, "notes.sqlite")
	st, err := store.OpenSQLiteWithAssetStore(dbPath, filepath.Join(root, "assets"))
	if err != nil {
		fatalf("open store: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		fatalf("bootstrap: %v", err)
	}
	if journal {
		if _, err := st.EnrollLocalJournal(ctx, "G4 100k overhead evidence"); err != nil {
			fatalf("enroll journal: %v", err)
		}
	}
	started := time.Now()
	const batchSize = 500
	for start := 0; start < count; start += batchSize {
		end := start + batchSize
		if end > count {
			end = count
		}
		mutations := make([]store.ImportDocumentMutation, 0, end-start)
		for i := start; i < end; i++ {
			id := fmt.Sprintf("doc_g4_%06d", i)
			key := fmt.Sprintf("note-%06d.md", i)
			mutations = append(mutations, store.ImportDocumentMutation{
				Action: "create",
				Document: store.CreateDocumentRequest{
					PreferredID: id,
					Title:       fmt.Sprintf("G4 profile %06d", i),
					Body:        fmt.Sprintf("# G4 profile %06d\n\nGenerated local journal evidence.\n", i),
					Message:     "G4 generated import",
				},
				Source: store.SetDocumentSourceRequest{
					DocumentID: id, SourceSystem: "g4_profile", ExternalID: key,
				},
				State: store.ImportItemState{
					SourceSystem: "g4_profile", SourceKey: "generated", CollectionID: "default",
					ItemKey: key, ItemType: "note", Fingerprint: fmt.Sprintf("fingerprint-%06d", i),
					TargetID: id, Action: "create",
				},
			})
		}
		checkpoint := store.ImportCheckpoint{
			SourceSystem: "g4_profile", SourceKey: "generated", CollectionID: "default",
			InventoryFingerprint: "g4-100k-v1", Phase: "notes", NextIndex: end,
			TotalItems: count, ProcessedItems: end, Status: "running", ReportJSON: "{}",
		}
		if end == count {
			checkpoint.Status = "completed"
		}
		if err := st.ApplyImportDocumentBatch(ctx, store.ImportDocumentBatchRequest{Documents: mutations, Checkpoint: checkpoint}); err != nil {
			fatalf("apply import batch %d-%d: %v", start, end, err)
		}
	}
	elapsed := time.Since(started)
	if err := st.Exec(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		fatalf("checkpoint WAL: %v", err)
	}
	status, err := st.JournalStatus(ctx)
	if err != nil {
		fatalf("journal status: %v", err)
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		fatalf("database stat: %v", err)
	}
	return runResult{
		JournalEnabled: status.Enabled,
		Documents:      count,
		Operations:     status.LastSequence,
		ElapsedMS:      float64(elapsed.Microseconds()) / 1000,
		DocumentsPerS:  float64(count) / elapsed.Seconds(),
		DatabaseBytes:  info.Size(),
	}
}

func percent(delta, base float64) float64 {
	if base == 0 {
		return 0
	}
	return delta / base * 100
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
