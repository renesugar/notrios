package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/renesugar/notrios/performance/v0.7-g1a/xdelta"
)

const measurementRuns = 7

type fixture struct {
	ID       string
	Class    string
	Case     string
	Interval string
	Source   []byte
	Target   []byte
}

type metric struct {
	CPUNanosP50       int64  `json:"cpu_nanos_p50"`
	CPUNanosP95       int64  `json:"cpu_nanos_p95"`
	WallNanosP50      int64  `json:"wall_nanos_p50"`
	AllocatedBytesP50 uint64 `json:"allocated_bytes_p50"`
	PeakRSSKiB        int64  `json:"peak_rss_kib"`
}

type operationSummary struct {
	Count     int `json:"count"`
	Adds      int `json:"adds"`
	Copies    int `json:"copies"`
	Runs      int `json:"runs"`
	AddBytes  int `json:"add_bytes"`
	CopyBytes int `json:"copy_bytes"`
	RunBytes  int `json:"run_bytes"`
}

type record struct {
	ID                    string           `json:"id"`
	Class                 string           `json:"class"`
	Case                  string           `json:"case,omitempty"`
	Interval              string           `json:"interval,omitempty"`
	SourceBytes           int              `json:"source_bytes"`
	TargetBytes           int              `json:"target_bytes"`
	SourceSHA256          string           `json:"source_sha256"`
	TargetSHA256          string           `json:"target_sha256"`
	CompleteBytes         int              `json:"complete_bytes"`
	G1LineJSONBytes       *int             `json:"g1_line_json_bytes,omitempty"`
	VCDIFFBytes           int              `json:"vcdiff_bytes"`
	VCDIFFRatio           float64          `json:"vcdiff_ratio_to_complete"`
	PrivateBytes          int              `json:"private_bytes"`
	PrivateRatio          float64          `json:"private_ratio_to_complete"`
	Operations            operationSummary `json:"operations"`
	VCDIFFEncode          metric           `json:"vcdiff_encode"`
	VCDIFFDecode          metric           `json:"vcdiff_decode"`
	PrivateEncode         metric           `json:"private_encode"`
	PrivateDecode         metric           `json:"private_decode"`
	ExactVCDIFFRoundTrip  bool             `json:"exact_vcdiff_round_trip"`
	ExactPrivateRoundTrip bool             `json:"exact_private_round_trip"`
	Deterministic         bool             `json:"deterministic"`
}

type evidence struct {
	Schema      string         `json:"schema"`
	GeneratedAt string         `json:"generated_at"`
	Generator   string         `json:"generator"`
	Environment map[string]any `json:"environment"`
	Profile     map[string]any `json:"profile"`
	Records     []record       `json:"records"`
	Summary     map[string]any `json:"summary"`
}

type g1Results struct {
	DeltaRecords []struct {
		Case         string `json:"case"`
		Interval     string `json:"interval"`
		Method       string `json:"method"`
		EncodedBytes int    `json:"encoded_bytes"`
	} `json:"delta_records"`
}

func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func textParagraph(index int) string {
	return fmt.Sprintf(
		"Paragraph %04d keeps deterministic Markdown with café, 東京, and 😀. It links to [note %d](notrios://document/%032x) and contains enough ordinary prose to resemble a note body.\n",
		index,
		index%17,
		index%17,
	)
}

func textFixture(caseName, interval string) ([]byte, []byte) {
	editsByInterval := map[string]int{"1-hour": 1, "30-days": 32}
	edits := editsByInterval[interval]
	if caseName == "very-long-line" {
		words := make([]string, 6_000)
		for index := range words {
			words[index] = fmt.Sprintf("token%05d", index)
		}
		base := strings.Join(words, " ") + "\n"
		for index := 0; index < edits; index++ {
			position := (index * 173) % len(words)
			words[position] += "-revised"
		}
		return []byte(base), []byte(strings.Join(words, " ") + "\n")
	}
	var builder strings.Builder
	builder.WriteString("# Deterministic synchronization workload\n\n")
	for index := 0; index < 96+edits*8; index++ {
		builder.WriteString(textParagraph(index))
		builder.WriteByte('\n')
	}
	base := builder.String()
	target := base
	switch caseName {
	case "localized":
		target = strings.Replace(target, "ordinary prose", "locally revised prose", 1)
	case "scattered":
		for index := 0; index < edits; index++ {
			old := fmt.Sprintf("Paragraph %04d", index*7)
			target = strings.Replace(target, old, old+" revised", 1)
		}
	case "append":
		for index := 0; index < edits; index++ {
			target += fmt.Sprintf("\nOffline addition %d: exact append payload.\n", index)
		}
	case "unicode":
		target = strings.Replace(target, "café, 東京, and 😀", "cafè, 京都, and 🧭", edits)
	case "markdown":
		target = strings.Replace(target, "](notrios://document/", "](notrios://stable/", edits)
	case "binary-looking":
		target = strings.Replace(target, "ordinary prose", "\x00\x01\x02 valid UTF-8 control-bearing text \x7f", edits)
	default:
		panic("unknown text case " + caseName)
	}
	return []byte(base), []byte(target)
}

