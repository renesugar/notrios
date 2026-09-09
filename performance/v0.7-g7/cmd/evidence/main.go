// Command evidence measures what G7 actually costs to transfer and whether it
// converges, using the production store rather than a prototype. It builds real
// replicas in a temporary directory, runs deterministic divergence workloads at
// G1's four offline intervals, and reports transfer bytes with and without
// deltas alongside the merge outcome of every case.
//
// It generates all of its own content. No private corpus, path, or note text is
// read or written.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncstate"
)

type intervalResult struct {
	Interval           string         `json:"interval"`
	EditsPerSide       int            `json:"edits_per_side"`
	Documents          int            `json:"documents"`
	BaseBodyBytes      int            `json:"base_body_bytes"`
	Revisions          int            `json:"revisions"`
	DeltaRevisions     int            `json:"delta_revisions"`
	InlineRevisions    int            `json:"inline_revisions"`
	CompleteBodyBytes  int            `json:"complete_body_bytes"`
	TransferBytes      int            `json:"transfer_bytes"`
	TransferPercent    float64        `json:"transfer_percent_of_complete"`
	CleanMerges        int            `json:"clean_merges"`
	Conflicts          int            `json:"conflicts"`
	ConflictKinds      map[string]int `json:"conflict_kinds"`
	PendingReasons     map[string]int `json:"pending_reasons"`
	PendingBodies      int            `json:"pending_bodies"`
	ConvergedDocuments int            `json:"converged_documents"`
	ElapsedMS          int64          `json:"elapsed_ms"`
}

type report struct {
	Schema       string           `json:"schema"`
	GeneratedFor string           `json:"generated_for"`
	Protocol     string           `json:"protocol"`
	SchemaV      int              `json:"database_schema_version"`
	Bounds       map[string]int   `json:"bounds"`
	Intervals    []intervalResult `json:"intervals"`
	Notes        []string         `json:"notes"`
}

