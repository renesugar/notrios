package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type scaleProfile struct {
	GeneratedAt     string                        `json:"generated_at"`
	DocumentCount   int                           `json:"document_count"`
	ResourceCount   int64                         `json:"resource_count"`
	PhysicalBlobs   int64                         `json:"physical_blob_count"`
	LinkCount       int64                         `json:"link_count"`
	DatabaseBytes   int64                         `json:"database_bytes"`
	PeakRSSBytes    int64                         `json:"peak_rss_bytes"`
	Environment     scaleProfileEnvironment       `json:"environment"`
	Metrics         map[string]scaleProfileMetric `json:"metrics"`
	QueryPlans      map[string][]string           `json:"query_plans"`
	PageTargetP95MS float64                       `json:"ordinary_page_target_p95_ms"`
	PageTargetMet   bool                          `json:"ordinary_page_target_met"`
}

type scaleProfileEnvironment struct {
	GOOS          string `json:"goos"`
	GOARCH        string `json:"goarch"`
	GoVersion     string `json:"go_version"`
	SQLiteVersion string `json:"sqlite_version"`
	CPUModel      string `json:"cpu_model"`
}

type scaleProfileMetric struct {
	Iterations int     `json:"iterations"`
	P50MS      float64 `json:"p50_ms"`
	P95MS      float64 `json:"p95_ms"`
	MaxMS      float64 `json:"max_ms"`
	Items      int     `json:"items,omitempty"`
}

