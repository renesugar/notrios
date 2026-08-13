// Package syncdelta is Notrios' pure-Go transfer-delta codec: a constrained
// RFC 3284 default-table VCDIFF encoder/decoder over a Subversion-style match
// finder. It is a transfer optimization only. A delta is never canonical state
// and never the sole recovery representation of a note body, so every caller
// must retain a path to the complete object and must verify the exact result
// hash before the reconstructed bytes reach canonical storage.
//
// The match finder follows the algorithm described by Apache Subversion's
// libsvn_delta/xdelta.c: index non-overlapping 64-byte source blocks with a
// rolling pseudo-Adler checksum, scan the target byte by byte, and extend
// confirmed matches in both directions. This implementation was written for
// Notrios from the published algorithm and RFC 3284; it is not a line-by-line
// translation of Subversion, and it contains no GPL-derived code. The full
// provenance record is `performance/v0.7-g1a/PROVENANCE.md`.
//
// This package is the v0.7 G7 reviewed promotion of the G1a investigation
// prototype under `performance/v0.7-g1a/xdelta`. The promotion drops the
// unselected private `NXD1` container and the buffering stream wrappers, and
// replaces the harness limits with production bounds.
package syncdelta

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

const (
	matchBlockSize = 64
	minRunLength   = 8
)

var (
	ErrCancelled        = errors.New("delta operation cancelled")
	ErrLimitExceeded    = errors.New("delta limit exceeded")
	ErrInvalidDelta     = errors.New("invalid delta")
	ErrUnsupported      = errors.New("unsupported VCDIFF feature")
	ErrChainTooDeep     = errors.New("delta chain depth exceeds limit")
	ErrExpansion        = errors.New("delta expansion ratio exceeds limit")
	ErrOutputMismatch   = errors.New("decoded output length mismatch")
	ErrSourceOutOfRange = errors.New("copy source range is invalid")
)

// Limits bounds every allocation and work unit performed by the codec. Zero
// fields are replaced by DefaultLimits values.
type Limits struct {
	MaxSourceBytes        int
	MaxTargetBytes        int
	MaxDeltaBytes         int
	MaxWindowBytes        int
	MaxSectionBytes       int
	MaxOperations         int
	MaxWindows            int
	MaxChecksumCandidates int
	MaxVarintBytes        int
	MaxExpansionRatio     int
	MaxChainDepth         int
}

// MaxObjectBytes is the protocol ceiling for one complete note body object. It
// sits far above the largest body G1 observed in any real corpus (326,359
// bytes) and well below G2's 16 MiB canonical envelope limit, so a single body
// can never fill an envelope on its own.
const MaxObjectBytes = 8 << 20

// DefaultLimits are the production bounds. Every one of them is checked before
// allocation-heavy work rather than after it.
func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes:  MaxObjectBytes,
		MaxTargetBytes:  MaxObjectBytes,
		MaxDeltaBytes:   MaxObjectBytes,
		MaxWindowBytes:  MaxObjectBytes,
		MaxSectionBytes: MaxObjectBytes,
		MaxOperations:   100_000,
		MaxWindows:      64,
		// A single 64-byte source block that repeats thousands of times must
		// not turn match finding into a quadratic scan of every occurrence.
		MaxChecksumCandidates: 64,
		MaxVarintBytes:        10,
		MaxExpansionRatio:     1_024,
		// A reconstructed body is stored complete, so a delta's base is always
		// a complete local object and a chain never accumulates. The parameter
		// stays because the decoder is also reachable from callers that
		// reconstruct a base first; see ReconstructBody.
		MaxChainDepth: 1,
	}
}

func normalizeLimits(in Limits) Limits {
	defaults := DefaultLimits()
	if in.MaxSourceBytes <= 0 {
		in.MaxSourceBytes = defaults.MaxSourceBytes
	}
	if in.MaxTargetBytes <= 0 {
		in.MaxTargetBytes = defaults.MaxTargetBytes
	}
	if in.MaxDeltaBytes <= 0 {
		in.MaxDeltaBytes = defaults.MaxDeltaBytes
	}
	if in.MaxWindowBytes <= 0 {
		in.MaxWindowBytes = defaults.MaxWindowBytes
	}
	if in.MaxSectionBytes <= 0 {
		in.MaxSectionBytes = defaults.MaxSectionBytes
	}
	if in.MaxOperations <= 0 {
		in.MaxOperations = defaults.MaxOperations
	}
	if in.MaxWindows <= 0 {
		in.MaxWindows = defaults.MaxWindows
	}
	if in.MaxChecksumCandidates <= 0 {
		in.MaxChecksumCandidates = defaults.MaxChecksumCandidates
	}
	if in.MaxVarintBytes <= 0 {
		in.MaxVarintBytes = defaults.MaxVarintBytes
	}
	if in.MaxExpansionRatio <= 0 {
		in.MaxExpansionRatio = defaults.MaxExpansionRatio
	}
	if in.MaxChainDepth <= 0 {
		in.MaxChainDepth = defaults.MaxChainDepth
	}
	return in
}

