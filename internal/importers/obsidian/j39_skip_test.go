package obsidian

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// j39Vault is a small vault that has every kind of dependency a skip has to
// respect: links by root path, by relative path and by name, an alias, an
// embedded asset used by two notes, a folder, and a note with no links.
//
//	index.md      [[alpha]], ![[pic.png]], [b](sub/beta.md)
//	alpha.md      alias "first", [[Beta]] by name
//	sub/beta.md   [[index]]
//	gamma.md      ![](pic.png), [[alpha#Heading|shown]]
//	plain.md      nothing
//	pic.png
func j39Vault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.md"), "# Index\n\nSee [[alpha]] and ![[pic.png]] and [b](sub/beta.md).\n")
	writeFile(t, filepath.Join(dir, "alpha.md"), "---\naliases: [first]\n---\n# Alpha\n\n## Heading\n\nOn to [[Beta]].\n")
	writeFile(t, filepath.Join(dir, "sub", "beta.md"), "# Beta\n\nBack to [[index]].\n")
	writeFile(t, filepath.Join(dir, "gamma.md"), "# Gamma\n\nA picture ![](pic.png) and [[alpha#Heading|shown]].\n")
	writeFile(t, filepath.Join(dir, "plain.md"), "# Plain\n\nNothing links from here.\n")
	writeBytes(t, filepath.Join(dir, "pic.png"), []byte("\x89PNG\r\n\x1a\nj39"))
	return dir
}

var j39Notes = []string{"alpha.md", "gamma.md", "index.md", "plain.md", "sub/beta.md"}

func j39Import(t *testing.T, st store.Store, dir string, options Options) Report {
	t.Helper()
	if options.CollectionID == "" {
		options.CollectionID = "default"
	}
	report, err := Import(context.Background(), st, dir, options)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return report
}

func j39Revisions(t *testing.T, st store.Store) map[string]int {
	t.Helper()
	ids := make([]string, 0, len(j39Notes))
	for _, note := range j39Notes {
		ids = append(ids, documentID(note))
	}
	return j36cRevisionCounts(t, context.Background(), st, ids...)
}

func j39Refs(t *testing.T, st store.Store, relPath string) []string {
	t.Helper()
	refs, err := st.ListDocumentResources(context.Background(), documentID(relPath))
	if err != nil {
		t.Fatalf("list resources of %s: %v", relPath, err)
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.ResourceID+" "+ref.RelationType+" "+ref.AnchorJSON)
	}
	sort.Strings(out)
	return out
}

// TestJ39ANoOpReimportSkipsEveryNote: a reimport of a vault where nothing moved
// reads no note, and still runs every phase after the notes. The link pass
// refreshes every note, the checkpoint completes, and no note gains a revision.
func TestJ39ANoOpReimportSkipsEveryNote(t *testing.T) {
	dir := j39Vault(t)
	st := openTestStore(t)
	first := j39Import(t, st, dir, Options{})
	if first.NotesImported != 5 || first.NotesSkipped != 0 {
		t.Fatalf("a first import reads every note: %#v", first)
	}
	before := j39Revisions(t, st)

	report := j39Import(t, st, dir, Options{})
	if report.NotesSkipped != 5 || report.NotesUnchanged != 5 || report.NotesUpdated != 0 || report.NotesImported != 0 {
		t.Fatalf("a no-op reimport should skip every note: %#v", report)
	}
	if report.LinkIndexesRefreshed != 5 || report.CheckpointStatus != "completed" {
		t.Fatalf("a reimport that skips every note still runs the link pass and completes: %#v", report)
	}
	if after := j39Revisions(t, st); !reflect.DeepEqual(after, before) {
		t.Fatalf("revisions moved on a skipped reimport: before %v, after %v", before, after)
	}
	if got := j39Refs(t, st, "index.md"); len(got) != 1 || !strings.HasPrefix(got[0], resourceID("pic.png")+" embed ") {
		t.Fatalf("a skipped note keeps its references: %v", got)
	}
}

