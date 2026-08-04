package archivev2

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// stagingSuffix names the private directory that holds in-progress object and
// manifest writes. It is a sibling of the archive so the archive directory
// itself never contains a temporary file, which the format contract forbids.
const stagingSuffix = ".staging"

// copyBufferBytes bounds every streamed body, resource, and source-bundle copy.
const copyBufferBytes = 64 << 10

// ExportOptions describes one archive-v2 export. The zero value exports a
// complete full-database backup of the default collection.
type ExportOptions struct {
	// Target is full_archive (default) or subset_transfer. publication_handoff
	// belongs to the reviewed publishing slice and is refused here.
	Target string
	// Selection and Policy are passed to the shared P1 planner unchanged.
	Selection store.SelectionSpec
	Policy    store.PrivacyPolicy
	// MaxDocuments bounds the selection; zero uses the planner default.
	MaxDocuments int
	// Limits may tighten, but never widen, the format admission bounds.
	Limits Limits
	// RecordsPerObject bounds one JSONL chunk; zero uses the format maximum,
	// which keeps the object count as low as the limits allow.
	RecordsPerObject int
	// SnapshotID and CreatedAt exist so fixtures and determinism tests can pin
	// the only two otherwise time/random-derived manifest fields.
	SnapshotID string
	CreatedAt  time.Time
	// Overwrite replaces an existing complete archive in the destination.
	Overwrite bool
	// SkipVerification disables the read-only verification pass that normally
	// runs before the export is reported as successful.
	SkipVerification bool
}

// ExportReport is the aggregate, content-free result of one export.
type ExportReport struct {
	Target                  string   `json:"target"`
	FullBackup              bool     `json:"full_backup"`
	SnapshotID              string   `json:"snapshot_id"`
	DatabaseID              string   `json:"database_id"`
	SourceReplicaID         string   `json:"source_replica_id"`
	CreatedAt               string   `json:"created_at"`
	CommitSHA256            string   `json:"commit_sha256"`
	SelectionManifestSHA256 string   `json:"selection_manifest_sha256"`
	CollectionIDs           []string `json:"collection_ids"`
	Counts                  Counts   `json:"counts"`
	Objects                 int      `json:"objects"`
	RecordObjects           int      `json:"record_objects"`
	BlobObjects             int      `json:"blob_objects"`
	DeduplicatedObjects     int      `json:"deduplicated_objects"`
	ReusedObjects           int      `json:"reused_objects"`
	Bytes                   int64    `json:"bytes"`
	BytesWritten            int64    `json:"bytes_written"`
	SelectedDocuments       int      `json:"selected_documents"`
	ExcludedDocuments       int      `json:"excluded_documents"`
	ReachableResources      int      `json:"reachable_resources"`
	ClearedLinkTargets      int      `json:"cleared_link_targets"`
	Verified                bool     `json:"verified"`
	Warnings                []string `json:"warnings"`
	ElapsedSeconds          float64  `json:"elapsed_seconds"`
}

// Export streams one transactionally consistent snapshot of the canonical
// store into an archive-v2 directory. Objects are published first and the
// manifest last, so an interrupted export always leaves an archive the
// verifier reports as incomplete rather than one that looks finished.
func Export(ctx context.Context, source store.ExportReader, destination string, options ExportOptions) (ExportReport, error) {
	started := time.Now()
	if source == nil {
		return ExportReport{}, fmt.Errorf("an export source is required")
	}
	options, err := normalizeExportOptions(options)
	if err != nil {
		return ExportReport{}, err
	}
	root, staging, err := prepareDestination(destination, options.Overwrite)
	if err != nil {
		return ExportReport{}, err
	}
	defer os.RemoveAll(staging)

	writer := newArchiveWriter(root, staging, options)
	export := &exportRun{source: source, options: options, writer: writer}
	if err := source.WithReadSnapshot(ctx, func() error { return export.collect(ctx) }); err != nil {
		return ExportReport{}, err
	}
	manifest, err := export.buildManifest()
	if err != nil {
		return ExportReport{}, err
	}
	if err := writer.pruneUnlistedObjects(); err != nil {
		return ExportReport{}, err
	}
	if err := writer.publishManifest(manifest); err != nil {
		return ExportReport{}, err
	}

	report := export.report(manifest, started)
	if !options.SkipVerification {
		if _, err := VerifyDirectory(root, options.Limits); err != nil {
			// A published manifest is the completion marker, so an archive that
			// fails its own verification must not keep one.
			_ = os.Remove(filepath.Join(root, "manifest.json"))
			return ExportReport{}, fmt.Errorf("published archive failed verification: %w", err)
		}
		report.Verified = true
	}
	report.ElapsedSeconds = time.Since(started).Seconds()
	return report, nil
}

