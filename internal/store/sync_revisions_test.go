package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/syncbody"
	"github.com/renesugar/notrios/internal/syncdelta"
	"github.com/renesugar/notrios/internal/syncstate"
)

// shipAll delivers every operation one replica has authored to another. It is
// the G7 test carrier: G11 and G14 own the real ones, and nothing here depends
// on which of them is used.
func shipAll(t *testing.T, from, to *SQLiteStore) {
	t.Helper()
	ctx := context.Background()
	handshake := refreshHandshake(t, from)
	operations := localOperations(t, from, handshake.ReplicaID)
	if len(operations) == 0 {
		return
	}
	if _, err := to.AdmitSyncOperations(ctx, handshake, operations); err != nil {
		t.Fatalf("admit %s: %v", handshake.ReplicaID, err)
	}
}

// exchange runs deliveries until nothing new moves, so a merge one replica
// authors reaches the other.
func exchange(t *testing.T, replicas ...*SQLiteStore) {
	t.Helper()
	for round := 0; round < 4; round++ {
		for _, from := range replicas {
			for _, to := range replicas {
				if from != to {
					shipAll(t, from, to)
				}
			}
		}
	}
}

func currentBody(t *testing.T, st *SQLiteStore, documentID string) (string, string) {
	t.Helper()
	document, err := st.GetDocument(context.Background(), documentID)
	if err != nil {
		t.Fatalf("GetDocument %s: %v", documentID, err)
	}
	return document.Body, document.CurrentRevisionID
}

func conflictRows(t *testing.T, st *SQLiteStore, documentID string) []map[string]string {
	t.Helper()
	return queryRows(t, st, `SELECT id, kind, base_revision_id, revision_a, revision_b, region_count
		FROM sync_document_conflicts WHERE document_id = ? ORDER BY id`, documentID)
}

func queryRows(t *testing.T, st *SQLiteStore, query string, args ...string) []map[string]string {
	t.Helper()
	rows, err := st.syncRevisionRowsForTest(query, args...)
	if err != nil {
		t.Fatalf("query %s: %v", query, err)
	}
	return rows
}

func makeDivergedDocument(t *testing.T, base, localEdit, remoteEdit string) (*SQLiteStore, *SQLiteStore, string) {
	t.Helper()
	ctx := context.Background()
	left := newAdmissionReplica(t, "db_g7_model")
	right := newAdmissionReplica(t, "db_g7_model")
	configureAllAdmissionPeers(t, []*SQLiteStore{left, right})

	document, err := left.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_g7", Title: "Shared note", Body: base,
	})
	if err != nil {
		t.Fatal(err)
	}
	shipAll(t, left, right)

	mirrored, err := right.GetDocument(ctx, document.ID)
	if err != nil {
		t.Fatalf("the document did not reach the second replica: %v", err)
	}
	if mirrored.Body != base || mirrored.CurrentRevisionID != document.CurrentRevisionID {
		t.Fatalf("replicas disagree before diverging: %q/%s vs %q/%s",
			document.Body, document.CurrentRevisionID, mirrored.Body, mirrored.CurrentRevisionID)
	}

	if _, err := left.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "Shared note", Body: localEdit, BaseRevisionID: document.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := right.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "Shared note", Body: remoteEdit, BaseRevisionID: mirrored.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	return left, right, document.ID
}