// TestJ39AStateFromAnOlderBuildIsNotSkipped: a library written before J39 holds
// the bare note fingerprint. Its first reimport reads every note and records the
// new form, and only the reimport after that skips. That is the safe direction.
func TestJ39AStateFromAnOlderBuildIsNotSkipped(t *testing.T) {
	dir := j39Vault(t)
	st := openTestStore(t)
	j39Import(t, st, dir, Options{})

	// Put back exactly what a pre-J39 build recorded: noteFingerprint over the
	// file's hash, the notebook and the canonical body's hash.
	j39RewriteStates(t, st, dir, func(item vaultFile, head store.ImportDocumentHead, _ string) string {
		return noteFingerprint(item, head.NotebookID, head.ContentSHA256)
	})
	before := j39Revisions(t, st)

	for relPath, prefixed := range j39StatePrefixes(t, st, dir) {
		if prefixed {
			t.Fatalf("%s: a pre-J39 state is not in the skip form", relPath)
		}
	}
	report := j39Import(t, st, dir, Options{})
	if report.NotesSkipped != 0 || report.NotesUnchanged != 5 {
		t.Fatalf("a pre-J39 state must not be skipped: %#v", report)
	}
	if after := j39Revisions(t, st); !reflect.DeepEqual(after, before) {
		t.Fatalf("reading every note again must not revise any: before %v, after %v", before, after)
	}
	if again := j39Import(t, st, dir, Options{}); again.NotesSkipped != 5 {
		t.Fatalf("the import after that should skip every note: %#v", again)
	}
}

