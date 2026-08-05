package archivev2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// RestoreOptions describes one restore. Intent is mandatory: P2 defines what
// each choice does to the database universe, and there is deliberately no
// default.
type RestoreOptions struct {
	Intent        RestoreIntent
	NewDatabaseID string
	Limits        Limits
	// BatchSize bounds how many records are applied per transaction.
	BatchSize int
}

const defaultRestoreBatch = 500

// Restore admits a verified archive into a canonical store. Verification
// completes in full before the first canonical write, so a corrupt or
// inconsistent archive can never leave a partial restore behind.
func Restore(ctx context.Context, target store.RestoreTarget, root string, options RestoreOptions) (store.RestoreSummary, error) {
	started := time.Now()
	if target == nil {
		return store.RestoreSummary{}, fmt.Errorf("a restore target is required")
	}
	if options.Limits == (Limits{}) {
		options.Limits = DefaultLimits()
	}
	if options.BatchSize <= 0 {
		options.BatchSize = defaultRestoreBatch
	}
	if _, err := VerifyDirectory(root, options.Limits); err != nil {
		return store.RestoreSummary{}, fmt.Errorf("archive failed verification; nothing was written: %w", err)
	}
	manifest, err := readManifestFile(root, options.Limits)
	if err != nil {
		return store.RestoreSummary{}, err
	}

	identity, err := target.GetDatabaseIdentity(ctx)
	if err != nil {
		return store.RestoreSummary{}, err
	}
	empty, err := target.LibraryIsEmpty(ctx)
	if err != nil {
		return store.RestoreSummary{}, err
	}
	request := RestoreIdentityRequest{Intent: options.Intent, TargetEmpty: empty, NewDatabaseID: options.NewDatabaseID}
	if !empty {
		request.TargetDatabaseID = identity.DatabaseID
	}
	decision, err := PlanRestoreIdentity(manifest, request)
	if err != nil {
		return store.RestoreSummary{}, err
	}

	run := &restoreRun{
		target: target, root: root, options: options, manifest: manifest,
		additive: decision.Intent == RestoreMerge,
		source:   newPackSource(options.Limits),
	}
	defer run.source.close()
	if err := run.buildObjectIndex(ctx); err != nil {
		return store.RestoreSummary{}, err
	}
	defer run.closeObjectIndex()

	if decision.Intent == RestoreReplace {
		if err := target.ClearLibraryForReplace(ctx); err != nil {
			return store.RestoreSummary{}, err
		}
	}
	if err := run.applyContainers(ctx); err != nil {
		return store.RestoreSummary{}, err
	}
	if err := run.applyContent(ctx); err != nil {
		return store.RestoreSummary{}, err
	}
	if err := run.applyRelations(ctx); err != nil {
		return store.RestoreSummary{}, err
	}
	if err := target.FinalizeRestoredDocuments(ctx); err != nil {
		return store.RestoreSummary{}, err
	}

	summary := store.RestoreSummary{
		Intent:    string(decision.Intent),
		Applied:   run.counts,
		Conflicts: run.conflicts,
		BlobBytes: run.blobBytes,
		Blobs:     run.blobs,
		Warnings:  run.warnings,
		SourceBundle: store.SourceBundleNote{
			Restored: run.counts.SourceBundles, KeysAreHash: run.counts.SourceBundles > 0,
			Note: "archives carry hashed source and item keys so local paths never travel; restored bundles keep exact bytes but cannot reproduce the original importer keys, so importer resume against the original source directory will not match",
		},
	}
	if decision.MintNewReplicaID || decision.PreserveArchiveUniverse {
		updated, err := target.AdoptDatabaseIdentity(ctx, decision.ResultDatabaseID)
		if err != nil {
			return store.RestoreSummary{}, err
		}
		summary.DatabaseID, summary.ReplicaID = updated.DatabaseID, updated.ReplicaID
	} else {
		summary.DatabaseID, summary.ReplicaID = identity.DatabaseID, identity.ReplicaID
	}
	summary.Elapsed = time.Since(started).Seconds()
	return summary, nil
}

