package archivev2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCompatibilityProfilesAdmitLooseAndBoundPacked(t *testing.T) {
	golden := filepath.Join("testdata", "golden-minimal")
	current, err := ReaderProfileByName(ReaderProfileCurrent)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := ReaderProfileByName(ReaderProfilePreviousLooseV2)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []ReaderProfile{current, previous} {
		report, err := EvaluateCompatibility(golden, profile, DefaultLimits())
		if err != nil {
			t.Fatalf("%s: %v", profile.Name, err)
		}
		if report.Decision != "accept" || !report.FullVerificationRequired {
			t.Fatalf("%s report = %+v", profile.Name, report)
		}
	}

	packed := copyCompatibilityManifest(t, golden, func(manifest map[string]any) {
		compatibility := manifest["compatibility"].(map[string]any)
		required := stringSlice(t, compatibility["required_capabilities"])
		required = append(required, CapabilityObjectPack)
		sort.Strings(required)
		compatibility["required_capabilities"] = required
	})
	if report, err := EvaluateCompatibility(packed, previous, DefaultLimits()); err != nil {
		t.Fatal(err)
	} else if report.Decision != "refuse" || report.ReasonCode != "unsupported_required_capability" || !strings.Contains(report.Reason, CapabilityObjectPack) {
		t.Fatalf("previous packed report = %+v", report)
	}
	if report, err := EvaluateCompatibility(packed, current, DefaultLimits()); err != nil {
		t.Fatal(err)
	} else if report.Decision != "accept" {
		t.Fatalf("current packed report = %+v", report)
	}
}