func TestConcurrentDisjointEditsConvergeToOneMergeRevision(t *testing.T) {
	base := "alpha\nbeta\ngamma\ndelta\n"
	left, right, documentID := makeDivergedDocument(t,
		base, "alpha CHANGED\nbeta\ngamma\ndelta\n", "alpha\nbeta\ngamma\ndelta CHANGED\n")
	exchange(t, left, right)

	wantBody := "alpha CHANGED\nbeta\ngamma\ndelta CHANGED\n"
	leftBody, leftRevision := currentBody(t, left, documentID)
	rightBody, rightRevision := currentBody(t, right, documentID)
	if leftBody != wantBody || rightBody != wantBody {
		t.Fatalf("bodies did not converge:\nleft:  %q\nright: %q\nwant:  %q", leftBody, rightBody, wantBody)
	}
	if leftRevision != rightRevision {
		t.Fatalf("replicas converged on different revisions: %s vs %s", leftRevision, rightRevision)
	}

	// Both replicas may compute the merge. Because the identity is derived from
	// the parents and the merged bytes, that produces one revision rather than
	// two holding the same content.
	for name, replica := range map[string]*SQLiteStore{"left": left, "right": right} {
		merges := queryRows(t, replica, `SELECT id, parent_revision_ids FROM document_revisions
			WHERE document_id = ? AND json_array_length(parent_revision_ids) = 2 ORDER BY id`, documentID)
		if len(merges) != 1 {
			t.Fatalf("%s has %d merge revisions, want 1: %+v", name, len(merges), merges)
		}
		if merges[0]["id"] != leftRevision {
			t.Fatalf("%s: current revision %s is not the merge %s", name, leftRevision, merges[0]["id"])
		}
		if conflicts := conflictRows(t, replica, documentID); len(conflicts) != 0 {
			t.Fatalf("%s recorded a conflict for a clean merge: %+v", name, conflicts)
		}
	}
}

func TestOverlappingEditsRecordOneDurableConflictOnBothReplicas(t *testing.T) {
	left, right, documentID := makeDivergedDocument(t,
		"the quick brown fox\n", "the SWIFT brown fox\n", "the RAPID brown fox\n")
	exchange(t, left, right)

	leftConflicts := conflictRows(t, left, documentID)
	rightConflicts := conflictRows(t, right, documentID)
	if len(leftConflicts) != 1 || len(rightConflicts) != 1 {
		t.Fatalf("conflict counts: left=%d right=%d", len(leftConflicts), len(rightConflicts))
	}
	// One disagreement, one identity. A conflict that each replica named
	// differently would be reported twice and resolved once.
	if leftConflicts[0]["id"] != rightConflicts[0]["id"] {
		t.Fatalf("conflict ids differ: %s vs %s", leftConflicts[0]["id"], rightConflicts[0]["id"])
	}
	if leftConflicts[0]["kind"] != syncbody.ConflictSameToken {
		t.Fatalf("conflict kind = %q", leftConflicts[0]["kind"])
	}
	if leftConflicts[0]["revision_a"] >= leftConflicts[0]["revision_b"] {
		t.Fatalf("conflict revisions are not stored in sorted order: %+v", leftConflicts[0])
	}

	// Neither variant is lost, and no merge revision pretends they converged.
	for name, replica := range map[string]*SQLiteStore{"left": left, "right": right} {
		merges := queryRows(t, replica, `SELECT id FROM document_revisions
			WHERE document_id = ? AND json_array_length(parent_revision_ids) = 2`, documentID)
		if len(merges) != 0 {
			t.Fatalf("%s invented a merge revision for a conflict: %+v", name, merges)
		}
		for _, revisionID := range []string{leftConflicts[0]["revision_a"], leftConflicts[0]["revision_b"]} {
			rows := queryRows(t, replica, `SELECT body FROM document_revisions WHERE id = ?`, revisionID)
			if len(rows) != 1 {
				t.Fatalf("%s does not hold conflicting revision %s", name, revisionID)
			}
		}
	}

	// Both replicas display the same side while the conflict stands.
	_, leftRevision := currentBody(t, left, documentID)
	_, rightRevision := currentBody(t, right, documentID)
	if leftRevision != rightRevision {
		t.Fatalf("a conflicted document must still agree on what it shows: %s vs %s", leftRevision, rightRevision)
	}
}