func main() {
	out := flag.String("out", "", "path to write the JSON report")
	documents := flag.Int("documents", 24, "documents per interval")
	flag.Parse()
	if *out == "" {
		fail(fmt.Errorf("-out is required"))
	}

	result := report{
		Schema:       "notrios.g7.transfer.v1",
		GeneratedFor: "v0.7 G7 note revision objects, transfer deltas, three-way merge, and conflicts",
		Protocol:     fmt.Sprintf("%d.%d", syncstate.ProtocolMajor, syncstate.ProtocolMinor),
		SchemaV:      store.CurrentSchemaVersion,
		Bounds: map[string]int{
			"max_payload_bytes":   syncstate.MaxPayloadBytes,
			"max_operation_bytes": syncstate.MaxOperationBytes,
		},
		Notes: []string{
			"Every body is generated. No private corpus content, path, or hash is recorded.",
			"transfer_bytes counts the revision operation payloads that actually travelled.",
			"complete_body_bytes is what the same revisions would cost with every body inline.",
			"Desktop measurements. G2's emulator and physical-device checklists still gate any mobile claim.",
		},
	}
	for _, interval := range []struct {
		name  string
		edits int
	}{
		{"1 hour", 1},
		{"1 day", 4},
		{"1 week", 12},
		{"30 days", 32},
	} {
		measured, err := measure(interval.name, interval.edits, *documents)
		if err != nil {
			fail(err)
		}
		result.Intervals = append(result.Intervals, measured)
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	for _, row := range result.Intervals {
		fmt.Printf("%-8s edits=%2d revisions=%3d delta=%3d transfer=%8d complete=%8d %6.2f%% clean=%d conflicts=%d\n",
			row.Interval, row.EditsPerSide, row.Revisions, row.DeltaRevisions,
			row.TransferBytes, row.CompleteBodyBytes, row.TransferPercent, row.CleanMerges, row.Conflicts)
		fmt.Printf("         conflict kinds=%v pending=%v\n", row.ConflictKinds, row.PendingReasons)
	}
}

func measure(interval string, editsPerSide, documents int) (intervalResult, error) {
	ctx := context.Background()
	started := time.Now()
	root, err := os.MkdirTemp("", "notrios-g7-evidence-")
	if err != nil {
		return intervalResult{}, err
	}
	defer os.RemoveAll(root)

	left, err := openReplica(ctx, root, "left")
	if err != nil {
		return intervalResult{}, err
	}
	defer left.Close()
	right, err := openReplica(ctx, root, "right")
	if err != nil {
		return intervalResult{}, err
	}
	defer right.Close()
	if err := pair(ctx, left, right); err != nil {
		return intervalResult{}, err
	}

	base := generatedBody()
	created := make([]store.Document, 0, documents)
	for index := 0; index < documents; index++ {
		document, err := left.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_g7_%03d", index),
			Title:       fmt.Sprintf("Generated note %03d", index),
			Body:        base,
		})
		if err != nil {
			return intervalResult{}, err
		}
		created = append(created, document)
	}
	if err := ship(ctx, left, right); err != nil {
		return intervalResult{}, err
	}

	// Two replicas edit every note the same number of times. A third of the
	// documents receive deliberately overlapping edits so the report describes
	// conflicts as well as clean merges.
	for index, document := range created {
		overlapping := index%3 == 0
		leftHead, rightHead := document.CurrentRevisionID, document.CurrentRevisionID
		leftBody, rightBody := base, base
		for edit := 0; edit < editsPerSide; edit++ {
			leftBody = applyEdit(leftBody, "L", edit, overlapping)
			updated, err := left.UpdateDocument(ctx, store.UpdateDocumentRequest{
				ID: document.ID, Title: document.Title, Body: leftBody, BaseRevisionID: leftHead,
			})
			if err != nil {
				return intervalResult{}, err
			}
			leftHead = updated.CurrentRevisionID

			rightBody = applyEdit(rightBody, "R", edit, overlapping)
			mirrored, err := right.UpdateDocument(ctx, store.UpdateDocumentRequest{
				ID: document.ID, Title: document.Title, Body: rightBody, BaseRevisionID: rightHead,
			})
			if err != nil {
				return intervalResult{}, err
			}
			rightHead = mirrored.CurrentRevisionID
		}
	}

	transfer, err := measureTransfer(ctx, left)
	if err != nil {
		return intervalResult{}, err
	}
	for round := 0; round < 3; round++ {
		if err := ship(ctx, left, right); err != nil {
			return intervalResult{}, err
		}
		if err := ship(ctx, right, left); err != nil {
			return intervalResult{}, err
		}
	}

	converged := 0
	for _, document := range created {
		leftDocument, leftErr := left.GetDocument(ctx, document.ID)
		rightDocument, rightErr := right.GetDocument(ctx, document.ID)
		if leftErr != nil || rightErr != nil {
			return intervalResult{}, fmt.Errorf("read back %s: %v %v", document.ID, leftErr, rightErr)
		}
		if leftDocument.CurrentRevisionID == rightDocument.CurrentRevisionID && leftDocument.Body == rightDocument.Body {
			converged++
		}
	}
	status, err := left.SyncBodyStatus(ctx)
	if err != nil {
		return intervalResult{}, err
	}

	percent := 0.0
	if transfer.completeBytes > 0 {
		percent = float64(transfer.transferBytes) * 100 / float64(transfer.completeBytes)
	}
	return intervalResult{
		Interval: interval, EditsPerSide: editsPerSide, Documents: documents, BaseBodyBytes: len(base),
		Revisions: transfer.revisions, DeltaRevisions: transfer.deltaRevisions, InlineRevisions: transfer.inlineRevisions,
		CompleteBodyBytes: transfer.completeBytes, TransferBytes: transfer.transferBytes, TransferPercent: percent,
		CleanMerges: status.MergeRevisions, Conflicts: status.Conflicts, ConflictKinds: status.ConflictKinds,
		PendingReasons: status.PendingReasons, PendingBodies: status.PendingBodies,
		ConvergedDocuments: converged,
		ElapsedMS:          time.Since(started).Milliseconds(),
	}, nil
}

