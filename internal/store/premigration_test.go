package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// olderLibrary builds a real database and winds its recorded version back, so
// the next open has something worth losing and a migration to perform.
func olderLibrary(t *testing.T, version int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.sqlite")
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("seed bootstrap: %v", err)
	}
	if _, err := st.CreateDocument(context.Background(), CreateDocumentRequest{
		Title: "a note that must survive", Body: "kept", Message: "seed",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(context.Background(), "PRAGMA user_version = "+strconv.Itoa(version)+";"); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func backupDirs(t *testing.T, databasePath string) []string {
	t.Helper()
	root := PreMigrationBackupRoot(databasePath)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, filepath.Join(root, entry.Name()))
		}
	}
	return out
}

// The central promise: an old database is copied, the copy is verified, and
// only then is the original migrated.
func TestMigrationBacksUpTheDatabaseAndVerifiesIt(t *testing.T) {
	path := olderLibrary(t, 19)
	before, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	report, ok := st.LastMigration()
	if !ok {
		t.Fatal("a migration happened and was not reported")
	}
	if report.FromVersion != 19 || report.ToVersion != CurrentSchemaVersion {
		t.Fatalf("reported %d -> %d", report.FromVersion, report.ToVersion)
	}

	// The backup is a byte-for-byte copy of the database as it was.
	copied := filepath.Join(report.BackupDir, "notes.sqlite")
	after, err := sha256File(copied)
	if err != nil {
		t.Fatalf("the backup is not readable: %v", err)
	}
	if after != before {
		t.Fatal("the backup does not match the database as it was before the migration")
	}

	// And the manifest says so independently.
	raw, err := os.ReadFile(filepath.Join(report.BackupDir, "MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		FromVersion int               `json:"from_version"`
		ToVersion   int               `json:"to_version"`
		SHA256      map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FromVersion != 19 || manifest.SHA256["notes.sqlite"] != before {
		t.Fatalf("manifest disagrees with the copy: %+v", manifest)
	}

	// The original really was migrated, and the note is still there.
	status, err := st.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version %d after migration", status.SchemaVersion)
	}
	empty, err := st.LibraryIsEmpty(context.Background())
	if err != nil || empty {
		t.Fatalf("the migrated library lost its notes: empty=%v err=%v", empty, err)
	}
}

// A fresh database has nothing to lose, and backing one up would leave a
// directory of nothing beside every new library.
func TestAFreshDatabaseIsNotBackedUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.sqlite")
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.LastMigration(); ok {
		t.Fatal("a fresh database reported a migration")
	}
	if dirs := backupDirs(t, path); len(dirs) != 0 {
		t.Fatalf("a fresh database was backed up: %v", dirs)
	}
}

// Opening an already-current database must not back it up again. Otherwise
// every restart of the service would copy the library.
func TestRepeatedOpensDoNotBackUpAgain(t *testing.T) {
	path := olderLibrary(t, 19)

	first, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := first.LastMigration(); !ok {
		t.Fatal("the first open should have migrated")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	afterFirst := backupDirs(t, path)
	if len(afterFirst) != 1 {
		t.Fatalf("expected one backup after the migration, got %v", afterFirst)
	}

	for attempt := 0; attempt < 3; attempt++ {
		again, err := OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := again.Bootstrap(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, ok := again.LastMigration(); ok {
			t.Fatalf("open %d reported a migration on an already-current database", attempt+2)
		}
		if err := again.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if got := backupDirs(t, path); len(got) != 1 {
		t.Fatalf("repeated opens changed the backups: %v", got)
	}
}

// Retention: the most recent backup is kept and older ones are removed.
func TestOnlyTheMostRecentBackupIsKept(t *testing.T) {
	path := olderLibrary(t, 19)
	root := PreMigrationBackupRoot(path)
	stale := filepath.Join(root, "17-to-19-20200101T000000Z")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "notes.sqlite"), []byte("older"), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	report, _ := st.LastMigration()

	dirs := backupDirs(t, path)
	if len(dirs) != 1 || dirs[0] != report.BackupDir {
		t.Fatalf("retention kept %v, want only %s", dirs, report.BackupDir)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the older backup was not removed")
	}
}

// The backup holds the user's notes, so it is owner-only like everything else
// that does.
func TestBackupPermissionsAreOwnerOnly(t *testing.T) {
	path := olderLibrary(t, 19)
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	report, _ := st.LastMigration()

	info, err := os.Stat(report.BackupDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("backup directory mode %o, want 700", info.Mode().Perm())
	}
	for _, file := range report.Files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if fileInfo.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode %o, want 600", file, fileInfo.Mode().Perm())
		}
	}
}

