package snapshotimage

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/version"
)

type CreateOptions struct {
	PackTargetBytes int64
	PackMaxEntries  int
	// AfterPublish is a fault-injection seam used by publication-order tests.
	// Production callers leave it nil.
	AfterPublish func(stage string) error
}

type CreateReport struct {
	SnapshotID    string `json:"snapshot_id"`
	DatabaseID    string `json:"database_id"`
	CommitSHA256  string `json:"commit_sha256"`
	ContentSHA256 string `json:"content_sha256"`
	Packs         int    `json:"packs"`
	Objects       int64  `json:"objects"`
	DatabaseBytes int64  `json:"database_bytes"`
	ExternalBytes int64  `json:"external_bytes"`
	ResumedPacks  int    `json:"resumed_packs"`
	Verified      bool   `json:"verified"`
}

// Create publishes a manifest-last physical snapshot. The target may contain
// verified pack files from an interrupted run, but a manifest marks a complete
// immutable snapshot and is never overwritten by this operation.
func Create(ctx context.Context, source *store.SQLiteStore, assetRoot, target string, options CreateOptions) (CreateReport, error) {
	ctx = contextOrBackground(ctx)
	if source == nil {
		return CreateReport{}, fmt.Errorf("source store is required")
	}
	packTarget := options.PackTargetBytes
	if packTarget == 0 {
		packTarget = DefaultPackTargetBytes
	}
	packEntries := options.PackMaxEntries
	if packEntries == 0 {
		packEntries = DefaultPackMaxEntries
	}
	if packTarget <= 0 || packTarget > DefaultPackTargetBytes || packEntries <= 0 || packEntries > DefaultPackMaxEntries {
		return CreateReport{}, fmt.Errorf("invalid snapshot pack bounds")
	}
	root, assets, err := prepareCreateRoot(target, assetRoot)
	if err != nil {
		return CreateReport{}, err
	}
	status, err := source.Status(ctx)
	if err != nil {
		return CreateReport{}, err
	}
	if samePath(filepath.Join(root, DatabaseFile), status.Path) || pathContains(root, status.Path) {
		return CreateReport{}, fmt.Errorf("snapshot output must not contain or overwrite the source database")
	}

	imagePartial, err := os.CreateTemp(root, ".notes.sqlite.partial-*")
	if err != nil {
		return CreateReport{}, err
	}
	imagePartialPath := imagePartial.Name()
	if err := imagePartial.Close(); err != nil {
		return CreateReport{}, err
	}
	if err := os.Remove(imagePartialPath); err != nil {
		return CreateReport{}, err
	}
	defer os.Remove(imagePartialPath)
	state, err := source.CreateSQLiteSnapshotImage(ctx, imagePartialPath)
	if err != nil {
		return CreateReport{}, err
	}
	imageSHA, imageBytes, err := hashRegularFile(imagePartialPath, DefaultLimits().MaxTotalBytes)
	if err != nil {
		return CreateReport{}, err
	}
	if err := syncFile(imagePartialPath); err != nil {
		return CreateReport{}, err
	}

	imageReader, err := store.OpenSQLiteSnapshotReadOnly(imagePartialPath)
	if err != nil {
		return CreateReport{}, err
	}
	cursor := newObjectCursor(ctx, imageReader)
	contentHash := sha256.New()
	writeContentDatabase(contentHash, imageSHA, imageBytes)
	manifest := Manifest{
		Format:  FormatName,
		Version: FormatVersion,
		Snapshot: SnapshotMetadata{
			Consistency: "sqlite-online-backup", DatabaseID: state.DatabaseID,
			SourceReplicaID: state.SourceReplicaID, Vector: state.Vector, Floors: state.Floors,
		},
		Compatibility: Compatibility{
			ApplicationID: ApplicationID, ApplicationVersion: version.Version,
			SourceSchemaVersion: state.SchemaVersion, MinimumSchemaVersion: state.SchemaVersion,
			MaximumSchemaVersion:   state.SchemaVersion,
			RequiredCapabilities:   []string{CapabilityArchiveV2, CapabilitySQLiteImage},
			SemanticFallbackFormat: SemanticFallback,
		},
		Database:      FileDescriptor{Path: DatabaseFile, SHA256: imageSHA, SizeBytes: imageBytes},
		External:      ExternalManifest{Layout: "deterministic-ustar", PackTarget: packTarget, PackEntries: packEntries},
		ClearedTables: append([]string(nil), store.SnapshotLocalTables...),
	}
	sort.Strings(manifest.ClearedTables)

	resumed := 0
	for packNumber := 0; ; packNumber++ {
		objects, err := nextPackObjects(cursor, packTarget, packEntries)
		if err != nil {
			_ = imageReader.Close()
			return CreateReport{}, err
		}
		if len(objects) == 0 {
			break
		}
		for _, object := range objects {
			writeContentObject(contentHash, object)
		}
		pack, reused, err := publishPack(ctx, root, assets, packNumber, objects, packTarget)
		if err != nil {
			_ = imageReader.Close()
			return CreateReport{}, err
		}
		if reused {
			resumed++
		}
		manifest.External.Packs = append(manifest.External.Packs, pack)
		manifest.External.Objects += int64(pack.Entries)
		manifest.External.PayloadBytes += pack.PayloadBytes
		if options.AfterPublish != nil {
			if err := options.AfterPublish(fmt.Sprintf("pack:%d", packNumber)); err != nil {
				_ = imageReader.Close()
				return CreateReport{}, err
			}
		}
	}
	if err := cursor.err; err != nil {
		_ = imageReader.Close()
		return CreateReport{}, err
	}
	if err := imageReader.Close(); err != nil {
		return CreateReport{}, err
	}
	if err := refuseUnexpectedPack(root, len(manifest.External.Packs)); err != nil {
		return CreateReport{}, err
	}

	snapshotID, err := store.NewID("snapshot")
	if err != nil {
		return CreateReport{}, err
	}
	manifest.Snapshot.ID = snapshotID
	manifest.Snapshot.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	manifest.ContentSHA256 = hex.EncodeToString(contentHash.Sum(nil))
	if err := finalizeManifest(&manifest); err != nil {
		return CreateReport{}, err
	}
	if err := validateManifest(manifest, DefaultLimits()); err != nil {
		return CreateReport{}, err
	}

	imageFinal := filepath.Join(root, DatabaseFile)
	if err := os.Rename(imagePartialPath, imageFinal); err != nil {
		return CreateReport{}, err
	}
	if err := syncDirectory(root); err != nil {
		return CreateReport{}, err
	}
	if options.AfterPublish != nil {
		if err := options.AfterPublish("database"); err != nil {
			return CreateReport{}, err
		}
		if err := options.AfterPublish("before_manifest"); err != nil {
			return CreateReport{}, err
		}
	}
	if err := publishManifest(root, manifest); err != nil {
		return CreateReport{}, err
	}
	if options.AfterPublish != nil {
		if err := options.AfterPublish("manifest"); err != nil {
			return CreateReport{}, err
		}
	}

	verified, err := VerifyDirectory(ctx, root, DefaultLimits())
	if err != nil {
		return CreateReport{}, fmt.Errorf("verify published snapshot: %w", err)
	}
	return CreateReport{
		SnapshotID: snapshotID, DatabaseID: state.DatabaseID, CommitSHA256: manifest.CommitSHA256,
		ContentSHA256: manifest.ContentSHA256, Packs: len(manifest.External.Packs),
		Objects: manifest.External.Objects, DatabaseBytes: imageBytes,
		ExternalBytes: manifest.External.PayloadBytes, ResumedPacks: resumed, Verified: verified.ReadyForInstall,
	}, nil
}

