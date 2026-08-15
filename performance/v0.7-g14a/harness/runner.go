package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrResultExists = errors.New("phase result already exists")

type Runner struct {
	Workspace string
	Tier      int
	Adapter   string
	Phase     Stage
}

func (r Runner) ResultPath() string {
	return filepath.Join(r.Workspace, "results", fmt.Sprintf("generated-%d", r.Tier), r.Adapter, string(r.Phase)+".json")
}

func (r Runner) CheckpointPath() string {
	return filepath.Join(r.Workspace, "checkpoints", fmt.Sprintf("generated-%d", r.Tier), r.Adapter, string(r.Phase)+".json")
}

// Run publishes a separately valid immutable result. If it already exists,
// Run returns it without invoking work; that is phase-level resume.
func (r Runner) Run(work func() (Result, error)) (Result, bool, error) {
	if err := ValidateIdentity(r.Tier, r.Adapter, r.Phase); err != nil {
		return Result{}, false, err
	}
	if result, err := loadResult(r.ResultPath()); err == nil {
		return result, true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, false, err
	}
	attempt := 1
	if checkpoint, err := loadCheckpoint(r.CheckpointPath()); err == nil {
		attempt = checkpoint.Attempt + 1
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, false, err
	}
	checkpoint := Checkpoint{Schema: CheckpointSchema, Tier: r.Tier, Adapter: r.Adapter, Phase: r.Phase, Status: "started", Attempt: attempt}
	if err := writeAtomicJSON(r.CheckpointPath(), checkpoint, true); err != nil {
		return Result{}, false, err
	}
	result, err := work()
	if err != nil {
		return Result{}, false, err
	}
	result.Schema, result.Tier, result.Adapter, result.Phase, result.Status = ResultSchema, r.Tier, r.Adapter, r.Phase, "completed"
	if err := ValidateResult(result); err != nil {
		return Result{}, false, err
	}
	if err := writeAtomicJSON(r.ResultPath(), result, false); err != nil {
		return Result{}, false, err
	}
	checkpoint.Status = "completed"
	if err := writeAtomicJSON(r.CheckpointPath(), checkpoint, true); err != nil {
		return Result{}, false, err
	}
	return result, false, nil
}

func loadResult(path string) (Result, error) {
	var result Result
	if err := readJSON(path, &result); err != nil {
		return Result{}, err
	}
	if err := ValidateResult(result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func loadCheckpoint(path string) (Checkpoint, error) {
	var checkpoint Checkpoint
	if err := readJSON(path, &checkpoint); err != nil {
		return Checkpoint{}, err
	}
	if checkpoint.Schema != CheckpointSchema || checkpoint.Attempt < 1 || (checkpoint.Status != "started" && checkpoint.Status != "completed") {
		return Checkpoint{}, fmt.Errorf("invalid checkpoint")
	}
	return checkpoint, nil
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeAtomicJSON(path string, value any, overwrite bool) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return ErrResultExists
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	temporary, err := os.CreateTemp(directory, ".g14a-result-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if !overwrite {
		// A hard link gives atomic no-replace publication on the ext4 benchmark
		// workspace. Google Drive is a copy target, never the result store.
		if err := os.Link(temporaryPath, path); err != nil {
			if errors.Is(err, os.ErrExist) {
				return ErrResultExists
			}
			return err
		}
	} else if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	err = dir.Sync()
	closeErr := dir.Close()
	if err != nil {
		return err
	}
	return closeErr
}
