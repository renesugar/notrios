package snapshotimage

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/store"
)

func testSource(t *testing.T) (*store.SQLiteStore, string) {
	t.Helper()
	root := t.TempDir()
	assets := filepath.Join(root, "assets")
	database := filepath.Join(root, "source.sqlite")
	st, err := store.OpenSQLiteWithAssetStore(database, assets)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		st.Close()
		t.Fatal(err)
	}
	document, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{Title: "physical snapshot", Body: "canonical body\n"})
	if err != nil {
		st.Close()
		t.Fatal(err)
	}
	for index, value := range []string{strings.Repeat("a", 700), strings.Repeat("b", 700), strings.Repeat("c", 1500)} {
		resource, err := st.CreateResource(context.Background(), store.CreateResourceRequest{Filename: "fixture", MIMEType: "application/octet-stream", Content: strings.NewReader(value)})
		if err != nil {
			st.Close()
			t.Fatal(err)
		}
		if _, err := st.AttachDocumentResource(context.Background(), store.AttachResourceRequest{DocumentID: document.ID, ResourceID: resource.ID, RelationType: "attachment", Ordinal: index}); err != nil {
			st.Close()
			t.Fatal(err)
		}
	}
	return st, assets
}

func createTestSnapshot(t *testing.T) (string, CreateReport) {
	t.Helper()
	st, assets := testSource(t)
	defer st.Close()
	target := filepath.Join(t.TempDir(), "snapshot")
	report, err := Create(context.Background(), st, assets, target, CreateOptions{PackTargetBytes: 1024, PackMaxEntries: 2})
	if err != nil {
		t.Fatal(err)
	}
	return target, report
}

