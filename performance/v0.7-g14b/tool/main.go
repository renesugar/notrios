// Command tool exposes the current production catch-up pack/seal/open
// boundaries to the G14b performance harness without changing production APIs.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncwire"
)

type report struct {
	InputBytes  int64  `json:"input_bytes,omitempty"`
	OutputBytes int64  `json:"output_bytes,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Converged   bool   `json:"converged,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fail("usage: g14b-tool pack|seal|open|unpack ...")
	}
	var result report
	var err error
	switch os.Args[1] {
	case "pack":
		if len(os.Args) != 4 {
			fail("usage: g14b-tool pack <directory> <zip>")
		}
		result, err = pack(os.Args[2], os.Args[3])
	case "seal":
		if len(os.Args) != 5 {
			fail("usage: g14b-tool seal <input> <key-file> <output>")
		}
		result, err = seal(os.Args[2], os.Args[3], os.Args[4])
	case "open":
		if len(os.Args) != 5 {
			fail("usage: g14b-tool open <input> <key-file> <output>")
		}
		result, err = open(os.Args[2], os.Args[3], os.Args[4])
	case "unpack":
		if len(os.Args) != 4 {
			fail("usage: g14b-tool unpack <zip> <directory>")
		}
		err = syncbackup.Unpack(os.Args[2], os.Args[3])
	case "rotate-replica":
		if len(os.Args) != 4 {
			fail("usage: g14b-tool rotate-replica <database> <asset-root>")
		}
		err = rotateReplica(os.Args[2], os.Args[3])
	case "incremental-replay":
		if len(os.Args) != 7 {
			fail("usage: g14b-tool incremental-replay <source-db> <source-assets> <target-db> <target-assets> <carrier>")
		}
		result.Converged, err = incrementalReplay(os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6])
	default:
		fail("unknown command")
	}
	if err != nil {
		fail(err.Error())
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(result); err != nil {
		fail(err.Error())
	}
}

type keyRing struct {
	group   syncwire.GroupKey
	private ed25519.PrivateKey
}

func (k *keyRing) Current() (syncwire.GroupKey, error)              { return k.group, nil }
func (k *keyRing) Lookup(string, uint32) (syncwire.GroupKey, error) { return k.group, nil }
func (k *keyRing) Sign(message []byte) []byte                       { return ed25519.Sign(k.private, message) }
func (k *keyRing) SignerKeyID() string {
	return syncwire.SignerKeyID(k.private.Public().(ed25519.PublicKey))
}
func (k *keyRing) PublicKey(id string) (ed25519.PublicKey, bool) {
	if id != k.SignerKeyID() {
		return nil, false
	}
	return k.private.Public().(ed25519.PublicKey), true
}

