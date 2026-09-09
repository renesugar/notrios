// Package migrate moves a pre-H4 checkout-relative library into the resolved
// roots, explicitly and reversibly.
//
// # What actually needs migrating
//
// Much less than it first appears, and knowing which cases are already safe is
// what keeps this package small.
//
// A configuration that states a path keeps it: internal/config only fills keys
// the file left unsaid, so every existing config -- absolute or relative, in a
// checkout or not -- reads and writes exactly where it always did. A binary run
// from a checkout is in source mode, where all four mutable roots resolve to
// ./data, which is the historical layout unchanged. Neither of those moves, and
// neither is this package's business.
//
// One case does move. A user who ran a pre-H4 binary with no configuration
// file, from some directory of their own choosing, has a library at
// <that directory>/data. The compiled defaults were relative to the working
// directory, so that is where it went. An H4 binary resolves the native roots
// instead and finds them empty, and the user's notes appear to have vanished.
//
// The old location is not recorded anywhere -- it was whatever directory the
// user happened to be standing in -- so nothing can look it up. What can be
// done is to notice it when the user is standing there again, which is the
// common case for anyone with a habitual directory, and say so.
//
// # Why bytes rather than the store
//
// Migration copies files. It never opens the source database, and that is a
// requirement rather than an implementation detail: opening it would run the
// schema migrations against the user's only copy before any copy of it exists.
// The whole point of moving the library is to still have it afterwards.
package migrate

import (
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
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/paths"
)

// LegacyRootName is the directory the pre-H4 compiled defaults used, relative
// to the working directory.
const LegacyRootName = "data"

// DatabaseName is the pre-H4 database file name. It is unchanged by H4 -- only
// the directory holding it moved -- which is why the plan can be generated from
// the resolved-path table without a separate rename map.
const DatabaseName = "notes.sqlite"

// Errors callers distinguish.
var (
	// ErrNoLegacyLayout means there is nothing here to migrate.
	ErrNoLegacyLayout = errors.New("no pre-H4 layout found")
	// ErrDestinationOccupied means the resolved roots already hold a library.
	// Merging two libraries is a decision, not a copy.
	ErrDestinationOccupied = errors.New("the resolved location already holds a database")
	// ErrInsufficientSpace means the destination filesystem has too little room.
	ErrInsufficientSpace = errors.New("not enough free space to migrate safely")
	// ErrVerificationFailed means a copied file did not match its source.
	ErrVerificationFailed = errors.New("a copied file did not match its source")
)

// Action is what the plan does with one category.
type Action string

const (
	// ActionCopy copies the bytes and verifies them.
	ActionCopy Action = "copy"
	// ActionRebuild leaves the old copy behind because the destination is
	// derived data that regenerates from the library. Copying a Recoll index to
	// a new path would carry stale absolute paths inside it.
	ActionRebuild Action = "rebuild"
	// ActionDiscard drops work in flight that must not survive the move.
	ActionDiscard Action = "discard"
)

// Candidate is a pre-H4 layout found beside the process.
type Candidate struct {
	// Root is the old data directory.
	Root string
	// DatabasePath is the database inside it.
	DatabasePath string
	// ResolvedRoot is where this binary would keep a library instead.
	ResolvedRoot string
	// Occupied reports that the resolved location already holds a database, so
	// migration would be a merge and is refused.
	Occupied bool
}

// Detect reports a pre-H4 layout in dir, if there is one.
//
// Source mode never detects: a checkout's data root already is ./data, so the
// layout it would "find" is the one it is using.
//
// inUseDatabase is the database this process would actually open, and passing
// it is not optional in practice. A configuration stating a relative
// `directory: ./data` resolves against the working directory, so the library
// found here is the one already in use -- and reporting it as stranded would
// invite the user to migrate a library they are actively using, moving it out
// from under the configuration that names it. Pass "" only when there is
// genuinely no configuration to consult.
func Detect(dir string, resolution paths.Resolution, inUseDatabase string) (Candidate, bool) {
	if resolution.Mode == paths.ModeSource {
		return Candidate{}, false
	}
	resolvedRoot := strings.TrimSpace(resolution.Root(paths.RootData))
	if resolvedRoot == "" {
		return Candidate{}, false
	}
	root := filepath.Join(dir, LegacyRootName)
	if sameDir(root, resolvedRoot) {
		return Candidate{}, false
	}
	database := filepath.Join(root, DatabaseName)
	if !isRegularFile(database) {
		return Candidate{}, false
	}
	if sameDir(database, inUseDatabase) {
		return Candidate{}, false
	}
	return Candidate{
		Root:         root,
		DatabasePath: database,
		ResolvedRoot: resolvedRoot,
		Occupied:     isRegularFile(filepath.Join(resolvedRoot, DatabaseName)),
	}, true
}