func prepareCreateRoot(target, assetRoot string) (string, string, error) {
	root, err := filepath.Abs(strings.TrimSpace(target))
	if err != nil || strings.TrimSpace(target) == "" {
		return "", "", fmt.Errorf("snapshot output directory is required")
	}
	assets, err := filepath.Abs(strings.TrimSpace(assetRoot))
	if err != nil || strings.TrimSpace(assetRoot) == "" {
		return "", "", fmt.Errorf("asset store directory is required")
	}
	if pathContains(assets, root) || pathContains(root, assets) || samePath(root, assets) {
		return "", "", fmt.Errorf("snapshot output and asset store must not overlap")
	}
	if info, err := os.Lstat(root); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("snapshot output must be a real directory")
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return "", "", err
		}
	} else {
		return "", "", err
	}
	if _, err := os.Lstat(filepath.Join(root, ManifestFile)); err == nil {
		return "", "", fmt.Errorf("complete snapshot already exists at %s", root)
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Join(root, "packs"), 0o700); err != nil {
		return "", "", err
	}
	// Private partials are never resumable boundaries. Complete pack filenames
	// are retained and re-verified below; image creation always restarts.
	for _, directory := range []string{root, filepath.Join(root, "packs")} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return "", "", err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".notes.sqlite.partial-") && !strings.HasPrefix(entry.Name(), ".manifest.partial-") && !strings.HasPrefix(entry.Name(), ".pack.partial-") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return "", "", fmt.Errorf("unsafe stale snapshot partial %q", path)
			}
			if err := os.Remove(path); err != nil {
				return "", "", err
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", "", err
	}
	for _, entry := range entries {
		if entry.Name() != DatabaseFile && entry.Name() != "packs" {
			return "", "", fmt.Errorf("unexpected entry %q in incomplete snapshot output", entry.Name())
		}
	}
	return root, assets, nil
}

