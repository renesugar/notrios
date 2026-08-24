// Package syncstate implements the transport-neutral G5 state-vector and
// admission model. It deliberately knows nothing about SQLite, HTTP, folders,
// cryptography, or canonical record conflict semantics.
package syncstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	ProtocolMajor = 1
	ProtocolMinor = 0

	MinCompatibleSchema = 24
	MaxCompatibleSchema = 26

	MaxStateVectorEntries = 1_024
	MaxMissingRanges      = 1_024
	MaxPlannedSequences   = int64(10_000)
	MaxSequenceSkew       = int64(10_000)
	MaxSequence           = int64(math.MaxInt64 - 1)

	MaxPendingOperations   = 10_000
	MaxPendingBytes        = int64(64 << 20)
	MaxAdmissionBatchBytes = int64(16 << 20)
	MaxPayloadBytes        = 512 << 10
	MaxOperationBytes      = 1 << 20
	MaxDependencies        = 64
)

var (
	ErrDatabaseMismatch   = errors.New("sync database mismatch")
	ErrProtocolMismatch   = errors.New("sync protocol mismatch")
	ErrSchemaMismatch     = errors.New("sync schema mismatch")
	ErrCapabilityMismatch = errors.New("sync capability mismatch")
	ErrInvalidState       = errors.New("invalid sync state")
)

var requiredCapabilities = []string{
	"sync.dependencies.v1",
	"sync.metadata-lww.v1",
	"sync.operations.v1",
	"sync.state-vectors.v1",
}

// RequiredCapabilities returns a copy of G5's closed required capability set.
func RequiredCapabilities() []string {
	return append([]string(nil), requiredCapabilities...)
}

type Vector map[string]int64

func CloneVector(vector Vector) Vector {
	cloned := make(Vector, len(vector))
	for replicaID, sequence := range vector {
		cloned[replicaID] = sequence
	}
	return cloned
}

func ValidateVector(vector Vector) error {
	if len(vector) > MaxStateVectorEntries {
		return fmt.Errorf("%w: state vector has more than %d entries", ErrInvalidState, MaxStateVectorEntries)
	}
	for replicaID, sequence := range vector {
		if err := validateIdentifier("replica id", replicaID); err != nil {
			return err
		}
		if sequence < 0 || sequence > MaxSequence {
			return fmt.Errorf("%w: sequence for replica %q is outside 0..%d", ErrInvalidState, replicaID, MaxSequence)
		}
	}
	return nil
}

type VectorRelation string

const (
	VectorEqual      VectorRelation = "equal"
	VectorAhead      VectorRelation = "ahead"
	VectorBehind     VectorRelation = "behind"
	VectorConcurrent VectorRelation = "concurrent"
)

func CompareVectors(left, right Vector) (VectorRelation, error) {
	if err := ValidateVector(left); err != nil {
		return "", err
	}
	if err := ValidateVector(right); err != nil {
		return "", err
	}
	leftGreater := false
	rightGreater := false
	for replicaID, sequence := range left {
		other := right[replicaID]
		leftGreater = leftGreater || sequence > other
		rightGreater = rightGreater || other > sequence
	}
	for replicaID, sequence := range right {
		if _, seen := left[replicaID]; seen {
			continue
		}
		rightGreater = rightGreater || sequence > 0
	}
	switch {
	case leftGreater && rightGreater:
		return VectorConcurrent, nil
	case leftGreater:
		return VectorAhead, nil
	case rightGreater:
		return VectorBehind, nil
	default:
		return VectorEqual, nil
	}
}

type Handshake struct {
	DatabaseID           string   `json:"database_id"`
	ReplicaID            string   `json:"replica_id"`
	ProtocolMajor        int      `json:"protocol_major"`
	ProtocolMinMinor     int      `json:"protocol_min_minor"`
	ProtocolMaxMinor     int      `json:"protocol_max_minor"`
	SchemaVersion        int      `json:"schema_version"`
	MinCompatibleSchema  int      `json:"min_compatible_schema"`
	MaxCompatibleSchema  int      `json:"max_compatible_schema"`
	RequiredCapabilities []string `json:"required_capabilities"`
	OptionalCapabilities []string `json:"optional_capabilities"`
	StateVector          Vector   `json:"state_vector"`
}

func NewHandshake(databaseID, replicaID string, schemaVersion int, vector Vector) Handshake {
	return Handshake{
		DatabaseID:           databaseID,
		ReplicaID:            replicaID,
		ProtocolMajor:        ProtocolMajor,
		ProtocolMinMinor:     ProtocolMinor,
		ProtocolMaxMinor:     ProtocolMinor,
		SchemaVersion:        schemaVersion,
		MinCompatibleSchema:  MinCompatibleSchema,
		MaxCompatibleSchema:  MaxCompatibleSchema,
		RequiredCapabilities: RequiredCapabilities(),
		OptionalCapabilities: []string{},
		StateVector:          CloneVector(vector),
	}
}

