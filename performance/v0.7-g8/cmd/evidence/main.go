// Command evidence measures G8 against the production store: how quickly a
// replica can use a note whose attachment has not arrived, what the default
// policy fetches on its own, and what chunked transfer costs at each size the
// G2 thresholds care about.
//
// Every attachment is generated. No private corpus, path, filename, or content
// is read or recorded, and the temporary replicas are deleted at the end.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncassets"
	"github.com/renesugar/notrios/internal/syncstate"
)

type sizeResult struct {
	Label                 string `json:"label"`
	ObjectBytes           int64  `json:"object_bytes"`
	Chunks                int    `json:"chunks"`
	Attachments           int    `json:"attachments"`
	MetadataOnlyMS        int64  `json:"metadata_only_ms"`
	AutoRequested         int    `json:"auto_requested"`
	AutoReason            string `json:"auto_reason"`
	ReadableBeforeBytes   bool   `json:"note_readable_before_bytes"`
	MaterializedCount     int    `json:"materialized"`
	FetchedBytes          int64  `json:"fetched_bytes"`
	MaterializeMS         int64  `json:"materialize_ms"`
	ResumeChunksRefetched int    `json:"resume_chunks_refetched"`
	ResumeChunksTotal     int    `json:"resume_chunks_total"`
	ExactReconstruction   bool   `json:"exact_reconstruction"`
}

type report struct {
	Schema       string           `json:"schema"`
	GeneratedFor string           `json:"generated_for"`
	SchemaV      int              `json:"database_schema_version"`
	Bounds       map[string]int64 `json:"bounds"`
	Sizes        []sizeResult     `json:"sizes"`
	Notes        []string         `json:"notes"`
}

