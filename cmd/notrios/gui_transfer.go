//go:build gui

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/importers/joplinraw"
	"github.com/renesugar/notrios/internal/importers/obsidian"
	"github.com/renesugar/notrios/internal/jobs"
	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/store"
)

// Importing, exporting and taking snapshots reach the core through this bridge
// rather than over REST, because there is no REST route that starts them.
//
// api/openapi.yaml exposes /api/v1/jobs for listing, {job_id} for reading, and
// cancel, retry and reset -- and exactly one start, /api/v1/jobs/sync/start,
// described there as "the exception to watching-only" because its controls are
// closed and path-free. The four kinds here name a directory on a filesystem,
// and the jobs listing deliberately never returns their parameters for that
// reason. Adding an HTTP route that accepts an output path would be adding the
// surface the API has declined to have, so the desktop app uses the route it
// already has: a bound method in the process that owns the store.
//
// That is also what makes the browser case honest rather than cosmetic. The
// bridge is bound only when this process owns the service, so in -gui-only mode
// none of these methods exists and the frontend greys the controls out because
// the capability is genuinely absent, not because it was hidden.

// TransferReport is what one of these operations tells the interface.
//
// The summary is the importer's or exporter's own report, passed through rather
// than reshaped, so the window shows the same counts the command line prints
// and neither surface can drift into describing the work differently.
type TransferReport struct {
	Kind    string `json:"kind"`
	DryRun  bool   `json:"dry_run"`
	JobID   string `json:"job_id,omitempty"`
	Summary any    `json:"summary"`
}

var errNoLocalService = errors.New("this window is showing a service on another machine, so it cannot reach a directory here")

// directory checks the caller's path before any of it is used.
//
// The path arrives from the frontend, which in this mode is the person driving
// their own machine, so this is not a trust boundary. It is a diagnostic one: a
// typo reaching an importer surfaces as a failure deep inside a scan, where a
// clear refusal here says which directory was wrong.
func (b *NativeUIBridge) directory(path string) (string, error) {
	if b == nil || b.local == nil {
		return "", errNoLocalService
	}
	if path == "" {
		return "", errors.New("choose a folder first")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file; these operations name a folder", path)
	}
	return path, nil
}

// track opens a job record, runs the work, and settles the record either way.
//
// Only real runs are tracked. A dry run writes nothing and is fast, and a job
// list full of records for runs that changed nothing would be harder to read
// for no gain -- the same reasoning, and the same behaviour, as the command
// line.
func (b *NativeUIBridge) track(kind string, parameters []store.JobParameter,
	work func(context.Context, *jobs.Runner) (any, error)) (TransferReport, error) {
	runner, ctx, err := jobs.Start(context.Background(), b.local.Store, store.CreateJobRequest{
		Kind: kind, Parameters: parameters,
	})
	if err != nil {
		return TransferReport{}, err
	}
	summary, failure := work(ctx, runner)
	counts, _ := summary.(map[string]any)
	if _, settleErr := runner.Finish(counts, failure); settleErr != nil && failure == nil {
		failure = settleErr
	}
	if failure != nil {
		return TransferReport{}, failure
	}
	return TransferReport{Kind: kind, JobID: runner.ID(), Summary: summary}, nil
}

// ChooseDirectory opens the native directory dialog.
//
// It takes the purpose rather than a caller-supplied title so that the window
// never shows a string the frontend invented, and so that the four callers
// cannot each name the same dialog differently.
func (b *NativeUIBridge) ChooseDirectory(purpose string) (string, error) {
	titles := map[string]string{
		"sync":     "Choose a Notrios synchronization folder",
		"joplin":   "Choose the Joplin RAW export folder to import",
		"obsidian": "Choose the Obsidian vault to import",
		"export":   "Choose where to write the exported archive",
		"snapshot": "Choose where to write the snapshot",
	}
	title, known := titles[purpose]
	if !known {
		return "", fmt.Errorf("unknown directory purpose %q", purpose)
	}
	return b.chooseDirectory(title)
}

