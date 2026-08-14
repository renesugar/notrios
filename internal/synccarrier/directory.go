package synccarrier

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		root:      absolute,
		database:  syncwire.CarrierName(group, "database", databaseID),
		namespace: syncwire.CarrierName(group, "replica", replicaID),
		group:     group,
		rename:    os.Rename,
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
	for _, class := range []Class{ClassAdvertisement, ClassEnvelope, ClassRequest, ClassObject, ClassSnapshot} {
		if err := os.MkdirAll(d.classPath(d.namespace, class), 0o755); err != nil {
			return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(d.namespacePath(d.namespace), stagingDir), 0o700); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	// An artifact already published under this name is the same artifact
	// logically, but it may have been damaged in transit — half-copied by a
	// provider, or renamed non-atomically — and this replica's own namespace is
	// the one place it is entitled to repair. Callers decide whether to
	// republish by reading what is there first; a write that reaches here
	// replaces it.
	if err := d.writeAtomic(namespace, target, artifact); err != nil {
		return "", err
	}
	return name, nil
}

// writeAtomic stages, flushes, and renames. Where a filesystem refuses the
// rename it writes in place instead and records that it did: the protocol does
// not need atomic rename to be correct, because names commit to contents, and
// pretending the carrier supports it would be the actual risk.
func (d *Directory) writeAtomic(namespace, target string, artifact []byte) error {
	staging := filepath.Join(d.namespacePath(namespace), stagingDir)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	temporary := filepath.Join(staging, hex.EncodeToString(suffix)+tempExt)
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if _, err := file.Write(artifact); err != nil {
		file.Close()
		os.Remove(temporary)
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	// Sync failures are not fatal: several network and FUSE filesystems do not
	// implement it. The reader's hash check is what actually protects us.
	_ = file.Sync()
	if err := file.Close(); err != nil {
		os.Remove(temporary)
		return fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if err := d.rename(temporary, target); err != nil {
		os.Remove(temporary)
		d.nonAtomic = true
		if writeErr := os.WriteFile(target, artifact, 0o644); writeErr != nil {
			return fmt.Errorf("%w: %v", ErrCarrierUnavailable, writeErr)
		}
		return nil
	}
	if parent, err := os.Open(filepath.Dir(target)); err == nil {
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
	entries, err := os.ReadDir(filepath.Join(d.Root(), replicasDir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	names := make([]string, 0, len(entries))
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
	if class != ClassObject {
		return listArtifacts(base)
	}
	// Objects fan out one level so a large library does not put a hundred
	// thousand entries in one directory, which several providers list slowly
	// and some refuse outright.
	shards, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	var names []string
	for _, shard := range shards {
		if !shard.IsDir() || len(shard.Name()) != 2 || !lowercaseHex(shard.Name()) {
			continue
		}
		found, err := listArtifacts(filepath.Join(base, shard.Name()))
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

func listArtifacts(base string) ([]string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
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
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
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
	info, err := os.Stat(path)
	if err != nil {
		return nil, ErrUnreadable
	}
	if info.IsDir() {
		return nil, ErrUnreadable
	}
	if info.Size() > MaxArtifactBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrTooLarge, info.Size())
	}
	artifact, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrUnreadable
	}
	if len(name) == 2*sha256.Size {
		digest := sha256.Sum256(artifact)
		if hex.EncodeToString(digest[:]) != name {
			return nil, fmt.Errorf("%w: contents do not match the name", ErrUnreadable)
		}
	}
	return artifact, nil
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
	if err := os.Remove(d.artifactPath(namespace, class, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