func deterministicBytes(seed int64, size int) []byte {
	random := rand.New(rand.NewSource(seed))
	value := make([]byte, size)
	_, _ = random.Read(value)
	return value
}

func binaryFixtures() []fixture {
	sparseSource := deterministicBytes(73, 256<<10)
	sparseTarget := append([]byte(nil), sparseSource...)
	for offset := 97; offset < len(sparseTarget); offset += 7_919 {
		sparseTarget[offset] ^= 0x5a
	}
	sparseTarget = append(sparseTarget[:100_000], append(bytes.Repeat([]byte{0, 0xff, 0x7f}, 341), sparseTarget[100_000:]...)...)

	repetitiveSource := bytes.Repeat([]byte("0123456789ABCDEF"), 64<<10)
	repetitiveTarget := append([]byte(nil), repetitiveSource...)
	copy(repetitiveTarget[400_000:404_096], bytes.Repeat([]byte{0xA5}, 4_096))

	nulSource := make([]byte, 64<<10)
	for index := range nulSource {
		nulSource[index] = byte(index)
	}
	nulTarget := append([]byte(nil), nulSource...)
	copy(nulTarget[4_096:8_192], bytes.Repeat([]byte{0}, 4_096))

	pngSource := append([]byte("\x89PNG\r\n\x1a\nNOTRIOS-SYNTHETIC\x00"), deterministicBytes(99, 128<<10)...)
	pngTarget := append([]byte(nil), pngSource...)
	copy(pngTarget[1_024:1_088], []byte("changed-metadata-without-private-content.........................."))

	return []fixture{
		{ID: "binary-empty", Class: "binary", Source: nil, Target: nil},
		{ID: "binary-identical-random-65536", Class: "binary", Source: deterministicBytes(44, 64<<10), Target: deterministicBytes(44, 64<<10)},
		{ID: "binary-sparse-262144", Class: "binary", Source: sparseSource, Target: sparseTarget},
		{ID: "binary-unrelated-262144", Class: "binary", Source: deterministicBytes(1, 256<<10), Target: deterministicBytes(2, 256<<10)},
		{ID: "binary-repetitive-1048576", Class: "binary", Source: repetitiveSource, Target: repetitiveTarget},
		{ID: "binary-nul-65536", Class: "binary", Source: nulSource, Target: nulTarget},
		{ID: "attachment-png-like-131096", Class: "binary", Source: pngSource, Target: pngTarget},
	}
}

func allFixtures() []fixture {
	result := make([]fixture, 0, 21)
	for _, interval := range []string{"1-hour", "30-days"} {
		for _, caseName := range []string{"localized", "scattered", "append", "unicode", "markdown", "binary-looking", "very-long-line"} {
			source, target := textFixture(caseName, interval)
			result = append(result, fixture{
				ID:    "g1-" + caseName + "-" + interval,
				Class: "text", Case: caseName, Interval: interval, Source: source, Target: target,
			})
		}
	}
	return append(result, binaryFixtures()...)
}

func cpuNanos(value syscall.Rusage) int64 {
	return value.Utime.Sec*int64(time.Second) + value.Utime.Usec*int64(time.Microsecond) +
		value.Stime.Sec*int64(time.Second) + value.Stime.Usec*int64(time.Microsecond)
}

func percentileInt64(values []int64, fraction float64) int64 {
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	return values[int(float64(len(values)-1)*fraction)]
}

func percentileUint64(values []uint64, fraction float64) uint64 {
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	return values[int(float64(len(values)-1)*fraction)]
}

