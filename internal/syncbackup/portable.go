package syncbackup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synccatchup"
)

// PortableMagic identifies the password-wrapped form of the same NBK1 payload
// used for peer catch-up. Only the payload-key wrapper differs.
const PortableMagic = "NPB1"

const maxPortableHeaderBytes = 64 << 10

type PortableHeader struct {
	Version         int    `json:"version"`
	Format          string `json:"format"`
	WrappingMode    string `json:"wrapping_mode"`
	Salt            string `json:"salt"`
	WrappedKey      string `json:"wrapped_payload_key"`
	SealedBytes     int64  `json:"sealed_bytes"`
	SealedSHA256    string `json:"sealed_sha256"`
	SnapshotID      string `json:"snapshot_id"`
	DatabaseID      string `json:"database_id"`
	SourceReplicaID string `json:"source_replica_id"`
	SchemaVersion   int    `json:"schema_version"`
	CommitSHA256    string `json:"commit_sha256"`
	Objects         int64  `json:"objects"`
}

type PortableReport struct {
	Path   string                           `json:"-"`
	Bytes  int64                            `json:"bytes"`
	Header PortableHeader                   `json:"header"`
	Verify snapshotimage.VerificationReport `json:"verify"`
}

// CreatePortable creates a verified physical snapshot, seals the payload with
// a fresh key, and wraps only that key with Argon2id. The password is never put
// in a struct, file, log value, or command line.
func CreatePortable(ctx context.Context, source *store.SQLiteStore, assetRoot, workspace, password string) (PortableReport, error) {
	if source == nil {
		return PortableReport{}, errors.New("portable backup requires a store")
	}
	if password == "" {
		return PortableReport{}, fmt.Errorf("%w: backup password is required", synccatchup.ErrInvalid)
	}
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return PortableReport{}, err
	}
	snapshotDir := filepath.Join(workspace, "snapshot")
	if _, err := snapshotimage.Create(ctx, source, assetRoot, snapshotDir, snapshotimage.CreateOptions{}); err != nil {
		return PortableReport{}, err
	}
	verified, err := snapshotimage.VerifyDirectory(ctx, snapshotDir, snapshotimage.DefaultLimits())
	if err != nil {
		return PortableReport{}, err
	}
	payloadKey := make([]byte, KeyBytes)
	if _, err := rand.Read(payloadKey); err != nil {
		return PortableReport{}, err
	}
	salt, wrapped, err := synccatchup.WrapWithPassword(password, payloadKey)
	if err != nil {
		return PortableReport{}, err
	}
	sealedPath := filepath.Join(workspace, "payload.nbk")
	sealed, err := os.OpenFile(sealedPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return PortableReport{}, err
	}
	reader, writer := io.Pipe()
	go func() {
		_, packErr := Pack(snapshotDir, writer)
		_ = writer.CloseWithError(packErr)
	}()
	sealedBytes, sealedSHA, sealErr := Seal(reader, payloadKey, sealed)
	_ = reader.CloseWithError(sealErr)
	if sealErr == nil {
		sealErr = sealed.Sync()
	}
	if closeErr := sealed.Close(); sealErr == nil {
		sealErr = closeErr
	}
	if sealErr != nil {
		return PortableReport{}, sealErr
	}
	header := PortableHeader{
		Version: 1, Format: snapshotimage.CapabilitySQLiteImage, WrappingMode: string(synccatchup.WrapPassword),
		Salt: base64.StdEncoding.EncodeToString(salt), WrappedKey: base64.StdEncoding.EncodeToString(wrapped),
		SealedBytes: sealedBytes, SealedSHA256: sealedSHA, SnapshotID: verified.SnapshotID,
		DatabaseID: verified.DatabaseID, SourceReplicaID: verified.SourceReplicaID,
		SchemaVersion: verified.SchemaVersion, CommitSHA256: verified.CommitSHA256, Objects: verified.Objects,
	}
	headerBytes, err := json.Marshal(header)
	if err != nil || len(headerBytes) > maxPortableHeaderBytes {
		return PortableReport{}, fmt.Errorf("portable backup header is not bounded")
	}
	outputPath := filepath.Join(workspace, "notrios-backup.npb")
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return PortableReport{}, err
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(headerBytes)))
	for _, part := range [][]byte{[]byte(PortableMagic), length[:], headerBytes} {
		if _, err = output.Write(part); err != nil {
			output.Close()
			return PortableReport{}, err
		}
	}
	sealed, err = os.Open(sealedPath)
	if err == nil {
		_, err = io.Copy(output, sealed)
		_ = sealed.Close()
	}
	if err == nil {
		err = output.Sync()
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return PortableReport{}, err
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return PortableReport{}, err
	}
	return PortableReport{Path: outputPath, Bytes: info.Size(), Header: header, Verify: verified}, nil
}