// TestLargeLibraryProfile is opt-in because the 100k and 500k cases are
// evidence runs, not ordinary unit tests. scripts/run_large_library_profile.sh
// supplies one of the three reviewed H7 sizes.
func TestLargeLibraryProfile(t *testing.T) {
	countText := strings.TrimSpace(os.Getenv("NOTRIOS_SCALE_PROFILE"))
	if countText == "" {
		t.Skip("set NOTRIOS_SCALE_PROFILE to 10000, 100000, or 500000")
	}
	count, err := strconv.Atoi(countText)
	if err != nil || (count != 10_000 && count != 100_000 && count != 500_000) {
		t.Fatalf("unsupported NOTRIOS_SCALE_PROFILE %q", countText)
	}

	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scale.sqlite")
	st, err := OpenSQLiteWithAssetStore(dbPath, filepath.Join(dir, "assets"))
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	seedStarted := time.Now()
	seedLargeLibrary(t, ctx, st, count)
	seedElapsed := time.Since(seedStarted)
	if err := st.Exec(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	profile := scaleProfile{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		DocumentCount: count,
		Environment: scaleProfileEnvironment{
			GOOS:      runtime.GOOS,
			GOARCH:    runtime.GOARCH,
			GoVersion: runtime.Version(),
			CPUModel:  cpuModel(),
		},
		Metrics:         map[string]scaleProfileMetric{},
		QueryPlans:      map[string][]string{},
		PageTargetP95MS: 100,
	}
	profile.Environment.SQLiteVersion, _ = st.sqliteVersion(ctx)
	profile.ResourceCount = profileCount(t, st, `SELECT COUNT(*) FROM resources`)
	profile.PhysicalBlobs = profileCount(t, st, `SELECT COUNT(*) FROM blobs`)
	profile.LinkCount = profileCount(t, st, `SELECT COUNT(*) FROM document_links`)
	if info, err := os.Stat(dbPath); err == nil {
		profile.DatabaseBytes = info.Size()
	}
	profile.Metrics["synthetic_import_batch_seed"] = scaleProfileMetric{
		Iterations: 1,
		P50MS:      durationMS(seedElapsed),
		P95MS:      durationMS(seedElapsed),
		MaxMS:      durationMS(seedElapsed),
		Items:      count,
	}

	first, err := st.Search(ctx, SearchRequest{Limit: 100})
	if err != nil || len(first.Hits) != 100 || first.NextCursor == "" {
		t.Fatalf("first keyset page: hits=%d cursor=%t err=%v", len(first.Hits), first.NextCursor != "", err)
	}
	profile.Metrics["all_notes_first_page"] = measureProfile(t, 30, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Limit: 100})
		return err
	})
	profile.Metrics["all_notes_next_page"] = measureProfile(t, 30, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Limit: 100, Cursor: first.NextCursor})
		return err
	})

	deepCursor := ""
	deepPages := (count * 9 / 10) / 100
	for page := 0; page < deepPages; page++ {
		result, err := st.Search(ctx, SearchRequest{Limit: 100, Cursor: deepCursor})
		if err != nil {
			t.Fatalf("walk to deep cursor page %d: %v", page, err)
		}
		deepCursor = result.NextCursor
		if deepCursor == "" {
			t.Fatalf("deep cursor exhausted at page %d", page)
		}
	}
	profile.Metrics["all_notes_90_percent_deep_page"] = measureProfile(t, 30, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Limit: 100, Cursor: deepCursor})
		return err
	})

	selective := fmt.Sprintf("unique%06d", count/2)
	profile.Metrics["fts_selective_first_page"] = measureProfile(t, 15, 1, func() error {
		_, err := st.Search(ctx, SearchRequest{Query: selective, Limit: 100})
		return err
	})
	profile.Metrics["fts_nonselective_first_page"] = measureProfile(t, 5, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Query: "commonterm group7", Limit: 100})
		return err
	})
	profile.Metrics["notebook_filter_first_page"] = measureProfile(t, 30, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Query: `notebook:"Scale 2"`, Limit: 100})
		return err
	})
	profile.Metrics["tag_filter_first_page"] = measureProfile(t, 30, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Query: `tag:scale_tag_7`, Limit: 100})
		return err
	})
	profile.Metrics["boolean_text_or_first_page"] = measureProfile(t, 15, 2, func() error {
		_, err := st.Search(ctx, SearchRequest{Query: `unique000001 OR unique000002`, Limit: 100})
		return err
	})
	profile.Metrics["boolean_negated_field_first_page"] = measureProfile(t, 15, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Query: `commonterm -tag:scale_tag_7`, Limit: 100})
		return err
	})
	profile.Metrics["category_alias_first_page"] = measureProfile(t, 30, 100, func() error {
		_, err := st.Search(ctx, SearchRequest{Query: `category:"Scale 2"`, Limit: 100})
		return err
	})

	precedence, err := st.Search(ctx, SearchRequest{Query: `unique000001 OR unique000002 group2`, Limit: 10})
	if err != nil || len(precedence.Hits) != 2 {
		t.Fatalf("generated boolean precedence: hits=%d err=%v", len(precedence.Hits), err)
	}
	negated, err := st.Search(ctx, SearchRequest{Query: `(unique000001 OR unique000002) -tag:scale_tag_1`, Limit: 10})
	if err != nil || len(negated.Hits) != 1 || negated.Hits[0].ID != "scale_doc_000002" {
		t.Fatalf("generated grouped negation: hits=%+v err=%v", negated.Hits, err)
	}
	notebookPage, _ := st.Search(ctx, SearchRequest{Query: `notebook:"Scale 2"`, Limit: 100})
	categoryPage, _ := st.Search(ctx, SearchRequest{Query: `category:"Scale 2"`, Limit: 100})
	if fmt.Sprint(sortedHitIDs(notebookPage)) != fmt.Sprint(sortedHitIDs(categoryPage)) {
		t.Fatal("generated category alias differs from notebook filter")
	}

	streamStarted := time.Now()
	streamed := 0
	cursor := ""
	for {
		page, err := st.Search(ctx, SearchRequest{Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatalf("export cursor stream: %v", err)
		}
		streamed += len(page.Hits)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if streamed != count {
		t.Fatalf("export cursor stream visited %d documents, want %d", streamed, count)
	}
	streamElapsed := time.Since(streamStarted)
	profile.Metrics["export_cursor_stream"] = scaleProfileMetric{
		Iterations: 1,
		P50MS:      durationMS(streamElapsed),
		P95MS:      durationMS(streamElapsed),
		MaxMS:      durationMS(streamElapsed),
		Items:      streamed,
	}

	profile.QueryPlans["all_notes"], _ = st.explainQueryPlan(ctx, `SELECT id FROM documents
		WHERE collection_id = ? AND deleted_at IS NULL
		ORDER BY updated_at DESC, id DESC LIMIT 101`, "default")
	profile.QueryPlans["notebook"], _ = st.explainQueryPlan(ctx, `SELECT id FROM documents
		WHERE notebook_id = ? AND deleted_at IS NULL
		ORDER BY updated_at DESC, id DESC LIMIT 101`, "scale_nb_2")
	profile.QueryPlans["tag_membership"], _ = st.explainQueryPlan(ctx, `SELECT document_id FROM note_tags
		WHERE tag_id = ? ORDER BY document_id`, "scale_tag_7")

	profile.PeakRSSBytes = peakRSSBytes()
	profile.PageTargetMet = profile.Metrics["all_notes_first_page"].P95MS < profile.PageTargetP95MS &&
		profile.Metrics["all_notes_next_page"].P95MS < profile.PageTargetP95MS &&
		profile.Metrics["all_notes_90_percent_deep_page"].P95MS < profile.PageTargetP95MS

	raw, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	raw = append(raw, '\n')
	if output := strings.TrimSpace(os.Getenv("NOTRIOS_PROFILE_OUTPUT")); output != "" {
		if err := os.WriteFile(output, raw, 0o644); err != nil {
			t.Fatalf("write profile: %v", err)
		}
	}
	t.Logf("large-library profile:\n%s", raw)
	if !profile.PageTargetMet {
		t.Fatalf("ordinary page p95 exceeded %.0fms target", profile.PageTargetP95MS)
	}
}

func seedLargeLibrary(t *testing.T, ctx context.Context, st *SQLiteStore, count int) {
	t.Helper()
	digits := `WITH digits(d) AS (
			VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9)
		), seq(n) AS (
			SELECT a.d + 10*b.d + 100*c.d + 1000*d.d + 10000*e.d + 100000*f.d
			FROM digits a, digits b, digits c, digits d, digits e, digits f
		)`
	statements := []string{
		`BEGIN IMMEDIATE`,
		`INSERT OR IGNORE INTO notebooks(id, name, position) VALUES
			('scale_nb_0', 'Scale 0', 100),
			('scale_nb_1', 'Scale 1', 101),
			('scale_nb_2', 'Scale 2', 102),
			('scale_nb_3', 'Scale 3', 103)`,
		`INSERT OR IGNORE INTO tags(id, name) VALUES
			('scale_tag_0', 'scale_tag_0'), ('scale_tag_1', 'scale_tag_1'),
			('scale_tag_2', 'scale_tag_2'), ('scale_tag_3', 'scale_tag_3'),
			('scale_tag_4', 'scale_tag_4'), ('scale_tag_5', 'scale_tag_5'),
			('scale_tag_6', 'scale_tag_6'), ('scale_tag_7', 'scale_tag_7'),
			('scale_tag_8', 'scale_tag_8'), ('scale_tag_9', 'scale_tag_9')`,
		digits + fmt.Sprintf(` INSERT INTO documents(
				id, collection_id, notebook_id, title, current_revision_id, created_at, updated_at
			)
			SELECT printf('scale_doc_%%06d', n), 'default',
				printf('scale_nb_%%d', n %% 4),
				printf('Scale document %%06d', n),
				printf('scale_rev_%%06d', n),
				datetime('2025-01-01', printf('+%%d seconds', n)),
				datetime('2025-01-01', printf('+%%d seconds', n))
			FROM seq WHERE n < %d`, count),
		digits + fmt.Sprintf(` INSERT INTO document_revisions(
				id, document_id, title, body, message, created_at
			)
			SELECT printf('scale_rev_%%06d', n), printf('scale_doc_%%06d', n),
				printf('Scale document %%06d', n),
				printf('commonterm group%%d unique%%06d generated scale body', n %% 10, n),
				'large-library profile',
				datetime('2025-01-01', printf('+%%d seconds', n))
			FROM seq WHERE n < %d`, count),
		`INSERT INTO documents_fts(document_id, collection_id, title, body)
			SELECT d.id, d.collection_id, r.title, r.body
			FROM documents d JOIN document_revisions r ON r.id = d.current_revision_id
			WHERE d.id LIKE 'scale_doc_%'`,
		digits + fmt.Sprintf(` INSERT INTO note_tags(document_id, tag_id)
			SELECT printf('scale_doc_%%06d', n), printf('scale_tag_%%d', n %% 10)
			FROM seq WHERE n < %d`, count),
		digits + fmt.Sprintf(` INSERT INTO blobs(sha256, storage_path, size_bytes, mime_type)
			SELECT printf('%%064x', n), printf('synthetic/%%06d.bin', n), 128, 'application/octet-stream'
			FROM seq WHERE n < %d`, (count+999)/1000),
		digits + fmt.Sprintf(` INSERT INTO resources(
				id, collection_id, blob_sha256, filename, mime_type
			)
			SELECT printf('scale_res_%%06d', n), 'default',
				printf('%%064x', n / 10), printf('resource-%%06d.bin', n),
				'application/octet-stream'
			FROM seq WHERE n < %d`, (count+99)/100),
		digits + fmt.Sprintf(` INSERT INTO document_resource_refs(
				document_id, resource_id, relation_type, ordinal
			)
			SELECT printf('scale_doc_%%06d', n * 100), printf('scale_res_%%06d', n),
				'attachment', 0
			FROM seq WHERE n < %d`, (count+99)/100),
		digits + fmt.Sprintf(` INSERT INTO document_links(
				source_document_id, target_document_id, relation_type, source_format,
				raw_target, resolution_status
			)
			SELECT printf('scale_doc_%%06d', n), printf('scale_doc_%%06d', (n + 1) %% %d),
				'link', 'markdown', 'next', 'resolved'
			FROM seq WHERE n < %d
			UNION ALL
			SELECT printf('scale_doc_%%06d', n), printf('scale_doc_%%06d', (n + 2) %% %d),
				'link', 'markdown', 'next2', 'resolved'
			FROM seq WHERE n < %d`, count, count, count, count),
		`COMMIT`,
	}
	for _, statement := range statements {
		if err := st.Exec(ctx, statement); err != nil {
			_ = st.Exec(ctx, `ROLLBACK`)
			t.Fatalf("seed statement failed: %v", err)
		}
	}
}

func profileCount(t *testing.T, st *SQLiteStore, sql string) int64 {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	count, err := st.countLocked(sql)
	if err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return count
}

func measureProfile(t *testing.T, iterations, items int, fn func() error) scaleProfileMetric {
	t.Helper()
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		started := time.Now()
		if err := fn(); err != nil {
			t.Fatalf("profile metric: %v", err)
		}
		samples = append(samples, time.Since(started))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p50 := samples[(len(samples)-1)/2]
	p95 := samples[(len(samples)*95+99)/100-1]
	return scaleProfileMetric{
		Iterations: iterations,
		P50MS:      durationMS(p50),
		P95MS:      durationMS(p95),
		MaxMS:      durationMS(samples[len(samples)-1]),
		Items:      items,
	}
}

func durationMS(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func peakRSSBytes() int64 {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 3 && fields[0] == "VmHWM:" {
			kib, _ := strconv.ParseInt(fields[1], 10, 64)
			return kib * 1024
		}
	}
	return 0
}

func cpuModel() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			if _, value, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}
