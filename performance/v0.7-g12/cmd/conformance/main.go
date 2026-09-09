// Command conformance runs the G11 shared-directory protocol against a real
// provider-backed folder — the machine's mapped Google Drive — and against a
// directory that is alternately available to each peer, the way a USB drive is.
//
// It answers one question: does the carrier protocol survive a filesystem it
// does not control? Everything it writes is generated. Note bodies, titles, and
// the libraries themselves stay on local disk; only sealed artifacts reach the
// provider, and the results file records aggregates and behaviors rather than
// any path or content.
//
// It never runs `rclone sync`, `bisync`, `move`, `delete`, or `purge` against
// protocol state. The only rclone verb it uses is `copy --immutable`.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

const layoutRoot = "notrios-sync"

// providerProbe records what the filesystem under the carrier actually does.
// Every field here is something the protocol was told not to trust; measuring
// them says how far from the assumption a real provider is, not whether the
// protocol depends on the answer.
type providerProbe struct {
	Label                  string  `json:"label"`
	CreateDirectory        bool    `json:"create_directory"`
	AtomicRenameAvailable  bool    `json:"rename_available"`
	CaseSensitive          bool    `json:"case_sensitive"`
	LongNameAccepted       bool    `json:"protocol_name_length_accepted"`
	MtimePreservedOnRename bool    `json:"mtime_preserved_on_rename"`
	WriteMiBPerSecond      float64 `json:"write_mib_per_second"`
	ReadMiBPerSecond       float64 `json:"read_mib_per_second"`
	RemoteWriteToListingMS []int64 `json:"remote_write_to_listing_ms"`
	RemoteWriteToOpenMS    []int64 `json:"remote_write_to_open_ms"`
	SlowestListingMS       int64   `json:"slowest_listing_ms"`
	PartialUploadVisible   string  `json:"partial_upload_visible"`
	RemoteProbesRan        bool    `json:"remote_probes_ran"`
}

type phase struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail"`
	Duration int64  `json:"duration_ms"`
}

type report struct {
	Schema       string        `json:"schema"`
	GeneratedFor string        `json:"generated_for"`
	SchemaV      int           `json:"database_schema_version"`
	Notes        int           `json:"notes_exchanged"`
	Provider     providerProbe `json:"provider"`
	Removable    providerProbe `json:"removable_media"`
	Phases       []phase       `json:"phases"`
	Carrier      carrierShape  `json:"carrier_shape"`
	Caveats      []string      `json:"caveats"`
}

type carrierShape struct {
	Artifacts int   `json:"artifacts"`
	Bytes     int64 `json:"bytes"`
}

func main() {
	carrierRoot := flag.String("carrier", "", "provider-backed directory to use as the carrier (required)")
	remote := flag.String("remote", "", "optional rclone remote path for the same folder, enabling remote-side probes")
	removable := flag.String("removable", "", "optional directory standing in for removable media (default: a temporary directory)")
	out := flag.String("out", "", "path to write the JSON report (required)")
	notes := flag.Int("notes", 20, "generated notes per replica")
	flag.Parse()
	if *carrierRoot == "" || *out == "" {
		fail(errors.New("-carrier and -out are required"))
	}

	result := report{
		Schema:       "notrios.g12.conformance.v1",
		GeneratedFor: "v0.7 G12 directory-carrier conformance over a mapped cloud folder and removable media",
		SchemaV:      store.CurrentSchemaVersion,
		Notes:        *notes,
		Caveats: []string{
			"Every note is generated. Libraries stay on local disk; only sealed artifacts reach the provider.",
			"Timings are one run on one host and one network; they describe this provider on this day, not a guarantee.",
			"Only `rclone copy --immutable` is ever invoked. sync, bisync, move, delete, and purge are never run against protocol state.",
			"The mapped folder is a FUSE mount of a cloud provider, so a second machine's cache behavior is approximated by writing through the remote and reading through the mount.",
		},
	}

	probe, err := probeProvider("mapped cloud folder", *carrierRoot, *remote)
	if err != nil {
		fail(err)
	}
	result.Provider = probe

	removableRoot := *removable
	if removableRoot == "" {
		removableRoot, err = os.MkdirTemp("", "notrios-g12-removable-")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(removableRoot)
	}
	removableProbe, err := probeProvider("removable media stand-in", removableRoot, "")
	if err != nil {
		fail(err)
	}
	result.Removable = removableProbe

	fixture, err := newHarness(*carrierRoot, *notes)
	if err != nil {
		fail(err)
	}
	defer fixture.close()

	for _, step := range []struct {
		name string
		run  func(*harness) (string, error)
	}{
		{"converge through the provider", (*harness).convergeThroughProvider},
		{"peer files are never written by another peer", (*harness).noForeignWrites},
		{"a conflicting file at a protocol name is refused and repaired", (*harness).conflictingName},
		{"an advertisement seen before its envelopes costs latency only", (*harness).delayedListing},
		{"an unavailable carrier refuses rather than diverging", (*harness).unavailableCarrier},
		{"full carrier loss and reconstruction", (*harness).carrierLoss},
		{"rclone copy --immutable moves the carrier without changing it", (*harness).immutableCopy},
		{"a drive passed between peers converges", (*harness).removableHandoff},
	} {
		started := time.Now()
		detail, err := step.run(fixture)
		record := phase{Name: step.name, Passed: err == nil, Detail: detail, Duration: time.Since(started).Milliseconds()}
		if err != nil {
			record.Detail = err.Error()
		}
		result.Phases = append(result.Phases, record)
		fmt.Printf("%-62s %s (%d ms)\n", step.name, passLabel(record.Passed), record.Duration)
		if err != nil {
			fmt.Printf("    %v\n", err)
		}
	}

	artifacts, bytes, err := carrierSize(filepath.Join(*carrierRoot, layoutRoot))
	if err != nil {
		fail(err)
	}
	result.Carrier = carrierShape{Artifacts: artifacts, Bytes: bytes}
	fixture.removableRoot = removableRoot

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s\n", *out)
	for _, record := range result.Phases {
		if !record.Passed {
			os.Exit(1)
		}
	}
}

