package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/renesugar/notrios/internal/paths"
)

// The pre-migration backup.
//
// Bootstrap used to migrate a user's only copy in place, with no backup and no
// notice, on every open by every binary. The forward path works, and that was
// the problem: it works on the one copy there is.
//
// # Where the backup goes, and why it is not configurable by accident
//
// Beside the database, under the data root. That is not a preference, it is the
// only root that satisfies the constraint: the copy must land on the same
// filesystem as its source. A user may point data.directory at another disk, so
// <state> is not guaranteed to be the same device -- which would make the
// free-space check meaningless and let the copy fail part-way across devices.
// <cache> is excluded outright, because a purge disposes of it without backing
// it up, and a safety net that another feature is entitled to delete is not one.
//
// The location is named rather than temporary for one reason: recovery depends
// on a user finding it after a crash or a refusal. `notriosctl paths` reports
// it and every failure message names it.
//
// # What is copied
//
// A schema migration touches only the database file. Assets, projections and
// the search index are untouched, so this is the .sqlite plus its -wal and -shm
// sidecars, not a whole-library image -- `notriosctl snapshot create` already
// exists for that and is much heavier than this needs to be. The three files
// are copied as a set, which is what makes them consistent with each other; no
// checkpoint is forced, so the original is never written to in the act of
// protecting it.

// PreMigrationBackupDirName is the directory beside the database holding
// pre-migration copies.
const PreMigrationBackupDirName = "pre-migration-backups"

// migrationLockSuffix names the lock that serializes migration.
//
// It is deliberately not the ABI's <db>.owner lock. That one is held by
// notrioslib for a whole session, and taking it here would deadlock the very
// process that holds it: flock claims belong to an open file description, so a
// second descriptor on the same file in the same process conflicts with the
// first.
const migrationLockSuffix = ".migrating"

// Errors a caller distinguishes.
var (
	// ErrMigrationInProgress means another process holds the migration lock.
	ErrMigrationInProgress = errors.New("another process is migrating this database")
	// ErrBackupFailed means the pre-migration backup could not be made, so the
	// migration did not start.
	ErrBackupFailed = errors.New("the pre-migration backup could not be made")
	// ErrMigrationFailed means migration failed after the backup was taken.
	ErrMigrationFailed = errors.New("the schema migration failed")
)

// MigrationReport describes a migration this process performed.
type MigrationReport struct {
	FromVersion int      `json:"from_version"`
	ToVersion   int      `json:"to_version"`
	BackupDir   string   `json:"backup_dir"`
	Files       []string `json:"files"`
	// SHA256 maps each backed-up file name to its verified hash. The
	// incomplete-migration marker carries these forward so a later process can
	// tell a good copy from a damaged one without trusting the copy itself.
	SHA256 map[string]string `json:"sha256"`
	At     time.Time         `json:"at"`
}

// LastMigration reports the migration this store performed when it opened, if
// it performed one.
//
// Bootstrap keeps its `error`-only signature -- it has more than thirty callers
// -- so the notice a user should see is fetched rather than returned. A user
// whose schema was upgraded should not have to infer it from the absence of a
// complaint.
func (s *SQLiteStore) LastMigration() (MigrationReport, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastMigration == nil {
		return MigrationReport{}, false
	}
	return *s.lastMigration, true
}

// PreMigrationBackupRoot is where backups for this database are kept.
//
// Exported so `notriosctl paths` can report a location whose whole value is
// being findable, and so a failure message and the diagnostics agree.
func PreMigrationBackupRoot(databasePath string) string {
	databasePath = strings.TrimSpace(databasePath)
	if databasePath == "" || databasePath == ":memory:" {
		return ""
	}
	return filepath.Join(filepath.Dir(databasePath), PreMigrationBackupDirName)
}

// backupNameFor is the directory one migration writes into. The versions and
// the time are in the name because that is what a user reads first.
func backupNameFor(from, to int, at time.Time) string {
	return fmt.Sprintf("%d-to-%d-%s", from, to, at.UTC().Format("20060102T150405Z"))
}

// migrationLock is a held claim on migrating one database.
type migrationLock struct{ file *os.File }

func (l *migrationLock) release() {
	if l == nil || l.file == nil {
		return
	}
	// Closing drops the flock. The lock file is left in place: removing it
	// would race with a process that has already opened it and is about to
	// flock it, which would end up holding a lock on an unlinked inode while a
	// third took a fresh one.
	_ = l.file.Close()
	l.file = nil
}