// Item is one category's move.
type Item struct {
	Key         string `json:"key"`
	Root        string `json:"root"`
	Action      Action `json:"action"`
	Source      string `json:"source"`
	Destination string `json:"destination,omitempty"`
	Files       int    `json:"files"`
	Bytes       int64  `json:"bytes"`
	Present     bool   `json:"present"`
}

// Plan is the whole move, written to disk before anything is touched.
type Plan struct {
	LegacyRoot    string    `json:"legacy_root"`
	GeneratedAt   time.Time `json:"generated_at"`
	Items         []Item    `json:"items"`
	CopyBytes     int64     `json:"copy_bytes"`
	CopyFiles     int       `json:"copy_files"`
	FreeBytes     int64     `json:"free_bytes"`
	RequiredBytes int64     `json:"required_bytes"`
	JournalPath   string    `json:"journal_path"`
	PlanPath      string    `json:"plan_path"`
}

// BuildPlan enumerates the move from the resolved-path table.
//
// The table is config's, not a copy of it. A path added to Notrios appears here
// without this file being edited, which is the only way a new category cannot
// be quietly left behind.
func BuildPlan(candidate Candidate, resolution paths.Resolution, now time.Time) (Plan, error) {
	stateRoot := strings.TrimSpace(resolution.Root(paths.RootState))
	if stateRoot == "" {
		return Plan{}, errors.New("no state root resolved; the migration journal has nowhere to live")
	}
	journalDir := filepath.Join(stateRoot, "migration")

	// A destination holding a database is a merge -- unless it is our own
	// half-finished copy. An interrupted migration that got as far as the
	// database leaves exactly that, and without this check the resume H3
	// requires is impossible: every retry would be refused as a merge with the
	// library it had itself just copied. The journal is what tells the two
	// apart, and it is trusted only when its plan names this same source.
	//
	// Occupied is cleared by the caller when the destination database turned
	// out to hold nothing, which is the ordinary case rather than an exotic
	// one: a user notices their notes are missing by running the new binary,
	// and `doctor` or the service creates an empty library at the resolved path
	// in the act of looking. Refusing that as a merge would tell the user their
	// own empty artefact is a second library they must reconcile.
	if candidate.Occupied {
		resuming, err := inProgressFor(filepath.Join(journalDir, "plan.json"), filepath.Join(journalDir, "journal.jsonl"), candidate.Root)
		if err != nil {
			return Plan{}, err
		}
		if !resuming {
			return Plan{}, fmt.Errorf("%w: %s already exists; the library at %s was not touched",
				ErrDestinationOccupied, filepath.Join(candidate.ResolvedRoot, DatabaseName), candidate.Root)
		}
	}

	plan := Plan{
		LegacyRoot:  candidate.Root,
		GeneratedAt: now.UTC(),
		JournalPath: filepath.Join(journalDir, "journal.jsonl"),
		PlanPath:    filepath.Join(journalDir, "plan.json"),
	}

	for _, mapping := range config.ResolvedPathMappings() {
		if len(mapping.Relative) == 0 {
			// A root itself, not a thing inside one.
			continue
		}
		destRoot := strings.TrimSpace(resolution.Root(mapping.Root))
		if destRoot == "" {
			continue
		}
		source := filepath.Join(append([]string{candidate.Root}, mapping.Relative...)...)
		item := Item{
			Key:    mapping.Key,
			Root:   mapping.Root,
			Source: source,
			Action: actionFor(mapping.Root),
		}
		if item.Action == ActionCopy {
			item.Destination = filepath.Join(append([]string{destRoot}, mapping.Relative...)...)
		}
		files, bytes, present, err := measure(source)
		if err != nil {
			return Plan{}, err
		}
		item.Files, item.Bytes, item.Present = files, bytes, present
		if item.Action == ActionCopy && present {
			plan.CopyBytes += bytes
			plan.CopyFiles += files
		}
		plan.Items = append(plan.Items, item)
	}

	// The database sidecars travel with the database. They are not settings, so
	// they are not in the table, but leaving a -wal behind would discard
	// committed transactions that had not yet been checkpointed.
	plan.Items = append(plan.Items, sidecarItems(candidate, resolution, &plan)...)

	sort.SliceStable(plan.Items, func(i, j int) bool { return plan.Items[i].Source < plan.Items[j].Source })

	free, err := paths.FreeBytes(candidate.ResolvedRoot)
	if err != nil {
		return Plan{}, err
	}
	plan.FreeBytes = free
	// Twice the payload: the copy, plus room for the destination filesystem's
	// own overhead and for a rollback that has to put things back.
	plan.RequiredBytes = plan.CopyBytes * 2
	return plan, nil
}

