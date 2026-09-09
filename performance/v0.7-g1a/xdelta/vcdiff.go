package xdelta

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
)

var vcdiffMagic = []byte{0xD6, 0xC3, 0xC4, 0x00, 0x00}

const (
	vcdSource = 0x01
)

func appendVarint(destination []byte, value uint64) []byte {
	var encoded [10]byte
	position := len(encoded)
	encoded[position-1] = byte(value & 0x7f)
	position--
	for value >>= 7; value > 0; value >>= 7 {
		encoded[position-1] = byte(value&0x7f) | 0x80
		position--
	}
	return append(destination, encoded[position:]...)
}

type parser struct {
	data           []byte
	position       int
	maxVarintBytes int
}

func (p *parser) remaining() int {
	return len(p.data) - p.position
}

func (p *parser) readByte() (byte, error) {
	if p.position >= len(p.data) {
		return 0, io.ErrUnexpectedEOF
	}
	value := p.data[p.position]
	p.position++
	return value, nil
}

func (p *parser) readBytes(length int) ([]byte, error) {
	if length < 0 || length > p.remaining() {
		return nil, io.ErrUnexpectedEOF
	}
	value := p.data[p.position : p.position+length]
	p.position += length
	return value, nil
}

func (p *parser) readVarint() (uint64, error) {
	var value uint64
	for count := 0; count < p.maxVarintBytes; count++ {
		current, err := p.readByte()
		if err != nil {
			return 0, err
		}
		if value > math.MaxUint64>>7 {
			return 0, fmt.Errorf("%w: varint overflow", ErrInvalidDelta)
		}
		value = value<<7 | uint64(current&0x7f)
		if current&0x80 == 0 {
			return value, nil
		}
	}
	return 0, fmt.Errorf("%w: varint length", ErrLimitExceeded)
}

func intValue(value uint64, name string) (int, error) {
	if value > uint64(math.MaxInt) {
		return 0, fmt.Errorf("%w: %s integer overflow", ErrInvalidDelta, name)
	}
	return int(value), nil
}

func readBounded(reader io.Reader, maximum int, name string) ([]byte, error) {
	value, err := io.ReadAll(io.LimitReader(reader, int64(maximum)+1))
	if err != nil {
		return nil, err
	}
	if len(value) > maximum {
		return nil, fmt.Errorf("%w: %s bytes %d > %d", ErrLimitExceeded, name, len(value), maximum)
	}
	return value, nil
}

// EncodeVCDIFF encodes one target window using the RFC 3284 file/window
// grammar and default code table. The constrained encoder emits RUN (opcode
// 0), variable-size ADD (opcode 1), and SELF-mode COPY (opcode 19) only.
func EncodeVCDIFF(ctx context.Context, source, target []byte, requested Limits) ([]byte, error) {
	limits := normalizeLimits(requested)
	ops, err := Diff(ctx, source, target, limits)
	if err != nil {
		return nil, err
	}
	result := append([]byte(nil), vcdiffMagic...)
	if len(target) == 0 {
		return result, nil
	}

	dataSection := make([]byte, 0)
	instructionSection := make([]byte, 0, len(ops)*2)
	addressSection := make([]byte, 0)
	hasCopy := false
	for index, op := range ops {
		if err := checkContext(ctx, index); err != nil {
			return nil, err
		}
		switch op.Kind {
		case OpAdd:
			instructionSection = append(instructionSection, 1)
			instructionSection = appendVarint(instructionSection, uint64(op.Length))
			dataSection = append(dataSection, op.Data...)
		case OpRun:
			instructionSection = append(instructionSection, 0)
			instructionSection = appendVarint(instructionSection, uint64(op.Length))
			dataSection = append(dataSection, op.Byte)
		case OpCopy:
			hasCopy = true
			instructionSection = append(instructionSection, 19)
			instructionSection = appendVarint(instructionSection, uint64(op.Length))
			addressSection = appendVarint(addressSection, uint64(op.Offset))
		default:
			return nil, fmt.Errorf("%w: operation kind %d", ErrInvalidDelta, op.Kind)
		}
	}
	for name, length := range map[string]int{
		"data": len(dataSection), "instruction": len(instructionSection), "address": len(addressSection),
	} {
		if length > limits.MaxSectionBytes {
			return nil, fmt.Errorf("%w: %s section", ErrLimitExceeded, name)
		}
	}

	deltaBody := appendVarint(nil, uint64(len(target)))
	deltaBody = append(deltaBody, 0) // no secondary compression
	deltaBody = appendVarint(deltaBody, uint64(len(dataSection)))
	deltaBody = appendVarint(deltaBody, uint64(len(instructionSection)))
	deltaBody = appendVarint(deltaBody, uint64(len(addressSection)))
	deltaBody = append(deltaBody, dataSection...)
	deltaBody = append(deltaBody, instructionSection...)
	deltaBody = append(deltaBody, addressSection...)

	if hasCopy {
		result = append(result, vcdSource)
		result = appendVarint(result, uint64(len(source)))
		result = appendVarint(result, 0)
	} else {
		result = append(result, 0)
	}
	result = appendVarint(result, uint64(len(deltaBody)))
	result = append(result, deltaBody...)
	if len(result) > limits.MaxDeltaBytes {
		return nil, fmt.Errorf("%w: delta bytes %d > %d", ErrLimitExceeded, len(result), limits.MaxDeltaBytes)
	}
	return result, nil
}

