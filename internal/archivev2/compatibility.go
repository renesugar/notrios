package archivev2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

const (
	ReaderProfileCurrent         = "current-v2"
	ReaderProfilePreviousLooseV2 = "previous-loose-v2"

	physicalFormatName          = "notrios-sqlite-image"
	physicalFormatVersion       = 1
	physicalApplicationID       = "notrios"
	physicalCapabilityArchiveV2 = "archive-v2.fallback.v1"
	physicalCapabilitySQLite    = "sqlite-image+packed-assets.v1"
	physicalSemanticFallback    = "notrios-archive-v2"
)

// ReaderProfile is a frozen declaration-level reader capability set. It is
// intentionally smaller than a verifier: compatibility admission must happen
// before a reader opens index or object content.
type ReaderProfile struct {
	Name                  string   `json:"name"`
	ArchiveVersion        int      `json:"archive_version"`
	SchemaVersion         int      `json:"schema_version"`
	SupportedCapabilities []string `json:"supported_capabilities"`
	HistoricalCommit      string   `json:"historical_commit,omitempty"`
}

type CompatibilityReport struct {
	InputPath                string   `json:"input_path"`
	Format                   string   `json:"format"`
	Version                  int      `json:"version"`
	ReaderProfile            string   `json:"reader_profile"`
	ReaderSchemaVersion      int      `json:"reader_schema_version"`
	RequiredCapabilities     []string `json:"required_capabilities,omitempty"`
	OptionalCapabilities     []string `json:"optional_capabilities,omitempty"`
	Decision                 string   `json:"decision"`
	ReasonCode               string   `json:"reason_code"`
	Reason                   string   `json:"reason"`
	SemanticFallbackFormat   string   `json:"semantic_fallback_format,omitempty"`
	FullVerificationRequired bool     `json:"full_verification_required"`
}

type compatibilityEnvelope struct {
	Format        string          `json:"format"`
	Version       int             `json:"version"`
	Commit        json.RawMessage `json:"commit_sha256,omitempty"`
	Content       json.RawMessage `json:"content_sha256,omitempty"`
	Snapshot      json.RawMessage `json:"snapshot,omitempty"`
	Compatibility json.RawMessage `json:"compatibility"`
	Index         json.RawMessage `json:"index,omitempty"`
	Totals        json.RawMessage `json:"totals,omitempty"`
	Counts        json.RawMessage `json:"counts,omitempty"`
	Database      json.RawMessage `json:"database,omitempty"`
	External      json.RawMessage `json:"external,omitempty"`
	ClearedTables json.RawMessage `json:"cleared_local_tables,omitempty"`
}

type semanticCompatibility struct {
	MinimumReaderVersion int      `json:"minimum_reader_version"`
	SourceSchemaVersion  int      `json:"source_schema_version"`
	MinimumSchemaVersion int      `json:"minimum_schema_version"`
	MaximumSchemaVersion int      `json:"maximum_schema_version"`
	RequiredCapabilities []string `json:"required_capabilities"`
	OptionalCapabilities []string `json:"optional_capabilities"`
}

type physicalCompatibility struct {
	ApplicationID          string   `json:"application_id"`
	ApplicationVersion     string   `json:"application_version"`
	SourceSchemaVersion    int      `json:"source_schema_version"`
	MinimumSchemaVersion   int      `json:"minimum_schema_version"`
	MaximumSchemaVersion   int      `json:"maximum_schema_version"`
	RequiredCapabilities   []string `json:"required_capabilities"`
	SemanticFallbackFormat string   `json:"semantic_fallback_format"`
}

func ReaderProfiles() []ReaderProfile {
	base := RequiredCapabilities()
	current := append(append([]string(nil), base...), CapabilityObjectPack)
	sort.Strings(current)
	return []ReaderProfile{
		{
			Name:                  ReaderProfileCurrent,
			ArchiveVersion:        FormatVersion,
			SchemaVersion:         store.CurrentSchemaVersion,
			SupportedCapabilities: current,
		},
		{
			Name:                  ReaderProfilePreviousLooseV2,
			ArchiveVersion:        FormatVersion,
			SchemaVersion:         store.CurrentSchemaVersion,
			SupportedCapabilities: base,
			HistoricalCommit:      "5ae93df8e14880a6f83cf20014bddcb4e9079f1a",
		},
	}
}

