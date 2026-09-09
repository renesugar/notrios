package archivev2

import "fmt"

type RestoreIntent string

const (
	RestoreReplace RestoreIntent = "replace"
	RestoreMerge   RestoreIntent = "merge"
	RestoreFork    RestoreIntent = "fork"
	RestoreAdopt   RestoreIntent = "adopt"
)

type RestoreIdentityRequest struct {
	Intent           RestoreIntent `json:"intent"`
	TargetDatabaseID string        `json:"target_database_id,omitempty"`
	TargetEmpty      bool          `json:"target_empty"`
	NewDatabaseID    string        `json:"new_database_id,omitempty"`
}

type RestoreIdentityDecision struct {
	Intent                  RestoreIntent `json:"intent"`
	ArchiveDatabaseID       string        `json:"archive_database_id"`
	ResultDatabaseID        string        `json:"result_database_id"`
	MintNewReplicaID        bool          `json:"mint_new_replica_id"`
	PreserveArchiveUniverse bool          `json:"preserve_archive_universe"`
	TreatRecordsAsForeign   bool          `json:"treat_records_as_foreign"`
}

// PlanRestoreIdentity makes database-universe handling explicit without
// mutating either archive or target. P4 must call this after full verification
// and before its first canonical write.
func PlanRestoreIdentity(manifest Manifest, request RestoreIdentityRequest) (RestoreIdentityDecision, error) {
	archiveID := manifest.Snapshot.DatabaseID
	if !validID(archiveID) {
		return RestoreIdentityDecision{}, fmt.Errorf("archive database identity is invalid")
	}
	decision := RestoreIdentityDecision{
		Intent:            request.Intent,
		ArchiveDatabaseID: archiveID,
	}
	switch request.Intent {
	case RestoreReplace:
		if request.TargetEmpty || manifest.Snapshot.Target != TargetFullArchive {
			return RestoreIdentityDecision{}, fmt.Errorf("replace requires a non-empty target and a full archive")
		}
		if !validID(request.TargetDatabaseID) {
			return RestoreIdentityDecision{}, fmt.Errorf("replace requires the current target database ID")
		}
		decision.ResultDatabaseID = archiveID
		decision.PreserveArchiveUniverse = true
		decision.MintNewReplicaID = true
	case RestoreAdopt:
		if !request.TargetEmpty || request.TargetDatabaseID != "" || manifest.Snapshot.Target != TargetFullArchive {
			return RestoreIdentityDecision{}, fmt.Errorf("adopt requires an empty target and a full archive")
		}
		decision.ResultDatabaseID = archiveID
		decision.PreserveArchiveUniverse = true
		decision.MintNewReplicaID = true
	case RestoreMerge:
		if request.TargetEmpty || !validID(request.TargetDatabaseID) {
			return RestoreIdentityDecision{}, fmt.Errorf("merge requires a non-empty target database")
		}
		decision.ResultDatabaseID = request.TargetDatabaseID
		decision.TreatRecordsAsForeign = archiveID != request.TargetDatabaseID
	case RestoreFork:
		if !validID(request.NewDatabaseID) || request.NewDatabaseID == archiveID || request.NewDatabaseID == request.TargetDatabaseID {
			return RestoreIdentityDecision{}, fmt.Errorf("fork requires a distinct explicit new database ID")
		}
		decision.ResultDatabaseID = request.NewDatabaseID
		decision.TreatRecordsAsForeign = false
		decision.MintNewReplicaID = true
	default:
		return RestoreIdentityDecision{}, fmt.Errorf("restore intent must be replace, merge, fork, or adopt")
	}
	return decision, nil
}
