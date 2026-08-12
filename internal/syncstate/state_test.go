package syncstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestCompareVectorsAndMissingRanges(t *testing.T) {
	tests := []struct {
		name     string
		left     Vector
		right    Vector
		relation VectorRelation
	}{
		{"equal", Vector{"a": 2}, Vector{"a": 2}, VectorEqual},
		{"ahead", Vector{"a": 3}, Vector{"a": 2}, VectorAhead},
		{"behind", Vector{"a": 1}, Vector{"a": 2}, VectorBehind},
		{"concurrent", Vector{"a": 2, "b": 1}, Vector{"a": 1, "b": 2}, VectorConcurrent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CompareVectors(test.left, test.right)
			if err != nil || got != test.relation {
				t.Fatalf("CompareVectors = %q, %v; want %q", got, err, test.relation)
			}
		})
	}
	plan, err := PlanMissing(Vector{"a": 1}, Vector{"a": 7, "b": 2}, []OperationRef{{ReplicaID: "a", Sequence: 3}, {ReplicaID: "a", Sequence: 6}})
	if err != nil {
		t.Fatal(err)
	}
	want := []SequenceRange{
		{ReplicaID: "a", Start: 2, End: 2},
		{ReplicaID: "a", Start: 4, End: 5},
		{ReplicaID: "a", Start: 7, End: 7},
		{ReplicaID: "b", Start: 1, End: 2},
	}
	if !reflect.DeepEqual(plan.Ranges, want) || plan.SequenceCount != 6 || plan.MoreAvailable {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	bounded, err := PlanMissing(Vector{}, Vector{"a": MaxPlannedSequences + 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bounded.MoreAvailable || bounded.SequenceCount != MaxPlannedSequences || bounded.Ranges[0].End != MaxPlannedSequences {
		t.Fatalf("unbounded plan: %+v", bounded)
	}
}

func TestVectorAndPlanBoundsRefuseWorkInsteadOfTruncatingState(t *testing.T) {
	oversized := Vector{}
	for index := 0; index <= MaxStateVectorEntries; index++ {
		oversized[fmt.Sprintf("replica_%04d", index)] = 0
	}
	if err := ValidateVector(oversized); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("oversized vector error = %v", err)
	}
	if _, err := PlanMissing(Vector{}, Vector{"replica_a": MaxSequence + 1}, nil); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("overflowing vector error = %v", err)
	}
}

func TestHandshakeCompatibilityIsClosedAndExplicit(t *testing.T) {
	remote := NewHandshake("db_one", "replica_remote", MaxCompatibleSchema, Vector{"replica_remote": 0})
	if err := ValidateHandshake("db_one", "replica_local", MinCompatibleSchema, remote); err != nil {
		t.Fatalf("compatible handshake: %v", err)
	}
	tests := []struct {
		name string
		edit func(*Handshake)
		want error
	}{
		{"database", func(h *Handshake) { h.DatabaseID = "db_two" }, ErrDatabaseMismatch},
		{"protocol major", func(h *Handshake) { h.ProtocolMajor++ }, ErrProtocolMismatch},
		{"minor range", func(h *Handshake) { h.ProtocolMinMinor = ProtocolMinor + 1; h.ProtocolMaxMinor = ProtocolMinor + 1 }, ErrProtocolMismatch},
		{"schema", func(h *Handshake) { h.SchemaVersion = MaxCompatibleSchema + 1 }, ErrSchemaMismatch},
		{"self-inconsistent schema range", func(h *Handshake) {
			h.MinCompatibleSchema = MinCompatibleSchema
			h.MaxCompatibleSchema = MinCompatibleSchema
		}, ErrSchemaMismatch},
		{"required capability", func(h *Handshake) { h.RequiredCapabilities = append(h.RequiredCapabilities, "sync.unknown.v1") }, ErrCapabilityMismatch},
		{"missing own vector", func(h *Handshake) { h.StateVector = Vector{} }, ErrInvalidState},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := remote
			changed.RequiredCapabilities = append([]string(nil), remote.RequiredCapabilities...)
			changed.StateVector = CloneVector(remote.StateVector)
			test.edit(&changed)
			if err := ValidateHandshake("db_one", "replica_local", MinCompatibleSchema, changed); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNormalizeOperationRejectsAmbiguityAndExhaustion(t *testing.T) {
	operation := Operation{
		ReplicaID: "replica_a", Sequence: 2, OperationID: "replica_a:00000000000000000002",
		Kind: "sync.noop", RecordType: "sync_noop", RecordID: "noop_2",
		Payload: json.RawMessage(`{ "ok": true }`), CreatedAt: time.Unix(1, 0).UTC().Format(time.RFC3339Nano),
		Dependencies: []OperationRef{{ReplicaID: "replica_b", Sequence: 1}, {ReplicaID: "replica_a", Sequence: 1}},
	}
	normalized, encoded, err := NormalizeOperation(operation)
	if err != nil {
		t.Fatal(err)
	}
	if string(normalized.Payload) != `{"ok":true}` || normalized.Dependencies[0].ReplicaID != "replica_a" {
		t.Fatalf("operation was not canonicalized: %+v", normalized)
	}
	decoded, err := DecodeOperation(encoded)
	if err != nil || !reflect.DeepEqual(decoded, normalized) {
		t.Fatalf("DecodeOperation = %+v, %v", decoded, err)
	}
	bad := operation
	bad.OperationID = "reused"
	if _, _, err := NormalizeOperation(bad); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("bad operation id error = %v", err)
	}
	bad = operation
	bad.Sequence = MaxSequence + 1
	bad.OperationID = fmt.Sprintf("%s:%020d", bad.ReplicaID, bad.Sequence)
	if _, _, err := NormalizeOperation(bad); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("sequence exhaustion error = %v", err)
	}
}