func sidecarItems(candidate Candidate, resolution paths.Resolution, plan *Plan) []Item {
	destRoot := strings.TrimSpace(resolution.Root(paths.RootData))
	items := []Item{}
	for _, suffix := range []string{"-wal", "-shm"} {
		source := candidate.DatabasePath + suffix
		files, bytes, present, err := measure(source)
		if err != nil || !present {
			continue
		}
		items = append(items, Item{
			Key:         "data.database_path" + suffix,
			Root:        paths.RootData,
			Action:      ActionCopy,
			Source:      source,
			Destination: filepath.Join(destRoot, DatabaseName+suffix),
			Files:       files,
			Bytes:       bytes,
			Present:     true,
		})
		plan.CopyBytes += bytes
		plan.CopyFiles += files
	}
	return items
}

func actionFor(root string) Action {
	switch root {
	case paths.RootCache:
		return ActionRebuild
	case paths.RootRuntime:
		return ActionDiscard
	default:
		return ActionCopy
	}
}

// Report is what happened.
type Report struct {
	Plan      Plan     `json:"plan"`
	Copied    []string `json:"copied"`
	Skipped   []string `json:"skipped"`
	Resumed   []string `json:"resumed"`
	RetiredTo string   `json:"retired_to"`
}

// Options tune one run.
type Options struct {
	// Now fixes the timestamps so a test can assert them.
	Now time.Time
	// DryRun writes the plan and stops.
	DryRun bool
}

// Execute performs the move: plan, space check, copy, verify, commit, retire.
//
// The source is untouched until the very last step, and that step renames
// rather than deletes. The worst outcome of an interruption is wasted disk.
func Execute(plan Plan, options Options) (Report, error) {
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	report := Report{Plan: plan}

	if plan.CopyBytes > 0 && plan.FreeBytes < plan.RequiredBytes {
		return report, fmt.Errorf("%w: %d bytes to copy needs %d bytes free, and %s has %d",
			ErrInsufficientSpace, plan.CopyBytes, plan.RequiredBytes, filepath.Dir(plan.PlanPath), plan.FreeBytes)
	}

	if err := os.MkdirAll(filepath.Dir(plan.PlanPath), 0o700); err != nil {
		return report, err
	}
	encoded, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return report, err
	}
	if err := os.WriteFile(plan.PlanPath, append(encoded, '\n'), 0o600); err != nil {
		return report, err
	}
	if options.DryRun {
		return report, nil
	}

	done, err := completedItems(plan.JournalPath)
	if err != nil {
		return report, err
	}

	for _, item := range plan.Items {
		if item.Action != ActionCopy || !item.Present {
			report.Skipped = append(report.Skipped, item.Key)
			continue
		}
		if done[item.Key] {
			report.Resumed = append(report.Resumed, item.Key)
			continue
		}
		if err := appendJournal(plan.JournalPath, journalEntry{Item: item.Key, Phase: "begin", At: now.UTC()}); err != nil {
			return report, err
		}
		staging := item.Destination + ".migrating"
		if err := os.RemoveAll(staging); err != nil {
			return report, err
		}
		if err := copyTree(item.Source, staging); err != nil {
			return report, err
		}
		if err := verifyTree(item.Source, staging); err != nil {
			_ = os.RemoveAll(staging)
			return report, err
		}
		if err := os.MkdirAll(filepath.Dir(item.Destination), 0o700); err != nil {
			return report, err
		}
		// An empty directory already at the destination is in the way rather
		// than in use: `doctor` and the service create <data>/assets simply by
		// looking, and a rename onto an existing directory fails. Removing an
		// empty one is safe; a non-empty one is someone's data and the rename
		// is left to fail loudly.
		if err := removeEmptyDirectory(item.Destination); err != nil {
			return report, err
		}
		if err := os.Rename(staging, item.Destination); err != nil {
			return report, err
		}
		if err := appendJournal(plan.JournalPath, journalEntry{Item: item.Key, Phase: "done", At: now.UTC()}); err != nil {
			return report, err
		}
		report.Copied = append(report.Copied, item.Key)
	}

	retired := plan.LegacyRoot + ".migrated-" + now.UTC().Format("20060102T150405Z")
	if err := os.Rename(plan.LegacyRoot, retired); err != nil {
		return report, err
	}
	report.RetiredTo = retired
	if err := appendJournal(plan.JournalPath, journalEntry{Item: "retire", Phase: "done", At: now.UTC(), Note: retired}); err != nil {
		return report, err
	}
	return report, nil
}

