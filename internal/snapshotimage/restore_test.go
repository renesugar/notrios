package snapshotimage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func TestPhysicalRestoreCreatesEmergencyRotatesAndRebuilds(t *testing.T) {
	ctx := context.Background()
	snapshot, _ := createTestSnapshot(t)
	verified, err := VerifyDirectory(ctx, snapshot, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	db := filepath.Join(root, "notes.sqlite")
	assets := filepath.Join(root, "assets")
	target, err := store.OpenSQLiteWithAssetStore(db, assets)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	original, err := target.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "before_restore", Title: "before", Body: "emergency only\n"})
	if err != nil {
		t.Fatal(err)
	}
	oldIdentity, err := target.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}

	emergency := filepath.Join(root, "emergency")
	report, err := Restore(ctx, snapshot, RestoreOptions{
		Intent: "adopt", TargetDatabase: db, TargetAssetRoot: assets, EmergencyDirectory: emergency,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Stage != "complete" || report.NewReplicaID == "" || report.NewReplicaID == oldIdentity.ReplicaID || report.NewReplicaID == verified.SourceReplicaID {
		t.Fatalf("restore identities/stage: %+v", report)
	}
	if _, err := os.Stat(db + ".notrios-restore-plan.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active restore plan still exists after completion: %v", err)
	}
	completedPlans, err := filepath.Glob(db + ".notrios-restore-*.completed.json")
	if err != nil || len(completedPlans) != 1 {
		t.Fatalf("completed restore records = %v, err = %v", completedPlans, err)
	}
	if _, err := VerifyDirectory(ctx, emergency, DefaultLimits()); err != nil {
		t.Fatalf("emergency snapshot: %v", err)
	}
	emergencyImage, err := store.OpenSQLiteSnapshotReadOnly(filepath.Join(emergency, DatabaseFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := emergencyImage.GetDocument(ctx, original.ID); err != nil {
		t.Fatalf("emergency snapshot lost the previous library: %v", err)
	}
	emergencyImage.Close()

	installed, err := store.OpenSQLiteWithAssetStore(db, assets)
	if err != nil {
		t.Fatal(err)
	}
	defer installed.Close()
	identity, err := installed.GetDatabaseIdentity(ctx)
	if err != nil || identity.DatabaseID != verified.DatabaseID || identity.ReplicaID != report.NewReplicaID {
		t.Fatalf("installed identity: %+v %v", identity, err)
	}
	if _, err := installed.GetDocument(ctx, "before_restore"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("replace retained the target-only note: %v", err)
	}
	status, err := installed.JournalStatus(ctx)
	if err != nil || !status.Enabled || status.ReplicaID != report.NewReplicaID {
		t.Fatalf("installed journal: %+v %v", status, err)
	}
}

func TestPhysicalRestoreResumesEveryCutoverBoundary(t *testing.T) {
	ctx := context.Background()
	snapshot, _ := createTestSnapshot(t)
	for _, fault := range []string{
		"emergency_verified", "assets_staged", "prepared", "cutover_blocked",
		"previous_assets_moved", "assets_installed", "previous_database_moved",
		"installed", "verified_installed",
	} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			db := filepath.Join(root, "notes.sqlite")
			assets := filepath.Join(root, "assets")
			target, err := store.OpenSQLiteWithAssetStore(db, assets)
			if err != nil {
				t.Fatal(err)
			}
			if err := target.Bootstrap(ctx); err != nil {
				t.Fatal(err)
			}
			target.Close()
			injected := errors.New("injected cutover fault")
			_, err = Restore(ctx, snapshot, RestoreOptions{
				Intent: "adopt", TargetDatabase: db, TargetAssetRoot: assets,
				EmergencyDirectory: filepath.Join(root, "emergency"),
				AfterStage: func(stage string) error {
					if stage == fault {
						return injected
					}
					return nil
				},
			})
			if err == nil || !errors.Is(err, ErrRestoreRecoverable) {
				t.Fatalf("fault %s was not recoverable: %v", fault, err)
			}
			if fault == "cutover_blocked" || fault == "previous_assets_moved" || fault == "assets_installed" || fault == "previous_database_moved" || fault == "installed" || fault == "verified_installed" {
				if ordinary, openErr := store.OpenSQLiteWithAssetStore(db, assets); ordinary != nil || !errors.Is(openErr, store.ErrPhysicalRestoreInProgress) {
					if ordinary != nil {
						ordinary.Close()
					}
					t.Fatalf("startup was not blocked at %s: %v", fault, openErr)
				}
			}
			resumed, err := Restore(ctx, snapshot, RestoreOptions{
				Intent: "adopt", TargetDatabase: db, TargetAssetRoot: assets,
				EmergencyDirectory: filepath.Join(root, "emergency"),
			})
			if err != nil || !resumed.Resumed || resumed.Stage != "complete" {
				t.Fatalf("resume from %s: %+v %v", fault, resumed, err)
			}
		})
	}
}
