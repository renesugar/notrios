package harness

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func validResult() Result {
	return Result{
		CacheState: "interleaved-first", Generated: true,
		Assertions: Assertions{SourceUnchanged: true, ArithmeticValid: true},
	}
}

func TestEveryAdapterMapsEveryBoundary(t *testing.T) {
	adapters := Adapters()
	if len(adapters) != 11 {
		t.Fatalf("adapters=%d", len(adapters))
	}
	if err := ValidateAdapters(adapters); err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedPhaseResumesWithoutOverwritingCompletedResult(t *testing.T) {
	runner := Runner{Workspace: t.TempDir(), Tier: 10_000, Adapter: "archive-v2-loose", Phase: StageSnapshotVerify}
	interrupted := errors.New("intentional interruption")
	if _, _, err := runner.Run(func() (Result, error) { return Result{}, interrupted }); !errors.Is(err, interrupted) {
		t.Fatalf("first run error=%v", err)
	}
	checkpoint, err := loadCheckpoint(runner.CheckpointPath())
	if err != nil || checkpoint.Status != "started" || checkpoint.Attempt != 1 {
		t.Fatalf("interrupted checkpoint=%+v err=%v", checkpoint, err)
	}
	completed, resumed, err := runner.Run(func() (Result, error) { return validResult(), nil })
	if err != nil || resumed || completed.Status != "completed" {
		t.Fatalf("completion result=%+v resumed=%t err=%v", completed, resumed, err)
	}
	before, err := os.ReadFile(runner.ResultPath())
	if err != nil {
		t.Fatal(err)
	}
	invoked := false
	cached, resumed, err := runner.Run(func() (Result, error) { invoked = true; return Result{}, nil })
	if err != nil || !resumed || invoked || cached.Status != "completed" {
		t.Fatalf("resume result=%+v resumed=%t invoked=%t err=%v", cached, resumed, invoked, err)
	}
	after, err := os.ReadFile(runner.ResultPath())
	if err != nil || string(after) != string(before) {
		t.Fatalf("immutable result changed: err=%v", err)
	}
}

func TestAtomicPublicationNeverLeavesInvalidResult(t *testing.T) {
	runner := Runner{Workspace: t.TempDir(), Tier: 10_000, Adapter: "archive-v2-pack", Phase: StageSnapshotCreate}
	invalid := validResult()
	invalid.Assertions.ArithmeticValid = false
	if _, _, err := runner.Run(func() (Result, error) { return invalid, nil }); err == nil {
		t.Fatal("invalid result was accepted")
	}
	if _, err := os.Stat(runner.ResultPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid result was published: %v", err)
	}
}

func TestAggregatePrivacyScannerRejectsDetails(t *testing.T) {
	for _, value := range []any{
		map[string]any{"title": "private"},
		map[string]any{"detail": "/home/person/private"},
		map[string]any{"filename": "note.md"},
	} {
		if err := ScanAggregatePrivacy(value); err == nil {
			t.Fatalf("accepted private value %#v", value)
		}
	}
	if err := ScanAggregatePrivacy(validResult()); err != nil {
		t.Fatalf("aggregate result rejected: %v", err)
	}
}

func TestInventoryArithmeticAndReadOnlyComparison(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "filled"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "filled", "one.bin"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := InspectTree(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := InspectTree(root)
	if err != nil || !SameInventory(before, after) {
		t.Fatalf("unchanged tree differed: err=%v before=%+v after=%+v", err, before, after)
	}
	if before.Directories != 3 || before.DirectoryHistogram.Total() != 3 || before.Files != 1 {
		t.Fatalf("inventory arithmetic=%+v", before)
	}
	if err := os.WriteFile(filepath.Join(root, "filled", "two.bin"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := InspectTree(root)
	if err != nil || SameInventory(before, changed) {
		t.Fatalf("mutation was not observed: err=%v", err)
	}
}

func TestCompressionRatioArithmetic(t *testing.T) {
	result := validResult()
	result.Schema, result.Tier, result.Adapter, result.Phase, result.Status = ResultSchema, 10_000, "catchup-packed", StageTransportSeal, "completed"
	result.Metrics.InputBytes, result.Metrics.OutputBytes, result.Metrics.CompressionRatio = 100, 50, 1
	if err := ValidateResult(result); err == nil {
		t.Fatal("invalid compression arithmetic was accepted")
	}
	result.Metrics.CompressionRatio = 2
	if err := ValidateResult(result); err != nil {
		t.Fatal(err)
	}
}

func TestNegativeInventoryAndContainerMetricsAreRejected(t *testing.T) {
	result := validResult()
	result.Schema, result.Tier, result.Adapter, result.Phase, result.Status = ResultSchema, 10_000, "catchup-loose", StageTransportPrepare, "completed"
	result.Output.Files = -1
	if err := ValidateResult(result); err == nil {
		t.Fatal("negative inventory was accepted")
	}
	result.Output.Files = 0
	result.Metrics.ContainerEntries = -1
	if err := ValidateResult(result); err == nil {
		t.Fatal("negative container entry count was accepted")
	}
}