func TestCompatibilityUnknownCapabilitiesAreClosedAndExplicit(t *testing.T) {
	golden := filepath.Join("testdata", "golden-minimal")
	current, _ := ReaderProfileByName(ReaderProfileCurrent)

	unknownRequired := copyCompatibilityManifest(t, golden, func(manifest map[string]any) {
		compatibility := manifest["compatibility"].(map[string]any)
		required := append(stringSlice(t, compatibility["required_capabilities"]), "objects.future.v1")
		sort.Strings(required)
		compatibility["required_capabilities"] = required
	})
	report, err := EvaluateCompatibility(unknownRequired, current, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "refuse" || report.ReasonCode != "unsupported_required_capability" {
		t.Fatalf("unknown required report = %+v", report)
	}

	unknownOptional := copyCompatibilityManifest(t, golden, func(manifest map[string]any) {
		manifest["compatibility"].(map[string]any)["optional_capabilities"] = []string{"consumer.future.v1"}
	})
	report, err = EvaluateCompatibility(unknownOptional, current, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "accept" || len(report.OptionalCapabilities) != 1 {
		t.Fatalf("unknown optional report = %+v", report)
	}
}

func TestCompatibilityClassifiesPhysicalWithoutOpeningPayload(t *testing.T) {
	root := t.TempDir()
	manifest := `{
  "format": "notrios-sqlite-image",
  "version": 1,
  "compatibility": {
    "application_id": "notrios",
    "application_version": "0.7.0-dev",
    "source_schema_version": 27,
    "minimum_schema_version": 27,
    "maximum_schema_version": 27,
    "required_capabilities": ["archive-v2.fallback.v1", "sqlite-image+packed-assets.v1"],
    "semantic_fallback_format": "notrios-archive-v2"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, _ := ReaderProfileByName(ReaderProfileCurrent)
	report, err := EvaluateCompatibility(root, profile, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "refuse" || report.ReasonCode != "physical_snapshot_requires_semantic_fallback" || report.SemanticFallbackFormat != "notrios-archive-v2" || report.FullVerificationRequired {
		t.Fatalf("physical report = %+v", report)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(manifest), &decoded); err != nil {
		t.Fatal(err)
	}
	decoded["compatibility"].(map[string]any)["semantic_fallback_format"] = "vendor-archive"
	raw, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err = EvaluateCompatibility(root, profile, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if report.ReasonCode != "invalid_physical_declaration" || report.SemanticFallbackFormat != "" {
		t.Fatalf("untrusted physical fallback report = %+v", report)
	}
}

func TestPublishedCurrentPreviousReaderMatrix(t *testing.T) {
	fixtures := filepath.Join("..", "..", "contracts", "archive-v2", "fixtures")
	current, _ := ReaderProfileByName(ReaderProfileCurrent)
	previous, _ := ReaderProfileByName(ReaderProfilePreviousLooseV2)
	tests := []struct {
		fixture        string
		profile        ReaderProfile
		decision       string
		reasonCode     string
		fullVerify     bool
		fallbackFormat string
	}{
		{"loose-schema12", current, "accept", "declaration_supported", true, ""},
		{"loose-schema12", previous, "accept", "declaration_supported", true, ""},
		{"packed-schema12", current, "accept", "declaration_supported", true, ""},
		{"packed-schema12", previous, "refuse", "unsupported_required_capability", false, ""},
		{"sync-era-schema27-packed", current, "accept", "declaration_supported", true, ""},
		{"sync-era-schema27-packed", previous, "refuse", "unsupported_required_capability", false, ""},
		{"unknown-required-capability", current, "refuse", "unsupported_required_capability", false, ""},
		{"unknown-required-capability", previous, "refuse", "unsupported_required_capability", false, ""},
		{"unknown-optional-capability", current, "accept", "declaration_supported", true, ""},
		{"unknown-optional-capability", previous, "accept", "declaration_supported", true, ""},
		{"physical-refusal", current, "refuse", "physical_snapshot_requires_semantic_fallback", false, "notrios-archive-v2"},
		{"physical-refusal", previous, "refuse", "physical_snapshot_requires_semantic_fallback", false, "notrios-archive-v2"},
	}
	for _, test := range tests {
		t.Run(test.fixture+"/"+test.profile.Name, func(t *testing.T) {
			report, err := EvaluateCompatibility(filepath.Join(fixtures, test.fixture), test.profile, DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			if report.Decision != test.decision || report.ReasonCode != test.reasonCode || report.FullVerificationRequired != test.fullVerify || report.SemanticFallbackFormat != test.fallbackFormat {
				t.Fatalf("report = %+v", report)
			}
		})
	}
}

func TestCompatibilityRejectsInvalidBoundsAndUnsafeManifest(t *testing.T) {
	golden := filepath.Join("testdata", "golden-minimal")
	current, _ := ReaderProfileByName(ReaderProfileCurrent)
	if _, err := EvaluateCompatibility(golden, current, Limits{}); err == nil || !strings.Contains(err.Error(), "archive limits") {
		t.Fatalf("invalid limits error = %v", err)
	}

	badReader := copyCompatibilityManifest(t, golden, func(manifest map[string]any) {
		manifest["compatibility"].(map[string]any)["minimum_reader_version"] = float64(1)
	})
	if report, err := EvaluateCompatibility(badReader, current, DefaultLimits()); err != nil {
		t.Fatal(err)
	} else if report.Decision != "refuse" || report.ReasonCode != "unsupported_minimum_reader_version" {
		t.Fatalf("minimum reader report = %+v", report)
	}

	root := t.TempDir()
	if err := os.Symlink(filepath.Join("..", "..", golden, "manifest.json"), filepath.Join(root, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := EvaluateCompatibility(root, current, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "regular manifest") {
		t.Fatalf("symlink manifest error = %v", err)
	}
}

func copyCompatibilityManifest(t *testing.T, source string, mutate func(map[string]any)) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(source, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	mutate(manifest)
	raw, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func stringSlice(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("not a JSON array: %T", value)
	}
	result := make([]string, len(items))
	for index, item := range items {
		result[index], ok = item.(string)
		if !ok {
			t.Fatalf("not a string at %d: %T", index, item)
		}
	}
	return result
}