// ValidateHandshake applies the fixed G5 compatibility table. A successful
// handshake says an already configured peer is compatible; it is not peer
// discovery, enrollment, authentication, or authorization.
func ValidateHandshake(localDatabaseID, localReplicaID string, localSchema int, remote Handshake) error {
	if err := validateIdentifier("local database id", localDatabaseID); err != nil {
		return err
	}
	if err := validateIdentifier("local replica id", localReplicaID); err != nil {
		return err
	}
	if err := validateIdentifier("database id", remote.DatabaseID); err != nil {
		return err
	}
	if err := validateIdentifier("replica id", remote.ReplicaID); err != nil {
		return err
	}
	if remote.DatabaseID != localDatabaseID {
		return fmt.Errorf("%w: local %q remote %q", ErrDatabaseMismatch, localDatabaseID, remote.DatabaseID)
	}
	if remote.ReplicaID == localReplicaID {
		return fmt.Errorf("%w: remote replica is the local replica", ErrInvalidState)
	}
	if remote.ProtocolMajor != ProtocolMajor || remote.ProtocolMinMinor > ProtocolMinor || remote.ProtocolMaxMinor < ProtocolMinor || remote.ProtocolMinMinor < 0 || remote.ProtocolMaxMinor < remote.ProtocolMinMinor {
		return fmt.Errorf("%w: local %d.%d remote %d.%d-%d", ErrProtocolMismatch, ProtocolMajor, ProtocolMinor, remote.ProtocolMajor, remote.ProtocolMinMinor, remote.ProtocolMaxMinor)
	}
	if localSchema < MinCompatibleSchema || localSchema > MaxCompatibleSchema ||
		remote.SchemaVersion < MinCompatibleSchema || remote.SchemaVersion > MaxCompatibleSchema ||
		remote.SchemaVersion < remote.MinCompatibleSchema || remote.SchemaVersion > remote.MaxCompatibleSchema ||
		localSchema < remote.MinCompatibleSchema || localSchema > remote.MaxCompatibleSchema ||
		remote.MinCompatibleSchema > remote.MaxCompatibleSchema {
		return fmt.Errorf("%w: local schema %d remote schema %d range %d-%d", ErrSchemaMismatch, localSchema, remote.SchemaVersion, remote.MinCompatibleSchema, remote.MaxCompatibleSchema)
	}
	if err := validateCapabilities(remote.RequiredCapabilities, remote.OptionalCapabilities); err != nil {
		return err
	}
	if err := ValidateVector(remote.StateVector); err != nil {
		return err
	}
	if _, ok := remote.StateVector[remote.ReplicaID]; !ok {
		return fmt.Errorf("%w: handshake omits its own replica from the state vector", ErrInvalidState)
	}
	return nil
}

func validateCapabilities(required, optional []string) error {
	if len(required)+len(optional) > 64 {
		return fmt.Errorf("%w: too many capabilities", ErrCapabilityMismatch)
	}
	if err := validateSortedUnique("required", required); err != nil {
		return err
	}
	if err := validateSortedUnique("optional", optional); err != nil {
		return err
	}
	supported := make(map[string]bool, len(requiredCapabilities))
	for _, capability := range requiredCapabilities {
		supported[capability] = true
		if !contains(required, capability) {
			return fmt.Errorf("%w: missing base capability %q", ErrCapabilityMismatch, capability)
		}
	}
	for _, capability := range required {
		if !supported[capability] {
			return fmt.Errorf("%w: unsupported required capability %q", ErrCapabilityMismatch, capability)
		}
	}
	for _, capability := range optional {
		if contains(required, capability) {
			return fmt.Errorf("%w: capability %q is required and optional", ErrCapabilityMismatch, capability)
		}
	}
	return nil
}

func validateSortedUnique(label string, values []string) error {
	for index, value := range values {
		if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
			return fmt.Errorf("%w: invalid %s capability", ErrCapabilityMismatch, label)
		}
		for _, r := range value {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-') {
				return fmt.Errorf("%w: invalid %s capability", ErrCapabilityMismatch, label)
			}
		}
		if index > 0 && values[index-1] >= value {
			return fmt.Errorf("%w: %s capabilities must be sorted and unique", ErrCapabilityMismatch, label)
		}
	}
	return nil
}

type OperationRef struct {
	ReplicaID string `json:"replica_id"`
	Sequence  int64  `json:"sequence"`
}

