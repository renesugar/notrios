package archivev2

import (
	"encoding/json"
	"fmt"
	"strings"
)

// IndexMediaType marks an object-index chunk. Index chunks use the same
// LF-terminated JSONL discipline as record chunks.
const IndexMediaType = "application/vnd.notrios.archive-v2-index+jsonl"

// LayoutFanout stores one object as one regular file at a content-addressed
// path. It is the default and the only layout a reader is required to support.
const LayoutFanout = "fanout"

// LayoutPack stores many objects inside one large sequential file. It exists
// because measurement showed one file per object makes throughput track
// filesystem operations rather than bytes: a real 382,206-note backup needed
// 382,407 create+fsync+rename cycles to store 1.14 GB, and v0.7 sync would pay
// one transport round trip per object on top of that.
const LayoutPack = "pack"

// ObjectLocation says where an object's bytes live. Only the fields of the
// declared layout may be present, so the two layouts can never be confused.
// An object's SHA-256 identity never depends on its location.
type ObjectLocation struct {
	Layout     string `json:"layout"`
	Path       string `json:"path,omitempty"`
	PackSHA256 string `json:"pack_sha256,omitempty"`
	Offset     int64  `json:"offset,omitempty"`
	Length     int64  `json:"length,omitempty"`
}

// IndexEntry describes one immutable object. The set of entries replaces the
// inline `objects` array a v0.4 P3 manifest carried: that array cost roughly
// 645 bytes per object inside a 4 MiB manifest, which capped an archive near
// 6,500 objects and made a real library unarchivable.
type IndexEntry struct {
	SHA256       string         `json:"sha256"`
	Kind         string         `json:"kind"`
	MediaType    string         `json:"media_type"`
	SizeBytes    int64          `json:"size_bytes"`
	Records      int            `json:"records,omitempty"`
	RecordCounts Counts         `json:"record_counts,omitempty"`
	Location     ObjectLocation `json:"location"`
}

// IndexObject is the manifest-level descriptor of one index chunk. Index
// chunks are the only objects still listed inline, and there are at most
// MaxObjects/MaxIndexEntriesPerObject of them.
type IndexObject struct {
	SHA256    string         `json:"sha256"`
	SizeBytes int64          `json:"size_bytes"`
	Entries   int            `json:"entries"`
	MediaType string         `json:"media_type"`
	Location  ObjectLocation `json:"location"`
}

// ObjectTotals are the aggregate object facts the manifest still states
// directly so a reader can bound its work before opening any index chunk.
type ObjectTotals struct {
	Objects      int   `json:"objects"`
	Bytes        int64 `json:"bytes"`
	RecordChunks int   `json:"record_chunks"`
	Blobs        int   `json:"blobs"`
	Packs        int   `json:"packs,omitempty"`
}

// fanoutObjectPath places an object under a two-level fanout. One level would
// put every object of a million-note archive into 256 directories of tens of
// thousands of entries; two levels keeps them near 25 and matches the layout
// the canonical asset store already uses.
func fanoutObjectPath(hash string) string {
	return "objects/sha256/" + hash[:2] + "/" + hash[2:4] + "/" + hash
}

func newFanoutLocation(hash string) ObjectLocation {
	return ObjectLocation{Layout: LayoutFanout, Path: fanoutObjectPath(hash)}
}

func (location ObjectLocation) validate(hash string, limits Limits) error {
	switch location.Layout {
	case LayoutFanout:
		if location.PackSHA256 != "" || location.Offset != 0 || location.Length != 0 {
			return fmt.Errorf("fanout object %s declares pack fields", hash)
		}
		if location.Path != fanoutObjectPath(hash) || len(location.Path) > limits.MaxPathBytes {
			return fmt.Errorf("object path %q does not match its content hash", location.Path)
		}
		return nil
	case LayoutPack:
		if location.Path != "" {
			return fmt.Errorf("packed object %s declares its own path", hash)
		}
		if !validSHA256(location.PackSHA256) {
			return fmt.Errorf("packed object %s names an invalid pack", hash)
		}
		if location.PackSHA256 == hash {
			return fmt.Errorf("pack %s cannot contain itself", hash)
		}
		if location.Offset < 0 || location.Length < 0 || location.Offset > limits.MaxBlobBytes || location.Length > limits.MaxBlobBytes {
			return fmt.Errorf("packed object %s has an out-of-range extent", hash)
		}
		return nil
	default:
		return fmt.Errorf("unsupported object layout %q", location.Layout)
	}
}