// acquireMigrationLock claims the right to migrate this database.
//
// Non-blocking on purpose. This runs at startup, which must not hang, and a
// second process arriving mid-migration is better told what is happening than
// left waiting on a migration that may take a while.
func acquireMigrationLock(databasePath string) (*migrationLock, error) {
	lockPath := databasePath + migrationLockSuffix
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open migration lock %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("%w: %s is held by another Notrios process.\n"+
			"Wait for it to finish and open the database again; migrating from two\n"+
			"processes at once is what this lock exists to prevent.",
			ErrMigrationInProgress, lockPath)
	}
	return &migrationLock{file: file}, nil
}

// backupFilesFor lists the database and whichever sidecars exist.
func backupFilesFor(databasePath string) []string {
	files := []string{databasePath}
	for _, suffix := range []string{"-wal", "-shm"} {
		candidate := databasePath + suffix
		if info, err := os.Lstat(candidate); err == nil && info.Mode().IsRegular() {
			files = append(files, candidate)
		}
	}
	return files
}

// backupBeforeMigration copies the database and its sidecars, verifies every
// byte, and only then lets the migration proceed.
func (s *SQLiteStore) backupBeforeMigration(ctx context.Context, from, to int, now time.Time) (MigrationReport, error) {
	if err := ctx.Err(); err != nil {
		return MigrationReport{}, err
	}
	root := PreMigrationBackupRoot(s.path)
	if root == "" {
		return MigrationReport{}, fmt.Errorf("%w: this database has no directory to keep a backup beside", ErrBackupFailed)
	}
	sources := backupFilesFor(s.path)

	var total int64
	for _, source := range sources {
		info, err := os.Stat(source)
		if err != nil {
			return MigrationReport{}, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}
		total += info.Size()
	}

	// Twice the payload: the copy itself, plus room for the migration's own
	// journal and growth. Refusing here is the whole point -- a backup that
	// half-fits is not a backup.
	free, err := paths.FreeBytes(root)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	if free < total*2 {
		return MigrationReport{}, fmt.Errorf(
			"%w: backing up %d bytes needs %d bytes free beside the database, and %s has %d.\n"+
				"The database was not migrated and is exactly as it was. Free some space and open it again.",
			ErrBackupFailed, total, total*2, filepath.Dir(s.path), free)
	}

	directory := filepath.Join(root, backupNameFor(from, to, now))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return MigrationReport{}, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	report := MigrationReport{FromVersion: from, ToVersion: to, BackupDir: directory, At: now.UTC()}
	digests := map[string]string{}
	report.SHA256 = digests
	for _, source := range sources {
		name := filepath.Base(source)
		destination := filepath.Join(directory, name)
		digest, err := copyAndVerify(source, destination)
		if err != nil {
			// Leave the partial directory: it is named after the attempt and a
			// user reading the error should find what there is of it.
			return MigrationReport{}, fmt.Errorf("%w: copying %s to %s: %v", ErrBackupFailed, source, destination, err)
		}
		digests[name] = digest
		report.Files = append(report.Files, destination)
	}

	manifest := map[string]any{
		"schema":       "notrios.premigration.backup.v1",
		"from_version": from,
		"to_version":   to,
		"created_at":   report.At.Format(time.RFC3339Nano),
		"database":     filepath.Base(s.path),
		"sha256":       digests,
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return MigrationReport{}, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "MANIFEST.json"), append(encoded, '\n'), 0o600); err != nil {
		return MigrationReport{}, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	if err := syncDirectory(directory); err != nil {
		return MigrationReport{}, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	return report, nil
}

// copyAndVerify copies one file and hashes both ends.
//
// A copy that is not checked is not a backup: the failure it exists for is the
// one where the bytes did not all arrive, and that failure is silent.
func copyAndVerify(source, destination string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, digest), in); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	written := hex.EncodeToString(digest.Sum(nil))

	// Read the copy back rather than trusting the hash of what was sent. The
	// point is to know what is on the disk, not what went towards it.
	stored, err := sha256File(destination)
	if err != nil {
		return "", err
	}
	if stored != written {
		return "", fmt.Errorf("%s hashes %s but its copy hashes %s", source, written, stored)
	}
	return stored, nil
}

func sha256File(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, handle); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func syncDirectory(path string) error {
	handle, err := os.Open(path)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}

// pruneOldBackups keeps the most recent backup and removes the rest.
//
// Deleting immediately after a successful migration would make the net useless
// the moment a problem surfaces later than the migration -- which is when they
// usually do. Keeping every one grows without bound on a library that migrates
// often. Keeping the newest is the compromise, and it is only ever run after a
// migration has succeeded.
func pruneOldBackups(root, keep string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() && filepath.Join(root, entry.Name()) != keep {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		_ = os.RemoveAll(filepath.Join(root, name))
	}
}

