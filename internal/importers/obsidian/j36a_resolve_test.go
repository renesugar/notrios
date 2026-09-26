package obsidian

import "testing"

// j36Namespace is the vault the J36 table is stated against: two notes sharing
// the base name "index" in different folders, a name shared between the vault
// root and a folder, and a note whose base name is unique in the vault.
//
//	area-01/topic-00001/index.md
//	area-02/topic-00002/index.md
//	note.md
//	folder/note.md
//	only/deep/unique.md
func j36Namespace() linkNamespace {
	return linkNamespace{
		notesByPath: map[string]string{
			"area-01/topic-00001/index": "doc-a1",
			"area-02/topic-00002/index": "doc-a2",
			"note":                      "doc-root",
			"folder/note":               "doc-folder",
			"only/deep/unique":          "doc-unique",
		},
		notesByName: map[string][]string{
			"index":  {"doc-a1", "doc-a2"},
			"note":   {"doc-folder", "doc-root"},
			"unique": {"doc-unique"},
		},
		assetsByPath: map[string]string{
			"only/deep/picture.png": "res-unique",
			"img/logo.png":          "res-root",
			"folder/img/logo.png":   "res-folder",
		},
		assetsByBase: map[string][]string{
			"picture.png": {"res-unique"},
			"logo.png":    {"res-folder", "res-root"},
		},
	}
}

// TestJ36ANoteResolutionAsItIs pins what resolveNoteID does, case by case, with
// what Obsidian does for the same link beside it. After J36-B and J36-D the two
// agree everywhere except the row marked J36-E. Obsidian's rule is that
// "Folder paths start at the vault root and use forward slashes", so a link
// "Folder paths start at the vault root and use forward slashes", so a link
// containing a slash names a path and nothing else.
func TestJ36ANoteResolutionAsItIs(t *testing.T) {
	ns := j36Namespace()
	cases := []struct {
		name      string
		from      string
		raw       string
		wantID    string
		ambiguous bool
		obsidian  string
	}{
		{
			name:     "exact vault path resolves",
			from:     "area-01/topic-00001/index.md",
			raw:      "area-02/topic-00002/index",
			wantID:   "doc-a2",
			obsidian: "same: the path exists from the vault root",
		},
		{
			name:     "note-relative path resolves",
			from:     "folder/other.md",
			raw:      "folder/note",
			wantID:   "doc-folder",
			obsidian: "same: the path exists from the vault root",
		},
		{
			// The link this item was opened for. J36-B stopped calling it
			// ambiguous: it is simply a path that does not exist.
			name:     "partial path is not a path and not a name",
			from:     "area-01/topic-00001/index.md",
			raw:      "topic-00002/index",
			wantID:   "",
			obsidian: "same: no such path, and no suffix matching",
		},
		{
			// The defect J36-A found, fixed by J36-B: before the fix this
			// resolved to only/deep/unique.md, a note the link never named.
			name:     "path that matches nothing is unresolved",
			from:     "area-01/topic-00001/index.md",
			raw:      "wrong/path/unique",
			wantID:   "",
			obsidian: "same: there is no wrong/path/unique",
		},
		{
			// J36-D: before the fix this picked folder/note.md, so [[note]]
			// meant a different note depending on where it was written.
			name:     "bare shared name prefers the vault root",
			from:     "folder/other.md",
			raw:      "note",
			wantID:   "doc-root",
			obsidian: "same: the vault-root path is preferred",
		},
		{
			// J36-D also rescues this one: the root path now answers a name
			// that the name map could only call ambiguous.
			name:     "bare shared name reaches the root from elsewhere",
			from:     "elsewhere/other.md",
			raw:      "note",
			wantID:   "doc-root",
			obsidian: "same",
		},
		{
			name:     "bare name resolves relative before the ambiguous name map",
			from:     "area-01/topic-00001/index.md",
			raw:      "index",
			wantID:   "doc-a1",
			obsidian: "same: area-01/topic-00001/index.md is in the same folder",
		},
		{
			// The residual divergence, recorded as J36-E: with no note of this
			// name at the vault root, Obsidian still picks one of the two by
			// its own index order, which this importer cannot reproduce, so it
			// reports the ambiguity instead of guessing.
			name:      "bare shared name with no root or relative match is ambiguous",
			from:      "elsewhere/other.md",
			raw:       "index",
			wantID:    "",
			ambiguous: true,
			obsidian:  "resolves: Obsidian picks one rather than refusing (J36-E)",
		},
		{
			name:     "unique bare name resolves from anywhere",
			from:     "elsewhere/other.md",
			raw:      "unique",
			wantID:   "doc-unique",
			obsidian: "same",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			id, ambiguous := resolveNoteID(testCase.from, testCase.raw, ns)
			if id != testCase.wantID || ambiguous != testCase.ambiguous {
				t.Fatalf("resolveNoteID(%q, %q) = (%q, %v), want (%q, %v); Obsidian: %s",
					testCase.from, testCase.raw, id, ambiguous, testCase.wantID, testCase.ambiguous, testCase.obsidian)
			}
		})
	}
}

// TestJ36AAssetResolutionAsItIs pins the same questions for attachments, which
// resolve through their own maps but with the same shape.
func TestJ36AAssetResolutionAsItIs(t *testing.T) {
	ns := j36Namespace()
	cases := []struct {
		name      string
		from      string
		raw       string
		wantID    string
		ambiguous bool
		obsidian  string
	}{
		{
			name:     "exact vault path resolves",
			from:     "note.md",
			raw:      "only/deep/picture.png",
			wantID:   "res-unique",
			obsidian: "same",
		},
		{
			// J36-D: before the fix the note-relative folder/img/logo.png won.
			name:     "path present at the root and relative prefers the root",
			from:     "folder/other.md",
			raw:      "img/logo.png",
			wantID:   "res-root",
			obsidian: "same: the vault-root path is preferred",
		},
		{
			name:     "note-relative path resolves when the root has no match",
			from:     "folder/other.md",
			raw:      "only/deep/picture.png",
			wantID:   "res-unique",
			obsidian: "same",
		},
		{
			// J36-B: before the fix this resolved to only/deep/picture.png.
			name:     "path that matches nothing is unresolved",
			from:     "note.md",
			raw:      "wrong/path/picture.png",
			wantID:   "",
			obsidian: "same: there is no wrong/path/picture.png",
		},
		{
			// J36-E again, for attachments.
			name:      "shared file name with no path match is ambiguous",
			from:      "elsewhere/other.md",
			raw:       "logo.png",
			wantID:    "",
			ambiguous: true,
			obsidian:  "resolves: Obsidian picks one rather than refusing (J36-E)",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			id, ambiguous := resolveAssetID(testCase.from, testCase.raw, ns)
			if id != testCase.wantID || ambiguous != testCase.ambiguous {
				t.Fatalf("resolveAssetID(%q, %q) = (%q, %v), want (%q, %v); Obsidian: %s",
					testCase.from, testCase.raw, id, ambiguous, testCase.wantID, testCase.ambiguous, testCase.obsidian)
			}
		})
	}
}
