package harness

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// InspectTree reads metadata only. The digest binds relative names and metadata
// for a before/after read-only assertion, but no name is returned in evidence.
func InspectTree(root string) (Inventory, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, err
	}
	directoryEntries := map[string]int64{}
	metadata := make([]string, 0, 1024)
	result := Inventory{}
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if relative != "." {
			directoryEntries[filepath.Dir(relative)]++
		}
		metadata = append(metadata, fmt.Sprintf("%s\x00%d\x00%d\x00%d", filepath.ToSlash(relative), info.Mode(), info.Size(), info.ModTime().UnixNano()))
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			result.Symlinks++
		case entry.IsDir():
			result.Directories++
		case info.Mode().IsRegular():
			result.Files++
			result.ApparentBytes += info.Size()
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				result.AllocatedBytes += stat.Blocks * 512
			}
		default:
			result.OtherEntries++
		}
		return nil
	})
	if err != nil {
		return Inventory{}, err
	}
	// WalkDir includes root even when empty.
	if result.Directories > 0 {
		for directory := range directoryEntries {
			_ = directory
		}
		// Directories absent from directoryEntries are empty.
		nonempty := int64(len(directoryEntries))
		result.DirectoryHistogram.Empty = result.Directories - nonempty
	}
	for _, count := range directoryEntries {
		if count > result.MaximumDirectorySize {
			result.MaximumDirectorySize = count
		}
		switch {
		case count <= 10:
			result.DirectoryHistogram.OneToTen++
		case count <= 100:
			result.DirectoryHistogram.ElevenTo100++
		case count <= 1_000:
			result.DirectoryHistogram.HundredOneTo1K++
		case count <= 10_000:
			result.DirectoryHistogram.OneKTo10K++
		case count <= 100_000:
			result.DirectoryHistogram.TenKTo100K++
		default:
			result.DirectoryHistogram.Over100K++
		}
	}
	sort.Strings(metadata)
	digest := sha256.New()
	for _, line := range metadata {
		_, _ = digest.Write([]byte(line))
		_, _ = digest.Write([]byte{'\n'})
	}
	result.MetadataSHA256 = hex.EncodeToString(digest.Sum(nil))
	return result, nil
}

func SameInventory(left, right Inventory) bool {
	return left == right
}

func DetectEnvironment(path string) Environment {
	return Environment{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(),
		CPUModel: cpuModel(), LogicalCPUs: runtime.NumCPU(), SystemMemoryKiB: systemMemoryKiB(),
		FilesystemType: commandLine("stat", "-f", "-c", "%T", path),
		ToolVersions: map[string]string{
			"borg": commandLine("borg", "--version"), "restic": commandLine("restic", "version"),
			"sqlite": commandLine("sqlite3", "--version"), "go": commandLine("go", "version"),
		},
	}
}

func commandLine(name string, args ...string) string {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return "unavailable"
	}
	line := strings.TrimSpace(string(output))
	if before, _, ok := strings.Cut(line, "\n"); ok {
		line = before
	}
	if len(line) > 240 {
		line = line[:240]
	}
	return line
}

func cpuModel() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "unavailable"
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if key, value, ok := strings.Cut(scanner.Text(), ":"); ok && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return "unavailable"
}

func systemMemoryKiB() int64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			value, _ := strconv.ParseInt(fields[1], 10, 64)
			return value
		}
	}
	return 0
}
