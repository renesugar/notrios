package purge

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupDirName is where a purge backup goes, beside the state root.
const BackupDirName = "notrios-purge-backups"

// Step is what purge would do with one resolved root.
type Step struct {
	Category string `json:"category"`
	Path     string `json:"path"`
	Policy   string `json:"policy"`
	// Action is backup_then_delete, dispose, keep or refuse.
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
	Bytes  int64  `json:"bytes"`
	Files  int    `json:"files"`
}

// Plan decides, for every resolved root, what purge would do.
//
// Every candidate goes through the oracle. Nothing is deleted because this file
// thinks a path looks like a Notrios directory.
func Plan(roots map[string]string, env Environment) []Step {
	owned := make([]string, 0, len(roots))
	for _, path := range roots {
		owned = append(owned, realpath(path))
	}
	env.OwnedRoots = owned

	categories := make([]string, 0, len(roots))
	for category := range roots {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	steps := make([]Step, 0, len(categories))
	for _, category := range categories {
		path := roots[category]
		policy := BackupPolicy(category)

		if category == "program_assets" {
			// Installed artifacts can share a directory with the data root, and
			// that overlap is worth naming rather than leaving a reader to
			// notice two lines about one path: whatever installed the files
			// removes them, and the data step removes what is left.
			overlapping := []string{}
			for name, other := range roots {
				if name != category && realpath(other) == realpath(path) {
					overlapping = append(overlapping, name)
				}
			}
			sort.Strings(overlapping)
			reason := "not a data root; removed by whatever installed it"
			if len(overlapping) > 0 {
				reason += "; shares a directory with the " + strings.Join(overlapping, ", ") + " root"
			}
			steps = append(steps, Step{category, path, policy, "keep", reason, 0, 0})
			continue
		}

		decision := Decide(path, env)
		if decision.Verdict == Refuse {
			steps = append(steps, Step{category, path, policy, "refuse",
				fmt.Sprintf("%s [%s]", decision.Reason, decision.Rule), 0, 0})
			continue
		}

		var files int
		var total int64
		reason := "already absent"
		if decision.Verdict == Allow {
			files, total = measureTree(path)
			reason = ""
		}
		action := "backup_then_delete"
		if policy == "dispose" {
			action = "dispose"
		}
		steps = append(steps, Step{category, path, policy, action, reason, total, files})
	}
	return steps
}

// BackupDestination is where the backup goes: beside the state root rather than
// inside any root purge removes, so it cannot land somewhere the same run would
// delete. H3 proved this container and asserted the destination is itself
// refused by the oracle.
func BackupDestination(roots map[string]string, now time.Time) (string, error) {
	state := roots["state"]
	if state == "" {
		return "", fmt.Errorf("no state root resolved, so there is nowhere safe to put a backup")
	}
	parent := filepath.Dir(realpath(state))
	return filepath.Join(parent, BackupDirName, now.UTC().Format("20060102T150405Z")), nil
}

// Sync key material is the one thing a purge backup must not contain. The
// backup is an ordinary tar in a place chosen for convenience, and this file is
// the password to a library's synchronised traffic -- copying it there would
// put the key beside the lock. The sealed form is excluded too: on its own it
// is ciphertext, but the data key that opens it lives in the operating system's
// store and survives a purge, so the pair would be recoverable.
const syncKeyPrefix = "sync-keys-"

// IsSyncKeyMaterial reports whether a file name is a library's sync key
// material, matched by name rather than by reading the file: an unreadable or
// unrecognised file named like key material is still excluded, which is the
// safe direction to be wrong in.
func IsSyncKeyMaterial(name string) bool {
	base := filepath.Base(name)
	return base == "sync-keys.json" ||
		(strings.HasPrefix(base, syncKeyPrefix) && strings.HasSuffix(base, ".json"))
}

type backupEntry struct {
	Member string `json:"member"`
	Source string `json:"source"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// BackupManifest is the record written beside the archive.
type BackupManifest struct {
	Schema              string        `json:"schema"`
	CreatedAt           string        `json:"created_at"`
	ArchiveSHA256       string        `json:"archive_sha256"`
	ArchiveBytes        int64         `json:"archive_bytes"`
	Entries             []backupEntry `json:"entries"`
	ExcludedKeyMaterial []string      `json:"excluded_key_material"`
}

func sha256File(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, handle); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// CreateBackup writes one owner-only tar plus a manifest, before anything is
// deleted. Both are created 0600 from the start rather than chmod-ed
// afterwards, because a file that is briefly world-readable while it holds
// someone's notes was briefly wrong.
func CreateBackup(steps []Step, destination string) (BackupManifest, error) {
	manifest := BackupManifest{
		Schema:    "notrios.purge-backup/1",
		CreatedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Entries:   []backupEntry{},
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return manifest, err
	}
	archivePath := filepath.Join(destination, "backup.tar")
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return manifest, err
	}
	writer := tar.NewWriter(archive)

	excluded := []string{}
	for _, step := range steps {
		if step.Action != "backup_then_delete" {
			continue
		}
		root := step.Path
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !entry.Type().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			member := filepath.ToSlash(filepath.Join(step.Category, relative))
			if IsSyncKeyMaterial(path) {
				excluded = append(excluded, member)
				return nil
			}
			stat, err := entry.Info()
			if err != nil {
				return err
			}
			digest, err := sha256File(path)
			if err != nil {
				return err
			}
			header := &tar.Header{Name: member, Mode: 0o600, Size: stat.Size(),
				ModTime: stat.ModTime(), Typeflag: tar.TypeReg}
			if err := writer.WriteHeader(header); err != nil {
				return err
			}
			source, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(writer, source)
			source.Close()
			if copyErr != nil {
				return copyErr
			}
			manifest.Entries = append(manifest.Entries, backupEntry{member, path, stat.Size(), digest})
			return nil
		})
		if walkErr != nil {
			writer.Close()
			archive.Close()
			return manifest, walkErr
		}
	}
	if err := writer.Close(); err != nil {
		archive.Close()
		return manifest, err
	}
	if err := archive.Close(); err != nil {
		return manifest, err
	}

	digest, err := sha256File(archivePath)
	if err != nil {
		return manifest, err
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return manifest, err
	}
	sort.Strings(excluded)
	manifest.ArchiveSHA256 = digest
	manifest.ArchiveBytes = info.Size()
	manifest.ExcludedKeyMaterial = excluded

	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	return manifest, os.WriteFile(filepath.Join(destination, "MANIFEST.json"),
		append(encoded, '\n'), 0o600)
}

// VerifyBackup confirms the backup before anything is deleted.
//
// A failed verification is the only thing standing between the user and the
// deletion, so it checks the archive's own hash, that every recorded file is
// actually a member, and that the archive opens.
func VerifyBackup(destination string) (bool, string) {
	archivePath := filepath.Join(destination, "backup.tar")
	manifestPath := filepath.Join(destination, "MANIFEST.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, "the backup is incomplete"
	}
	if _, err := os.Stat(archivePath); err != nil {
		return false, "the backup is incomplete"
	}
	var manifest BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return false, fmt.Sprintf("the backup manifest cannot be read: %v", err)
	}
	digest, err := sha256File(archivePath)
	if err != nil {
		return false, fmt.Sprintf("the archive cannot be read: %v", err)
	}
	if digest != manifest.ArchiveSHA256 {
		return false, "the archive does not match the hash recorded when it was written"
	}

	handle, err := os.Open(archivePath)
	if err != nil {
		return false, fmt.Sprintf("the archive cannot be read: %v", err)
	}
	defer handle.Close()
	members := map[string]bool{}
	reader := tar.NewReader(handle)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Sprintf("the archive cannot be read: %v", err)
		}
		members[header.Name] = true
	}
	for _, entry := range manifest.Entries {
		if !members[entry.Member] {
			return false, entry.Member + " is recorded but not in the archive"
		}
	}
	// Checked against the archive itself rather than trusting that the filter
	// ran. An exclusion nothing verifies is an intention, and this one is a
	// boundary: sync key material must never reach a purge backup.
	leaked := []string{}
	for member := range members {
		if IsSyncKeyMaterial(member) {
			leaked = append(leaked, member)
		}
	}
	if len(leaked) > 0 {
		sort.Strings(leaked)
		return false, "the archive contains sync key material, which must never be backed up: " +
			strings.Join(leaked, ", ")
	}
	detail := fmt.Sprintf("%d files verified", len(manifest.Entries))
	if n := len(manifest.ExcludedKeyMaterial); n > 0 {
		detail += fmt.Sprintf("; %d sync key file(s) deliberately excluded", n)
	}
	return true, detail
}

func measureTree(path string) (int, int64) {
	var files int
	var total int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // an unreadable subtree is measured as empty, never as a reason to delete
		}
		if info, err := entry.Info(); err == nil {
			files++
			total += info.Size()
		}
		return nil
	})
	return files, total
}

// Remove deletes the paths a plan allows, and reports what it removed.
func Remove(steps []Step) ([]string, error) {
	removed := []string{}
	for _, step := range steps {
		if step.Action != "backup_then_delete" && step.Action != "dispose" {
			continue
		}
		if _, err := os.Lstat(step.Path); err != nil {
			continue
		}
		if err := os.RemoveAll(step.Path); err != nil {
			return removed, fmt.Errorf("removing %s: %w", step.Path, err)
		}
		removed = append(removed, step.Path)
	}
	return removed, nil
}
