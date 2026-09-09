// Package syncwire is the canonical protocol encoding: the exact bytes a
// Notrios replica produces for a logical envelope, and the encrypted, signed
// artifact those bytes travel inside. Every transport carries the same
// artifacts, so nothing here knows about folders, HTTP, or SQLite.
//
// Two properties matter more than efficiency. The encoding is **canonical**:
// one logical envelope has exactly one byte representation, so two replicas
// that agree on the content agree on its hash and its signature. And it is
// **bounded**: every length is checked before the allocation it would cause,
// because the sender of an artifact is not yet a party this replica trusts.
package syncwire

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
)

// Magic strings. They exist so a decoder rejects the wrong kind of bytes
// immediately rather than misreading them as a length.
const (
	OperationsMagic = "NCB1"
	EnvelopeMagic   = "NEV1"
	ArtifactMagic   = "NAR1"
)

// GzipLevel is pinned. Compression level is part of the canonical bytes:
// changing it changes every artifact hash and every signature.
const GzipLevel = 6

// Limits are G2's measured admission bounds, promoted unchanged. They bound
// what a receiver will allocate for an artifact it has not yet authenticated.
type Limits struct {
	MaxOperations      int
	MaxIdentifierBytes int
	MaxRecordIDBytes   int
	MaxPayloadBytes    int
	MaxDependencies    int
	MaxVectorEntries   int
	MaxEncodedBytes    int64
	MaxCompressedBytes int64
	MaxExpansionRatio  int64
}

// DefaultLimits returns the production bounds. The operation, payload,
// dependency, and vector figures are the ones G5 already enforces, so an
// artifact that decodes here cannot fail admission for being too big.
func DefaultLimits() Limits {
	return Limits{
		MaxOperations:      int(syncstate.MaxPlannedSequences),
		MaxIdentifierBytes: 128,
		MaxRecordIDBytes:   2_048,
		MaxPayloadBytes:    syncstate.MaxPayloadBytes,
		MaxDependencies:    syncstate.MaxDependencies,
		MaxVectorEntries:   syncstate.MaxStateVectorEntries,
		MaxEncodedBytes:    16 << 20,
		MaxCompressedBytes: 4 << 20,
		MaxExpansionRatio:  64,
	}
}

var (
	// ErrMalformed reports bytes that are not a canonical artifact.
	ErrMalformed = errors.New("malformed canonical bytes")
	// ErrNotCanonical reports bytes that decode but are not what an encoder
	// would have produced — a non-minimal varint, or trailing data. Accepting
	// them would mean two byte strings for one logical envelope, and therefore
	// two hashes and two signatures for one thing.
	ErrNotCanonical = errors.New("bytes are not the canonical encoding")
	// ErrLimitExceeded reports a bound reached before the allocation it guards.
	ErrLimitExceeded = errors.New("canonical bytes exceed a protocol limit")
)

func normalizeLimits(requested Limits) Limits {
	defaults := DefaultLimits()
	if requested.MaxOperations <= 0 {
		requested.MaxOperations = defaults.MaxOperations
	}
	if requested.MaxIdentifierBytes <= 0 {
		requested.MaxIdentifierBytes = defaults.MaxIdentifierBytes
	}
	if requested.MaxRecordIDBytes <= 0 {
		requested.MaxRecordIDBytes = defaults.MaxRecordIDBytes
	}
	if requested.MaxPayloadBytes <= 0 {
		requested.MaxPayloadBytes = defaults.MaxPayloadBytes
	}
	if requested.MaxDependencies <= 0 {
		requested.MaxDependencies = defaults.MaxDependencies
	}
	if requested.MaxVectorEntries <= 0 {
		requested.MaxVectorEntries = defaults.MaxVectorEntries
	}
	if requested.MaxEncodedBytes <= 0 {
		requested.MaxEncodedBytes = defaults.MaxEncodedBytes
	}
	if requested.MaxCompressedBytes <= 0 {
		requested.MaxCompressedBytes = defaults.MaxCompressedBytes
	}
	if requested.MaxExpansionRatio <= 0 {
		requested.MaxExpansionRatio = defaults.MaxExpansionRatio
	}
	return requested
}

