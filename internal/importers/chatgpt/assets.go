package chatgpt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/renesugar/notrios/internal/importers/archivesource"
	"github.com/renesugar/notrios/internal/store"
)

// J26-C: the Privacy Portal strips an asset's name and extension, leaving
// file-<id>.dat. Two records in the same archive put them back, and the bytes
// themselves settle what is left:
//
//  1. conversation_asset_file_names.json maps the .dat name to the real one.
//  2. library_files.json records each file's name, extension and MIME type
//     against its file ID.
//  3. Otherwise the file's first bytes are sniffed for its type.
//
// The direct ChatGPT export needs none of this: it keeps the real name in the
// archive name, as file-<id>-<name>.<ext> or file_<hash>-<name>.<ext>.

// librarySpec opens the Privacy Portal's Files ZIP, whose entries are the file
// library itself rather than any named data file.
func librarySpec() archivesource.Spec {
	return archivesource.Spec{
		IsDataFile:    func(string) bool { return true },
		DirCandidates: []string{"personal/files", "personal", ""},
		NotAnArchive: func(name string, err error) error {
			return fmt.Errorf("%s is not a readable file-library ZIP: %w", name, err)
		},
		NoDataDirectory: func(where, kind string) error {
			return fmt.Errorf("no files in %s", where)
		},
	}
}

// libraryRecord is one row of library_files.json, reduced to what naming needs.
type libraryRecord struct {
	FileID    string `json:"file_id"`
	FileName  string `json:"file_name"`
	Extension string `json:"file_extension"`
	MIMEType  string `json:"mime_type"`
}

// assetIndex knows where an asset's bytes are in the archive, and what it is
// called.
type assetIndex struct {
	source archivesource.Source
	// byID maps a file ID to the archive name holding its bytes.
	byID map[string]string
	// datNames maps an archive name to the real name the asset map gave it.
	datNames map[string]string
	// library maps a file ID to its library record.
	library map[string]libraryRecord
	// imported caches the resource created for a file ID within one run.
	imported map[string]string
}

// newAssetIndex indexes the archive's asset files and reads whichever naming
// records it carries.
func newAssetIndex(opened *export, report *Report) (*assetIndex, error) {
	index := &assetIndex{
		source:   opened.conversations,
		byID:     map[string]string{},
		datNames: map[string]string{},
		library:  map[string]libraryRecord{},
		imported: map[string]string{},
	}
	names, err := opened.conversations.All()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		base := path.Base(name)
		if id := assetID(base); id != "" && !conversationsFileRE.MatchString(base) {
			// The first name wins, so a stable archive gives a stable import.
			if _, seen := index.byID[id]; !seen {
				index.byID[id] = name
			}
		}
	}

	if archivesource.HasName(names, "conversation_asset_file_names.json") {
		file, err := opened.conversations.Open("conversation_asset_file_names.json", archivesource.Limits.DataFileBytes)
		if err != nil {
			return nil, err
		}
		var mapping map[string]string
		err = archivesource.DecodeObject(file, "conversation_asset_file_names.json", &mapping)
		file.Close()
		if err != nil {
			return nil, err
		}
		for datName, realName := range mapping {
			index.datNames[datName] = realName
		}
	}

	if archivesource.HasName(names, "library_files.json") {
		file, err := opened.conversations.Open("library_files.json", archivesource.Limits.DataFileBytes)
		if err != nil {
			return nil, err
		}
		_, err = archivesource.DecodeArray(file, "library_files.json", func(record libraryRecord) error {
			if strings.TrimSpace(record.FileID) != "" {
				index.library[record.FileID] = record
			}
			return nil
		})
		file.Close()
		if err != nil {
			// A library record that cannot be read costs names, not the
			// import: the asset map and sniffing still apply.
			report.Warnings = append(report.Warnings, fmt.Sprintf("library_files.json: %v", err))
		}
	}
	return index, nil
}