type SequenceRange struct {
	ReplicaID string `json:"replica_id"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
}

type MissingPlan struct {
	Ranges        []SequenceRange `json:"ranges"`
	SequenceCount int64           `json:"sequence_count"`
	MoreAvailable bool            `json:"more_available"`
	Dependencies  []OperationRef  `json:"dependencies,omitempty"`
}

// PlanMissing returns deterministic bounded ranges after removing operations
// already present in a receiver's durable pending queue.
func PlanMissing(local, remote Vector, present []OperationRef) (MissingPlan, error) {
	if err := ValidateVector(local); err != nil {
		return MissingPlan{}, err
	}
	if err := ValidateVector(remote); err != nil {
		return MissingPlan{}, err
	}
	presentByReplica := make(map[string][]int64)
	for _, ref := range present {
		if err := validateRef(ref); err != nil {
			return MissingPlan{}, err
		}
		presentByReplica[ref.ReplicaID] = append(presentByReplica[ref.ReplicaID], ref.Sequence)
	}
	for replicaID := range presentByReplica {
		sort.Slice(presentByReplica[replicaID], func(i, j int) bool { return presentByReplica[replicaID][i] < presentByReplica[replicaID][j] })
	}
	replicaIDs := make([]string, 0, len(remote))
	for replicaID := range remote {
		replicaIDs = append(replicaIDs, replicaID)
	}
	sort.Strings(replicaIDs)
	plan := MissingPlan{Ranges: []SequenceRange{}, Dependencies: []OperationRef{}}
	for _, replicaID := range replicaIDs {
		start := local[replicaID] + 1
		end := remote[replicaID]
		if start > end {
			continue
		}
		cursor := start
		for _, sequence := range presentByReplica[replicaID] {
			if sequence < cursor || sequence > end {
				continue
			}
			if sequence > cursor && !appendBoundedRange(&plan, replicaID, cursor, sequence-1) {
				return plan, nil
			}
			cursor = sequence + 1
		}
		if cursor <= end && !appendBoundedRange(&plan, replicaID, cursor, end) {
			return plan, nil
		}
	}
	return plan, nil
}

func appendBoundedRange(plan *MissingPlan, replicaID string, start, end int64) bool {
	if len(plan.Ranges) >= MaxMissingRanges || plan.SequenceCount >= MaxPlannedSequences {
		plan.MoreAvailable = true
		return false
	}
	remaining := MaxPlannedSequences - plan.SequenceCount
	length := end - start + 1
	if length > remaining {
		end = start + remaining - 1
		length = remaining
		plan.MoreAvailable = true
	}
	plan.Ranges = append(plan.Ranges, SequenceRange{ReplicaID: replicaID, Start: start, End: end})
	plan.SequenceCount += length
	return !plan.MoreAvailable
}

type Operation struct {
	ReplicaID    string          `json:"replica_id"`
	Sequence     int64           `json:"sequence"`
	OperationID  string          `json:"operation_id"`
	Kind         string          `json:"kind"`
	RecordType   string          `json:"record_type"`
	RecordID     string          `json:"record_id"`
	Payload      json.RawMessage `json:"payload"`
	HLC          HLC             `json:"hlc"`
	CreatedAt    string          `json:"created_at"`
	Dependencies []OperationRef  `json:"dependencies"`
}

// HLC is the durable hybrid logical timestamp used for field and membership
// ordering. Delivery completeness remains the responsibility of state vectors.
type HLC struct {
	WallMS  int64 `json:"wall_ms"`
	Logical int64 `json:"logical"`
}

const MaxHLCWallMS int64 = 253402300799999 // 9999-12-31T23:59:59.999Z
const MaxHLCLogical int64 = math.MaxInt32

// NormalizeOperation produces the one internal G5/G6 pending representation. G9
// still owns the production wire codec and strict cross-version canonical form.
func NormalizeOperation(operation Operation) (Operation, []byte, error) {
	if err := validateIdentifier("replica id", operation.ReplicaID); err != nil {
		return Operation{}, nil, err
	}
	if operation.Sequence <= 0 || operation.Sequence > MaxSequence {
		return Operation{}, nil, fmt.Errorf("%w: operation sequence is outside 1..%d", ErrInvalidState, MaxSequence)
	}
	wantID := fmt.Sprintf("%s:%020d", operation.ReplicaID, operation.Sequence)
	if operation.OperationID != wantID {
		return Operation{}, nil, fmt.Errorf("%w: operation id must be %q", ErrInvalidState, wantID)
	}
	if operation.HLC.WallMS <= 0 || operation.HLC.WallMS > MaxHLCWallMS || operation.HLC.Logical < 0 || operation.HLC.Logical > MaxHLCLogical {
		return Operation{}, nil, fmt.Errorf("%w: HLC is outside protocol bounds", ErrInvalidState)
	}
	if strings.TrimSpace(operation.Kind) == "" || len(operation.Kind) > 128 {
		return Operation{}, nil, fmt.Errorf("%w: invalid operation kind", ErrInvalidState)
	}
	if strings.TrimSpace(operation.RecordType) == "" || len(operation.RecordType) > 128 {
		return Operation{}, nil, fmt.Errorf("%w: invalid operation record type", ErrInvalidState)
	}
	if err := validateRecordID(operation.RecordID); err != nil {
		return Operation{}, nil, err
	}
	if len(operation.Payload) == 0 || len(operation.Payload) > MaxPayloadBytes || !json.Valid(operation.Payload) {
		return Operation{}, nil, fmt.Errorf("%w: payload must be valid JSON of at most %d bytes", ErrInvalidState, MaxPayloadBytes)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, operation.Payload); err != nil {
		return Operation{}, nil, fmt.Errorf("%w: compact payload: %v", ErrInvalidState, err)
	}
	operation.Payload = append(json.RawMessage(nil), compact.Bytes()...)
	createdAt, err := time.Parse(time.RFC3339Nano, operation.CreatedAt)
	if err != nil {
		return Operation{}, nil, fmt.Errorf("%w: created_at must be RFC3339", ErrInvalidState)
	}
	operation.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	if len(operation.Dependencies) > MaxDependencies {
		return Operation{}, nil, fmt.Errorf("%w: more than %d dependencies", ErrInvalidState, MaxDependencies)
	}
	operation.Dependencies = append([]OperationRef(nil), operation.Dependencies...)
	sort.Slice(operation.Dependencies, func(i, j int) bool {
		if operation.Dependencies[i].ReplicaID == operation.Dependencies[j].ReplicaID {
			return operation.Dependencies[i].Sequence < operation.Dependencies[j].Sequence
		}
		return operation.Dependencies[i].ReplicaID < operation.Dependencies[j].ReplicaID
	})
	for index, dependency := range operation.Dependencies {
		if err := validateRef(dependency); err != nil {
			return Operation{}, nil, err
		}
		if dependency.ReplicaID == operation.ReplicaID && dependency.Sequence >= operation.Sequence {
			return Operation{}, nil, fmt.Errorf("%w: same-replica dependency must precede the operation", ErrInvalidState)
		}
		if index > 0 && dependency == operation.Dependencies[index-1] {
			return Operation{}, nil, fmt.Errorf("%w: duplicate dependency", ErrInvalidState)
		}
	}
	encoded, err := json.Marshal(operation)
	if err != nil {
		return Operation{}, nil, err
	}
	if len(encoded) > MaxOperationBytes {
		return Operation{}, nil, fmt.Errorf("%w: encoded operation exceeds %d bytes", ErrInvalidState, MaxOperationBytes)
	}
	return operation, encoded, nil
}

func DecodeOperation(encoded []byte) (Operation, error) {
	if len(encoded) == 0 || len(encoded) > MaxOperationBytes {
		return Operation{}, fmt.Errorf("%w: encoded operation size is outside bounds", ErrInvalidState)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var operation Operation
	if err := decoder.Decode(&operation); err != nil {
		return Operation{}, fmt.Errorf("%w: decode pending operation: %v", ErrInvalidState, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Operation{}, fmt.Errorf("%w: trailing pending operation data", ErrInvalidState)
	}
	normalized, canonical, err := NormalizeOperation(operation)
	if err != nil {
		return Operation{}, err
	}
	if !bytes.Equal(canonical, encoded) {
		return Operation{}, fmt.Errorf("%w: pending operation is not canonical", ErrInvalidState)
	}
	return normalized, nil
}

func validateRef(ref OperationRef) error {
	if err := validateIdentifier("dependency replica id", ref.ReplicaID); err != nil {
		return err
	}
	if ref.Sequence <= 0 || ref.Sequence > MaxSequence {
		return fmt.Errorf("%w: dependency sequence is outside bounds", ErrInvalidState)
	}
	return nil
}

func validateIdentifier(label, value string) error {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return fmt.Errorf("%w: invalid %s", ErrInvalidState, label)
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return fmt.Errorf("%w: invalid %s", ErrInvalidState, label)
		}
	}
	return nil
}

func validateRecordID(value string) error {
	if value == "" || len(value) > 2_048 || !utf8.ValidString(value) {
		return fmt.Errorf("%w: invalid record id", ErrInvalidState)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: record id contains a control character", ErrInvalidState)
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}