// ImportJoplinRaw scans, and on a real run imports, a Joplin RAW export.
func (b *NativeUIBridge) ImportJoplinRaw(path string, dryRun bool) (TransferReport, error) {
	source, err := b.directory(path)
	if err != nil {
		return TransferReport{}, err
	}
	options := joplinraw.Options{CollectionID: defaultCollection, BatchSize: defaultBatchSize}
	if dryRun {
		_, report, err := joplinraw.DryRun(context.Background(), b.local.Store, source, options)
		if err != nil {
			return TransferReport{}, err
		}
		return TransferReport{Kind: store.JobKindImportJoplinRaw, DryRun: true, Summary: report}, nil
	}
	return b.track(store.JobKindImportJoplinRaw, []store.JobParameter{
		{Name: "collection", Value: defaultCollection},
		{Name: "batch-size", Value: strconv.Itoa(defaultBatchSize)},
		{Name: "_source-dir", Value: source, Path: true},
	}, func(ctx context.Context, runner *jobs.Runner) (any, error) {
		options.AfterBatch = func(phase string, processed, total int) error {
			return runner.Progress(phase, processed, total)
		}
		report, err := joplinraw.Import(ctx, b.local.Store, source, options)
		return map[string]any{
			"notes_imported": report.NotesImported,
			"notes_updated":  report.NotesUpdated,
			"resources":      report.ResourcesImported,
		}, err
	})
}

// ImportObsidian scans, and on a real run imports, an Obsidian vault.
//
// The dry run writes no import configuration. The command line writes one into
// the vault so a person can edit the folder renames before the real import; a
// window that silently wrote a file into somebody's vault for looking at it
// would be a worse trade, and the configuration path stays a command-line
// affordance until the interface has somewhere to edit it.
func (b *NativeUIBridge) ImportObsidian(path string, dryRun bool) (TransferReport, error) {
	source, err := b.directory(path)
	if err != nil {
		return TransferReport{}, err
	}
	options := obsidian.Options{CollectionID: defaultCollection, BatchSize: defaultBatchSize}
	if dryRun {
		_, report, err := obsidian.DryRun(context.Background(), b.local.Store, source, options)
		if err != nil {
			return TransferReport{}, err
		}
		return TransferReport{Kind: store.JobKindImportObsidian, DryRun: true, Summary: report}, nil
	}
	return b.track(store.JobKindImportObsidian, []store.JobParameter{
		{Name: "collection", Value: defaultCollection},
		{Name: "batch-size", Value: strconv.Itoa(defaultBatchSize)},
		{Name: "_source-dir", Value: source, Path: true},
	}, func(ctx context.Context, runner *jobs.Runner) (any, error) {
		options.AfterBatch = func(phase string, processed, total int) error {
			return runner.Progress(phase, processed, total)
		}
		report, err := obsidian.Import(ctx, b.local.Store, source, options)
		return map[string]any{
			"notes_imported": report.NotesImported,
			"notes_updated":  report.NotesUpdated,
			"resources":      report.ResourcesImported,
		}, err
	})
}

// ExportArchive writes a complete archive-v2 export into the chosen folder.
//
// Only the full archive is offered here. The subset selectors the command line
// carries -- notebooks, tags, a query, explicit documents -- are a selection
// problem, and offering them as blank fields with no way to see what they
// select would produce exports nobody can predict. They belong with the search
// work that can show the selection first.
func (b *NativeUIBridge) ExportArchive(path string) (TransferReport, error) {
	destination, err := b.directory(path)
	if err != nil {
		return TransferReport{}, err
	}
	return b.track(store.JobKindExportArchiveV2, []store.JobParameter{
		{Name: "collection", Value: defaultCollection},
		{Name: "target", Value: "full_archive"},
		{Name: "_out-dir", Value: destination, Path: true},
	}, func(ctx context.Context, _ *jobs.Runner) (any, error) {
		report, err := archivev2.Export(ctx, b.local.Store, destination, archivev2.ExportOptions{
			Target:    "full_archive",
			Selection: store.SelectionSpec{CollectionID: defaultCollection, Match: "any"},
		})
		return map[string]any{
			"documents": report.SelectedDocuments,
			"objects":   report.Objects,
		}, err
	})
}

