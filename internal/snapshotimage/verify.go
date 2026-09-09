package snapshotimage

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// VerifyDirectory performs the complete non-mutating G14c admission pass. It
// does not extract packs or replace a target library; G14d owns those actions.
func VerifyDirectory(ctx context.Context, root string, limits Limits) (VerificationReport, error) {
	ctx = contextOrBackground(ctx)
	if err := validateLimits(limits); err != nil {
		return VerificationReport{}, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return VerificationReport{}, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return VerificationReport{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return VerificationReport{}, fmt.Errorf("snapshot root must be a real directory")
	}
	manifest, err := readManifest(root, limits)
	if err != nil {
		return VerificationReport{}, err
	}
	if err := validateSnapshotTree(root, manifest); err != nil {
		return VerificationReport{}, err
	}

	databasePath := filepath.Join(root, DatabaseFile)
	databaseSHA, databaseBytes, err := hashRegularFile(databasePath, limits.MaxTotalBytes)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("snapshot database: %w", err)
	}
	if databaseSHA != manifest.Database.SHA256 || databaseBytes != manifest.Database.SizeBytes {
		return VerificationReport{}, fmt.Errorf("snapshot database hash or length mismatch")
	}
	state, err := store.InspectSQLiteSnapshot(ctx, databasePath)
	if err != nil {
		return VerificationReport{}, err
	}
	if state.SchemaVersion != manifest.Compatibility.SourceSchemaVersion || state.DatabaseID != manifest.Snapshot.DatabaseID || state.SourceReplicaID != manifest.Snapshot.SourceReplicaID || !reflect.DeepEqual(state.Vector, manifest.Snapshot.Vector) || !reflect.DeepEqual(state.Floors, manifest.Snapshot.Floors) {
		return VerificationReport{}, fmt.Errorf("snapshot database state does not match the manifest")
	}

	image, err := store.OpenSQLiteSnapshotReadOnly(databasePath)
	if err != nil {
		return VerificationReport{}, err
	}
	defer image.Close()
	cursor := newObjectCursor(ctx, image)
	contentHash := sha256.New()
	writeContentDatabase(contentHash, databaseSHA, databaseBytes)
	var objectCount int64
	var payloadBytes int64
	for _, descriptor := range manifest.External.Packs {
		objects := make([]store.SnapshotExternalObject, 0, descriptor.Entries)
		for len(objects) < descriptor.Entries {
			object, found := cursor.next()
			if !found {
				if cursor.err != nil {
					return VerificationReport{}, cursor.err
				}
				return VerificationReport{}, fmt.Errorf("snapshot pack inventory has more entries than the database")
			}
			if err := validateExternalObject(object, limits); err != nil {
				return VerificationReport{}, err
			}
			objects = append(objects, object)
			writeContentObject(contentHash, object)
		}
		if objects[0].StoragePath != descriptor.FirstPath || objects[len(objects)-1].StoragePath != descriptor.LastPath {
			return VerificationReport{}, fmt.Errorf("snapshot pack %s path bounds do not match the database", descriptor.Path)
		}
		if err := verifyPackAgainstObjects(ctx, filepath.Join(root, filepath.FromSlash(descriptor.Path)), descriptor, objects, limits); err != nil {
			return VerificationReport{}, err
		}
		objectCount += int64(len(objects))
		payloadBytes += descriptor.PayloadBytes
	}
	if _, found := cursor.next(); found {
		return VerificationReport{}, fmt.Errorf("snapshot database names external objects absent from all packs")
	}
	if cursor.err != nil {
		return VerificationReport{}, cursor.err
	}
	if objectCount != manifest.External.Objects || payloadBytes != manifest.External.PayloadBytes {
		return VerificationReport{}, fmt.Errorf("verified external totals do not match the manifest")
	}
	if content := hex.EncodeToString(contentHash.Sum(nil)); content != manifest.ContentSHA256 {
		return VerificationReport{}, fmt.Errorf("snapshot content aggregate mismatch")
	}
	return VerificationReport{
		SnapshotID: manifest.Snapshot.ID, DatabaseID: state.DatabaseID,
		SourceReplicaID: state.SourceReplicaID, SchemaVersion: state.SchemaVersion,
		CommitSHA256: manifest.CommitSHA256, ContentSHA256: manifest.ContentSHA256,
		Packs: len(manifest.External.Packs), Objects: objectCount,
		DatabaseBytes: databaseBytes, ExternalBytes: payloadBytes, ReadyForInstall: true,
		InstallBoundary: "verified staging; installation requires an explicit intent and verified emergency snapshot",
		SnapshotVector:  manifest.Snapshot.Vector, SnapshotFloors: manifest.Snapshot.Floors,
	}, nil
}

