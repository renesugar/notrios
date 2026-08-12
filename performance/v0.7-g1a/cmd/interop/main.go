// Command interop records external VCDIFF oracle results for the G1a
// investigation. External executables are test oracles only; they are not
// linked, vendored, or used by Notrios at runtime.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/performance/v0.7-g1a/xdelta"
)

type toolEvidence struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	License      string `json:"license"`
	SourceCommit string `json:"source_commit"`
	Version      string `json:"version"`
}

type directionEvidence struct {
	DeltaBytes int    `json:"delta_bytes"`
	Accepted   bool   `json:"accepted"`
	Exact      bool   `json:"exact"`
	Note       string `json:"note,omitempty"`
}

type fixtureEvidence struct {
	ID          string                                  `json:"id"`
	TargetBytes int                                     `json:"target_bytes"`
	TargetHash  string                                  `json:"target_sha256"`
	Results     map[string]map[string]directionEvidence `json:"results"`
}

type report struct {
	Schema      string            `json:"schema"`
	GeneratedAt string            `json:"generated_at"`
	Profile     string            `json:"profile"`
	Tools       []toolEvidence    `json:"tools"`
	Fixtures    []fixtureEvidence `json:"fixtures"`
	Summary     map[string]any    `json:"summary"`
}

func hash(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func run(name string, arguments ...string) ([]byte, error) {
	command := exec.Command(name, arguments...)
	return command.CombinedOutput()
}

func version(name string, arguments ...string) string {
	output, err := run(name, arguments...)
	if err != nil {
		return "unavailable"
	}
	line := strings.Split(strings.TrimSpace(string(output)), "\n")[0]
	return strings.TrimSpace(line)
}

func externalDecode(tool, sourcePath, deltaPath, outputPath string, openVCDIFF bool) error {
	var output []byte
	var err error
	if openVCDIFF {
		output, err = run(tool, "decode", "-dictionary="+sourcePath, "-delta="+deltaPath, "-target="+outputPath)
	} else {
		output, err = run(tool, "-f", "-d", "-s", sourcePath, deltaPath, outputPath)
	}
	if err != nil {
		return fmt.Errorf("decode failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func externalEncode(tool, sourcePath, targetPath, deltaPath string, openVCDIFF bool) error {
	var output []byte
	var err error
	if openVCDIFF {
		output, err = run(tool, "encode", "-dictionary="+sourcePath, "-target="+targetPath, "-delta="+deltaPath)
	} else {
		output, err = run(tool, "-f", "-e", "-n", "-A=", "-S", "none", "-s", sourcePath, targetPath, deltaPath)
	}
	if err != nil {
		return fmt.Errorf("encode failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func safeNote(err error) string {
	if err == nil {
		return ""
	}
	// Decoder errors contain only grammar/limit categories, never fixture bytes.
	return err.Error()
}

func inspectTool(
	ctx context.Context,
	toolName string,
	toolPath string,
	openVCDIFF bool,
	sourcePath string,
	targetPath string,
	goDeltaPath string,
	temporaryDirectory string,
	source []byte,
	target []byte,
) (map[string]directionEvidence, error) {
	decodedPath := filepath.Join(temporaryDirectory, toolName+"-decoded.bin")
	goDelta, err := os.ReadFile(goDeltaPath)
	if err != nil {
		return nil, err
	}
	goToTool := directionEvidence{DeltaBytes: len(goDelta)}
	if err := externalDecode(toolPath, sourcePath, goDeltaPath, decodedPath, openVCDIFF); err != nil {
		goToTool.Note = err.Error()
	} else {
		decoded, readErr := os.ReadFile(decodedPath)
		if readErr != nil {
			return nil, readErr
		}
		goToTool.Accepted = true
		goToTool.Exact = bytes.Equal(decoded, target)
	}

	externalDeltaPath := filepath.Join(temporaryDirectory, toolName+"-encoded.vcdiff")
	if err := externalEncode(toolPath, sourcePath, targetPath, externalDeltaPath, openVCDIFF); err != nil {
		return nil, err
	}
	externalDelta, err := os.ReadFile(externalDeltaPath)
	if err != nil {
		return nil, err
	}
	toolToGo := directionEvidence{DeltaBytes: len(externalDelta)}
	decoded, decodeErr := xdelta.DecodeVCDIFF(ctx, source, externalDelta, xdelta.DefaultLimits(), 1)
	toolToGo.Accepted = decodeErr == nil
	toolToGo.Exact = decodeErr == nil && bytes.Equal(decoded, target)
	toolToGo.Note = safeNote(decodeErr)

	return map[string]directionEvidence{
		"go_to_tool": goToTool,
		"tool_to_go": toolToGo,
	}, nil
}

func main() {
	var fixturesDirectory, openVCDIFFPath, xdelta3Path, outputPath string
	flag.StringVar(&fixturesDirectory, "fixtures", "", "temporary oracle fixture directory")
	flag.StringVar(&openVCDIFFPath, "open-vcdiff", "", "open-vcdiff executable")
	flag.StringVar(&xdelta3Path, "xdelta3", "", "xdelta3 executable")
	flag.StringVar(&outputPath, "out", "performance/v0.7-g1a/interoperability.json", "evidence output")
	flag.Parse()
	if fixturesDirectory == "" || openVCDIFFPath == "" || xdelta3Path == "" {
		fmt.Fprintln(os.Stderr, "-fixtures, -open-vcdiff, and -xdelta3 are required")
		os.Exit(2)
	}

	entries, err := os.ReadDir(fixturesDirectory)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	reportValue := report{
		Schema:      "notrios.g1a.interoperability.v1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Profile:     "RFC 3284 default table; Go emitter uses RUN, ADD, and SELF COPY only",
		Tools: []toolEvidence{
			{Name: "open-vcdiff", Role: "external test oracle only", License: "Apache-2.0", SourceCommit: "868f459a8d815125c2457f8c74b12493853100f9", Version: "source commit (CLI has no version flag)"},
			{Name: "xdelta3", Role: "external test oracle only", License: "Apache-2.0", SourceCommit: "9822b17313263d458b80511b08124971fc0e04fa", Version: version(xdelta3Path, "-V")},
		},
	}
	allOutboundExact := true
	inboundAccepted := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fixturePath := filepath.Join(fixturesDirectory, entry.Name())
		sourcePath := filepath.Join(fixturePath, "source.bin")
		targetPath := filepath.Join(fixturePath, "target.bin")
		goDeltaPath := filepath.Join(fixturePath, "go.vcdiff")
		source, readErr := os.ReadFile(sourcePath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			os.Exit(1)
		}
		target, readErr := os.ReadFile(targetPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			os.Exit(1)
		}
		temporaryDirectory, tempErr := os.MkdirTemp("", "notrios-g1a-interop-")
		if tempErr != nil {
			fmt.Fprintln(os.Stderr, tempErr)
			os.Exit(1)
		}
		results := make(map[string]map[string]directionEvidence)
		for _, item := range []struct {
			name string
			path string
			open bool
		}{{"open-vcdiff", openVCDIFFPath, true}, {"xdelta3", xdelta3Path, false}} {
			result, inspectErr := inspectTool(context.Background(), item.name, item.path, item.open, sourcePath, targetPath, goDeltaPath, temporaryDirectory, source, target)
			if inspectErr != nil {
				_ = os.RemoveAll(temporaryDirectory)
				fmt.Fprintf(os.Stderr, "%s %s: %v\n", entry.Name(), item.name, inspectErr)
				os.Exit(1)
			}
			results[item.name] = result
			allOutboundExact = allOutboundExact && result["go_to_tool"].Exact
			if result["tool_to_go"].Accepted {
				inboundAccepted++
			}
		}
		_ = os.RemoveAll(temporaryDirectory)
		reportValue.Fixtures = append(reportValue.Fixtures, fixtureEvidence{
			ID: entry.Name(), TargetBytes: len(target), TargetHash: hash(target), Results: results,
		})
	}
	reportValue.Summary = map[string]any{
		"fixtures":                        len(reportValue.Fixtures),
		"outbound_external_decodes_exact": allOutboundExact,
		"external_encodes_accepted_by_constrained_go_decoder": inboundAccepted,
		"external_encode_attempts":                            len(reportValue.Fixtures) * 2,
		"interpretation":                                      "Outbound portability is demonstrated. The prototype decoder intentionally accepts a strict profile, not arbitrary default-table VCDIFF streams.",
	}
	rendered, err := json.MarshalIndent(reportValue, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rendered = append(rendered, '\n')
	if err := os.WriteFile(outputPath, rendered, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %d fixtures, outbound exact=%t\n", outputPath, len(reportValue.Fixtures), allOutboundExact)
}
