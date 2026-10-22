package store

// What a report of ambiguous links reads from the library (v1.0 J40-C).
//
// The report itself is assembled in internal/linkreport, because it also asks
// the Obsidian importer's own code which notes a name could mean, and the store
// cannot import the importer. These are the store's half: the links that open
// nothing, the provenance an import left, and the resolver's own candidate
// lookups. All of it is read under the store's lock, one statement per call,
// and none of it is written.

// LinkReportRow is one link that opens nothing, in a note the report measures:
// not in the Trash and not in a read-only notebook, as lint measures.
type LinkReportRow struct {
	DocumentID   string
	CollectionID string
	RawTarget    string
	SourceFormat string
	RelationType string
	// Status is ambiguous, unresolved, invalid or target_deleted.
	Status    string
	StartByte int
	EndByte   int
	Line      int
	Column    int
}

// LinkReportNote is a note as the report names it, with its provenance when it
// has any.
type LinkReportNote struct {
	DocumentID   string
	CollectionID string
	Title        string
	URI          string
	Source       DocumentSource
}

// LinkReportInputs is everything the report reads in one go.
type LinkReportInputs struct {
	// Rows are in source order: by note, then by line and column.
	Rows []LinkReportRow
	// Notes holds the measured notes that carry a row, and every measured note
	// with provenance from SourceSystem, keyed by ID.
	Notes map[string]LinkReportNote
}

// LinkCandidate is a note or a resource a link's text could mean.
type LinkCandidate struct {
	ID    string
	Kind  string // "note" or "attachment"
	Title string // the note's title, or the resource's file name
	URI   string
}