func measure(action func() ([]byte, error)) (metric, []byte, error) {
	cpuValues := make([]int64, 0, measurementRuns)
	wallValues := make([]int64, 0, measurementRuns)
	allocationValues := make([]uint64, 0, measurementRuns)
	var last []byte
	for run := 0; run < measurementRuns; run++ {
		var usageBefore, usageAfter syscall.Rusage
		var memoryBefore, memoryAfter runtime.MemStats
		runtime.ReadMemStats(&memoryBefore)
		_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usageBefore)
		started := time.Now()
		value, err := action()
		wall := time.Since(started).Nanoseconds()
		_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usageAfter)
		runtime.ReadMemStats(&memoryAfter)
		if err != nil {
			return metric{}, nil, err
		}
		last = value
		cpuValues = append(cpuValues, cpuNanos(usageAfter)-cpuNanos(usageBefore))
		wallValues = append(wallValues, wall)
		allocationValues = append(allocationValues, memoryAfter.TotalAlloc-memoryBefore.TotalAlloc)
	}
	return metric{
		CPUNanosP50:       percentileInt64(cpuValues, 0.50),
		CPUNanosP95:       percentileInt64(cpuValues, 0.95),
		WallNanosP50:      percentileInt64(wallValues, 0.50),
		AllocatedBytesP50: percentileUint64(allocationValues, 0.50),
		PeakRSSKiB:        peakRSSKiB(),
	}, last, nil
}

func summarizeOps(ops []xdelta.Op) operationSummary {
	summary := operationSummary{Count: len(ops)}
	for _, op := range ops {
		switch op.Kind {
		case xdelta.OpAdd:
			summary.Adds++
			summary.AddBytes += op.Length
		case xdelta.OpCopy:
			summary.Copies++
			summary.CopyBytes += op.Length
		case xdelta.OpRun:
			summary.Runs++
			summary.RunBytes += op.Length
		}
	}
	return summary
}

func ratio(encoded, complete int) float64 {
	if complete == 0 {
		if encoded == 0 {
			return 0
		}
		return 1
	}
	return float64(encoded) / float64(complete)
}

func loadG1LineBytes(path string) (map[string]int, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var input g1Results
	if err := json.Unmarshal(value, &input); err != nil {
		return nil, err
	}
	result := make(map[string]int)
	for _, record := range input.DeltaRecords {
		if record.Method == "line" {
			result[record.Case+"/"+record.Interval] = record.EncodedBytes
		}
	}
	return result, nil
}

func buildRecord(ctx context.Context, item fixture, lineBytes map[string]int, limits xdelta.Limits) (record, error) {
	ops, err := xdelta.Diff(ctx, item.Source, item.Target, limits)
	if err != nil {
		return record{}, err
	}
	vcdiffEncode, vcdiff, err := measure(func() ([]byte, error) {
		return xdelta.EncodeVCDIFF(ctx, item.Source, item.Target, limits)
	})
	if err != nil {
		return record{}, err
	}
	vcdiffDecode, decodedVCDIFF, err := measure(func() ([]byte, error) {
		return xdelta.DecodeVCDIFF(ctx, item.Source, vcdiff, limits, 1)
	})
	if err != nil {
		return record{}, err
	}
	privateEncode, private, err := measure(func() ([]byte, error) {
		return xdelta.EncodePrivate(ctx, item.Source, item.Target, limits)
	})
	if err != nil {
		return record{}, err
	}
	privateDecode, decodedPrivate, err := measure(func() ([]byte, error) {
		return xdelta.DecodePrivate(ctx, item.Source, private, limits, 1)
	})
	if err != nil {
		return record{}, err
	}
	repeatedVCDIFF, err := xdelta.EncodeVCDIFF(ctx, item.Source, item.Target, limits)
	if err != nil {
		return record{}, err
	}
	repeatedPrivate, err := xdelta.EncodePrivate(ctx, item.Source, item.Target, limits)
	if err != nil {
		return record{}, err
	}
	result := record{
		ID: item.ID, Class: item.Class, Case: item.Case, Interval: item.Interval,
		SourceBytes: len(item.Source), TargetBytes: len(item.Target),
		SourceSHA256: digest(item.Source), TargetSHA256: digest(item.Target), CompleteBytes: len(item.Target),
		VCDIFFBytes: len(vcdiff), VCDIFFRatio: ratio(len(vcdiff), len(item.Target)),
		PrivateBytes: len(private), PrivateRatio: ratio(len(private), len(item.Target)),
		Operations: summarizeOps(ops), VCDIFFEncode: vcdiffEncode, VCDIFFDecode: vcdiffDecode,
		PrivateEncode: privateEncode, PrivateDecode: privateDecode,
		ExactVCDIFFRoundTrip:  bytes.Equal(decodedVCDIFF, item.Target),
		ExactPrivateRoundTrip: bytes.Equal(decodedPrivate, item.Target),
		Deterministic:         bytes.Equal(vcdiff, repeatedVCDIFF) && bytes.Equal(private, repeatedPrivate),
	}
	if item.Class == "text" {
		if value, ok := lineBytes[item.Case+"/"+item.Interval]; ok {
			result.G1LineJSONBytes = &value
		}
	}
	return result, nil
}