// InspectPortable authenticates and verifies a password backup into staging.
// A wrong password is reported before any snapshot parser is invoked, and no
// target database is opened or changed.
func InspectPortable(ctx context.Context, sourcePath, workspace, password string) (PortableReport, error) {
	input, err := os.Open(sourcePath)
	if err != nil {
		return PortableReport{}, err
	}
	defer input.Close()
	header, payloadOffset, err := readPortableHeader(input)
	if err != nil {
		return PortableReport{}, err
	}
	salt, err := base64.StdEncoding.DecodeString(header.Salt)
	if err != nil {
		return PortableReport{}, fmt.Errorf("portable backup salt is malformed")
	}
	wrapped, err := base64.StdEncoding.DecodeString(header.WrappedKey)
	if err != nil {
		return PortableReport{}, fmt.Errorf("portable backup key wrapper is malformed")
	}
	payloadKey, err := synccatchup.UnwrapWithPassword(password, salt, wrapped)
	if err != nil {
		return PortableReport{}, err
	}
	info, err := input.Stat()
	if err != nil || info.Size()-payloadOffset != header.SealedBytes {
		return PortableReport{}, fmt.Errorf("%w: sealed length does not match the header", ErrMalformed)
	}
	section := io.NewSectionReader(input, payloadOffset, header.SealedBytes)
	digest := sha256.New()
	if _, err := io.Copy(digest, section); err != nil || hex.EncodeToString(digest.Sum(nil)) != header.SealedSHA256 {
		return PortableReport{}, fmt.Errorf("%w: sealed digest does not match the header", ErrFrame)
	}
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return PortableReport{}, err
	}
	tarPath := filepath.Join(workspace, "snapshot.tar")
	plain, err := os.OpenFile(tarPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return PortableReport{}, err
	}
	section = io.NewSectionReader(input, payloadOffset, header.SealedBytes)
	openErr := Open(section, payloadKey, plain)
	if closeErr := plain.Close(); openErr == nil {
		openErr = closeErr
	}
	if openErr != nil {
		return PortableReport{}, openErr
	}
	snapshotDir := filepath.Join(workspace, "snapshot")
	if err := Unpack(tarPath, snapshotDir); err != nil {
		return PortableReport{}, err
	}
	if err := os.MkdirAll(filepath.Join(snapshotDir, "packs"), 0o700); err != nil {
		return PortableReport{}, err
	}
	verified, err := snapshotimage.VerifyDirectory(ctx, snapshotDir, snapshotimage.DefaultLimits())
	if err != nil {
		return PortableReport{}, err
	}
	if verified.SnapshotID != header.SnapshotID || verified.DatabaseID != header.DatabaseID ||
		verified.SourceReplicaID != header.SourceReplicaID || verified.SchemaVersion != header.SchemaVersion ||
		verified.CommitSHA256 != header.CommitSHA256 || verified.Objects != header.Objects {
		return PortableReport{}, fmt.Errorf("%w: verified snapshot does not match its password wrapper", ErrFrame)
	}
	return PortableReport{Path: snapshotDir, Bytes: info.Size(), Header: header, Verify: verified}, nil
}

func readPortableHeader(input *os.File) (PortableHeader, int64, error) {
	prefix := make([]byte, len(PortableMagic)+4)
	if _, err := io.ReadFull(input, prefix); err != nil || string(prefix[:len(PortableMagic)]) != PortableMagic {
		return PortableHeader{}, 0, fmt.Errorf("%w: not a password backup", ErrMalformed)
	}
	length := binary.BigEndian.Uint32(prefix[len(PortableMagic):])
	if length == 0 || length > maxPortableHeaderBytes {
		return PortableHeader{}, 0, fmt.Errorf("%w: password backup header is not bounded", ErrMalformed)
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(input, raw); err != nil {
		return PortableHeader{}, 0, fmt.Errorf("%w: truncated password backup header", ErrMalformed)
	}
	var header PortableHeader
	if err := json.Unmarshal(raw, &header); err != nil || header.Version != 1 ||
		header.Format != snapshotimage.CapabilitySQLiteImage || header.WrappingMode != string(synccatchup.WrapPassword) ||
		header.SealedBytes <= 0 || header.SealedBytes > int64(MaxFrames)*(FrameBytes+64) || len(header.SealedSHA256) != 64 {
		return PortableHeader{}, 0, fmt.Errorf("%w: invalid password backup header", ErrMalformed)
	}
	return header, int64(len(prefix)) + int64(length), nil
}
