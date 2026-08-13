// Command evidence measures the promoted canonical codec and the artifact
// crypto against the JSON representation the local journal stores today, at
// G2's operation tiers. Every operation is generated; no private content is
// read or recorded.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

type tier struct {
	Operations         int     `json:"operations"`
	JSONBytes          int     `json:"json_bytes"`
	CanonicalBytes     int     `json:"canonical_bytes"`
	SmallerPercent     float64 `json:"canonical_smaller_percent"`
	JSONGzipBytes      int     `json:"json_gzip_bytes"`
	CanonicalGzipBytes int     `json:"canonical_gzip_bytes"`
	GzipSmallerPercent float64 `json:"canonical_gzip_smaller_percent"`
	EncodeMS           int64   `json:"encode_ms"`
	DecodeMS           int64   `json:"decode_ms"`
	SealMS             int64   `json:"seal_ms"`
	OpenMS             int64   `json:"open_ms"`
	ArtifactBytes      int     `json:"artifact_bytes"`
	ArtifactOverhead   int     `json:"artifact_overhead_bytes"`
	RoundTripExact     bool    `json:"round_trip_exact"`
}

type report struct {
	Schema       string         `json:"schema"`
	GeneratedFor string         `json:"generated_for"`
	Protocol     string         `json:"protocol"`
	Primitives   map[string]any `json:"primitives"`
	Tiers        []tier         `json:"tiers"`
	Notes        []string       `json:"notes"`
}

func main() {
	out := flag.String("out", "", "path to write the JSON report")
	flag.Parse()
	if *out == "" {
		fail(fmt.Errorf("-out is required"))
	}
	keys, err := syncwire.NewMemoryKeyRing("key_evidence")
	if err != nil {
		fail(err)
	}
	signer, err := syncwire.NewMemorySigner()
	if err != nil {
		fail(err)
	}
	verifier := syncwire.NewMemoryVerifier()
	verifier.Enroll(signer.Public())

	result := report{
		Schema:       "notrios.g9.wire.v1",
		GeneratedFor: "v0.7 G9 deterministic envelope codec, encryption, and signatures",
		Protocol:     fmt.Sprintf("%d.%d", syncwire.ProtocolMajor, syncwire.ProtocolMinor),
		Primitives: map[string]any{
			"aead":                  "AES-256-GCM (crypto/aes, crypto/cipher)",
			"kdf":                   "HKDF-SHA256 (crypto/hkdf)",
			"signature":             "Ed25519 (crypto/ed25519)",
			"routing_blind":         "HMAC-SHA256 (crypto/hmac)",
			"external_dependencies": 0,
			"license":               "Go standard library, BSD-3-Clause",
		},
		Notes: []string{
			"Every operation is generated. No private corpus content is read or recorded.",
			"The JSON baseline is the representation the local journal stores today.",
			"artifact_overhead_bytes is the whole cost of the header, the AEAD tag, and the signature.",
			"Desktop measurements on one host; no carrier or network is involved.",
		},
	}
	for _, count := range []int{100, 10_000} {
		measured, err := measure(count, keys, signer, verifier)
		if err != nil {
			fail(err)
		}
		result.Tiers = append(result.Tiers, measured)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	for _, row := range result.Tiers {
		fmt.Printf("%6d ops  json=%9d canonical=%9d (%5.2f%% smaller)  gzip %8d vs %8d (%5.2f%%)  seal=%4dms open=%4dms overhead=%d exact=%v\n",
			row.Operations, row.JSONBytes, row.CanonicalBytes, row.SmallerPercent,
			row.JSONGzipBytes, row.CanonicalGzipBytes, row.GzipSmallerPercent,
			row.SealMS, row.OpenMS, row.ArtifactOverhead, row.RoundTripExact)
	}
}

func measure(count int, keys syncwire.KeyRing, signer syncwire.Signer, verifier syncwire.Verifier) (tier, error) {
	envelope := syncwire.Envelope{
		ProtocolMajor: syncwire.ProtocolMajor, ProtocolMinor: syncwire.ProtocolMinor,
		DatabaseID: "db_evidence", SenderReplicaID: "replica-evidence",
		StateVector: syncstate.Vector{"replica-evidence": int64(count)},
	}
	for index := 0; index < count; index++ {
		envelope.Operations = append(envelope.Operations, generatedOperation(index))
	}

	// The baseline: what the journal stores per operation today.
	jsonBytes := 0
	for _, operation := range envelope.Operations {
		encoded, err := json.Marshal(operation)
		if err != nil {
			return tier{}, err
		}
		jsonBytes += len(encoded)
	}

	started := time.Now()
	canonical, err := syncwire.EncodeEnvelope(envelope, syncwire.Limits{})
	if err != nil {
		return tier{}, err
	}
	encodeMS := time.Since(started).Milliseconds()

	started = time.Now()
	decoded, err := syncwire.DecodeEnvelope(canonical, syncwire.Limits{})
	if err != nil {
		return tier{}, err
	}
	decodeMS := time.Since(started).Milliseconds()

	jsonGzip, err := syncwire.Compress(jsonBaseline(envelope.Operations))
	if err != nil {
		return tier{}, err
	}
	canonicalGzip, err := syncwire.Compress(canonical)
	if err != nil {
		return tier{}, err
	}

	started = time.Now()
	artifact, err := syncwire.Seal(keys, signer, syncwire.KindEnvelope, "", canonical, syncwire.Limits{})
	if err != nil {
		return tier{}, err
	}
	sealMS := time.Since(started).Milliseconds()

	started = time.Now()
	_, opened, err := syncwire.Open(keys, verifier, artifact, syncwire.Limits{})
	if err != nil {
		return tier{}, err
	}
	openMS := time.Since(started).Milliseconds()

	exact := len(opened) == len(canonical) && string(opened) == string(canonical) &&
		len(decoded.Operations) == count && decoded.DatabaseID == envelope.DatabaseID

	return tier{
		Operations: count, JSONBytes: jsonBytes, CanonicalBytes: len(canonical),
		SmallerPercent:     percent(jsonBytes-len(canonical), jsonBytes),
		JSONGzipBytes:      len(jsonGzip),
		CanonicalGzipBytes: len(canonicalGzip),
		GzipSmallerPercent: percent(len(jsonGzip)-len(canonicalGzip), len(jsonGzip)),
		EncodeMS:           encodeMS, DecodeMS: decodeMS, SealMS: sealMS, OpenMS: openMS,
		ArtifactBytes: len(artifact), ArtifactOverhead: len(artifact) - len(canonical),
		RoundTripExact: exact,
	}, nil
}

func jsonBaseline(operations []syncstate.Operation) []byte {
	var out []byte
	for _, operation := range operations {
		encoded, _ := json.Marshal(operation)
		out = append(out, encoded...)
		out = append(out, '\n')
	}
	return out
}

func generatedOperation(index int) syncstate.Operation {
	return syncstate.Operation{
		ReplicaID: "replica-evidence", Sequence: int64(index + 1),
		OperationID: fmt.Sprintf("replica-evidence:%020d", index+1),
		Kind:        "record.update", RecordType: "document",
		RecordID:  fmt.Sprintf("doc_%08d", index),
		CreatedAt: "2026-08-13T05:00:00Z",
		HLC:       syncstate.HLC{WallMS: 1_700_000_000_000 + int64(index), Logical: int64(index % 7)},
		Payload:   []byte(fmt.Sprintf(`{"title":"Generated note %d","notebook_id":"nb_%04d"}`, index, index%64)),
	}
}

func percent(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "g9 evidence:", err)
	os.Exit(1)
}
