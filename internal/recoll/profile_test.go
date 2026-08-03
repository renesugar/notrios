package recoll

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/query"
	"github.com/renesugar/notrios/internal/store"
)

type hardeningProfile struct {
	GeneratedAt       string                     `json:"generated_at"`
	DocumentCount     int                        `json:"document_count"`
	GeneratedBytes    int64                      `json:"generated_bytes"`
	BatchSize         int                        `json:"batch_size"`
	ExtractionMode    string                     `json:"extraction_mode"`
	RecollVersion     string                     `json:"recoll_version"`
	GoVersion         string                     `json:"go_version"`
	GOOS              string                     `json:"goos"`
	GOARCH            string                     `json:"goarch"`
	PeakRSSBytes      int64                      `json:"peak_rss_bytes"`
	GenerateDuration  time.Duration              `json:"generate_duration"`
	InitialIndex      time.Duration              `json:"initial_index_duration"`
	SelectiveQuery    time.Duration              `json:"selective_query_duration"`
	BoundedQuery      time.Duration              `json:"bounded_query_duration"`
	BoundedQueryHits  int                        `json:"bounded_query_hits"`
	IncrementalIndex  time.Duration              `json:"incremental_index_duration"`
	DeletedHitRemoved bool                       `json:"deleted_hit_removed"`
	NewHitFound       bool                       `json:"new_hit_found"`
	DamageRepair      projection.ReconcileReport `json:"damage_repair"`
	Converged         projection.ReconcileReport `json:"converged"`
}

