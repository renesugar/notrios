package synccarrier

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/renesugar/notrios/internal/syncwire"
)

// Layout constants. The version segment is in the path rather than only in the
// artifacts so that a future incompatible layout can live beside this one on
// the same drive instead of being mistaken for it.
const (
	rootFolder    = "notrios-sync"
	layoutVersion = "v1"
	replicasDir   = "replicas"
	stagingDir    = "staging"
	artifactExt   = ".nar"
	tempExt       = ".tmp"
)

// Bounds for one carrier. They exist to stop a hostile or broken carrier from
// deciding how much memory and time a replica spends: every one of them is
// checked before the allocation or the walk it guards.
const (
	// MaxArtifactBytes is G9's envelope ceiling plus room for the artifact
	// framing, header, and signature around it.
	MaxArtifactBytes = int64(17 << 20)
	// MaxEntriesPerClass bounds one directory scan.
	MaxEntriesPerClass = 4_096
	// MaxNamespaces matches G5's state-vector ceiling: a carrier cannot present
	// more replicas than a vector could describe.
	MaxNamespaces = 1_024
)

// Directory is the shared-folder carrier.
//
// Every path it produces below the database root is a keyed blind, so a
// directory listing tells an observer how many replicas and artifacts exist —
// which G0's budget allows — and nothing about which library, which device, or
// which content they belong to.
type Directory struct {
	root      string
	database  string
	namespace string
	group     syncwire.GroupKey

	// nonAtomic records that this filesystem refused an atomic rename, so the
	// evidence can say which mode a run exercised. Correctness does not depend
	// on it: a reader verifies bytes, so a torn file is skipped either way.
	nonAtomic bool
	// rename is os.Rename except in the test that has to prove the fallback
	// works. There is no way to ask a real filesystem to stop supporting
	// rename, and "we believe the fallback compiles" is not evidence.
	rename func(oldPath, newPath string) error

	// Snapshot publication may be deliberately split into many calls. A fresh
	// process verifies the durable prefix once; subsequent calls on this bound
	// carrier remember the exact length they just synced and avoid turning N
	// chunks into N whole-prefix reads. The completed file is still hashed in
	// full before publication, so carrier mutation can only make the transfer
	// restart, never admit different bytes.
	snapshotMu           sync.Mutex
	snapshotPrefixes     map[string]snapshotPrefix
	verifySnapshotPrefix func(*os.Root, *os.File, string, int64) (int64, error)
}

// NewDirectory binds a carrier root to one database and one local replica.
//
// The group key is required because the layout is blinded: without it a peer
// cannot even name the directory it should write to, which is the intended
// property. A carrier holder who is not an enrolled peer sees opaque folders.
func NewDirectory(root string, group syncwire.GroupKey, databaseID, replicaID string) (*Directory, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("%w: no carrier directory configured", ErrCarrierUnavailable)
	}
	if databaseID == "" || replicaID == "" {
		return nil, errors.New("synccarrier: a carrier names its database and its replica")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Directory{
		root:                 absolute,
		database:             syncwire.CarrierName(group, "database", databaseID),
		namespace:            syncwire.CarrierName(group, "replica", replicaID),
		group:                group,
		rename:               nil,
		snapshotPrefixes:     map[string]snapshotPrefix{},
		verifySnapshotPrefix: verifiedPrefixRoot,
	}, nil
}

// Namespace returns this replica's blinded namespace name.
func (d *Directory) Namespace() string { return d.namespace }

// NonAtomic reports whether a publish had to fall back to a direct write
// because the filesystem would not rename.
func (d *Directory) NonAtomic() bool { return d.nonAtomic }

// Root returns the database-scoped directory this carrier reads and writes.
func (d *Directory) Root() string {
	return filepath.Join(d.root, rootFolder, layoutVersion, d.database)
}

func (d *Directory) namespacePath(namespace string) string {
	return filepath.Join(d.Root(), replicasDir, namespace)
}

