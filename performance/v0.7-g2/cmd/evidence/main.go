package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"syscall"
	"time"

	"github.com/renesugar/notrios/performance/v0.7-g2/bounds"
)

const measurementRuns = 3

type distribution struct {
	Count            int64 `json:"count"`
	BytesTotal       int64 `json:"bytes_total"`
	BytesP50         int64 `json:"bytes_p50"`
	BytesP90         int64 `json:"bytes_p90"`
	BytesP95         int64 `json:"bytes_p95"`
	BytesP99         int64 `json:"bytes_p99"`
	BytesMax         int64 `json:"bytes_max"`
	CountAtLeast1MiB int64 `json:"count_at_least_1_mib,omitempty"`
}

type aggregateInputs struct {
	Schema  string `json:"schema"`
	Privacy string `json:"privacy"`
	Corpora []struct {
		Label     string       `json:"label"`
		Bodies    distribution `json:"bodies"`
		Resources distribution `json:"resources"`
	} `json:"corpora"`
	ArchiveLayouts []struct {
		Label         string  `json:"label"`
		BytesBasis    string  `json:"bytes_basis"`
		LooseFiles    int64   `json:"loose_files"`
		PackedFiles   int64   `json:"packed_files"`
		LooseBytes    int64   `json:"loose_bytes"`
		PackedBytes   int64   `json:"packed_bytes"`
		LooseSeconds  float64 `json:"loose_seconds"`
		PackedSeconds float64 `json:"packed_seconds"`
	} `json:"archive_layouts"`
}

type metric struct {
	CPUNanosP50       int64  `json:"cpu_nanos_p50"`
	WallNanosP50      int64  `json:"wall_nanos_p50"`
	AllocatedBytesP50 uint64 `json:"allocated_bytes_p50"`
	PeakRSSKiB        int64  `json:"peak_rss_kib"`
}

type formatResult struct {
	Format                         string  `json:"format"`
	Operations                     int     `json:"operations"`
	EncodedBytes                   int     `json:"encoded_bytes"`
	CompressedBytes                int     `json:"compressed_bytes"`
	CompressedRatio                float64 `json:"compressed_ratio"`
	Encode                         metric  `json:"encode"`
	Decode                         metric  `json:"decode"`
	Compress                       metric  `json:"compress"`
	Decompress                     metric  `json:"decompress"`
	Exact                          bool    `json:"exact"`
	Deterministic                  bool    `json:"deterministic"`
	ProposedEnvelopeCount          int     `json:"proposed_envelope_count"`
	MaximumEnvelopeEncodedBytes    int     `json:"maximum_envelope_encoded_bytes"`
	MaximumEnvelopeCompressedBytes int     `json:"maximum_envelope_compressed_bytes"`
}

type resourcePolicyResult struct {
	WholeBelow int64                              `json:"whole_below"`
	ChunkBytes int64                              `json:"chunk_bytes"`
	Cases      map[string]bounds.ResourceEstimate `json:"cases"`
	Corpora    map[string]map[string]int64        `json:"corpus_proxy"`
}

type evidence struct {
	Schema           string                 `json:"schema"`
	GeneratedAt      string                 `json:"generated_at"`
	Generator        string                 `json:"generator"`
	Environment      map[string]any         `json:"environment"`
	Inputs           aggregateInputs        `json:"inputs"`
	OperationTiers   []formatResult         `json:"operation_tiers"`
	ResourcePolicies []resourcePolicyResult `json:"resource_policies"`
	PackReuse        []map[string]any       `json:"pack_reuse"`
	HostileCases     []map[string]any       `json:"hostile_cases"`
	SelectedLimits   map[string]any         `json:"selected_limits"`
}

func cpuNanos(value syscall.Rusage) int64 {
	return value.Utime.Sec*int64(time.Second) + value.Utime.Usec*int64(time.Microsecond) +
		value.Stime.Sec*int64(time.Second) + value.Stime.Usec*int64(time.Microsecond)
}

func peakRSSKiB() int64 {
	var usage syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &usage) != nil {
		return 0
	}
	return usage.Maxrss
}

func medianInt64(values []int64) int64 {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[len(values)/2]
}
func medianUint64(values []uint64) uint64 {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[len(values)/2]
}

