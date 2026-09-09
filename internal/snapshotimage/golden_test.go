package snapshotimage

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/version"
)

// TestIndependentGoldenFixture assembles a complete fixture without calling
// Create, publishPack, publishManifest, or the production object cursor. The
// checked-in inputs and this deliberately direct encoder are the second writer
// used for the current-reader golden test.
func TestIndependentGoldenFixture(t *testing.T) {
	ctx := context.Background()
	work := t.TempDir()
	assets := filepath.Join(work, "assets")
	source, err := store.OpenSQLiteWithAssetStore(filepath.Join(work, "source.sqlite"), assets)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := source.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"object-a.txt", "object-b.txt"} {
		file, err := os.Open(filepath.Join("testdata", "golden-minimal", name))
		if err != nil {
			t.Fatal(err)
		}
		_, createErr := source.CreateResource(ctx, store.CreateResourceRequest{Filename: name, MIMEType: "text/plain", Content: file})
		file.Close()
		if createErr != nil {
			t.Fatal(createErr)
		}
	}

	root := filepath.Join(work, "fixture")
	if err := os.MkdirAll(filepath.Join(root, "packs"), 0o700); err != nil {
		t.Fatal(err)
	}
	state, err := source.CreateSQLiteSnapshotImage(ctx, filepath.Join(root, DatabaseFile))
	if err != nil {
		t.Fatal(err)
	}
	image, err := store.OpenSQLiteSnapshotReadOnly(filepath.Join(root, DatabaseFile))
	if err != nil {
		t.Fatal(err)
	}
	objects, err := image.SnapshotExternalObjects(ctx, "", 1000)
	image.Close()
	if err != nil || len(objects) != 2 {
		t.Fatalf("golden objects: %d %v", len(objects), err)
	}

	packPath := filepath.Join(root, "packs", "pack-000000.tar")
	pack, err := os.OpenFile(packPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	tarWriter := tar.NewWriter(pack)
	var payload int64
	for _, object := range objects {
		input, err := os.Open(filepath.Join(assets, filepath.FromSlash(object.StoragePath)))
		if err != nil {
			t.Fatal(err)
		}
		header := &tar.Header{Name: object.StoragePath, Mode: 0o600, Size: object.SizeBytes, Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(tarWriter, input); err != nil {
			t.Fatal(err)
		}
		input.Close()
		payload += object.SizeBytes
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pack.Close(); err != nil {
		t.Fatal(err)
	}
	databaseSHA, databaseSize, err := hashRegularFile(filepath.Join(root, DatabaseFile), DefaultLimits().MaxTotalBytes)
	if err != nil {
		t.Fatal(err)
	}
	packSHA, packSize, err := hashRegularFile(packPath, DefaultLimits().MaxTotalBytes)
	if err != nil {
		t.Fatal(err)
	}
	content := sha256.New()
	fmt.Fprintf(content, "database\x00%s\x00%d\n", databaseSHA, databaseSize)
	for _, object := range objects {
		fmt.Fprintf(content, "object\x00%s\x00%s\x00%d\n", object.StoragePath, object.SHA256, object.SizeBytes)
	}
	cleared := append([]string(nil), store.SnapshotLocalTables...)
	sort.Strings(cleared)
	manifest := Manifest{
		Format: FormatName, Version: FormatVersion, ContentSHA256: hex.EncodeToString(content.Sum(nil)),
		Snapshot:      SnapshotMetadata{ID: "snapshot_golden", CreatedAt: "2026-08-23T00:00:00Z", Consistency: "sqlite-online-backup", DatabaseID: state.DatabaseID, SourceReplicaID: state.SourceReplicaID, Vector: state.Vector, Floors: state.Floors},
		Compatibility: Compatibility{ApplicationID: ApplicationID, ApplicationVersion: version.Version, SourceSchemaVersion: store.CurrentSchemaVersion, MinimumSchemaVersion: store.CurrentSchemaVersion, MaximumSchemaVersion: store.CurrentSchemaVersion, RequiredCapabilities: []string{CapabilityArchiveV2, CapabilitySQLiteImage}, SemanticFallbackFormat: SemanticFallback},
		Database:      FileDescriptor{Path: DatabaseFile, SHA256: databaseSHA, SizeBytes: databaseSize},
		External:      ExternalManifest{Layout: "deterministic-ustar", PackTarget: DefaultPackTargetBytes, PackEntries: DefaultPackMaxEntries, Objects: int64(len(objects)), PayloadBytes: payload, Packs: []PackDescriptor{{Path: "packs/pack-000000.tar", SHA256: packSHA, SizeBytes: packSize, PayloadBytes: payload, Entries: len(objects), FirstPath: objects[0].StoragePath, LastPath: objects[len(objects)-1].StoragePath}}},
		ClearedTables: cleared,
	}
	manifest.CommitSHA256, err = ComputeCommitSHA256(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestFile, err := os.OpenFile(filepath.Join(root, ManifestFile), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(manifestFile, "%s\n", mustJSON(t, manifest)); err != nil {
		t.Fatal(err)
	}
	manifestFile.Close()

	report, err := VerifyDirectory(ctx, root, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReadyForInstall || report.Objects != 2 || report.SnapshotID != "snapshot_golden" {
		t.Fatalf("unexpected golden report: %+v", report)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