func (d *Directory) classPath(namespace string, class Class) string {
	return filepath.Join(d.namespacePath(namespace), string(class))
}

// Initialize creates the skeleton. Any enrolled replica may call it: G11's
// resolved decision is that no peer owns the carrier, because a carrier with an
// owner stops working the day that peer is retired.
func (d *Directory) Initialize(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// The layout is created inside an existing root; the root itself never is.
	// A removable drive that is not mounted, or a network share that is not
	// connected, is still an ordinary path — and creating it would quietly
	// write the carrier onto the boot disk under an empty mount point, where
	// the peer will never look and the space will never be noticed.
	if !d.Available() {
		return fmt.Errorf("%w: %s", ErrCarrierUnavailable, d.root)
	}
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer root.Close()
	for _, class := range []Class{ClassAdvertisement, ClassEnvelope, ClassRequest, ClassObject, ClassSnapshot} {
		if err := rejectSymlinkAncestors(root, d.rel(d.classPath(d.namespace, class))); err != nil {
			return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
		}
		if err := root.MkdirAll(d.rel(d.classPath(d.namespace, class)), 0o755); err != nil {
			return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
		}
	}
	stagingPath := d.rel(filepath.Join(d.namespacePath(d.namespace), stagingDir))
	if err := rejectSymlinkAncestors(root, stagingPath); err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if err := root.MkdirAll(stagingPath, 0o700); err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	return nil
}

// Available reports whether the carrier root can be reached at all. A removable
// drive that has been unplugged is an ordinary state, not an error to retry
// forever, and the caller is told which one it is.
func (d *Directory) Available() bool {
	info, err := os.Stat(d.root)
	return err == nil && info.IsDir()
}

// Publish writes one artifact into this replica's namespace.
//
// An empty name means the artifact is named by its own content: the file name
// is the SHA-256 of the bytes it holds. That is what lets a reader detect a
// torn or partially-synchronized file without trusting size, mtime, or the
// provider's claim to have written atomically.
func (d *Directory) Publish(ctx context.Context, class Class, name string, artifact []byte) (string, error) {
	return d.PublishTo(ctx, d.namespace, class, name, artifact)
}