func incrementalReplay(sourceDB, sourceAssets, targetDB, targetAssets, carrierRoot string) (bool, error) {
	ctx := context.Background()
	source, err := store.OpenSQLiteWithAssetStore(sourceDB, sourceAssets)
	if err != nil {
		return false, err
	}
	defer source.Close()
	target, err := store.OpenSQLiteWithAssetStore(targetDB, targetAssets)
	if err != nil {
		return false, err
	}
	defer target.Close()
	if err := source.Bootstrap(ctx); err != nil {
		return false, err
	}
	if err := target.Bootstrap(ctx); err != nil {
		return false, err
	}
	if _, err := source.EnrollLocalJournal(ctx, "G14b full-corpus replay"); err != nil {
		return false, err
	}
	if _, err := target.EnrollLocalJournal(ctx, "G14b full-corpus replay"); err != nil {
		return false, err
	}
	sourceHandshake, err := source.LocalSyncHandshake(ctx)
	if err != nil {
		return false, err
	}
	targetHandshake, err := target.LocalSyncHandshake(ctx)
	if err != nil {
		return false, err
	}
	group := syncwire.GroupKey{KeyID: "key_g14b", Epoch: 1}
	if _, err := rand.Read(group.Key[:]); err != nil {
		return false, err
	}
	sourcePublic, sourcePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return false, err
	}
	targetPublic, targetPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return false, err
	}
	if _, err := source.EnrollPeerSigningKey(ctx, targetHandshake.ReplicaID, targetPublic, "g14b"); err != nil {
		return false, err
	}
	if _, err := target.EnrollPeerSigningKey(ctx, sourceHandshake.ReplicaID, sourcePublic, "g14b"); err != nil {
		return false, err
	}
	if err := source.ConfigureSyncAdmissionPeer(ctx, targetHandshake); err != nil {
		return false, err
	}
	if err := target.ConfigureSyncAdmissionPeer(ctx, sourceHandshake); err != nil {
		return false, err
	}
	const documentID = "doc_g14b_post_snapshot"
	if _, err := source.GetDocument(ctx, documentID); errors.Is(err, store.ErrNotFound) {
		if _, err := source.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: documentID, Title: "Generated post-snapshot evidence", Body: "generated G14b incremental replay\n"}); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	if err := os.MkdirAll(carrierRoot, 0o700); err != nil {
		return false, err
	}
	sourceCarrier, err := synccarrier.NewDirectory(carrierRoot, group, sourceHandshake.DatabaseID, sourceHandshake.ReplicaID)
	if err != nil {
		return false, err
	}
	targetCarrier, err := synccarrier.NewDirectory(carrierRoot, group, targetHandshake.DatabaseID, targetHandshake.ReplicaID)
	if err != nil {
		return false, err
	}
	sourceRing := &keyRing{group: group, private: sourcePrivate}
	targetRing := &keyRing{group: group, private: targetPrivate}
	sourceRound := synccarrier.NewRound(synccarrier.NewStoreReplica(source), sourceCarrier, sourceRing, sourceRing,
		syncwire.MultiVerifier{sourceRing, source.PeerVerifier()}, store.NewLocalObjectProvider(source), synccarrier.Options{})
	targetRound := synccarrier.NewRound(synccarrier.NewStoreReplica(target), targetCarrier, targetRing, targetRing,
		syncwire.MultiVerifier{targetRing, target.PeerVerifier()}, store.NewLocalObjectProvider(target), synccarrier.Options{})
	for pass := 0; pass < 3; pass++ {
		if _, err := sourceRound.Run(ctx); err != nil {
			return false, err
		}
		if _, err := targetRound.Run(ctx); err != nil {
			return false, err
		}
	}
	document, err := target.GetDocument(ctx, documentID)
	if err != nil {
		return false, err
	}
	return document.Body == "generated G14b incremental replay\n", nil
}

func rotateReplica(database, assets string) error {
	st, err := store.OpenSQLiteWithAssetStore(database, assets)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		return err
	}
	_, err = st.RotateReplicaIdentity(context.Background())
	return err
}

func pack(root, target string) (report, error) {
	file, temporary, err := temporaryOutput(target)
	if err != nil {
		return report{}, err
	}
	defer os.Remove(temporary)
	written, packErr := syncbackup.Pack(root, file)
	if closeErr := file.Close(); packErr == nil {
		packErr = closeErr
	}
	if packErr != nil {
		return report{}, packErr
	}
	if err := os.Rename(temporary, target); err != nil {
		return report{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return report{}, err
	}
	return report{InputBytes: written, OutputBytes: info.Size()}, nil
}

func seal(sourcePath, keyPath, target string) (report, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return report{}, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return report{}, err
	}
	defer source.Close()
	file, temporary, err := temporaryOutput(target)
	if err != nil {
		return report{}, err
	}
	defer os.Remove(temporary)
	written, digest, sealErr := syncbackup.Seal(source, key, file)
	if closeErr := file.Close(); sealErr == nil {
		sealErr = closeErr
	}
	if sealErr != nil {
		return report{}, sealErr
	}
	if err := os.Rename(temporary, target); err != nil {
		return report{}, err
	}
	input, _ := os.Stat(sourcePath)
	return report{InputBytes: input.Size(), OutputBytes: written, SHA256: digest}, nil
}

func open(sourcePath, keyPath, target string) (report, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return report{}, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return report{}, err
	}
	defer source.Close()
	file, temporary, err := temporaryOutput(target)
	if err != nil {
		return report{}, err
	}
	defer os.Remove(temporary)
	openErr := syncbackup.Open(source, key, file)
	if closeErr := file.Close(); openErr == nil {
		openErr = closeErr
	}
	if openErr != nil {
		return report{}, openErr
	}
	if err := os.Rename(temporary, target); err != nil {
		return report{}, err
	}
	input, _ := os.Stat(sourcePath)
	output, _ := os.Stat(target)
	return report{InputBytes: input.Size(), OutputBytes: output.Size()}, nil
}

func temporaryOutput(target string) (*os.File, string, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return nil, "", err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".g14b-*")
	if err != nil {
		return nil, "", err
	}
	return file, file.Name(), nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	if key, err := os.ReadFile(path); err == nil {
		if len(key) != syncbackup.KeyBytes {
			return nil, fmt.Errorf("stored key has wrong length")
		}
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, syncbackup.KeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