func main() {
	out := flag.String("out", "", "path to write the JSON report")
	attachments := flag.Int("attachments", 8, "attachments per size")
	flag.Parse()
	if *out == "" {
		fail(fmt.Errorf("-out is required"))
	}
	result := report{
		Schema:       "notrios.g8.materialization.v1",
		GeneratedFor: "v0.7 G8 resource metadata, lazy materialization, chunks, and integrity",
		SchemaV:      store.CurrentSchemaVersion,
		Bounds: map[string]int64{
			"whole_object_threshold": syncassets.WholeObjectThreshold,
			"chunk_bytes":            syncassets.ChunkBytes,
			"max_chunks":             syncassets.MaxChunks,
			"max_object_bytes":       syncassets.MaxObjectBytes,
			"max_staging_bytes":      syncassets.MaxStagingBytes,
			"max_eager_bytes":        syncassets.MaxEagerBytes,
		},
		Notes: []string{
			"Every attachment is generated. No private corpus content, filename, or path is recorded.",
			"metadata_only_ms is the time to admit the operations, with no attachment bytes transferred.",
			"A resumed transfer refetches only the segments it did not already hold and verify.",
			"Desktop measurements. G2's emulator and physical-device checklists still gate any mobile claim.",
		},
	}
	for _, size := range []struct {
		label string
		bytes int64
	}{
		{"64 KiB (inline view)", 64 << 10},
		{"1 MiB (at the threshold)", syncassets.WholeObjectThreshold},
		{"4 MiB (chunked)", 4 * syncassets.ChunkBytes},
		{"16 MiB (chunked)", 16 * syncassets.ChunkBytes},
	} {
		measured, err := measure(size.label, size.bytes, *attachments)
		if err != nil {
			fail(err)
		}
		result.Sizes = append(result.Sizes, measured)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	for _, row := range result.Sizes {
		fmt.Printf("%-26s chunks=%3d metadata=%5dms auto=%d(%s) fetched=%9d in %5dms resume=%d/%d exact=%v\n",
			row.Label, row.Chunks, row.MetadataOnlyMS, row.AutoRequested, row.AutoReason,
			row.FetchedBytes, row.MaterializeMS, row.ResumeChunksRefetched, row.ResumeChunksTotal, row.ExactReconstruction)
	}
}

func measure(label string, objectBytes int64, attachments int) (sizeResult, error) {
	ctx := context.Background()
	root, err := os.MkdirTemp("", "notrios-g8-evidence-")
	if err != nil {
		return sizeResult{}, err
	}
	defer os.RemoveAll(root)

	origin, err := openReplica(ctx, root, "origin")
	if err != nil {
		return sizeResult{}, err
	}
	defer origin.Close()
	receiver, err := openReplica(ctx, root, "receiver")
	if err != nil {
		return sizeResult{}, err
	}
	defer receiver.Close()
	if err := pair(ctx, origin, receiver); err != nil {
		return sizeResult{}, err
	}

	chunks, err := syncassets.ChunkCount(objectBytes)
	if err != nil {
		return sizeResult{}, err
	}
	result := sizeResult{Label: label, ObjectBytes: objectBytes, Chunks: chunks, Attachments: attachments}

	document, err := origin.CreateDocument(ctx, store.CreateDocumentRequest{
		PreferredID: "doc_g8_evidence", Title: "Note with attachments", Body: "attachments follow\n",
	})
	if err != nil {
		return sizeResult{}, err
	}
	contents := make([][]byte, 0, attachments)
	resources := make([]store.Resource, 0, attachments)
	for index := 0; index < attachments; index++ {
		content := generatedContent(int64(index)*7919+objectBytes, objectBytes)
		resource, err := origin.CreateResource(ctx, store.CreateResourceRequest{
			CollectionID: document.CollectionID, Filename: fmt.Sprintf("attachment-%02d.bin", index),
			MIMEType: "application/octet-stream", Content: bytes.NewReader(content),
		})
		if err != nil {
			return sizeResult{}, err
		}
		if _, err := origin.AttachDocumentResource(ctx, store.AttachResourceRequest{
			DocumentID: document.ID, ResourceID: resource.ID, RelationType: "attachment",
		}); err != nil {
			return sizeResult{}, err
		}
		contents = append(contents, content)
		resources = append(resources, resource)
	}

	started := time.Now()
	if err := ship(ctx, origin, receiver); err != nil {
		return sizeResult{}, err
	}
	result.MetadataOnlyMS = time.Since(started).Milliseconds()

	// The note is readable before any attachment byte has moved. That is the
	// property the whole slice exists for.
	mirrored, err := receiver.GetDocument(ctx, document.ID)
	result.ReadableBeforeBytes = err == nil && mirrored.Body == "attachments follow\n"

	for _, resource := range resources {
		availability, err := receiver.ResourceAvailabilityFor(ctx, resource.ID)
		if err != nil {
			return sizeResult{}, err
		}
		if availability.Requested {
			result.AutoRequested++
		}
		result.AutoReason = availability.LastReason
	}

	// Everything is then pinned, which is what a user asking for their
	// attachments looks like.
	for _, resource := range resources {
		if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
			return sizeResult{}, err
		}
	}
	started = time.Now()
	materialization, err := receiver.MaterializeResources(ctx, store.NewLocalObjectProvider(origin), attachments)
	if err != nil {
		return sizeResult{}, err
	}
	result.MaterializeMS = time.Since(started).Milliseconds()
	result.MaterializedCount = materialization.Materialized
	result.FetchedBytes = materialization.FetchedBytes

	result.ExactReconstruction = true
	for index, resource := range resources {
		_, reader, err := receiver.OpenResourceContent(ctx, resource.ID)
		if err != nil {
			return sizeResult{}, err
		}
		var received bytes.Buffer
		_, copyErr := received.ReadFrom(reader)
		_ = reader.Close()
		if copyErr != nil {
			return sizeResult{}, copyErr
		}
		if !bytes.Equal(received.Bytes(), contents[index]) {
			result.ExactReconstruction = false
		}
	}

	refetched, total, err := measureResume(ctx, origin, receiver, objectBytes)
	if err != nil {
		return sizeResult{}, err
	}
	result.ResumeChunksRefetched, result.ResumeChunksTotal = refetched, total
	return result, nil
}