func ReaderProfileByName(name string) (ReaderProfile, error) {
	for _, profile := range ReaderProfiles() {
		if profile.Name == name {
			return profile, nil
		}
	}
	return ReaderProfile{}, fmt.Errorf("unknown reader profile %q", name)
}

// EvaluateCompatibility performs bounded declaration-level admission only. An
// accepted report still requires VerifyDirectory before restore or consumption.
func EvaluateCompatibility(inputPath string, profile ReaderProfile, limits Limits) (CompatibilityReport, error) {
	report := CompatibilityReport{
		InputPath:                inputPath,
		ReaderProfile:            profile.Name,
		ReaderSchemaVersion:      profile.SchemaVersion,
		Decision:                 "refuse",
		ReasonCode:               "invalid_manifest",
		Reason:                   "manifest declaration could not be admitted",
		FullVerificationRequired: false,
	}
	if err := validateLimits(limits); err != nil {
		return report, err
	}
	raw, manifestPath, err := readCompatibilityManifest(inputPath, limits.MaxManifestBytes)
	if err != nil {
		return report, err
	}
	report.InputPath = manifestPath
	if err := validateJSONDepth(raw, limits.MaxJSONDepth); err != nil {
		return report, fmt.Errorf("manifest.json: %w", err)
	}
	var envelope compatibilityEnvelope
	if err := decodeStrict(raw, &envelope); err != nil {
		return report, fmt.Errorf("manifest.json: %w", err)
	}
	report.Format = envelope.Format
	report.Version = envelope.Version

	if envelope.Format == physicalFormatName {
		if envelope.Version != physicalFormatVersion {
			report.ReasonCode = "unsupported_physical_version"
			report.Reason = fmt.Sprintf("unsupported physical snapshot version %d", envelope.Version)
			return report, nil
		}
		var compatibility physicalCompatibility
		if err := decodeStrict(envelope.Compatibility, &compatibility); err != nil {
			return report, fmt.Errorf("manifest.json compatibility: %w", err)
		}
		report.RequiredCapabilities = compatibility.RequiredCapabilities
		wantCapabilities := []string{physicalCapabilityArchiveV2, physicalCapabilitySQLite}
		if compatibility.ApplicationID != physicalApplicationID || strings.TrimSpace(compatibility.ApplicationVersion) == "" ||
			compatibility.SourceSchemaVersion <= 0 || compatibility.SourceSchemaVersion != compatibility.MinimumSchemaVersion || compatibility.SourceSchemaVersion != compatibility.MaximumSchemaVersion ||
			!slices.Equal(compatibility.RequiredCapabilities, wantCapabilities) || compatibility.SemanticFallbackFormat != physicalSemanticFallback {
			report.ReasonCode = "invalid_physical_declaration"
			report.Reason = "physical snapshot compatibility declaration is invalid; do not open it as archive-v2"
			return report, nil
		}
		report.SemanticFallbackFormat = compatibility.SemanticFallbackFormat
		report.ReasonCode = "physical_snapshot_requires_semantic_fallback"
		report.Reason = fmt.Sprintf("%s/%d is a same-schema physical snapshot, not archive-v2; request %s", envelope.Format, envelope.Version, compatibility.SemanticFallbackFormat)
		return report, nil
	}
	if envelope.Format != FormatName {
		report.ReasonCode = "unsupported_format"
		report.Reason = fmt.Sprintf("unsupported format %q", envelope.Format)
		return report, nil
	}
	if envelope.Version != profile.ArchiveVersion {
		report.ReasonCode = "unsupported_archive_version"
		report.Reason = fmt.Sprintf("archive version %d is unsupported by reader version %d", envelope.Version, profile.ArchiveVersion)
		return report, nil
	}

	var compatibility semanticCompatibility
	if err := decodeStrict(envelope.Compatibility, &compatibility); err != nil {
		return report, fmt.Errorf("manifest.json compatibility: %w", err)
	}
	report.RequiredCapabilities = compatibility.RequiredCapabilities
	report.OptionalCapabilities = compatibility.OptionalCapabilities
	if err := validateCapabilityDeclaration(compatibility, limits); err != nil {
		report.ReasonCode = "invalid_capability_declaration"
		report.Reason = err.Error()
		return report, nil
	}
	if compatibility.MinimumReaderVersion != FormatVersion {
		report.ReasonCode = "unsupported_minimum_reader_version"
		report.Reason = fmt.Sprintf("archive declares unsupported minimum reader version %d", compatibility.MinimumReaderVersion)
		return report, nil
	}
	if compatibility.MinimumSchemaVersion < MinimumSchemaVersion ||
		compatibility.SourceSchemaVersion < compatibility.MinimumSchemaVersion ||
		compatibility.SourceSchemaVersion > compatibility.MaximumSchemaVersion {
		report.ReasonCode = "invalid_schema_bounds"
		report.Reason = fmt.Sprintf("invalid schema bounds source=%d range=%d..%d", compatibility.SourceSchemaVersion, compatibility.MinimumSchemaVersion, compatibility.MaximumSchemaVersion)
		return report, nil
	}
	if profile.SchemaVersion < compatibility.MinimumSchemaVersion {
		report.ReasonCode = "reader_schema_too_old"
		report.Reason = fmt.Sprintf("archive requires schema %d, profile provides %d", compatibility.MinimumSchemaVersion, profile.SchemaVersion)
		return report, nil
	}
	supported := make(map[string]bool, len(profile.SupportedCapabilities))
	for _, capability := range profile.SupportedCapabilities {
		supported[capability] = true
	}
	for _, required := range requiredBaseCapabilities {
		if !contains(compatibility.RequiredCapabilities, required) {
			report.ReasonCode = "missing_required_base_capability"
			report.Reason = fmt.Sprintf("missing required base capability %q", required)
			return report, nil
		}
	}
	for _, capability := range compatibility.RequiredCapabilities {
		if !supported[capability] {
			report.ReasonCode = "unsupported_required_capability"
			report.Reason = fmt.Sprintf("reader profile %s does not support required capability %q", profile.Name, capability)
			return report, nil
		}
	}
	report.Decision = "accept"
	report.ReasonCode = "declaration_supported"
	report.Reason = "archive-v2 declaration is supported; run full verification before consumption"
	report.FullVerificationRequired = true
	return report, nil
}

