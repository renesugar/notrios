// Package synckeys is the development secret provider for v0.7 synchronization.
//
// It is deliberately small and deliberately labelled. v0.7's resolved decision
// is that the milestone defines an injectable secret-store interface and ships
// an explicit locked-file development provider with warnings and `0600`, while
// v0.8 selects and validates the native desktop and Android stores. This file
// is that development provider and nothing more: a JSON file holding one
// database group key per epoch, this replica's Ed25519 signing key, and the
// public keys of peers a user has explicitly paired with.
//
// What it is not: an operating-system keychain, a password-protected store, or
// a defence against another process running as the same user. A user who can
// read the file can read the library it protects.
package synckeys

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/renesugar/notrios/internal/paths"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// FileVersion is the on-disk format version.
const FileVersion = 1

var (
	// ErrNoKeyFile reports that no key material has been created yet.
	ErrNoKeyFile = errors.New("no sync key file: run `notriosctl sync init` first")
	// ErrInsecurePermissions reports a key file other users can read.
	ErrInsecurePermissions = errors.New("sync key file must not be readable by other users")
	// ErrUnknownFormat reports a file this version cannot read.
	ErrUnknownFormat = errors.New("unrecognized sync key file")
)

type peerKey struct {
	PublicKey string `json:"public_key"`
	ReplicaID string `json:"replica_id"`
}

type keyFile struct {
	Version      int                `json:"version"`
	KeyID        string             `json:"key_id"`
	CurrentEpoch uint32             `json:"current_epoch"`
	Epochs       map[string]string  `json:"epochs"`
	Retired      []uint32           `json:"retired,omitempty"`
	SigningKey   string             `json:"signing_key"`
	Peers        map[string]peerKey `json:"peers,omitempty"`
}

// KeyFile is an opened development key file. It satisfies syncwire's KeyRing,
// Signer, and Verifier interfaces, which is the whole point of the split: the
// protocol never learns where a key came from.
type KeyFile struct {
	path    string
	data    keyFile
	private ed25519.PrivateKey
	// paired records that this process has already adopted a group key from a
	// peer, so a second adoption in one session is refused for the same reason
	// a second one across sessions is.
	paired bool
}

// DefaultPath is where key material lives when a config does not say
// otherwise. It sits beside the profile registry rather than inside the
// library, because a database copied to another machine must not carry the
// keys that decrypt its traffic.
func DefaultPath(databaseID string) (string, error) {
	// The config root comes from internal/paths so this and the profile
	// registry cannot disagree about where it is; before H4 this asked
	// os.UserConfigDir and internal/profiles hand-rolled the same lookup, and
	// they gave different answers for a relative XDG_CONFIG_HOME.
	root, err := paths.ConfigRoot()
	if err != nil {
		return "", err
	}
	name := "sync-keys.json"
	if databaseID != "" {
		name = fmt.Sprintf("sync-keys-%s.json", databaseID)
	}
	return filepath.Join(root, name), nil
}

// Create writes new key material: one fresh group key at epoch one, and one
// fresh replica signing key. It refuses to overwrite an existing file, because
// replacing a group key silently would make every artifact a peer has already
// published unreadable.
func Create(path string) (*KeyFile, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("sync key file already exists: %s", path)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	group := make([]byte, syncwire.GroupKeyBytes)
	if _, err := rand.Read(group); err != nil {
		return nil, err
	}
	keyID := make([]byte, 16)
	if _, err := rand.Read(keyID); err != nil {
		return nil, err
	}
	file := &KeyFile{
		path: path,
		data: keyFile{
			Version: FileVersion, KeyID: base64.RawURLEncoding.EncodeToString(keyID), CurrentEpoch: 1,
			Epochs:     map[string]string{"1": base64.StdEncoding.EncodeToString(group)},
			SigningKey: base64.StdEncoding.EncodeToString(private),
			Peers:      map[string]peerKey{},
		},
		private: private,
	}
	_ = public
	if err := file.save(); err != nil {
		return nil, err
	}
	return file, nil
}

// Open reads existing key material and refuses a file other users can read.
func Open(path string) (*KeyFile, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNoKeyFile, path)
	}
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%w: %s is mode %#o", ErrInsecurePermissions, path, info.Mode().Perm())
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file := &KeyFile{path: path}
	if err := json.Unmarshal(contents, &file.data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnknownFormat, err)
	}
	if file.data.Version != FileVersion || file.data.KeyID == "" || file.data.SigningKey == "" {
		return nil, fmt.Errorf("%w: version %d", ErrUnknownFormat, file.data.Version)
	}
	raw, err := base64.StdEncoding.DecodeString(file.data.SigningKey)
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: signing key is not an Ed25519 private key", ErrUnknownFormat)
	}
	file.private = ed25519.PrivateKey(raw)
	if file.data.Peers == nil {
		file.data.Peers = map[string]peerKey{}
	}
	return file, nil
}

func (f *KeyFile) save() error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(f.data, "", "  ")
	if err != nil {
		return err
	}
	// Written 0600 from the first byte rather than chmodded afterwards: a
	// window in which a private key is world-readable is not a small one when
	// another process is looking for exactly that.
	return os.WriteFile(f.path, append(contents, '\n'), 0o600)
}