type restoreRun struct {
	target    store.RestoreTarget
	root      string
	options   RestoreOptions
	manifest  Manifest
	additive  bool
	source    *packSource
	counts    store.RestoreCounts
	conflicts []store.RestoreConflict
	warnings  []string
	blobs     int
	blobBytes int64
	admitted  map[string]bool
	objects   *store.ImportManifest
}

// forEachRecord streams every record of the wanted types. Records objects are a
// small fraction of an archive's bytes, so restoring in dependency order with
// more than one pass is cheap next to reading blobs once.
func (r *restoreRun) forEachRecord(wanted map[string]bool, visit func(RecordEnvelope) error) error {
	for _, indexObject := range r.manifest.Index {
		file, err := openRegular(r.root, indexObject.Location.Path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 16<<10), r.options.Limits.MaxIndexEntryBytes)
		entries := []IndexEntry{}
		for scanner.Scan() {
			var entry IndexEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				_ = file.Close()
				return err
			}
			if entry.Kind == "records" {
				entries = append(entries, entry)
			}
		}
		closeErr := file.Close()
		if err := firstError(scanner.Err(), closeErr); err != nil {
			return err
		}
		for _, entry := range entries {
			if err := r.readRecordObject(entry, wanted, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *restoreRun) readRecordObject(entry IndexEntry, wanted map[string]bool, visit func(RecordEnvelope) error) error {
	reader, closer, err := r.source.open(r.root, entry)
	if err != nil {
		return err
	}
	defer closer()
	scanner := bufio.NewScanner(io.NewSectionReader(reader, 0, entry.SizeBytes))
	scanner.Buffer(make([]byte, 64<<10), r.options.Limits.MaxRecordBytes)
	for scanner.Scan() {
		var envelope RecordEnvelope
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			return err
		}
		if !wanted[envelope.Type] {
			continue
		}
		if err := visit(envelope); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// applyContainers writes collections, notebooks, search notebooks, and tags.
// All four are bounded by format limits, so they are collected before writing;
// notebooks are ordered parent-first so a child never precedes its parent.
func (r *restoreRun) applyContainers(ctx context.Context) error {
	batch := store.RestoreRecords{}
	notebooks := map[string]store.Notebook{}
	err := r.forEachRecord(map[string]bool{
		RecordCollection: true, RecordNotebook: true, RecordSearchNotebook: true, RecordTag: true,
	}, func(envelope RecordEnvelope) error {
		switch envelope.Type {
		case RecordCollection:
			var record CollectionRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			batch.Collections = append(batch.Collections, store.ExportCollection{
				ID: record.ID, Name: record.Name, Description: record.Description,
				CreatedAt: parseArchiveTime(record.CreatedAt),
			})
		case RecordNotebook:
			var record NotebookRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			notebooks[record.ID] = store.Notebook{
				ID: record.ID, ParentID: record.ParentID, Name: record.Name, IconEmoji: record.IconEmoji,
				Builtin: record.Builtin, Position: record.Position,
				CreatedAt: parseArchiveTime(record.CreatedAt), UpdatedAt: parseArchiveTime(record.UpdatedAt),
			}
		case RecordSearchNotebook:
			var record SearchNotebookRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			batch.SearchNotebooks = append(batch.SearchNotebooks, store.SearchNotebook{
				ID: record.ID, Name: record.Name, IconEmoji: record.IconEmoji, Query: record.Query,
				Builtin: record.Builtin, SortAnchor: record.SortAnchor, CreatedAt: parseArchiveTime(record.CreatedAt),
			})
		case RecordTag:
			var record TagRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			batch.Tags = append(batch.Tags, store.ExportTag{ID: record.ID, Name: record.Name, CreatedAt: parseArchiveTime(record.CreatedAt)})
		}
		return nil
	})
	if err != nil {
		return err
	}
	batch.Notebooks = orderNotebooksParentFirst(notebooks)
	return r.apply(ctx, batch)
}

// orderNotebooksParentFirst sorts by tree depth so a child is never written
// before the parent it references.
func orderNotebooksParentFirst(notebooks map[string]store.Notebook) []store.Notebook {
	depth := func(id string) int {
		steps := 0
		for current := id; current != ""; steps++ {
			parent := notebooks[current].ParentID
			if parent == "" || steps > 64 {
				break
			}
			current = parent
		}
		return steps
	}
	ordered := make([]store.Notebook, 0, len(notebooks))
	for _, notebook := range notebooks {
		ordered = append(ordered, notebook)
	}
	sort.Slice(ordered, func(i, j int) bool {
		left, right := depth(ordered[i].ID), depth(ordered[j].ID)
		if left != right {
			return left < right
		}
		return ordered[i].ID < ordered[j].ID
	})
	return ordered
}

// applyContent writes resources and their blobs, then documents and revisions
// with their body blobs.
func (r *restoreRun) applyContent(ctx context.Context) error {
	r.admitted = map[string]bool{}
	batch := store.RestoreRecords{}
	flush := func() error {
		if err := r.apply(ctx, batch); err != nil {
			return err
		}
		batch = store.RestoreRecords{}
		return nil
	}
	err := r.forEachRecord(map[string]bool{RecordResource: true}, func(envelope RecordEnvelope) error {
		var record ResourceRecord
		if err := json.Unmarshal(envelope.Payload, &record); err != nil {
			return err
		}
		if err := r.admitBlob(ctx, record.Blob); err != nil {
			return fmt.Errorf("resource %s: %w", record.ID, err)
		}
		batch.Resources = append(batch.Resources, store.ExportResource{
			ID: record.ID, CollectionID: record.CollectionID, BlobSHA256: record.Blob.SHA256,
			BlobSizeBytes: record.Blob.SizeBytes, Filename: record.Filename, MIMEType: record.MIMEType,
			MetadataJSON: string(record.MetadataJSON), UnreferencedAt: parseArchiveTime(record.UnreferencedAt),
			UnreferencedReason: record.UnreferencedReason, CreatedAt: parseArchiveTime(record.CreatedAt),
		})
		if len(batch.Resources) >= r.options.BatchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}

	err = r.forEachRecord(map[string]bool{RecordDocument: true}, func(envelope RecordEnvelope) error {
		var record DocumentRecord
		if err := json.Unmarshal(envelope.Payload, &record); err != nil {
			return err
		}
		batch.Documents = append(batch.Documents, store.RestoreDocument{
			Document: store.ExportDocument{
				ID: record.ID, CollectionID: record.CollectionID, NotebookID: record.NotebookID,
				CurrentRevisionID: record.CurrentRevisionID, DeletedAt: parseArchiveTime(record.DeletedAt),
				CreatedAt: parseArchiveTime(record.CreatedAt), UpdatedAt: parseArchiveTime(record.UpdatedAt),
			},
		})
		if len(batch.Documents) >= r.options.BatchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}

	err = r.forEachRecord(map[string]bool{RecordRevision: true}, func(envelope RecordEnvelope) error {
		var record RevisionRecord
		if err := json.Unmarshal(envelope.Payload, &record); err != nil {
			return err
		}
		body, err := r.readBlobText(record.Body)
		if err != nil {
			return fmt.Errorf("revision %s body: %w", record.ID, err)
		}
		batch.Revisions = append(batch.Revisions, store.ExportRevision{
			ID: record.ID, DocumentID: record.DocumentID, Title: record.Title, Body: body,
			BodyMIMEType: record.BodyMIMEType, MetadataJSON: string(record.MetadataJSON),
			Message: record.Message, CreatedAt: parseArchiveTime(record.CreatedAt),
		})
		if len(batch.Revisions) >= r.options.BatchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
}

// applyRelations writes the rows that reference documents and resources, plus
// exact source bundles and their bytes.
func (r *restoreRun) applyRelations(ctx context.Context) error {
	batch := store.RestoreRecords{}
	count := 0
	flush := func() error {
		if err := r.apply(ctx, batch); err != nil {
			return err
		}
		batch = store.RestoreRecords{}
		count = 0
		return nil
	}
	err := r.forEachRecord(map[string]bool{
		RecordDocumentTag: true, RecordDocumentResource: true, RecordLink: true,
		RecordProvenance: true, RecordSourceBundle: true,
	}, func(envelope RecordEnvelope) error {
		switch envelope.Type {
		case RecordDocumentTag:
			var record DocumentTagRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			batch.DocumentTags = append(batch.DocumentTags, store.ExportDocumentTag{DocumentID: record.DocumentID, TagID: record.TagID})
		case RecordDocumentResource:
			var record DocumentResourceRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			batch.DocumentResources = append(batch.DocumentResources, store.ExportDocumentResource{
				DocumentID: record.DocumentID, ResourceID: record.ResourceID,
				RelationType: record.RelationType, Ordinal: record.Ordinal, AnchorJSON: string(record.AnchorJSON),
			})
		case RecordLink:
			var record LinkRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			batch.Links = append(batch.Links, store.DocumentLink{
				SourceDocumentID: record.SourceDocumentID, TargetDocumentID: record.TargetDocumentID,
				TargetResourceID: record.TargetResourceID, TargetURI: record.TargetURI,
				RelationType: record.RelationType, SourceFormat: record.SourceFormat, RawTarget: record.RawTarget,
				DisplayText: record.DisplayText, AnchorType: record.AnchorType, AnchorValue: record.AnchorValue,
				Context: record.Context, SourceStartByte: record.SourceStartByte, SourceEndByte: record.SourceEndByte,
				SourceLine: record.SourceLine, SourceColumn: record.SourceColumn, ResolutionStatus: record.ResolutionStatus,
			})
		case RecordProvenance:
			var record ProvenanceRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			source := store.DocumentSource{
				DocumentID: record.DocumentID, SourceSystem: record.SourceSystem, ExternalID: record.ExternalID,
				Author: record.Author, AuthorID: record.AuthorID, ThreadID: record.ThreadID, ReplyTo: record.ReplyTo,
				SourceURL: record.SourceURL, PublishedAt: record.PublishedAt, MetadataJSON: string(record.MetadataJSON),
				CreatedAt: parseArchiveTime(record.CreatedAt), UpdatedAt: parseArchiveTime(record.UpdatedAt),
			}
			if record.PublishedTS != nil {
				source.PublishedTS = *record.PublishedTS
			}
			batch.Provenance = append(batch.Provenance, source)
		case RecordSourceBundle:
			var record SourceBundleRecord
			if err := json.Unmarshal(envelope.Payload, &record); err != nil {
				return err
			}
			if err := r.admitBlob(ctx, record.Content); err != nil {
				return fmt.Errorf("source bundle %s: %w", record.ItemKeySHA256, err)
			}
			var order []string
			_ = json.Unmarshal(record.PropertyOrderJSON, &order)
			batch.SourceBundles = append(batch.SourceBundles, store.SourceBundleItem{
				SourceSystem: record.SourceSystem,
				// Raw keys are deliberately not archived; the hashes stand in.
				SourceKey: record.SourceKeySHA256, CollectionID: record.CollectionID,
				ItemKey: record.ItemKeySHA256, ItemType: record.ItemType, ExternalID: record.ExternalID,
				RelativePath: record.RelativePath, SHA256: record.Content.SHA256,
				SizeBytes: record.Content.SizeBytes, StoragePath: sourceBundleStoragePath(record.Content.SHA256),
				PropertyOrder: order, UpdatedAt: parseArchiveTime(record.UpdatedAt),
			})
		}
		count++
		if count >= r.options.BatchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
}

func (r *restoreRun) apply(ctx context.Context, batch store.RestoreRecords) error {
	conflicts, err := r.target.ApplyRestoreRecords(ctx, batch, r.additive)
	if err != nil {
		return err
	}
	r.counts.AddBatch(batch)
	r.conflicts = append(r.conflicts, conflicts...)
	return nil
}

// admitBlob streams one archive object into the asset store exactly once.
func (r *restoreRun) admitBlob(ctx context.Context, reference BlobReference) error {
	if r.admitted == nil {
		r.admitted = map[string]bool{}
	}
	if r.admitted[reference.SHA256] {
		return nil
	}
	entry, err := r.findObject(reference.SHA256)
	if err != nil {
		return err
	}
	reader, closer, err := r.source.open(r.root, entry)
	if err != nil {
		return err
	}
	defer closer()
	size, err := r.target.AdmitRestoredBlob(ctx, reference.SHA256, reference.MediaType,
		io.NewSectionReader(reader, 0, entry.SizeBytes))
	if err != nil {
		return err
	}
	r.admitted[reference.SHA256] = true
	r.blobs++
	r.blobBytes += size
	return nil
}

func (r *restoreRun) readBlobText(reference BlobReference) (string, error) {
	entry, err := r.findObject(reference.SHA256)
	if err != nil {
		return "", err
	}
	reader, closer, err := r.source.open(r.root, entry)
	if err != nil {
		return "", err
	}
	defer closer()
	var builder strings.Builder
	if _, err := io.Copy(&builder, io.NewSectionReader(reader, 0, entry.SizeBytes)); err != nil {
		return "", err
	}
	return builder.String(), nil
}

// buildObjectIndex reads the index once into a temporary indexed spool so an
// object lookup is an indexed query rather than a scan.
//
// The first implementation scanned every index chunk per lookup. That is
// quadratic — roughly 112,000 blob lookups against 1.1M index entries on the
// attachment corpus — and it made restore unusable at real scale while looking
// deceptively "bounded" because it held no map.
func (r *restoreRun) buildObjectIndex(ctx context.Context) error {
	spool, err := store.OpenImportManifest()
	if err != nil {
		return err
	}
	r.objects = spool
	batch := make([]store.ImportManifestRecord, 0, objectIndexBatch)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := r.objects.Put(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}
	for _, indexObject := range r.manifest.Index {
		file, openErr := openRegular(r.root, indexObject.Location.Path)
		if openErr != nil {
			return openErr
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 16<<10), r.options.Limits.MaxIndexEntryBytes)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var entry IndexEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				_ = file.Close()
				return err
			}
			batch = append(batch, store.ImportManifestRecord{
				Kind: "object", SortKey: entry.SHA256, LookupKey: entry.SHA256, Payload: line,
			})
			if len(batch) >= objectIndexBatch {
				if err := flush(); err != nil {
					_ = file.Close()
					return err
				}
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if err := firstError(scanErr, closeErr); err != nil {
			return err
		}
	}
	return flush()
}

// objectIndexBatch stays inside the spool's per-call bound.
const objectIndexBatch = 500

func (r *restoreRun) closeObjectIndex() {
	if r.objects != nil {
		_ = r.objects.Close()
		r.objects = nil
	}
}

// findObject resolves one index entry by hash through the indexed spool.
func (r *restoreRun) findObject(hash string) (IndexEntry, error) {
	found, err := r.objects.Lookup(context.Background(), "object", []string{hash})
	if err != nil {
		return IndexEntry{}, err
	}
	records := found[hash]
	if len(records) == 0 {
		return IndexEntry{}, fmt.Errorf("archive does not contain object %s", hash)
	}
	var entry IndexEntry
	if err := json.Unmarshal(records[0].Payload, &entry); err != nil {
		return IndexEntry{}, err
	}
	return entry, nil
}

func sourceBundleStoragePath(hash string) string {
	return "source-bundles/sha256/" + hash[0:2] + "/" + hash[2:4] + "/" + hash
}

func parseArchiveTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