func validateCapabilityDeclaration(compatibility semanticCompatibility, limits Limits) error {
	required := compatibility.RequiredCapabilities
	optional := compatibility.OptionalCapabilities
	if len(required)+len(optional) > limits.MaxCapabilities || !sortedUnique(required) || !sortedUnique(optional) {
		return fmt.Errorf("capabilities must be sorted, unique, and bounded")
	}
	for _, capability := range append(append([]string(nil), required...), optional...) {
		if len(capability) == 0 || len(capability) > limits.MaxCapabilityNameBytes || !idPattern.MatchString(capability) {
			return fmt.Errorf("capability %q is invalid", capability)
		}
	}
	for _, capability := range required {
		if contains(optional, capability) {
			return fmt.Errorf("capability %q is both required and optional", capability)
		}
	}
	return nil
}

func readCompatibilityManifest(inputPath string, maximum int64) ([]byte, string, error) {
	path := inputPath
	info, err := os.Lstat(path)
	if err != nil {
		return nil, path, err
	}
	if info.IsDir() {
		path = filepath.Join(path, "manifest.json")
		info, err = os.Lstat(path)
		if err != nil {
			return nil, path, err
		}
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, path, fmt.Errorf("compatibility input must be a regular manifest or archive directory")
	}
	if info.Size() > maximum {
		return nil, path, fmt.Errorf("manifest exceeds %d bytes", maximum)
	}
	raw, err := readRegularBounded(filepath.Dir(path), filepath.Base(path), maximum)
	if err != nil {
		return nil, path, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, path, fmt.Errorf("manifest is empty")
	}
	return raw, path, nil
}