// j39RewriteStates replaces every note's recorded fingerprint, as a library
// written by another build would hold it. fingerprint receives the note as the
// inventory sees it, the store's head for it, and this run's basis.
func j39RewriteStates(t *testing.T, st store.Store, dir string, fingerprint func(item vaultFile, head store.ImportDocumentHead, basis string) string) {
	t.Helper()
	ctx := context.Background()
	run, err := newImportRun(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	keys, ids := []string{}, []string{}
	for _, item := range run.inventory.Notes {
		keys = append(keys, item.itemKey())
		ids = append(ids, item.TargetID)
	}
	states, err := st.GetImportItemStates(ctx, sourceSystem, run.report.SourceKey, "default", keys)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := st.ImportDocumentHeads(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := []store.ImportItemState{}
	for _, item := range run.inventory.Notes {
		state, ok := states[item.itemKey()]
		if !ok {
			t.Fatalf("no state for %s", item.RelPath)
		}
		state.Fingerprint = fingerprint(item, heads[item.TargetID], run.basis)
		rewritten = append(rewritten, state)
	}
	if err := st.PutImportItemStates(ctx, rewritten); err != nil {
		t.Fatal(err)
	}
}

// TestJ39BEveryDivergenceIsRead is J39-B's list of repair cases. In each one
// the notes that diverged are read and repaired exactly as they were before
// skips existed, and the others are skipped.
func TestJ39BEveryDivergenceIsRead(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		// change is applied after the first import, to the vault or the library.
		change func(t *testing.T, st *store.SQLiteStore, dir string)
		// skipped and updated are the reimport's counts.
		skipped, updated int
		check            func(t *testing.T, st *store.SQLiteStore, report Report)
	}{
		{
			name: "edited inside Notrios",
			change: func(t *testing.T, st *store.SQLiteStore, _ string) {
				doc, err := st.GetDocument(ctx, documentID("alpha.md"))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
					ID: doc.ID, Title: doc.Title, Body: doc.Body + "\nAn edit made in Notrios.\n",
					BodyMIMEType: doc.BodyMIMEType, BaseRevisionID: doc.CurrentRevisionID,
				}); err != nil {
					t.Fatal(err)
				}
			},
			skipped: 4, updated: 1,
			check: func(t *testing.T, st *store.SQLiteStore, _ Report) {
				doc, err := st.GetDocument(ctx, documentID("alpha.md"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(doc.Body, "An edit made in Notrios.") {
					t.Fatalf("the reimport did not put the vault's note back:\n%s", doc.Body)
				}
			},
		},
		{
			name: "a resource detached",
			change: func(t *testing.T, st *store.SQLiteStore, _ string) {
				if err := st.DetachDocumentResource(ctx, documentID("index.md"), resourceID("pic.png")); err != nil {
					t.Fatal(err)
				}
			},
			skipped: 4,
			check: func(t *testing.T, st *store.SQLiteStore, _ Report) {
				if got := j39Refs(t, st, "index.md"); len(got) != 1 {
					t.Fatalf("the reimport did not attach the picture again: %v", got)
				}
			},
		},
		{
			name: "a resource purged",
			change: func(t *testing.T, st *store.SQLiteStore, _ string) {
				// What a peer's deletion leaves (applySyncResourceRecordsLocked):
				// no references and no resource row. The link rows are let go
				// first, because DeleteResource refuses a resource a note still
				// links to and the foreign key would refuse it here too.
				id := resourceID("pic.png")
				for _, statement := range []string{
					`DELETE FROM document_resource_refs WHERE resource_id = '` + id + `'`,
					`UPDATE document_links SET target_resource_id = NULL WHERE target_resource_id = '` + id + `'`,
					`DELETE FROM resources WHERE id = '` + id + `'`,
				} {
					if err := st.Exec(ctx, statement); err != nil {
						t.Fatal(err)
					}
				}
			},
			skipped: 3,
			check: func(t *testing.T, st *store.SQLiteStore, report Report) {
				if report.ResourcesImported != 1 {
					t.Fatalf("the picture should be imported again: %#v", report)
				}
				for _, note := range []string{"index.md", "gamma.md"} {
					if got := j39Refs(t, st, note); len(got) != 1 {
						t.Fatalf("%s did not get the recreated picture back: %v", note, got)
					}
				}
			},
		},
		{
			name: "moved to another notebook",
			change: func(t *testing.T, st *store.SQLiteStore, _ string) {
				notebook, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Elsewhere"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := st.MoveDocumentToNotebook(ctx, documentID("plain.md"), notebook.ID); err != nil {
					t.Fatal(err)
				}
			},
			skipped: 4, updated: 1,
			check: func(t *testing.T, st *store.SQLiteStore, _ Report) {
				doc, err := st.GetDocument(ctx, documentID("plain.md"))
				if err != nil {
					t.Fatal(err)
				}
				if doc.NotebookID != store.DefaultNotebookID {
					t.Fatalf("the reimport did not move the note back: %q", doc.NotebookID)
				}
			},
		},
		{
			name: "the rule version changed",
			change: func(t *testing.T, st *store.SQLiteStore, dir string) {
				j39RewriteStates(t, st, dir, func(item vaultFile, head store.ImportDocumentHead, _ string) string {
					run, err := newImportRun(ctx, st, dir, Options{CollectionID: "default"})
					if err != nil {
						t.Fatal(err)
					}
					other := canonicalBasis(canonicalRules+1, "default", run.namespace)
					return recordedFingerprint(item, head.NotebookID, head.ContentSHA256, other, head.ResourceRefsSHA256)
				})
			},
			skipped: 0,
		},
		{
			name: "a new note makes a name ambiguous",
			change: func(t *testing.T, _ *store.SQLiteStore, dir string) {
				writeFile(t, filepath.Join(dir, "other", "beta.md"), "# Beta\n\nA second Beta.\n")
			},
			skipped: 0, updated: 1,
			check: func(t *testing.T, st *store.SQLiteStore, report Report) {
				if report.NotesImported != 1 {
					t.Fatalf("the new note should be imported: %#v", report)
				}
				if !strings.Contains(strings.Join(report.Warnings, "\n"), `ambiguous Obsidian link "Beta" in alpha.md`) {
					t.Fatalf("alpha.md's link is ambiguous now and should warn: %v", report.Warnings)
				}
				doc, err := st.GetDocument(ctx, documentID("alpha.md"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(doc.Body, store.DocumentURI("default", documentID("sub/beta.md"))) {
					t.Fatalf("an ambiguous link must not keep its old target:\n%s", doc.Body)
				}
			},
		},
		{
			name: "another note's title changed",
			change: func(t *testing.T, _ *store.SQLiteStore, dir string) {
				writeFile(t, filepath.Join(dir, "sub", "beta.md"), "# Beta, retitled\n\nBack to [[index]].\n")
			},
			skipped: 0, updated: 1,
		},
		{
			name: "one note edited, no name moved",
			change: func(t *testing.T, _ *store.SQLiteStore, dir string) {
				writeFile(t, filepath.Join(dir, "plain.md"), "# Plain\n\nNothing links from here, still.\n")
			},
			skipped: 4, updated: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := j39Vault(t)
			st := openTestStore(t)
			j39Import(t, st, dir, Options{})
			tc.change(t, st, dir)
			report := j39Import(t, st, dir, Options{})
			if report.NotesSkipped != tc.skipped || report.NotesUpdated != tc.updated {
				t.Fatalf("skipped %d and updated %d, want %d and %d: %#v",
					report.NotesSkipped, report.NotesUpdated, tc.skipped, tc.updated, report)
			}
			if tc.check != nil {
				tc.check(t, st, report)
			}
			// Each warning in these vaults comes from a different note, and a
			// note that warned is read every time.
			if again := j39Import(t, st, dir, Options{}); again.NotesSkipped != again.MarkdownSeen-len(again.Warnings) {
				t.Fatalf("once repaired, every note that did not warn should be skipped: %#v", again)
			}
		})
	}
}

// TestJ39BANoteThatWarnedIsAlwaysRead: a warning is reported on every import,
// so a note whose links warned is read on every import too.
func TestJ39BANoteThatWarnedIsAlwaysRead(t *testing.T) {
	dir := j39Vault(t)
	writeFile(t, filepath.Join(dir, "other", "beta.md"), "# Beta\n\nA second Beta.\n")
	st := openTestStore(t)
	first := j39Import(t, st, dir, Options{})
	if len(first.Warnings) != 1 {
		t.Fatalf("the vault should warn once: %v", first.Warnings)
	}
	report := j39Import(t, st, dir, Options{})
	if report.NotesSkipped != 5 || report.NotesUnchanged != 6 {
		t.Fatalf("every note but the one that warned should be skipped: %#v", report)
	}
	if !reflect.DeepEqual(report.Warnings, first.Warnings) {
		t.Fatalf("a reimport should report what the import before it did: %v, then %v", first.Warnings, report.Warnings)
	}
	// The warned note is known by its state alone: it records the bare note
	// fingerprint, so no head is fetched for it and nothing is hashed to rule
	// it out. Every other note records the form a skip can match.
	for relPath, prefixed := range j39StatePrefixes(t, st, dir) {
		if want := relPath != "alpha.md"; prefixed != want {
			t.Errorf("%s: skip form recorded = %v, want %v", relPath, prefixed, want)
		}
	}
}

// j39StatePrefixes reports, per note, whether its state is in skipPrefix form.
func j39StatePrefixes(t *testing.T, st store.Store, dir string) map[string]bool {
	t.Helper()
	ctx := context.Background()
	run, err := newImportRun(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, item := range run.inventory.Notes {
		keys = append(keys, item.itemKey())
	}
	states, err := st.GetImportItemStates(ctx, sourceSystem, run.report.SourceKey, "default", keys)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, item := range run.inventory.Notes {
		out[item.RelPath] = strings.HasPrefix(states[item.itemKey()].Fingerprint, skipPrefix)
	}
	return out
}

// j39Library is everything a reimport writes, in a form two libraries can be
// compared by. Revision IDs and timestamps are left out, because they are new
// on every write; revision counts stand in for them.
func j39Library(t *testing.T, st *store.SQLiteStore, dir string) string {
	t.Helper()
	ctx := context.Background()
	var out strings.Builder
	run, err := newImportRun(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	ids, keys := []string{}, []string{}
	for _, item := range run.inventory.Notes {
		ids = append(ids, item.TargetID)
		keys = append(keys, item.itemKey())
	}
	documents, err := st.GetDocuments(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := st.GetDocumentSources(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	states, err := st.GetImportItemStates(ctx, sourceSystem, run.report.SourceKey, "default", keys)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range run.inventory.Notes {
		id := item.TargetID
		doc := documents[id]
		revisions, err := st.ListDocumentRevisions(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&out, "== %s %s title=%q notebook=%s revisions=%d\n%s\n", item.RelPath, id, doc.Title, doc.NotebookID, len(revisions), doc.Body)
		links, err := st.ListDocumentLinks(ctx, id, "outgoing")
		if err != nil {
			t.Fatal(err)
		}
		for _, link := range links.Outgoing {
			link.ID = 0
			fmt.Fprintf(&out, "link %#v\n", link)
		}
		for _, ref := range j39Refs(t, st, item.RelPath) {
			fmt.Fprintf(&out, "ref %s\n", ref)
		}
		source := sources[id]
		fmt.Fprintf(&out, "source %s %s %s\n", source.SourceSystem, source.ExternalID, source.MetadataJSON)
		state := states[item.itemKey()]
		fmt.Fprintf(&out, "state %s %s %s %s\n", state.ItemType, state.Fingerprint, state.TargetID, state.Action)
	}
	return out.String()
}

// TestJ39CSkipsLeaveTheSameLibrary is J39-C: two libraries go through the same
// history, and the last reimport skips in one and reads every note in the
// other. They must end the same in everything a reimport writes.
func TestJ39CSkipsLeaveTheSameLibrary(t *testing.T) {
	ctx := context.Background()
	dir := j39Vault(t)
	withSkips, without := openTestStore(t), openTestStore(t)
	for _, st := range []*store.SQLiteStore{withSkips, without} {
		j39Import(t, st, dir, Options{})
		// The library side diverges in each way a skip has to notice.
		doc, err := st.GetDocument(ctx, documentID("alpha.md"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID: doc.ID, Title: doc.Title, Body: doc.Body + "\nedited\n",
			BodyMIMEType: doc.BodyMIMEType, BaseRevisionID: doc.CurrentRevisionID,
		}); err != nil {
			t.Fatal(err)
		}
		if err := st.DetachDocumentResource(ctx, documentID("gamma.md"), resourceID("pic.png")); err != nil {
			t.Fatal(err)
		}
	}
	// And so does the vault, without moving a name.
	writeFile(t, filepath.Join(dir, "plain.md"), "# Plain\n\nEdited in the vault.\n")

	skipped := j39Import(t, withSkips, dir, Options{})
	read := j39Import(t, without, dir, Options{readEveryNote: true})
	if skipped.NotesSkipped != 2 || read.NotesSkipped != 0 {
		t.Fatalf("the comparison needs skips on one side only: %d and %d", skipped.NotesSkipped, read.NotesSkipped)
	}
	if left, right := j39Library(t, withSkips, dir), j39Library(t, without, dir); left != right {
		t.Fatalf("a reimport with skips left a different library.\nwith skips:\n%s\nwithout:\n%s", left, right)
	}
	for _, pair := range [][2]int{
		{skipped.NotesImported, read.NotesImported},
		{skipped.NotesUpdated, read.NotesUpdated},
		{skipped.NotesUnchanged, read.NotesUnchanged},
		{skipped.LinkIndexesRefreshed, read.LinkIndexesRefreshed},
	} {
		if pair[0] != pair[1] {
			t.Fatalf("the reports differ beyond what skipping counts:\n%#v\n%#v", skipped, read)
		}
	}
	if !reflect.DeepEqual(skipped.Warnings, read.Warnings) {
		t.Fatalf("warnings differ: %v and %v", skipped.Warnings, read.Warnings)
	}
}

// j39RulesCorpus is a vault of the cases the rewriting rules have had to
// decide, from J32 to J43: every link form, every way a target resolves or does
// not, titles, anchors, frontmatter, and the overlap the one-pass assembly
// declines.
func j39RulesCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	notes := map[string]string{
		"root.md":                  "# Root\n\n[[note]] [[folder/note]] [[note|shown]] [[note#Heading]] [[note#^block]] ![[pic.png]] ![[folder/pic.png]]\n",
		"note.md":                  "# Note\n\n## Heading\n\nA block ^block\n",
		"folder/note.md":           "# Folder Note\n\n[[note]] [[./note]] [[../root]] [wrong](wrong/path/note.md)\n",
		"folder/pic-user.md":       "# Pictures\n\n![](pic.png) ![alt](folder/pic.png) ![[pic.png|200]]\n",
		"titles.md":                "# Titles\n\n[a](note.md \"double\") [b](note.md 'single') [c](note.md (paren)) [d](<folder/note.md> \"angle\") ![e](pic.png \"img\")\n",
		"angle.md":                 "# Angle\n\n[spaced](<with space.md>) [enc](with%20space.md) [paren](paren(1).md)\n",
		"with space.md":            "# With Space\n",
		"paren(1).md":              "# Paren One\n",
		"external.md":              "# External\n\n[x](https://example.com) [m](mailto:a@example.com) [d](document://default/doc_x) [r](resource://default/res_x)\n",
		"frontmatter.md":           "---\ntitle: Front Title\naliases:\n  - Fronted\nrelated: \"[[note]]\"\n---\nBody after frontmatter. [[Fronted]]\n",
		"no-frontmatter-crlf.md":   "# CRLF\r\n\r\nLine [[note]]\r\n",
		"ambiguous.md":             "# Ambiguous\n\n[[Twin]] [[twin]]\n",
		"a/twin.md":                "# Twin\n",
		"b/twin.md":                "# Twin\n",
		"[[note]].md":              "# Odd Name\n",
		"overlap.md":               "# Overlap\n\n[x]([[note]].md) [[[note]]]\n",
		"empty.md":                 "",
		"only-frontmatter.md":      "---\ntags: [a]\n---\n",
		"aliased.md":               "---\naliases: [Other Name, \"Quoted, Name\"]\n---\n# Aliased\n",
		"uses-alias.md":            "# Uses Alias\n\n[[Other Name]] [[Quoted, Name]] [[aliased]]\n",
		"unicode/Grüße.md":         "# Grüße 🍲\n\n[[grüße]] [[Grüße#Überschrift]]\n",
		"deep/er/still/deepest.md": "# Deepest\n\n[[deepest]] [[deep/er/still/deepest]] [[still/deepest]]\n",
	}
	for path, body := range notes {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(path)), body)
	}
	writeBytes(t, filepath.Join(dir, "pic.png"), []byte("root picture"))
	writeBytes(t, filepath.Join(dir, "folder", "pic.png"), []byte("folder picture"))
	return dir
}

// j39PinnedCanonical holds, for each value canonicalRules has had, the digest of
// what those rules make of j39RulesCorpus: every canonical body, attachment and
// warning.
var j39PinnedCanonical = map[int]string{
	1: "e7c5c7dd6644128121472e0e7a27d74814133f3a1a75e53aadb67b4e30aa8070",
}

// TestJ39CanonicalRulesArePinned is the guard on J39's rule version. A skip
// trusts that a note recorded under canonicalRules would be written the same
// way now; that holds only if the rules' output cannot change while the number
// stays the same. Change the rules and this fails, until canonicalRules is
// moved and the new digest is pinned beside it. Pinning a new digest under the
// old number would let every library keep notes written under the old rules.
func TestJ39CanonicalRulesArePinned(t *testing.T) {
	ctx := context.Background()
	dir := j39RulesCorpus(t)
	run, err := newImportRun(ctx, openTestStore(t), dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	for _, item := range run.inventory.Notes {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(item.RelPath)))
		if err != nil {
			t.Fatal(err)
		}
		canonical, attachments, rewritten, warnings := run.canonicalBody(item, raw)
		fmt.Fprintf(hash, "%s\x00%s\x00%d\x00", item.RelPath, canonical, rewritten)
		for _, attachment := range attachments {
			fmt.Fprintf(hash, "%s\x00%s\x00%s\x00", attachment.ResourceID, attachment.RelationType, attachment.RawTarget)
		}
		for _, warning := range warnings {
			fmt.Fprintf(hash, "%s\x00", warning)
		}
		hash.Write([]byte("\n"))
	}
	got := hex.EncodeToString(hash.Sum(nil))
	pinned, ok := j39PinnedCanonical[canonicalRules]
	if !ok {
		t.Fatalf("canonicalRules is %d and nothing is pinned for it; pin %s", canonicalRules, got)
	}
	if got != pinned {
		t.Fatalf("the canonical rules' output changed under version %d: pinned %s, now %s.\n"+
			"If the change is intended, move canonicalRules to %s and pin %s beside it, so libraries "+
			"imported under the old rules are read again.", canonicalRules, pinned, got, strconv.Itoa(canonicalRules+1), got)
	}
}