// TestRecollHardeningProfile is opt-in native evidence. The reviewed H10 run
// uses 100,000 generated Markdown projections; 100 is allowed for a quick
// script/test sanity pass.
func TestRecollHardeningProfile(t *testing.T) {
	value := strings.TrimSpace(os.Getenv("NOTRIOS_RECOLL_PROFILE"))
	if value == "" {
		t.Skip("set NOTRIOS_RECOLL_PROFILE to 100 or 100000")
	}
	count, err := strconv.Atoi(value)
	if err != nil || count != 100 && count != 100_000 {
		t.Fatalf("NOTRIOS_RECOLL_PROFILE must be 100 or 100000; got %q", value)
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg-config"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(dir, "xdg-runtime"))
	if err := os.MkdirAll(os.Getenv("XDG_RUNTIME_DIR"), 0o700); err != nil {
		t.Fatal(err)
	}
	sidecar := New(filepath.Join(dir, "conf"), filepath.Join(dir, "projection"), "recollindex")
	if !sidecar.Available() {
		t.Skip("recollindex/recollq not installed")
	}
	if err := sidecar.EnsureConfig(); err != nil {
		t.Fatal(err)
	}
	extractionMode := "notrios-frontmatter-handler"
	if count >= 100_000 {
		// The production handler is validated against real Recoll by
		// TestLiveRecollPipeline. At scale, isolate native index/query behavior
		// from per-document Python startup cost while retaining the real .md
		// projection paths and Sidecar query/result boundary.
		extractionMode = "recoll-internal-plain-text"
		if err := os.WriteFile(filepath.Join(sidecar.ConfDir, "mimeconf"),
			[]byte("[index]\ntext/markdown = internal text/plain\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	notesDir := filepath.Join(sidecar.ProjectionDir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	profile := hardeningProfile{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), DocumentCount: count, BatchSize: 200,
		ExtractionMode: extractionMode,
		GoVersion:      runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}
	if output, _ := exec.Command("recollindex", "--version").CombinedOutput(); len(output) > 0 {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "Recoll version:") {
				profile.RecollVersion = strings.TrimSpace(strings.TrimPrefix(line, "Recoll version:"))
			}
		}
	}
	generateStarted := time.Now()
	for index := 0; index < count; index++ {
		body := fmt.Sprintf("---\nid: profile_%06d\ntitle: Profile %06d\ntags:\n  - generated\n---\n\ncommonterm profiletoken%06d generated native Recoll evidence.\n",
			index, index, index)
		profile.GeneratedBytes += int64(len(body))
		if err := os.WriteFile(filepath.Join(notesDir, fmt.Sprintf("profile_%06d.md", index)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	profile.GenerateDuration = time.Since(generateStarted)

	ctx := context.Background()
	parse := func(value string) query.Query {
		t.Helper()
		parsed, err := query.Parse(value, time.Now())
		if err != nil {
			t.Fatalf("parse profile query %q: %v", value, err)
		}
		return parsed
	}
	indexStarted := time.Now()
	if err := sidecar.Index(ctx); err != nil {
		t.Fatalf("initial index: %v", err)
	}
	profile.InitialIndex = time.Since(indexStarted)

	selectiveStarted := time.Now()
	selective, err := sidecar.Search(ctx, parse(fmt.Sprintf("profiletoken%06d", count-1)), 10)
	if err != nil || len(selective) != 1 {
		t.Fatalf("selective query: hits=%d err=%v", len(selective), err)
	}
	profile.SelectiveQuery = time.Since(selectiveStarted)
	boundedStarted := time.Now()
	bounded, err := sidecar.Search(ctx, parse("commonterm"), 1001)
	if err != nil {
		t.Fatalf("bounded query: %v", err)
	}
	profile.BoundedQuery = time.Since(boundedStarted)
	profile.BoundedQueryHits = len(bounded)
	if want := min(count, 1001); len(bounded) != want {
		t.Fatalf("bounded query hits=%d want=%d", len(bounded), want)
	}

	if err := os.Remove(filepath.Join(notesDir, "profile_000010.md")); err != nil {
		t.Fatal(err)
	}
	newBody := "---\nid: profile_new\ntitle: New drift note\n---\n\nnewdrifttoken\n"
	if err := os.WriteFile(filepath.Join(notesDir, "profile_new.md"), []byte(newBody), 0o644); err != nil {
		t.Fatal(err)
	}
	incrementalStarted := time.Now()
	if err := sidecar.Index(ctx); err != nil {
		t.Fatalf("incremental index: %v", err)
	}
	profile.IncrementalIndex = time.Since(incrementalStarted)
	deleted, err := sidecar.Search(ctx, parse("profiletoken000010"), 10)
	if err != nil {
		t.Fatal(err)
	}
	added, err := sidecar.Search(ctx, parse("newdrifttoken"), 10)
	if err != nil {
		t.Fatal(err)
	}
	profile.DeletedHitRemoved = len(deleted) == 0
	profile.NewHitFound = len(added) == 1 && added[0].DocumentID == "profile_new"
	if !profile.DeletedHitRemoved || !profile.NewHitFound {
		t.Fatalf("incremental drift did not converge: deleted=%+v added=%+v", deleted, added)
	}

	profile.DamageRepair, profile.Converged = profileReconciliation(t, ctx, dir)
	profile.PeakRSSBytes = profilePeakRSS()
	raw, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if output := strings.TrimSpace(os.Getenv("NOTRIOS_RECOLL_PROFILE_OUTPUT")); output != "" {
		if err := os.WriteFile(output, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Recoll hardening profile:\n%s", raw)
}

func profileReconciliation(t *testing.T, ctx context.Context, root string) (projection.ReconcileReport, projection.ReconcileReport) {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 100; index++ {
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			Title: fmt.Sprintf("Reconcile %03d", index), Body: fmt.Sprintf("body %03d", index),
		}); err != nil {
			t.Fatal(err)
		}
	}
	writer := projection.Writer{Dir: filepath.Join(root, "reconciliation")}
	initial, err := projection.Reconcile(ctx, st, writer, 20)
	if err != nil || !initial.Complete || initial.Missing != 100 {
		t.Fatalf("initial reconciliation: %+v err=%v", initial, err)
	}
	entries, err := os.ReadDir(filepath.Join(writer.Dir, "notes"))
	if err != nil || len(entries) != 100 {
		t.Fatalf("projection inventory: entries=%d err=%v", len(entries), err)
	}
	if err := os.Remove(filepath.Join(writer.Dir, "notes", entries[0].Name())); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writer.Dir, "notes", entries[1].Name()), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writer.Dir, "notes", "orphan.md"), []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}
	damaged, err := projection.Reconcile(ctx, st, writer, 20)
	if err != nil || !damaged.Complete || damaged.Missing != 1 || damaged.Stale != 1 || damaged.Orphaned != 1 {
		t.Fatalf("damage reconciliation: %+v err=%v", damaged, err)
	}
	converged, err := projection.Reconcile(ctx, st, writer, 20)
	if err != nil || !converged.Complete || converged.Missing != 0 || converged.Stale != 0 || converged.Orphaned != 0 {
		t.Fatalf("converged reconciliation: %+v err=%v", converged, err)
	}
	return damaged, converged
}

func profilePeakRSS() int64 {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var kib int64
		if _, err := fmt.Sscanf(scanner.Text(), "VmHWM: %d kB", &kib); err == nil {
			return kib * 1024
		}
	}
	return 0
}