// ReadManifest returns the strictly decoded and validated manifest without
// weakening VerifyDirectory's full admission boundary. Installation calls it
// only after complete verification to obtain the declared pack inventory.
func ReadManifest(root string, limits Limits) (Manifest, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, err
	}
	return readManifest(absolute, limits)
}

func readManifest(root string, limits Limits) (Manifest, error) {
	path := filepath.Join(root, ManifestFile)
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, fmt.Errorf("incomplete snapshot: manifest.json is absent")
		}
		return Manifest{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limits.MaxManifestBytes {
		return Manifest{}, fmt.Errorf("snapshot manifest is not a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	limited := &io.LimitedReader{R: file, N: limits.MaxManifestBytes + 1}
	raw, err := io.ReadAll(limited)
	if err != nil {
		return Manifest{}, err
	}
	if int64(len(raw)) > limits.MaxManifestBytes {
		return Manifest{}, fmt.Errorf("snapshot manifest exceeds reader limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("snapshot manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("snapshot manifest contains trailing JSON")
	}
	if err := validateManifest(manifest, limits); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateSnapshotTree(root string, manifest Manifest) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) != 3 {
		return fmt.Errorf("snapshot root must contain only manifest.json, notes.sqlite, and packs")
	}
	wantRoot := map[string]bool{ManifestFile: true, DatabaseFile: true, "packs": true}
	for _, entry := range entries {
		if !wantRoot[entry.Name()] {
			return fmt.Errorf("unexpected snapshot root entry %q", entry.Name())
		}
		info, err := os.Lstat(filepath.Join(root, entry.Name()))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot entry %q is not safe", entry.Name())
		}
		if entry.Name() == "packs" && !info.IsDir() {
			return fmt.Errorf("snapshot packs entry is not a directory")
		}
		if entry.Name() != "packs" && !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot entry %q is not a regular file", entry.Name())
		}
	}
	packEntries, err := os.ReadDir(filepath.Join(root, "packs"))
	if err != nil {
		return err
	}
	if len(packEntries) != len(manifest.External.Packs) {
		return fmt.Errorf("snapshot pack file count does not match manifest")
	}
	for index, entry := range packEntries {
		if entry.Name() != filepath.Base(manifest.External.Packs[index].Path) {
			return fmt.Errorf("unexpected snapshot pack entry %q", entry.Name())
		}
		info, err := os.Lstat(filepath.Join(root, "packs", entry.Name()))
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot pack %q is not a real regular file", entry.Name())
		}
	}
	return nil
}

