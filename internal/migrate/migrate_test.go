package migrate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/paths"
)

// installedAt builds a resolution whose roots are all under home, the way an
// installed Linux instance resolves them.
func installedAt(t *testing.T, home string) paths.Resolution {
	t.Helper()
	return paths.Resolution{
		Mode: paths.ModeInstalled,
		Roots: map[string]string{
			paths.RootConfig:        filepath.Join(home, ".config", "notrios"),
			paths.RootData:          filepath.Join(home, ".local", "share", "notrios"),
			paths.RootState:         filepath.Join(home, ".local", "state", "notrios"),
			paths.RootCache:         filepath.Join(home, ".cache", "notrios"),
			paths.RootRuntime:       filepath.Join(home, ".local", "state", "notrios", "runtime"),
			paths.RootProgramAssets: filepath.Join(home, "opt", "notrios"),
		},
	}
}

// legacyLayout writes a pre-0.8 library into dir/data.
func legacyLayout(t *testing.T, dir string) string {
	t.Helper()
	root := filepath.Join(dir, LegacyRootName)
	write := func(rel, content string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(DatabaseName, "the library")
	write(DatabaseName+"-wal", "uncheckpointed transactions")
	write(filepath.Join("assets", "ab", "blob.bin"), "an attachment")
	write(filepath.Join("quarantine", "q.bin"), "untrusted bytes")
	write(filepath.Join("projections", "p.json"), "derived")
	write(filepath.Join("search-index", "x.dat"), "derived")
	return root
}

func TestDetectFindsAPre08LibraryInTheWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)

	candidate, found := Detect(habit, installedAt(t, home), "")
	if !found {
		t.Fatal("a library sitting in the working directory was not detected")
	}
	if candidate.Root != filepath.Join(habit, LegacyRootName) {
		t.Fatalf("detected %s", candidate.Root)
	}
	if candidate.Occupied {
		t.Fatal("the resolved root is empty, so nothing is occupied")
	}
}

// A checkout is a separate instance whose data root already is ./data. There is
// nothing to migrate, and offering to migrate it would propose moving a
// developer's working library into their own installed one.
func TestDetectIgnoresASourceCheckout(t *testing.T) {
	habit := t.TempDir()
	legacyLayout(t, habit)

	// The real resolver returns *relative* roots in source mode -- "data", not
	// an absolute path -- so the same-directory check cannot be what rules this
	// out: it resolves a relative root against the process working directory,
	// which is not necessarily the directory being examined. The mode guard is
	// the only thing standing here, and this asserts it rather than something
	// that happens to agree with it.
	resolution := paths.Resolution{
		Mode:  paths.ModeSource,
		Roots: map[string]string{paths.RootData: LegacyRootName},
	}
	if _, found := Detect(habit, resolution, ""); found {
		t.Fatal("source mode must never report a migration")
	}
}

