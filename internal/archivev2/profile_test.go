package archivev2

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

type exportProfile struct {
	GeneratedAt      string                   `json:"generated_at"`
	Tier             int                      `json:"tier"`
	Environment      exportProfileEnvironment `json:"environment"`
	SeedSeconds      float64                  `json:"seed_seconds"`
	Documents        int                      `json:"documents"`
	Resources        int                      `json:"reachable_resources"`
	Records          int                      `json:"records"`
	Objects          int                      `json:"objects"`
	RecordObjects    int                      `json:"record_objects"`
	BlobObjects      int                      `json:"blob_objects"`
	Deduplicated     int                      `json:"deduplicated_objects"`
	ArchiveBytes     int64                    `json:"archive_bytes"`
	ExportSeconds    float64                  `json:"export_seconds"`
	DocumentsPerSec  float64                  `json:"documents_per_second"`
	VerifySeconds    float64                  `json:"verify_seconds"`
	ResumeSeconds    float64                  `json:"resume_seconds"`
	ResumeReused     int                      `json:"resume_reused_objects"`
	ResumeRewritten  int64                    `json:"resume_bytes_rewritten"`
	SubsetSeconds    float64                  `json:"subset_seconds"`
	SubsetDocuments  int                      `json:"subset_documents"`
	PeakRSSBytes     int64                    `json:"peak_rss_bytes"`
	CommitStable     bool                     `json:"commit_stable_across_runs"`
	ObjectBudgetUsed float64                  `json:"object_budget_used_fraction"`
}

type exportProfileEnvironment struct {
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	GoVersion string `json:"go_version"`
	CPUModel  string `json:"cpu_model"`
}

