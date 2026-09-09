package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncwire"
)

// The data plane carries G11's artifacts over G13's authenticated surface.
//
// It is deliberately the *same* artifacts. G11 wrote its exchange against a
// `Carrier` interface, so a REST implementation of that interface makes the
// transcript identical by construction rather than by a second implementation
// that has to be kept in step. What arrives here is opaque sealed bytes; the
// merge stays where it always was, in the round, on the machine that owns the
// library. REST is a courier.

// MaxArtifactUploadBytes bounds one published artifact. It matches the
// carrier's own ceiling, so a peer cannot use HTTP to exceed what a folder
// would have refused.
const MaxArtifactUploadBytes = synccarrier.MaxArtifactBytes

// backupRecord is one produced snapshot, addressed by an opaque id.
//
// The id is random rather than derived from a path or a content hash: the
// boundary G14 was given says downloads use opaque ids and authorization, never
// a server filesystem path, and a hash would still be a name a caller could
// guess at from elsewhere.
type backupRecord struct {
	ID              string    `json:"backup_id"`
	Format          string    `json:"format"`
	SnapshotID      string    `json:"snapshot_id"`
	CommitSHA256    string    `json:"commit_sha256"`
	SourceReplicaID string    `json:"source_replica_id"`
	RequesterID     string    `json:"requester_replica_id"`
	SealedBytes     int64     `json:"sealed_bytes"`
	SealedSHA256    string    `json:"sealed_sha256"`
	WrappedKey      string    `json:"wrapped_payload_key"`
	SnapshotVector  any       `json:"snapshot_vector"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	path            string
}

// backupStore holds produced backups for the lifetime of the service.
type backupStore struct {
	mu      sync.Mutex
	root    string
	records map[string]*backupRecord
}

func newBackupStore(root string) *backupStore {
	return &backupStore{root: root, records: map[string]*backupRecord{}}
}

func (b *backupStore) put(record *backupRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.records[record.ID] = record
}

func (b *backupStore) get(id string) (*backupRecord, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record, found := b.records[id]
	if !found {
		return nil, false
	}
	if time.Now().After(record.ExpiresAt) {
		// An expired backup is removed rather than served: it is a complete
		// copy of a library sitting on disk, and the shortest life that still
		// allows a resumed download is the right one.
		delete(b.records, id)
		_ = os.Remove(record.path)
		return nil, false
	}
	return record, true
}

// dataRoutes registers the transport. Every one of them is behind requirePeer,
// which is what makes "authorization limited to one database and the sync
// capability" a property of the routing table rather than of each handler.
func (s *Server) dataRoutes() {
	s.mux.HandleFunc("GET /api/v1/sync/carrier/namespaces", s.requirePeer(s.handleCarrierNamespaces))
	s.mux.HandleFunc("GET /api/v1/sync/carrier/{namespace}/{class}", s.requirePeer(s.handleCarrierList))
	s.mux.HandleFunc("GET /api/v1/sync/carrier/{namespace}/{class}/{name}", s.requirePeer(s.handleCarrierRead))
	s.mux.HandleFunc("HEAD /api/v1/sync/carrier/{namespace}/{class}/{name}", s.requirePeer(s.handleCarrierRead))
	s.mux.HandleFunc("PUT /api/v1/sync/carrier/{class}/{name}", s.requirePeer(s.handleCarrierPublish))
	s.mux.HandleFunc("DELETE /api/v1/sync/carrier/{class}/{name}", s.requirePeer(s.handleCarrierRemove))
	s.mux.HandleFunc("POST /api/v1/sync/backups", s.requirePeer(s.handleBackupCreate))
	s.mux.HandleFunc("GET /api/v1/sync/backups/{backup_id}", s.requirePeer(s.handleBackupDownload))
	s.mux.HandleFunc("HEAD /api/v1/sync/backups/{backup_id}", s.requirePeer(s.handleBackupDownload))
}

// hostCarrier is the folder this service hosts on behalf of its peers. It is an
// ordinary G11 carrier: the same layout, the same blinded names, the same
// artifacts. A peer that syncs through a shared folder and a peer that syncs
// over REST leave the same bytes behind.
func (s *Server) hostCarrier() (*synccarrier.Directory, error) {
	if s.sync == nil || s.sync.carrierRoot == "" {
		return nil, errors.New("no carrier is configured on this service")
	}
	group, err := s.sync.keys.Current()
	if err != nil {
		return nil, err
	}
	handshake, err := s.sync.store.LocalSyncHandshake(nil)
	if err != nil {
		return nil, err
	}
	return synccarrier.NewDirectory(s.sync.carrierRoot, group, handshake.DatabaseID, handshake.ReplicaID)
}

// callerNamespace is the only namespace a peer may write to, derived from the
// authenticated principal rather than from anything the request said. A peer
// cannot publish into another's namespace here for the same reason it cannot on
// a shared folder — except that here the server enforces it.
func (s *Server) callerNamespace(principal syncauth.Principal) (string, error) {
	group, err := s.sync.keys.Current()
	if err != nil {
		return "", err
	}
	return syncwire.CarrierName(group, "replica", principal.ReplicaID), nil
}

func (s *Server) handleCarrierNamespaces(w http.ResponseWriter, r *http.Request, _ syncauth.Principal) {
	carrier, err := s.hostCarrier()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "this service hosts no sync carrier")
		return
	}
	if err := carrier.Initialize(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "the carrier could not be prepared")
		return
	}
	namespaces, err := carrier.Namespaces(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "the carrier could not be listed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"namespaces": namespaces})
}

func (s *Server) handleCarrierList(w http.ResponseWriter, r *http.Request, _ syncauth.Principal) {
	carrier, err := s.hostCarrier()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "this service hosts no sync carrier")
		return
	}
	names, err := carrier.List(r.Context(), r.PathValue("namespace"), synccarrier.Class(r.PathValue("class")))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "the carrier could not be listed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"names": names})
}

// handleCarrierRead serves one artifact, with HTTP range support.
//
// Ranges are `http.ServeContent`'s, as F3 established for resource content: the
// standard library already implements `206`, `416`, `Content-Range`, and
// conditional requests correctly, and a second implementation of range parsing
// is a second place to get it wrong.
func (s *Server) handleCarrierRead(w http.ResponseWriter, r *http.Request, _ syncauth.Principal) {
	carrier, err := s.hostCarrier()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "this service hosts no sync carrier")
		return
	}
	artifact, err := carrier.Read(r.Context(), r.PathValue("namespace"),
		synccarrier.Class(r.PathValue("class")), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_not_found", "no such artifact")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	// The entity tag is the artifact's own name, which is either a content hash
	// or a stable keyed blind of what it covers. Either way it changes when the
	// artifact does, which is the whole contract of an ETag.
	w.Header().Set("ETag", `"`+r.PathValue("name")+`"`)
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(string(artifact)))
}

func (s *Server) handleCarrierPublish(w http.ResponseWriter, r *http.Request, principal syncauth.Principal) {
	carrier, err := s.hostCarrier()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "this service hosts no sync carrier")
		return
	}
	namespace, err := s.callerNamespace(principal)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "carrier_unavailable", "no local key material")
		return
	}
	artifact, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxArtifactUploadBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "artifact_too_large", "artifact exceeds the protocol limit")
		return
	}
	name, err := carrier.PublishTo(r.Context(), namespace, synccarrier.Class(r.PathValue("class")),
		r.PathValue("name"), artifact)
	if err != nil {
		writeError(w, http.StatusBadRequest, "artifact_refused", "the carrier refused that artifact")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "bytes": len(artifact)})
}

func (s *Server) handleCarrierRemove(w http.ResponseWriter, r *http.Request, principal syncauth.Principal) {
	carrier, err := s.hostCarrier()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "carrier_unavailable", "this service hosts no sync carrier")
		return
	}
	namespace, err := s.callerNamespace(principal)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "carrier_unavailable", "no local key material")
		return
	}
	if err := carrier.RemoveFrom(r.Context(), namespace, synccarrier.Class(r.PathValue("class")), r.PathValue("name")); err != nil {
		writeError(w, http.StatusBadRequest, "artifact_refused", "the carrier refused that removal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": r.PathValue("name")})
}

// handleBackupCreate produces a snapshot for a peer that is explicitly
// permitted to receive one.
//
// Enrolment is not permission: answering means handing over a complete copy of
// the library, which G10 made a separate, revocable decision. This route checks
// that decision before it does any work at all.
func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request, principal syncauth.Principal) {
	if !s.sync.store.SnapshotSources().PermittedSource(principal.ReplicaID) {
		_ = s.sync.store.RecordSyncAuthEvent(r.Context(), "peer.auth_refused", principal.ReplicaID, "not_a_permitted_snapshot_source")
		writeError(w, http.StatusForbidden, "not_permitted",
			"that replica is not permitted to receive a snapshot of this library")
		return
	}
	if s.sync.backups == nil {
		writeError(w, http.StatusServiceUnavailable, "backups_unavailable", "this service produces no backups")
		return
	}
	// The service-wide write deadline is deliberately short for ordinary API
	// traffic. Snapshot creation cannot write response headers until Online
	// Backup, packing, and sealing complete, so extend only this authenticated,
	// separately permitted operation to the same bounded G14 stage ceiling as
	// the client. ResponseController reaches the underlying net/http writer even
	// through middleware; direct recorder tests may not implement deadlines.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(syncauth.BackupCreationTimeout))
	workspace, err := os.MkdirTemp(s.sync.backups.root, "staging-")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "could not stage the snapshot")
		return
	}
	defer os.RemoveAll(workspace)
	snapshotDir := filepath.Join(workspace, "snapshot")
	if _, err := snapshotimage.Create(r.Context(), s.sync.store, s.sync.store.AssetRoot(), snapshotDir, snapshotimage.CreateOptions{}); err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "the snapshot export failed")
		return
	}
	verified, err := snapshotimage.VerifyDirectory(r.Context(), snapshotDir, snapshotimage.DefaultLimits())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "the physical snapshot did not verify")
		return
	}

	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "could not name the snapshot")
		return
	}
	payloadKey := make([]byte, syncbackup.KeyBytes)
	if _, err := rand.Read(payloadKey); err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "could not key the snapshot")
		return
	}
	record := &backupRecord{
		ID: hex.EncodeToString(identifier), RequesterID: principal.ReplicaID,
		Format: snapshotimage.CapabilitySQLiteImage, SnapshotID: verified.SnapshotID,
		CommitSHA256: verified.CommitSHA256, SourceReplicaID: verified.SourceReplicaID,
		SnapshotVector: verified.SnapshotVector, CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	record.path = filepath.Join(s.sync.backups.root, record.ID+".nbk")

	// Packed and sealed in one pass through a pipe: the container is never
	// buffered whole, so a library's size does not become this process's memory.
	sealed, err := os.OpenFile(record.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "could not open the snapshot file")
		return
	}
	reader, writer := io.Pipe()
	go func() {
		_, packErr := syncbackup.Pack(snapshotDir, writer)
		_ = writer.CloseWithError(packErr)
	}()
	length, digest, sealErr := syncbackup.Seal(reader, payloadKey, sealed)
	_ = reader.CloseWithError(sealErr)
	if sealErr == nil {
		sealErr = sealed.Sync()
	}
	closeErr := sealed.Close()
	if sealErr != nil || closeErr != nil {
		os.Remove(record.path)
		writeError(w, http.StatusInternalServerError, "backup_failed", "the snapshot could not be sealed")
		return
	}
	record.SealedBytes = length
	record.SealedSHA256 = digest

	// The payload key travels sealed for the enrolled group, which is G10's
	// peer-key wrapping mode: both replicas hold the group key, and the
	// artifact is signed so the receiver knows who produced it.
	wrapped, err := syncwire.Seal(s.sync.keys, s.sync.signer, syncwire.KindObject, "", payloadKey, syncwire.Limits{})
	if err != nil {
		os.Remove(record.path)
		writeError(w, http.StatusInternalServerError, "backup_failed", "could not wrap the payload key")
		return
	}
	record.WrappedKey = encodeBase64(wrapped)
	s.sync.backups.put(record)
	writeJSON(w, http.StatusOK, record)
}

// handleBackupDownload serves a produced snapshot, resumably.
//
// Only the replica that asked for it may fetch it, which is not the same
// question as being enrolled: a backup is a complete copy of a library, and
// authorization for one is not authorization for another's.
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request, principal syncauth.Principal) {
	if s.sync.backups == nil {
		writeError(w, http.StatusNotFound, "backup_not_found", "no such backup")
		return
	}
	record, found := s.sync.backups.get(r.PathValue("backup_id"))
	if !found || record.RequesterID != principal.ReplicaID {
		// One answer for "no such backup" and "not yours": the difference would
		// tell a caller which opaque ids exist.
		writeError(w, http.StatusNotFound, "backup_not_found", "no such backup")
		return
	}
	file, err := os.Open(record.path)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup_not_found", "no such backup")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", `"`+record.SealedSHA256+`"`)
	w.Header().Set("X-Notrios-Sealed-SHA256", record.SealedSHA256)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "", record.CreatedAt, file)
}

func encodeBase64(value []byte) string {
	return base64.StdEncoding.EncodeToString(value)
}
