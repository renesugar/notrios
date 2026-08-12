package xdelta

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"math/rand"
	"testing"
)

func fixtureBytes(seed int64, size int) []byte {
	random := rand.New(rand.NewSource(seed))
	value := make([]byte, size)
	_, _ = random.Read(value)
	return value
}

func mutateFixture(seed int64, source []byte) []byte {
	random := rand.New(rand.NewSource(seed))
	target := append([]byte(nil), source...)
	for count := 0; count < 12 && len(target) > 0; count++ {
		position := random.Intn(len(target))
		target[position] ^= byte(1 + random.Intn(255))
	}
	insert := fixtureBytes(seed+99, 37)
	position := len(target) / 3
	target = append(target[:position], append(insert, target[position:]...)...)
	if len(target) > 100 {
		target = append(target[:70], target[83:]...)
	}
	return target
}

func TestRoundTripArbitraryBytes(t *testing.T) {
	t.Parallel()
	repetitive := bytes.Repeat([]byte{0xA5}, 64<<10)
	random := fixtureBytes(7, 128<<10)
	cases := []struct {
		name   string
		source []byte
		target []byte
	}{
		{name: "empty"},
		{name: "identical-text", source: []byte("# café 東京 😀\n\nalpha beta gamma\n"), target: []byte("# café 東京 😀\n\nalpha beta gamma\n")},
		{name: "insert-delete", source: []byte("alpha beta gamma delta"), target: []byte("alpha revised gamma epsilon")},
		{name: "nul-bearing", source: []byte{0, 1, 2, 3, 0xff, 0}, target: []byte{0, 1, 9, 3, 0xff, 0, 0}},
		{name: "repetitive", source: repetitive, target: append(append([]byte(nil), repetitive[:32768]...), bytes.Repeat([]byte{0x5A}, 4096)...)},
		{name: "random-sparse", source: random, target: mutateFixture(11, random)},
		{name: "unrelated", source: fixtureBytes(1, 4096), target: fixtureBytes(2, 4096)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			vcdiff, err := EncodeVCDIFF(context.Background(), test.source, test.target, Limits{})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeVCDIFF(context.Background(), test.source, vcdiff, Limits{}, 1)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decoded, test.target) {
				t.Fatal("VCDIFF reconstruction mismatch")
			}

			private, err := EncodePrivate(context.Background(), test.source, test.target, Limits{})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err = DecodePrivate(context.Background(), test.source, private, Limits{}, 1)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decoded, test.target) {
				t.Fatal("private reconstruction mismatch")
			}
		})
	}
}

