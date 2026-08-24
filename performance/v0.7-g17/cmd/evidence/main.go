// Command evidence measures the physical cost of retaining one generated
// metadata operation per document in the frozen 382,206-document full-corpus
// workload. It reads no private corpus content; only the already-published
// aggregate count is used.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

const publishedFullCorpusDocuments = 382206

type report struct {
	Schema                    string   `json:"schema"`
	GeneratedFor              string   `json:"generated_for"`
	SchemaVersion             int      `json:"schema_version"`
	PublishedCorpusDocuments  int      `json:"published_corpus_documents"`
	GeneratedOperations       int      `json:"generated_operations"`
	BaselineDatabaseBytes     int64    `json:"baseline_database_bytes"`
	RetainedDatabaseBytes     int64    `json:"retained_database_bytes"`
	IncrementalBytes          int64    `json:"incremental_bytes"`
	BytesPerOperation         float64  `json:"bytes_per_operation"`
	NinetyDayDailyOperations  int      `json:"ninety_day_daily_operations"`
	NinetyDayProjectedBytes   int64    `json:"ninety_day_projected_bytes"`
	FullCorpusChurnEquivalent float64  `json:"full_corpus_churn_equivalent"`
	ElapsedMilliseconds       int64    `json:"elapsed_milliseconds"`
	SourcePrivateDataRead     bool     `json:"source_private_data_read"`
	Notes                     []string `json:"notes"`
}

func main() {
	root := flag.String("workspace", "", "empty private workspace for the generated SQLite measurement")
	operations := flag.Int("operations", publishedFullCorpusDocuments, "generated operation count")
	daily := flag.Int("daily-operations", 5000, "daily operation rate for the 90-day projection")
	flag.Parse()
	if *root == "" || *operations <= 0 || *daily <= 0 {
		fail(fmt.Errorf("-workspace, positive -operations, and positive -daily-operations are required"))
	}
	if err := os.MkdirAll(*root, 0o700); err != nil {
		fail(err)
	}
	database := filepath.Join(*root, "retention-cost.sqlite")
	if _, err := os.Stat(database); err == nil {
		fail(fmt.Errorf("workspace database already exists"))
	}
	st, err := store.OpenSQLite(database)
	if err != nil {
		fail(err)
	}
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		fail(err)
	}
	journal, err := st.EnrollLocalJournal(ctx, "G17 generated disk-cost evidence")
	if err != nil {
		fail(err)
	}
	if err := st.Exec(ctx, "VACUUM; PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		fail(err)
	}
	baseline := fileSize(database)
	started := time.Now()
	sql := fmt.Sprintf(`BEGIN IMMEDIATE;
		WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<%d)
		INSERT INTO sync_operations(replica_id, sequence, operation_id, kind, record_type,
			record_id, payload_json, hlc_wall_ms, hlc_logical, created_at)
		SELECT %q, x, 'op_generated_' || printf('%%012d', x), 'document.update', 'document',
			'doc_generated_' || printf('%%012d', x),
			'{"title":"generated retention evidence","notebook_id":"default","metadata":{"source":"generated"}}',
			1780000000000 + x, 0, '2026-01-01T00:00:00Z' FROM n;
		COMMIT; PRAGMA wal_checkpoint(TRUNCATE);`, *operations, journal.ReplicaID)
	if err := st.Exec(ctx, sql); err != nil {
		fail(err)
	}
	if err := st.Close(); err != nil {
		fail(err)
	}
	retained := fileSize(database)
	delta := retained - baseline
	perOperation := float64(delta) / float64(*operations)
	projectedOperations := int64(*daily) * 90
	result := report{
		Schema: "notrios.g17.retention-cost.v1", GeneratedFor: "v0.7 G17 retention horizon default",
		SchemaVersion: store.CurrentSchemaVersion, PublishedCorpusDocuments: publishedFullCorpusDocuments,
		GeneratedOperations: *operations, BaselineDatabaseBytes: baseline, RetainedDatabaseBytes: retained,
		IncrementalBytes: delta, BytesPerOperation: perOperation, NinetyDayDailyOperations: *daily,
		NinetyDayProjectedBytes:   int64(perOperation * float64(projectedOperations)),
		FullCorpusChurnEquivalent: float64(*operations) / float64(publishedFullCorpusDocuments),
		ElapsedMilliseconds:       time.Since(started).Milliseconds(), SourcePrivateDataRead: false,
		Notes: []string{
			"The 382,206-document count is the aggregate published by G14b/G14e; no corpus title, body, path, or database is opened.",
			"The generated workload retains one representative metadata operation per full-corpus document in the production schema, including production indexes.",
			"The 5,000-operations/day scenario is a transparent projection, not an observed user edit rate; actual cost scales with operation count and payload size.",
			"Snapshot bytes are not added here because G14b/G14e already measure the independently required retained physical snapshot.",
		},
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(raw))
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		fail(err)
	}
	return info.Size()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
