// Package linkreport reports every ambiguous link in a library, with the notes
// each one might mean (v1.0 J40-C).
//
// Lint already finds ambiguous links, and deliberately says nothing a person
// could act on: a finding is a note ID, a line and a digest of the target,
// because a link's text is often a private note's title (v0.5 E2). This report
// is the other half of that decision. It quotes the link as written and names
// every candidate by its title, so it is only ever given to the owner: as the
// command's output and as a read-only note in the Reports notebook, both on the
// owner's machine. REST carries the content-free form, and MCP does not offer
// it at all (decision 1).
//
// A link is ambiguous here for one of two reasons, and the report says which:
//
//   - **The library's.** The store's resolver found several notes with the
//     link's text as their title, or several attachments with it as their file
//     name, and the row says "ambiguous".
//   - **The import's.** An Obsidian import found several notes the name could
//     mean, by file name, title or alias, and left the link as written; the
//     store, which resolves by title alone, then found none and recorded the
//     link "unresolved". The candidates are rebuilt from provenance by the
//     importer's own code (J40-E).
//
// Nothing is guessed and nothing is repaired: choosing a candidate stays an
// ordinary edit by the owner.
package linkreport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/renesugar/notrios/internal/importers/obsidian"
	"github.com/renesugar/notrios/internal/stablelink"
	"github.com/renesugar/notrios/internal/store"
)

// Version is the report's shape. A change to what a field means changes it.
const Version = 1

// The caps follow lint's shape: a default, a ceiling and a flag, with counts
// that are always complete (decision 8). One name shared by 5,000 notes is a
// list nobody can read, which is what the candidate cap is for.
const (
	DefaultDetailLimit    = 50
	MaxDetailLimit        = 1000
	DefaultCandidateLimit = 10
	MaxCandidateLimit     = 100
)

// Why a link is ambiguous.
const (
	FoundByLibrary = "library"
	FoundByImport  = "import"
)

// What the candidates share with the link's text.
const (
	KindTitle      = "title"      // the library: notes with it as their title
	KindAttachment = "attachment" // the library: attachments with it as their file name
	KindName       = "name"       // the import: notes with it as file name, title or alias
)

// Source is what the report reads. *store.SQLiteStore is one.
type Source interface {
	LinkReportInputs(ctx context.Context, collectionID, sourceSystem string) (store.LinkReportInputs, error)
	LinkCandidates(ctx context.Context, collectionID, rawTarget string) ([]store.LinkCandidate, error)
	GetDocument(ctx context.Context, id string) (store.Document, error)
	GetDatabaseIdentity(ctx context.Context) (store.DatabaseIdentity, error)
}

// Request narrows and caps a report.
type Request struct {
	// CollectionID narrows the report to one collection; empty is every one.
	CollectionID string `json:"collection_id,omitempty"`
	// DetailLimit caps the notes shown, and the links shown per note.
	DetailLimit int `json:"detail_limit,omitempty"`
	// CandidateLimit caps the candidates shown per link.
	CandidateLimit int `json:"candidate_limit,omitempty"`
}

// Report is every ambiguous link in the library, with what the owner needs to
// choose between its candidates.
type Report struct {
	Version        int    `json:"version"`
	CollectionID   string `json:"collection_id"`
	DetailLimit    int    `json:"detail_limit"`
	CandidateLimit int    `json:"candidate_limit"`
	// The counts are complete whatever the caps hide.
	NotesWithAmbiguousLinks int `json:"notes_with_ambiguous_links"`
	AmbiguousLinks          int `json:"ambiguous_links"`
	FoundByLibraryCount     int `json:"found_by_library"`
	FoundByImportCount      int `json:"found_by_import"`
	// UnresolvedLinks counts the links that open nothing for any other reason.
	// They are lint's to locate, and this report only counts them (decision 2).
	UnresolvedLinks int `json:"unresolved_links"`
	// Notes are in title order, then by ID.
	Notes []Note `json:"notes"`
	// Truncated says DetailLimit hid notes. A note says so for its own links,
	// and a link for its own candidates.
	Truncated bool `json:"truncated"`
	// ReportSHA256 covers every finding, including the ones the caps hide, so
	// two reports of the same library agree whatever their caps.
	ReportSHA256 string   `json:"report_sha256"`
	Warnings     []string `json:"warnings"`
	ElapsedMS    float64  `json:"elapsed_ms"`
}

// Note is one note with an ambiguous link.
type Note struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	URI        string `json:"uri"`
	StableURI  string `json:"stable_uri,omitempty"`
	// LinkCount is complete; Links is capped by DetailLimit.
	LinkCount int    `json:"link_count"`
	Links     []Link `json:"links"`
	Truncated bool   `json:"truncated"`
}