// This model test uses only vectors and immutable operation IDs. It proves
// that arbitrary shuffle/duplicate/drop-then-deliver schedules cannot advance
// a contiguous vector across a gap.
func TestThreeReplicaModelEventuallyConvergesWithoutClosingGaps(t *testing.T) {
	const replicas = 3
	const operations = 25
	for seed := int64(1); seed <= 100; seed++ {
		random := rand.New(rand.NewSource(seed))
		vectors := make([]Vector, replicas)
		pending := make([]map[OperationRef]bool, replicas)
		for receiver := 0; receiver < replicas; receiver++ {
			vectors[receiver] = Vector{}
			pending[receiver] = map[OperationRef]bool{}
		}
		deliver := func(receiver int, ref OperationRef) {
			pending[receiver][ref] = true
			for next := vectors[receiver][ref.ReplicaID] + 1; pending[receiver][OperationRef{ReplicaID: ref.ReplicaID, Sequence: next}]; next++ {
				vectors[receiver][ref.ReplicaID] = next
			}
		}
		refs := make([]OperationRef, 0, replicas*operations)
		for source := 0; source < replicas; source++ {
			id := fmt.Sprintf("replica_%d", source)
			for sequence := int64(1); sequence <= operations; sequence++ {
				refs = append(refs, OperationRef{ReplicaID: id, Sequence: sequence})
			}
		}
		firstWave := append([]OperationRef(nil), refs...)
		random.Shuffle(len(firstWave), func(i, j int) { firstWave[i], firstWave[j] = firstWave[j], firstWave[i] })
		for _, ref := range firstWave {
			for receiver := 0; receiver < replicas; receiver++ {
				if random.Intn(4) == 0 { // deliberate first-wave drop
					continue
				}
				deliver(receiver, ref)
				if random.Intn(3) == 0 { // duplicate replay
					deliver(receiver, ref)
				}
			}
		}
		// A deliberately absent first sequence must not be inferred from later
		// receipt. Then the second wave delivers every missing operation.
		vectors[0]["replica_0"] = 0
		delete(pending[0], OperationRef{ReplicaID: "replica_0", Sequence: 1})
		if vectors[0]["replica_0"] != 0 {
			t.Fatal("model closed an explicit gap")
		}
		for _, ref := range refs {
			for receiver := 0; receiver < replicas; receiver++ {
				deliver(receiver, ref)
			}
		}
		for receiver := 0; receiver < replicas; receiver++ {
			for source := 0; source < replicas; source++ {
				if got := vectors[receiver][fmt.Sprintf("replica_%d", source)]; got != operations {
					t.Fatalf("seed %d receiver %d source %d = %d", seed, receiver, source, got)
				}
			}
		}
	}
}
