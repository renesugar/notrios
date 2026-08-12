package syncmerge

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
)

func baselineRecord(recordType, id, payload string, active bool) *Record {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		panic(err)
	}
	registers := make(map[string]Register, len(fields))
	for field, value := range fields {
		registers[field] = Register{Value: value}
	}
	return &Record{Type: recordType, ID: id, Active: active, Fields: registers}
}

func operation(replica string, sequence, wall int64, kind, recordType, recordID, payload string) syncstate.Operation {
	return syncstate.Operation{
		ReplicaID: replica, Sequence: sequence, OperationID: fmt.Sprintf("%s:%020d", replica, sequence),
		Kind: kind, RecordType: recordType, RecordID: recordID, Payload: json.RawMessage(payload),
		HLC: syncstate.HLC{WallMS: wall}, CreatedAt: time.UnixMilli(wall).UTC().Format(time.RFC3339Nano),
	}
}

func convergenceFixture() ([]*Record, []Membership, []syncstate.Operation) {
	baseline := []*Record{
		baselineRecord("collection", "default", `{"name":"Default","description":""}`, true),
		baselineRecord("notebook", RecoveredNotebookID, `{"parent_id":"","name":"Recovered","icon_emoji":"","position":0}`, true),
		baselineRecord("notebook", "nb_a", `{"parent_id":"","name":"A","icon_emoji":"","position":1}`, true),
		baselineRecord("notebook", "nb_b", `{"parent_id":"","name":"B","icon_emoji":"","position":2}`, true),
		baselineRecord("tag", "tag_a", `{"name":"alpha"}`, true),
		baselineRecord("document", "doc_a", `{"collection_id":"default","notebook_id":"nb_a","title":"base","body_mime_type":"text/markdown"}`, true),
	}
	operations := []syncstate.Operation{
		operation("replica_a", 1, 100, "record.update", "document", "doc_a", `{"title":"remote title"}`),
		operation("replica_b", 1, 150, "record.update", "document", "doc_a", `{"notebook_id":"missing_notebook"}`),
		operation("replica_a", 2, 200, "membership.add", "document_tag", "doc_a:tag_a", `{"document_id":"doc_a","tag_id":"tag_a"}`),
		operation("replica_b", 2, 250, "membership.remove", "document_tag", "doc_a:tag_a", `{"document_id":"doc_a","tag_id":"tag_a"}`),
		operation("replica_a", 3, 300, "record.update", "notebook", "nb_a", `{"parent_id":"nb_b","name":"Same"}`),
		operation("replica_b", 3, 350, "record.update", "notebook", "nb_b", `{"parent_id":"nb_a","name":"Same"}`),
		operation("replica_a", 4, 400, "document.trash", "document", "doc_a", `{}`),
		operation("replica_b", 4, 450, "document.restore", "document", "doc_a", `{}`),
	}
	return baseline, nil, operations
}

