// Package harness provides the evidence-only execution contract for the G14a
// archive scalability investigation. It is deliberately outside production
// packages: G14a may measure formats, but it may not select or ship one.
package harness

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	ResultSchema     = "notrios.g14a.phase-result.v1"
	CheckpointSchema = "notrios.g14a.checkpoint.v1"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Stage is one comparable boundary. Adapters must map every stage explicitly,
// even when a repository combines two boundaries or cannot provide one.
type Stage string

const (
	StageInventory         Stage = "inventory"
	StageForeignImport     Stage = "foreign-import"
	StageSnapshotCreate    Stage = "snapshot-create"
	StageSnapshotVerify    Stage = "snapshot-verify"
	StageTransportPrepare  Stage = "transport-prepare"
	StageTransportSeal     Stage = "transport-seal"
	StageSnapshotOpen      Stage = "snapshot-open"
	StageSnapshotRestore   Stage = "snapshot-restore"
	StageIncrementalReplay Stage = "incremental-replay"
)

var Stages = []Stage{
	StageInventory,
	StageForeignImport,
	StageSnapshotCreate,
	StageSnapshotVerify,
	StageTransportPrepare,
	StageTransportSeal,
	StageSnapshotOpen,
	StageSnapshotRestore,
	StageIncrementalReplay,
}

type StageMapping struct {
	Stage       Stage  `json:"stage"`
	Mode        string `json:"mode"` // measured, integrated, shared, or not-applicable
	Description string `json:"description"`
}

type Adapter struct {
	ID                 string         `json:"id"`
	Class              string         `json:"class"`
	Source             string         `json:"source"`
	EncryptionBoundary string         `json:"encryption_boundary"`
	RepositoryPolicy   string         `json:"repository_policy,omitempty"`
	Stages             []StageMapping `json:"stages"`
}

type DirectoryHistogram struct {
	Empty          int64 `json:"empty"`
	OneToTen       int64 `json:"one_to_ten"`
	ElevenTo100    int64 `json:"eleven_to_100"`
	HundredOneTo1K int64 `json:"hundred_one_to_1k"`
	OneKTo10K      int64 `json:"one_k_to_10k"`
	TenKTo100K     int64 `json:"ten_k_to_100k"`
	Over100K       int64 `json:"over_100k"`
}

func (h DirectoryHistogram) Total() int64 {
	return h.Empty + h.OneToTen + h.ElevenTo100 + h.HundredOneTo1K + h.OneKTo10K + h.TenKTo100K + h.Over100K
}

type Inventory struct {
	Files                int64              `json:"files"`
	Directories          int64              `json:"directories"`
	Symlinks             int64              `json:"symlinks"`
	OtherEntries         int64              `json:"other_entries"`
	ApparentBytes        int64              `json:"apparent_bytes"`
	AllocatedBytes       int64              `json:"allocated_bytes"`
	MaximumDirectorySize int64              `json:"maximum_directory_entries"`
	DirectoryHistogram   DirectoryHistogram `json:"directory_entry_histogram"`
	MetadataSHA256       string             `json:"metadata_sha256"`
}

type Environment struct {
	GOOS            string            `json:"goos"`
	GOARCH          string            `json:"goarch"`
	GoVersion       string            `json:"go_version"`
	CPUModel        string            `json:"cpu_model"`
	LogicalCPUs     int               `json:"logical_cpus"`
	SystemMemoryKiB int64             `json:"system_memory_kib"`
	FilesystemType  string            `json:"filesystem_type"`
	ToolVersions    map[string]string `json:"tool_versions"`
}

type Metrics struct {
	Items             int64   `json:"items"`
	Resources         int64   `json:"resources"`
	InputBytes        int64   `json:"input_bytes"`
	OutputBytes       int64   `json:"output_bytes"`
	OutputEntries     int64   `json:"output_entries"`
	ContainerEntries  int64   `json:"container_entries"`
	MaxDirectoryItems int64   `json:"maximum_directory_entries"`
	WallSeconds       float64 `json:"wall_seconds"`
	UserCPUSeconds    float64 `json:"user_cpu_seconds"`
	SystemCPUSeconds  float64 `json:"system_cpu_seconds"`
	PeakRSSBytes      int64   `json:"peak_rss_bytes"`
	ReadBytes         int64   `json:"read_bytes"`
	WriteBytes        int64   `json:"write_bytes"`
	CompressionRatio  float64 `json:"compression_ratio"`
}

type Assertions struct {
	SourceUnchanged      bool   `json:"source_unchanged"`
	ArithmeticValid      bool   `json:"arithmetic_valid"`
	ExactHashVerified    bool   `json:"exact_hash_verified"`
	IntegrityCheck       string `json:"integrity_check,omitempty"`
	CanonicalFingerprint string `json:"canonical_fingerprint,omitempty"`
}

// Result is the only row a phase publishes. It intentionally has no path,
// filename, title, body, source key, warning text, or free-form note field.
type Result struct {
	Schema         string      `json:"schema"`
	Tier           int         `json:"tier"`
	Adapter        string      `json:"adapter"`
	Phase          Stage       `json:"phase"`
	Status         string      `json:"status"`
	CacheState     string      `json:"cache_state"`
	Generated      bool        `json:"generated"`
	Environment    Environment `json:"environment"`
	Source         Inventory   `json:"source_inventory"`
	Output         Inventory   `json:"output_inventory"`
	Metrics        Metrics     `json:"metrics"`
	Assertions     Assertions  `json:"assertions"`
	ArtifactSHA256 string      `json:"artifact_sha256,omitempty"`
	SealedSHA256   string      `json:"sealed_sha256,omitempty"`
}

