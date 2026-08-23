package snapshotimage

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

const restorePlanVersion = 1

var ErrRestoreRecoverable = errors.New("physical snapshot restore stopped at a recoverable boundary")

type RestoreOptions struct {
	Intent             string
	TargetDatabase     string
	TargetAssetRoot    string
	EmergencyDirectory string
	// AfterStage is a fault-injection seam. Production callers leave it nil.
	AfterStage func(string) error
}

type RestoreReport struct {
	RestoreID         string `json:"restore_id"`
	SnapshotID        string `json:"snapshot_id"`
	EmergencySnapshot string `json:"emergency_snapshot"`
	DatabaseID        string `json:"database_id"`
	OldReplicaID      string `json:"old_replica_id"`
	NewReplicaID      string `json:"new_replica_id"`
	DerivedDocuments  int64  `json:"derived_documents"`
	Stage             string `json:"stage"`
	Resumed           bool   `json:"resumed"`
}

type restorePlan struct {
	Version           int    `json:"version"`
	RestoreID         string `json:"restore_id"`
	Stage             string `json:"stage"`
	Intent            string `json:"intent"`
	SnapshotRoot      string `json:"snapshot_root"`
	SnapshotID        string `json:"snapshot_id"`
	SnapshotCommit    string `json:"snapshot_commit_sha256"`
	DatabaseID        string `json:"database_id"`
	SourceReplicaID   string `json:"source_replica_id"`
	OldReplicaID      string `json:"old_replica_id"`
	NewReplicaID      string `json:"new_replica_id,omitempty"`
	TargetDatabase    string `json:"target_database"`
	TargetAssets      string `json:"target_assets"`
	EmergencySnapshot string `json:"emergency_snapshot"`
	StagedDatabase    string `json:"staged_database"`
	StagedAssets      string `json:"staged_assets"`
	PreviousDatabase  string `json:"previous_database"`
	PreviousAssets    string `json:"previous_assets"`
	DerivedDocuments  int64  `json:"derived_documents"`
	UpdatedAt         string `json:"updated_at"`
}