func normalizeExportOptions(options ExportOptions) (ExportOptions, error) {
	options.Target = strings.ToLower(strings.TrimSpace(options.Target))
	if options.Target == "" {
		options.Target = TargetFullArchive
	}
	switch options.Target {
	case TargetFullArchive, TargetSubsetTransfer:
	case TargetPublicationHandoff:
		return ExportOptions{}, fmt.Errorf("publication_handoff exports require the reviewed publication profile slice; use full_archive or subset_transfer")
	default:
		return ExportOptions{}, fmt.Errorf("unsupported export target %q", options.Target)
	}
	if action := strings.ToLower(strings.TrimSpace(options.Policy.LinkAction)); action == "plain_text" || action == "redact" {
		// Rewriting link syntax means rewriting note bodies, which belongs to
		// the publication projection rather than to a restore-fidelity archive.
		return ExportOptions{}, fmt.Errorf("link_action %q rewrites note content and is not available to archive export", action)
	}
	if options.Limits == (Limits{}) {
		options.Limits = DefaultLimits()
	}
	if err := validateLimits(options.Limits); err != nil {
		return ExportOptions{}, err
	}
	if options.RecordsPerObject <= 0 {
		options.RecordsPerObject = options.Limits.MaxRecordsPerObject
	}
	if options.RecordsPerObject > options.Limits.MaxRecordsPerObject {
		return ExportOptions{}, fmt.Errorf("records_per_object must be %d or less", options.Limits.MaxRecordsPerObject)
	}
	if options.SnapshotID == "" {
		id, err := store.NewID("snap")
		if err != nil {
			return ExportOptions{}, err
		}
		options.SnapshotID = id
	}
	if !validID(options.SnapshotID) {
		return ExportOptions{}, fmt.Errorf("snapshot ID %q is not a bounded opaque ID", options.SnapshotID)
	}
	if options.CreatedAt.IsZero() {
		options.CreatedAt = time.Now()
	}
	options.CreatedAt = options.CreatedAt.UTC().Truncate(time.Second)
	return options, nil
}

// prepareDestination refuses to write into a directory that is not an archive,
// removes any manifest left by a previous complete export when overwriting,
// and returns a clean private staging directory.
func prepareDestination(destination string, overwrite bool) (root, staging string, err error) {
	if strings.TrimSpace(destination) == "" {
		return "", "", fmt.Errorf("an archive destination directory is required")
	}
	root, err = filepath.Abs(destination)
	if err != nil {
		return "", "", err
	}
	staging = root + stagingSuffix
	entries, err := os.ReadDir(root)
	switch {
	case err == nil:
		manifestPresent := false
		for _, entry := range entries {
			name := entry.Name()
			if name == "manifest.json" && entry.Type().IsRegular() {
				manifestPresent = true
				continue
			}
			if name == "objects" && entry.IsDir() {
				continue
			}
			return "", "", fmt.Errorf("destination %q is not an archive-v2 directory (unexpected entry %q)", destination, name)
		}
		if manifestPresent && !overwrite {
			return "", "", fmt.Errorf("destination %q already holds a complete archive; pass overwrite to replace it", destination)
		}
		if manifestPresent {
			// Drop the completion marker first: while objects are being
			// rewritten the directory must not claim to be complete.
			if err := os.Remove(filepath.Join(root, "manifest.json")); err != nil {
				return "", "", err
			}
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(root, 0o755); err != nil {
			return "", "", err
		}
	default:
		return "", "", err
	}
	if err := os.RemoveAll(staging); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return "", "", err
	}
	return root, staging, nil
}

// archiveWriter owns object publication. Every object is content-addressed, so
// writing is idempotent: an object already present from an interrupted run is
// reused after its size is confirmed instead of being rewritten.
type archiveWriter struct {
	root    string
	staging string
	options ExportOptions

	objects       map[string]Object
	order         []string
	totalBytes    int64
	writtenBytes  int64
	reused        int
	deduplicated  int
	recordObjects int
	blobObjects   int

	buffer       bytes.Buffer
	bufferRecs   int
	bufferCounts Counts
	counts       Counts
	sequence     int
}

func newArchiveWriter(root, staging string, options ExportOptions) *archiveWriter {
	return &archiveWriter{root: root, staging: staging, options: options, objects: map[string]Object{}}
}