func TestCreateVerifyAndDeterministicPacks(t *testing.T) {
	st, assets := testSource(t)
	defer st.Close()
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	firstReport, err := Create(context.Background(), st, assets, first, CreateOptions{PackTargetBytes: 1024, PackMaxEntries: 2})
	if err != nil {
		t.Fatal(err)
	}
	secondReport, err := Create(context.Background(), st, assets, second, CreateOptions{PackTargetBytes: 1024, PackMaxEntries: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !firstReport.Verified || !secondReport.Verified || firstReport.Objects != 3 || firstReport.Packs != 3 {
		t.Fatalf("unexpected reports: first=%+v second=%+v", firstReport, secondReport)
	}
	firstManifest, err := readManifest(first, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	secondManifest, err := readManifest(second, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for index := range firstManifest.External.Packs {
		if firstManifest.External.Packs[index].SHA256 != secondManifest.External.Packs[index].SHA256 || firstManifest.External.Packs[index].SizeBytes != secondManifest.External.Packs[index].SizeBytes {
			t.Fatalf("pack %d is not deterministic", index)
		}
	}
	oversized := 0
	for _, pack := range firstManifest.External.Packs {
		if pack.Oversized {
			oversized++
		}
	}
	if oversized != 1 {
		t.Fatal("single object above the target was not declared oversized")
	}
}

func TestInterruptedPublicationResumesOnlyVerifiedPacks(t *testing.T) {
	st, assets := testSource(t)
	defer st.Close()
	target := filepath.Join(t.TempDir(), "snapshot")
	wantFailure := errors.New("injected publication failure")
	_, err := Create(context.Background(), st, assets, target, CreateOptions{
		PackTargetBytes: 1024, PackMaxEntries: 2,
		AfterPublish: func(stage string) error {
			if stage == "pack:0" {
				return wantFailure
			}
			return nil
		},
	})
	if !errors.Is(err, wantFailure) {
		t.Fatalf("fault injection: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ManifestFile)); !os.IsNotExist(err) {
		t.Fatal("failed publication exposed a completion manifest")
	}
	pack := filepath.Join(target, "packs", "pack-000000.tar")
	before, err := os.Stat(pack)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	report, err := Create(context.Background(), st, assets, target, CreateOptions{PackTargetBytes: 1024, PackMaxEntries: 2})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(pack)
	if err != nil {
		t.Fatal(err)
	}
	if report.ResumedPacks < 1 || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("verified pack was not resumed: report=%+v before=%v after=%v", report, before.ModTime(), after.ModTime())
	}
}

func TestManifestNeverPrecedesDatabasePublication(t *testing.T) {
	for _, faultStage := range []string{"database", "before_manifest"} {
		t.Run(faultStage, func(t *testing.T) {
			st, assets := testSource(t)
			defer st.Close()
			target := filepath.Join(t.TempDir(), "snapshot")
			injected := errors.New("injected " + faultStage)
			_, err := Create(context.Background(), st, assets, target, CreateOptions{
				PackTargetBytes: 1024, PackMaxEntries: 2,
				AfterPublish: func(stage string) error {
					if stage == faultStage {
						return injected
					}
					return nil
				},
			})
			if !errors.Is(err, injected) {
				t.Fatalf("fault %s: %v", faultStage, err)
			}
			if _, err := os.Stat(filepath.Join(target, ManifestFile)); !os.IsNotExist(err) {
				t.Fatalf("fault %s exposed manifest", faultStage)
			}
			if _, err := VerifyDirectory(context.Background(), target, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "incomplete snapshot") {
				t.Fatalf("fault %s did not remain explicitly incomplete: %v", faultStage, err)
			}
		})
	}
}

func TestVerificationRejectsCorruptionTruncationAndPathAttack(t *testing.T) {
	t.Run("database corruption", func(t *testing.T) {
		root, _ := createTestSnapshot(t)
		file, err := os.OpenFile(filepath.Join(root, DatabaseFile), os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteAt([]byte{0xff}, 100); err != nil {
			t.Fatal(err)
		}
		file.Close()
		if _, err := VerifyDirectory(context.Background(), root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "database hash") {
			t.Fatalf("database corruption accepted: %v", err)
		}
	})

	t.Run("pack truncation", func(t *testing.T) {
		root, _ := createTestSnapshot(t)
		pack := filepath.Join(root, "packs", "pack-000000.tar")
		info, _ := os.Stat(pack)
		if err := os.Truncate(pack, info.Size()-1); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(context.Background(), root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "hash or length") {
			t.Fatalf("truncated pack accepted: %v", err)
		}
	})

	t.Run("tar traversal", func(t *testing.T) {
		root, _ := createTestSnapshot(t)
		manifest, err := readManifest(root, DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		packPath := filepath.Join(root, filepath.FromSlash(manifest.External.Packs[0].Path))
		valid, err := os.Open(packPath)
		if err != nil {
			t.Fatal(err)
		}
		reader := tar.NewReader(valid)
		header, err := reader.Next()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		valid.Close()
		if err != nil {
			t.Fatal(err)
		}
		var malicious bytes.Buffer
		writer := tar.NewWriter(&malicious)
		header.Name = "../escape"
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(body); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(packPath, malicious.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		sha, size, err := hashRegularFile(packPath, DefaultLimits().MaxTotalBytes)
		if err != nil {
			t.Fatal(err)
		}
		manifest.External.Packs[0].SHA256 = sha
		manifest.External.Packs[0].SizeBytes = size
		if err := finalizeManifest(&manifest); err != nil {
			t.Fatal(err)
		}
		if err := publishManifest(root, manifest); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(context.Background(), root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "unsafe path") {
			t.Fatalf("path traversal accepted: %v", err)
		}
	})

	t.Run("tar expansion declaration", func(t *testing.T) {
		root, _ := createTestSnapshot(t)
		manifest, err := readManifest(root, DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		packPath := filepath.Join(root, filepath.FromSlash(manifest.External.Packs[0].Path))
		raw, err := os.ReadFile(packPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) < 512 {
			t.Fatal("pack is too small for a tar header")
		}
		copy(raw[124:136], []byte("77777777777\x00"))
		for index := 148; index < 156; index++ {
			raw[index] = ' '
		}
		checksum := 0
		for _, value := range raw[:512] {
			checksum += int(value)
		}
		copy(raw[148:156], []byte(fmt.Sprintf("%06o\x00 ", checksum)))
		if err := os.WriteFile(packPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		sha, size, err := hashRegularFile(packPath, DefaultLimits().MaxTotalBytes)
		if err != nil {
			t.Fatal(err)
		}
		manifest.External.Packs[0].SHA256, manifest.External.Packs[0].SizeBytes = sha, size
		if err := finalizeManifest(&manifest); err != nil {
			t.Fatal(err)
		}
		if err := publishManifest(root, manifest); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(context.Background(), root, DefaultLimits()); err == nil {
			t.Fatal("expanding tar size declaration was accepted")
		}
	})

	t.Run("pack symlink", func(t *testing.T) {
		root, _ := createTestSnapshot(t)
		pack := filepath.Join(root, "packs", "pack-000000.tar")
		moved := filepath.Join(t.TempDir(), "pack.tar")
		if err := os.Rename(pack, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(moved, pack); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(context.Background(), root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "not a real regular file") {
			t.Fatalf("pack symlink accepted: %v", err)
		}
	})
}

func TestReaderMatrixKeepsArchiveV2Separate(t *testing.T) {
	archiveRoot := filepath.Join("..", "archivev2", "testdata", "golden-minimal")
	if _, err := archivev2.VerifyDirectory(archiveRoot, archivev2.DefaultLimits()); err != nil {
		t.Fatalf("current archive-v2 reader lost its golden fixture: %v", err)
	}
	if _, err := VerifyDirectory(context.Background(), archiveRoot, DefaultLimits()); err == nil {
		t.Fatal("physical reader accepted a semantic archive-v2 fixture")
	}
	snapshotRoot, _ := createTestSnapshot(t)
	if _, err := archivev2.VerifyDirectory(snapshotRoot, archivev2.DefaultLimits()); err == nil {
		t.Fatal("archive-v2 reader accepted a physical snapshot")
	}
}
