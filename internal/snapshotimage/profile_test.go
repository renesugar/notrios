package snapshotimage

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

type generatedProfile struct {
	GeneratedAt        string `json:"generated_at"`
	Documents          int    `json:"documents"`
	SeedMilliseconds   int64  `json:"seed_milliseconds"`
	CreateMilliseconds int64  `json:"create_milliseconds"`
	VerifyMilliseconds int64  `json:"verify_milliseconds"`
	DatabaseBytes      int64  `json:"database_bytes"`
	ExternalObjects    int64  `json:"external_objects"`
	Packs              int    `json:"packs"`
	HeapAllocBytes     uint64 `json:"heap_alloc_bytes"`
	PeakRSSBytes       int64  `json:"peak_rss_bytes"`
	GoVersion          string `json:"go_version"`
	BoundedMemory      bool   `json:"bounded_memory"`
}

const profileDigits = `WITH digits(d) AS (
	VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9)
), seq(n) AS (
	SELECT a.d + 10*b.d + 100*c.d + 1000*d.d + 10000*e.d + 100000*f.d
	FROM digits a, digits b, digits c, digits d, digits e, digits f
)`

// TestGenerated100KSnapshotProfile is opt-in evidence, not an ordinary unit
// test. The data is synthetic and remains under t.TempDir.
func TestGenerated100KSnapshotProfile(t *testing.T) {
	if os.Getenv("NOTRIOS_SNAPSHOT_PROFILE") != "100000" {
		t.Skip("set NOTRIOS_SNAPSHOT_PROFILE=100000")
	}
	ctx := context.Background()
	root := t.TempDir()
	assets := filepath.Join(root, "assets")
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "source.sqlite"), assets)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	seedStarted := time.Now()
	statements := []string{
		"BEGIN IMMEDIATE",
		profileDigits + ` INSERT INTO documents(id, collection_id, notebook_id, title, current_revision_id, created_at, updated_at)
			SELECT printf('snapshot_doc_%06d', n), 'default', 'nb_notes', printf('Snapshot %06d', n),
			       printf('snapshot_rev_%06d', n), datetime('2026-01-01', printf('+%d seconds', n)), datetime('2026-01-01', printf('+%d seconds', n))
			  FROM seq WHERE n < 100000`,
		profileDigits + ` INSERT INTO document_revisions(id, document_id, title, body, message, created_at)
			SELECT printf('snapshot_rev_%06d', n), printf('snapshot_doc_%06d', n), printf('Snapshot %06d', n),
			       printf('generated physical snapshot body %06d', n), 'G14c generated evidence', datetime('2026-01-01', printf('+%d seconds', n))
			  FROM seq WHERE n < 100000`,
		"COMMIT",
	}
	for _, statement := range statements {
		if err := st.Exec(ctx, statement); err != nil {
			_ = st.Exec(ctx, "ROLLBACK")
			t.Fatal(err)
		}
	}
	seedElapsed := time.Since(seedStarted)
	output := filepath.Join(root, "snapshot")
	createStarted := time.Now()
	created, err := Create(ctx, st, assets, output, CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	createElapsed := time.Since(createStarted)
	verifyStarted := time.Now()
	verified, err := VerifyDirectory(ctx, output, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	verifyElapsed := time.Since(verifyStarted)
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	profile := generatedProfile{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Documents: 100000,
		SeedMilliseconds: seedElapsed.Milliseconds(), CreateMilliseconds: createElapsed.Milliseconds(),
		VerifyMilliseconds: verifyElapsed.Milliseconds(), DatabaseBytes: created.DatabaseBytes,
		ExternalObjects: verified.Objects, Packs: verified.Packs, HeapAllocBytes: memory.Alloc,
		PeakRSSBytes: snapshotPeakRSS(), GoVersion: runtime.Version(), BoundedMemory: memory.Alloc < 256<<20,
	}
	if !created.Verified || !verified.ReadyForInstall || !profile.BoundedMemory {
		t.Fatalf("generated profile failed gates: %+v", profile)
	}
	if target := strings.TrimSpace(os.Getenv("NOTRIOS_SNAPSHOT_PROFILE_OUTPUT")); target != "" {
		file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(profile); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(profile)
	t.Logf("%s", raw)
}

func snapshotPeakRSS() int64 {
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