func projectionBytes(t *testing.T, projection Projection) []byte {
	t.Helper()
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestConvergeExhaustiveDeliveryOrderIsByteEquivalent(t *testing.T) {
	baseline, memberships, operations := convergenceFixture()
	want, err := Converge(baseline, memberships, operations)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes := projectionBytes(t, want)
	indexes := make([]int, len(operations))
	for i := range indexes {
		indexes[i] = i
	}
	checked := 0
	var visit func(int)
	visit = func(position int) {
		if position == len(indexes) {
			shuffled := make([]syncstate.Operation, len(indexes))
			for i, index := range indexes {
				shuffled[i] = operations[index]
			}
			got, convergeErr := Converge(baseline, memberships, shuffled)
			if convergeErr != nil {
				t.Fatal(convergeErr)
			}
			if !reflect.DeepEqual(projectionBytes(t, got), wantBytes) {
				t.Fatalf("projection differs for permutation %v", indexes)
			}
			checked++
			return
		}
		for i := position; i < len(indexes); i++ {
			indexes[position], indexes[i] = indexes[i], indexes[position]
			visit(position + 1)
			indexes[position], indexes[i] = indexes[i], indexes[position]
		}
	}
	visit(0)
	if checked != 40320 {
		t.Fatalf("checked %d permutations", checked)
	}
	doc := want.Records[key("document", "doc_a")]
	if String(doc, "title", "") != "remote title" || !doc.Active || want.DocumentNotebook["doc_a"] != RecoveredNotebookID {
		t.Fatalf("document merge = %+v homes=%v", doc, want.DocumentNotebook)
	}
	if want.Memberships["doc_a:tag_a"].Present {
		t.Fatal("later remove must win membership")
	}
	if want.NotebookParent["nb_b"] != "nb_a" || want.NotebookParent["nb_a"] != "" {
		t.Fatalf("cycle repair = %v", want.NotebookParent)
	}
	if len(want.Repairs) < 2 {
		t.Fatalf("repair report = %+v", want.Repairs)
	}
}

func TestConvergeRandomizedReplayClockSkewAndFieldIndependence(t *testing.T) {
	baseline, memberships, operations := convergenceFixture()
	want, err := Converge(baseline, memberships, operations)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes := projectionBytes(t, want)
	for seed := int64(0); seed < 250; seed++ {
		random := rand.New(rand.NewSource(seed))
		shuffled := append([]syncstate.Operation(nil), operations...)
		random.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		for duplicate := 0; duplicate < random.Intn(8); duplicate++ {
			shuffled = append(shuffled, shuffled[random.Intn(len(shuffled))])
		}
		got, err := Converge(baseline, memberships, shuffled)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if !reflect.DeepEqual(projectionBytes(t, got), wantBytes) {
			t.Fatalf("seed %d diverged", seed)
		}
	}
	// An implausibly old wall clock still orders deterministically; it does not
	// overwrite a later field merely because it was delivered last.
	old := operation("replica_z", 1, 1, "record.update", "document", "doc_a", `{"title":"clock reset"}`)
	got, err := Converge(baseline, memberships, append(operations, old))
	if err != nil || String(got.Records[key("document", "doc_a")], "title", "") != "remote title" {
		t.Fatalf("clock skew = %+v, %v", got, err)
	}
}

func TestPurgeCertificateOutlivesDocumentAndRejectsSameIDRestore(t *testing.T) {
	baseline := []*Record{baselineRecord("document", "doc_dead", `{"collection_id":"default","notebook_id":"","title":"gone","body_mime_type":"text/markdown"}`, false)}
	purge := operation("replica_a", 1, 100, "document.purge", "document", "doc_dead", `{"document_id":"doc_dead","signer_replica_id":"replica_a","sequence":1,"signature":"fixture-signature"}`)
	projection, err := Converge(baseline, nil, []syncstate.Operation{purge})
	if err != nil {
		t.Fatal(err)
	}
	if projection.Deaths["doc_dead"].Signature == "" {
		t.Fatal("death certificate was not retained")
	}
	restore := operation("replica_b", 1, 200, "document.restore", "document", "doc_dead", `{}`)
	if _, err := Converge(baseline, nil, []syncstate.Operation{purge, restore}); err == nil {
		t.Fatal("same-ID restore after purge was accepted")
	}
	bad := purge
	bad.Payload = json.RawMessage(`{"document_id":"doc_dead","signer_replica_id":"replica_a","sequence":1,"signature":""}`)
	if _, err := Converge(baseline, nil, []syncstate.Operation{bad}); err == nil {
		t.Fatal("unsigned certificate was accepted")
	}
}

func TestOrderTieBreaksByReplicaThenSequence(t *testing.T) {
	orders := []Order{{WallMS: 1, Logical: 2, ReplicaID: "b", Sequence: 1}, {WallMS: 1, Logical: 2, ReplicaID: "a", Sequence: 9}, {WallMS: 1, Logical: 2, ReplicaID: "b", Sequence: 2}}
	sort.Slice(orders, func(i, j int) bool { return Compare(orders[i], orders[j]) < 0 })
	if orders[0].ReplicaID != "a" || orders[1].Sequence != 1 || orders[2].Sequence != 2 {
		t.Fatalf("order = %+v", orders)
	}
}