// Path returns the file this material was loaded from.
func (f *KeyFile) Path() string { return f.path }

// Current implements syncwire.KeyRing.
func (f *KeyFile) Current() (syncwire.GroupKey, error) {
	return f.Lookup(f.data.KeyID, f.data.CurrentEpoch)
}

// Lookup implements syncwire.KeyRing.
func (f *KeyFile) Lookup(keyID string, epoch uint32) (syncwire.GroupKey, error) {
	if keyID != f.data.KeyID {
		return syncwire.GroupKey{}, fmt.Errorf("%w: key %q", syncwire.ErrNoKey, keyID)
	}
	for _, retired := range f.data.Retired {
		if retired == epoch {
			return syncwire.GroupKey{}, fmt.Errorf("%w: epoch %d", syncwire.ErrRevokedEpoch, epoch)
		}
	}
	encoded, found := f.data.Epochs[fmt.Sprint(epoch)]
	if !found {
		return syncwire.GroupKey{}, fmt.Errorf("%w: epoch %d", syncwire.ErrNoKey, epoch)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) != syncwire.GroupKeyBytes {
		return syncwire.GroupKey{}, fmt.Errorf("%w: epoch %d is not a group key", ErrUnknownFormat, epoch)
	}
	group := syncwire.GroupKey{KeyID: keyID, Epoch: epoch}
	copy(group.Key[:], raw)
	return group, nil
}

// SignerKeyID implements syncwire.Signer.
func (f *KeyFile) SignerKeyID() string {
	return syncwire.SignerKeyID(f.private.Public().(ed25519.PublicKey))
}

// Sign implements syncwire.Signer.
func (f *KeyFile) Sign(message []byte) []byte { return ed25519.Sign(f.private, message) }

// PublicSigningKey is this replica's public identity, which peers enrol.
func (f *KeyFile) PublicSigningKey() ed25519.PublicKey {
	return f.private.Public().(ed25519.PublicKey)
}

// PrivateSigningKey is used to sign REST requests. It never leaves the process
// and is deliberately not part of any serialized structure.
func (f *KeyFile) PrivateSigningKey() ed25519.PrivateKey { return f.private }