// A second process cannot migrate while the first is doing so. The lock is
// held across backup and migration together, not just one of them.
func TestASecondProcessIsRefusedDuringMigration(t *testing.T) {
	path := olderLibrary(t, 19)

	// Stand in for the other process: hold the lock the way it would.
	held, err := acquireMigrationLock(path)
	if err != nil {
		t.Fatal(err)
	}

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	err = st.Bootstrap(context.Background())
	if !errors.Is(err, ErrMigrationInProgress) {
		t.Fatalf("expected a refusal while another process migrates, got %v", err)
	}
	// Nothing was copied and nothing was migrated.
	if dirs := backupDirs(t, path); len(dirs) != 0 {
		t.Fatalf("a refused open still wrote backups: %v", dirs)
	}

	held.release()

	// With the lock free the same open succeeds.
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("after the lock was released: %v", err)
	}
	if _, ok := st.LastMigration(); !ok {
		t.Fatal("the retry did not migrate")
	}
}

// Losing the race must not cause a second backup. The version is re-read once
// the lock is held, because the first reading is stale by then.
func TestAProcessThatLosesTheRaceDoesNotBackUpAgain(t *testing.T) {
	path := olderLibrary(t, 19)

	first, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	after := backupDirs(t, path)
	if len(after) != 1 {
		t.Fatalf("expected one backup, got %v", after)
	}

	// A second store that decided to migrate from a now-stale reading.
	second, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := second.migrateUnderLock(context.Background(), 19); err != nil {
		t.Fatal(err)
	}
	if _, ok := second.LastMigration(); ok {
		t.Fatal("the loser of the race reported migrating an already-current database")
	}
	if got := backupDirs(t, path); len(got) != 1 {
		t.Fatalf("the loser wrote a second backup: %v", got)
	}
}

// Not enough room means refuse before the first statement. A backup that half
// fits is not a backup, and migrating without one is the behaviour this exists
// to remove.
func TestInsufficientSpaceRefusesBeforeMigrating(t *testing.T) {
	path := olderLibrary(t, 19)
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// A directory that cannot be written to stands in for a full disk: both
	// end the backup before any migration statement runs.
	root := PreMigrationBackupRoot(path)
	if err := os.MkdirAll(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

	err = st.Bootstrap(context.Background())
	if !errors.Is(err, ErrBackupFailed) {
		t.Fatalf("expected the backup to refuse, got %v", err)
	}
	if !strings.Contains(err.Error(), "backup") {
		t.Fatalf("the refusal does not say what failed: %v", err)
	}

	// The database is untouched: still at its old version, still holding notes.
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	status, err := st.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.SchemaVersion != 19 {
		t.Fatalf("the database was migrated despite the backup failing: version %d", status.SchemaVersion)
	}
}

// PreMigrationBackupRoot is what `notriosctl paths` reports, so it has to agree
// with where a backup actually lands.
func TestTheReportedBackupRootIsWhereBackupsLand(t *testing.T) {
	path := olderLibrary(t, 19)
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	report, _ := st.LastMigration()
	if filepath.Dir(report.BackupDir) != PreMigrationBackupRoot(path) {
		t.Fatalf("backup went to %s, reported root is %s", report.BackupDir, PreMigrationBackupRoot(path))
	}
	if PreMigrationBackupRoot(":memory:") != "" {
		t.Fatal("an in-memory database must report no backup root")
	}
}

// An in-memory database is transient: there is nothing to lose, nowhere beside
// it to keep a copy, and no second process that could open it. It must migrate
// without a backup rather than be refused for the lack of one.
//
// This is pinned because getting it wrong broke five existing schema-upgrade
// tests at once: the backup refused, so every in-memory store whose recorded
// version was behind became unusable.
func TestAnInMemoryDatabaseMigratesWithoutABackup(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(ctx, "PRAGMA user_version = 19;"); err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("an in-memory database was refused a migration: %v", err)
	}
	status, err := st.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("in-memory schema version %d after migration", status.SchemaVersion)
	}
	if _, ok := st.LastMigration(); ok {
		t.Fatal("an in-memory database reported a backed-up migration")
	}
}