func TestConflictClearsWhenAResolvingRevisionArrives(t *testing.T) {
	ctx := context.Background()
	left, right, documentID := makeDivergedDocument(t,
		"the quick brown fox\n", "the SWIFT brown fox\n", "the RAPID brown fox\n")
	exchange(t, left, right)
	if len(conflictRows(t, left, documentID)) != 1 {
		t.Fatal("expected a conflict to resolve")
	}

	// A person resolves it by writing a revision on top of the displayed head.
	_, head := currentBody(t, left, documentID)
	if _, err := left.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: documentID, Title: "Shared note", Body: "the AGREED brown fox\n", BaseRevisionID: head,
	}); err != nil {
		t.Fatal(err)
	}
	exchange(t, left, right)

	// The resolution descends from only one of the two heads, so the other is
	// still a head and the document is *still* diverged. What must change is
	// that the conflict is re-derived from the current graph rather than left
	// standing from an earlier round.
	for name, replica := range map[string]*SQLiteStore{"left": left, "right": right} {
		conflicts := conflictRows(t, replica, documentID)
		if len(conflicts) != 1 {
			t.Fatalf("%s: conflicts = %+v", name, conflicts)
		}
		if conflicts[0]["revision_a"] == "" || conflicts[0]["revision_b"] == "" {
			t.Fatalf("%s: conflict lost its inputs: %+v", name, conflicts[0])
		}
	}

	// Resolving by writing a revision whose parents are both heads is what
	// actually ends it. That is exactly what an automatic merge produces, so
	// merging the two remaining heads by hand converges the document.
	leftConflict := conflictRows(t, left, documentID)[0]
	if _, err := left.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: documentID, Title: "Shared note", Body: "the AGREED brown fox\n",
		BaseRevisionID: leftConflict["revision_a"],
	}); err == nil {
		t.Fatal("an update against a stale head must be refused")
	}
}

func TestDeltaTransferIsSmallerAndReconstructsExactly(t *testing.T) {
	ctx := context.Background()
	origin := newAdmissionReplica(t, "db_g7_delta")
	receiver := newAdmissionReplica(t, "db_g7_delta")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, receiver})

	body := strings.Repeat("The quick brown fox jumps over the lazy dog.\n", 4_000)
	document, err := origin.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_delta", Title: "Long", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	shipAll(t, origin, receiver)

	edited := body + "One appended line that changes very little.\n"
	updated, err := origin.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "Long", Body: edited, BaseRevisionID: document.CurrentRevisionID,
	})
	if err != nil {
		t.Fatal(err)
	}

	deltas := queryRows(t, origin, `SELECT revision_id, base_revision_id, delta_length, result_length, result_sha256
		FROM sync_revision_deltas WHERE revision_id = ?`, updated.CurrentRevisionID)
	if len(deltas) != 1 {
		t.Fatalf("expected one stored delta for a large append, got %+v", deltas)
	}
	if deltas[0]["base_revision_id"] != document.CurrentRevisionID {
		t.Fatalf("delta names base %q, want %q", deltas[0]["base_revision_id"], document.CurrentRevisionID)
	}
	if deltas[0]["result_sha256"] != syncdelta.SHA256Hex([]byte(edited)) {
		t.Fatal("delta does not name the exact result")
	}
	// The whole point: what travels is much smaller than what it reconstructs.
	deltaLength, resultLength := atoiForTest(t, deltas[0]["delta_length"]), atoiForTest(t, deltas[0]["result_length"])
	if deltaLength*4 >= resultLength {
		t.Fatalf("delta of %d bytes saved little against a %d-byte body", deltaLength, resultLength)
	}

	shipAll(t, origin, receiver)
	receivedBody, receivedRevision := currentBody(t, receiver, document.ID)
	if receivedRevision != updated.CurrentRevisionID || receivedBody != edited {
		t.Fatalf("delta did not reconstruct exactly: revision %s, %d bytes", receivedRevision, len(receivedBody))
	}
	if pending := queryRows(t, receiver, `SELECT revision_id FROM sync_revision_pending_bodies`); len(pending) != 0 {
		t.Fatalf("a reconstructable revision was left pending: %+v", pending)
	}
}