type transferTotals struct {
	revisions, deltaRevisions, inlineRevisions int
	transferBytes, completeBytes               int
}

// measureTransfer compares what the journal will actually publish with what the
// same revisions would cost if every one of them carried its complete body.
func measureTransfer(ctx context.Context, replica *store.SQLiteStore) (transferTotals, error) {
	identity, err := replica.GetDatabaseIdentity(ctx)
	if err != nil {
		return transferTotals{}, err
	}
	var totals transferTotals
	for cursor := int64(0); ; {
		operations, err := replica.ListSyncOperations(ctx, identity.ReplicaID, cursor, 500)
		if err != nil {
			return transferTotals{}, err
		}
		if len(operations) == 0 {
			return totals, nil
		}
		for _, operation := range operations {
			cursor = operation.Sequence
			if operation.RecordType != "revision" {
				continue
			}
			var payload struct {
				ContentLength int             `json:"content_length"`
				Body          *string         `json:"body"`
				Delta         json.RawMessage `json:"delta"`
			}
			if err := json.Unmarshal(operation.Payload, &payload); err != nil {
				return transferTotals{}, err
			}
			totals.revisions++
			totals.transferBytes += len(operation.Payload)
			// The counterfactual is the same operation with its complete body
			// inline, which is what the payload already is when no delta was
			// selected.
			complete := len(operation.Payload)
			if payload.Body == nil {
				complete = len(operation.Payload) - len(payload.Delta) + payload.ContentLength
				totals.deltaRevisions++
			} else {
				totals.inlineRevisions++
			}
			totals.completeBytes += complete
		}
	}
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
	if _, err := replica.AdoptDatabaseIdentity(ctx, "db_g7_evidence"); err != nil {
		return nil, err
	}
	if _, err := replica.EnrollLocalJournal(ctx, "G7 evidence replica"); err != nil {
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

// The generated note's shape. Each section is a heading, a blank line, a prose
// line, and a blank line, after a two-line document header.
const (
	generatedSections    = 60
	sectionLines         = 4
	headerLines          = 2
	sectionProseOffset   = 2
	disjointBandSections = 15
)

// generatedBody produces the fixed Markdown note every interval starts from, so
// each run measures the same workload.
func generatedBody() string {
	var builder strings.Builder
	builder.WriteString("# Generated note\n\n")
	for index := 0; index < generatedSections; index++ {
		fmt.Fprintf(&builder, "## Section %02d\n\nThe quick brown fox jumps over the lazy dog, %02d times, at length and without variation.\n\n", index, index)
	}
	return builder.String()
}

// applyEdit changes the prose line of one section. Each side owns a disjoint
// band of sections — the first and last quarter of the note — so a disjoint
// case stays disjoint at every edit count, including the 32 edits per side that
// stand for thirty days offline. An overlapping case sends both sides at the
// same section, which is the case that must conflict.
//
// The bands matter. An earlier version walked each side inward by one section
// per edit, so the two met around the twelfth edit and the thirty-day row
// reported conflicts that were an artefact of the workload rather than a
// property of the merge.
func applyEdit(body, marker string, edit int, overlapping bool) string {
	lines := strings.SplitAfter(body, "\n")
	section := edit % disjointBandSections
	if marker == "R" {
		section += generatedSections - disjointBandSections
	}
	if overlapping {
		section = 0
	}
	target := headerLines + section*sectionLines + sectionProseOffset
	if target < 0 || target >= len(lines) {
		target = len(lines) / 2
	}
	suffix := "\n"
	if !strings.HasSuffix(lines[target], "\n") {
		suffix = ""
	}
	lines[target] = strings.TrimSuffix(lines[target], "\n") + " [" + marker + fmt.Sprint(edit) + "]" + suffix
	return strings.Join(lines, "")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "g7 evidence:", err)
	os.Exit(1)
}