// Link is one ambiguous link, located and quoted.
type Link struct {
	// Written is the link exactly as the note holds it.
	Written   string `json:"written"`
	RawTarget string `json:"raw_target"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	FoundBy   string `json:"found_by"`
	Kind      string `json:"kind"`
	// CandidateCount is complete; Candidates is capped by CandidateLimit.
	CandidateCount int         `json:"candidate_count"`
	Candidates     []Candidate `json:"candidates"`
	Truncated      bool        `json:"truncated"`
}

// Candidate is a note or attachment a link might mean.
type Candidate struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	URI       string `json:"uri"`
	StableURI string `json:"stable_uri,omitempty"`
}

// importNote is what the namespace was rebuilt from, kept to name candidates.
type importNote struct {
	title string
	uri   string
}

// finding is one ambiguous link before the caps.
type finding struct {
	row        store.LinkReportRow
	foundBy    string
	kind       string
	candidates candidateSet
}

// candidateSet is shared by every link that names the same thing, so what it
// costs to hash or to order is paid once per set rather than once per link.
type candidateSet struct {
	key string
	ids []string
	// digest identifies the set's members, for the report's digest.
	digest string
	named  func(limit int) ([]Candidate, error)
}

func setDigest(kind string, ids []string) string {
	sum := sha256.New()
	sum.Write([]byte(kind))
	for _, id := range ids {
		sum.Write([]byte{0})
		sum.Write([]byte(id))
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// Generate builds the report. It reads the whole library, like lint and the
// graph report, so it is run when asked and never on a timer or on save.
func Generate(ctx context.Context, src Source, req Request) (Report, error) {
	started := time.Now()
	req = normalize(req)
	inputs, err := src.LinkReportInputs(ctx, req.CollectionID, "obsidian")
	if err != nil {
		return Report{}, err
	}
	identity, err := src.GetDatabaseIdentity(ctx)
	if err != nil {
		return Report{}, err
	}
	stable := func(id string) string {
		if identity.DatabaseID == "" {
			return ""
		}
		return stablelink.Format(identity.DatabaseID, id)
	}

	// One namespace per collection: an import's namespace never spans two.
	byCollection := map[string][]obsidian.ProvenanceNote{}
	imported := map[string]importNote{}
	for _, note := range inputs.Notes {
		if provenance, ok := obsidian.ProvenanceNoteFrom(note.Source, note.Title); ok {
			byCollection[note.CollectionID] = append(byCollection[note.CollectionID], provenance)
			imported[note.DocumentID] = importNote{title: note.Title, uri: note.URI}
		}
	}
	namespaces := map[string]obsidian.Namespace{}
	for collection, notes := range byCollection {
		sort.Slice(notes, func(i, j int) bool { return notes[i].DocumentID < notes[j].DocumentID })
		namespaces[collection] = obsidian.NamespaceFromProvenance(notes)
	}

	report := Report{
		Version: Version, CollectionID: req.CollectionID,
		DetailLimit: req.DetailLimit, CandidateLimit: req.CandidateLimit,
		Notes: []Note{}, Warnings: []string{},
	}
	librarySets := map[string]candidateSet{}
	importSets := map[string]candidateSet{}
	findings := map[string][]finding{}
	for _, row := range inputs.Rows {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		switch {
		case row.Status == "ambiguous":
			key := row.CollectionID + "\x00" + strings.ToLower(strings.TrimSpace(row.RawTarget))
			set, ok := librarySets[key]
			if !ok {
				candidates, err := src.LinkCandidates(ctx, row.CollectionID, row.RawTarget)
				if err != nil {
					return Report{}, err
				}
				set = librarySet(key, candidates, stable)
				librarySets[key] = set
			}
			kind := KindTitle
			if set.key == "attachment" {
				kind = KindAttachment
			}
			findings[row.DocumentID] = append(findings[row.DocumentID], finding{row: row, foundBy: FoundByLibrary, kind: kind, candidates: set})
			report.FoundByLibraryCount++
		case row.Status == "unresolved":
			source, isImported := inputs.Notes[row.DocumentID]
			namespace, hasNamespace := namespaces[row.CollectionID]
			if isImported && hasNamespace && source.Source.SourceSystem == "obsidian" {
				if ambiguity, ok := namespace.Ambiguity(source.Source.ExternalID, row.RawTarget); ok {
					key := row.CollectionID + "\x00" + ambiguity.Name
					set, seen := importSets[key]
					if !seen {
						set = importSet(ambiguity, imported, stable)
						importSets[key] = set
					}
					findings[row.DocumentID] = append(findings[row.DocumentID], finding{row: row, foundBy: FoundByImport, kind: KindName, candidates: set})
					report.FoundByImportCount++
					continue
				}
			}
			report.UnresolvedLinks++
		default:
			report.UnresolvedLinks++
		}
	}
	report.AmbiguousLinks = report.FoundByLibraryCount + report.FoundByImportCount
	report.NotesWithAmbiguousLinks = len(findings)
	if report.FoundByImportCount > 0 {
		report.Warnings = append(report.Warnings, ImportNamespaceLimit)
	}

	ids := make([]string, 0, len(findings))
	for id := range findings {
		ids = append(ids, id)
	}
	title := func(id string) string { return inputs.Notes[id].Title }
	sort.Slice(ids, func(i, j int) bool { return titleOrder(title(ids[i]), ids[i], title(ids[j]), ids[j]) })

	digest := sha256.New()
	for _, id := range ids {
		for _, f := range findings[id] {
			fmt.Fprintf(digest, "%s\x00%d\x00%d\x00%s\x00%s\x00%s\x00%s\n", id, f.row.Line, f.row.Column,
				sha256Hex(f.row.RawTarget), f.foundBy, f.kind, f.candidates.digest)
		}
	}
	report.ReportSHA256 = hex.EncodeToString(digest.Sum(nil))

	for index, id := range ids {
		if index >= req.DetailLimit {
			report.Truncated = true
			break
		}
		note := inputs.Notes[id]
		document, err := src.GetDocument(ctx, id)
		if err != nil {
			return Report{}, err
		}
		entry := Note{DocumentID: id, Title: note.Title, URI: note.URI, StableURI: stable(id), LinkCount: len(findings[id]), Links: []Link{}}
		for linkIndex, f := range findings[id] {
			if linkIndex >= req.DetailLimit {
				entry.Truncated = true
				break
			}
			candidates, err := f.candidates.named(req.CandidateLimit)
			if err != nil {
				return Report{}, err
			}
			entry.Links = append(entry.Links, Link{
				Written:   written(document.Body, f.row),
				RawTarget: f.row.RawTarget, Line: f.row.Line, Column: f.row.Column,
				FoundBy: f.foundBy, Kind: f.kind,
				CandidateCount: len(f.candidates.ids), Candidates: candidates,
				Truncated: len(f.candidates.ids) > len(candidates),
			})
		}
		report.Notes = append(report.Notes, entry)
	}
	report.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
	return report, nil
}

// ImportNamespaceLimit is stated wherever an import's ambiguity is reported.
const ImportNamespaceLimit = "Candidates the Obsidian import found are rebuilt from each note's provenance, which " +
	"does not record which vault a note came from: two vaults imported into one collection are one namespace " +
	"here, so a list can include a note from the other vault."

func normalize(req Request) Request {
	req.CollectionID = strings.TrimSpace(req.CollectionID)
	req.DetailLimit = clamp(req.DetailLimit, DefaultDetailLimit, MaxDetailLimit)
	req.CandidateLimit = clamp(req.CandidateLimit, DefaultCandidateLimit, MaxCandidateLimit)
	return req
}

func clamp(value, fallback, ceiling int) int {
	if value <= 0 {
		return fallback
	}
	if value > ceiling {
		return ceiling
	}
	return value
}

// librarySet names the store's candidates, all of one kind.
func librarySet(key string, candidates []store.LinkCandidate, stable func(string) string) candidateSet {
	sort.Slice(candidates, func(i, j int) bool {
		return titleOrder(candidates[i].Title, candidates[i].ID, candidates[j].Title, candidates[j].ID)
	})
	set := candidateSet{key: "note", ids: make([]string, 0, len(candidates))}
	for _, candidate := range candidates {
		set.ids = append(set.ids, candidate.ID)
		set.key = candidate.Kind
	}
	set.digest = setDigest(set.key, set.ids)
	set.named = func(limit int) ([]Candidate, error) {
		out := []Candidate{}
		for _, candidate := range candidates {
			if len(out) >= limit {
				break
			}
			named := Candidate{ID: candidate.ID, Kind: candidate.Kind, Title: candidate.Title, URI: candidate.URI}
			if candidate.Kind == "note" {
				named.StableURI = stable(candidate.ID)
			}
			out = append(out, named)
		}
		return out, nil
	}
	return set
}

// importSet names the import's candidates. The list is the namespace's own and
// is not copied: the collision shape has one name shared by 5,000 notes and a
// link to it in every one of them.
func importSet(ambiguity obsidian.Ambiguity, notes map[string]importNote, stable func(string) string) candidateSet {
	set := candidateSet{key: "name", ids: ambiguity.Notes, digest: setDigest("name", ambiguity.Notes)}
	var ordered []string
	set.named = func(limit int) ([]Candidate, error) {
		if ordered == nil {
			ordered = append([]string(nil), ambiguity.Notes...)
			sort.Slice(ordered, func(i, j int) bool {
				return titleOrder(notes[ordered[i]].title, ordered[i], notes[ordered[j]].title, ordered[j])
			})
		}
		out := []Candidate{}
		for _, id := range ordered {
			if len(out) >= limit {
				break
			}
			out = append(out, Candidate{ID: id, Kind: "note", Title: notes[id].title, URI: notes[id].uri, StableURI: stable(id)})
		}
		return out, nil
	}
	return set
}

func titleOrder(leftTitle, leftID, rightTitle, rightID string) bool {
	left, right := strings.ToLower(leftTitle), strings.ToLower(rightTitle)
	if left != right {
		return left < right
	}
	return leftID < rightID
}

// written is the link exactly as the note holds it, or its target when the
// recorded span no longer fits the body.
func written(body string, row store.LinkReportRow) string {
	if row.StartByte < 0 || row.EndByte > len(body) || row.StartByte >= row.EndByte {
		return row.RawTarget
	}
	text := body[row.StartByte:row.EndByte]
	if !utf8.ValidString(text) {
		return row.RawTarget
	}
	return text
}

func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