// OpKind is one binary delta instruction before serialization.
type OpKind byte

const (
	OpAdd OpKind = iota
	OpCopy
	OpRun
)

// Op is an ADD, source COPY, or repeated-byte RUN instruction.
type Op struct {
	Kind   OpKind
	Offset int
	Length int
	Data   []byte
	Byte   byte
}

type blockIndex map[uint32][]int

func initPseudoAdler(data []byte) uint32 {
	var s1, s2 uint32
	for _, value := range data {
		s1 += uint32(value)
		s2 += s1
	}
	return s2<<16 + s1
}

func rollPseudoAdler(sum uint32, outgoing, incoming byte) uint32 {
	sum -= matchBlockSize * 0x10000 * uint32(outgoing)
	sum -= uint32(outgoing)
	sum += uint32(incoming)
	return sum + sum*0x10000
}

func buildBlockIndex(ctx context.Context, source []byte, limits Limits) (blockIndex, error) {
	index := make(blockIndex, len(source)/matchBlockSize)
	for offset := 0; offset+matchBlockSize <= len(source); offset += matchBlockSize {
		if err := checkContext(ctx, offset/matchBlockSize); err != nil {
			return nil, err
		}
		block := source[offset : offset+matchBlockSize]
		sum := initPseudoAdler(block)
		duplicate := false
		for _, previous := range index[sum] {
			if bytes.Equal(source[previous:previous+matchBlockSize], block) {
				duplicate = true
				break
			}
		}
		if !duplicate && len(index[sum]) < limits.MaxChecksumCandidates {
			index[sum] = append(index[sum], offset)
		}
	}
	return index, nil
}

func matchingBlock(index blockIndex, source, target []byte, targetOffset int, sum uint32) (int, bool) {
	for _, sourceOffset := range index[sum] {
		if bytes.Equal(
			source[sourceOffset:sourceOffset+matchBlockSize],
			target[targetOffset:targetOffset+matchBlockSize],
		) {
			return sourceOffset, true
		}
	}
	return 0, false
}

func commonPrefix(left, right []byte) int {
	limit := min(len(left), len(right))
	for index := 0; index < limit; index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return limit
}

func commonSuffix(left, right []byte, limit int) int {
	limit = min(limit, min(len(left), len(right)))
	for length := 0; length < limit; length++ {
		if left[len(left)-1-length] != right[len(right)-1-length] {
			return length
		}
	}
	return limit
}

func appendAdd(ops []Op, data []byte) []Op {
	if len(data) == 0 {
		return ops
	}
	copyOfData := append([]byte(nil), data...)
	if len(ops) > 0 && ops[len(ops)-1].Kind == OpAdd {
		ops[len(ops)-1].Data = append(ops[len(ops)-1].Data, copyOfData...)
		ops[len(ops)-1].Length = len(ops[len(ops)-1].Data)
		return ops
	}
	return append(ops, Op{Kind: OpAdd, Length: len(copyOfData), Data: copyOfData})
}

func appendCopy(ops []Op, offset, length int) []Op {
	if length == 0 {
		return ops
	}
	if len(ops) > 0 {
		last := &ops[len(ops)-1]
		if last.Kind == OpCopy && last.Offset+last.Length == offset {
			last.Length += length
			return ops
		}
	}
	return append(ops, Op{Kind: OpCopy, Offset: offset, Length: length})
}

func splitRuns(ops []Op) []Op {
	result := make([]Op, 0, len(ops))
	for _, op := range ops {
		if op.Kind != OpAdd {
			result = append(result, op)
			continue
		}
		start := 0
		for index := 0; index < len(op.Data); {
			end := index + 1
			for end < len(op.Data) && op.Data[end] == op.Data[index] {
				end++
			}
			if end-index >= minRunLength {
				result = appendAdd(result, op.Data[start:index])
				result = append(result, Op{Kind: OpRun, Length: end - index, Byte: op.Data[index]})
				start = end
			}
			index = end
		}
		result = appendAdd(result, op.Data[start:])
	}
	return result
}

func checkContext(ctx context.Context, iteration int) error {
	if iteration&1023 != 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrCancelled, ctx.Err())
	default:
		return nil
	}
}

func exceedsExpansion(outputBytes, inputBytes, ratio int) bool {
	if outputBytes <= 0 || inputBytes <= 0 {
		return false
	}
	// Compare by division so a caller-supplied large ratio cannot overflow a
	// multiplication and accidentally turn the bound into a small threshold.
	return uint64(outputBytes)/uint64(inputBytes) > uint64(ratio) ||
		(uint64(outputBytes)/uint64(inputBytes) == uint64(ratio) &&
			uint64(outputBytes)%uint64(inputBytes) != 0)
}

