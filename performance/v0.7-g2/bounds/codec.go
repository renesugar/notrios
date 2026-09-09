// Package bounds contains investigation-only G2 codecs and admission probes.
// It is deliberately outside the production synchronization packages.
package bounds

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	CompactMagic = "NCB1"
	GzipLevel    = 6
)

// Limits model the proposed G9 admission boundary. They are evidence inputs,
// not a production protocol contract.
type Limits struct {
	MaxOperations      int
	MaxRecordBytes     int
	MaxPayloadBytes    int
	MaxDependencies    int
	MaxEncodedBytes    int64
	MaxCompressedBytes int64
	MaxExpansionRatio  int64
}

func ProposedLimits() Limits {
	return Limits{
		MaxOperations:      10_000,
		MaxRecordBytes:     1 << 20,
		MaxPayloadBytes:    512 << 10,
		MaxDependencies:    64,
		MaxEncodedBytes:    16 << 20,
		MaxCompressedBytes: 4 << 20,
		MaxExpansionRatio:  64,
	}
}

// Operation is a stable logical record shared by the JSONL and compact
// candidates. Hex identifiers keep JSON readable; the compact codec validates
// and stores their raw bytes.
type Operation struct {
	ReplicaID    string          `json:"replica_id"`
	Sequence     uint64          `json:"sequence"`
	HLCWallMS    uint64          `json:"hlc_wall_ms"`
	HLCLogical   uint32          `json:"hlc_logical"`
	Kind         uint8           `json:"kind"`
	ObjectID     string          `json:"object_id"`
	Dependencies []string        `json:"dependencies"`
	Payload      json.RawMessage `json:"payload"`
}

func validateOperation(op Operation, limits Limits) error {
	if len(op.Payload) > limits.MaxPayloadBytes {
		return fmt.Errorf("payload exceeds limit")
	}
	if !json.Valid(op.Payload) {
		return fmt.Errorf("payload must be one valid JSON value supplied by its canonical kind encoder")
	}
	if len(op.Dependencies) > limits.MaxDependencies {
		return fmt.Errorf("dependencies exceed limit")
	}
	if _, err := fixedHex(op.ReplicaID, 16); err != nil {
		return fmt.Errorf("replica id: %w", err)
	}
	if _, err := fixedHex(op.ObjectID, 16); err != nil {
		return fmt.Errorf("object id: %w", err)
	}
	for _, dependency := range op.Dependencies {
		if _, err := fixedHex(dependency, 32); err != nil {
			return fmt.Errorf("dependency: %w", err)
		}
	}
	return nil
}

func fixedHex(value string, size int) ([]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != size {
		return nil, fmt.Errorf("must be %d lowercase hex bytes", size)
	}
	if hex.EncodeToString(decoded) != value {
		return nil, fmt.Errorf("must use canonical lowercase hex")
	}
	return decoded, nil
}

// EncodeJSONL uses a struct-only schema, fixed field order, LF framing, no
// insignificant whitespace, and no HTML escaping. Those constraints are the
// canonicalization profile under investigation.
func EncodeJSONL(operations []Operation, limits Limits) ([]byte, error) {
	if len(operations) > limits.MaxOperations {
		return nil, fmt.Errorf("operation count exceeds limit")
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	for _, operation := range operations {
		if err := validateOperation(operation, limits); err != nil {
			return nil, err
		}
		before := output.Len()
		if err := encoder.Encode(operation); err != nil {
			return nil, err
		}
		if output.Len()-before > limits.MaxRecordBytes {
			return nil, fmt.Errorf("record exceeds limit")
		}
		if int64(output.Len()) > limits.MaxEncodedBytes {
			return nil, fmt.Errorf("encoded bytes exceed limit")
		}
	}
	return output.Bytes(), nil
}

func DecodeJSONL(encoded []byte, limits Limits) ([]Operation, error) {
	if int64(len(encoded)) > limits.MaxEncodedBytes {
		return nil, fmt.Errorf("encoded bytes exceed limit")
	}
	scanner := bufio.NewScanner(bytes.NewReader(encoded))
	scanner.Buffer(make([]byte, 64<<10), limits.MaxRecordBytes)
	operations := make([]Operation, 0, min(limits.MaxOperations, 1024))
	for scanner.Scan() {
		if len(operations) >= limits.MaxOperations {
			return nil, fmt.Errorf("operation count exceeds limit")
		}
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		var operation Operation
		if err := decoder.Decode(&operation); err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("record has trailing data")
		}
		if err := validateOperation(operation, limits); err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan records: %w", err)
	}
	return operations, nil
}

func appendUvarint(output []byte, value uint64) []byte {
	var scratch [binary.MaxVarintLen64]byte
	count := binary.PutUvarint(scratch[:], value)
	return append(output, scratch[:count]...)
}

// EncodeCompact emits the investigation-only NCB1 candidate: magic, record
// count, then fixed identifiers and minimal unsigned-varint lengths/integers.
// It has no optional fields, maps, floats, or alternate integer spellings.
func EncodeCompact(operations []Operation, limits Limits) ([]byte, error) {
	if len(operations) > limits.MaxOperations {
		return nil, fmt.Errorf("operation count exceeds limit")
	}
	output := append([]byte(nil), CompactMagic...)
	output = appendUvarint(output, uint64(len(operations)))
	for _, operation := range operations {
		if err := validateOperation(operation, limits); err != nil {
			return nil, err
		}
		replica, _ := fixedHex(operation.ReplicaID, 16)
		object, _ := fixedHex(operation.ObjectID, 16)
		output = append(output, replica...)
		output = appendUvarint(output, operation.Sequence)
		output = appendUvarint(output, operation.HLCWallMS)
		output = appendUvarint(output, uint64(operation.HLCLogical))
		output = append(output, operation.Kind)
		output = append(output, object...)
		output = appendUvarint(output, uint64(len(operation.Dependencies)))
		for _, value := range operation.Dependencies {
			dependency, _ := fixedHex(value, 32)
			output = append(output, dependency...)
		}
		output = appendUvarint(output, uint64(len(operation.Payload)))
		output = append(output, operation.Payload...)
		if int64(len(output)) > limits.MaxEncodedBytes {
			return nil, fmt.Errorf("encoded bytes exceed limit")
		}
	}
	return output, nil
}