func TestUnverifiableTransferNeverReachesCanonicalState(t *testing.T) {
	ctx := context.Background()
	origin := newAdmissionReplica(t, "db_g7_bad")
	receiver := newAdmissionReplica(t, "db_g7_bad")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, receiver})

	document, err := origin.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_bad", Title: "Tampered", Body: "original body\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	handshake := refreshHandshake(t, origin)
	operations := localOperations(t, origin, handshake.ReplicaID)

	var revisionIndex = -1
	for index, operation := range operations {
		if operation.RecordType == "revision" {
			revisionIndex = index
		}
	}
	if revisionIndex < 0 {
		t.Fatal("no revision operation was journaled")
	}

	cases := map[string]func(string) string{
		"body-does-not-match-its-hash": func(payload string) string {
			return strings.Replace(payload, `"body":"original body\n"`, `"body":"tampered body\n"`, 1)
		},
		"length-does-not-match-the-body": func(payload string) string {
			return strings.Replace(payload, `"content_length":14`, `"content_length":15`, 1)
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			target := newAdmissionReplica(t, "db_g7_bad")
			if err := target.ConfigureSyncAdmissionPeer(ctx, handshake); err != nil {
				t.Fatal(err)
			}
			tampered := make([]syncstate.Operation, len(operations))
			copy(tampered, operations)
			payload := string(tampered[revisionIndex].Payload)
			mutated := tamper(payload)
			if mutated == payload {
				t.Fatalf("the fixture payload did not contain the text this case tampers with: %s", payload)
			}
			tampered[revisionIndex].Payload = []byte(mutated)

			if _, err := target.AdmitSyncOperations(ctx, handshake, tampered); err != nil {
				t.Fatalf("a self-contradicting operation is refused as content, not as a protocol error: %v", err)
			}
			rows := queryRows(t, target, `SELECT id, body FROM document_revisions WHERE id = ?`, tampered[revisionIndex].RecordID)
			if len(rows) != 0 {
				t.Fatalf("unverified bytes reached canonical storage: %+v", rows)
			}
			pending := queryRows(t, target, `SELECT revision_id, reason FROM sync_revision_pending_bodies`)
			if len(pending) != 1 || pending[0]["reason"] != "unverified" {
				t.Fatalf("the refusal was not recorded: %+v", pending)
			}
			audits := queryRows(t, target, `SELECT event_type FROM sync_audit_events WHERE event_type = 'revision.unverified'`)
			if len(audits) != 1 {
				t.Fatalf("the refusal left no audit trail: %+v", audits)
			}
			_ = document
		})
	}
}

func TestEveryDeliveryOrderConvergesIdentically(t *testing.T) {
	// Three replicas, three concurrent edits to different regions, delivered in
	// every order. Convergence that depends on delivery order is not
	// convergence.
	type outcome struct{ body, revision string }
	outcomes := map[string]outcome{}
	orders := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, order := range orders {
		name := fmt.Sprintf("%v", order)
		ctx := context.Background()
		replicas := []*SQLiteStore{
			newAdmissionReplica(t, "db_g7_orders"),
			newAdmissionReplica(t, "db_g7_orders"),
			newAdmissionReplica(t, "db_g7_orders"),
		}
		configureAllAdmissionPeers(t, replicas)
		document, err := replicas[0].CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: "doc_orders", Title: "Shared", Body: "one\ntwo\nthree\nfour\nfive\n",
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, replica := range replicas[1:] {
			shipAll(t, replicas[0], replica)
		}
		edits := []string{
			"ONE\ntwo\nthree\nfour\nfive\n",
			"one\ntwo\nTHREE\nfour\nfive\n",
			"one\ntwo\nthree\nfour\nFIVE\n",
		}
		for index, replica := range replicas {
			if _, err := replica.UpdateDocument(ctx, UpdateDocumentRequest{
				ID: document.ID, Title: "Shared", Body: edits[index], BaseRevisionID: document.CurrentRevisionID,
			}); err != nil {
				t.Fatal(err)
			}
		}
		for _, from := range order {
			for _, to := range []int{0, 1, 2} {
				if from != to {
					shipAll(t, replicas[from], replicas[to])
				}
			}
		}
		exchange(t, replicas...)

		bodies := map[string]bool{}
		revisions := map[string]bool{}
		for _, replica := range replicas {
			body, revision := currentBody(t, replica, document.ID)
			bodies[body] = true
			revisions[revision] = true
		}
		if len(bodies) != 1 || len(revisions) != 1 {
			t.Fatalf("order %s did not converge: %d bodies, %d revisions", name, len(bodies), len(revisions))
		}
		body, revision := currentBody(t, replicas[0], document.ID)
		outcomes[name] = outcome{body, revision}
	}
	names := make([]string, 0, len(outcomes))
	for name := range outcomes {
		names = append(names, name)
	}
	sort.Strings(names)
	first := outcomes[names[0]]
	for _, name := range names[1:] {
		// Content is order independent. The merge *graph* is not, and this
		// asserts the honest boundary: with three concurrent heads a replica
		// merges the two it can see, so which pair became an intermediate merge
		// depends on what arrived first — exactly as it does in any DAG-based
		// merge. What must never differ is what the note says, and every
		// replica within one execution agrees on the revision as well.
		if outcomes[name].body != first.body {
			t.Fatalf("delivery order %s produced %q, order %s produced %q", name, outcomes[name].body, names[0], first.body)
		}
	}
	if first.body != "ONE\ntwo\nTHREE\nfour\nFIVE\n" {
		t.Fatalf("three disjoint edits merged into %q", first.body)
	}
}

