package harness

import "fmt"

func mapping(stage Stage, mode, description string) StageMapping {
	return StageMapping{Stage: stage, Mode: mode, Description: description}
}

func sharedSetup() []StageMapping {
	return []StageMapping{
		mapping(StageInventory, "shared", "aggregate source and canonical-state inventory"),
		mapping(StageForeignImport, "shared", "foreign source import into one isolated canonical library"),
	}
}

func archiveStages(layout, wrapper string) []StageMapping {
	stages := sharedSetup()
	return append(stages,
		mapping(StageSnapshotCreate, "measured", "archive-v2 "+layout+" full export without the verifier folded into timing"),
		mapping(StageSnapshotVerify, "measured", "archive-v2 complete semantic and checksum verification"),
		mapping(StageTransportPrepare, "measured", wrapper),
		mapping(StageTransportSeal, "measured", "NBK1 fixed-frame authenticated encryption"),
		mapping(StageSnapshotOpen, "measured", "authenticate frames, reproduce wrapper hash, extract, then verify archive-v2"),
		mapping(StageSnapshotRestore, "measured", "archive-v2 adopt restore into an isolated canonical library"),
		mapping(StageIncrementalReplay, "measured", "one post-snapshot journal operation converges through the directory carrier"),
	)
}

func sqliteStages(create, prepare string) []StageMapping {
	stages := sharedSetup()
	return append(stages,
		mapping(StageSnapshotCreate, "measured", create),
		mapping(StageSnapshotVerify, "measured", "SQLite PRAGMA integrity_check plus external-object manifest verification"),
		mapping(StageTransportPrepare, "measured", prepare),
		mapping(StageTransportSeal, "measured", "NBK1 fixed-frame authenticated encryption"),
		mapping(StageSnapshotOpen, "measured", "authenticate, unpack, and repeat integrity/manifest verification"),
		mapping(StageSnapshotRestore, "measured", "close target, preserve displaced state, install compatible image and packed external objects"),
		mapping(StageIncrementalReplay, "measured", "one post-snapshot journal operation converges through the directory carrier"),
	)
}

func repositoryStages(tool, source string) []StageMapping {
	return []StageMapping{
		mapping(StageInventory, "shared", "aggregate "+source+" inventory"),
		mapping(StageForeignImport, "shared", "foreign import is measured separately before canonical-state rows"),
		mapping(StageSnapshotCreate, "measured", tool+" first snapshot and unchanged second snapshot are separate rows"),
		mapping(StageSnapshotVerify, "measured", tool+" repository and payload integrity verification"),
		mapping(StageTransportPrepare, "integrated", tool+" chunks, packs, indexes, compression, and deduplication during snapshot creation"),
		mapping(StageTransportSeal, "integrated", tool+" repository authentication/encryption; record actual configured mode"),
		mapping(StageSnapshotOpen, "measured", tool+" repository open and archive enumeration without restore"),
		mapping(StageSnapshotRestore, "measured", tool+" restore into a new ext4 destination"),
		mapping(StageIncrementalReplay, "measured", tool+" changed second snapshot; do not equate it with Notrios operation replay"),
	}
}

func Adapters() []Adapter {
	return []Adapter{
		{ID: "archive-v2-loose", Class: "notrios-semantic", Source: "canonical", EncryptionBoundary: "separate NBK1 transport envelope", Stages: archiveStages("fanout", "stored ZIP with one entry per archive file")},
		{ID: "archive-v2-pack", Class: "notrios-semantic", Source: "canonical", EncryptionBoundary: "separate NBK1 transport envelope", Stages: archiveStages("pack", "stored ZIP over the bounded packed archive tree")},
		{ID: "catchup-loose", Class: "notrios-catchup", Source: "canonical", EncryptionBoundary: "NBK1 fixed authenticated frames", Stages: archiveStages("fanout", "current loose-directory plus stored-ZIP catch-up preparation")},
		{ID: "catchup-packed", Class: "notrios-catchup", Source: "canonical", EncryptionBoundary: "NBK1 fixed authenticated frames", Stages: archiveStages("pack", "packed archive-v2 through the current stored-ZIP catch-up preparation")},
		{ID: "sqlite-stopped-copy", Class: "notrios-image", Source: "canonical", EncryptionBoundary: "separate NBK1 transport envelope; SQLCipher modeled separately", Stages: sqliteStages("close/checkpoint source and copy the database image", "versioned wrapper plus external objects")},
		{ID: "sqlite-online-backup", Class: "notrios-image", Source: "canonical", EncryptionBoundary: "separate NBK1 transport envelope; SQLCipher modeled separately", Stages: sqliteStages("SQLite Online Backup API consistent image", "versioned wrapper plus external objects")},
		{ID: "sqlite-image-bundle", Class: "notrios-image", Source: "canonical", EncryptionBoundary: "separate NBK1 transport envelope; SQLCipher modeled separately", Stages: sqliteStages("SQLite Online Backup API image plus versioned manifest", "bounded packed resources/source bundles behind the manifest")},
		{ID: "restic-raw", Class: "reference", Source: "raw-source", EncryptionBoundary: "restic repository encryption/authentication", RepositoryPolicy: "create a new explicitly named repository; refuse an existing repository", Stages: repositoryStages("restic", "raw source tree")},
		{ID: "restic-canonical", Class: "reference", Source: "canonical", EncryptionBoundary: "restic repository encryption/authentication", RepositoryPolicy: "create a new explicitly named repository; refuse an existing repository", Stages: repositoryStages("restic", "stopped/checkpointed canonical state")},
		{ID: "borg-raw", Class: "reference", Source: "raw-source", EncryptionBoundary: "record actual Borg repository encryption mode", RepositoryPolicy: "create a new explicitly named repository; never mutate recipedb_repo", Stages: repositoryStages("borg", "raw source tree")},
		{ID: "borg-canonical", Class: "reference", Source: "canonical", EncryptionBoundary: "record actual Borg repository encryption mode", RepositoryPolicy: "create a new explicitly named repository; never mutate recipedb_repo", Stages: repositoryStages("borg", "stopped/checkpointed canonical state")},
	}
}

func AdapterByID(id string) (Adapter, error) {
	for _, adapter := range Adapters() {
		if adapter.ID == id {
			return adapter, nil
		}
	}
	return Adapter{}, fmt.Errorf("unknown adapter %q", id)
}

func ValidateAdapters(adapters []Adapter) error {
	seen := map[string]bool{}
	for _, adapter := range adapters {
		if !slugPattern.MatchString(adapter.ID) || seen[adapter.ID] {
			return fmt.Errorf("invalid or duplicate adapter %q", adapter.ID)
		}
		seen[adapter.ID] = true
		mapped := map[Stage]bool{}
		for _, stage := range adapter.Stages {
			if stage.Mode != "measured" && stage.Mode != "integrated" && stage.Mode != "shared" && stage.Mode != "not-applicable" {
				return fmt.Errorf("adapter %s stage %s has invalid mode %q", adapter.ID, stage.Stage, stage.Mode)
			}
			if mapped[stage.Stage] || stage.Description == "" {
				return fmt.Errorf("adapter %s has duplicate or empty stage %s", adapter.ID, stage.Stage)
			}
			mapped[stage.Stage] = true
		}
		for _, stage := range Stages {
			if !mapped[stage] {
				return fmt.Errorf("adapter %s does not map stage %s", adapter.ID, stage)
			}
		}
	}
	return nil
}