type compactReader struct {
	value  []byte
	offset int
}

func (reader *compactReader) take(size int) ([]byte, error) {
	if size < 0 || size > len(reader.value)-reader.offset {
		return nil, io.ErrUnexpectedEOF
	}
	value := reader.value[reader.offset : reader.offset+size]
	reader.offset += size
	return value, nil
}

func (reader *compactReader) uvarint() (uint64, error) {
	value, count := binary.Uvarint(reader.value[reader.offset:])
	if count <= 0 {
		return 0, fmt.Errorf("invalid or truncated varint")
	}
	var scratch [binary.MaxVarintLen64]byte
	canonical := binary.PutUvarint(scratch[:], value)
	if canonical != count || !bytes.Equal(scratch[:canonical], reader.value[reader.offset:reader.offset+count]) {
		return 0, fmt.Errorf("non-minimal varint")
	}
	reader.offset += count
	return value, nil
}

func DecodeCompact(encoded []byte, limits Limits) ([]Operation, error) {
	if int64(len(encoded)) > limits.MaxEncodedBytes {
		return nil, fmt.Errorf("encoded bytes exceed limit")
	}
	reader := compactReader{value: encoded}
	magic, err := reader.take(len(CompactMagic))
	if err != nil || string(magic) != CompactMagic {
		return nil, fmt.Errorf("compact magic is absent")
	}
	count, err := reader.uvarint()
	if err != nil || count > uint64(limits.MaxOperations) {
		return nil, fmt.Errorf("operation count exceeds limit or is invalid")
	}
	operations := make([]Operation, 0, int(count))
	for index := uint64(0); index < count; index++ {
		replica, err := reader.take(16)
		if err != nil {
			return nil, err
		}
		sequence, err := reader.uvarint()
		if err != nil {
			return nil, err
		}
		wall, err := reader.uvarint()
		if err != nil {
			return nil, err
		}
		logical, err := reader.uvarint()
		if err != nil || logical > uint64(^uint32(0)) {
			return nil, fmt.Errorf("logical clock is invalid")
		}
		kind, err := reader.take(1)
		if err != nil {
			return nil, err
		}
		object, err := reader.take(16)
		if err != nil {
			return nil, err
		}
		dependencyCount, err := reader.uvarint()
		if err != nil || dependencyCount > uint64(limits.MaxDependencies) {
			return nil, fmt.Errorf("dependency count exceeds limit or is invalid")
		}
		dependencies := make([]string, 0, int(dependencyCount))
		for dependencyIndex := uint64(0); dependencyIndex < dependencyCount; dependencyIndex++ {
			dependency, err := reader.take(32)
			if err != nil {
				return nil, err
			}
			dependencies = append(dependencies, hex.EncodeToString(dependency))
		}
		payloadSize, err := reader.uvarint()
		if err != nil || payloadSize > uint64(limits.MaxPayloadBytes) {
			return nil, fmt.Errorf("payload exceeds limit or is invalid")
		}
		payload, err := reader.take(int(payloadSize))
		if err != nil {
			return nil, err
		}
		payloadCopy := make([]byte, len(payload))
		copy(payloadCopy, payload)
		operation := Operation{
			ReplicaID: hex.EncodeToString(replica), Sequence: sequence, HLCWallMS: wall,
			HLCLogical: uint32(logical), Kind: kind[0], ObjectID: hex.EncodeToString(object),
			Dependencies: dependencies, Payload: json.RawMessage(payloadCopy),
		}
		if err := validateOperation(operation, limits); err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	if reader.offset != len(encoded) {
		return nil, fmt.Errorf("compact stream has trailing bytes")
	}
	return operations, nil
}

// CompressDeterministic pins the gzip header and DEFLATE level. G9 would also
// need cross-toolchain golden bytes before treating the representation as a
// signed canonical artifact.
func CompressDeterministic(value []byte) ([]byte, error) {
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

func DecompressBounded(value []byte, limits Limits) ([]byte, error) {
	if int64(len(value)) > limits.MaxCompressedBytes {
		return nil, fmt.Errorf("compressed bytes exceed limit")
	}
	compressed := bytes.NewReader(value)
	reader, err := gzip.NewReader(compressed)
	if err != nil {
		return nil, err
	}
	reader.Multistream(false)
	defer reader.Close()
	maximum := limits.MaxEncodedBytes
	if len(value) > 0 && int64(len(value))*limits.MaxExpansionRatio < maximum {
		maximum = int64(len(value)) * limits.MaxExpansionRatio
	}
	bounded := io.LimitReader(reader, maximum+1)
	decoded, err := io.ReadAll(bounded)
	if err != nil {
		return nil, err
	}
	if int64(len(decoded)) > maximum {
		return nil, errors.New("expanded bytes or compression ratio exceeds limit")
	}
	if compressed.Len() != 0 {
		return nil, errors.New("gzip stream must contain exactly one member")
	}
	return decoded, nil
}
