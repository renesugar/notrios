package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func sortedHitIDs(response SearchResponse) []string {
	ids := make([]string, 0, len(response.Hits))
	for _, hit := range response.Hits {
		ids = append(ids, hit.ID)
	}
	slices.Sort(ids)
	return ids
}

func sortedStrings(values ...string) []string {
	slices.Sort(values)
	return values
}

func TestQueryLanguageSearch(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	work, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	sub, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Reports", ParentID: work.ID})

	inWork, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Quarterly planning", Body: "meeting agenda apples", NotebookID: work.ID})
	inSub, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "July report", Body: "quarterly numbers oranges", NotebookID: sub.ID})
	elsewhere, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Groceries", Body: "apples oranges"})

	if _, err := st.AddDocumentTag(ctx, elsewhere.ID, "todo"); err != nil {
		t.Fatalf("AddDocumentTag: %v", err)
	}

	// notebook: includes descendants and is case-insensitive.
	res, err := st.Search(ctx, SearchRequest{Query: `notebook:work`, Limit: 10})
	if err != nil || len(res.Hits) != 2 {
		t.Fatalf("notebook filter: %+v err=%v", res, err)
	}
	// notebook: + text term combine.
	res, _ = st.Search(ctx, SearchRequest{Query: `notebook:work apples`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != inWork.ID {
		t.Fatalf("notebook+term filter: %+v", res)
	}
	// title: uses the FTS title column only.
	res, _ = st.Search(ctx, SearchRequest{Query: `title:quarterly`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != inWork.ID {
		t.Fatalf("title filter must not match body text: %+v", res)
	}
	// tag: matches case-insensitively.
	res, _ = st.Search(ctx, SearchRequest{Query: `tag:TODO`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != elsewhere.ID {
		t.Fatalf("tag filter: %+v", res)
	}
	// Phrases must match exactly.
	res, _ = st.Search(ctx, SearchRequest{Query: `"meeting agenda"`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != inWork.ID {
		t.Fatalf("phrase search: %+v", res)
	}
	res, _ = st.Search(ctx, SearchRequest{Query: `"agenda meeting"`, Limit: 10})
	if len(res.Hits) != 0 {
		t.Fatalf("reversed phrase must not match: %+v", res)
	}
	// Unknown notebook name yields empty results, not an error.
	res, err = st.Search(ctx, SearchRequest{Query: `notebook:nope`, Limit: 10})
	if err != nil || len(res.Hits) != 0 {
		t.Fatalf("unknown notebook: %+v err=%v", res, err)
	}
	_ = inSub
}

func TestQueryAuthorAndTimeFilters(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	post1, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "post one", Body: "thread content"})
	post2, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "post two", Body: "thread content"})
	_, _ = st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID: post1.ID, SourceSystem: "twitter", ExternalID: "p1",
		Author: "Alice Smith", AuthorID: "alice@example.social", PublishedAt: "2026-07-01T12:00:00Z",
	})
	_, _ = st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID: post2.ID, SourceSystem: "twitter", ExternalID: "p2",
		Author: "Bob Jones", AuthorID: "bob@example.social", PublishedAt: "2026-07-20T12:00:00Z",
	})

	res, _ := st.Search(ctx, SearchRequest{Query: `author:"Alice Smith"`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != post1.ID {
		t.Fatalf("author filter: %+v", res)
	}
	res, _ = st.Search(ctx, SearchRequest{Query: `authorid:ALICE@example.social`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != post1.ID {
		t.Fatalf("authorid filter: %+v", res)
	}
	res, _ = st.Search(ctx, SearchRequest{Query: `since:2026-07-10 until:2026-07-31`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != post2.ID {
		t.Fatalf("since/until on published_ts: %+v", res)
	}
	res, _ = st.Search(ctx, SearchRequest{Query: `authorid:bob@example.social thread`, Limit: 10})
	if len(res.Hits) != 1 || res.Hits[0].ID != post2.ID {
		t.Fatalf("metadata+text combination: %+v", res)
	}
}

func TestBooleanExpressionsCategoryAliasAndEmoji(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	work, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	reports, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Reports", ParentID: work.ID})
	personal, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Personal"})

	alpha, _ := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "expr_alpha", NotebookID: work.ID, Title: "Alpha plan", Body: "roadmap 😀 well-known https://example.test/a_(b)",
	})
	beta, _ := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "expr_beta", NotebookID: reports.ID, Title: "Beta report", Body: "oranges 😀",
	})
	gamma, _ := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "expr_gamma", NotebookID: personal.ID, Title: "Gamma", Body: "alpha oranges re:invoice",
	})
	private, _ := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "expr_private", NotebookID: personal.ID, Title: "Private beta", Body: "secret oranges",
	})
	_, _ = st.AddDocumentTag(ctx, private.ID, "private")
	_, _ = st.AddDocumentTag(ctx, beta.ID, "team")
	_, _ = st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID: alpha.ID, SourceSystem: "twitter", ExternalID: "alpha", Author: "Alice Smith", AuthorID: "alice@example.social",
	})
	_, _ = st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID: beta.ID, SourceSystem: "twitter", ExternalID: "beta", Author: "Bob Jones", AuthorID: "bob@example.social",
	})

	tests := []struct {
		query string
		want  []string
	}{
		{`alpha OR beta oranges`, sortedStrings(alpha.ID, beta.ID, gamma.ID, private.ID)},
		{`(alpha OR beta) oranges`, sortedStrings(beta.ID, gamma.ID, private.ID)},
		{`(alpha OR beta) -tag:private`, sortedStrings(alpha.ID, beta.ID, gamma.ID)},
		{`category:work`, sortedStrings(alpha.ID, beta.ID)},
		{`notebook:WORK`, sortedStrings(alpha.ID, beta.ID)},
		{`category:"All notes" -tag:private`, sortedStrings(alpha.ID, beta.ID, gamma.ID)},
		{`-(title:Gamma OR tag:private) 😀`, sortedStrings(alpha.ID, beta.ID)},
		{`(author:"Alice Smith" OR author:"Bob Jones") -authorid:bob@example.social`, []string{alpha.ID}},
		{`re:invoice`, []string{gamma.ID}},
		{`https://example.test/a_(b)`, []string{alpha.ID}},
		{`well-known 😀`, []string{alpha.ID}},
	}
	for _, tc := range tests {
		response, err := st.Search(ctx, SearchRequest{Query: tc.query, Limit: 20})
		if err != nil {
			t.Errorf("Search(%q): %v", tc.query, err)
			continue
		}
		if got := sortedHitIDs(response); !slices.Equal(got, tc.want) {
			t.Errorf("Search(%q) IDs = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestExpressionCursorBindingAndQueryLimits(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()
	for index := 0; index < 4; index++ {
		_, _ = st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: fmt.Sprintf("binding_%d", index), Title: "alpha beta", Body: "cursor expression",
		})
	}

	first, err := st.Search(ctx, SearchRequest{Query: `alpha OR beta`, Limit: 1})
	if err != nil || first.NextCursor == "" {
		t.Fatalf("first expression page = %+v, %v", first, err)
	}
	if _, err := st.Search(ctx, SearchRequest{Query: `alpha   OR   beta`, Limit: 1, Cursor: first.NextCursor}); err != nil {
		t.Fatalf("canonical-equivalent query should accept cursor: %v", err)
	}
	if _, err := st.Search(ctx, SearchRequest{Query: `(alpha OR beta) gamma`, Limit: 1, Cursor: first.NextCursor}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("different expression must reject cursor: %v", err)
	}
	if _, err := st.Search(ctx, SearchRequest{Query: strings.Repeat("x", 4097), Limit: 1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized query error = %v", err)
	}
	if _, err := st.Search(ctx, SearchRequest{Query: strings.Repeat("(", 17) + "alpha" + strings.Repeat(")", 17), Limit: 1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("over-nested query error = %v", err)
	}
}

func TestTrashQueryAndCursorPagination(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	doc, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "deleted note", Body: "shredded lettuce"})
	_ = st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID})

	// The Trash search notebook query finds only trashed notes, with text terms.
	res, err := st.Search(ctx, SearchRequest{Query: `is:trashed`, Limit: 10})
	if err != nil || len(res.Hits) != 1 || res.Hits[0].ID != doc.ID {
		t.Fatalf("is:trashed: %+v err=%v", res, err)
	}
	res, _ = st.Search(ctx, SearchRequest{Query: `is:trashed lettuce`, Limit: 10})
	if len(res.Hits) != 1 {
		t.Fatalf("is:trashed with term: %+v", res)
	}
	// Trashed notes never appear in normal queries.
	res, _ = st.Search(ctx, SearchRequest{Query: `lettuce`, Limit: 10})
	if len(res.Hits) != 0 {
		t.Fatalf("trashed note leaked into normal search: %+v", res)
	}

	// Cursor pagination over the "All notes" (empty) query.
	for i := 0; i < 5; i++ {
		if _, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: fmt.Sprintf("note %d", i), Body: "filler"}); err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
	}
	page1, err := st.Search(ctx, SearchRequest{Query: "", Limit: 2})
	if err != nil || len(page1.Hits) != 2 || page1.NextCursor == "" {
		t.Fatalf("page1: %+v err=%v", page1, err)
	}
	page2, err := st.Search(ctx, SearchRequest{Query: "", Limit: 2, Cursor: page1.NextCursor})
	if err != nil || len(page2.Hits) != 2 || page2.NextCursor == "" {
		t.Fatalf("page2: %+v err=%v", page2, err)
	}
	if page1.Hits[0].ID == page2.Hits[0].ID {
		t.Fatalf("pages overlap: %+v %+v", page1.Hits, page2.Hits)
	}
	page3, err := st.Search(ctx, SearchRequest{Query: "", Limit: 2, Cursor: page2.NextCursor})
	if err != nil || len(page3.Hits) != 1 || page3.NextCursor != "" {
		t.Fatalf("page3 must be the last page: %+v err=%v", page3, err)
	}

	// A cursor is bound to its query.
	if _, err := st.Search(ctx, SearchRequest{Query: "different", Limit: 2, Cursor: page1.NextCursor}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor replay against another query must fail: %v", err)
	}
	if _, err := st.Search(ctx, SearchRequest{Query: "", Limit: 2, Cursor: "garbage!"}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("garbage cursor must fail cleanly: %v", err)
	}
}