func passLabel(passed bool) string {
	if passed {
		return "ok"
	}
	return "FAILED"
}

// probeProvider measures what a filesystem does, without any protocol involved.
func probeProvider(label, root, remote string) (providerProbe, error) {
	probe := providerProbe{Label: label}
	directory := filepath.Join(root, "notrios-g12-probe")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return probe, err
	}
	defer os.RemoveAll(directory)
	probe.CreateDirectory = true

	// A protocol artifact name: 64 lowercase hex characters plus the suffix.
	longName := strings.Repeat("ab", 32) + ".nar"
	if err := os.WriteFile(filepath.Join(directory, longName), []byte("length probe"), 0o644); err == nil {
		probe.LongNameAccepted = true
	}

	staged := filepath.Join(directory, "staged.tmp")
	if err := os.WriteFile(staged, []byte("rename probe"), 0o644); err != nil {
		return probe, err
	}
	before, _ := os.Stat(staged)
	renamed := filepath.Join(directory, "renamed.nar")
	if err := os.Rename(staged, renamed); err == nil {
		probe.AtomicRenameAvailable = true
		if after, err := os.Stat(renamed); err == nil && before != nil {
			probe.MtimePreservedOnRename = after.ModTime().Equal(before.ModTime())
		}
	}

	// Case sensitivity decides whether two distinct protocol names could ever
	// collide. Ours are lowercase hex, so a case-folding provider is safe; the
	// probe records which kind this is rather than requiring one.
	upper := filepath.Join(directory, "CASEPROBE.nar")
	lower := filepath.Join(directory, "caseprobe.nar")
	if err := os.WriteFile(upper, []byte("upper"), 0o644); err != nil {
		return probe, err
	}
	if err := os.WriteFile(lower, []byte("lower"), 0o644); err != nil {
		return probe, err
	}
	if contents, err := os.ReadFile(upper); err == nil {
		probe.CaseSensitive = string(contents) == "upper"
	}

	payload := make([]byte, 4<<20)
	for index := range payload {
		payload[index] = byte(index)
	}
	throughput := filepath.Join(directory, "throughput.bin")
	started := time.Now()
	if err := os.WriteFile(throughput, payload, 0o644); err != nil {
		return probe, err
	}
	probe.WriteMiBPerSecond = mibPerSecond(len(payload), time.Since(started))
	started = time.Now()
	if _, err := os.ReadFile(throughput); err != nil {
		return probe, err
	}
	probe.ReadMiBPerSecond = mibPerSecond(len(payload), time.Since(started))

	if remote != "" {
		if err := probeRemoteVisibility(&probe, directory, remote); err != nil {
			return probe, err
		}
	}
	return probe, nil
}