// EncodeVCDIFFStream is the bounded io.Reader/io.Writer form used to evaluate
// a transport-neutral API. The prototype buffers each bounded window; it does
// not claim an unbounded streaming implementation.
func EncodeVCDIFFStream(
	ctx context.Context,
	sourceReader io.Reader,
	targetReader io.Reader,
	output io.Writer,
	requested Limits,
) error {
	limits := normalizeLimits(requested)
	source, err := readBounded(sourceReader, limits.MaxSourceBytes, "source")
	if err != nil {
		return err
	}
	target, err := readBounded(targetReader, limits.MaxTargetBytes, "target")
	if err != nil {
		return err
	}
	delta, err := EncodeVCDIFF(ctx, source, target, limits)
	if err != nil {
		return err
	}
	return writeAll(output, delta)
}

func decodeCopy(window *[]byte, source []byte, address, length, expectedLength int) error {
	if address < 0 || length < 0 || address >= len(source)+len(*window) {
		return ErrSourceOutOfRange
	}
	for count := 0; count < length; count++ {
		if len(*window) >= expectedLength {
			return ErrOutputMismatch
		}
		absolute := address + count
		if absolute < len(source) {
			*window = append(*window, source[absolute])
			continue
		}
		targetOffset := absolute - len(source)
		if targetOffset < 0 || targetOffset >= len(*window) {
			return ErrSourceOutOfRange
		}
		*window = append(*window, (*window)[targetOffset])
	}
	return nil
}

func decodeWindow(
	ctx context.Context,
	source []byte,
	body []byte,
	limits Limits,
	operationCount *int,
) ([]byte, error) {
	p := parser{data: body, maxVarintBytes: limits.MaxVarintBytes}
	targetLengthValue, err := p.readVarint()
	if err != nil {
		return nil, err
	}
	targetLength, err := intValue(targetLengthValue, "target window length")
	if err != nil {
		return nil, err
	}
	if targetLength > limits.MaxWindowBytes {
		return nil, fmt.Errorf("%w: target window", ErrLimitExceeded)
	}
	deltaIndicator, err := p.readByte()
	if err != nil {
		return nil, err
	}
	if deltaIndicator != 0 {
		return nil, fmt.Errorf("%w: compressed VCDIFF sections", ErrUnsupported)
	}
	sectionLengths := make([]int, 3)
	for index, name := range []string{"data", "instruction", "address"} {
		value, readErr := p.readVarint()
		if readErr != nil {
			return nil, readErr
		}
		length, convertErr := intValue(value, name+" section length")
		if convertErr != nil {
			return nil, convertErr
		}
		if length > limits.MaxSectionBytes {
			return nil, fmt.Errorf("%w: %s section", ErrLimitExceeded, name)
		}
		sectionLengths[index] = length
	}
	if sectionLengths[0] > p.remaining() || sectionLengths[1] > p.remaining()-sectionLengths[0] ||
		sectionLengths[2] != p.remaining()-sectionLengths[0]-sectionLengths[1] {
		return nil, fmt.Errorf("%w: section lengths", ErrInvalidDelta)
	}
	dataBytes, _ := p.readBytes(sectionLengths[0])
	instructionBytes, _ := p.readBytes(sectionLengths[1])
	addressBytes, _ := p.readBytes(sectionLengths[2])
	dataParser := parser{data: dataBytes, maxVarintBytes: limits.MaxVarintBytes}
	instructionParser := parser{data: instructionBytes, maxVarintBytes: limits.MaxVarintBytes}
	addressParser := parser{data: addressBytes, maxVarintBytes: limits.MaxVarintBytes}
	output := make([]byte, 0, targetLength)

	for instructionParser.remaining() > 0 {
		*operationCount++
		if *operationCount > limits.MaxOperations {
			return nil, fmt.Errorf("%w: operation count", ErrLimitExceeded)
		}
		if err := checkContext(ctx, *operationCount); err != nil {
			return nil, err
		}
		opcode, _ := instructionParser.readByte()
		var kind OpKind
		var size int
		switch {
		case opcode == 0:
			kind = OpRun
		case opcode == 1:
			kind = OpAdd
		case opcode >= 2 && opcode <= 18:
			kind = OpAdd
			size = int(opcode - 1)
		case opcode == 19:
			kind = OpCopy
		case opcode >= 20 && opcode <= 34:
			kind = OpCopy
			size = int(opcode - 16)
		default:
			return nil, fmt.Errorf("%w: default-code-table opcode %d", ErrUnsupported, opcode)
		}
		if size == 0 {
			value, readErr := instructionParser.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			size, err = intValue(value, "instruction size")
			if err != nil {
				return nil, err
			}
		}
		if size <= 0 || len(output) > targetLength-size {
			return nil, ErrOutputMismatch
		}
		switch kind {
		case OpAdd:
			value, readErr := dataParser.readBytes(size)
			if readErr != nil {
				return nil, readErr
			}
			output = append(output, value...)
		case OpRun:
			value, readErr := dataParser.readByte()
			if readErr != nil {
				return nil, readErr
			}
			for count := 0; count < size; count++ {
				output = append(output, value)
			}
		case OpCopy:
			addressValue, readErr := addressParser.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			address, convertErr := intValue(addressValue, "COPY address")
			if convertErr != nil {
				return nil, convertErr
			}
			if err := decodeCopy(&output, source, address, size, targetLength); err != nil {
				return nil, err
			}
		}
	}
	if dataParser.remaining() != 0 || addressParser.remaining() != 0 {
		return nil, fmt.Errorf("%w: unused data or addresses", ErrInvalidDelta)
	}
	if len(output) != targetLength {
		return nil, ErrOutputMismatch
	}
	return output, nil
}