// AdoptGroupKey installs a group key received through a pairing exchange.
//
// It is the REST counterpart of ImportBundle, minus the peer key: G13 records
// peer public keys in the database, where enrolment and revocation are
// transactional and auditable, and leaves this file holding only secrets.
func (f *KeyFile) AdoptGroupKey(keyID string, epoch uint32, key []byte) error {
	if len(key) != syncwire.GroupKeyBytes {
		return fmt.Errorf("%w: a group key is %d bytes", ErrUnknownFormat, syncwire.GroupKeyBytes)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	slot := fmt.Sprint(epoch)
	if f.data.KeyID == keyID {
		if existing, held := f.data.Epochs[slot]; held && existing != encoded {
			return fmt.Errorf("this replica already holds a different key for %s epoch %d", keyID, epoch)
		}
		f.data.Epochs[slot] = encoded
		f.data.CurrentEpoch = epoch
		return f.save()
	}
	if len(f.data.Peers) > 0 || f.paired {
		return fmt.Errorf(
			"this replica is already paired under key %s; adopting key %s would make its peers unreadable",
			f.data.KeyID, keyID)
	}
	f.data.KeyID = keyID
	f.data.Epochs = map[string]string{slot: encoded}
	f.data.CurrentEpoch = epoch
	f.data.Retired = nil
	f.paired = true
	return f.save()
}

// AdvanceEpoch mints a new group key and makes it current.
//
// It is the rotation hook G13 owes: after a revocation, remaining peers move to
// a new epoch so a compromised device cannot read what is published next.
// Retiring the old epoch is a separate act, because a library should not lose
// the ability to read its own history in order to exclude a device.
func (f *KeyFile) AdvanceEpoch() (uint32, error) {
	key := make([]byte, syncwire.GroupKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return 0, err
	}
	epoch := f.data.CurrentEpoch + 1
	f.data.Epochs[fmt.Sprint(epoch)] = base64.StdEncoding.EncodeToString(key)
	f.data.CurrentEpoch = epoch
	return epoch, f.save()
}

// RetireEpoch refuses artifacts published under an epoch.
func (f *KeyFile) RetireEpoch(epoch uint32) error {
	for _, retired := range f.data.Retired {
		if retired == epoch {
			return nil
		}
	}
	f.data.Retired = append(f.data.Retired, epoch)
	return f.save()
}

// PublicKey implements syncwire.Verifier. This replica's own key is included
// so that a round can read back and verify what it published itself.
func (f *KeyFile) PublicKey(signerKeyID string) (ed25519.PublicKey, bool) {
	if signerKeyID == f.SignerKeyID() {
		return f.private.Public().(ed25519.PublicKey), true
	}
	peer, found := f.data.Peers[signerKeyID]
	if !found {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(peer.PublicKey)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, false
	}
	return ed25519.PublicKey(raw), true
}

// EnrolledPeers lists the signing keys this file trusts, newest last.
func (f *KeyFile) EnrolledPeers() []string {
	keys := make([]string, 0, len(f.data.Peers))
	for keyID := range f.data.Peers {
		keys = append(keys, keyID)
	}
	sort.Strings(keys)
	return keys
}

// Bundle is the development pairing artifact: one file a user copies from one
// replica to another by whatever means they trust.
//
// It contains a reusable library key in plain text, which is exactly what G13's
// resolved decision says a real pairing bundle must not do. G13 replaces this
// with a short-lived, one-use exchange that proves possession of the signing
// key; until then this is a development convenience and every command that
// touches it says so.
type Bundle struct {
	Version   int                 `json:"version"`
	KeyID     string              `json:"key_id"`
	Epoch     uint32              `json:"epoch"`
	GroupKey  string              `json:"group_key"`
	PublicKey string              `json:"public_key"`
	Handshake syncstate.Handshake `json:"handshake"`
}

// ExportBundle produces the pairing material another replica needs to accept
// this one: the group key, this replica's public signing key, and the
// compatibility handshake its admission configuration requires.
func (f *KeyFile) ExportBundle(handshake syncstate.Handshake) (Bundle, error) {
	group, err := f.Current()
	if err != nil {
		return Bundle{}, err
	}
	// The vector is reduced to the one entry a handshake is required to carry —
	// this replica's own, at zero. A pairing file is about identity and
	// compatibility; a real vector in it would be stale the moment it was
	// written, and it would put a description of the library's shape into a
	// file that gets carried around on removable media.
	handshake.StateVector = syncstate.Vector{handshake.ReplicaID: 0}
	return Bundle{
		Version: FileVersion, KeyID: group.KeyID, Epoch: group.Epoch,
		GroupKey:  base64.StdEncoding.EncodeToString(group.Key[:]),
		PublicKey: base64.StdEncoding.EncodeToString(f.private.Public().(ed25519.PublicKey)),
		Handshake: handshake,
	}, nil
}

// ImportBundle adopts a peer's group key and signing key.
//
// Adopting the group key is the act that joins a database's sync group, so it
// refuses to silently replace a different one: two peers that disagree about
// the group key cannot read each other, and discovering that by having every
// artifact fail to decrypt is a bad way to learn it.
func (f *KeyFile) ImportBundle(bundle Bundle) (string, error) {
	if bundle.Version != FileVersion || bundle.GroupKey == "" || bundle.PublicKey == "" {
		return "", fmt.Errorf("%w: pairing bundle version %d", ErrUnknownFormat, bundle.Version)
	}
	if bundle.Handshake.ReplicaID == "" || bundle.Handshake.DatabaseID == "" {
		return "", fmt.Errorf("%w: a pairing bundle names its replica and database", ErrUnknownFormat)
	}
	group, err := base64.StdEncoding.DecodeString(bundle.GroupKey)
	if err != nil || len(group) != syncwire.GroupKeyBytes {
		return "", fmt.Errorf("%w: bundle group key", ErrUnknownFormat)
	}
	public, err := base64.StdEncoding.DecodeString(bundle.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize {
		return "", fmt.Errorf("%w: bundle public key", ErrUnknownFormat)
	}
	epoch := fmt.Sprint(bundle.Epoch)
	existing, held := f.data.Epochs[epoch]
	sameGroup := f.data.KeyID == bundle.KeyID && held && existing == bundle.GroupKey
	if !sameGroup {
		// Joining a group means adopting its key, and a replica that has never
		// paired with anyone has nothing to lose by doing so: the key it minted
		// at init has protected only artifacts no one else could read anyway.
		//
		// Once it has a peer, the same act would make everything that peer
		// published unreadable, so it is refused. The consequence for a user is
		// an order: the replica that is joining pairs first, and only then hands
		// out its own bundle.
		if len(f.data.Peers) > 0 {
			return "", fmt.Errorf(
				"this replica is already paired under key %s; adopting key %s would make its peers unreadable",
				f.data.KeyID, bundle.KeyID)
		}
		f.data.KeyID = bundle.KeyID
		f.data.Epochs = map[string]string{epoch: bundle.GroupKey}
		f.data.Retired = nil
	}
	f.data.CurrentEpoch = bundle.Epoch
	signerKeyID := syncwire.SignerKeyID(ed25519.PublicKey(public))
	f.data.Peers[signerKeyID] = peerKey{PublicKey: bundle.PublicKey, ReplicaID: bundle.Handshake.ReplicaID}
	return signerKeyID, f.save()
}

// Redacted describes the key file without disclosing any of it.
func (f *KeyFile) Redacted() map[string]any {
	return map[string]any{
		"path":           f.path,
		"key_id":         f.data.KeyID,
		"current_epoch":  f.data.CurrentEpoch,
		"epochs_held":    len(f.data.Epochs),
		"retired_epochs": f.data.Retired,
		"signer_key_id":  f.SignerKeyID(),
		"enrolled_peers": f.EnrolledPeers(),
	}
}