func (entry IndexEntry) validate(limits Limits) error {
	if !validSHA256(entry.SHA256) {
		return fmt.Errorf("index entry hash %q is not a lowercase SHA-256", entry.SHA256)
	}
	if err := entry.Location.validate(entry.SHA256, limits); err != nil {
		return err
	}
	if entry.SizeBytes < 0 || entry.SizeBytes > limits.MaxBlobBytes {
		return fmt.Errorf("index entry %s declares an out-of-range size", entry.SHA256)
	}
	switch entry.Kind {
	case "pack":
		// A pack container holds other objects; it carries no records and is
		// always stored as its own file.
		if entry.Location.Layout != LayoutFanout || entry.MediaType != PackMediaType ||
			entry.Records != 0 || entry.RecordCounts.Total() != 0 || entry.SizeBytes < packFooterBytes {
			return fmt.Errorf("invalid pack index entry %s", entry.SHA256)
		}
		return nil
	case "records":
		if entry.Location.Layout == LayoutPack && entry.Location.Length != entry.SizeBytes {
			return fmt.Errorf("packed records object %s length disagrees with its size", entry.SHA256)
		}
		if entry.MediaType != RecordsMediaType || entry.SizeBytes > limits.MaxRecordObjectBytes ||
			entry.Records <= 0 || entry.Records > limits.MaxRecordsPerObject || entry.RecordCounts.Total() != entry.Records {
			return fmt.Errorf("invalid records index entry %s", entry.SHA256)
		}
		return validateCounts(entry.RecordCounts, limits.MaxRecordsPerObject)
	case "blob":
		if entry.Records != 0 || entry.RecordCounts.Total() != 0 {
			return fmt.Errorf("blob index entry %s declares records", entry.SHA256)
		}
		if entry.Location.Layout == LayoutPack && entry.Location.Length != entry.SizeBytes {
			return fmt.Errorf("packed blob %s length disagrees with its size", entry.SHA256)
		}
		if _, err := normalizeMediaType(entry.MediaType); err != nil {
			return fmt.Errorf("blob index entry %s: %w", entry.SHA256, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported object kind %q", entry.Kind)
	}
}

func (object IndexObject) validate(limits Limits) error {
	if !validSHA256(object.SHA256) {
		return fmt.Errorf("index object hash %q is not a lowercase SHA-256", object.SHA256)
	}
	if err := object.Location.validate(object.SHA256, limits); err != nil {
		return err
	}
	if object.MediaType != IndexMediaType {
		return fmt.Errorf("index object %s has media type %q", object.SHA256, object.MediaType)
	}
	if object.Entries <= 0 || object.Entries > limits.MaxIndexEntriesPerObject {
		return fmt.Errorf("index object %s declares %d entries", object.SHA256, object.Entries)
	}
	if object.SizeBytes <= 0 || object.SizeBytes > limits.MaxIndexObjectBytes {
		return fmt.Errorf("index object %s declares an out-of-range size", object.SHA256)
	}
	return nil
}

// encodeIndexEntry renders one entry as a single JSONL line.
func encodeIndexEntry(entry IndexEntry) ([]byte, error) {
	raw, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// spoolIndexEntry encodes one entry for the writer's external sort. The hash
// leads so bucket selection and ordering follow the object hash, and the JSON
// payload rides along after a tab that JSON can never contain unescaped.
func spoolIndexEntry(entry IndexEntry) (string, error) {
	raw, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return entry.SHA256 + "\t" + string(raw), nil
}

func parseSpooledIndexEntry(line string) (IndexEntry, error) {
	hash, payload, ok := strings.Cut(line, "\t")
	if !ok {
		return IndexEntry{}, fmt.Errorf("malformed spooled index entry")
	}
	var entry IndexEntry
	if err := json.Unmarshal([]byte(payload), &entry); err != nil {
		return IndexEntry{}, err
	}
	if entry.SHA256 != hash {
		return IndexEntry{}, fmt.Errorf("spooled index entry hash mismatch")
	}
	return entry, nil
}