type Checkpoint struct {
	Schema  string `json:"schema"`
	Tier    int    `json:"tier"`
	Adapter string `json:"adapter"`
	Phase   Stage  `json:"phase"`
	Status  string `json:"status"`
	Attempt int    `json:"attempt"`
}

func ValidateIdentity(tier int, adapter string, phase Stage) error {
	if tier != 10_000 && tier != 100_000 {
		return fmt.Errorf("tier must be 10000 or 100000")
	}
	if !slugPattern.MatchString(adapter) {
		return fmt.Errorf("adapter must be a lowercase slug")
	}
	for _, candidate := range Stages {
		if phase == candidate {
			return nil
		}
	}
	return fmt.Errorf("unsupported phase %q", phase)
}

func ValidateResult(result Result) error {
	if result.Schema != ResultSchema {
		return fmt.Errorf("wrong result schema %q", result.Schema)
	}
	if err := ValidateIdentity(result.Tier, result.Adapter, result.Phase); err != nil {
		return err
	}
	if result.Status != "completed" {
		return fmt.Errorf("result status must be completed")
	}
	if result.CacheState != "interleaved-first" && result.CacheState != "interleaved-repeat" && result.CacheState != "uncontrolled" {
		return fmt.Errorf("invalid cache state %q", result.CacheState)
	}
	if result.Metrics.Items < 0 || result.Metrics.Resources < 0 || result.Metrics.InputBytes < 0 || result.Metrics.OutputBytes < 0 ||
		result.Metrics.OutputEntries < 0 || result.Metrics.ContainerEntries < 0 || result.Metrics.MaxDirectoryItems < 0 || result.Metrics.WallSeconds < 0 ||
		result.Metrics.UserCPUSeconds < 0 || result.Metrics.SystemCPUSeconds < 0 || result.Metrics.PeakRSSBytes < 0 ||
		result.Metrics.ReadBytes < 0 || result.Metrics.WriteBytes < 0 || result.Metrics.CompressionRatio < 0 {
		return fmt.Errorf("metrics cannot be negative")
	}
	if err := validateInventory("source", result.Source); err != nil {
		return err
	}
	if err := validateInventory("output", result.Output); err != nil {
		return err
	}
	if result.Source.Directories != result.Source.DirectoryHistogram.Total() && result.Source.Directories != 0 {
		return fmt.Errorf("source directory histogram totals %d, want %d", result.Source.DirectoryHistogram.Total(), result.Source.Directories)
	}
	if result.Output.Directories != result.Output.DirectoryHistogram.Total() && result.Output.Directories != 0 {
		return fmt.Errorf("output directory histogram totals %d, want %d", result.Output.DirectoryHistogram.Total(), result.Output.Directories)
	}
	if result.Metrics.InputBytes > 0 && result.Metrics.OutputBytes > 0 {
		want := float64(result.Metrics.InputBytes) / float64(result.Metrics.OutputBytes)
		if result.Metrics.CompressionRatio < want*0.999999 || result.Metrics.CompressionRatio > want*1.000001 {
			return fmt.Errorf("compression ratio %.9f does not match %.9f", result.Metrics.CompressionRatio, want)
		}
	}
	if !result.Assertions.SourceUnchanged || !result.Assertions.ArithmeticValid {
		return fmt.Errorf("mandatory source/arithmetic assertions did not pass")
	}
	return ScanAggregatePrivacy(result)
}

func validateInventory(label string, inventory Inventory) error {
	if inventory.Files < 0 || inventory.Directories < 0 || inventory.Symlinks < 0 || inventory.OtherEntries < 0 ||
		inventory.ApparentBytes < 0 || inventory.AllocatedBytes < 0 || inventory.MaximumDirectorySize < 0 ||
		inventory.DirectoryHistogram.Empty < 0 || inventory.DirectoryHistogram.OneToTen < 0 ||
		inventory.DirectoryHistogram.ElevenTo100 < 0 || inventory.DirectoryHistogram.HundredOneTo1K < 0 ||
		inventory.DirectoryHistogram.OneKTo10K < 0 || inventory.DirectoryHistogram.TenKTo100K < 0 ||
		inventory.DirectoryHistogram.Over100K < 0 {
		return fmt.Errorf("%s inventory cannot be negative", label)
	}
	return nil
}

// ScanAggregatePrivacy rejects the field classes that have leaked private
// corpus details in past ad-hoc profiles. Exact artifact hashes are permitted:
// G14a requires them, and generated evidence is safe to commit. G14b keeps
// private-corpus result rows outside the repository.
func ScanAggregatePrivacy(value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	forbiddenKeys := map[string]bool{
		"path": true, "filename": true, "title": true, "body": true,
		"source_key": true, "warning": true, "warnings": true, "argv": true,
	}
	var walk func(any) error
	walk = func(current any) error {
		switch typed := current.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if forbiddenKeys[strings.ToLower(key)] {
					return fmt.Errorf("private detail field %q is forbidden", key)
				}
				if err := walk(typed[key]); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range typed {
				if err := walk(item); err != nil {
					return err
				}
			}
		case string:
			lower := strings.ToLower(typed)
			if strings.Contains(lower, "/home/") || strings.Contains(lower, "\\users\\") || strings.Contains(lower, "file://") {
				return fmt.Errorf("local path-like string is forbidden")
			}
		}
		return nil
	}
	return walk(decoded)
}