// name returns an asset's real name and MIME type, and where they came from.
func (a *assetIndex) name(id, archiveName string) (string, string, string) {
	base := path.Base(archiveName)
	if real, ok := a.datNames[base]; ok && strings.TrimSpace(real) != "" {
		return real, store.MIMETypeFromFilename(real), "map"
	}
	if record, ok := a.library[id]; ok {
		name := strings.TrimSpace(record.FileName)
		if name == "" {
			name = id
		}
		if path.Ext(name) == "" && strings.TrimSpace(record.Extension) != "" {
			name += "." + strings.TrimPrefix(record.Extension, ".")
		}
		mime := strings.TrimSpace(record.MIMEType)
		if mime == "" {
			mime = store.MIMETypeFromFilename(name)
		}
		return name, mime, "library"
	}
	if strings.EqualFold(path.Ext(base), ".dat") {
		// Nothing names it; the bytes will.
		return strings.TrimSuffix(base, path.Ext(base)), "", "sniff"
	}
	return base, store.MIMETypeFromFilename(base), "archive"
}

// importAsset imports the bytes behind a file ID, returning the resource ID and
// the name to show. An asset the archive does not carry returns "".
func (a *assetIndex) importAsset(ctx context.Context, st store.Store, id, collectionID string, report *Report) (string, string, error) {
	archiveName, ok := a.byID[id]
	if !ok {
		return "", "", nil
	}
	display, mime, from := a.name(id, archiveName)
	if resourceID, done := a.imported[id]; done {
		return resourceID, display, nil
	}
	resourceID, resolvedName, err := importOne(ctx, st, a.source, archiveName, display, mime, collectionID, report)
	if err != nil {
		return "", display, err
	}
	switch from {
	case "map":
		report.AssetNamesFromMap++
	case "library":
		report.AssetNamesFromLibrary++
	case "sniff":
		report.AssetNamesSniffed++
	}
	report.AssetsImported++
	a.imported[id] = resourceID
	return resourceID, resolvedName, nil
}

// importOne streams one file from the archive into the asset store. When no
// record named it, its type is sniffed from its first bytes and its name gains
// the matching extension.
func importOne(ctx context.Context, st store.Store, source archivesource.Source, archiveName, display, mime, collectionID string, report *Report) (string, string, error) {
	resourceID := "res_chatgpt_" + sanitizeID(archiveName)
	if existing, err := st.GetResource(ctx, resourceID); err == nil {
		// The name the first import settled on is the name every later import
		// uses: an asset whose extension came from sniffing its bytes would
		// otherwise be named differently the second time, and the note would
		// be rewritten for no reason.
		if strings.TrimSpace(existing.Filename) != "" {
			display = existing.Filename
		}
		return resourceID, display, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", display, err
	}
	file, err := source.Open(archiveName, archivesource.Limits.MediaFileBytes)
	if err != nil {
		return "", display, err
	}
	defer file.Close()

	var content io.Reader = file
	if strings.TrimSpace(mime) == "" {
		sniffed, rest, err := sniffMIME(file)
		if err != nil {
			return "", display, err
		}
		mime, content = sniffed, rest
		if path.Ext(display) == "" {
			if extension := extensionFor(sniffed); extension != "" {
				display += extension
			}
		}
	}
	if strings.TrimSpace(mime) == "" {
		mime = "application/octet-stream"
	}
	resource, err := st.CreateResource(ctx, store.CreateResourceRequest{
		PreferredID: resourceID, CollectionID: collectionID,
		Filename: display, MIMEType: mime, Content: content,
	})
	if err != nil {
		return "", display, err
	}
	return resource.ID, display, nil
}

// extensionFor names the file extension for the types an export actually
// carries. An unknown type keeps no extension rather than a guessed one.
func extensionFor(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg":
		return ".jpeg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "text/plain", "text/plain; charset=utf-8":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "application/zip":
		return ".zip"
	case "audio/wave", "audio/wav":
		return ".wav"
	default:
		return ""
	}
}