// A migration that dies part-way is undone at the next open, before anything
// opens the database. That timing is the whole design: no handle is held, no
// write-ahead log is being replayed, and the copy being restored was verified
// when it was made.
func TestAnIncompleteMigrationIsRolledBackAtTheNextOpen(t *testing.T) {
	path := olderLibrary(t, 19)
	original, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}

	// Take the backup the way a migration does, then leave the marker behind
	// and damage the database, exactly as a failure part-way would.
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := st.backupBeforeMigration(context.Background(), 19, CurrentSchemaVersion, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMigrationMarker(path, MigrationMarker{
		FromVersion: 19, ToVersion: CurrentSchemaVersion, Database: path,
		BackupDir: report.BackupDir, SHA256: report.SHA256, StartedAt: report.At,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(context.Background(), "PRAGMA user_version = 23;"); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	// The next open undoes it and refuses to carry on.
	_, err = OpenSQLite(path)
	if !errors.Is(err, ErrMigrationRolledBack) {
		t.Fatalf("expected an automatic rollback, got %v", err)
	}
	message := err.Error()
	for _, want := range []string{report.BackupDir, "notriosctl paths", "export archive-v2", "as they were"} {
		if !strings.Contains(message, want) {
			t.Errorf("the rollback message does not mention %q:\n%s", want, message)
		}
	}

	// The database really is back, byte for byte.
	restored, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if restored != original {
		t.Fatal("the restored database does not match the one that was backed up")
	}
	// And the marker is gone, so a further open proceeds normally.
	if _, err := os.Stat(MigrationMarkerPath(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the marker survived a successful rollback")
	}
	reopened, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("the open after a rollback should succeed: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
}

// A successful migration must leave no marker, or every subsequent start would
// roll a perfectly good library back.
func TestASuccessfulMigrationLeavesNoMarker(t *testing.T) {
	path := olderLibrary(t, 19)
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(MigrationMarkerPath(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a successful migration left an incomplete-migration marker behind")
	}
}

// If the backup cannot be trusted, refuse. Replacing a partly migrated database
// with a corrupt copy would turn a recoverable situation into a lost library.
func TestARollbackRefusesWhenTheBackupDoesNotMatch(t *testing.T) {
	path := olderLibrary(t, 19)
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := st.backupBeforeMigration(context.Background(), 19, CurrentSchemaVersion, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMigrationMarker(path, MigrationMarker{
		FromVersion: 19, ToVersion: CurrentSchemaVersion, Database: path,
		BackupDir: report.BackupDir, SHA256: report.SHA256, StartedAt: report.At,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	// Something damaged the copy after it was verified.
	copied := filepath.Join(report.BackupDir, "notes.sqlite")
	if err := os.WriteFile(copied, []byte("not the database"), 0o600); err != nil {
		t.Fatal(err)
	}
	damagedBefore, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = OpenSQLite(path)
	if !errors.Is(err, ErrMigrationRollbackImpossible) {
		t.Fatalf("expected a refusal on a mismatched backup, got %v", err)
	}
	// Nothing was changed, and both copies are still there.
	after, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if after != damagedBefore {
		t.Fatal("the database was modified despite the refusal")
	}
	if _, err := os.Stat(MigrationMarkerPath(path)); err != nil {
		t.Fatal("the marker was removed even though the rollback did not happen")
	}
}

// A write-ahead log created by the failed migration must not survive the
// rollback: SQLite would replay it straight back over the restored database.
func TestRollbackRemovesASidecarTheBackupDoesNotHave(t *testing.T) {
	path := olderLibrary(t, 19)
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := st.backupBeforeMigration(context.Background(), 19, CurrentSchemaVersion, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Record only the database, as if no sidecars existed when it was taken.
	onlyDatabase := map[string]string{"notes.sqlite": report.SHA256["notes.sqlite"]}
	if err := writeMigrationMarker(path, MigrationMarker{
		FromVersion: 19, ToVersion: CurrentSchemaVersion, Database: path,
		BackupDir: report.BackupDir, SHA256: onlyDatabase, StartedAt: report.At,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	// The failed migration left a write-ahead log behind.
	if err := os.WriteFile(path+"-wal", []byte("a log from the failed attempt"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenSQLite(path); !errors.Is(err, ErrMigrationRolledBack) {
		t.Fatalf("expected a rollback, got %v", err)
	}
	if _, err := os.Stat(path + "-wal"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a write-ahead log from the failed migration survived the rollback")
	}
}

// A marker this build does not understand is refused rather than guessed at.
func TestAnUnknownMarkerFormatIsRefused(t *testing.T) {
	path := olderLibrary(t, 19)
	if err := os.WriteFile(MigrationMarkerPath(path),
		[]byte(`{"schema":"notrios.premigration.marker.v99"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSQLite(path); !errors.Is(err, ErrMigrationRollbackImpossible) {
		t.Fatalf("expected a refusal on an unknown marker format, got %v", err)
	}
}
