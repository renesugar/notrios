package bounds

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func fixtureOperations(count int) []Operation {
	result := make([]Operation, count)
	for index := range result {
		result[index] = Operation{
			ReplicaID: "00112233445566778899aabbccddeeff",
			Sequence:  uint64(index + 1), HLCWallMS: 1_786_000_000_000 + uint64(index),
			HLCLogical: uint32(index % 17), Kind: uint8(index % 8),
			ObjectID:     "ffeeddccbbaa99887766554433221100",
			Dependencies: []string{"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
			Payload:      []byte(fmt.Sprintf("{\"index\":%d,\"padding\":\"%s\"}", index, string(bytes.Repeat([]byte{'x'}, index%97)))),
		}
	}
	return result
}

func TestCodecsAreExactAndDeterministic(t *testing.T) {
	limits := ProposedLimits()
	operations := fixtureOperations(100)
	for name, encodeDecode := range map[string]func([]Operation) ([]Operation, []byte, []byte, error){
		"jsonl": func(input []Operation) ([]Operation, []byte, []byte, error) {
			first, err := EncodeJSONL(input, limits)
			if err != nil {
				return nil, nil, nil, err
			}
			second, err := EncodeJSONL(input, limits)
			if err != nil {
				return nil, nil, nil, err
			}
			decoded, err := DecodeJSONL(first, limits)
			return decoded, first, second, err
		},
		"compact": func(input []Operation) ([]Operation, []byte, []byte, error) {
			first, err := EncodeCompact(input, limits)
			if err != nil {
				return nil, nil, nil, err
			}
			second, err := EncodeCompact(input, limits)
			if err != nil {
				return nil, nil, nil, err
			}
			decoded, err := DecodeCompact(first, limits)
			return decoded, first, second, err
		},
	} {
		t.Run(name, func(t *testing.T) {
			decoded, first, second, err := encodeDecode(operations)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatal("encoding is nondeterministic")
			}
			if !reflect.DeepEqual(decoded, operations) {
				t.Fatal("round trip differs")
			}
		})
	}
}

func TestGzipIsExactDeterministicAndBounded(t *testing.T) {
	limits := ProposedLimits()
	value := bytes.Repeat([]byte("bounded deterministic envelope\n"), 100)
	first, err := CompressDeterministic(value)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CompressDeterministic(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("gzip encoding is nondeterministic")
	}
	decoded, err := DecompressBounded(first, limits)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, value) {
		t.Fatal("gzip round trip differs")
	}

	bomb := bytes.Repeat([]byte{0}, 2<<20)
	compressed, err := CompressDeterministic(bomb)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecompressBounded(compressed, limits); err == nil {
		t.Fatal("compression-ratio bomb was accepted")
	}
	if _, err := DecompressBounded(append(append([]byte(nil), first...), first...), limits); err == nil {
		t.Fatal("concatenated gzip members were accepted")
	}
}

func TestAdmissionLimitsRejectCountsAndSizes(t *testing.T) {
	limits := ProposedLimits()
	tooMany := fixtureOperations(limits.MaxOperations + 1)
	if _, err := EncodeJSONL(tooMany, limits); err == nil {
		t.Fatal("JSONL count limit not enforced")
	}
	if _, err := EncodeCompact(tooMany, limits); err == nil {
		t.Fatal("compact count limit not enforced")
	}

	oversized := fixtureOperations(1)
	oversized[0].Payload = make([]byte, limits.MaxPayloadBytes+1)
	if _, err := EncodeJSONL(oversized, limits); err == nil {
		t.Fatal("payload limit not enforced")
	}
	if _, err := EncodeCompact(oversized, limits); err == nil {
		t.Fatal("payload limit not enforced")
	}
}

func TestResourceBoundaryAndRange(t *testing.T) {
	policy := ProposedResourcePolicy()
	whole, err := EstimateResource(policy.WholeBelow-1, 64<<10, policy)
	if err != nil {
		t.Fatal(err)
	}
	if whole.Objects != 1 || whole.RangeTransferBytes != policy.WholeBelow-1 {
		t.Fatalf("unexpected whole estimate: %+v", whole)
	}
	chunked, err := EstimateResource(policy.WholeBelow, 64<<10, policy)
	if err != nil {
		t.Fatal(err)
	}
	if chunked.Objects != 1 || chunked.RangeTransferBytes != policy.ChunkBytes {
		t.Fatalf("unexpected chunk estimate: %+v", chunked)
	}
	large, err := EstimateResource(108<<20, 64<<10, policy)
	if err != nil {
		t.Fatal(err)
	}
	if large.Objects != 108 || large.MaximumRetryBytes != 1<<20 {
		t.Fatalf("unexpected large estimate: %+v", large)
	}
	if large.RangeTransferBytes != 1<<20 || large.WorstCaseRangeTransferBytes != 2<<20 {
		t.Fatalf("unexpected aligned/worst range estimate: %+v", large)
	}
}