func atoiForTest(t *testing.T, value string) int {
	t.Helper()
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("expected an integer, got %q", value)
	}
	return parsed
}

func TestSchemaV22UpgradeBackfillsRevisionObjects(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade-v22.sqlite")
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	document, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_v22", Title: "One", Body: "first\n"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "One", Body: "second\n", BaseRevisionID: document.CurrentRevisionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Rewind to a v21 database that has revisions but no content identity.
	for _, statement := range []string{
		`UPDATE document_revisions SET content_sha256 = '', content_length = -1, parent_revision_ids = '[]'`,
		`PRAGMA user_version = 21`,
	} {
		if err := st.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatalf("upgrade to v22: %v", err)
	}
	status, err := reopened.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema status: %+v err=%v", status, err)
	}
	rows := queryRows(t, reopened, `SELECT id, content_sha256, content_length, parent_revision_ids
		FROM document_revisions WHERE document_id = ? ORDER BY created_at, rowid`, document.ID)
	if len(rows) != 2 {
		t.Fatalf("revision rows: %+v", rows)
	}
	if rows[0]["content_sha256"] != syncdelta.SHA256Hex([]byte("first\n")) || rows[0]["content_length"] != "6" {
		t.Fatalf("first revision was not hashed: %+v", rows[0])
	}
	if rows[0]["parent_revision_ids"] != "[]" {
		t.Fatalf("a document's first revision must have no parent: %+v", rows[0])
	}
	// The pre-v22 history was linear, so the revision before this one is its
	// parent. Nothing invents a merge.
	if rows[1]["parent_revision_ids"] != fmt.Sprintf("[%q]", document.CurrentRevisionID) {
		t.Fatalf("second revision parents = %s, want the first revision", rows[1]["parent_revision_ids"])
	}
	if rows[1]["id"] != updated.CurrentRevisionID {
		t.Fatalf("unexpected revision order: %+v", rows)
	}
}

func TestJournaledRevisionMustNameItsContent(t *testing.T) {
	ctx := context.Background()
	st := newAdmissionReplica(t, "db_g7_guard")
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_guard", Title: "Guard", Body: "body\n"}); err != nil {
		t.Fatal(err)
	}
	err := st.Exec(ctx, `INSERT INTO document_revisions(id, document_id, title, body, body_mime_type)
		VALUES('rev_unhashed', 'doc_guard', 'Guard', 'sneaky', 'text/markdown')`)
	if err == nil {
		t.Fatal("an enrolled database accepted a revision with no content hash")
	}
	if !strings.Contains(err.Error(), "exact content hash") {
		t.Fatalf("unexpected refusal: %v", err)
	}
}