// DecodeVCDIFF decodes the constrained RFC 3284 profile. It rejects custom
// code tables, secondary compression, VCD_TARGET windows, non-SELF COPY modes,
// and compound default-table opcodes before producing canonical bytes.
func DecodeVCDIFF(
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
	if len(delta) < len(vcdiffMagic) || !bytes.Equal(delta[:len(vcdiffMagic)], vcdiffMagic) {
		return nil, fmt.Errorf("%w: VCDIFF header or unsupported custom table/compressor", ErrInvalidDelta)
	}
	p := parser{data: delta[len(vcdiffMagic):], maxVarintBytes: limits.MaxVarintBytes}
	result := make([]byte, 0)
	operationCount := 0
	windowCount := 0
	for p.remaining() > 0 {
		windowCount++
		if windowCount > limits.MaxWindows {
			return nil, fmt.Errorf("%w: window count", ErrLimitExceeded)
		}
		indicator, err := p.readByte()
		if err != nil {
			return nil, err
		}
		if indicator&^byte(vcdSource) != 0 {
			return nil, fmt.Errorf("%w: window indicator 0x%02x", ErrUnsupported, indicator)
		}
		windowSource := []byte(nil)
		if indicator&vcdSource != 0 {
			sizeValue, readErr := p.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			positionValue, readErr := p.readVarint()
			if readErr != nil {
				return nil, readErr
			}
			size, convertErr := intValue(sizeValue, "source segment size")
			if convertErr != nil {
				return nil, convertErr
			}
			position, convertErr := intValue(positionValue, "source segment position")
			if convertErr != nil {
				return nil, convertErr
			}
			if position < 0 || size < 0 || position > len(source)-size {
				return nil, ErrSourceOutOfRange
			}
			windowSource = source[position : position+size]
		}
		deltaLengthValue, err := p.readVarint()
		if err != nil {
			return nil, err
		}
		deltaLength, err := intValue(deltaLengthValue, "delta encoding length")
		if err != nil {
			return nil, err
		}
		if deltaLength > limits.MaxDeltaBytes {
			return nil, fmt.Errorf("%w: delta encoding length", ErrLimitExceeded)
		}
		body, err := p.readBytes(deltaLength)
		if err != nil {
			return nil, err
		}
		window, err := decodeWindow(ctx, windowSource, body, limits, &operationCount)
		if err != nil {
			return nil, err
		}
		if len(result) > limits.MaxTargetBytes-len(window) {
			return nil, fmt.Errorf("%w: total output", ErrLimitExceeded)
		}
		result = append(result, window...)
	}
	denominator := len(source) + len(delta)
	if exceedsExpansion(len(result), denominator, limits.MaxExpansionRatio) {
		return nil, ErrExpansion
	}
	return result, nil
}

// DecodeVCDIFFStream is the bounded io.Reader/io.Writer decoder form.
func DecodeVCDIFFStream(
	ctx context.Context,
	sourceReader io.Reader,
	deltaReader io.Reader,
	output io.Writer,
	requested Limits,
	chainDepth int,
) error {
	limits := normalizeLimits(requested)
	source, err := readBounded(sourceReader, limits.MaxSourceBytes, "source")
	if err != nil {
		return err
	}
	delta, err := readBounded(deltaReader, limits.MaxDeltaBytes, "delta")
	if err != nil {
		return err
	}
	decoded, err := DecodeVCDIFF(ctx, source, delta, limits, chainDepth)
	if err != nil {
		return err
	}
	return writeAll(output, decoded)
}

func writeAll(output io.Writer, value []byte) error {
	for len(value) > 0 {
		written, err := output.Write(value)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(value) {
			return io.ErrShortWrite
		}
		value = value[written:]
	}
	return nil
}
