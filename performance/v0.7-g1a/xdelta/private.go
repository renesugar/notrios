package xdelta

import (
	"bytes"
	"context"
	"fmt"
)

var privateMagic = []byte{'N', 'X', 'D', '1'}

// EncodePrivate serializes the same matcher operations in the smallest
// deterministic Notrios-specific comparison container considered by G1a. It
// exists only to measure the portability/size tradeoff against VCDIFF.
func EncodePrivate(ctx context.Context, source, target []byte, requested Limits) ([]byte, error) {
	limits := normalizeLimits(requested)
	ops, err := Diff(ctx, source, target, limits)
	if err != nil {
		return nil, err
	}
	result := append([]byte(nil), privateMagic...)
	result = appendVarint(result, uint64(len(target)))
	result = appendVarint(result, uint64(len(ops)))
	for index, op := range ops {
		if err := checkContext(ctx, index); err != nil {
			return nil, err
		}
		result = append(result, byte(op.Kind))
		switch op.Kind {
		case OpAdd:
			result = appendVarint(result, uint64(op.Length))
			result = append(result, op.Data...)
		case OpCopy:
			result = appendVarint(result, uint64(op.Offset))
			result = appendVarint(result, uint64(op.Length))
		case OpRun:
			result = appendVarint(result, uint64(op.Length))
			result = append(result, op.Byte)
		default:
			return nil, fmt.Errorf("%w: operation kind %d", ErrInvalidDelta, op.Kind)
		}
	}
	if len(result) > limits.MaxDeltaBytes {
		return nil, fmt.Errorf("%w: private delta bytes", ErrLimitExceeded)
	}
	return result, nil
}

// DecodePrivate applies the comparison container under the same bounds as the
// VCDIFF decoder. The source/result hashes still belong to Notrios's future
// outer object contract; neither investigation codec embeds them.
func DecodePrivate(
	ctx context.Context,
	source, delta []byte,
	requested Limits,
	chainDepth int,
) ([]byte, error) {
	limits := normalizeLimits(requested)
	if chainDepth < 0 || chainDepth > limits.MaxChainDepth {
		return nil, ErrChainTooDeep
	}
	if len(source) > limits.MaxSourceBytes || len(delta) > limits.MaxDeltaBytes {
		return nil, fmt.Errorf("%w: source or delta bytes", ErrLimitExceeded)
	}
	if len(delta) < len(privateMagic) || !bytes.Equal(delta[:len(privateMagic)], privateMagic) {
		return nil, fmt.Errorf("%w: private header", ErrInvalidDelta)
	}
	p := parser{data: delta[len(privateMagic):], maxVarintBytes: limits.MaxVarintBytes}
	targetValue, err := p.readVarint()
	if err != nil {
		return nil, err
	}
	targetLength, err := intValue(targetValue, "target length")
	if err != nil {
		return nil, err
	}
	if targetLength > limits.MaxTargetBytes || targetLength > limits.MaxWindowBytes {
		return nil, fmt.Errorf("%w: target length", ErrLimitExceeded)
	}
	countValue, err := p.readVarint()
	if err != nil {
		return nil, err
	}
	count, err := intValue(countValue, "operation count")
	if err != nil {
		return nil, err
	}
	if count > limits.MaxOperations {
		return nil, fmt.Errorf("%w: operation count", ErrLimitExceeded)
	}
	ops := make([]Op, 0, count)
	for index := 0; index < count; index++ {
		if err := checkContext(ctx, index); err != nil {
			return nil, err
		}
		kind, readErr := p.readByte()
		if readErr != nil {
			return nil, readErr
		}
		switch OpKind(kind) {
		case OpAdd:
			lengthValue, readErr := p.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			length, convertErr := intValue(lengthValue, "ADD length")
			if convertErr != nil {
				return nil, convertErr
			}
			if length <= 0 {
				return nil, fmt.Errorf("%w: zero ADD length", ErrInvalidDelta)
			}
			if length > limits.MaxSectionBytes {
				return nil, fmt.Errorf("%w: ADD length", ErrLimitExceeded)
			}
			value, readErr := p.readBytes(length)
			if readErr != nil {
				return nil, readErr
			}
			ops = append(ops, Op{Kind: OpAdd, Length: length, Data: append([]byte(nil), value...)})
		case OpCopy:
			offsetValue, readErr := p.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			lengthValue, readErr := p.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			offset, convertErr := intValue(offsetValue, "COPY offset")
			if convertErr != nil {
				return nil, convertErr
			}
			length, convertErr := intValue(lengthValue, "COPY length")
			if convertErr != nil {
				return nil, convertErr
			}
			if length <= 0 {
				return nil, fmt.Errorf("%w: zero COPY length", ErrInvalidDelta)
			}
			ops = append(ops, Op{Kind: OpCopy, Offset: offset, Length: length})
		case OpRun:
			lengthValue, readErr := p.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			length, convertErr := intValue(lengthValue, "RUN length")
			if convertErr != nil {
				return nil, convertErr
			}
			if length <= 0 {
				return nil, fmt.Errorf("%w: zero RUN length", ErrInvalidDelta)
			}
			value, readErr := p.readByte()
			if readErr != nil {
				return nil, readErr
			}
			ops = append(ops, Op{Kind: OpRun, Length: length, Byte: value})
		default:
			return nil, fmt.Errorf("%w: private opcode %d", ErrInvalidDelta, kind)
		}
	}
	if p.remaining() != 0 {
		return nil, fmt.Errorf("%w: trailing private bytes", ErrInvalidDelta)
	}
	result, err := ApplyOps(ctx, source, ops, targetLength, limits)
	if err != nil {
		return nil, err
	}
	denominator := len(source) + len(delta)
	if exceedsExpansion(len(result), denominator, limits.MaxExpansionRatio) {
		return nil, ErrExpansion
	}
	return result, nil
}