func TestSearchKeysetsChronologicalAndRelevanceResults(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("keyset_doc_%02d", i)
		if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: id,
			Title:       fmt.Sprintf("Ranked %02d", i),
			Body:        "shared ranking phrase",
		}); err != nil {
			t.Fatalf("CreateDocument %s: %v", id, err)
		}
	}

	assertAllPages := func(queryText string) {
		t.Helper()
		cursor := ""
		seen := map[string]bool{}
		for {
			page, err := st.Search(ctx, SearchRequest{Query: queryText, Limit: 2, Cursor: cursor})
			if err != nil {
				t.Fatalf("Search(%q): %v", queryText, err)
			}
			for _, hit := range page.Hits {
				if seen[hit.ID] {
					t.Fatalf("Search(%q) repeated %s across pages", queryText, hit.ID)
				}
				seen[hit.ID] = true
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		if len(seen) != 7 {
			t.Fatalf("Search(%q) visited %d documents, want 7", queryText, len(seen))
		}
	}
	assertAllPages("")
	assertAllPages(`"shared ranking"`)

	chronological, err := st.Search(ctx, SearchRequest{Limit: 2})
	if err != nil || chronological.NextCursor == "" {
		t.Fatalf("chronological first page: %+v err=%v", chronological, err)
	}
	if _, err := st.Search(ctx, SearchRequest{
		Query:  `"shared ranking"`,
		Limit:  2,
		Cursor: chronological.NextCursor,
	}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("chronological cursor must not replay as relevance cursor: %v", err)
	}
}

// Searching by where a note came from, which nothing could ask before:
// `category:` is an alias for `notebook:`, so filing was addressable and
// provenance was not.
func TestSearchByCollection(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	if _, err := st.CreateCollection(ctx, Collection{
		ID: "joplin-raw-2026-07", Name: "Joplin, July 2026",
	}); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	mine, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Reed beds", Body: "harriers at dusk"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	imported, err := st.CreateDocument(ctx, CreateDocumentRequest{
		CollectionID: "joplin-raw-2026-07", Title: "Old note", Body: "harriers at dusk",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// Both notes say the same thing, so only the collection distinguishes them.
	only, err := st.Search(ctx, SearchRequest{Query: `harriers collection:"joplin-raw-2026-07"`})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(only.Hits) != 1 || only.Hits[0].ID != imported.ID {
		t.Fatalf("expected only the imported note: %+v", only.Hits)
	}

	// Case-insensitive, because a person types the id.
	shouted, err := st.Search(ctx, SearchRequest{Query: `collection:"JOPLIN-RAW-2026-07"`})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(shouted.Hits) != 1 {
		t.Fatalf("collection ids match case-insensitively: %+v", shouted.Hits)
	}

	// Exact, never a prefix. Collection ids are dated by convention, and a
	// prefix match would answer a question about July with August's notes.
	prefix, err := st.Search(ctx, SearchRequest{Query: `collection:"joplin-raw-2026"`})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(prefix.Hits) != 0 {
		t.Fatalf("a shorter id is a different collection: %+v", prefix.Hits)
	}

	// And the default collection is addressable the same way.
	def, err := st.Search(ctx, SearchRequest{Query: `harriers collection:"default"`})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(def.Hits) != 1 || def.Hits[0].ID != mine.ID {
		t.Fatalf("expected only my own note: %+v", def.Hits)
	}
}

// Creating a provenance, and refusing to create it twice.
func TestCreateCollectionRefusesDuplicatesAndNamesItself(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)

	// A blank name becomes the id: an import that says where notes came from
	// should not have to say it twice, and an empty name is worse than a
	// redundant one.
	created, err := st.CreateCollection(ctx, Collection{ID: "twitter-archive"})
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if created.Name != "twitter-archive" {
		t.Fatalf("a blank name defaults to the id: %+v", created)
	}

	if _, err := st.CreateCollection(ctx, Collection{ID: "twitter-archive", Name: "Again"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("creating one twice is a conflict, not a silent overwrite: %v", err)
	}
	// And the first name survived the refusal.
	read, err := st.Collection(ctx, "twitter-archive")
	if err != nil || read.Name != "twitter-archive" {
		t.Fatalf("the refused create must not have changed anything: %+v %v", read, err)
	}

	if _, err := st.Collection(ctx, "never-made"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown collection is not found: %v", err)
	}
	if _, err := st.CreateCollection(ctx, Collection{ID: "  "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a collection needs an id: %v", err)
	}
}

// EnsureCollection is what the importers use, and is the reason --collection
// works at all: documents.collection_id is a foreign key, so a note naming a
// collection nobody created failed on the constraint.
func TestEnsureCollectionIsCreateOrAdopt(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)

	first, err := st.EnsureCollection(ctx, "joplin-raw-2026-07", "Joplin, July 2026")
	if err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	// Called again it adopts rather than fails or renames: an importer run
	// twice is an ordinary thing to do.
	second, err := st.EnsureCollection(ctx, "joplin-raw-2026-07", "A different name")
	if err != nil {
		t.Fatalf("EnsureCollection again: %v", err)
	}
	if second.ID != first.ID || second.Name != first.Name {
		t.Fatalf("ensuring an existing collection must not rename it: %+v then %+v", first, second)
	}

	// And a note can now name it, which is the whole point.
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		CollectionID: "joplin-raw-2026-07", Title: "Imported", Body: "body",
	}); err != nil {
		t.Fatalf("a note could not name the ensured collection: %v", err)
	}
}