func measure(action func() error) (metric, error) {
	cpuValues := make([]int64, 0, measurementRuns)
	wallValues := make([]int64, 0, measurementRuns)
	allocations := make([]uint64, 0, measurementRuns)
	for run := 0; run < measurementRuns; run++ {
		var beforeUsage, afterUsage syscall.Rusage
		var beforeMemory, afterMemory runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&beforeMemory)
		_ = syscall.Getrusage(syscall.RUSAGE_SELF, &beforeUsage)
		started := time.Now()
		err := action()
		wall := time.Since(started).Nanoseconds()
		_ = syscall.Getrusage(syscall.RUSAGE_SELF, &afterUsage)
		runtime.ReadMemStats(&afterMemory)
		if err != nil {
			return metric{}, err
		}
		cpuValues = append(cpuValues, cpuNanos(afterUsage)-cpuNanos(beforeUsage))
		wallValues = append(wallValues, wall)
		allocations = append(allocations, afterMemory.TotalAlloc-beforeMemory.TotalAlloc)
	}
	return metric{CPUNanosP50: medianInt64(cpuValues), WallNanosP50: medianInt64(wallValues), AllocatedBytesP50: medianUint64(allocations), PeakRSSKiB: peakRSSKiB()}, nil
}

func identifier(prefix string, index int, size int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", prefix, index)))
	return hex.EncodeToString(sum[:size])
}

func payloadFor(index int, corpora []struct {
	Label     string       `json:"label"`
	Bodies    distribution `json:"bodies"`
	Resources distribution `json:"resources"`
}) []byte {
	corpus := corpora[index%len(corpora)]
	bodySizes := []int64{corpus.Bodies.BytesP50, corpus.Bodies.BytesP90, corpus.Bodies.BytesP95, corpus.Bodies.BytesP99, corpus.Bodies.BytesMax}
	payload := fmt.Sprintf("{\"corpus\":\"%s\",\"result_length\":%d,\"field\":\"%s\",\"value\":\"synthetic-%08d\"}", corpus.Label, bodySizes[index%len(bodySizes)], []string{"revision", "title", "tag", "resource", "parent"}[index%5], index)
	return []byte(payload)
}

func operations(count int, inputs aggregateInputs) []bounds.Operation {
	result := make([]bounds.Operation, count)
	for index := range result {
		dependencyCount := 0
		if index%5 == 0 {
			dependencyCount = 1
		}
		if index%29 == 0 {
			dependencyCount = 2
		}
		dependencies := make([]string, dependencyCount)
		for dependencyIndex := range dependencies {
			dependencies[dependencyIndex] = identifier("dependency", index*3+dependencyIndex, 32)
		}
		result[index] = bounds.Operation{
			ReplicaID: identifier("replica", index%3, 16), Sequence: uint64(index/3 + 1),
			HLCWallMS: 1_786_000_000_000 + uint64(index/64), HLCLogical: uint32(index % 64),
			Kind: uint8(index % 8), ObjectID: identifier("object", index%16384, 16),
			Dependencies: dependencies, Payload: payloadFor(index, inputs.Corpora),
		}
	}
	return result
}

type formatFunctions struct {
	name   string
	encode func([]bounds.Operation, bounds.Limits) ([]byte, error)
	decode func([]byte, bounds.Limits) ([]bounds.Operation, error)
}