// addRecord appends one typed record to the open JSONL chunk and flushes the
// chunk when it reaches the configured record or byte bound.
func (w *archiveWriter) addRecord(recordType string, payload any) error {
	counts, ok := countForRecordType(recordType)
	if !ok {
		return fmt.Errorf("unsupported record type %q", recordType)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s record: %w", recordType, err)
	}
	line, err := json.Marshal(RecordEnvelope{Type: recordType, Payload: raw})
	if err != nil {
		return fmt.Errorf("encode %s envelope: %w", recordType, err)
	}
	if len(line) > w.options.Limits.MaxRecordBytes {
		return fmt.Errorf("one %s record is %d bytes, above the %d byte limit", recordType, len(line), w.options.Limits.MaxRecordBytes)
	}
	if w.bufferRecs > 0 && (w.bufferRecs >= w.options.RecordsPerObject || int64(w.buffer.Len()+len(line)+1) > w.options.Limits.MaxRecordObjectBytes) {
		if err := w.flushRecords(); err != nil {
			return err
		}
	}
	w.buffer.Write(line)
	w.buffer.WriteByte('\n')
	w.bufferRecs++
	w.bufferCounts.Add(counts)
	w.counts.Add(counts)
	if w.counts.Total() > w.options.Limits.MaxRecords {
		return fmt.Errorf("archive holds more than %d records", w.options.Limits.MaxRecords)
	}
	return nil
}

func (w *archiveWriter) flushRecords() error {
	if w.bufferRecs == 0 {
		return nil
	}
	payload := append([]byte(nil), w.buffer.Bytes()...)
	records, counts := w.bufferRecs, w.bufferCounts
	w.buffer.Reset()
	w.bufferRecs, w.bufferCounts = 0, Counts{}
	object, deduplicated, err := w.writeObject(bytes.NewReader(payload), "records", RecordsMediaType)
	if err != nil {
		return err
	}
	if deduplicated {
		// Every chunk carries unique record identities, so two chunks can only
		// share a hash if the traversal emitted the same records twice.
		return fmt.Errorf("records object %s was produced twice", object.SHA256)
	}
	descriptor := w.objects[object.SHA256]
	descriptor.Records = records
	descriptor.RecordCounts = counts
	w.objects[object.SHA256] = descriptor
	w.recordObjects++
	return nil
}

// writeBlob streams content into an immutable object and returns the reference
// records use. Identical bytes are stored exactly once.
func (w *archiveWriter) writeBlob(content io.Reader, mediaType string) (BlobReference, error) {
	canonical, err := normalizeMediaType(mediaType)
	if err != nil {
		canonical = "application/octet-stream"
	}
	object, deduplicated, err := w.writeObject(content, "blob", canonical)
	if err != nil {
		return BlobReference{}, err
	}
	if !deduplicated {
		w.blobObjects++
	}
	return BlobReference{SHA256: object.SHA256, SizeBytes: object.SizeBytes, MediaType: canonical}, nil
}

func (w *archiveWriter) writeObject(content io.Reader, kind, mediaType string) (Object, bool, error) {
	w.sequence++
	temporary := filepath.Join(w.staging, "object-"+strconv.Itoa(w.sequence))
	file, err := os.Create(temporary)
	if err != nil {
		return Object{}, false, err
	}
	digest := sha256.New()
	buffered := bufio.NewWriterSize(file, copyBufferBytes)
	size, copyErr := io.CopyBuffer(io.MultiWriter(buffered, digest), content, make([]byte, copyBufferBytes))
	if copyErr == nil {
		copyErr = buffered.Flush()
	}
	if copyErr == nil {
		copyErr = file.Sync()
	}
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(temporary)
		return Object{}, false, firstError(copyErr, closeErr)
	}
	hash := hex.EncodeToString(digest.Sum(nil))
	if existing, ok := w.objects[hash]; ok {
		_ = os.Remove(temporary)
		if existing.SizeBytes != size || existing.Kind != kind {
			return Object{}, false, fmt.Errorf("object %s was published with a different size or kind", hash)
		}
		w.deduplicated++
		return existing, true, nil
	}
	if size > w.options.Limits.MaxBlobBytes || w.totalBytes > w.options.Limits.MaxTotalBytes-size {
		_ = os.Remove(temporary)
		return Object{}, false, fmt.Errorf("object or archive byte limit exceeded")
	}
	if len(w.objects) >= w.options.Limits.MaxObjects {
		_ = os.Remove(temporary)
		return Object{}, false, fmt.Errorf("archive needs more than %d immutable objects; archive-v2 currently bounds one archive to that many bodies, resources, source bundles, and record chunks", w.options.Limits.MaxObjects)
	}

	relative := objectPath(hash)
	final := filepath.Join(w.root, filepath.FromSlash(relative))
	if info, statErr := os.Lstat(final); statErr == nil && info.Mode().IsRegular() && info.Size() == size {
		// An interrupted export already published this exact object.
		_ = os.Remove(temporary)
		w.reused++
	} else {
		if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
			_ = os.Remove(temporary)
			return Object{}, false, err
		}
		if err := os.Rename(temporary, final); err != nil {
			_ = os.Remove(temporary)
			return Object{}, false, err
		}
		w.writtenBytes += size
	}
	object := Object{SHA256: hash, Path: relative, Kind: kind, MediaType: mediaType, SizeBytes: size}
	w.objects[hash] = object
	w.order = append(w.order, hash)
	w.totalBytes += size
	return object, false, nil
}