type objectCursor struct {
	ctx     context.Context
	image   *store.SQLiteStore
	page    []store.SnapshotExternalObject
	index   int
	after   string
	pending *store.SnapshotExternalObject
	err     error
}

func newObjectCursor(ctx context.Context, image *store.SQLiteStore) *objectCursor {
	return &objectCursor{ctx: ctx, image: image}
}

func (cursor *objectCursor) next() (store.SnapshotExternalObject, bool) {
	if cursor.err != nil {
		return store.SnapshotExternalObject{}, false
	}
	if cursor.pending != nil {
		object := *cursor.pending
		cursor.pending = nil
		return object, true
	}
	if cursor.index >= len(cursor.page) {
		cursor.page, cursor.err = cursor.image.SnapshotExternalObjects(cursor.ctx, cursor.after, 1000)
		cursor.index = 0
		if cursor.err != nil || len(cursor.page) == 0 {
			return store.SnapshotExternalObject{}, false
		}
	}
	object := cursor.page[cursor.index]
	cursor.index++
	if object.StoragePath <= cursor.after {
		cursor.err = fmt.Errorf("snapshot object inventory is not strictly ordered")
		return store.SnapshotExternalObject{}, false
	}
	cursor.after = object.StoragePath
	return object, true
}

func (cursor *objectCursor) putBack(object store.SnapshotExternalObject) {
	cursor.pending = &object
}

func nextPackObjects(cursor *objectCursor, target int64, maxEntries int) ([]store.SnapshotExternalObject, error) {
	objects := make([]store.SnapshotExternalObject, 0, min(maxEntries, 1024))
	var payload int64
	for len(objects) < maxEntries {
		object, found := cursor.next()
		if !found {
			break
		}
		if err := validateExternalObject(object, DefaultLimits()); err != nil {
			return nil, err
		}
		if len(objects) > 0 && payload+object.SizeBytes > target {
			cursor.putBack(object)
			break
		}
		objects = append(objects, object)
		payload += object.SizeBytes
		if object.SizeBytes > target {
			break
		}
	}
	return objects, cursor.err
}