// appendUvarint writes the minimal encoding of a value. Minimality is what
// makes the format canonical: a decoder re-encodes and refuses anything longer.
func appendUvarint(output []byte, value uint64) []byte {
	var scratch [binary.MaxVarintLen64]byte
	count := binary.PutUvarint(scratch[:], value)
	return append(output, scratch[:count]...)
}

func appendString(output []byte, value string) []byte {
	output = appendUvarint(output, uint64(len(value)))
	return append(output, value...)
}

type reader struct {
	value  []byte
	offset int
}

func (r *reader) remaining() int { return len(r.value) - r.offset }

func (r *reader) take(size int) ([]byte, error) {
	if size < 0 || size > r.remaining() {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, io.ErrUnexpectedEOF)
	}
	value := r.value[r.offset : r.offset+size]
	r.offset += size
	return value, nil
}

func (r *reader) uvarint() (uint64, error) {
	value, count := binary.Uvarint(r.value[r.offset:])
	if count <= 0 {
		return 0, fmt.Errorf("%w: invalid or truncated varint", ErrMalformed)
	}
	var scratch [binary.MaxVarintLen64]byte
	minimal := binary.PutUvarint(scratch[:], value)
	if minimal != count || !bytes.Equal(scratch[:minimal], r.value[r.offset:r.offset+count]) {
		return 0, fmt.Errorf("%w: non-minimal varint", ErrNotCanonical)
	}
	r.offset += count
	return value, nil
}