func writeOracleFixture(directory string, item fixture, limits xdelta.Limits) error {
	path := filepath.Join(directory, item.ID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	delta, err := xdelta.EncodeVCDIFF(context.Background(), item.Source, item.Target, limits)
	if err != nil {
		return err
	}
	for name, value := range map[string][]byte{"source.bin": item.Source, "target.bin": item.Target, "go.vcdiff": delta} {
		if err := os.WriteFile(filepath.Join(path, name), value, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func commandOutput(name string, arguments ...string) string {
	value, err := exec.Command(name, arguments...).Output()
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(value))
}

func peakRSSKiB() int64 {
	var usage syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	return usage.Maxrss
}

func main() {
	var outputPath, oracleDirectory, g1Path string
	flag.StringVar(&outputPath, "out", "performance/v0.7-g1a/benchmark-results.json", "evidence output")
	flag.StringVar(&oracleDirectory, "oracle-dir", "", "optional temporary interoperability fixture directory")
	flag.StringVar(&g1Path, "g1-results", "performance/v0.7-g1/workload-results.json", "G1 comparison evidence")
	flag.Parse()

	lineBytes, err := loadG1LineBytes(g1Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	limits := xdelta.DefaultLimits()
	ctx := context.Background()
	fixtures := allFixtures()
	records := make([]record, 0, len(fixtures))
	for _, item := range fixtures {
		value, buildErr := buildRecord(ctx, item, lineBytes, limits)
		if buildErr != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", item.ID, buildErr)
			os.Exit(1)
		}
		records = append(records, value)
		if oracleDirectory != "" && (item.ID == "g1-localized-30-days" || item.ID == "binary-sparse-262144" || item.ID == "binary-repetitive-1048576") {
			if err := writeOracleFixture(oracleDirectory, item, limits); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}

	beneficialVCDIFF := 0
	beneficialPrivate := 0
	allExact := true
	allDeterministic := true
	for _, item := range records {
		if item.VCDIFFBytes < item.CompleteBytes {
			beneficialVCDIFF++
		}
		if item.PrivateBytes < item.CompleteBytes {
			beneficialPrivate++
		}
		allExact = allExact && item.ExactVCDIFFRoundTrip && item.ExactPrivateRoundTrip
		allDeterministic = allDeterministic && item.Deterministic
	}
	value := evidence{
		Schema:      "notrios.g1a.benchmark.v1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Generator:   "go run ./performance/v0.7-g1a/cmd/evidence",
		Environment: map[string]any{
			"go": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
			"kernel": commandOutput("uname", "-srmo"), "cpu_count": runtime.NumCPU(), "peak_rss_kib": peakRSSKiB(),
		},
		Profile: map[string]any{
			"measurement_runs": measurementRuns,
			"matcher":          "Subversion-style 64-byte source blocks and rolling pseudo-Adler checksum",
			"vcdiff":           "RFC 3284 default table; emit RUN/ADD/SELF-COPY only; one bounded target window",
			"private":          "NXD1 comparison container over identical operations",
			"limits":           limits,
		},
		Records: records,
		Summary: map[string]any{
			"fixtures": len(records), "text_fixtures": len(records) - len(binaryFixtures()), "binary_fixtures": len(binaryFixtures()),
			"beneficial_vcdiff": beneficialVCDIFF, "beneficial_private": beneficialPrivate,
			"all_exact": allExact, "all_deterministic": allDeterministic,
		},
	}
	rendered, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rendered = append(rendered, '\n')
	if err := os.WriteFile(outputPath, rendered, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %d fixtures, peak RSS %d KiB\n", outputPath, len(records), peakRSSKiB())
}