func measureFormat(ops []bounds.Operation, limits bounds.Limits, functions formatFunctions) (formatResult, error) {
	encoded, err := functions.encode(ops, limits)
	if err != nil {
		return formatResult{}, err
	}
	encodedAgain, err := functions.encode(ops, limits)
	if err != nil {
		return formatResult{}, err
	}
	compressed, err := bounds.CompressDeterministic(encoded)
	if err != nil {
		return formatResult{}, err
	}
	compressedAgain, err := bounds.CompressDeterministic(encoded)
	if err != nil {
		return formatResult{}, err
	}
	decoded, err := functions.decode(encoded, limits)
	if err != nil {
		return formatResult{}, err
	}
	exact := reflect.DeepEqual(decoded, ops)
	encodeMetric, err := measure(func() error { value, err := functions.encode(ops, limits); runtime.KeepAlive(value); return err })
	if err != nil {
		return formatResult{}, err
	}
	decodeMetric, err := measure(func() error { value, err := functions.decode(encoded, limits); runtime.KeepAlive(value); return err })
	if err != nil {
		return formatResult{}, err
	}
	compressMetric, err := measure(func() error {
		value, err := bounds.CompressDeterministic(encoded)
		runtime.KeepAlive(value)
		return err
	})
	if err != nil {
		return formatResult{}, err
	}
	decompressMetric, err := measure(func() error {
		value, err := bounds.DecompressBounded(compressed, limits)
		runtime.KeepAlive(value)
		return err
	})
	if err != nil {
		return formatResult{}, err
	}

	proposed := bounds.ProposedLimits()
	maxEncoded, maxCompressed, envelopeCount := 0, 0, 0
	for start := 0; start < len(ops); start += proposed.MaxOperations {
		end := min(len(ops), start+proposed.MaxOperations)
		part, err := functions.encode(ops[start:end], proposed)
		if err != nil {
			return formatResult{}, fmt.Errorf("proposed envelope: %w", err)
		}
		packed, err := bounds.CompressDeterministic(part)
		if err != nil {
			return formatResult{}, err
		}
		if len(part) > maxEncoded {
			maxEncoded = len(part)
		}
		if len(packed) > maxCompressed {
			maxCompressed = len(packed)
		}
		envelopeCount++
	}
	return formatResult{
		Format: functions.name, Operations: len(ops), EncodedBytes: len(encoded), CompressedBytes: len(compressed),
		CompressedRatio: float64(len(compressed)) / float64(max(1, len(encoded))), Encode: encodeMetric, Decode: decodeMetric,
		Compress: compressMetric, Decompress: decompressMetric, Exact: exact,
		Deterministic:         bytes.Equal(encoded, encodedAgain) && bytes.Equal(compressed, compressedAgain),
		ProposedEnvelopeCount: envelopeCount, MaximumEnvelopeEncodedBytes: maxEncoded, MaximumEnvelopeCompressedBytes: maxCompressed,
	}, nil
}

func resourcePolicies(inputs aggregateInputs) []resourcePolicyResult {
	var results []resourcePolicyResult
	for _, whole := range []int64{256 << 10, 1 << 20, 4 << 20} {
		for _, chunk := range []int64{256 << 10, 1 << 20, 4 << 20} {
			policy := bounds.ResourcePolicy{WholeBelow: whole, ChunkBytes: chunk}
			cases := map[string]bounds.ResourceEstimate{}
			for label, size := range map[string]int64{"zero": 0, "small": 64 << 10, "large": 32 << 20, "attachment-heavy": 107951795} {
				cases[label], _ = bounds.EstimateResource(size, 64<<10, policy)
			}
			corpora := map[string]map[string]int64{}
			for _, corpus := range inputs.Corpora {
				corpora[corpus.Label] = bounds.EstimateDistribution(bounds.Distribution{
					Count: corpus.Resources.Count, BytesTotal: corpus.Resources.BytesTotal, BytesP50: corpus.Resources.BytesP50,
					BytesP90: corpus.Resources.BytesP90, BytesP95: corpus.Resources.BytesP95, BytesP99: corpus.Resources.BytesP99, BytesMax: corpus.Resources.BytesMax,
				}, policy)
			}
			results = append(results, resourcePolicyResult{WholeBelow: whole, ChunkBytes: chunk, Cases: cases, Corpora: corpora})
		}
	}
	return results
}

func hostileCases() []map[string]any {
	limits := bounds.ProposedLimits()
	cases := []map[string]any{}
	add := func(name string, err error) {
		cases = append(cases, map[string]any{"name": name, "rejected": err != nil})
	}
	_, err := bounds.EncodeJSONL(make([]bounds.Operation, limits.MaxOperations+1), limits)
	add("operation-count", err)
	oversized := []bounds.Operation{{ReplicaID: identifier("replica", 0, 16), ObjectID: identifier("object", 0, 16), Payload: make([]byte, limits.MaxPayloadBytes+1)}}
	_, err = bounds.EncodeJSONL(oversized, limits)
	add("payload-size", err)
	bomb, _ := bounds.CompressDeterministic(bytes.Repeat([]byte{0}, 2<<20))
	_, err = bounds.DecompressBounded(bomb, limits)
	add("compression-ratio", err)
	oneMember, _ := bounds.CompressDeterministic([]byte("one member"))
	_, err = bounds.DecompressBounded(append(append([]byte(nil), oneMember...), oneMember...), limits)
	add("multiple-gzip-members", err)
	_, err = bounds.DecodeCompact([]byte("NCB1\x81\x00"), limits)
	add("non-minimal-varint", err)
	_, err = bounds.DecodeJSONL([]byte("{\"replica_id\":\"00\",\"unknown\":true}\n"), limits)
	add("unknown-json-field", err)
	return cases
}