func verifyPackAgainstObjects(ctx context.Context, path string, descriptor PackDescriptor, objects []store.SnapshotExternalObject, limits Limits) error {
	digest, size, err := hashRegularFile(path, limits.MaxTotalBytes)
	if err != nil {
		return fmt.Errorf("snapshot pack %s: %w", descriptor.Path, err)
	}
	if descriptor.SHA256 != "" && (digest != descriptor.SHA256 || size != descriptor.SizeBytes) {
		return fmt.Errorf("snapshot pack %s hash or length mismatch", descriptor.Path)
	}
	wantTarSize := int64(1024)
	var payload int64
	for _, object := range objects {
		wantTarSize += 512 + ((object.SizeBytes + 511) / 512 * 512)
		payload += object.SizeBytes
	}
	if size != wantTarSize || payload != descriptor.PayloadBytes || len(objects) != descriptor.Entries {
		return fmt.Errorf("snapshot pack %s has non-canonical size or totals", descriptor.Path)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := tar.NewReader(bufio.NewReaderSize(file, 64<<10))
	buffer := make([]byte, 64<<10)
	for index, object := range objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if err != nil {
			return fmt.Errorf("snapshot pack %s entry %d: %w", descriptor.Path, index, err)
		}
		if !validStoragePath(header.Name, limits) {
			return fmt.Errorf("snapshot pack %s contains unsafe path %q", descriptor.Path, header.Name)
		}
		if header.Name != object.StoragePath || header.Size != object.SizeBytes || header.Typeflag != tar.TypeReg || header.Mode != 0o600 || header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" || !header.ModTime.Equal(time.Unix(0, 0).UTC()) || header.Format != tar.FormatUSTAR || len(header.PAXRecords) != 0 || header.Linkname != "" {
			return fmt.Errorf("snapshot pack %s entry %q is not canonical", descriptor.Path, header.Name)
		}
		objectHash := sha256.New()
		written, err := io.CopyBuffer(objectHash, reader, buffer)
		if err != nil || written != object.SizeBytes || hex.EncodeToString(objectHash.Sum(nil)) != object.SHA256 {
			if err != nil {
				return fmt.Errorf("snapshot pack %s entry %q: %w", descriptor.Path, header.Name, err)
			}
			return fmt.Errorf("snapshot pack %s entry %q content mismatch", descriptor.Path, header.Name)
		}
	}
	if header, err := reader.Next(); !errors.Is(err, io.EOF) || header != nil {
		if err != nil {
			return fmt.Errorf("snapshot pack %s trailer: %w", descriptor.Path, err)
		}
		return fmt.Errorf("snapshot pack %s contains undeclared entries", descriptor.Path)
	}
	return nil
}

func validateExternalObject(object store.SnapshotExternalObject, limits Limits) error {
	if !validStoragePath(object.StoragePath, limits) || !shaPattern.MatchString(object.SHA256) || object.SizeBytes < 0 || object.SizeBytes > limits.MaxObjectBytes {
		return fmt.Errorf("snapshot database contains an invalid external object declaration")
	}
	parts := strings.Split(object.StoragePath, "/")
	wantKind := "blob"
	hashIndex := 3
	if parts[0] == "source-bundles" {
		wantKind = "source_bundle"
		hashIndex = 4
	}
	if object.Kind != wantKind || parts[hashIndex] != object.SHA256 || parts[hashIndex-2] != object.SHA256[:2] || parts[hashIndex-1] != object.SHA256[2:4] {
		return fmt.Errorf("snapshot external object path does not match its type and hash")
	}
	return nil
}

func validStoragePath(value string, limits Limits) bool {
	if value == "" || len(value) > limits.MaxPathBytes || strings.Contains(value, "\\") || strings.ContainsRune(value, 0) || filepath.IsAbs(value) {
		return false
	}
	parts := strings.Split(value, "/")
	if len(parts) == 4 && parts[0] == "sha256" {
		return len(parts[1]) == 2 && len(parts[2]) == 2 && shaPattern.MatchString(parts[3])
	}
	return len(parts) == 5 && parts[0] == "source-bundles" && parts[1] == "sha256" && len(parts[2]) == 2 && len(parts[3]) == 2 && shaPattern.MatchString(parts[4])
}

func validateTimestamp(value string) error {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Location() != time.UTC || parsed.Format(time.RFC3339Nano) != value {
		return fmt.Errorf("snapshot created_at must be canonical UTC RFC3339Nano")
	}
	return nil
}

func validateLimits(limits Limits) error {
	defaults := DefaultLimits()
	if limits.MaxManifestBytes <= 0 || limits.MaxManifestBytes > defaults.MaxManifestBytes || limits.MaxPacks <= 0 || limits.MaxPacks > defaults.MaxPacks || limits.MaxObjects <= 0 || limits.MaxObjects > defaults.MaxObjects || limits.MaxObjectBytes <= 0 || limits.MaxObjectBytes > defaults.MaxObjectBytes || limits.MaxTotalBytes <= 0 || limits.MaxTotalBytes > defaults.MaxTotalBytes || limits.MaxPathBytes <= 0 || limits.MaxPathBytes > defaults.MaxPathBytes || limits.MaxVectorEntries <= 0 || limits.MaxVectorEntries > defaults.MaxVectorEntries {
		return fmt.Errorf("invalid snapshot reader limits")
	}
	return nil
}
