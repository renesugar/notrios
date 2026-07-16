package archive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// loadArchive reads notes and the notebook list from an archive directory.
func loadArchive(archiveDir string) ([]archiveNote, []NotebookEntry, error) {
	manifestRaw, err := os.ReadFile(filepath.Join(archiveDir, "manifest.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("not a Notrios archive (missing manifest.json): %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil || manifest.Format != "notrios-archive" {
		return nil, nil, errors.New("not a Notrios archive (bad manifest.json)")
	}

	notebooks := []NotebookEntry{}
	if raw, err := os.ReadFile(filepath.Join(archiveDir, "notebooks.json")); err == nil {
		if err := json.Unmarshal(raw, &notebooks); err != nil {
			return nil, nil, fmt.Errorf("notebooks.json: %w", err)
		}
	}

	notesDir := filepath.Join(archiveDir, "notes")
	entries, err := os.ReadDir(notesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, notebooks, nil
	}
	if err != nil {
		return nil, nil, err
	}
	notes := []archiveNote{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(notesDir, entry.Name()))
		if err != nil {
			return nil, nil, err
		}
		note, err := parseNoteFile(string(raw))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if note.ID == "" {
			note.ID = strings.TrimSuffix(entry.Name(), ".md")
		}
		notes = append(notes, note)
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	return notes, notebooks, nil
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