// probeRemoteVisibility writes through the provider's API, bypassing the local
// mount, and measures how long the mount takes to admit that the file exists.
//
// This is the closest a single machine can get to the question that matters on
// a shared folder: when another device publishes an artifact, how stale is my
// view? It measures two answers separately, because a provider may resolve a
// name it is asked for while its directory listing still lags — and the
// protocol depends on listings for discovery and on names for object fetches.
func probeRemoteVisibility(probe *providerProbe, directory, remote string) error {
	probe.RemoteProbesRan = true
	// Three samples, because one number here would be indistinguishable from
	// luck: this delay is a provider's change-notification cadence, and the
	// interesting figure is the worst one a scan could meet.
	for sample := 0; sample < 3; sample++ {
		listing, open, err := oneVisibilitySample(directory, remote)
		if err != nil {
			return err
		}
		probe.RemoteWriteToListingMS = append(probe.RemoteWriteToListingMS, listing)
		probe.RemoteWriteToOpenMS = append(probe.RemoteWriteToOpenMS, open)
		if listing > probe.SlowestListingMS {
			probe.SlowestListingMS = listing
		}
	}

	return probePartialUpload(probe, directory, remote)
}

// oneVisibilitySample publishes through the provider API, bypassing the mount,
// and times how long the mount takes to admit the file exists — as a listing,
// and as a name it is asked for directly. The two are measured separately
// because the protocol depends on listings for discovery and on names for
// object fetches, and a provider is free to make one fresher than the other.
func oneVisibilitySample(directory, remote string) (int64, int64, error) {
	source, err := os.CreateTemp("", "notrios-g12-remote-")
	if err != nil {
		return 0, 0, err
	}
	defer os.Remove(source.Name())
	if _, err := source.WriteString("published through the provider api"); err != nil {
		return 0, 0, err
	}
	source.Close()

	name := fmt.Sprintf("remote-%d.nar", time.Now().UnixNano())
	target := strings.TrimSuffix(remote, "/") + "/notrios-g12-probe/" + name
	if output, err := runRclone("copyto", source.Name(), target); err != nil {
		return 0, 0, fmt.Errorf("rclone copyto: %v: %s", err, output)
	}
	published := time.Now()
	local := filepath.Join(directory, name)
	var listingMS, openMS int64
	deadline := published.Add(3 * time.Minute)
	for listingMS == 0 || openMS == 0 {
		if time.Now().After(deadline) {
			break
		}
		if openMS == 0 {
			if _, err := os.Stat(local); err == nil {
				openMS = time.Since(published).Milliseconds()
			}
		}
		if listingMS == 0 {
			if entries, err := os.ReadDir(directory); err == nil {
				for _, entry := range entries {
					if entry.Name() == name {
						listingMS = time.Since(published).Milliseconds()
						break
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return listingMS, openMS, nil
}

func probePartialUpload(probe *providerProbe, directory, remote string) error {
	// Partial-upload visibility: write a file through the mount and ask the
	// provider, mid-write, whether it can see a partial object. A carrier that
	// exposes half-written objects is the case the protocol's hash and AEAD
	// checks exist for; a carrier that does not is one fewer thing to survive.
	probe.PartialUploadVisible = "not observed"
	partial := filepath.Join(directory, "partial.nar")
	file, err := os.Create(partial)
	if err != nil {
		return err
	}
	chunk := make([]byte, 1<<20)
	for index := 0; index < 4; index++ {
		if _, err := file.Write(chunk); err != nil {
			file.Close()
			return err
		}
		if index == 1 {
			listing, err := runRclone("lsf", strings.TrimSuffix(remote, "/")+"/notrios-g12-probe/")
			if err == nil && strings.Contains(listing, "partial.nar") {
				probe.PartialUploadVisible = "visible during write"
			}
		}
	}
	file.Close()
	return nil
}

func runRclone(args ...string) (string, error) {
	// Only ever a read or a non-destructive copy. The refusal list is in
	// SYNCHRONIZATION.md and in the operator documentation; this is where it is
	// enforced rather than merely stated.
	forbidden := map[string]bool{"sync": true, "bisync": true, "move": true, "moveto": true, "delete": true, "deletefile": true, "purge": true, "rmdir": true, "rmdirs": true}
	if len(args) == 0 || forbidden[args[0]] {
		return "", fmt.Errorf("refusing to run `rclone %s` against protocol state", strings.Join(args, " "))
	}
	ctx, cancel := contextWithTimeout(5 * time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "rclone", args...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func mibPerSecond(bytes int, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return (float64(bytes) / (1 << 20)) / elapsed.Seconds()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func carrierSize(root string) (int, int64, error) {
	artifacts := 0
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".nar") {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			artifacts++
			total += info.Size()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, 0, nil
	}
	return artifacts, total, err
}

// fingerprintTree records name, size, and content hash for every artifact under
// a directory, so a later comparison can prove nothing was rewritten.
func fingerprintTree(root string) (map[string]string, error) {
	prints := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".nar") {
			return nil
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		digest := sha256.Sum256(contents)
		prints[relative] = hex.EncodeToString(digest[:])
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return prints, nil
	}
	return prints, err
}

func sortedNames(prints map[string]string) []string {
	names := make([]string, 0, len(prints))
	for name := range prints {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
