package syncdelta

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"math"
	"math/rand"
	"strings"
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
			t.Parallel()
			delta, err := EncodeVCDIFF(context.Background(), test.source, test.target, Limits{})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeVCDIFF(context.Background(), test.source, delta, Limits{}, 1)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decoded, test.target) {
				t.Fatal("VCDIFF reconstruction mismatch")
			}
		})
	}
}

func TestEncodingIsDeterministic(t *testing.T) {
	t.Parallel()
	source := fixtureBytes(17, 256<<10)
	target := mutateFixture(18, source)
	first, err := EncodeVCDIFF(context.Background(), source, target, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 5; run++ {
		delta, err := EncodeVCDIFF(context.Background(), source, target, Limits{})
		if err != nil || !bytes.Equal(delta, first) {
			t.Fatalf("run %d is not deterministic: %v", run, err)
		}
	}
}

// The golden pins the promoted encoder to the exact bytes G1a's pinned xdelta3
// and open-vcdiff oracles decoded. A change here is a wire-format change, not a
// refactor, and every peer that already stored a delta is affected by it.
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
	// overlapping copy expands "abc" into "abcabcabc". The encoder never emits
	// this, but a conforming peer's decoder must accept it.
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
		delta, err := EncodeVCDIFF(context.Background(), source, target, Limits{})
		if err != nil {
			t.Fatalf("seed=%d: %v", seed, err)
		}
		decoded, err := DecodeVCDIFF(context.Background(), source, delta, Limits{}, 1)
		if err != nil || !bytes.Equal(decoded, target) {
			t.Fatalf("seed=%d: %v", seed, err)
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

func TestCancellationStopsMatchFinding(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Diff(ctx, []byte("source"), []byte("target"), Limits{}); !errors.Is(err, ErrCancelled) {
		t.Fatalf("cancelled diff: %v", err)
	}
	if _, _, err := EncodeBodyDelta(ctx, bytes.Repeat([]byte("a"), 512), bytes.Repeat([]byte("b"), 512)); !errors.Is(err, ErrCancelled) {
		t.Fatalf("cancellation must surface rather than be folded into a fallback: %v", err)
	}
}

func markdownBody(paragraphs int, marker string) []byte {
	var builder strings.Builder
	builder.WriteString("# Café 東京 notes 😀\n\n")
	for index := 0; index < paragraphs; index++ {
		builder.WriteString("## Section ")
		builder.WriteString(marker)
		builder.WriteString("\n\nThe quick brown fox jumps over the lazy dog, repeatedly and at length.\n\n")
	}
	return []byte(builder.String())
}

func TestBodyDeltaRoundTripAndBenefit(t *testing.T) {
	t.Parallel()
	base := markdownBody(40, "one")
	result := append(append([]byte(nil), base...), []byte("\nA short appended paragraph.\n")...)

	delta, ok, err := EncodeBodyDelta(context.Background(), base, result)
	if err != nil || !ok {
		t.Fatalf("expected a beneficial delta for an append: ok=%v err=%v", ok, err)
	}
	if delta.Format != FormatVCDIFF1 || delta.BaseSHA256 != SHA256Hex(base) || delta.ResultSHA256 != SHA256Hex(result) ||
		delta.BaseLength != len(base) || delta.ResultLength != len(result) {
		t.Fatalf("delta does not name both endpoints exactly: %+v", delta)
	}
	if len(delta.Bytes) >= len(result) {
		t.Fatalf("delta of %d bytes is not smaller than the %d-byte body", len(delta.Bytes), len(result))
	}
	decoded, err := ReconstructBody(context.Background(), base, delta)
	if err != nil || !bytes.Equal(decoded, result) {
		t.Fatalf("reconstruction mismatch: %v", err)
	}
}

// Both honest G1a fallbacks and the degenerate cases must produce "send the
// complete object" rather than an error, because a caller that cannot tell a
// refusal from a failure will treat one of them wrongly.
func TestUnbeneficialDeltasFallBackToTheCompleteObject(t *testing.T) {
	t.Parallel()
	large := fixtureBytes(3, 256<<10)
	cases := []struct {
		name         string
		base, result []byte
	}{
		{name: "empty-result", base: markdownBody(4, "x"), result: nil},
		{name: "no-base", base: nil, result: markdownBody(4, "x")},
		{name: "tiny-result", base: markdownBody(4, "x"), result: []byte("short")},
		{name: "unrelated-random", base: large, result: fixtureBytes(4, 256<<10)},
		{name: "complete-rewrite", base: markdownBody(20, "one"), result: fixtureBytes(5, 40<<10)},
		{name: "oversized-result", base: markdownBody(4, "x"), result: make([]byte, MaxObjectBytes+1)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			delta, ok, err := EncodeBodyDelta(context.Background(), test.base, test.result)
			if err != nil {
				t.Fatalf("an unbeneficial delta is an ordinary outcome, not an error: %v", err)
			}
			if ok {
				t.Fatalf("delta of %d bytes was accepted for a %d-byte body", len(delta.Bytes), len(test.result))
			}
		})
	}
}

func TestReconstructionRefusesEveryUnverifiedInput(t *testing.T) {
	t.Parallel()
	base := markdownBody(40, "one")
	result := append(append([]byte(nil), base...), []byte("\nAn appended paragraph that is long enough to matter.\n")...)
	delta, ok, err := EncodeBodyDelta(context.Background(), base, result)
	if err != nil || !ok {
		t.Fatalf("fixture delta: ok=%v err=%v", ok, err)
	}

	wrongBase := markdownBody(40, "two")
	if _, err := ReconstructBody(context.Background(), wrongBase, delta); !errors.Is(err, ErrBaseMismatch) {
		t.Fatalf("wrong base bytes: %v", err)
	}

	renamedBase := delta
	renamedBase.BaseSHA256 = SHA256Hex([]byte("something else"))
	if _, err := ReconstructBody(context.Background(), base, renamedBase); !errors.Is(err, ErrBaseMismatch) {
		t.Fatalf("wrong named base: %v", err)
	}

	shortBase := delta
	shortBase.BaseLength = len(base) - 1
	if _, err := ReconstructBody(context.Background(), base, shortBase); !errors.Is(err, ErrBaseMismatch) {
		t.Fatalf("wrong base length: %v", err)
	}

	wrongResult := delta
	wrongResult.ResultSHA256 = SHA256Hex([]byte("not the result"))
	if _, err := ReconstructBody(context.Background(), base, wrongResult); !errors.Is(err, ErrResultMismatch) {
		t.Fatalf("wrong named result: %v", err)
	}

	wrongLength := delta
	wrongLength.ResultLength = len(result) + 1
	if _, err := ReconstructBody(context.Background(), base, wrongLength); !errors.Is(err, ErrResultMismatch) {
		t.Fatalf("wrong result length: %v", err)
	}

	unknownFormat := delta
	unknownFormat.Format = "svndiff"
	if _, err := ReconstructBody(context.Background(), base, unknownFormat); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unknown format: %v", err)
	}

	// Every truncation and every single-byte corruption is refused. None may
	// return partially reconstructed bytes alongside an error.
	for length := 0; length < len(delta.Bytes); length++ {
		truncated := delta
		truncated.Bytes = delta.Bytes[:length]
		body, err := ReconstructBody(context.Background(), base, truncated)
		if err == nil {
			t.Fatalf("truncation to %d bytes was accepted", length)
		}
		if body != nil {
			t.Fatalf("truncation to %d bytes returned %d bytes with an error", length, len(body))
		}
	}
	for position := 0; position < len(delta.Bytes); position++ {
		corrupt := delta
		corrupt.Bytes = append([]byte(nil), delta.Bytes...)
		corrupt.Bytes[position] ^= 0xff
		if body, err := ReconstructBody(context.Background(), base, corrupt); err == nil && !bytes.Equal(body, result) {
			t.Fatalf("corruption at %d produced different bytes without an error", position)
		}
	}
}

func TestBeneficialGate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		delta, body int
		want        bool
	}{
		{name: "clear-saving", delta: 100, body: 10_000, want: true},
		{name: "body-below-floor", delta: 10, body: MinDeltaTargetBytes - 1, want: false},
		{name: "at-the-percentage-bound", delta: 750, body: 1_000, want: true},
		{name: "just-over-the-percentage-bound", delta: 751, body: 1_000, want: false},
		{name: "saving-below-the-byte-floor", delta: 200 - MinDeltaSavingBytes + 1, body: 200, want: false},
		{name: "larger-than-the-body", delta: 1_200, body: 1_000, want: false},
	}
	for _, test := range cases {
		if got := Beneficial(test.delta, test.body); got != test.want {
			t.Fatalf("%s: Beneficial(%d, %d) = %v", test.name, test.delta, test.body, got)
		}
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