// TestArchiveExportProfile is opt-in evidence, not an ordinary unit test.
// scripts/run_archive_export_profile.sh supplies one of the reviewed tiers.
// The largest tier is bounded by the archive-v2 object budget: one body object
// per revision plus resource, source-bundle, and record objects must stay
// within Limits.MaxObjects.
func TestArchiveExportProfile(t *testing.T) {
	tierText := strings.TrimSpace(os.Getenv("NOTRIOS_ARCHIVE_PROFILE"))
	if tierText == "" {
		t.Skip("set NOTRIOS_ARCHIVE_PROFILE to 100, 1000, or 5000")
	}
	tier, err := strconv.Atoi(tierText)
	if err != nil || (tier != 100 && tier != 1_000 && tier != 5_000) {
		t.Fatalf("unsupported NOTRIOS_ARCHIVE_PROFILE %q", tierText)
	}

	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(dir, "profile.sqlite"), filepath.Join(dir, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	seedStarted := time.Now()
	notebookIDs := seedExportProfile(t, ctx, st, tier)
	profile := exportProfile{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Tier:        tier,
		SeedSeconds: time.Since(seedStarted).Seconds(),
		Environment: exportProfileEnvironment{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), CPUModel: profileCPUModel()},
	}

	full := filepath.Join(dir, "full")
	exportStarted := time.Now()
	report, err := Export(ctx, st, full, ExportOptions{SkipVerification: true})
	if err != nil {
		t.Fatal(err)
	}
	profile.ExportSeconds = time.Since(exportStarted).Seconds()
	profile.Documents = report.Counts.Documents
	profile.Resources = report.ReachableResources
	profile.Records = report.Counts.Total()
	profile.Objects = report.Objects
	profile.RecordObjects = report.RecordObjects
	profile.BlobObjects = report.BlobObjects
	profile.Deduplicated = report.DeduplicatedObjects
	profile.ArchiveBytes = report.Bytes
	if profile.ExportSeconds > 0 {
		profile.DocumentsPerSec = float64(profile.Documents) / profile.ExportSeconds
	}
	profile.ObjectBudgetUsed = float64(report.Objects) / float64(DefaultLimits().MaxObjects)

	verifyStarted := time.Now()
	if _, err := VerifyDirectory(full, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	profile.VerifySeconds = time.Since(verifyStarted).Seconds()

	// A resumed export must reuse published objects rather than rewrite them.
	if err := os.Remove(filepath.Join(full, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	resumeStarted := time.Now()
	resumed, err := Export(ctx, st, full, ExportOptions{SkipVerification: true})
	if err != nil {
		t.Fatal(err)
	}
	profile.ResumeSeconds = time.Since(resumeStarted).Seconds()
	profile.ResumeReused = resumed.ReusedObjects
	profile.ResumeRewritten = resumed.BytesWritten
	profile.CommitStable = resumed.CommitSHA256 != "" && resumed.Counts == report.Counts

	subset := filepath.Join(dir, "subset")
	subsetStarted := time.Now()
	subsetReport, err := Export(ctx, st, subset, ExportOptions{
		Target:           TargetSubsetTransfer,
		Selection:        store.SelectionSpec{NotebookIDs: notebookIDs[:1]},
		SkipVerification: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile.SubsetSeconds = time.Since(subsetStarted).Seconds()
	profile.SubsetDocuments = subsetReport.Counts.Documents
	if _, err := VerifyDirectory(subset, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	profile.PeakRSSBytes = profilePeakRSSBytes()

	encoded, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("archive export profile: %s", encoded)
	if output := strings.TrimSpace(os.Getenv("NOTRIOS_PROFILE_OUTPUT")); output != "" {
		if err := os.WriteFile(output, append(encoded, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// seedExportProfile creates a generated library with nested notebooks, tags,
// deduplicated resources, and cross-note links. It returns the notebook IDs so
// the profile can also measure a bounded subset export.
func seedExportProfile(t *testing.T, ctx context.Context, st *store.SQLiteStore, count int) []string {
	t.Helper()
	notebookCount := 8
	notebookIDs := make([]string, 0, notebookCount)
	for index := 0; index < notebookCount; index++ {
		notebook, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{
			PreferredID: fmt.Sprintf("nb_profile_%02d", index),
			Name:        fmt.Sprintf("Profile %02d", index),
		})
		if err != nil {
			t.Fatal(err)
		}
		notebookIDs = append(notebookIDs, notebook.ID)
	}
	// One resource per 100 notes, each with distinct bytes so blob storage is
	// exercised without exhausting the object budget.
	resourceIDs := make([]string, 0, count/100+1)
	for index := 0; index <= count/100; index++ {
		resource, err := st.CreateResource(ctx, store.CreateResourceRequest{
			PreferredID: fmt.Sprintf("res_profile_%04d", index),
			Filename:    fmt.Sprintf("asset-%04d.bin", index),
			MIMEType:    "application/octet-stream",
			Content:     bytes.NewReader([]byte(strings.Repeat(fmt.Sprintf("asset-%04d;", index), 64))),
		})
		if err != nil {
			t.Fatal(err)
		}
		resourceIDs = append(resourceIDs, resource.ID)
	}
	for index := 0; index < count; index++ {
		id := fmt.Sprintf("doc_profile_%06d", index)
		body := fmt.Sprintf("Profile note %d.\n\n[previous](document://default/documents/doc_profile_%06d)\n\nBody padding: %s\n",
			index, max(index-1, 0), strings.Repeat("lorem ipsum ", 20))
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: id,
			NotebookID:  notebookIDs[index%notebookCount],
			Title:       fmt.Sprintf("Profile note %06d", index),
			Body:        body,
		}); err != nil {
			t.Fatal(err)
		}
		if index%25 == 0 {
			if _, err := st.AddDocumentTag(ctx, id, fmt.Sprintf("tag-%02d", index%13)); err != nil {
				t.Fatal(err)
			}
		}
		if index%100 == 0 {
			if _, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{
				DocumentID: id, ResourceID: resourceIDs[index/100], RelationType: "attachment",
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return notebookIDs
}

func profilePeakRSSBytes() int64 {
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

func profileCPUModel() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "model name") {
			if _, value, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}