// Restore performs an explicit same-schema physical replace/adopt. It is a
// local stopped-service operation; no path here can come from REST or MCP.
// Every destructive step follows a verified emergency snapshot and is
// roll-forward resumable through the adjacent plan and startup-blocker files.
func Restore(ctx context.Context, snapshotRoot string, options RestoreOptions) (RestoreReport, error) {
	ctx = contextOrBackground(ctx)
	if options.Intent != "replace" && options.Intent != "adopt" {
		return RestoreReport{}, fmt.Errorf("physical restore intent must be replace or adopt")
	}
	report, err := VerifyDirectory(ctx, snapshotRoot, DefaultLimits())
	if err != nil {
		return RestoreReport{}, err
	}
	manifest, err := ReadManifest(snapshotRoot, DefaultLimits())
	if err != nil {
		return RestoreReport{}, err
	}
	targetDB, err := filepath.Abs(strings.TrimSpace(options.TargetDatabase))
	if err != nil || strings.TrimSpace(options.TargetDatabase) == "" || targetDB == ":memory:" {
		return RestoreReport{}, fmt.Errorf("a real target database path is required")
	}
	targetAssets, err := filepath.Abs(strings.TrimSpace(options.TargetAssetRoot))
	if err != nil || strings.TrimSpace(options.TargetAssetRoot) == "" {
		return RestoreReport{}, fmt.Errorf("a target asset root is required")
	}
	snapshotRoot, _ = filepath.Abs(snapshotRoot)
	planPath := targetDB + ".notrios-restore-plan.json"
	plan, resumed, err := loadOrCreateRestorePlan(ctx, planPath, snapshotRoot, targetDB, targetAssets, report, options)
	if err != nil {
		return RestoreReport{}, err
	}
	if plan.SnapshotCommit != report.CommitSHA256 || plan.SnapshotID != report.SnapshotID || plan.Intent != options.Intent || plan.TargetDatabase != targetDB || plan.TargetAssets != targetAssets {
		return RestoreReport{}, fmt.Errorf("%w: the existing restore plan names different input or targets", ErrRestoreRecoverable)
	}

	advance := func(stage string) error {
		plan.Stage = stage
		plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := writeRestorePlan(planPath, plan); err != nil {
			return err
		}
		if options.AfterStage != nil {
			if err := options.AfterStage(stage); err != nil {
				return fmt.Errorf("%w at %s: %v", ErrRestoreRecoverable, stage, err)
			}
		}
		return nil
	}

	if plan.Stage == "initialized" {
		live, err := store.OpenSQLiteWithAssetStore(targetDB, targetAssets)
		if err != nil {
			return RestoreReport{}, err
		}
		_, createErr := Create(ctx, live, targetAssets, plan.EmergencySnapshot, CreateOptions{})
		closeErr := live.Close()
		if createErr != nil {
			return RestoreReport{}, fmt.Errorf("create emergency snapshot: %w", createErr)
		}
		if closeErr != nil {
			return RestoreReport{}, closeErr
		}
		if _, err := VerifyDirectory(ctx, plan.EmergencySnapshot, DefaultLimits()); err != nil {
			return RestoreReport{}, fmt.Errorf("verify emergency snapshot: %w", err)
		}
		if err := advance("emergency_verified"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "emergency_verified" {
		if err := restartDirectory(plan.StagedAssets); err != nil {
			return RestoreReport{}, err
		}
		if err := extractAssetPacks(ctx, snapshotRoot, plan.StagedAssets, manifest); err != nil {
			return RestoreReport{}, fmt.Errorf("stage snapshot assets: %w", err)
		}
		if err := advance("assets_staged"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "assets_staged" {
		_ = os.Remove(plan.StagedDatabase)
		if err := copyRegularFile(filepath.Join(snapshotRoot, DatabaseFile), plan.StagedDatabase, 0o600); err != nil {
			return RestoreReport{}, fmt.Errorf("stage snapshot database: %w", err)
		}
		staged, err := store.OpenSQLiteWithAssetStore(plan.StagedDatabase, plan.StagedAssets)
		if err != nil {
			return RestoreReport{}, err
		}
		activation, activateErr := staged.ActivatePhysicalSnapshot(ctx, report.SnapshotID, report.SourceReplicaID, report.SnapshotVector, report.SnapshotFloors)
		closeErr := staged.Close()
		if activateErr != nil {
			return RestoreReport{}, fmt.Errorf("activate staged snapshot: %w", activateErr)
		}
		if closeErr != nil {
			return RestoreReport{}, closeErr
		}
		plan.NewReplicaID = activation.NewReplicaID
		plan.DerivedDocuments = activation.DerivedDocuments
		if err := advance("prepared"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "prepared" {
		if err := writeBlocker(store.PhysicalRestoreMarkerPath(targetDB), plan); err != nil {
			return RestoreReport{}, err
		}
		if err := advance("cutover_blocked"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "cutover_blocked" {
		if err := moveIfPresent(targetAssets, plan.PreviousAssets); err != nil {
			return RestoreReport{}, fmt.Errorf("preserve previous asset tree: %w", err)
		}
		if err := advance("previous_assets_moved"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "previous_assets_moved" {
		if err := renameIfNeeded(plan.StagedAssets, targetAssets); err != nil {
			return RestoreReport{}, fmt.Errorf("install snapshot assets: %w", err)
		}
		if err := advance("assets_installed"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "assets_installed" {
		if err := moveDatabaseFamily(targetDB, plan.PreviousDatabase); err != nil {
			return RestoreReport{}, fmt.Errorf("preserve previous database: %w", err)
		}
		if err := advance("previous_database_moved"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "previous_database_moved" {
		if err := renameIfNeeded(plan.StagedDatabase, targetDB); err != nil {
			return RestoreReport{}, fmt.Errorf("install snapshot database: %w", err)
		}
		if err := advance("installed"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "installed" {
		installed, err := store.OpenSQLiteWithAssetStoreForRestore(targetDB, targetAssets)
		if err != nil {
			return RestoreReport{}, fmt.Errorf("open installed snapshot: %w", err)
		}
		identity, identityErr := installed.GetDatabaseIdentity(ctx)
		closeErr := installed.Close()
		if identityErr != nil || identity.DatabaseID != report.DatabaseID || identity.ReplicaID != plan.NewReplicaID {
			return RestoreReport{}, fmt.Errorf("%w: installed snapshot identity is not the prepared identity", ErrRestoreRecoverable)
		}
		if closeErr != nil {
			return RestoreReport{}, closeErr
		}
		if err := advance("verified_installed"); err != nil {
			return RestoreReport{}, err
		}
	}
	if plan.Stage == "verified_installed" {
		if err := os.Remove(store.PhysicalRestoreMarkerPath(targetDB)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return RestoreReport{}, err
		}
		if err := syncParent(targetDB); err != nil {
			return RestoreReport{}, err
		}
		if err := advance("complete"); err != nil {
			return RestoreReport{}, err
		}
		// The verified emergency snapshot is the recovery copy; the raw previous
		// paths are redundant and may contain machine-local state.
		_ = os.Remove(plan.PreviousDatabase)
		_ = os.Remove(plan.PreviousDatabase + "-wal")
		_ = os.Remove(plan.PreviousDatabase + "-shm")
		_ = os.RemoveAll(plan.PreviousAssets)
	}
	if plan.Stage != "complete" {
		return RestoreReport{}, fmt.Errorf("%w at %s", ErrRestoreRecoverable, plan.Stage)
	}
	result := RestoreReport{
		RestoreID: plan.RestoreID, SnapshotID: plan.SnapshotID,
		EmergencySnapshot: plan.EmergencySnapshot, DatabaseID: plan.DatabaseID,
		OldReplicaID: plan.OldReplicaID, NewReplicaID: plan.NewReplicaID,
		DerivedDocuments: plan.DerivedDocuments, Stage: plan.Stage, Resumed: resumed,
	}
	// Keep the completed recovery record for diagnosis, but free the canonical
	// plan path so a later explicitly requested restore can start independently.
	completedPlan := targetDB + ".notrios-restore-" + plan.RestoreID + ".completed.json"
	if err := os.Rename(planPath, completedPlan); err != nil && !errors.Is(err, os.ErrNotExist) {
		return RestoreReport{}, fmt.Errorf("archive completed restore plan: %w", err)
	}
	if err := syncParent(targetDB); err != nil {
		return RestoreReport{}, err
	}
	return result, nil
}

func loadOrCreateRestorePlan(ctx context.Context, planPath, snapshotRoot, targetDB, targetAssets string, report VerificationReport, options RestoreOptions) (restorePlan, bool, error) {
	if raw, err := os.ReadFile(planPath); err == nil {
		var plan restorePlan
		if err := json.Unmarshal(raw, &plan); err != nil || plan.Version != restorePlanVersion {
			return restorePlan{}, true, fmt.Errorf("%w: restore plan is malformed", ErrRestoreRecoverable)
		}
		return plan, true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return restorePlan{}, false, err
	}
	live, err := store.OpenSQLiteWithAssetStore(targetDB, targetAssets)
	if err != nil {
		return restorePlan{}, false, err
	}
	identity, identityErr := live.GetDatabaseIdentity(ctx)
	closeErr := live.Close()
	if identityErr != nil {
		return restorePlan{}, false, identityErr
	}
	if closeErr != nil {
		return restorePlan{}, false, closeErr
	}
	if options.Intent == "replace" && identity.DatabaseID != report.DatabaseID {
		return restorePlan{}, false, fmt.Errorf("replace requires the same database id; use semantic archive-v2 or explicit adopt")
	}
	id, err := store.NewID("restore")
	if err != nil {
		return restorePlan{}, false, err
	}
	emergency := strings.TrimSpace(options.EmergencyDirectory)
	if emergency == "" {
		emergency = filepath.Join(filepath.Dir(targetDB), "notrios-emergency", report.SnapshotID+"-"+id)
	}
	emergency, err = filepath.Abs(emergency)
	if err != nil {
		return restorePlan{}, false, err
	}
	dbBase := filepath.Base(targetDB)
	assetBase := filepath.Base(targetAssets)
	plan := restorePlan{
		Version: restorePlanVersion, RestoreID: id, Stage: "initialized", Intent: options.Intent,
		SnapshotRoot: snapshotRoot, SnapshotID: report.SnapshotID, SnapshotCommit: report.CommitSHA256,
		DatabaseID: report.DatabaseID, SourceReplicaID: report.SourceReplicaID, OldReplicaID: identity.ReplicaID,
		TargetDatabase: targetDB, TargetAssets: targetAssets, EmergencySnapshot: emergency,
		StagedDatabase:   filepath.Join(filepath.Dir(targetDB), "."+dbBase+"."+id+".staged"),
		PreviousDatabase: filepath.Join(filepath.Dir(targetDB), "."+dbBase+"."+id+".previous"),
		StagedAssets:     filepath.Join(filepath.Dir(targetAssets), "."+assetBase+"."+id+".staged"),
		PreviousAssets:   filepath.Join(filepath.Dir(targetAssets), "."+assetBase+"."+id+".previous"),
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	return plan, false, writeRestorePlan(planPath, plan)
}

func extractAssetPacks(ctx context.Context, snapshotRoot, destination string, manifest Manifest) error {
	seen := map[string]bool{}
	for _, descriptor := range manifest.External.Packs {
		file, err := os.Open(filepath.Join(snapshotRoot, filepath.FromSlash(descriptor.Path)))
		if err != nil {
			return err
		}
		reader := tar.NewReader(file)
		for {
			if err := ctx.Err(); err != nil {
				file.Close()
				return err
			}
			header, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil || header.Typeflag != tar.TypeReg || !validStoragePath(header.Name, DefaultLimits()) || seen[header.Name] {
				file.Close()
				return fmt.Errorf("pack %s contains an invalid or duplicate entry", descriptor.Path)
			}
			seen[header.Name] = true
			target, err := safeRestoreJoin(destination, header.Name)
			if err != nil {
				file.Close()
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				file.Close()
				return err
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				file.Close()
				return err
			}
			digest := sha256.New()
			written, copyErr := io.Copy(io.MultiWriter(output, digest), reader)
			closeErr := output.Close()
			wantHash := filepath.Base(header.Name)
			if copyErr != nil || closeErr != nil || written != header.Size || hex.EncodeToString(digest.Sum(nil)) != wantHash {
				file.Close()
				return fmt.Errorf("extracted object %s failed its hash or length", header.Name)
			}
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return syncDirectory(destination)
}

func safeRestoreJoin(root, relative string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(relative))
	back, err := filepath.Rel(root, target)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("unsafe snapshot asset path %q", relative)
	}
	return target, nil
}

func restartDirectory(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.MkdirAll(path, 0o700)
}

func copyRegularFile(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyBuffer(output, input, make([]byte, 64<<10))
	if copyErr == nil {
		copyErr = output.Sync()
	}
	if closeErr := output.Close(); copyErr == nil {
		copyErr = closeErr
	}
	return copyErr
}

func moveIfPresent(source, target string) error {
	if _, err := os.Lstat(target); err == nil {
		return nil
	}
	if _, err := os.Lstat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Rename(source, target)
}

func renameIfNeeded(source, target string) error {
	if _, err := os.Lstat(source); errors.Is(err, os.ErrNotExist) {
		if _, targetErr := os.Lstat(target); targetErr == nil {
			return nil
		}
		return err
	}
	return os.Rename(source, target)
}

func moveDatabaseFamily(source, target string) error {
	if err := moveIfPresent(source, target); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := moveIfPresent(source+suffix, target+suffix); err != nil {
			return err
		}
	}
	return nil
}

func writeRestorePlan(path string, plan restorePlan) error {
	return writeJSONAtomic(path, plan, 0o600)
}

func writeBlocker(path string, plan restorePlan) error {
	return writeJSONAtomic(path, map[string]string{
		"restore_id": plan.RestoreID, "stage": plan.Stage,
		"plan": plan.TargetDatabase + ".notrios-restore-plan.json",
	}, 0o600)
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".restore-state-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncParent(path)
}

func syncParent(path string) error {
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
