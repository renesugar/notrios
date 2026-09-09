package syncdelta

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// FormatVCDIFF1 names the constrained RFC 3284 default-table profile this
// package emits and accepts. It is recorded with every stored delta so a later
// format can be added without guessing what existing rows contain.
const FormatVCDIFF1 = "vcdiff-1"

const (
	// MinDeltaTargetBytes is the smallest body worth attempting a delta for. A
	// VCDIFF window header alone costs a handful of bytes, so below this a
	// delta is arithmetic that cannot pay for itself.
	MinDeltaTargetBytes = 128

	// MaxBeneficialDeltaPercent caps an accepted delta at three quarters of the
	// complete body. G1 measured line deltas between 2.49% and 10.90% of a full
	// body across every ordinary edit interval and VCDIFF smaller still, so a
	// delta anywhere near this bound is a degenerate case — a very long line, a
	// rewritten note, or unrelated content — and is discarded rather than sent.
	MaxBeneficialDeltaPercent = 75

	// MinDeltaSavingBytes stops a delta that is technically smaller from being
	// stored and transferred for a saving that no carrier will notice.
	MinDeltaSavingBytes = 64
)

var (
	// ErrNoNamedBase reports that a delta named a base this replica cannot
	// produce. It is a refusal, never an invitation to patch something else.
	ErrNoNamedBase = errors.New("delta names an unavailable base object")
	// ErrBaseMismatch reports that the available base bytes do not hash to the
	// base the delta was generated against.
	ErrBaseMismatch = errors.New("delta base hash mismatch")
	// ErrResultMismatch reports that reconstruction produced bytes that are not
	// the named result. The reconstructed bytes are discarded.
	ErrResultMismatch = errors.New("reconstructed result hash mismatch")
)

// SHA256Hex is the single content-identity function for body and delta
// objects. Exact SHA-256 is the only identity Notrios recognizes.
func SHA256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// Delta is a transfer delta bound to both of its endpoints by exact hash. It
// carries the base it was generated against and the result it reconstructs, so
// a receiver can refuse it without decoding anything.
type Delta struct {
	Format       string `json:"format"`
	BaseSHA256   string `json:"base_sha256"`
	BaseLength   int    `json:"base_length"`
	ResultSHA256 string `json:"result_sha256"`
	ResultLength int    `json:"result_length"`
	Bytes        []byte `json:"bytes"`
}

// Beneficial reports whether a delta of this size is worth sending in place of
// the complete result. The gate is deliberately applied to the encoded delta
// rather than to a prediction, because G1a recorded two honest cases where a
// delta is larger than the content it replaces: an empty target still costs a
// five-byte header, and 256 KiB of unrelated random data encoded 22 bytes
// larger than simply sending it.
func Beneficial(deltaBytes, resultBytes int) bool {
	if resultBytes < MinDeltaTargetBytes {
		return false
	}
	if resultBytes-deltaBytes < MinDeltaSavingBytes {
		return false
	}
	return deltaBytes*100 <= resultBytes*MaxBeneficialDeltaPercent
}

// EncodeBodyDelta generates a delta from base to result and returns it only
// when it clears the benefit gate. A false second return is an ordinary
// outcome meaning "send the complete object", not a failure; so is any encoder
// limit refusal, which is why those are folded into it rather than returned.
func EncodeBodyDelta(ctx context.Context, base, result []byte) (Delta, bool, error) {
	if len(base) == 0 || len(base) > MaxObjectBytes || len(result) > MaxObjectBytes {
		return Delta{}, false, nil
	}
	encoded, err := EncodeVCDIFF(ctx, base, result, Limits{})
	if err != nil {
		if errors.Is(err, ErrCancelled) {
			return Delta{}, false, err
		}
		return Delta{}, false, nil
	}
	if !Beneficial(len(encoded), len(result)) {
		return Delta{}, false, nil
	}
	// Never publish a delta this replica cannot itself reconstruct. A generator
	// that emits an unverifiable patch turns its own bug into every peer's
	// refusal, and the cost of proving it here is one local decode.
	delta := Delta{
		Format:       FormatVCDIFF1,
		BaseSHA256:   SHA256Hex(base),
		BaseLength:   len(base),
		ResultSHA256: SHA256Hex(result),
		ResultLength: len(result),
		Bytes:        encoded,
	}
	if _, err := ReconstructBody(ctx, base, delta); err != nil {
		return Delta{}, false, nil
	}
	return delta, true, nil
}

// ReconstructBody verifies the named base, decodes the delta, and verifies the
// exact result hash and length before returning any bytes. Every failure is a
// refusal that leaves the caller to fetch the complete object; nothing partial
// or best-effort is ever returned.
func ReconstructBody(ctx context.Context, base []byte, delta Delta) ([]byte, error) {
	if delta.Format != FormatVCDIFF1 {
		return nil, fmt.Errorf("%w: delta format %q", ErrUnsupported, delta.Format)
	}
	if delta.ResultLength < 0 || delta.ResultLength > MaxObjectBytes || len(delta.Bytes) > MaxObjectBytes {
		return nil, fmt.Errorf("%w: delta or result bytes", ErrLimitExceeded)
	}
	if len(base) != delta.BaseLength || SHA256Hex(base) != delta.BaseSHA256 {
		return nil, ErrBaseMismatch
	}
	decoded, err := DecodeVCDIFF(ctx, base, delta.Bytes, Limits{}, 1)
	if err != nil {
		return nil, err
	}
	if len(decoded) != delta.ResultLength || SHA256Hex(decoded) != delta.ResultSHA256 {
		return nil, ErrResultMismatch
	}
	return decoded, nil
}
