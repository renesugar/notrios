package archive

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// MaxArchiveFileBytes bounds legacy metadata and note files. Resources retain
// the canonical 16 GiB per-object streaming ceiling.
const MaxArchiveFileBytes int64 = 16 << 20

type archiveLimitsConfig struct {
	MaxNotes          int
	MaxResources      int
	MaxScannedEntries int
	MaxNoteBytes      int64
	MaxResourceBytes  int64
}

var legacyArchiveLimits = archiveLimitsConfig{
	MaxNotes:          1_000_000,
	MaxResources:      1_000_000,
	MaxScannedEntries: 2_000_000,
	MaxNoteBytes:      16 << 30,
	MaxResourceBytes:  1 << 40,
}

const archiveReadDirBatch = 256

type archiveContents struct {
	noteNames []string
	notebooks []NotebookEntry
	resources []string
}

func openArchiveRegular(root *os.Root, path string, maxBytes int64) (*os.File, os.FileInfo, error) {
	before, err := root.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("archive entry %s is not a regular file", path)
	}
	if before.Size() > maxBytes {
		return nil, nil, fmt.Errorf("archive entry %s exceeds %d bytes", path, maxBytes)
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, nil, err
	}
	after, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		_ = f.Close()
		return nil, nil, fmt.Errorf("archive entry %s changed while opening", path)
	}
	if after.Size() > maxBytes {
		_ = f.Close()
		return nil, nil, fmt.Errorf("archive entry %s exceeds %d bytes", path, maxBytes)
	}
	return f, after, nil
}

func readArchiveFile(root *os.Root, path string, maxBytes int64) ([]byte, error) {
	f, _, err := openArchiveRegular(root, path, maxBytes)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("archive entry %s exceeds %d bytes", path, maxBytes)
	}
	return raw, nil
}

func validateArchiveDir(root *os.Root) (archiveContents, error) {
	contents := archiveContents{}
	var noteBytes, resourceBytes int64
	var scannedEntries int
	info, err := root.Lstat(".")
	if err != nil {
		return contents, err
	}
	if !info.IsDir() {
		return contents, errors.New("archive path is not a directory")
	}
	for _, name := range []string{"manifest.json", "notebooks.json"} {
		if _, err := root.Lstat(name); err == nil {
			if _, err := readArchiveFile(root, name, MaxArchiveFileBytes); err != nil {
				return contents, err
			}
		} else if !errors.Is(err, os.ErrNotExist) || name == "manifest.json" {
			if errors.Is(err, os.ErrNotExist) {
				return contents, fmt.Errorf("not a Notrios archive (missing manifest.json)")
			}
			return contents, err
		}
	}
	for _, dirName := range []string{"notes", "resources"} {
		info, err := root.Lstat(dirName)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return contents, err
		}
		if !info.IsDir() {
			return contents, fmt.Errorf("archive directory %s is not a directory", dirName)
		}
		dir, err := root.Open(dirName)
		if err != nil {
			return contents, err
		}
		for {
			entries, err := dir.ReadDir(archiveReadDirBatch)
			if err != nil && !errors.Is(err, io.EOF) {
				_ = dir.Close()
				return contents, err
			}
			for _, entry := range entries {
				scannedEntries++
				if scannedEntries > legacyArchiveLimits.MaxScannedEntries {
					_ = dir.Close()
					return contents, fmt.Errorf("archive contains more than %d directory entries", legacyArchiveLimits.MaxScannedEntries)
				}
				// Legacy imports ignore note sidecars and resource files without an
				// id__filename prefix; retain that compatibility during preflight.
				if dirName == "notes" && (entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md")) {
					continue
				}
				if dirName == "resources" && (entry.IsDir() || !validResourceEntryName(entry.Name())) {
					continue
				}
				candidate := dirName + "/" + entry.Name()
				if dirName == "resources" {
					f, info, err := openArchiveRegular(root, candidate, store.MaxResourceContentBytes)
					if err != nil {
						_ = dir.Close()
						return contents, err
					}
					contents.resources = append(contents.resources, entry.Name())
					if len(contents.resources) > legacyArchiveLimits.MaxResources {
						_ = f.Close()
						_ = dir.Close()
						return contents, fmt.Errorf("archive contains more than %d resources", legacyArchiveLimits.MaxResources)
					}
					if info.Size() > legacyArchiveLimits.MaxResourceBytes-resourceBytes {
						_ = f.Close()
						_ = dir.Close()
						return contents, fmt.Errorf("archive resources exceed %d bytes", legacyArchiveLimits.MaxResourceBytes)
					}
					resourceBytes += info.Size()
					if err := f.Close(); err != nil {
						_ = dir.Close()
						return contents, err
					}
					continue
				}
				if len(contents.noteNames) >= legacyArchiveLimits.MaxNotes {
					_ = dir.Close()
					return contents, fmt.Errorf("archive contains more than %d notes", legacyArchiveLimits.MaxNotes)
				}
				if raw, err := readArchiveFile(root, candidate, MaxArchiveFileBytes); err != nil {
					_ = dir.Close()
					return contents, err
				} else {
					contents.noteNames = append(contents.noteNames, entry.Name())
					noteBytes += int64(len(raw))
					if noteBytes > legacyArchiveLimits.MaxNoteBytes {
						_ = dir.Close()
						return contents, fmt.Errorf("archive notes exceed %d bytes", legacyArchiveLimits.MaxNoteBytes)
					}
				}
			}
			if len(entries) == 0 || errors.Is(err, io.EOF) {
				break
			}
		}
		if err := dir.Close(); err != nil {
			return contents, err
		}
	}
	return contents, nil
}