func TestEncodingIsDeterministic(t *testing.T) {
	t.Parallel()
	source := fixtureBytes(17, 256<<10)
	target := mutateFixture(18, source)
	firstVCDIFF, err := EncodeVCDIFF(context.Background(), source, target, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	firstPrivate, err := EncodePrivate(context.Background(), source, target, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 5; run++ {
		vcdiff, err := EncodeVCDIFF(context.Background(), source, target, Limits{})
		if err != nil || !bytes.Equal(vcdiff, firstVCDIFF) {
			t.Fatalf("VCDIFF run %d is not deterministic: %v", run, err)
		}
		private, err := EncodePrivate(context.Background(), source, target, Limits{})
		if err != nil || !bytes.Equal(private, firstPrivate) {
			t.Fatalf("private run %d is not deterministic: %v", run, err)
		}
	}
}

func TestVCDIFFGoldenDefaultCodeTable(t *testing.T) {
	t.Parallel()
	source := []byte("abcdefghij")
	target := []byte("abcdefghijX")
	delta, err := EncodeVCDIFF(context.Background(), source, target, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	const expectedHex = "d6c3c40000010a000b0b0001040158130a010100"
	if actual := hex.EncodeToString(delta); actual != expectedHex {
		t.Fatalf("VCDIFF golden drift\n got: %s\nwant: %s", actual, expectedHex)
	}
}

func TestVCDIFFTargetCopyOverlap(t *testing.T) {
	t.Parallel()
	// RFC 3284 default-table ADD(size=3), then SELF COPY(size=6,address=0).
	// With no source segment, address zero names the decoded target and the
	// overlapping copy expands "abc" into "abcabcabc".
	delta, err := hex.DecodeString("d6c3c40000000b0900030201616263041600")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeVCDIFF(context.Background(), nil, delta, Limits{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "abcabcabc" {
		t.Fatalf("got %q", decoded)
	}
}

func TestMatcherUsesSourceCopyAndRun(t *testing.T) {
	t.Parallel()
	source := []byte("prefix-" + string(bytes.Repeat([]byte("0123456789"), 100)) + "-suffix")
	target := []byte("prefix-" + string(bytes.Repeat([]byte("0123456789"), 100)) + string(bytes.Repeat([]byte{'Z'}, 64)) + "-suffix")
	ops, err := Diff(context.Background(), source, target, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var hasCopy, hasRun bool
	for _, op := range ops {
		hasCopy = hasCopy || op.Kind == OpCopy
		hasRun = hasRun || op.Kind == OpRun
	}
	if !hasCopy || !hasRun {
		t.Fatalf("expected COPY and RUN, got %#v", ops)
	}
	decoded, err := ApplyOps(context.Background(), source, ops, len(target), Limits{})
	if err != nil || !bytes.Equal(decoded, target) {
		t.Fatalf("apply failed: %v", err)
	}
}

func TestRandomizedRoundTrips(t *testing.T) {
	t.Parallel()
	for seed := int64(0); seed < 100; seed++ {
		size := 256 + int(seed%16)*241
		source := fixtureBytes(seed, size)
		target := mutateFixture(seed+1_000, source)
		for _, codec := range []string{"vcdiff", "private"} {
			var delta, decoded []byte
			var err error
			if codec == "vcdiff" {
				delta, err = EncodeVCDIFF(context.Background(), source, target, Limits{})
				if err == nil {
					decoded, err = DecodeVCDIFF(context.Background(), source, delta, Limits{}, 1)
				}
			} else {
				delta, err = EncodePrivate(context.Background(), source, target, Limits{})
				if err == nil {
					decoded, err = DecodePrivate(context.Background(), source, delta, Limits{}, 1)
				}
			}
			if err != nil || !bytes.Equal(decoded, target) {
				t.Fatalf("seed=%d codec=%s: %v", seed, codec, err)
			}
		}
	}
}

func makeVCDIFFWindow(indicator byte, sourceSize, sourcePosition, targetLength int, data, instructions, addresses []byte) []byte {
	body := appendVarint(nil, uint64(targetLength))
	body = append(body, 0)
	body = appendVarint(body, uint64(len(data)))
	body = appendVarint(body, uint64(len(instructions)))
	body = appendVarint(body, uint64(len(addresses)))
	body = append(body, data...)
	body = append(body, instructions...)
	body = append(body, addresses...)
	window := []byte{indicator}
	if indicator&vcdSource != 0 {
		window = appendVarint(window, uint64(sourceSize))
		window = appendVarint(window, uint64(sourcePosition))
	}
	window = appendVarint(window, uint64(len(body)))
	return append(window, body...)
}

func makeVCDIFF(indicator byte, sourceSize, sourcePosition, targetLength int, data, instructions, addresses []byte) []byte {
	return append(append([]byte(nil), vcdiffMagic...), makeVCDIFFWindow(indicator, sourceSize, sourcePosition, targetLength, data, instructions, addresses)...)
}

func TestHostileVCDIFFIsRejected(t *testing.T) {
	t.Parallel()
	source := []byte("abcdefghij")
	valid, err := EncodeVCDIFF(context.Background(), source, []byte("abcdefghijX"), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for length := 0; length < len(valid); length++ {
		if length == len(vcdiffMagic) {
			continue // A header-only VCDIFF file validly represents empty output.
		}
		if _, err := DecodeVCDIFF(context.Background(), source, valid[:length], Limits{}, 1); err == nil {
			t.Fatalf("truncated length %d was accepted", length)
		}
	}

	cases := []struct {
		name  string
		delta []byte
	}{
		{name: "custom-code-table", delta: append([]byte{0xD6, 0xC3, 0xC4, 0, 2}, valid[5:]...)},
		{name: "target-window", delta: makeVCDIFF(2, 0, 0, 1, []byte{'x'}, []byte{2}, nil)},
		{name: "bad-source-range", delta: makeVCDIFF(1, 99, 0, 1, []byte{'x'}, []byte{2}, nil)},
		{name: "unsupported-opcode", delta: makeVCDIFF(0, 0, 0, 1, nil, []byte{35, 1}, []byte{0})},
		{name: "unused-data", delta: makeVCDIFF(0, 0, 0, 1, []byte{'x', 'y'}, []byte{2}, nil)},
		{name: "copy-before-target", delta: makeVCDIFF(0, 0, 0, 4, nil, []byte{20}, []byte{0})},
	}
	compressed := makeVCDIFF(0, 0, 0, 1, []byte{'x'}, []byte{2}, nil)
	compressed[8] = 1 // target length at 7; delta indicator at 8
	cases = append(cases, struct {
		name  string
		delta []byte
	}{name: "secondary-compression", delta: compressed})

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeVCDIFF(context.Background(), source, test.delta, Limits{}, 1); err == nil {
				t.Fatal("hostile delta was accepted")
			}
		})
	}
}

func TestEveryDecoderLimit(t *testing.T) {
	t.Parallel()
	source := bytes.Repeat([]byte("source"), 40)
	target := append(append([]byte(nil), source...), 'x')
	delta, err := EncodeVCDIFF(context.Background(), source, target, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeVCDIFF(context.Background(), source, target, Limits{MaxSourceBytes: 8}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("source limit: %v", err)
	}
	if _, err := EncodeVCDIFF(context.Background(), source, target, Limits{MaxTargetBytes: 8}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("target limit: %v", err)
	}
	if _, err := DecodeVCDIFF(context.Background(), source, delta, Limits{MaxDeltaBytes: 8}, 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("delta limit: %v", err)
	}
	if _, err := DecodeVCDIFF(context.Background(), source, delta, Limits{MaxWindowBytes: 8}, 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("window limit: %v", err)
	}
	if _, err := DecodeVCDIFF(context.Background(), source, delta, Limits{MaxSectionBytes: 1}, 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("section limit: %v", err)
	}
	if _, err := DecodeVCDIFF(context.Background(), source, delta, Limits{MaxOperations: 1}, 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("operation limit: %v", err)
	}
	if _, err := DecodeVCDIFF(context.Background(), source, delta, Limits{MaxVarintBytes: 1}, 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("varint limit: %v", err)
	}
	if _, err := DecodeVCDIFF(context.Background(), source, delta, Limits{}, DefaultLimits().MaxChainDepth+1); !errors.Is(err, ErrChainTooDeep) {
		t.Fatalf("chain limit: %v", err)
	}

	window := makeVCDIFFWindow(0, 0, 0, 1, []byte{'x'}, []byte{2}, nil)
	twoWindows := append(append(append([]byte(nil), vcdiffMagic...), window...), window...)
	if _, err := DecodeVCDIFF(context.Background(), nil, twoWindows, Limits{MaxWindows: 1}, 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("window-count limit: %v", err)
	}

	runInstructions := append([]byte{0}, appendVarint(nil, 100)...)
	expanding := makeVCDIFF(0, 0, 0, 100, []byte{'x'}, runInstructions, nil)
	if _, err := DecodeVCDIFF(context.Background(), nil, expanding, Limits{MaxExpansionRatio: 1}, 1); !errors.Is(err, ErrExpansion) {
		t.Fatalf("expansion limit: %v", err)
	}
}

func TestIntegerOverflowAndExpansionArithmetic(t *testing.T) {
	t.Parallel()
	overflowing := append(bytes.Repeat([]byte{0xff}, 9), 0x7f)
	p := parser{data: overflowing, maxVarintBytes: 10}
	if _, err := p.readVarint(); !errors.Is(err, ErrInvalidDelta) {
		t.Fatalf("overflowing varint: %v", err)
	}
	if exceedsExpansion(math.MaxInt, 2, math.MaxInt) {
		t.Fatal("large expansion limit overflowed into a refusal")
	}
	if !exceedsExpansion(101, 10, 10) {
		t.Fatal("fractional expansion above the limit was accepted")
	}
}

func TestCancellationAndStreaming(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Diff(ctx, []byte("source"), []byte("target"), Limits{}); !errors.Is(err, ErrCancelled) {
		t.Fatalf("cancelled diff: %v", err)
	}

	source := bytes.Repeat([]byte("0123456789"), 100)
	target := append(append([]byte(nil), source...), []byte("tail")...)
	var encoded bytes.Buffer
	if err := EncodeVCDIFFStream(context.Background(), bytes.NewReader(source), bytes.NewReader(target), &encoded, Limits{}); err != nil {
		t.Fatal(err)
	}
	var decoded bytes.Buffer
	if err := DecodeVCDIFFStream(context.Background(), bytes.NewReader(source), bytes.NewReader(encoded.Bytes()), &decoded, Limits{}, 1); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Bytes(), target) {
		t.Fatal("stream round trip mismatch")
	}
	if err := EncodeVCDIFFStream(context.Background(), bytes.NewReader(source), bytes.NewReader(target), zeroWriter{}, Limits{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short writer: %v", err)
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func TestPrivateCorruptionIsRejected(t *testing.T) {
	t.Parallel()
	source := bytes.Repeat([]byte("source"), 100)
	target := mutateFixture(91, source)
	delta, err := EncodePrivate(context.Background(), source, target, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for length := 0; length < len(delta); length++ {
		if _, err := DecodePrivate(context.Background(), source, delta[:length], Limits{}, 1); err == nil {
			t.Fatalf("truncated private length %d accepted", length)
		}
	}
	trailing := append(append([]byte(nil), delta...), 0)
	if _, err := DecodePrivate(context.Background(), source, trailing, Limits{}, 1); err == nil {
		t.Fatal("trailing private byte accepted")
	}
}

func FuzzDecodeVCDIFFNeverPanics(f *testing.F) {
	source := []byte("abcdefghij")
	valid, _ := EncodeVCDIFF(context.Background(), source, []byte("abcdefghijX"), Limits{})
	f.Add(source, valid)
	f.Add([]byte(nil), []byte{0xD6, 0xC3, 0xC4, 0, 0})
	f.Fuzz(func(t *testing.T, source, delta []byte) {
		limits := DefaultLimits()
		limits.MaxSourceBytes = 64 << 10
		limits.MaxDeltaBytes = 64 << 10
		limits.MaxTargetBytes = 64 << 10
		limits.MaxWindowBytes = 64 << 10
		_, _ = DecodeVCDIFF(context.Background(), source, delta, limits, 1)
	})
}

func FuzzDecodePrivateNeverPanics(f *testing.F) {
	source := []byte("abcdefghij")
	valid, _ := EncodePrivate(context.Background(), source, []byte("abcdefghijX"), Limits{})
	f.Add(source, valid)
	f.Add([]byte(nil), []byte("NXD1"))
	f.Fuzz(func(t *testing.T, source, delta []byte) {
		limits := DefaultLimits()
		limits.MaxSourceBytes = 64 << 10
		limits.MaxDeltaBytes = 64 << 10
		limits.MaxTargetBytes = 64 << 10
		limits.MaxWindowBytes = 64 << 10
		_, _ = DecodePrivate(context.Background(), source, delta, limits, 1)
	})
}