// PublishTo writes into a named namespace.
//
// It exists for one caller: a service hosting this carrier for its peers, which
// writes on behalf of the replica it has just authenticated. That is not a hole
// in "each replica writes only its own namespace" — it is where the rule is
// enforced rather than assumed, because the host derives the namespace from the
// authenticated principal and never from anything the request said.
func (d *Directory) PublishTo(ctx context.Context, namespace string, class Class, name string, artifact []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !validBlindName(namespace) {
		return "", ErrNotOwned
	}
	if !classes[class] {
		return "", fmt.Errorf("synccarrier: unknown artifact class %q", class)
	}
	if int64(len(artifact)) > MaxArtifactBytes {
		return "", fmt.Errorf("%w: %d bytes", ErrTooLarge, len(artifact))
	}
	if name == "" {
		digest := sha256.Sum256(artifact)
		name = hex.EncodeToString(digest[:])
	}
	if !validArtifactName(name) {
		return "", fmt.Errorf("synccarrier: refusing to publish under name %q", name)
	}
	target := d.artifactPath(namespace, class, name)
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer root.Close()
	if err := rejectSymlinkAncestors(root, d.rel(filepath.Dir(target))); err != nil {
		return "", fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if err := root.MkdirAll(d.rel(filepath.Dir(target)), 0o755); err != nil {
		return "", fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	// An artifact already published under this name is the same artifact
	// logically, but it may have been damaged in transit — half-copied by a
	// provider, or renamed non-atomically — and this replica's own namespace is
	// the one place it is entitled to repair. Callers decide whether to
	// republish by reading what is there first; a write that reaches here
	// replaces it.
	if err := d.writeAtomic(root, namespace, target, artifact); err != nil {
		return "", err
	}
	return name, nil
}

// writeAtomic stages, flushes, and renames. Where a filesystem refuses the
// rename it writes in place instead and records that it did: the protocol does
// not need atomic rename to be correct, because names commit to contents, and
// pretending the carrier supports it would be the actual risk.
func (d *Directory) writeAtomic(root *os.Root, namespace, target string, artifact []byte) error {
	staging := filepath.Join(d.namespacePath(namespace), stagingDir)
	if err := rejectSymlinkAncestors(root, d.rel(staging)); err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if err := root.MkdirAll(d.rel(staging), 0o700); err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	temporary := filepath.Join(staging, hex.EncodeToString(suffix)+tempExt)
	file, err := root.OpenFile(d.rel(temporary), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if _, err := file.Write(artifact); err != nil {
		file.Close()
		_ = root.Remove(d.rel(temporary))
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	// Sync failures are not fatal: several network and FUSE filesystems do not
	// implement it. The reader's hash check is what actually protects us.
	_ = file.Sync()
	if err := file.Close(); err != nil {
		_ = root.Remove(d.rel(temporary))
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	var renameErr error
	if d.rename != nil {
		renameErr = d.rename(temporary, target)
	} else {
		renameErr = root.Rename(d.rel(temporary), d.rel(target))
	}
	if renameErr != nil {
		_ = root.Remove(d.rel(temporary))
		d.nonAtomic = true
		targetRel := d.rel(target)
		// Remove the directory entry before creating the fallback. WriteFile
		// would follow an existing symlink or truncate a hard-linked inode,
		// allowing a hostile carrier to modify a file outside this tree.
		if removeErr := root.Remove(targetRel); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return fmt.Errorf("%w: %v", ErrCarrierUnavailable, removeErr)
		}
		fallback, writeErr := root.OpenFile(targetRel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if writeErr != nil {
			return fmt.Errorf("%w: %v", ErrCarrierUnavailable, writeErr)
		}
		_, writeErr = fallback.Write(artifact)
		if writeErr == nil {
			_ = fallback.Sync()
		}
		if closeErr := fallback.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			_ = root.Remove(targetRel)
			return fmt.Errorf("%w: %v", ErrCarrierUnavailable, writeErr)
		}
		return nil
	}
	if parent, err := root.Open(d.rel(filepath.Dir(target))); err == nil {
		_ = parent.Sync()
		parent.Close()
	}
	return nil
}

// Namespaces lists the replica namespaces present. It sorts what it finds:
// nothing may depend on the order a filesystem or a cloud provider chose to
// return, and a test that reverses the listing has to make no difference.
func (d *Directory) Namespaces(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer root.Close()
	replicas := d.rel(filepath.Join(d.Root(), replicasDir))
	if err := rejectSymlinkAncestors(root, replicas); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	directory, err := openCarrierDirectory(root, replicas)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer directory.Close()
	entries, err := readCarrierEntries(directory, MaxScannedDirectoryEntries)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	names := make([]string, 0, MaxEntriesPerClass)
	for _, entry := range entries {
		if len(names) >= MaxNamespaces {
			break
		}
		if !entry.IsDir() || !validBlindName(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// List names the artifacts in one namespace and class. Anything that is not a
// well-formed artifact name is ignored rather than reported: a carrier holds
// other people's temporary files, provider sidecars, and conflict copies, and
// none of that is an error.
func (d *Directory) List(ctx context.Context, namespace string, class Class) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !classes[class] || !validBlindName(namespace) {
		return nil, nil
	}
	base := d.classPath(namespace, class)
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer root.Close()
	if err := rejectSymlinkAncestors(root, d.rel(base)); err != nil {
		return nil, nil
	}
	if class != ClassObject {
		return listArtifacts(root, d.rel(base))
	}
	// Objects fan out one level so a large library does not put a hundred
	// thousand entries in one directory, which several providers list slowly
	// and some refuse outright.
	shardDir, err := openCarrierDirectory(root, d.rel(base))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer shardDir.Close()
	shards, err := readCarrierEntries(shardDir, MaxScannedDirectoryEntries)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	var names []string
	for _, shard := range shards {
		if !shard.IsDir() || len(shard.Name()) != 2 || !lowercaseHex(shard.Name()) {
			continue
		}
		found, err := listArtifacts(root, filepath.Join(d.rel(base), shard.Name()))
		if err != nil {
			return nil, err
		}
		names = append(names, found...)
		if len(names) >= MaxEntriesPerClass {
			names = names[:MaxEntriesPerClass]
			break
		}
	}
	sort.Strings(names)
	return names, nil
}

func listArtifacts(root *os.Root, base string) ([]string, error) {
	directory, err := openCarrierDirectory(root, base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer directory.Close()
	names := make([]string, 0, MaxEntriesPerClass)
	for scanned := 0; scanned < MaxScannedDirectoryEntries; {
		batch, readErr := directory.ReadDir(minInt(MaxDirectoryReadBatch, MaxScannedDirectoryEntries-scanned))
		scanned += len(batch)
		for _, entry := range batch {
			if len(names) >= MaxEntriesPerClass {
				break
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), artifactExt) {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), artifactExt)
			if !validArtifactName(name) {
				continue
			}
			// The listing is untrusted input too. Lstat/open/fstat closes the
			// symlink and special-file cases, including a replacement between the
			// directory read and this check.
			file, err := openBoundedRegular(root, filepath.Join(base, entry.Name()), MaxArtifactBytes)
			if err != nil {
				continue
			}
			_ = file.Close()
			names = append(names, name)
		}
		if len(names) >= MaxEntriesPerClass || errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, readErr)
		}
	}
	sort.Strings(names)
	return names, nil
}

const (
	MaxScannedDirectoryEntries = 16_384
	MaxDirectoryReadBatch      = 256
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func readCarrierEntries(directory *os.File, limit int) ([]os.DirEntry, error) {
	entries := make([]os.DirEntry, 0, minInt(limit, MaxDirectoryReadBatch))
	for len(entries) < limit {
		batch, err := directory.ReadDir(minInt(MaxDirectoryReadBatch, limit-len(entries)))
		entries = append(entries, batch...)
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func openCarrierDirectory(root *os.Root, path string) (*os.File, error) {
	if err := rejectSymlinkAncestors(root, path); err != nil {
		return nil, err
	}
	lstat, err := root.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !lstat.IsDir() || lstat.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnreadable
	}
	directory, err := openRootDirectoryNoFollow(root, path)
	if err != nil {
		return nil, err
	}
	fstat, err := directory.Stat()
	if err != nil || !fstat.IsDir() || !os.SameFile(lstat, fstat) {
		directory.Close()
		return nil, ErrUnreadable
	}
	return directory, nil
}

// Read returns one artifact's bytes.
//
// A content-named artifact is verified against its own name here, before the
// caller sees it. That check is what makes every hostile-carrier case in G11's
// validation list — torn write, half-copied file, stale cached copy, truncated
// upload — arrive as the same ordinary skip.
func (d *Directory) Read(ctx context.Context, namespace string, class Class, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !classes[class] || !validBlindName(namespace) || !validArtifactName(name) {
		return nil, ErrUnreadable
	}
	path := d.artifactPath(namespace, class, name)
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, ErrUnreadable
	}
	defer root.Close()
	file, err := openBoundedRegular(root, d.rel(path), MaxArtifactBytes)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// The size check in openBoundedRegular is a fast rejection. The limited
	// read is still required: a carrier can grow after fstat, and ReadFile
	// would otherwise make the bound advisory.
	artifact, err := io.ReadAll(io.LimitReader(file, MaxArtifactBytes+1))
	if err != nil {
		return nil, ErrUnreadable
	}
	if int64(len(artifact)) > MaxArtifactBytes {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrTooLarge, MaxArtifactBytes)
	}
	if len(name) == 2*sha256.Size {
		digest := sha256.Sum256(artifact)
		if hex.EncodeToString(digest[:]) != name {
			return nil, fmt.Errorf("%w: contents do not match the name", ErrUnreadable)
		}
	}
	return artifact, nil
}

// openBoundedRegular performs the carrier trust boundary in descriptor order:
// Lstat rejects links without following them, open obtains the object, and
// fstat/SameFile proves that the path was not swapped during that transition.
// The returned descriptor is the one subsequently consumed by Read.
func openBoundedRegular(root *os.Root, path string, maxBytes int64) (*os.File, error) {
	if err := rejectSymlinkAncestors(root, path); err != nil {
		return nil, ErrUnreadable
	}
	lstat, err := root.Lstat(path)
	if err != nil {
		return nil, ErrUnreadable
	}
	if !lstat.Mode().IsRegular() || lstat.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnreadable
	}
	file, err := root.Open(path)
	if err != nil {
		return nil, ErrUnreadable
	}
	fstat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, ErrUnreadable
	}
	if !fstat.Mode().IsRegular() || !os.SameFile(lstat, fstat) {
		file.Close()
		return nil, ErrUnreadable
	}
	if fstat.Size() > maxBytes {
		file.Close()
		return nil, fmt.Errorf("%w: %d bytes", ErrTooLarge, fstat.Size())
	}
	return file, nil
}

// Remove deletes one of this replica's own artifacts, and refuses every other
// namespace. Cleanup on a shared carrier must never be able to delete another
// peer's only copy of an envelope.
func (d *Directory) Remove(ctx context.Context, class Class, name string) error {
	return d.RemoveFrom(ctx, d.namespace, class, name)
}

// RemoveFrom deletes from a named namespace, for a host acting on behalf of the
// replica it authenticated. Every other caller uses Remove and can only reach
// its own.
func (d *Directory) RemoveFrom(ctx context.Context, namespace string, class Class, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !classes[class] || !validArtifactName(name) || !validBlindName(namespace) {
		return ErrNotOwned
	}
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer root.Close()
	target := d.rel(d.artifactPath(namespace, class, name))
	if err := rejectSymlinkAncestors(root, filepath.Dir(target)); err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if err := root.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	return nil
}

// artifactPath resolves one artifact. Names are validated before they reach it,
// so no caller-supplied string can escape the carrier root.
func (d *Directory) artifactPath(namespace string, class Class, name string) string {
	base := d.classPath(namespace, class)
	if class == ClassObject {
		base = filepath.Join(base, name[:2])
	}
	return filepath.Join(base, name+artifactExt)
}

func (d *Directory) rel(path string) string {
	relative, err := filepath.Rel(d.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(relative)
}

// rejectSymlinkAncestors prevents os.Root's intentional in-root symlink
// following from making carrier layout aliases ambiguous. Missing trailing
// components are allowed because callers may be creating them.
func rejectSymlinkAncestors(root *os.Root, path string) error {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." || path == "" || strings.HasPrefix(path, "../") {
		return ErrUnreadable
	}
	parts := strings.Split(path, "/")
	for index := 1; index <= len(parts); index++ {
		prefix := strings.Join(parts[:index], "/")
		info, err := root.Lstat(prefix)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("synccarrier: symlink in carrier path")
		}
		if index < len(parts) && !info.IsDir() {
			return errors.New("synccarrier: non-directory in carrier path")
		}
	}
	return nil
}

// validArtifactName accepts the two name shapes the protocol produces: a
// 32-character keyed blind and a 64-character content hash. Both are lowercase
// hex, which is also what keeps a case-insensitive filesystem from folding two
// distinct artifacts onto one path.
func validArtifactName(name string) bool {
	if len(name) != 32 && len(name) != 2*sha256.Size {
		return false
	}
	return lowercaseHex(name)
}

func validBlindName(name string) bool { return len(name) == 32 && lowercaseHex(name) }

func lowercaseHex(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch {
		case character >= '0' && character <= '9':
		case character >= 'a' && character <= 'f':
		default:
			return false
		}
	}
	return len(value) > 0
}