// pruneUnlistedObjects removes object files that are not part of this archive,
// so a resumed export with a different selection cannot leave stale objects
// behind for the verifier to reject.
func (w *archiveWriter) pruneUnlistedObjects() error {
	objectsRoot := filepath.Join(w.root, "objects")
	if _, err := os.Stat(objectsRoot); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	listed := map[string]bool{}
	for _, hash := range w.order {
		listed[filepath.Join(w.root, filepath.FromSlash(w.objects[hash].Path))] = true
	}
	var directories []string
	err := filepath.WalkDir(objectsRoot, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			directories = append(directories, current)
			return nil
		}
		if !listed[current] {
			return os.Remove(current)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Remove now-empty fanout directories deepest first; a non-empty directory
	// simply fails and is left alone.
	for index := len(directories) - 1; index >= 0; index-- {
		if directories[index] == objectsRoot {
			continue
		}
		_ = os.Remove(directories[index])
	}
	return nil
}

func (w *archiveWriter) publishManifest(manifest Manifest) error {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if int64(len(raw)) > w.options.Limits.MaxManifestBytes {
		return fmt.Errorf("manifest is %d bytes, above the %d byte limit", len(raw), w.options.Limits.MaxManifestBytes)
	}
	temporary := filepath.Join(w.staging, "manifest.json")
	if err := os.WriteFile(temporary, raw, 0o644); err != nil {
		return err
	}
	file, err := os.Open(temporary)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := firstError(syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(temporary, filepath.Join(w.root, "manifest.json")); err != nil {
		return err
	}
	directory, err := os.Open(w.root)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (w *archiveWriter) sortedObjects() []Object {
	objects := make([]Object, 0, len(w.order))
	for _, hash := range w.order {
		objects = append(objects, w.objects[hash])
	}
	return SortedObjects(objects)
}

// exportRun holds the state one export accumulates while streaming canonical
// records. Only bounded identity sets are retained: documents, revisions,
// resources, and source bundles are processed in batches and never buffered.
type exportRun struct {
	source  store.ExportReader
	options ExportOptions
	writer  *archiveWriter

	resolution      store.SelectionResolution
	identity        store.DatabaseIdentity
	schemaVersion   int
	usedCollections map[string]bool
	usedNotebooks   map[string]bool
	usedTags        map[string]bool
	clearedLinks    int
	warnings        []string
	collectionIDs   []string
}

func (r *exportRun) collect(ctx context.Context) error {
	r.usedCollections = map[string]bool{}
	r.usedNotebooks = map[string]bool{}
	r.usedTags = map[string]bool{}

	request := store.SelectionPlanRequest{
		Target:       r.options.Target,
		Selection:    r.options.Selection,
		Policy:       r.options.Policy,
		DetailLimit:  1,
		MaxDocuments: r.options.MaxDocuments,
	}
	resolution, err := r.source.ResolveSelection(ctx, request)
	if err != nil {
		return err
	}
	r.resolution = resolution
	if r.identity, err = r.source.GetDatabaseIdentity(ctx); err != nil {
		return err
	}
	status, err := r.source.Status(ctx)
	if err != nil {
		return err
	}
	r.schemaVersion = status.SchemaVersion
	if r.schemaVersion < MinimumSchemaVersion {
		return fmt.Errorf("archive v2 requires schema %d or newer; this database is at %d", MinimumSchemaVersion, r.schemaVersion)
	}

	if err := r.writeDocuments(ctx); err != nil {
		return err
	}
	if err := r.writeResources(ctx); err != nil {
		return err
	}
	if err := r.writeSourceBundles(ctx); err != nil {
		return err
	}
	if err := r.writeHeaderRecords(ctx); err != nil {
		return err
	}
	return r.writer.flushRecords()
}

func (r *exportRun) writeDocuments(ctx context.Context) error {
	documentIDs := r.resolution.DocumentIDs
	currentOnly := false
	for start := 0; start < len(documentIDs); start += exportBatch {
		end := min(start+exportBatch, len(documentIDs))
		batch := documentIDs[start:end]
		documents, err := r.source.ExportDocuments(ctx, batch)
		if err != nil {
			return err
		}
		for _, document := range documents {
			notebookID := document.NotebookID
			if notebookID == "" {
				notebookID = store.DefaultNotebookID
				r.warnf("document %s had no notebook and was archived under the default notebook", document.ID)
			}
			r.usedCollections[document.CollectionID] = true
			r.usedNotebooks[notebookID] = true
			if err := r.writer.addRecord(RecordDocument, DocumentRecord{
				ID:                document.ID,
				CollectionID:      document.CollectionID,
				NotebookID:        notebookID,
				CurrentRevisionID: document.CurrentRevisionID,
				DeletedAt:         optionalTimestamp(document.DeletedAt),
				CreatedAt:         timestamp(document.CreatedAt),
				UpdatedAt:         timestamp(document.UpdatedAt),
			}); err != nil {
				return err
			}
		}
		if err := r.source.ExportRevisions(ctx, batch, currentOnly, func(revision store.ExportRevision) error {
			return r.writeRevision(revision)
		}); err != nil {
			return err
		}
		memberships, err := r.source.ExportDocumentTags(ctx, batch)
		if err != nil {
			return err
		}
		for _, membership := range memberships {
			r.usedTags[membership.TagID] = true
			if err := r.writer.addRecord(RecordDocumentTag, DocumentTagRecord{DocumentID: membership.DocumentID, TagID: membership.TagID}); err != nil {
				return err
			}
		}
		references, err := r.source.ExportDocumentResources(ctx, batch)
		if err != nil {
			return err
		}
		for _, reference := range references {
			if !containsSorted(r.resolution.ResourceIDs, reference.ResourceID) {
				// The planner reports reachable resources; a relation the plan
				// excluded (for example an oversized resource) is not archived.
				continue
			}
			if err := r.writer.addRecord(RecordDocumentResource, DocumentResourceRecord{
				DocumentID:   reference.DocumentID,
				ResourceID:   reference.ResourceID,
				RelationType: reference.RelationType,
				Ordinal:      reference.Ordinal,
				AnchorJSON:   jsonObjectOrEmpty(reference.AnchorJSON),
			}); err != nil {
				return err
			}
		}
		if err := r.source.ExportLinks(ctx, batch, r.writeLink); err != nil {
			return err
		}
		if r.resolution.Plan.Policy.IncludeProvenance {
			sources, err := r.source.ExportProvenance(ctx, batch)
			if err != nil {
				return err
			}
			for _, source := range sources {
				if err := r.writeProvenance(source); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *exportRun) writeRevision(revision store.ExportRevision) error {
	mimeType := strings.TrimSpace(revision.BodyMIMEType)
	if mimeType == "" {
		mimeType = "text/markdown"
	}
	canonical, err := normalizeMediaType(mimeType)
	if err != nil {
		r.warnf("revision %s had MIME type %q and was archived as text/markdown", revision.ID, revision.BodyMIMEType)
		canonical = "text/markdown"
	}
	body, err := r.writer.writeBlob(strings.NewReader(revision.Body), canonical)
	if err != nil {
		return fmt.Errorf("revision %s body: %w", revision.ID, err)
	}
	return r.writer.addRecord(RecordRevision, RevisionRecord{
		ID:           revision.ID,
		DocumentID:   revision.DocumentID,
		Title:        revision.Title,
		Body:         body,
		BodyMIMEType: canonical,
		MetadataJSON: jsonObjectOrEmpty(revision.MetadataJSON),
		Message:      revision.Message,
		CreatedAt:    timestamp(revision.CreatedAt),
	})
}

// writeLink preserves the canonical link record but never lets it point
// outside the archive: a target the selection excluded is reported instead of
// being encoded as a dangling reference.
func (r *exportRun) writeLink(link store.DocumentLink) error {
	record := LinkRecord{
		ID:               "lnk_" + strconv.FormatInt(link.ID, 10),
		SourceDocumentID: link.SourceDocumentID,
		TargetDocumentID: link.TargetDocumentID,
		TargetResourceID: link.TargetResourceID,
		TargetURI:        link.TargetURI,
		RelationType:     defaultString(link.RelationType, "link"),
		SourceFormat:     defaultString(link.SourceFormat, "markdown"),
		RawTarget:        link.RawTarget,
		DisplayText:      link.DisplayText,
		AnchorType:       link.AnchorType,
		AnchorValue:      link.AnchorValue,
		Context:          link.Context,
		SourceStartByte:  link.SourceStartByte,
		SourceEndByte:    max(link.SourceEndByte, link.SourceStartByte),
		SourceLine:       link.SourceLine,
		SourceColumn:     link.SourceColumn,
		ResolutionStatus: defaultString(link.ResolutionStatus, "unresolved"),
	}
	if record.TargetDocumentID != "" && !containsSorted(r.resolution.DocumentIDs, record.TargetDocumentID) {
		record.TargetDocumentID = ""
		record.ResolutionStatus = "target_excluded"
		r.clearedLinks++
	}
	if record.TargetResourceID != "" && !containsSorted(r.resolution.ResourceIDs, record.TargetResourceID) {
		record.TargetResourceID = ""
		if record.ResolutionStatus != "target_excluded" {
			record.ResolutionStatus = "target_excluded"
			r.clearedLinks++
		}
	}
	return r.writer.addRecord(RecordLink, record)
}

func (r *exportRun) writeProvenance(source store.DocumentSource) error {
	metadata := jsonObjectOrEmpty(source.MetadataJSON)
	if !r.resolution.Plan.Policy.IncludePrivateMetadata {
		// P1 classifies source metadata_json as private; a subset transfer
		// keeps the provenance identity but not the source-specific payload.
		metadata = json.RawMessage("{}")
	}
	record := ProvenanceRecord{
		DocumentID:   source.DocumentID,
		SourceSystem: source.SourceSystem,
		ExternalID:   source.ExternalID,
		Author:       source.Author,
		AuthorID:     source.AuthorID,
		ThreadID:     source.ThreadID,
		ReplyTo:      source.ReplyTo,
		SourceURL:    source.SourceURL,
		PublishedAt:  source.PublishedAt,
		MetadataJSON: json.RawMessage(metadata),
		CreatedAt:    timestamp(source.CreatedAt),
		UpdatedAt:    timestamp(source.UpdatedAt),
	}
	if source.PublishedTS != 0 {
		published := source.PublishedTS
		record.PublishedTS = &published
	}
	return r.writer.addRecord(RecordProvenance, record)
}

func (r *exportRun) writeResources(ctx context.Context) error {
	resourceIDs := r.resolution.ResourceIDs
	for start := 0; start < len(resourceIDs); start += exportBatch {
		end := min(start+exportBatch, len(resourceIDs))
		resources, err := r.source.ExportResources(ctx, resourceIDs[start:end])
		if err != nil {
			return err
		}
		for _, resource := range resources {
			canonical, err := normalizeMediaType(resource.MIMEType)
			if err != nil {
				r.warnf("resource %s had MIME type %q and was archived as application/octet-stream", resource.ID, resource.MIMEType)
				canonical = "application/octet-stream"
			}
			content, _, err := r.source.OpenBlobContent(ctx, resource.BlobSHA256)
			if err != nil {
				return fmt.Errorf("resource %s content: %w", resource.ID, err)
			}
			blob, writeErr := r.writer.writeBlob(content, canonical)
			closeErr := content.Close()
			if err := firstError(writeErr, closeErr); err != nil {
				return fmt.Errorf("resource %s content: %w", resource.ID, err)
			}
			if blob.SHA256 != resource.BlobSHA256 {
				return fmt.Errorf("resource %s content hash %s does not match the canonical blob %s", resource.ID, blob.SHA256, resource.BlobSHA256)
			}
			r.usedCollections[resource.CollectionID] = true
			if err := r.writer.addRecord(RecordResource, ResourceRecord{
				ID:                 resource.ID,
				CollectionID:       resource.CollectionID,
				Blob:               blob,
				Filename:           resource.Filename,
				MIMEType:           canonical,
				MetadataJSON:       jsonObjectOrEmpty(resource.MetadataJSON),
				UnreferencedAt:     optionalTimestamp(resource.UnreferencedAt),
				UnreferencedReason: resource.UnreferencedReason,
				CreatedAt:          timestamp(resource.CreatedAt),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *exportRun) writeSourceBundles(ctx context.Context) error {
	keys := r.resolution.SourceBundleKeys
	for start := 0; start < len(keys); start += exportBatch {
		end := min(start+exportBatch, len(keys))
		items, err := r.source.ExportSourceBundles(ctx, keys[start:end])
		if err != nil {
			return err
		}
		for _, item := range items {
			relative := path.Clean(filepath.ToSlash(item.RelativePath))
			if !validRelativePath(relative, r.options.Limits) {
				r.warnf("source bundle item for %s has an unsafe relative path and was skipped", item.SourceSystem)
				continue
			}
			content, err := r.source.OpenSourceBundleContent(ctx, item)
			if err != nil {
				return fmt.Errorf("source bundle content: %w", err)
			}
			blob, writeErr := r.writer.writeBlob(content, "application/octet-stream")
			closeErr := content.Close()
			if err := firstError(writeErr, closeErr); err != nil {
				return fmt.Errorf("source bundle content: %w", err)
			}
			propertyOrder, err := json.Marshal(nonNilStrings(item.PropertyOrder))
			if err != nil {
				return err
			}
			r.usedCollections[item.CollectionID] = true
			if err := r.writer.addRecord(RecordSourceBundle, SourceBundleRecord{
				SourceSystem: item.SourceSystem,
				// Raw source and item keys can be local filesystem paths, so an
				// archive carries only their fingerprints.
				SourceKeySHA256:   sha256Hex(item.SourceKey),
				CollectionID:      item.CollectionID,
				ItemKeySHA256:     sha256Hex(item.ItemKey),
				ItemType:          defaultString(item.ItemType, "file"),
				ExternalID:        item.ExternalID,
				RelativePath:      relative,
				Content:           blob,
				PropertyOrderJSON: propertyOrder,
				UpdatedAt:         timestamp(item.UpdatedAt),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeHeaderRecords emits the container records last: by this point the run
// knows exactly which collections, notebooks, and tags the archived documents,
// resources, and bundles actually reference.
func (r *exportRun) writeHeaderRecords(ctx context.Context) error {
	full := r.options.Target == TargetFullArchive

	notebooks, err := r.source.ListNotebooks(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]store.Notebook, len(notebooks))
	for _, notebook := range notebooks {
		byID[notebook.ID] = notebook
	}
	selected := map[string]bool{}
	for _, notebook := range notebooks {
		if full || r.usedNotebooks[notebook.ID] {
			// Ancestors must travel with a notebook or the tree is incomplete.
			for current := notebook.ID; current != "" && !selected[current]; {
				selected[current] = true
				current = byID[current].ParentID
			}
		}
	}
	for _, notebook := range notebooks {
		if !selected[notebook.ID] {
			continue
		}
		if err := r.writer.addRecord(RecordNotebook, NotebookRecord{
			ID:        notebook.ID,
			ParentID:  notebook.ParentID,
			Name:      notebook.Name,
			IconEmoji: notebook.IconEmoji,
			Builtin:   notebook.Builtin,
			Position:  notebook.Position,
			CreatedAt: timestamp(notebook.CreatedAt),
			UpdatedAt: timestamp(notebook.UpdatedAt),
		}); err != nil {
			return err
		}
	}
	for _, used := range sortedKeys(r.usedNotebooks) {
		if !selected[used] {
			return fmt.Errorf("document notebook %q is missing from the canonical notebook tree", used)
		}
	}

	if full {
		searchNotebooks, err := r.source.ListSearchNotebooks(ctx)
		if err != nil {
			return err
		}
		for _, searchNotebook := range searchNotebooks {
			if err := r.writer.addRecord(RecordSearchNotebook, SearchNotebookRecord{
				ID:         searchNotebook.ID,
				Name:       searchNotebook.Name,
				IconEmoji:  searchNotebook.IconEmoji,
				Query:      searchNotebook.Query,
				Builtin:    searchNotebook.Builtin,
				SortAnchor: defaultString(searchNotebook.SortAnchor, "normal"),
				CreatedAt:  timestamp(searchNotebook.CreatedAt),
			}); err != nil {
				return err
			}
		}
	} else if len(r.resolution.DocumentIDs) > 0 {
		r.warn("query-backed search notebooks are omitted from a subset transfer because their queries describe the whole library")
	}

	var tagIDs []string
	if !full {
		tagIDs = sortedKeys(r.usedTags)
	}
	tags, err := r.source.ExportTags(ctx, tagIDs)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if err := r.writer.addRecord(RecordTag, TagRecord{ID: tag.ID, Name: tag.Name, CreatedAt: timestamp(tag.CreatedAt)}); err != nil {
			return err
		}
	}

	if r.options.Selection.CollectionID != "" {
		r.usedCollections[r.options.Selection.CollectionID] = true
	} else {
		r.usedCollections["default"] = true
	}
	collections, err := r.source.ExportCollections(ctx, sortedKeys(r.usedCollections))
	if err != nil {
		return err
	}
	if len(collections) == 0 {
		return fmt.Errorf("no collection rows matched the selection; nothing can be archived")
	}
	for _, collection := range collections {
		r.collectionIDs = append(r.collectionIDs, collection.ID)
		settings, err := json.Marshal(map[string]string{})
		if err != nil {
			return err
		}
		if err := r.writer.addRecord(RecordCollection, CollectionRecord{
			ID:           collection.ID,
			Name:         collection.Name,
			Description:  collection.Description,
			Capabilities: sortedStrings(collection.Capabilities),
			SettingsJSON: settings,
			CreatedAt:    timestamp(collection.CreatedAt),
		}); err != nil {
			return err
		}
	}
	sort.Strings(r.collectionIDs)
	return nil
}

func (r *exportRun) buildManifest() (Manifest, error) {
	manifest := Manifest{
		Format:  FormatName,
		Version: FormatVersion,
		Snapshot: SnapshotMetadata{
			ID:                      r.options.SnapshotID,
			CreatedAt:               r.options.CreatedAt.Format(time.RFC3339),
			Target:                  r.options.Target,
			DatabaseID:              r.identity.DatabaseID,
			SourceReplicaID:         r.identity.ReplicaID,
			Consistency:             "sqlite_read_transaction",
			CollectionIDs:           r.collectionIDs,
			SelectionManifestSHA256: r.resolution.Plan.ManifestSHA256,
		},
		Compatibility: Compatibility{
			MinimumReaderVersion: FormatVersion,
			SourceSchemaVersion:  r.schemaVersion,
			MinimumSchemaVersion: MinimumSchemaVersion,
			MaximumSchemaVersion: r.schemaVersion,
			RequiredCapabilities: sortedStrings(RequiredCapabilities()),
			OptionalCapabilities: []string{},
		},
		Objects: r.writer.sortedObjects(),
		Counts:  r.writer.counts,
	}
	if err := FinalizeManifest(&manifest); err != nil {
		return Manifest{}, err
	}
	if err := validateManifest(manifest, r.options.Limits); err != nil {
		return Manifest{}, fmt.Errorf("generated manifest is not admissible: %w", err)
	}
	return manifest, nil
}

func (r *exportRun) report(manifest Manifest, started time.Time) ExportReport {
	fullBackup := r.options.Target == TargetFullArchive &&
		len(r.options.Selection.NotebookIDs) == 0 && len(r.options.Selection.Tags) == 0 &&
		len(r.options.Selection.DocumentIDs) == 0 && strings.TrimSpace(r.options.Selection.Query) == "" &&
		r.resolution.Plan.Policy.IncludeTrashed
	if !fullBackup {
		r.warn("this archive is a scoped snapshot, not a complete database backup")
	}
	if len(r.resolution.DocumentIDs) == 0 {
		r.warn("the selection matched no notes; this archive contains only container records")
	}
	if r.clearedLinks > 0 {
		r.warnf("%d link targets fell outside the archived selection and were recorded as target_excluded", r.clearedLinks)
	}
	warnings := append([]string{}, r.warnings...)
	warnings = append(warnings, r.resolution.Plan.Warnings...)
	return ExportReport{
		Target:                  manifest.Snapshot.Target,
		FullBackup:              fullBackup,
		SnapshotID:              manifest.Snapshot.ID,
		DatabaseID:              manifest.Snapshot.DatabaseID,
		SourceReplicaID:         manifest.Snapshot.SourceReplicaID,
		CreatedAt:               manifest.Snapshot.CreatedAt,
		CommitSHA256:            manifest.CommitSHA256,
		SelectionManifestSHA256: manifest.Snapshot.SelectionManifestSHA256,
		CollectionIDs:           manifest.Snapshot.CollectionIDs,
		Counts:                  manifest.Counts,
		Objects:                 len(manifest.Objects),
		RecordObjects:           r.writer.recordObjects,
		BlobObjects:             r.writer.blobObjects,
		DeduplicatedObjects:     r.writer.deduplicated,
		ReusedObjects:           r.writer.reused,
		Bytes:                   r.writer.totalBytes,
		BytesWritten:            r.writer.writtenBytes,
		SelectedDocuments:       r.resolution.Plan.Counts.SelectedDocuments,
		ExcludedDocuments:       r.resolution.Plan.Counts.ExcludedDocuments,
		ReachableResources:      r.resolution.Plan.Counts.ReachableResources,
		ClearedLinkTargets:      r.clearedLinks,
		Warnings:                warnings,
		ElapsedSeconds:          time.Since(started).Seconds(),
	}
}

func (r *exportRun) warnf(format string, args ...any) {
	r.warn(fmt.Sprintf(format, args...))
}

// warn records one deduplicated aggregate warning. Warnings never carry note
// titles, bodies, or local paths.
func (r *exportRun) warn(message string) {
	for _, existing := range r.warnings {
		if existing == message {
			return
		}
	}
	r.warnings = append(r.warnings, message)
}

// exportBatch matches the selection planner's bounded read batch.
const exportBatch = 400

func timestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func optionalTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return timestamp(value)
}

// jsonObjectOrEmpty keeps embedded canonical JSON when it is a well-formed
// object and substitutes an empty object otherwise, so one malformed legacy
// row cannot make a whole archive inadmissible.
func jsonObjectOrEmpty(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !validJSONObject(json.RawMessage(trimmed), DefaultLimits().MaxJSONDepth) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(trimmed)
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func containsSorted(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
