// Package snapshotimage implements the same-schema whole-library physical
// snapshot selected by v0.7 G14b. It is deliberately local-filesystem only;
// encrypted catch-up transport and destructive installation remain separate
// boundaries.
package snapshotimage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncstate"
)

const (
	FormatName    = "notrios-sqlite-image"
	FormatVersion = 1

	CapabilitySQLiteImage = "sqlite-image+packed-assets.v1"
	CapabilityArchiveV2   = "archive-v2.fallback.v1"
	ApplicationID         = "notrios"
	SemanticFallback      = "notrios-archive-v2"

	DatabaseFile = "notes.sqlite"
	ManifestFile = "manifest.json"

	DefaultPackTargetBytes int64 = 256 << 20
	DefaultPackMaxEntries        = 65_536
)

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]{1,127}$`)
	shaPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	packPathRegex = regexp.MustCompile(`^packs/pack-[0-9]{6}\.tar$`)
)

// Limits are hard reader-side bounds. A writer may choose smaller pack
// targets, but never a wider object, manifest, path, or aggregate bound without
// a new capability review.
type Limits struct {
	MaxManifestBytes int64
	MaxPacks         int
	MaxObjects       int64
	MaxObjectBytes   int64
	MaxTotalBytes    int64
	MaxPathBytes     int
	MaxVectorEntries int
}

func DefaultLimits() Limits {
	return Limits{
		MaxManifestBytes: 4 << 20,
		MaxPacks:         65_536,
		MaxObjects:       8_000_000,
		MaxObjectBytes:   16 << 30,
		MaxTotalBytes:    4 << 40,
		MaxPathBytes:     255,
		MaxVectorEntries: syncstate.MaxStateVectorEntries,
	}
}

type Manifest struct {
	Format        string           `json:"format"`
	Version       int              `json:"version"`
	CommitSHA256  string           `json:"commit_sha256"`
	ContentSHA256 string           `json:"content_sha256"`
	Snapshot      SnapshotMetadata `json:"snapshot"`
	Compatibility Compatibility    `json:"compatibility"`
	Database      FileDescriptor   `json:"database"`
	External      ExternalManifest `json:"external"`
	ClearedTables []string         `json:"cleared_local_tables"`
}

type SnapshotMetadata struct {
	ID              string           `json:"id"`
	CreatedAt       string           `json:"created_at"`
	Consistency     string           `json:"consistency"`
	DatabaseID      string           `json:"database_id"`
	SourceReplicaID string           `json:"source_replica_id"`
	Vector          syncstate.Vector `json:"vector"`
	Floors          syncstate.Vector `json:"floors"`
}

type Compatibility struct {
	ApplicationID          string   `json:"application_id"`
	ApplicationVersion     string   `json:"application_version"`
	SourceSchemaVersion    int      `json:"source_schema_version"`
	MinimumSchemaVersion   int      `json:"minimum_schema_version"`
	MaximumSchemaVersion   int      `json:"maximum_schema_version"`
	RequiredCapabilities   []string `json:"required_capabilities"`
	SemanticFallbackFormat string   `json:"semantic_fallback_format"`
}

type FileDescriptor struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

type ExternalManifest struct {
	Layout       string           `json:"layout"`
	PackTarget   int64            `json:"pack_target_payload_bytes"`
	PackEntries  int              `json:"pack_max_entries"`
	Objects      int64            `json:"objects"`
	PayloadBytes int64            `json:"payload_bytes"`
	Packs        []PackDescriptor `json:"packs"`
}

type PackDescriptor struct {
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"size_bytes"`
	PayloadBytes int64  `json:"payload_bytes"`
	Entries      int    `json:"entries"`
	FirstPath    string `json:"first_path"`
	LastPath     string `json:"last_path"`
	Oversized    bool   `json:"oversized,omitempty"`
}

type VerificationReport struct {
	SnapshotID      string `json:"snapshot_id"`
	DatabaseID      string `json:"database_id"`
	SourceReplicaID string `json:"source_replica_id"`
	SchemaVersion   int    `json:"schema_version"`
	CommitSHA256    string `json:"commit_sha256"`
	ContentSHA256   string `json:"content_sha256"`
	Packs           int    `json:"packs"`
	Objects         int64  `json:"objects"`
	DatabaseBytes   int64  `json:"database_bytes"`
	ExternalBytes   int64  `json:"external_bytes"`
	ReadyForInstall bool   `json:"ready_for_install"`
	InstallBoundary string `json:"install_boundary"`
}