// Diff produces deterministic source COPY, ADD, and RUN operations. It indexes
// only source bytes; VCDIFF target-copy semantics are decoded and tested but
// deliberately not emitted by this Subversion-style match finder.
func Diff(ctx context.Context, source, target []byte, requested Limits) ([]Op, error) {
	limits := normalizeLimits(requested)
	if len(source) > limits.MaxSourceBytes {
		return nil, fmt.Errorf("%w: source bytes %d > %d", ErrLimitExceeded, len(source), limits.MaxSourceBytes)
	}
	if len(target) > limits.MaxTargetBytes || len(target) > limits.MaxWindowBytes {
		return nil, fmt.Errorf("%w: target bytes %d", ErrLimitExceeded, len(target))
	}
	if err := checkContext(ctx, 0); err != nil {
		return nil, err
	}
	if len(target) == 0 {
		return nil, nil
	}

	var ops []Op
	position := commonPrefix(source, target)
	pending := 0
	if position > 4 || position == len(target) {
		ops = appendCopy(ops, 0, position)
		pending = position
	} else {
		position = 0
	}

	if len(target)-position >= matchBlockSize && len(source) >= matchBlockSize {
		index, err := buildBlockIndex(ctx, source, limits)
		if err != nil {
			return nil, err
		}
		upper := len(target) - matchBlockSize
		rolling := initPseudoAdler(target[position : position+matchBlockSize])
		for position <= upper {
			if err := checkContext(ctx, position); err != nil {
				return nil, err
			}
			sourceOffset, found := matchingBlock(index, source, target, position, rolling)
			if !found {
				if position < upper {
					rolling = rollPseudoAdler(rolling, target[position], target[position+matchBlockSize])
				}
				position++
				continue
			}

			matchLength := matchBlockSize
			for sourceOffset+matchLength < len(source) && position+matchLength < len(target) &&
				source[sourceOffset+matchLength] == target[position+matchLength] {
				matchLength++
				if err := checkContext(ctx, matchLength); err != nil {
					return nil, err
				}
			}
			for sourceOffset > 0 && position > pending && source[sourceOffset-1] == target[position-1] {
				sourceOffset--
				position--
				matchLength++
			}

			ops = appendAdd(ops, target[pending:position])
			ops = appendCopy(ops, sourceOffset, matchLength)
			position += matchLength
			pending = position
			if len(ops) > limits.MaxOperations {
				return nil, fmt.Errorf("%w: operation count", ErrLimitExceeded)
			}
			if position <= upper {
				rolling = initPseudoAdler(target[position : position+matchBlockSize])
			}
		}
	}

	suffix := commonSuffix(source, target, min(len(source), len(target)-pending))
	if suffix <= 4 {
		suffix = 0
	}
	addEnd := len(target) - suffix
	ops = appendAdd(ops, target[pending:addEnd])
	if suffix > 0 {
		ops = appendCopy(ops, len(source)-suffix, suffix)
	}
	ops = splitRuns(ops)
	if len(ops) > limits.MaxOperations {
		return nil, fmt.Errorf("%w: operation count %d > %d", ErrLimitExceeded, len(ops), limits.MaxOperations)
	}
	return ops, nil
}

// ApplyOps applies the matcher operation stream and verifies all ranges.
func ApplyOps(ctx context.Context, source []byte, ops []Op, expectedLength int, requested Limits) ([]byte, error) {
	limits := normalizeLimits(requested)
	if len(source) > limits.MaxSourceBytes || expectedLength < 0 || expectedLength > limits.MaxTargetBytes {
		return nil, fmt.Errorf("%w: source or target length", ErrLimitExceeded)
	}
	if len(ops) > limits.MaxOperations {
		return nil, fmt.Errorf("%w: operation count", ErrLimitExceeded)
	}
	output := make([]byte, 0, expectedLength)
	for index, op := range ops {
		if err := checkContext(ctx, index); err != nil {
			return nil, err
		}
		if op.Length < 0 || len(output) > expectedLength-op.Length {
			return nil, ErrOutputMismatch
		}
		switch op.Kind {
		case OpAdd:
			if op.Length != len(op.Data) {
				return nil, fmt.Errorf("%w: ADD length", ErrInvalidDelta)
			}
			output = append(output, op.Data...)
		case OpCopy:
			if op.Offset < 0 || op.Length < 0 || op.Offset > len(source)-op.Length {
				return nil, ErrSourceOutOfRange
			}
			output = append(output, source[op.Offset:op.Offset+op.Length]...)
		case OpRun:
			for count := 0; count < op.Length; count++ {
				output = append(output, op.Byte)
			}
		default:
			return nil, fmt.Errorf("%w: operation kind %d", ErrInvalidDelta, op.Kind)
		}
	}
	if len(output) != expectedLength {
		return nil, ErrOutputMismatch
	}
	return output, nil
}