func main() {
	inputPath := "performance/v0.7-g2/corpus-inputs.json"
	outputPath := "performance/v0.7-g2/benchmark-results.json"
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		panic(err)
	}
	var inputs aggregateInputs
	if err := json.Unmarshal(raw, &inputs); err != nil {
		panic(err)
	}
	measurementLimits := bounds.ProposedLimits()
	measurementLimits.MaxOperations = 100_000
	measurementLimits.MaxEncodedBytes = 256 << 20
	measurementLimits.MaxCompressedBytes = 64 << 20
	measurementLimits.MaxExpansionRatio = 256
	formats := []formatFunctions{{"canonical-jsonl", bounds.EncodeJSONL, bounds.DecodeJSONL}, {"compact-ncb1", bounds.EncodeCompact, bounds.DecodeCompact}}
	var tiers []formatResult
	for _, count := range []int{100, 10_000, 100_000} {
		ops := operations(count, inputs)
		for _, functions := range formats {
			result, err := measureFormat(ops, measurementLimits, functions)
			if err != nil {
				panic(fmt.Sprintf("%s/%d: %v", functions.name, count, err))
			}
			tiers = append(tiers, result)
		}
	}
	packReuse := make([]map[string]any, 0, len(inputs.ArchiveLayouts))
	for _, layout := range inputs.ArchiveLayouts {
		packReuse = append(packReuse, map[string]any{
			"label": layout.Label, "bytes_basis": layout.BytesBasis, "loose_files": layout.LooseFiles, "packed_files": layout.PackedFiles,
			"file_reduction": float64(layout.LooseFiles) / float64(layout.PackedFiles),
			"byte_ratio":     float64(layout.PackedBytes) / float64(layout.LooseBytes),
			"speedup":        layout.LooseSeconds / layout.PackedSeconds,
		})
	}
	proposed := bounds.ProposedLimits()
	resourcePolicy := bounds.ProposedResourcePolicy()
	result := evidence{
		Schema: "notrios.g2.bounds-evidence.v1", GeneratedAt: time.Now().UTC().Format(time.RFC3339), Generator: "go run ./performance/v0.7-g2/cmd/evidence",
		Environment: map[string]any{"go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH, "cpu_count": runtime.NumCPU(), "measurement_runs": measurementRuns, "desktop_proxy_only": true},
		Inputs:      inputs, OperationTiers: tiers, ResourcePolicies: resourcePolicies(inputs), PackReuse: packReuse, HostileCases: hostileCases(),
		SelectedLimits: map[string]any{
			"operation_encoding":  "NCB1 compact canonical record stream; canonical JSON remains the outer manifest candidate",
			"envelope_operations": proposed.MaxOperations, "envelope_encoded_bytes": proposed.MaxEncodedBytes, "envelope_compressed_bytes": proposed.MaxCompressedBytes,
			"record_bytes": proposed.MaxRecordBytes, "operation_payload_bytes": proposed.MaxPayloadBytes, "dependencies_per_operation": proposed.MaxDependencies,
			"compression": "gzip/DEFLATE level 6; mtime 0; OS 255; no name/comment/extra fields", "expansion_ratio": proposed.MaxExpansionRatio,
			"pending_operations_per_peer": 10_000, "pending_encoded_bytes_per_peer": 64 << 20,
			"whole_resource_below_bytes": resourcePolicy.WholeBelow, "fixed_chunk_bytes": resourcePolicy.ChunkBytes, "maximum_resource_chunks": 16_384,
			"sync_pack_target_bytes": 64 << 20, "sync_pack_objects": 4_096, "sync_pack_trailer_bytes": 4 << 20,
		},
	}
	rendered, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	rendered = append(rendered, '\n')
	if err := os.WriteFile(outputPath, rendered, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s: %d tier rows, %d resource policies, %d hostile cases\n", outputPath, len(tiers), len(result.ResourcePolicies), len(result.HostileCases))
}
