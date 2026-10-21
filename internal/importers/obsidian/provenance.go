package obsidian

import (
	"encoding/json"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// The importer's link namespace, rebuilt from what the library keeps (v1.0
// J40-E).
//
// An import leaves a link as written when its name could mean several notes,
// and the store then re-resolves that text by title alone, so a name the notes
// share only as file names or aliases is recorded "unresolved". Which notes the
// import saw is not lost, though: every imported note's provenance keeps its
// vault path and aliases, and its title is the note's. That is what
// buildLinkNamespace reads, so the namespace is rebuilt from it and asked the
// same question the import asked, through the same code. Nothing here is a
// second copy of the rules.
//
// Two things differ from the import, and a caller reports both:
//
//   - Provenance does not say which vault a note came from, so every Obsidian
//     note in a collection is one namespace here, where two vaults imported
//     into one collection were two to the import.
//   - A title is the note's current one. A note retitled in Notrios is found by
//     its new title, as the store would find it.

// ProvenanceNote is one imported note as the library records it.
type ProvenanceNote struct {
	DocumentID string
	// RelPath is the note's vault path, its provenance external ID.
	RelPath string
	Title   string
	Aliases []string
}

// ProvenanceNoteFrom reads a note's provenance as the import wrote it. It
// reports false for provenance that is not an Obsidian note's.
func ProvenanceNoteFrom(source store.DocumentSource, title string) (ProvenanceNote, bool) {
	if source.SourceSystem != sourceSystem || strings.TrimSpace(source.ExternalID) == "" {
		return ProvenanceNote{}, false
	}
	note := ProvenanceNote{DocumentID: source.DocumentID, RelPath: source.ExternalID, Title: title}
	var metadata struct {
		Aliases []string `json:"aliases"`
	}
	if source.MetadataJSON != "" && json.Unmarshal([]byte(source.MetadataJSON), &metadata) == nil {
		note.Aliases = metadata.Aliases
	}
	return note, true
}

// Namespace is the import's link namespace for a set of imported notes.
type Namespace struct {
	ns linkNamespace
}

// NamespaceFromProvenance builds the namespace the import would have built
// from these notes. Assets are left out: a resource keeps its file name, so the
// store already finds an attachment name ambiguous on its own.
func NamespaceFromProvenance(notes []ProvenanceNote) Namespace {
	inv := inventory{Notes: make([]vaultFile, 0, len(notes))}
	for _, note := range notes {
		inv.Notes = append(inv.Notes, vaultFile{RelPath: note.RelPath, Title: note.Title, Aliases: note.Aliases, TargetID: note.DocumentID})
	}
	return Namespace{ns: buildLinkNamespace(inv)}
}

// Ambiguity is a name the import found several notes for.
type Ambiguity struct {
	// Name is the name as the import looked it up: lower-cased, without its
	// folder or extension. Every link that names it has the same notes.
	Name string
	// Notes are in ID order and shared with the namespace, so a name a
	// thousand links use costs one list, not a thousand. A caller must not
	// modify it.
	Notes []string
}

// Ambiguity reports the notes a link written in the note at notePath could
// mean, when the import would have found it ambiguous. raw is the link's
// target as the extractor records it, without its anchor.
func (n Namespace) Ambiguity(notePath, raw string) (Ambiguity, bool) {
	raw = strings.TrimSpace(raw)
	if !vaultTarget(raw) || !looksLikeMarkdownNote(raw) {
		return Ambiguity{}, false
	}
	id, matches := noteMatches(notePath, raw, n.ns)
	if id != "" || len(matches) < 2 {
		return Ambiguity{}, false
	}
	return Ambiguity{Name: nameKey(decodeVaultTarget(raw)), Notes: matches}, true
}