func TestDetectIgnoresADirectoryWithNoDatabase(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	if err := os.MkdirAll(filepath.Join(habit, LegacyRootName, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, found := Detect(habit, installedAt(t, home), ""); found {
		t.Fatal("a data directory with no database is not a library")
	}
}

func TestMigrationCopiesVerifiesRoutesAndRetires(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacy := legacyLayout(t, habit)
	resolution := installedAt(t, home)

	candidate, found := Detect(habit, resolution, "")
	if !found {
		t.Fatal("not detected")
	}
	plan, err := BuildPlan(candidate, resolution, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(plan, Options{Now: time.Date(2026, 9, 2, 6, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}

	// The library and its sidecar land under the data root.
	assertFile(t, filepath.Join(resolution.Root(paths.RootData), DatabaseName), "the library")
	assertFile(t, filepath.Join(resolution.Root(paths.RootData), DatabaseName+"-wal"), "uncheckpointed transactions")
	assertFile(t, filepath.Join(resolution.Root(paths.RootData), "assets", "ab", "blob.bin"), "an attachment")

	// The quarantine is state, not data: it holds the record of what a note
	// tried to fetch, which is not derived and not disposable.
	assertFile(t, filepath.Join(resolution.Root(paths.RootState), "quarantine", "q.bin"), "untrusted bytes")

	// Derived data is rebuilt rather than carried; a search index copied to a
	// new path would hold stale absolute paths inside it.
	for _, rebuilt := range []string{"projections", "search-index"} {
		if _, err := os.Stat(filepath.Join(resolution.Root(paths.RootCache), rebuilt)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s was copied into the cache root; it must be rebuilt", rebuilt)
		}
	}

	// The source is renamed, never deleted.
	if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the old root should have been renamed away")
	}
	assertFile(t, filepath.Join(report.RetiredTo, DatabaseName), "the library")
	if !strings.Contains(report.RetiredTo, "migrated-2026") {
		t.Fatalf("retired to %s, which does not name when", report.RetiredTo)
	}
}

// Merging two libraries is a decision rather than a copy, so a destination that
// already holds a database is refused and both paths are named.
func TestMigrationRefusesToMergeIntoAnExistingLibrary(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)
	resolution := installedAt(t, home)

	dataRoot := resolution.Root(paths.RootData)
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataRoot, DatabaseName), []byte("a different library"), 0o600); err != nil {
		t.Fatal(err)
	}

	candidate, found := Detect(habit, resolution, "")
	if !found {
		t.Fatal("not detected")
	}
	if !candidate.Occupied {
		t.Fatal("the destination holds a database and was not reported as occupied")
	}
	_, err := BuildPlan(candidate, resolution, time.Now())
	if !errors.Is(err, ErrDestinationOccupied) {
		t.Fatalf("expected a refusal, got %v", err)
	}
	// And the existing library is untouched.
	assertFile(t, filepath.Join(dataRoot, DatabaseName), "a different library")
}

// A dry run must leave the disk exactly as it found it apart from the plan.
func TestDryRunCopiesNothingAndRetiresNothing(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacy := legacyLayout(t, habit)
	resolution := installedAt(t, home)

	candidate, _ := Detect(habit, resolution, "")
	plan, err := BuildPlan(candidate, resolution, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(plan, Options{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(resolution.Root(paths.RootData), DatabaseName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a dry run copied the database")
	}
	assertFile(t, filepath.Join(legacy, DatabaseName), "the library")
	if _, err := os.Stat(plan.PlanPath); err != nil {
		t.Fatalf("the plan should still have been written: %v", err)
	}
}

// An interrupted migration resumes rather than repeating, and the items it
// already finished are not copied a second time.
func TestInterruptedMigrationResumesFromTheJournal(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)
	resolution := installedAt(t, home)

	candidate, _ := Detect(habit, resolution, "")
	plan, err := BuildPlan(candidate, resolution, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a crash after the assets finished: the journal records it done
	// and the copy is in place.
	assetsDestination := filepath.Join(resolution.Root(paths.RootData), "assets")
	if err := copyTree(filepath.Join(candidate.Root, "assets"), assetsDestination); err != nil {
		t.Fatal(err)
	}
	if err := appendJournal(plan.JournalPath, journalEntry{Item: "data.asset_store", Phase: "done", At: time.Now()}); err != nil {
		t.Fatal(err)
	}

	report, err := Execute(plan, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(report.Resumed, "data.asset_store") {
		t.Fatalf("the finished item was not resumed past: %+v", report)
	}
	if contains(report.Copied, "data.asset_store") {
		t.Fatal("the finished item was copied again")
	}
	assertFile(t, filepath.Join(resolution.Root(paths.RootData), DatabaseName), "the library")
	assertFile(t, filepath.Join(assetsDestination, "ab", "blob.bin"), "an attachment")
}

// The free-space rule refuses rather than filling the disk part-way.
func TestMigrationRefusesWithoutRoomForTheCopy(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)
	resolution := installedAt(t, home)

	candidate, _ := Detect(habit, resolution, "")
	plan, err := BuildPlan(candidate, resolution, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	plan.FreeBytes = plan.CopyBytes // enough for the copy, not for the margin
	if _, err := Execute(plan, Options{}); !errors.Is(err, ErrInsufficientSpace) {
		t.Fatalf("expected a space refusal, got %v", err)
	}
	assertFile(t, filepath.Join(candidate.Root, DatabaseName), "the library")
}

// Verification has to actually bite. A copy that silently differed from its
// source would be worse than no migration, because the user would then delete
// the original believing it redundant.
func TestVerificationRejectsACorruptedCopy(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.bin")
	destination := filepath.Join(dir, "copy.bin")
	if err := os.WriteFile(source, []byte("original bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("original bytez"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyTree(source, destination); !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("a differing copy passed verification: %v", err)
	}
}

// The plan is generated from the resolved-path table rather than a list of its
// own, so a path added to Notrios cannot be silently left behind.
func TestPlanCoversEveryResolvedPathThatLivesInsideARoot(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)
	resolution := installedAt(t, home)

	candidate, _ := Detect(habit, resolution, "")
	plan, err := BuildPlan(candidate, resolution, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"data.database_path", "data.asset_store", "data.projection_dir",
		"search_sidecar.index_dir", "remote_media.quarantine_dir",
	} {
		found := false
		for _, item := range plan.Items {
			if item.Key == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("the plan says nothing about %s", want)
		}
	}
}

// A symbolic link is refused rather than followed: it would either dangle in
// the new root or reach outside the tree being moved.
func TestMigrationRefusesASymbolicLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "outside")
	if err := os.WriteFile(target, []byte("elsewhere"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	err := copyTree(link, filepath.Join(dir, "copy"))
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected a refusal, got %v", err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(raw) != want {
		t.Fatalf("%s holds %q, want %q", path, raw, want)
	}
}

func contains(list []string, want string) bool {
	for _, entry := range list {
		if entry == want {
			return true
		}
	}
	return false
}

// A configuration stating a relative `directory: ./data` resolves against the
// working directory, so the library found here is the one already in use.
// Reporting it as stranded would invite the user to migrate a library they are
// actively using -- moving it out from under the configuration naming it.
func TestDetectIgnoresTheLibraryThisInstanceIsUsing(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)
	resolution := installedAt(t, home)

	inUse := filepath.Join(habit, LegacyRootName, DatabaseName)
	if _, found := Detect(habit, resolution, inUse); found {
		t.Fatal("the library in use was reported as strandable")
	}
	// A relative spelling of the same path is the same library.
	if _, found := Detect(habit, resolution, filepath.Join(habit, ".", LegacyRootName, DatabaseName)); found {
		t.Fatal("a relative spelling of the in-use path defeated the check")
	}
	// A different library is still detected.
	if _, found := Detect(habit, resolution, filepath.Join(home, "elsewhere", DatabaseName)); !found {
		t.Fatal("an unrelated in-use path suppressed a real detection")
	}
	// An unreadable configuration reports rather than suppresses: being unable
	// to read a config is not evidence that a library is in use.
	if _, found := Detect(habit, resolution, ""); !found {
		t.Fatal("an empty in-use path suppressed detection")
	}
}

// The interruption test above crashes after the *assets*, which sort before the
// database. Crashing after the database is the case that matters: it leaves a
// notes.sqlite at the destination, and the collision check then refused every
// retry as a merge with the library the migration had itself just copied.
// H3 requires the migration to be resumable; without this it was not.
func TestInterruptionAfterTheDatabaseStillResumes(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)
	resolution := installedAt(t, home)

	candidate, _ := Detect(habit, resolution, "")
	plan, err := BuildPlan(candidate, resolution, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Crash after the database: the copy is in place, the journal records it,
	// and the plan naming this source has been written.
	if err := copyTree(filepath.Join(candidate.Root, DatabaseName), filepath.Join(resolution.Root(paths.RootData), DatabaseName)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(plan.PlanPath), 0o700); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PlanPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appendJournal(plan.JournalPath, journalEntry{Item: "data.database_path", Phase: "done", At: time.Now()}); err != nil {
		t.Fatal(err)
	}

	// Re-detect: the destination now holds a database, exactly as after a crash.
	resumed, found := Detect(habit, resolution, "")
	if !found || !resumed.Occupied {
		t.Fatalf("expected an occupied destination: found=%v %+v", found, resumed)
	}
	resumePlan, err := BuildPlan(resumed, resolution, time.Now())
	if err != nil {
		t.Fatalf("the retry was refused instead of resumed: %v", err)
	}
	report, err := Execute(resumePlan, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(report.Resumed, "data.database_path") {
		t.Fatalf("the finished database was copied again: %+v", report)
	}
	// And the rest of the move completed.
	assertFile(t, filepath.Join(resolution.Root(paths.RootState), "quarantine", "q.bin"), "untrusted bytes")
	if report.RetiredTo == "" {
		t.Fatal("the resumed migration did not retire the old layout")
	}
}

// A plan left behind by a migration of some *other* directory must not license
// overwriting a real library.
func TestAStalePlanFromAnotherSourceDoesNotDefeatTheMergeRefusal(t *testing.T) {
	home := t.TempDir()
	habit := t.TempDir()
	legacyLayout(t, habit)
	resolution := installedAt(t, home)

	dataRoot := resolution.Root(paths.RootData)
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataRoot, DatabaseName), []byte("a different library"), 0o600); err != nil {
		t.Fatal(err)
	}

	journalDir := filepath.Join(resolution.Root(paths.RootState), "migration")
	if err := os.MkdirAll(journalDir, 0o700); err != nil {
		t.Fatal(err)
	}
	stale, err := json.Marshal(Plan{LegacyRoot: filepath.Join(t.TempDir(), "data")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(journalDir, "plan.json"), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appendJournal(filepath.Join(journalDir, "journal.jsonl"), journalEntry{Item: "data.database_path", Phase: "done", At: time.Now()}); err != nil {
		t.Fatal(err)
	}

	candidate, _ := Detect(habit, resolution, "")
	if _, err := BuildPlan(candidate, resolution, time.Now()); !errors.Is(err, ErrDestinationOccupied) {
		t.Fatalf("a stale plan from another source defeated the merge refusal: %v", err)
	}
	assertFile(t, filepath.Join(dataRoot, DatabaseName), "a different library")
}