// measureResume interrupts one fresh transfer after a single chunk and reports
// how many segments the next pass had to ask for.
func measureResume(ctx context.Context, origin, receiver *store.SQLiteStore, objectBytes int64) (int, int, error) {
	content := generatedContent(objectBytes+104729, objectBytes)
	resource, err := origin.CreateResource(ctx, store.CreateResourceRequest{
		CollectionID: "default", Filename: "resume.bin",
		MIMEType: "application/octet-stream", Content: bytes.NewReader(content),
	})
	if err != nil {
		return 0, 0, err
	}
	if err := ship(ctx, origin, receiver); err != nil {
		return 0, 0, err
	}
	if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
		return 0, 0, err
	}
	total, err := syncassets.ChunkCount(objectBytes)
	if err != nil {
		return 0, 0, err
	}
	interrupted := &limitedProvider{inner: store.NewLocalObjectProvider(origin), limit: 1}
	if _, err := receiver.MaterializeResources(ctx, interrupted, 4); err != nil {
		return 0, 0, err
	}
	counting := &countingProvider{inner: store.NewLocalObjectProvider(origin)}
	if _, err := receiver.MaterializeResources(ctx, counting, 4); err != nil {
		return 0, 0, err
	}
	return counting.chunks, total, nil
}

type limitedProvider struct {
	inner  store.ObjectProvider
	limit  int
	served int
}

func (p *limitedProvider) FetchManifest(ctx context.Context, sha string) (syncassets.Manifest, error) {
	return p.inner.FetchManifest(ctx, sha)
}

func (p *limitedProvider) FetchChunk(ctx context.Context, sha string, ordinal int) ([]byte, error) {
	if p.served >= p.limit {
		return nil, fmt.Errorf("connection dropped")
	}
	content, err := p.inner.FetchChunk(ctx, sha, ordinal)
	if err == nil {
		p.served++
	}
	return content, err
}

type countingProvider struct {
	inner  store.ObjectProvider
	chunks int
}

func (p *countingProvider) FetchManifest(ctx context.Context, sha string) (syncassets.Manifest, error) {
	return p.inner.FetchManifest(ctx, sha)
}

func (p *countingProvider) FetchChunk(ctx context.Context, sha string, ordinal int) ([]byte, error) {
	p.chunks++
	return p.inner.FetchChunk(ctx, sha, ordinal)
}

func generatedContent(seed, size int64) []byte {
	content := make([]byte, size)
	value := uint64(seed)*6364136223846793005 + 1442695040888963407
	for index := range content {
		value = value*6364136223846793005 + 1442695040888963407
		content[index] = byte(value >> 33)
	}
	return content
}

func openReplica(ctx context.Context, root, name string) (*store.SQLiteStore, error) {
	directory := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(directory, "assets"), 0o755); err != nil {
		return nil, err
	}
	replica, err := store.OpenSQLiteWithAssetStore(filepath.Join(directory, "notes.sqlite"), filepath.Join(directory, "assets"))
	if err != nil {
		return nil, err
	}
	if err := replica.Bootstrap(ctx); err != nil {
		return nil, err
	}
	if _, err := replica.AdoptDatabaseIdentity(ctx, "db_g8_evidence"); err != nil {
		return nil, err
	}
	if _, err := replica.EnrollLocalJournal(ctx, "G8 evidence replica"); err != nil {
		return nil, err
	}
	return replica, nil
}

func pair(ctx context.Context, replicas ...*store.SQLiteStore) error {
	handshakes := make([]syncstate.Handshake, len(replicas))
	for index, replica := range replicas {
		handshake, err := replica.LocalSyncHandshake(ctx)
		if err != nil {
			return err
		}
		handshakes[index] = handshake
	}
	for receiver, replica := range replicas {
		for source, handshake := range handshakes {
			if receiver == source {
				continue
			}
			if err := replica.ConfigureSyncAdmissionPeer(ctx, handshake); err != nil {
				return err
			}
		}
	}
	return nil
}

func ship(ctx context.Context, from, to *store.SQLiteStore) error {
	handshake, err := from.LocalSyncHandshake(ctx)
	if err != nil {
		return err
	}
	for cursor := int64(0); ; {
		operations, err := from.ListSyncOperations(ctx, handshake.ReplicaID, cursor, 500)
		if err != nil {
			return err
		}
		if len(operations) == 0 {
			return nil
		}
		cursor = operations[len(operations)-1].Sequence
		if _, err := to.AdmitSyncOperations(ctx, handshake, operations); err != nil {
			return err
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "g8 evidence:", err)
	os.Exit(1)
}