func ComputeCommitSHA256(manifest Manifest) (string, error) {
	manifest.CommitSHA256 = ""
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func finalizeManifest(manifest *Manifest) error {
	digest, err := ComputeCommitSHA256(*manifest)
	if err != nil {
		return err
	}
	manifest.CommitSHA256 = digest
	return nil
}

func validateManifest(manifest Manifest, limits Limits) error {
	if manifest.Format != FormatName || manifest.Version != FormatVersion {
		return fmt.Errorf("unsupported snapshot format/version %q/%d", manifest.Format, manifest.Version)
	}
	if !shaPattern.MatchString(manifest.CommitSHA256) || !shaPattern.MatchString(manifest.ContentSHA256) {
		return fmt.Errorf("snapshot digests must be lowercase SHA-256")
	}
	wantCommit, err := ComputeCommitSHA256(manifest)
	if err != nil || wantCommit != manifest.CommitSHA256 {
		return fmt.Errorf("snapshot manifest checksum mismatch")
	}
	if !idPattern.MatchString(manifest.Snapshot.ID) || !idPattern.MatchString(manifest.Snapshot.DatabaseID) || !idPattern.MatchString(manifest.Snapshot.SourceReplicaID) {
		return fmt.Errorf("snapshot, database, and replica IDs must be bounded opaque IDs")
	}
	if manifest.Snapshot.Consistency != "sqlite-online-backup" {
		return fmt.Errorf("unsupported snapshot consistency %q", manifest.Snapshot.Consistency)
	}
	if err := validateTimestamp(manifest.Snapshot.CreatedAt); err != nil {
		return err
	}
	if err := validateVector("snapshot vector", manifest.Snapshot.Vector, limits); err != nil {
		return err
	}
	if err := validateVector("snapshot floors", manifest.Snapshot.Floors, limits); err != nil {
		return err
	}
	compatibility := manifest.Compatibility
	if compatibility.ApplicationID != ApplicationID {
		return fmt.Errorf("snapshot application is %q, not %q", compatibility.ApplicationID, ApplicationID)
	}
	if strings.TrimSpace(compatibility.ApplicationVersion) == "" || len(compatibility.ApplicationVersion) > 64 {
		return fmt.Errorf("snapshot application version is invalid")
	}
	if compatibility.SourceSchemaVersion != store.CurrentSchemaVersion || compatibility.MinimumSchemaVersion != store.CurrentSchemaVersion || compatibility.MaximumSchemaVersion != store.CurrentSchemaVersion {
		return fmt.Errorf("snapshot schema %d range [%d,%d] is incompatible with exact schema %d; use %s", compatibility.SourceSchemaVersion, compatibility.MinimumSchemaVersion, compatibility.MaximumSchemaVersion, store.CurrentSchemaVersion, SemanticFallback)
	}
	wantCapabilities := []string{CapabilityArchiveV2, CapabilitySQLiteImage}
	if fmt.Sprint(compatibility.RequiredCapabilities) != fmt.Sprint(wantCapabilities) {
		return fmt.Errorf("unsupported required snapshot capabilities %v", compatibility.RequiredCapabilities)
	}
	if compatibility.SemanticFallbackFormat != SemanticFallback {
		return fmt.Errorf("snapshot does not declare the required semantic fallback")
	}
	if manifest.Database.Path != DatabaseFile || !shaPattern.MatchString(manifest.Database.SHA256) || manifest.Database.SizeBytes <= 0 || manifest.Database.SizeBytes > limits.MaxTotalBytes {
		return fmt.Errorf("snapshot database descriptor is invalid")
	}
	if manifest.External.Layout != "deterministic-ustar" || manifest.External.PackTarget <= 0 || manifest.External.PackTarget > DefaultPackTargetBytes || manifest.External.PackEntries <= 0 || manifest.External.PackEntries > DefaultPackMaxEntries {
		return fmt.Errorf("snapshot external pack policy is invalid")
	}
	if manifest.External.Objects < 0 || manifest.External.Objects > limits.MaxObjects || manifest.External.PayloadBytes < 0 || manifest.External.PayloadBytes > limits.MaxTotalBytes || len(manifest.External.Packs) > limits.MaxPacks {
		return fmt.Errorf("snapshot external totals exceed reader limits")
	}
	if manifest.External.Objects == 0 && len(manifest.External.Packs) != 0 {
		return fmt.Errorf("empty external inventory must not contain packs")
	}
	var entries int64
	var payload int64
	previous := ""
	for index, pack := range manifest.External.Packs {
		wantPath := fmt.Sprintf("packs/pack-%06d.tar", index)
		if pack.Path != wantPath || !packPathRegex.MatchString(pack.Path) || !shaPattern.MatchString(pack.SHA256) {
			return fmt.Errorf("snapshot pack %d descriptor is invalid", index)
		}
		if pack.SizeBytes <= 0 || pack.SizeBytes > limits.MaxTotalBytes || pack.PayloadBytes < 0 || pack.PayloadBytes > limits.MaxTotalBytes || pack.Entries <= 0 || pack.Entries > manifest.External.PackEntries {
			return fmt.Errorf("snapshot pack %s exceeds reader limits", pack.Path)
		}
		if !validStoragePath(pack.FirstPath, limits) || !validStoragePath(pack.LastPath, limits) || pack.FirstPath > pack.LastPath || (previous != "" && pack.FirstPath <= previous) {
			return fmt.Errorf("snapshot pack %s path bounds are invalid", pack.Path)
		}
		if pack.Oversized {
			if pack.Entries != 1 || pack.PayloadBytes <= manifest.External.PackTarget {
				return fmt.Errorf("snapshot pack %s has an invalid oversized declaration", pack.Path)
			}
		} else if pack.PayloadBytes > manifest.External.PackTarget {
			return fmt.Errorf("snapshot pack %s exceeds its payload target", pack.Path)
		}
		entries += int64(pack.Entries)
		payload += pack.PayloadBytes
		previous = pack.LastPath
	}
	if entries != manifest.External.Objects || payload != manifest.External.PayloadBytes {
		return fmt.Errorf("snapshot external totals do not match pack descriptors")
	}
	wantCleared := append([]string(nil), store.SnapshotLocalTables...)
	sort.Strings(wantCleared)
	if !sort.StringsAreSorted(manifest.ClearedTables) || fmt.Sprint(manifest.ClearedTables) != fmt.Sprint(wantCleared) {
		return fmt.Errorf("snapshot local-state clearing declaration is incomplete")
	}
	return nil
}

func validateVector(name string, vector syncstate.Vector, limits Limits) error {
	if len(vector) > limits.MaxVectorEntries {
		return fmt.Errorf("%s exceeds reader limit", name)
	}
	if err := syncstate.ValidateVector(vector); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