// The incomplete-migration marker.
//
// A migration that dies part-way -- a failed statement, a killed process, a
// power cut -- leaves a database that is neither the old shape nor the new one.
// Until now the only remedy was a message telling the user to put the backup
// back by hand.
//
// The marker makes it automatic without ever writing over a database that is in
// use. It is written before the first migration statement and removed after the
// last, so finding one at startup means a migration did not finish. The undo
// then happens in `openSQLiteWithAssetStore` *before* the database is opened:
// nothing holds the file, nothing is mid-flight, and the copy named in the
// marker is known-good because its hashes were recorded when it was verified.
//
// It is JSON because every machine-written record here already is -- the backup
// manifest, the relocation plan and journal, the evidence reports -- and
// because internal/store has no YAML parser. The recovery path is the last
// place to need a new one.

// MigrationMarkerPath is the file recording a migration in progress. It sits
// beside the database, like the physical-restore marker it is modelled on.
func MigrationMarkerPath(databasePath string) string {
	return databasePath + ".migration-incomplete"
}

// MigrationMarker links a half-finished migration to the copy that undoes it.
type MigrationMarker struct {
	Schema      string            `json:"schema"`
	FromVersion int               `json:"from_version"`
	ToVersion   int               `json:"to_version"`
	Database    string            `json:"database"`
	BackupDir   string            `json:"backup_dir"`
	SHA256      map[string]string `json:"sha256"`
	StartedAt   time.Time         `json:"started_at"`
}

// MigrationMarkerSchema versions the marker so a future format change can be
// recognised rather than misread by a build that predates it.
const MigrationMarkerSchema = "notrios.premigration.marker.v1"

// ErrMigrationRolledBack reports that an incomplete migration was undone. It is
// not a failure of this run: the library is back as it was, and the caller's
// job is to say so and stop.
var ErrMigrationRolledBack = errors.New("an incomplete schema migration was rolled back")

// ErrMigrationRollbackImpossible means the marker was found but the backup it
// names cannot be trusted. Nothing is changed.
var ErrMigrationRollbackImpossible = errors.New("an incomplete schema migration could not be rolled back")

// RecoveryCommands are the commands named in migration recovery messages.
//
// They are listed here so a test can assert they still exist. A recovery
// message that names a command which has been renamed is worse than no message:
// it is read at exactly the moment the user has least room to improvise.
func RecoveryCommands() []string {
	return []string{
		"notriosctl paths",
		"notriosctl export archive-v2",
		"notriosctl import archive",
	}
}

func writeMigrationMarker(databasePath string, marker MigrationMarker) error {
	marker.Schema = MigrationMarkerSchema
	encoded, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	path := MigrationMarkerPath(databasePath)
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	// The marker must reach the disk before the first migration statement does,
	// or a power cut between them leaves a changed database and no record of it.
	handle, err := os.Open(path)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}

