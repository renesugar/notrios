package syncwire

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/syncstate"
)

// The goldens are produced by `performance/v0.7-g9/generate_goldens.py`, a
// second implementation written from FORMAT.md rather than from this package.
// Comparing against bytes this encoder produced would prove only that it is
// self-consistent; comparing against an independent implementation of the same
// written specification is what makes the format a specification at all.
const goldenDirectory = "../../performance/v0.7-g9"

type goldenInputs struct {
	Cases []struct {
		Name       string              `json:"name"`
		Kind       string              `json:"kind"`
		Operations []goldenOperation   `json:"operations"`
		Envelope   goldenEnvelopeInput `json:"envelope"`
	} `json:"cases"`
}

type goldenOperation struct {
	ReplicaID    string `json:"replica_id"`
	Sequence     int64  `json:"sequence"`
	HLCWallMS    int64  `json:"hlc_wall_ms"`
	HLCLogical   int64  `json:"hlc_logical"`
	Kind         string `json:"kind"`
	RecordType   string `json:"record_type"`
	RecordID     string `json:"record_id"`
	CreatedAt    string `json:"created_at"`
	Dependencies []struct {
		ReplicaID string `json:"replica_id"`
		Sequence  int64  `json:"sequence"`
	} `json:"dependencies"`
	Payload string `json:"payload"`
}

type goldenEnvelopeInput struct {
	ProtocolMajor   int               `json:"protocol_major"`
	ProtocolMinor   int               `json:"protocol_minor"`
	DatabaseID      string            `json:"database_id"`
	SenderReplicaID string            `json:"sender_replica_id"`
	StateVector     map[string]int64  `json:"state_vector"`
	Operations      []goldenOperation `json:"operations"`
}

type goldenOutputs struct {
	Cases []struct {
		Name       string `json:"name"`
		Kind       string `json:"kind"`
		Hex        string `json:"hex"`
		ByteLength int    `json:"byte_length"`
	} `json:"cases"`
}

func (g goldenOperation) operation() syncstate.Operation {
	operation := syncstate.Operation{
		ReplicaID: g.ReplicaID, Sequence: g.Sequence,
		OperationID: fmt.Sprintf("%s:%020d", g.ReplicaID, g.Sequence),
		Kind:        g.Kind, RecordType: g.RecordType, RecordID: g.RecordID,
		CreatedAt: g.CreatedAt, Payload: []byte(g.Payload),
		HLC: syncstate.HLC{WallMS: g.HLCWallMS, Logical: g.HLCLogical},
	}
	for _, dependency := range g.Dependencies {
		operation.Dependencies = append(operation.Dependencies,
			syncstate.OperationRef{ReplicaID: dependency.ReplicaID, Sequence: dependency.Sequence})
	}
	return operation
}

func loadJSON(t *testing.T, name string, into any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goldenDirectory, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
}

func TestCanonicalBytesMatchIndependentGoldens(t *testing.T) {
	t.Parallel()
	var inputs goldenInputs
	var outputs goldenOutputs
	loadJSON(t, "golden-inputs.json", &inputs)
	loadJSON(t, "goldens.json", &outputs)

	if len(inputs.Cases) != len(outputs.Cases) || len(inputs.Cases) == 0 {
		t.Fatalf("%d inputs and %d goldens", len(inputs.Cases), len(outputs.Cases))
	}
	for index, input := range inputs.Cases {
		golden := outputs.Cases[index]
		if golden.Name != input.Name || golden.Kind != input.Kind {
			t.Fatalf("case %d: goldens are out of step with their inputs (%q/%q vs %q/%q)",
				index, golden.Name, golden.Kind, input.Name, input.Kind)
		}
		t.Run(input.Name, func(t *testing.T) {
			var encoded []byte
			var err error
			switch input.Kind {
			case "operations":
				operations := make([]syncstate.Operation, 0, len(input.Operations))
				for _, operation := range input.Operations {
					operations = append(operations, operation.operation())
				}
				encoded, err = EncodeOperations(operations, Limits{})
			case "envelope":
				envelope := Envelope{
					ProtocolMajor: input.Envelope.ProtocolMajor, ProtocolMinor: input.Envelope.ProtocolMinor,
					DatabaseID: input.Envelope.DatabaseID, SenderReplicaID: input.Envelope.SenderReplicaID,
					StateVector: syncstate.Vector{},
				}
				for replicaID, sequence := range input.Envelope.StateVector {
					envelope.StateVector[replicaID] = sequence
				}
				for _, operation := range input.Envelope.Operations {
					envelope.Operations = append(envelope.Operations, operation.operation())
				}
				encoded, err = EncodeEnvelope(envelope, Limits{})
			default:
				t.Fatalf("unknown golden kind %q", input.Kind)
			}
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(encoded) != golden.Hex {
				t.Fatalf("canonical bytes differ from the independent generator\n go: %s\npy: %s",
					hex.EncodeToString(encoded), golden.Hex)
			}
			if len(encoded) != golden.ByteLength {
				t.Fatalf("length = %d, golden says %d", len(encoded), golden.ByteLength)
			}

			// The goldens must also decode, or they would pin bytes nothing can
			// read.
			raw, err := hex.DecodeString(golden.Hex)
			if err != nil {
				t.Fatal(err)
			}
			switch input.Kind {
			case "operations":
				decoded, err := DecodeOperations(raw, Limits{})
				if err != nil {
					t.Fatalf("golden bytes did not decode: %v", err)
				}
				if len(decoded) != len(input.Operations) {
					t.Fatalf("decoded %d operations, expected %d", len(decoded), len(input.Operations))
				}
			case "envelope":
				decoded, err := DecodeEnvelope(raw, Limits{})
				if err != nil {
					t.Fatalf("golden bytes did not decode: %v", err)
				}
				if decoded.DatabaseID != input.Envelope.DatabaseID ||
					len(decoded.Operations) != len(input.Envelope.Operations) ||
					len(decoded.StateVector) != len(input.Envelope.StateVector) {
					t.Fatalf("decoded envelope does not match its input: %+v", decoded)
				}
			}
		})
	}
}