// inProgressFor reports whether an unfinished migration from legacyRoot is what
// filled the destination.
//
// Both halves are required. The plan must name this same source, so a finished
// migration from somewhere else cannot license overwriting a real library; and
// the journal must show work done but no retire, because a migration that
// reached retire is finished and anything at the destination afterwards is
// someone else's.
func inProgressFor(planPath, journalPath, legacyRoot string) (bool, error) {
	raw, err := os.ReadFile(planPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var recorded Plan
	if err := json.Unmarshal(raw, &recorded); err != nil {
		return false, fmt.Errorf("migration plan %s is unreadable: %w", planPath, err)
	}
	if !sameDir(recorded.LegacyRoot, legacyRoot) {
		return false, nil
	}
	done, err := completedItems(journalPath)
	if err != nil {
		return false, err
	}
	if done["retire"] {
		return false, nil
	}
	for item := range done {
		if item != "retire" {
			return true, nil
		}
	}
	return false, nil
}

type journalEntry struct {
	Item  string    `json:"item"`
	Phase string    `json:"phase"`
	At    time.Time `json:"at"`
	Note  string    `json:"note,omitempty"`
}

func appendJournal(path string, entry journalEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer handle.Close()
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := handle.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return handle.Sync()
}

// completedItems reads the journal so a restart resumes rather than repeats.
//
// An item with a begin and no done was interrupted. Its staging directory is
// removed and it is copied again, which is safe because the source was never
// touched.
func completedItems(path string) (map[string]bool, error) {
	done := map[string]bool{}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return done, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("migration journal %s is unreadable: %w", path, err)
		}
		if entry.Phase == "done" {
			done[entry.Item] = true
		}
	}
	return done, nil
}

// removeEmptyDirectory removes path only when it is a directory with nothing
// in it. Anything else is left exactly as it is.
func removeEmptyDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		return nil
	}
	return os.Remove(path)
}

func copyTree(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// A symlink in a library is not something to follow blindly into a new
		// root; it would either dangle or reach outside the tree being moved.
		return fmt.Errorf("refusing to migrate the symbolic link %s", source)
	}
	if !info.IsDir() {
		return copyFile(source, destination)
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyTree(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// verifyTree hashes every copied file against its source.
//
// The copy is only trustworthy if it is checked; a migration that reported
// success on a short write would be worse than none, because the user would
// then delete the original believing it redundant.
func verifyTree(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return verifyFile(source, destination)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := verifyTree(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func verifyFile(source, destination string) error {
	sourceSum, err := sha256File(source)
	if err != nil {
		return err
	}
	destinationSum, err := sha256File(destination)
	if err != nil {
		return err
	}
	if sourceSum != destinationSum {
		return fmt.Errorf("%w: %s hashes %s but its copy hashes %s", ErrVerificationFailed, source, sourceSum, destinationSum)
	}
	return nil
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

func measure(path string) (files int, bytes int64, present bool, err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	if !info.IsDir() {
		return 1, info.Size(), true, nil
	}
	err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		entryInfo, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		files++
		bytes += entryInfo.Size()
		return nil
	})
	if err != nil {
		return 0, 0, false, err
	}
	return files, bytes, true, nil
}

func isRegularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func sameDir(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	left, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	right, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