func publishPack(ctx context.Context, root, assetRoot string, number int, objects []store.SnapshotExternalObject, target int64) (PackDescriptor, bool, error) {
	relative := fmt.Sprintf("packs/pack-%06d.tar", number)
	final := filepath.Join(root, filepath.FromSlash(relative))
	descriptor := packDescriptorFor(relative, objects, target)
	if info, err := os.Lstat(final); err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		if err := verifyPackAgainstObjects(ctx, final, descriptor, objects, DefaultLimits()); err == nil {
			digest, size, hashErr := hashRegularFile(final, DefaultLimits().MaxTotalBytes)
			if hashErr == nil {
				descriptor.SHA256, descriptor.SizeBytes = digest, size
				return descriptor, true, nil
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return PackDescriptor{}, false, err
	}

	temporary, err := os.CreateTemp(filepath.Dir(final), ".pack.partial-*")
	if err != nil {
		return PackDescriptor{}, false, err
	}
	name := temporary.Name()
	defer os.Remove(name)
	packHash := sha256.New()
	counting := &countWriter{writer: io.MultiWriter(temporary, packHash)}
	tarWriter := tar.NewWriter(counting)
	buffer := make([]byte, 64<<10)
	for _, object := range objects {
		if err := ctx.Err(); err != nil {
			_ = tarWriter.Close()
			_ = temporary.Close()
			return PackDescriptor{}, false, err
		}
		source, err := openExternalObject(assetRoot, object)
		if err != nil {
			_ = tarWriter.Close()
			_ = temporary.Close()
			return PackDescriptor{}, false, err
		}
		header := &tar.Header{
			Name: object.StoragePath, Mode: 0o600, Size: object.SizeBytes,
			ModTime: time.Unix(0, 0).UTC(), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = source.Close()
			_ = tarWriter.Close()
			_ = temporary.Close()
			return PackDescriptor{}, false, err
		}
		objectHash := sha256.New()
		written, copyErr := io.CopyBuffer(tarWriter, io.TeeReader(source, objectHash), buffer)
		closeErr := source.Close()
		if copyErr != nil || closeErr != nil || written != object.SizeBytes || hex.EncodeToString(objectHash.Sum(nil)) != object.SHA256 {
			_ = tarWriter.Close()
			_ = temporary.Close()
			if copyErr != nil {
				return PackDescriptor{}, false, copyErr
			}
			if closeErr != nil {
				return PackDescriptor{}, false, closeErr
			}
			return PackDescriptor{}, false, fmt.Errorf("external object %s changed while snapshotting", object.StoragePath)
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = temporary.Close()
		return PackDescriptor{}, false, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return PackDescriptor{}, false, err
	}
	if err := temporary.Close(); err != nil {
		return PackDescriptor{}, false, err
	}
	descriptor.SHA256 = hex.EncodeToString(packHash.Sum(nil))
	descriptor.SizeBytes = counting.count
	if err := verifyPackAgainstObjects(ctx, name, descriptor, objects, DefaultLimits()); err != nil {
		return PackDescriptor{}, false, err
	}
	if err := os.Rename(name, final); err != nil {
		return PackDescriptor{}, false, err
	}
	if err := syncDirectory(filepath.Dir(final)); err != nil {
		return PackDescriptor{}, false, err
	}
	return descriptor, false, nil
}

func packDescriptorFor(relative string, objects []store.SnapshotExternalObject, target int64) PackDescriptor {
	var payload int64
	for _, object := range objects {
		payload += object.SizeBytes
	}
	return PackDescriptor{
		Path: relative, PayloadBytes: payload, Entries: len(objects),
		FirstPath: objects[0].StoragePath, LastPath: objects[len(objects)-1].StoragePath,
		Oversized: len(objects) == 1 && payload > target,
	}
}

func openExternalObject(assetRoot string, object store.SnapshotExternalObject) (*os.File, error) {
	if err := validateExternalObject(object, DefaultLimits()); err != nil {
		return nil, err
	}
	path := filepath.Join(assetRoot, filepath.FromSlash(object.StoragePath))
	if !pathContains(assetRoot, path) {
		return nil, fmt.Errorf("external object escapes asset root")
	}
	rootInfo, err := os.Lstat(assetRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("asset root is not a real directory")
	}
	current := assetRoot
	parts := strings.Split(object.StoragePath, "/")
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("external object %s crosses an unsafe directory", object.StoragePath)
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("external object %s: %w", object.StoragePath, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != object.SizeBytes {
		return nil, fmt.Errorf("external object %s is not the declared regular file", object.StoragePath)
	}
	return os.Open(path)
}

func publishManifest(root string, manifest Manifest) error {
	temporary, err := os.CreateTemp(root, ".manifest.partial-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, filepath.Join(root, ManifestFile)); err != nil {
		return err
	}
	return syncDirectory(root)
}

func refuseUnexpectedPack(root string, count int) error {
	entries, err := os.ReadDir(filepath.Join(root, "packs"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".pack.partial-") {
			continue
		}
		if name >= fmt.Sprintf("pack-%06d.tar", count) || !regexpPackName(name) {
			return fmt.Errorf("unexpected file %q in snapshot packs directory; resume in a clean output directory", name)
		}
	}
	return nil
}

func regexpPackName(name string) bool {
	return packPathRegex.MatchString("packs/" + name)
}

func hashRegularFile(path string, max int64) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > max {
		return "", 0, fmt.Errorf("%s is not a bounded regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	written, err := io.CopyBuffer(digest, file, make([]byte, 64<<10))
	if err != nil {
		return "", 0, err
	}
	if written != info.Size() {
		return "", 0, fmt.Errorf("%s changed while hashing", path)
	}
	return hex.EncodeToString(digest.Sum(nil)), written, nil
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	err = file.Sync()
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func writeContentDatabase(digest hash.Hash, sha string, size int64) {
	fmt.Fprintf(digest, "database\x00%s\x00%d\n", sha, size)
}

func writeContentObject(digest hash.Hash, object store.SnapshotExternalObject) {
	fmt.Fprintf(digest, "object\x00%s\x00%s\x00%d\n", object.StoragePath, object.SHA256, object.SizeBytes)
}

type countWriter struct {
	writer io.Writer
	count  int64
}

func (writer *countWriter) Write(p []byte) (int, error) {
	n, err := writer.writer.Write(p)
	writer.count += int64(n)
	return n, err
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func samePath(a, b string) bool {
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	return aErr == nil && bErr == nil && filepath.Clean(aAbs) == filepath.Clean(bAbs)
}

func pathContains(root, child string) bool {
	relative, err := filepath.Rel(root, child)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