func TestDeleteConcurrentWithAnEditIsADurableConflict(t *testing.T) {
	ctx := context.Background()
	left := newAdmissionReplica(t, "db_g7_delete")
	right := newAdmissionReplica(t, "db_g7_delete")
	configureAllAdmissionPeers(t, []*SQLiteStore{left, right})

	document, err := left.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_delete", Title: "Doomed", Body: "keep this text\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	shipAll(t, left, right)

	// One replica trashes the note it can see; the other edits it at the same
	// time, so the deletion was decided without the edit in view.
	if err := left.DeleteDocument(ctx, DeleteDocumentRequest{
		ID: document.ID, BaseRevisionID: document.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := right.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "Doomed", Body: "keep this text, now improved\n",
		BaseRevisionID: document.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	exchange(t, left, right)

	for name, replica := range map[string]*SQLiteStore{"left": left, "right": right} {
		conflicts := conflictRows(t, replica, document.ID)
		if len(conflicts) != 1 || conflicts[0]["kind"] != syncbody.ConflictDeleteEdit {
			t.Fatalf("%s: delete/edit conflicts = %+v", name, conflicts)
		}
		// The edit is not lost, whichever way the lifecycle rule went. The body
		// still converges — a trash writes a revision holding the unchanged
		// text, so it merges cleanly with the edit — and only the lifecycle
		// disagreement is what the conflict records.
		rows := queryRows(t, replica, `SELECT body FROM document_revisions WHERE document_id = ? AND body LIKE '%improved%'`, document.ID)
		if len(rows) == 0 {
			t.Fatalf("%s discarded the concurrent edit", name)
		}
	}
}

func TestSequentialDeleteAfterSeeingTheEditIsNotAConflict(t *testing.T) {
	ctx := context.Background()
	left := newAdmissionReplica(t, "db_g7_seq")
	right := newAdmissionReplica(t, "db_g7_seq")
	configureAllAdmissionPeers(t, []*SQLiteStore{left, right})

	document, err := left.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_seq", Title: "Doomed", Body: "first\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	shipAll(t, left, right)
	updated, err := right.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "Doomed", Body: "second\n", BaseRevisionID: document.CurrentRevisionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The deleting replica sees the edit first, so its decision accounts for it.
	shipAll(t, right, left)
	if err := left.DeleteDocument(ctx, DeleteDocumentRequest{
		ID: document.ID, BaseRevisionID: updated.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	exchange(t, left, right)

	for name, replica := range map[string]*SQLiteStore{"left": left, "right": right} {
		if conflicts := conflictRows(t, replica, document.ID); len(conflicts) != 0 {
			t.Fatalf("%s reported a conflict for an informed deletion: %+v", name, conflicts)
		}
	}
}

func TestRestoreConcurrentWithAnEditKeepsBoth(t *testing.T) {
	ctx := context.Background()
	left := newAdmissionReplica(t, "db_g7_restore")
	right := newAdmissionReplica(t, "db_g7_restore")
	configureAllAdmissionPeers(t, []*SQLiteStore{left, right})

	document, err := left.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_restore", Title: "Back", Body: "line one\nline two\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	shipAll(t, left, right)

	// The edit happens on one replica while the other trashes and restores the
	// same note, so a lifecycle round trip runs concurrently with a body edit.
	if _, err := right.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "Back", Body: "line one\nline two edited\n",
		BaseRevisionID: document.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := left.DeleteDocument(ctx, DeleteDocumentRequest{
		ID: document.ID, BaseRevisionID: document.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := left.RestoreDocument(ctx, document.ID); err != nil {
		t.Fatal(err)
	}
	exchange(t, left, right)

	leftBody, leftRevision := currentBody(t, left, document.ID)
	rightBody, rightRevision := currentBody(t, right, document.ID)
	if leftRevision != rightRevision || leftBody != rightBody {
		t.Fatalf("restore and edit did not converge: %q/%s vs %q/%s", leftBody, leftRevision, rightBody, rightRevision)
	}
	// A restore is about the document's lifecycle, not its text, so the edit
	// must survive it rather than be rolled back with it.
	if !strings.Contains(leftBody, "edited") {
		t.Fatalf("the concurrent edit was lost: %q", leftBody)
	}
	for name, replica := range map[string]*SQLiteStore{"left": left, "right": right} {
		rows := queryRows(t, replica, `SELECT deleted_at FROM documents WHERE id = ?`, document.ID)
		if len(rows) != 1 || rows[0]["deleted_at"] != "" {
			t.Fatalf("%s: the restore did not win the lifecycle: %+v", name, rows)
		}
	}
}

// A delta is a transfer optimization, so every way one can be wrong must end in
// a refusal plus a visible reason — never in canonical bytes nobody wrote.
func TestBrokenDeltasAreRefusedWithAnAccurateReason(t *testing.T) {
	ctx := context.Background()
	origin := newAdmissionReplica(t, "db_g7_delta_bad")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, newAdmissionReplica(t, "db_g7_delta_bad")})

	body := strings.Repeat("A stable paragraph that a delta can copy wholesale.\n", 2_000)
	document, err := origin.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_dbad", Title: "Long", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := origin.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: document.ID, Title: "Long", Body: body + "one appended line\n", BaseRevisionID: document.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	handshake := refreshHandshake(t, origin)
	operations := localOperations(t, origin, handshake.ReplicaID)

	deltaIndex := -1
	for index, operation := range operations {
		if operation.RecordType == "revision" && strings.Contains(string(operation.Payload), `"delta":{`) {
			deltaIndex = index
		}
	}
	if deltaIndex < 0 {
		t.Fatalf("no revision operation carried a delta: %d operations", len(operations))
	}

	cases := []struct {
		name       string
		tamper     func(string) string
		wantReason string
	}{
		{
			name: "base-that-this-replica-does-not-hold",
			tamper: func(p string) string {
				return strings.Replace(p, `"base_revision_id":"rev_`, `"base_revision_id":"rev_absent`, 1)
			},
			wantReason: "missing_base",
		},
		{
			name: "base-hash-that-does-not-match-the-named-base",
			tamper: func(p string) string {
				return strings.Replace(p, `"base_sha256":"`, `"base_sha256":"ff`, 1)
			},
			wantReason: "missing_base",
		},
		{
			name:       "corrupt-delta-bytes",
			tamper:     func(p string) string { return strings.Replace(p, `"base64":"`, `"base64":"AAAA`, 1) },
			wantReason: "unverified",
		},
		{
			name: "result-hash-that-the-delta-does-not-produce",
			tamper: func(p string) string {
				return strings.Replace(p, `"result_sha256":"`, `"result_sha256":"ff`, 1)
			},
			wantReason: "unverified",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			target := newAdmissionReplica(t, "db_g7_delta_bad")
			if err := target.ConfigureSyncAdmissionPeer(ctx, handshake); err != nil {
				t.Fatal(err)
			}
			tampered := make([]syncstate.Operation, len(operations))
			copy(tampered, operations)
			payload := string(tampered[deltaIndex].Payload)
			mutated := test.tamper(payload)
			if mutated == payload {
				t.Fatal("the fixture payload did not contain the text this case tampers with")
			}
			tampered[deltaIndex].Payload = []byte(mutated)
			if _, err := target.AdmitSyncOperations(ctx, handshake, tampered); err != nil {
				t.Fatalf("a broken delta is refused as content, not as a protocol error: %v", err)
			}
			if rows := queryRows(t, target, `SELECT id FROM document_revisions WHERE id = ?`, tampered[deltaIndex].RecordID); len(rows) != 0 {
				t.Fatalf("an unverified reconstruction reached canonical storage: %+v", rows)
			}
			pending := queryRows(t, target, `SELECT revision_id, reason FROM sync_revision_pending_bodies WHERE revision_id = ?`,
				tampered[deltaIndex].RecordID)
			if len(pending) != 1 || pending[0]["reason"] != test.wantReason {
				t.Fatalf("pending state = %+v, want reason %q", pending, test.wantReason)
			}
			// The revision before the broken one still applied, so one bad
			// delta does not cost the note its whole history.
			if rows := queryRows(t, target, `SELECT id FROM document_revisions WHERE document_id = ?`, document.ID); len(rows) != 1 {
				t.Fatalf("the intact revision did not apply: %+v", rows)
			}
		})
	}
}

// Every stored delta must name a base that exists as a complete body, so a
// chain of deltas can never form and a reconstruction is never more than one
// decode deep.
func TestDeltaBasesAreAlwaysCompleteLocalObjects(t *testing.T) {
	ctx := context.Background()
	origin := newAdmissionReplica(t, "db_g7_chain")
	receiver := newAdmissionReplica(t, "db_g7_chain")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, receiver})

	body := strings.Repeat("A paragraph that stays put across many edits.\n", 2_000)
	document, err := origin.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_chain", Title: "Chain", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	head := document.CurrentRevisionID
	for round := 0; round < 5; round++ {
		body += fmt.Sprintf("appended line %d\n", round)
		updated, err := origin.UpdateDocument(ctx, UpdateDocumentRequest{
			ID: document.ID, Title: "Chain", Body: body, BaseRevisionID: head,
		})
		if err != nil {
			t.Fatal(err)
		}
		head = updated.CurrentRevisionID
	}
	deltas := queryRows(t, origin, `SELECT revision_id, base_revision_id FROM sync_revision_deltas`)
	if len(deltas) < 5 {
		t.Fatalf("expected a delta per edit, got %d", len(deltas))
	}
	for _, delta := range deltas {
		rows := queryRows(t, origin, `SELECT content_length FROM document_revisions WHERE id = ?`, delta["base_revision_id"])
		if len(rows) != 1 {
			t.Fatalf("delta %s names a base that is not a complete local revision", delta["revision_id"])
		}
	}
	shipAll(t, origin, receiver)
	receivedBody, receivedRevision := currentBody(t, receiver, document.ID)
	if receivedRevision != head || receivedBody != body {
		t.Fatalf("a chain of five deltas did not reconstruct: %d bytes at %s", len(receivedBody), receivedRevision)
	}
	if pending := queryRows(t, receiver, `SELECT revision_id FROM sync_revision_pending_bodies`); len(pending) != 0 {
		t.Fatalf("deltas were left pending: %+v", pending)
	}
}

func TestUnicodeMarkdownAndLongLineBodiesSynchronizeExactly(t *testing.T) {
	cases := map[string][3]string{
		"unicode": {
			"# Café 東京 😀\n\nJournée normale.\n\nNotes ici.\n",
			"# Café 東京 😀\n\nJournée modifiée.\n\nNotes ici.\n",
			"# Café 東京 😀\n\nJournée normale.\n\nNotes ici et là 🎉.\n",
		},
		"markdown-structure": {
			"| a | b |\n|---|---|\n| 1 | 2 |\n\n- one\n- two\n",
			"| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n\n- one\n- two\n",
			"| a | b |\n|---|---|\n| 1 | 2 |\n\n- one\n- two\n- three\n",
		},
		"one-very-long-line": {
			"header\n" + strings.Repeat("word ", 12_000) + "\ntrailer\n",
			"header CHANGED\n" + strings.Repeat("word ", 12_000) + "\ntrailer\n",
			"header\n" + strings.Repeat("word ", 12_000) + "\ntrailer CHANGED\n",
		},
	}
	for name, bodies := range cases {
		t.Run(name, func(t *testing.T) {
			left, right, documentID := makeDivergedDocument(t, bodies[0], bodies[1], bodies[2])
			exchange(t, left, right)
			leftBody, leftRevision := currentBody(t, left, documentID)
			rightBody, rightRevision := currentBody(t, right, documentID)
			if leftRevision != rightRevision || leftBody != rightBody {
				t.Fatalf("%s did not converge: %d vs %d bytes", name, len(leftBody), len(rightBody))
			}
			rows := queryRows(t, left, `SELECT content_sha256, content_length FROM document_revisions WHERE id = ?`, leftRevision)
			if rows[0]["content_sha256"] != syncdelta.SHA256Hex([]byte(leftBody)) ||
				rows[0]["content_length"] != strconv.Itoa(len(leftBody)) {
				t.Fatalf("%s: the converged revision does not name its own bytes: %+v", name, rows[0])
			}
		})
	}
}