func (r *reader) boundedString(maximum int, what string) (string, error) {
	length, err := r.uvarint()
	if err != nil {
		return "", err
	}
	if length > uint64(maximum) {
		return "", fmt.Errorf("%w: %s is %d bytes, limit %d", ErrLimitExceeded, what, length, maximum)
	}
	value, err := r.take(int(length))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// EncodeOperations produces the canonical NCB1 record block.
//
// This is G2's compact candidate promoted, with one deliberate change. The
// investigation encoded replica and object identifiers as fixed sixteen-byte
// hex values, which its generated corpus satisfied; production identifiers are
// bounded strings — a replica id is up to 128 characters and a record id up to
// 2,048 — so every identifier is length-prefixed instead. The saving G2
// measured came from dropping JSON's field names, quoting, and escaping, and
// that saving is unaffected.
func EncodeOperations(operations []syncstate.Operation, requested Limits) ([]byte, error) {
	limits := normalizeLimits(requested)
	if len(operations) > limits.MaxOperations {
		return nil, fmt.Errorf("%w: %d operations, limit %d", ErrLimitExceeded, len(operations), limits.MaxOperations)
	}
	output := append([]byte(nil), OperationsMagic...)
	output = appendUvarint(output, uint64(len(operations)))
	for _, operation := range operations {
		normalized, _, err := syncstate.NormalizeOperation(operation)
		if err != nil {
			return nil, err
		}
		output = appendString(output, normalized.ReplicaID)
		output = appendUvarint(output, uint64(normalized.Sequence))
		output = appendUvarint(output, uint64(normalized.HLC.WallMS))
		output = appendUvarint(output, uint64(normalized.HLC.Logical))
		output = appendString(output, normalized.Kind)
		output = appendString(output, normalized.RecordType)
		output = appendString(output, normalized.RecordID)
		output = appendString(output, normalized.CreatedAt)
		output = appendUvarint(output, uint64(len(normalized.Dependencies)))
		for _, dependency := range normalized.Dependencies {
			output = appendString(output, dependency.ReplicaID)
			output = appendUvarint(output, uint64(dependency.Sequence))
		}
		output = appendUvarint(output, uint64(len(normalized.Payload)))
		output = append(output, normalized.Payload...)
		if int64(len(output)) > limits.MaxEncodedBytes {
			return nil, fmt.Errorf("%w: encoded operations exceed %d bytes", ErrLimitExceeded, limits.MaxEncodedBytes)
		}
	}
	return output, nil
}

// DecodeOperations reverses EncodeOperations and re-normalizes every record, so
// bytes that decode are bytes G5 will accept. Trailing data is refused: one
// logical block has one encoding, and anything after it is either a bug or an
// attempt to smuggle content past a hash.
func DecodeOperations(encoded []byte, requested Limits) ([]syncstate.Operation, error) {
	limits := normalizeLimits(requested)
	if int64(len(encoded)) > limits.MaxEncodedBytes {
		return nil, fmt.Errorf("%w: %d encoded bytes", ErrLimitExceeded, len(encoded))
	}
	r := &reader{value: encoded}
	magic, err := r.take(len(OperationsMagic))
	if err != nil {
		return nil, err
	}
	if string(magic) != OperationsMagic {
		return nil, fmt.Errorf("%w: not an %s block", ErrMalformed, OperationsMagic)
	}
	count, err := r.uvarint()
	if err != nil {
		return nil, err
	}
	if count > uint64(limits.MaxOperations) {
		return nil, fmt.Errorf("%w: %d operations, limit %d", ErrLimitExceeded, count, limits.MaxOperations)
	}
	operations := make([]syncstate.Operation, 0, min(int(count), 1024))
	for index := uint64(0); index < count; index++ {
		operation, err := decodeOperation(r, limits)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	if r.remaining() != 0 {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrNotCanonical, r.remaining())
	}
	return operations, nil
}

func decodeOperation(r *reader, limits Limits) (syncstate.Operation, error) {
	var operation syncstate.Operation
	var err error
	if operation.ReplicaID, err = r.boundedString(limits.MaxIdentifierBytes, "replica id"); err != nil {
		return syncstate.Operation{}, err
	}
	sequence, err := r.uvarint()
	if err != nil {
		return syncstate.Operation{}, err
	}
	operation.Sequence = int64(sequence)
	wall, err := r.uvarint()
	if err != nil {
		return syncstate.Operation{}, err
	}
	logical, err := r.uvarint()
	if err != nil {
		return syncstate.Operation{}, err
	}
	operation.HLC = syncstate.HLC{WallMS: int64(wall), Logical: int64(logical)}
	if operation.Kind, err = r.boundedString(limits.MaxIdentifierBytes, "kind"); err != nil {
		return syncstate.Operation{}, err
	}
	if operation.RecordType, err = r.boundedString(limits.MaxIdentifierBytes, "record type"); err != nil {
		return syncstate.Operation{}, err
	}
	if operation.RecordID, err = r.boundedString(limits.MaxRecordIDBytes, "record id"); err != nil {
		return syncstate.Operation{}, err
	}
	if operation.CreatedAt, err = r.boundedString(limits.MaxIdentifierBytes, "created at"); err != nil {
		return syncstate.Operation{}, err
	}
	dependencyCount, err := r.uvarint()
	if err != nil {
		return syncstate.Operation{}, err
	}
	if dependencyCount > uint64(limits.MaxDependencies) {
		return syncstate.Operation{}, fmt.Errorf("%w: %d dependencies", ErrLimitExceeded, dependencyCount)
	}
	operation.Dependencies = make([]syncstate.OperationRef, 0, dependencyCount)
	for index := uint64(0); index < dependencyCount; index++ {
		replicaID, err := r.boundedString(limits.MaxIdentifierBytes, "dependency replica id")
		if err != nil {
			return syncstate.Operation{}, err
		}
		sequence, err := r.uvarint()
		if err != nil {
			return syncstate.Operation{}, err
		}
		operation.Dependencies = append(operation.Dependencies, syncstate.OperationRef{
			ReplicaID: replicaID, Sequence: int64(sequence),
		})
	}
	payloadLength, err := r.uvarint()
	if err != nil {
		return syncstate.Operation{}, err
	}
	if payloadLength > uint64(limits.MaxPayloadBytes) {
		return syncstate.Operation{}, fmt.Errorf("%w: payload is %d bytes", ErrLimitExceeded, payloadLength)
	}
	payload, err := r.take(int(payloadLength))
	if err != nil {
		return syncstate.Operation{}, err
	}
	operation.Payload = append([]byte(nil), payload...)
	operation.OperationID = fmt.Sprintf("%s:%020d", operation.ReplicaID, operation.Sequence)
	normalized, _, err := syncstate.NormalizeOperation(operation)
	if err != nil {
		return syncstate.Operation{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return normalized, nil
}

// Envelope is one bounded exchange: who sent it, for which database, what they
// know, and the operations they are handing over. Everything in it is
// plaintext that gets encrypted — a state vector says which replicas exist and
// how much each has written, which is exactly what G0 said must not be visible
// to a carrier.
type Envelope struct {
	ProtocolMajor   int
	ProtocolMinor   int
	DatabaseID      string
	SenderReplicaID string
	StateVector     syncstate.Vector
	Operations      []syncstate.Operation
}

// EncodeEnvelope produces the canonical plaintext of an envelope. State-vector
// entries are emitted in sorted replica order, because a Go map has no order
// and two replicas encoding the same vector must produce the same bytes.
func EncodeEnvelope(envelope Envelope, requested Limits) ([]byte, error) {
	limits := normalizeLimits(requested)
	if err := syncstate.ValidateVector(envelope.StateVector); err != nil {
		return nil, err
	}
	if len(envelope.StateVector) > limits.MaxVectorEntries {
		return nil, fmt.Errorf("%w: %d vector entries", ErrLimitExceeded, len(envelope.StateVector))
	}
	if envelope.DatabaseID == "" || envelope.SenderReplicaID == "" {
		return nil, fmt.Errorf("%w: an envelope names its database and its sender", ErrMalformed)
	}
	operations, err := EncodeOperations(envelope.Operations, limits)
	if err != nil {
		return nil, err
	}
	output := append([]byte(nil), EnvelopeMagic...)
	output = appendUvarint(output, uint64(envelope.ProtocolMajor))
	output = appendUvarint(output, uint64(envelope.ProtocolMinor))
	output = appendString(output, envelope.DatabaseID)
	output = appendString(output, envelope.SenderReplicaID)
	output = appendUvarint(output, uint64(len(envelope.StateVector)))
	for _, replicaID := range sortedVectorKeys(envelope.StateVector) {
		output = appendString(output, replicaID)
		output = appendUvarint(output, uint64(envelope.StateVector[replicaID]))
	}
	output = appendUvarint(output, uint64(len(operations)))
	output = append(output, operations...)
	if int64(len(output)) > limits.MaxEncodedBytes {
		return nil, fmt.Errorf("%w: envelope is %d bytes", ErrLimitExceeded, len(output))
	}
	return output, nil
}

// DecodeEnvelope reverses EncodeEnvelope.
func DecodeEnvelope(encoded []byte, requested Limits) (Envelope, error) {
	limits := normalizeLimits(requested)
	if int64(len(encoded)) > limits.MaxEncodedBytes {
		return Envelope{}, fmt.Errorf("%w: %d encoded bytes", ErrLimitExceeded, len(encoded))
	}
	r := &reader{value: encoded}
	magic, err := r.take(len(EnvelopeMagic))
	if err != nil {
		return Envelope{}, err
	}
	if string(magic) != EnvelopeMagic {
		return Envelope{}, fmt.Errorf("%w: not an %s envelope", ErrMalformed, EnvelopeMagic)
	}
	major, err := r.uvarint()
	if err != nil {
		return Envelope{}, err
	}
	minor, err := r.uvarint()
	if err != nil {
		return Envelope{}, err
	}
	envelope := Envelope{ProtocolMajor: int(major), ProtocolMinor: int(minor), StateVector: syncstate.Vector{}}
	if envelope.DatabaseID, err = r.boundedString(limits.MaxIdentifierBytes, "database id"); err != nil {
		return Envelope{}, err
	}
	if envelope.SenderReplicaID, err = r.boundedString(limits.MaxIdentifierBytes, "sender replica id"); err != nil {
		return Envelope{}, err
	}
	entries, err := r.uvarint()
	if err != nil {
		return Envelope{}, err
	}
	if entries > uint64(limits.MaxVectorEntries) {
		return Envelope{}, fmt.Errorf("%w: %d vector entries", ErrLimitExceeded, entries)
	}
	previous := ""
	for index := uint64(0); index < entries; index++ {
		replicaID, err := r.boundedString(limits.MaxIdentifierBytes, "vector replica id")
		if err != nil {
			return Envelope{}, err
		}
		if index > 0 && replicaID <= previous {
			return Envelope{}, fmt.Errorf("%w: vector entries must be sorted and unique", ErrNotCanonical)
		}
		previous = replicaID
		sequence, err := r.uvarint()
		if err != nil {
			return Envelope{}, err
		}
		envelope.StateVector[replicaID] = int64(sequence)
	}
	blockLength, err := r.uvarint()
	if err != nil {
		return Envelope{}, err
	}
	if blockLength > uint64(limits.MaxEncodedBytes) {
		return Envelope{}, fmt.Errorf("%w: operation block is %d bytes", ErrLimitExceeded, blockLength)
	}
	block, err := r.take(int(blockLength))
	if err != nil {
		return Envelope{}, err
	}
	if r.remaining() != 0 {
		return Envelope{}, fmt.Errorf("%w: %d trailing bytes", ErrNotCanonical, r.remaining())
	}
	if envelope.Operations, err = DecodeOperations(block, limits); err != nil {
		return Envelope{}, err
	}
	if err := syncstate.ValidateVector(envelope.StateVector); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func sortedVectorKeys(vector syncstate.Vector) []string {
	keys := make([]string, 0, len(vector))
	for replicaID := range vector {
		keys = append(keys, replicaID)
	}
	// A simple insertion sort keeps this file free of a sort import for a list
	// bounded at 1,024 entries; it is also stable and obviously total.
	for index := 1; index < len(keys); index++ {
		for position := index; position > 0 && keys[position] < keys[position-1]; position-- {
			keys[position], keys[position-1] = keys[position-1], keys[position]
		}
	}
	return keys
}

// Compress produces deterministic gzip. The modification time and OS byte are
// pinned because gzip records both by default, and an artifact whose bytes
// depended on when or where it was produced could not be content-addressed.
func Compress(value []byte) ([]byte, error) {
	var output bytes.Buffer
	writer, err := gzip.NewWriterLevel(&output, GzipLevel)
	if err != nil {
		return nil, err
	}
	writer.Header.ModTime = time.Unix(0, 0).UTC()
	writer.Header.OS = 255
	if _, err := writer.Write(value); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// Decompress expands within two bounds at once: an absolute ceiling and a ratio
// against the compressed size. A few kilobytes that expand to gigabytes is the
// oldest trick there is, and the ratio is what catches it before the ceiling
// would.
func Decompress(value []byte, requested Limits) ([]byte, error) {
	limits := normalizeLimits(requested)
	if int64(len(value)) > limits.MaxCompressedBytes {
		return nil, fmt.Errorf("%w: %d compressed bytes", ErrLimitExceeded, len(value))
	}
	compressed := bytes.NewReader(value)
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	gzipReader.Multistream(false)
	defer gzipReader.Close()
	maximum := limits.MaxEncodedBytes
	if len(value) > 0 && int64(len(value))*limits.MaxExpansionRatio < maximum {
		maximum = int64(len(value)) * limits.MaxExpansionRatio
	}
	decoded, err := io.ReadAll(io.LimitReader(gzipReader, maximum+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if int64(len(decoded)) > maximum {
		return nil, fmt.Errorf("%w: expansion ratio or size ceiling", ErrLimitExceeded)
	}
	if compressed.Len() != 0 {
		return nil, fmt.Errorf("%w: a gzip stream must contain exactly one member", ErrNotCanonical)
	}
	return decoded, nil
}
