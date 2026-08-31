package main

import (
	"path/filepath"
	"testing"
)

func TestCompatibilityCommandEmitsMachineReadableAdmissionAndRefusal(t *testing.T) {
	binary := buildCLI(t)
	fixtures := filepath.Join("..", "..", "contracts", "archive-v2", "fixtures")

	accepted := runCLI(t, binary, "compatibility", "archive-v2", filepath.Join(fixtures, "loose-schema12"))
	if accepted.exitCode != 0 {
		t.Fatalf("accepted fixture exited %d: %s", accepted.exitCode, accepted.stderr)
	}
	acceptedJSON := decodeCLIJSON(t, accepted.stdout)
	if acceptedJSON["decision"] != "accept" || acceptedJSON["reason_code"] != "declaration_supported" || acceptedJSON["full_verification_required"] != true {
		t.Fatalf("accepted report = %#v", acceptedJSON)
	}

	packed := runCLI(t, binary, "compatibility", "archive-v2", "--reader", "previous-loose-v2", filepath.Join(fixtures, "packed-schema12"))
	if packed.exitCode != 1 {
		t.Fatalf("packed refusal exited %d: %s", packed.exitCode, packed.stderr)
	}
	packedJSON := decodeCLIJSON(t, packed.stdout)
	if packedJSON["decision"] != "refuse" || packedJSON["reason_code"] != "unsupported_required_capability" {
		t.Fatalf("packed report = %#v", packedJSON)
	}

	physical := runCLI(t, binary, "compatibility", "archive-v2", filepath.Join(fixtures, "physical-refusal", "manifest.json"))
	if physical.exitCode != 1 {
		t.Fatalf("physical refusal exited %d: %s", physical.exitCode, physical.stderr)
	}
	physicalJSON := decodeCLIJSON(t, physical.stdout)
	if physicalJSON["reason_code"] != "physical_snapshot_requires_semantic_fallback" || physicalJSON["semantic_fallback_format"] != "notrios-archive-v2" || physicalJSON["full_verification_required"] != false {
		t.Fatalf("physical report = %#v", physicalJSON)
	}

	badProfile := runCLI(t, binary, "compatibility", "archive-v2", "--reader", "invented", filepath.Join(fixtures, "loose-schema12"))
	if badProfile.exitCode != 2 {
		t.Fatalf("bad reader profile exited %d: %s", badProfile.exitCode, badProfile.stderr)
	}
}