func validResourceEntryName(name string) bool {
	idx := strings.Index(name, "__")
	return idx > 0
}

// loadArchive reads notes and the notebook list from an archive directory.
// The explicitly selected archiveDir may itself be a symlink; OpenRoot pins
// that selected root while rejecting descendant symlinks at the archive
// boundary.
func loadArchive(archiveDir string) ([]archiveNote, []NotebookEntry, []string, error) {
	root, err := os.OpenRoot(archiveDir)
	if err != nil {
		return nil, nil, nil, err
	}
	defer root.Close()
	notes, notebooks, resources, err := loadArchiveRoot(root)
	return notes, notebooks, resources, err
}

// loadArchiveRoot keeps validation and consumption on one pinned directory
// capability. Import holds this same root through resource admission so a
// path replacement between inventory and use cannot select a different tree.
func loadArchiveRoot(root *os.Root) ([]archiveNote, []NotebookEntry, []string, error) {
	contents, err := validateArchiveDir(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifestRaw, err := readArchiveFile(root, "manifest.json", MaxArchiveFileBytes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("not a Notrios archive (missing manifest.json): %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil || manifest.Format != "notrios-archive" {
		return nil, nil, nil, errors.New("not a Notrios archive (bad manifest.json)")
	}

	notebooks := []NotebookEntry{}
	if raw, err := readArchiveFile(root, "notebooks.json", MaxArchiveFileBytes); err == nil {
		if err := json.Unmarshal(raw, &notebooks); err != nil {
			return nil, nil, nil, fmt.Errorf("notebooks.json: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil, err
	}
	notes := []archiveNote{}
	for _, name := range contents.noteNames {
		raw, err := readArchiveFile(root, "notes/"+name, MaxArchiveFileBytes)
		if err != nil {
			return nil, nil, nil, err
		}
		note, err := parseNoteFile(string(raw))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		if note.ID == "" {
			note.ID = strings.TrimSuffix(name, ".md")
		}
		notes = append(notes, note)
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	return notes, notebooks, contents.resources, nil
}

// parseNoteFile reads the flat YAML front matter this package writes.
func parseNoteFile(content string) (archiveNote, error) {
	note := archiveNote{}
	if !strings.HasPrefix(content, "---\n") {
		note.Body = content
		return note, nil
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return note, errors.New("unterminated front matter")
	}
	front := rest[:end]
	body := rest[end+4:]
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\n")
	note.Body = body

	currentList := ""
	for _, line := range strings.Split(front, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			value := unquote(strings.TrimSpace(trimmed[2:]))
			switch currentList {
			case "tags":
				note.Tags = append(note.Tags, value)
			case "resources":
				note.Resources = append(note.Resources, value)
			}
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "" {
			currentList = key
			continue
		}
		currentList = ""
		switch key {
		case "id":
			note.ID = unquote(value)
		case "title":
			note.Title = unquote(value)
		case "notebook":
			note.Notebook = unquote(value)
		}
	}
	return note, nil
}

func unquote(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		value = value[1 : len(value)-1]
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)
	}
	return value
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