// CreateSnapshot writes a verified snapshot image into the chosen folder.
func (b *NativeUIBridge) CreateSnapshot(path string) (TransferReport, error) {
	destination, err := b.directory(path)
	if err != nil {
		return TransferReport{}, err
	}
	return b.track(store.JobKindSnapshotImage, []store.JobParameter{
		{Name: "_out-dir", Value: destination, Path: true},
	}, func(ctx context.Context, _ *jobs.Runner) (any, error) {
		report, err := snapshotimage.Create(ctx, b.local.Store, b.local.Config.Data.AssetStore,
			destination, snapshotimage.CreateOptions{})
		// Recorded only when the snapshot verified, exactly as the command line
		// does it: an unverified image must not become the thing a later
		// catch-up trusts.
		if err == nil && report.Verified {
			createdAt, parseErr := time.Parse(time.RFC3339Nano, report.CreatedAt)
			if parseErr != nil {
				err = parseErr
			} else {
				err = b.local.Store.RecordVerifiedSyncSnapshot(ctx, store.VerifiedSyncSnapshot{
					SnapshotID: report.SnapshotID, CommitSHA256: report.CommitSHA256,
					CreatedAt: createdAt, Vector: report.SnapshotVector,
				})
			}
		}
		return map[string]any{
			"snapshot_id": report.SnapshotID,
			"objects":     report.Objects,
			"verified":    report.Verified,
		}, err
	})
}

// VerifyArchive checks a Notrios archive without opening a database.
//
// This is the dry run for importing from Notrios, and it is a better one than
// the importers get: archivev2.VerifyDirectory reads the archive and nothing
// else, so a person can find out whether an archive is sound before deciding
// anything. Restore runs the same verification again and refuses to write if it
// fails, so choosing not to look first is safe -- looking is for the person, not
// for the machine.
func (b *NativeUIBridge) VerifyArchive(path string) (TransferReport, error) {
	root, err := b.directory(path)
	if err != nil {
		return TransferReport{}, err
	}
	report, err := archivev2.VerifyDirectory(root, archivev2.DefaultLimits())
	if err != nil {
		return TransferReport{}, err
	}
	return TransferReport{Kind: "verify_archive_v2", DryRun: true, Summary: report}, nil
}

// ImportArchive merges a Notrios archive into this library.
//
// Merge is the only intent offered here, and that is a deliberate narrowing.
// archivev2 has four -- replace, merge, fork and adopt -- and three of them are
// database-universe surgery: replace overwrites this library with the archive's
// identity, adopt requires an empty target, fork mints a new database. Merge
// keeps this library's identity and admits the archive's records as foreign,
// which is the only one that means what "import from another Notrios library"
// sounds like. A dropdown offering all four next to a folder chooser would let
// somebody replace their library while believing they were adding to it.
//
// There is no job record because there is no job kind for a restore, and the
// command line does not create one either. Verification happens inside Restore
// before the first write, so a failed archive leaves nothing behind.
func (b *NativeUIBridge) ImportArchive(path string) (TransferReport, error) {
	root, err := b.directory(path)
	if err != nil {
		return TransferReport{}, err
	}
	summary, err := archivev2.Restore(context.Background(), b.local.Store, root, archivev2.RestoreOptions{
		Intent: archivev2.RestoreMerge,
	})
	if err != nil {
		return TransferReport{}, err
	}
	return TransferReport{Kind: "import_archive_v2", Summary: summary}, nil
}

const (
	defaultCollection = "default"
	defaultBatchSize  = 100
)