func clearMigrationMarker(databasePath string) error {
	err := os.Remove(MigrationMarkerPath(databasePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readMigrationMarker(databasePath string) (MigrationMarker, bool, error) {
	raw, err := os.ReadFile(MigrationMarkerPath(databasePath))
	if errors.Is(err, os.ErrNotExist) {
		return MigrationMarker{}, false, nil
	}
	if err != nil {
		return MigrationMarker{}, false, err
	}
	var marker MigrationMarker
	if err := json.Unmarshal(raw, &marker); err != nil {
		return MigrationMarker{}, false, fmt.Errorf("%w: the marker %s is unreadable: %v",
			ErrMigrationRollbackImpossible, MigrationMarkerPath(databasePath), err)
	}
	if marker.Schema != MigrationMarkerSchema {
		return MigrationMarker{}, false, fmt.Errorf("%w: the marker %s records format %q, and this build understands %q",
			ErrMigrationRollbackImpossible, MigrationMarkerPath(databasePath), marker.Schema, MigrationMarkerSchema)
	}
	return marker, true, nil
}

// rollBackIncompleteMigration undoes a migration that did not finish.
//
// It runs before the database is opened, which is the whole reason it can be
// automatic. The order matters: verify every byte of the backup first and
// refuse if anything is wrong, because replacing a partly migrated database
// with a corrupt copy would turn a recoverable situation into a lost library.
func rollBackIncompleteMigration(databasePath string) (bool, error) {
	marker, found, err := readMigrationMarker(databasePath)
	if err != nil || !found {
		return false, err
	}

	// Verify before touching anything. A backup that does not match what was
	// recorded when it was made is not a backup any more.
	for name, want := range marker.SHA256 {
		candidate := filepath.Join(marker.BackupDir, name)
		got, hashErr := sha256File(candidate)
		if hashErr != nil {
			return false, fmt.Errorf("%w: %s is named by %s but cannot be read (%v).\n"+
				"Nothing was changed. The database at %s may be partly migrated from schema %d to %d.",
				ErrMigrationRollbackImpossible, candidate, MigrationMarkerPath(databasePath), hashErr,
				databasePath, marker.FromVersion, marker.ToVersion)
		}
		if got != want {
			return false, fmt.Errorf("%w: %s hashes %s and %s recorded %s.\n"+
				"Nothing was changed, and both copies are left in place.",
				ErrMigrationRollbackImpossible, candidate, got, MigrationMarkerPath(databasePath), want)
		}
	}

	// Put every recorded file back. Each lands under a temporary name first, so
	// the rename that commits it is atomic.
	for name := range marker.SHA256 {
		source := filepath.Join(marker.BackupDir, name)
		destination := filepath.Join(filepath.Dir(databasePath), name)
		staging := destination + ".restoring"
		if _, copyErr := copyAndVerify(source, staging); copyErr != nil {
			_ = os.Remove(staging)
			return false, fmt.Errorf("%w: restoring %s: %v", ErrMigrationRollbackImpossible, destination, copyErr)
		}
		if renameErr := os.Rename(staging, destination); renameErr != nil {
			_ = os.Remove(staging)
			return false, fmt.Errorf("%w: restoring %s: %v", ErrMigrationRollbackImpossible, destination, renameErr)
		}
	}

	// Remove any sidecar the failed attempt created that the backup does not
	// have. Leaving one behind would let SQLite replay a write-ahead log
	// belonging to the migration we just undid, straight back over the
	// restored database.
	for _, suffix := range []string{"-wal", "-shm"} {
		name := filepath.Base(databasePath) + suffix
		if _, recorded := marker.SHA256[name]; recorded {
			continue
		}
		if err := os.Remove(databasePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("%w: removing %s left over from the failed migration: %v",
				ErrMigrationRollbackImpossible, databasePath+suffix, err)
		}
	}

	if err := syncDirectory(filepath.Dir(databasePath)); err != nil {
		return false, fmt.Errorf("%w: %v", ErrMigrationRollbackImpossible, err)
	}
	// Only now: the marker is the record that the undo is still owed, so it is
	// removed last. A crash before this point simply repeats the restore, which
	// is safe because it is a copy of a verified copy.
	if err := clearMigrationMarker(databasePath); err != nil {
		return false, fmt.Errorf("%w: the database was restored but %s could not be removed: %v",
			ErrMigrationRollbackImpossible, MigrationMarkerPath(databasePath), err)
	}
	return true, nil
}

// migrationRolledBackError is the message a user sees after an automatic undo.
//
// It stops rather than carrying on, for two reasons. The database is back at
// its old schema, so this build would immediately try the same migration again
// and most likely fail the same way. And a user whose library was just rolled
// back should hear about it once, plainly, rather than find it in a log.
func migrationRolledBackError(marker MigrationMarker, databasePath string) error {
	return fmt.Errorf("%w.\n\n%s", ErrMigrationRolledBack, MigrationRolledBackMessage(marker, databasePath))
}

// MigrationRolledBackMessage is the body of that message.
//
// It is exported so a test in the CLI can check that the commands it names
// still exist. The alternative -- trusting that nobody renames a subcommand --
// is exactly the staleness this text cannot afford.
func MigrationRolledBackMessage(marker MigrationMarker, databasePath string) string {
	return fmt.Sprintf(
		"A migration from schema %d to %d did not finish, so %s has been restored\n"+
			"from the verified copy taken before it started:\n"+
			"  %s\n"+
			"Your notes are as they were. Nothing was lost, and Notrios has stopped\n"+
			"rather than attempt the same migration again.\n\n"+
			"What to do next:\n"+
			"  1. `notriosctl paths` shows which database this is and where its backups are.\n"+
			"  2. If the upgrade keeps failing, move your notes across instead of migrating:\n"+
			"       notriosctl export archive-v2 --db %s /path/to/export\n"+
			"     then start the new Notrios with an empty library and run:\n"+
			"       notriosctl import archive /path/to/export\n"+
			"  3. Keep %s until you are satisfied; nothing deletes it for you.",
		marker.FromVersion, marker.ToVersion, databasePath,
		marker.BackupDir, databasePath, marker.BackupDir)
}
