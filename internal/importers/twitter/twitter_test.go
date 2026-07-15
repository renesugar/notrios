package twitter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "data", "account.js"), `window.YTD.account.part0 = [
  {"account": {"username": "alice", "accountId": "12345", "accountDisplayName": "Alice Smith"}}
]`)
	writeFile(t, filepath.Join(dir, "data", "tweets.js"), `window.YTD.tweets.part0 = [
  {"tweet": {"id_str": "1002", "full_text": "Second post in thread, replying to myself.", "created_at": "Mon Jul 13 12:01:00 +0000 2026", "in_reply_to_status_id_str": "1001", "entities": {"hashtags": [], "urls": []}}},
  {"tweet": {"id_str": "1001", "full_text": "Thread root about #searchengines with a link https://t.co/abc and photo https://t.co/pic", "created_at": "Mon Jul 13 12:00:00 +0000 2026", "entities": {"hashtags": [{"text": "searchengines"}], "urls": [{"url": "https://t.co/abc", "expanded_url": "https://www.recoll.org/"}]}, "extended_entities": {"media": [{"id_str": "9001", "url": "https://t.co/pic", "media_url_https": "https://pbs.twimg.com/media/xyz.png"}]}}},
  {"tweet": {"id_str": "2001", "full_text": "Standalone reply to someone else.", "created_at": "Tue Jul 14 09:00:00 +0000 2026", "in_reply_to_status_id_str": "555", "entities": {"hashtags": [], "urls": []}}}
]`)
	writeFile(t, filepath.Join(dir, "data", "tweets_media", "1001-xyz.png"), "PNGDATA")
	return dir
}

func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return st
}

func TestImportTwitterArchiveFixture(t *testing.T) {
	ctx := context.Background()
	dir := buildFixture(t)
	st := newTestStore(t)

	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if report.NotesImported != 3 || report.MediaImported != 1 || report.ThreadsRecovered != 1 || report.TagsApplied != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.AccountUsername != "alice" || report.AccountDisplay != "Alice Smith" {
		t.Fatalf("account not parsed: %+v", report)
	}

	// Root tweet: expanded URL, media resource embed, original-post link,
	// Twitter notebook membership.
	doc, err := st.GetDocument(ctx, "doc_twitter_1001")
	if err != nil {
		t.Fatalf("root tweet doc: %v", err)
	}
	if !strings.Contains(doc.Body, "https://www.recoll.org/") {
		t.Fatalf("t.co URL not expanded: %s", doc.Body)
	}
	if strings.Contains(doc.Body, "https://t.co/pic") {
		t.Fatalf("media t.co link should be dropped: %s", doc.Body)
	}
	if !strings.Contains(doc.Body, "resource://default/resources/res_twitter_1001_xyz_png") {
		t.Fatalf("media not embedded: %s", doc.Body)
	}
	if !strings.Contains(doc.Body, "https://twitter.com/alice/status/1001") {
		t.Fatalf("original post link missing: %s", doc.Body)
	}
	if doc.NotebookID != TwitterNotebookID {
		t.Fatalf("tweet not in Twitter notebook: %+v", doc)
	}
	nb, err := st.GetNotebook(ctx, TwitterNotebookID)
	if err != nil || nb.Name != "Twitter" || nb.IconEmoji != "🐦" {
		t.Fatalf("Twitter notebook wrong: %+v err=%v", nb, err)
	}

	// Provenance and thread recovery: 1001 <- 1002 share thread 1001; the
	// reply to an unarchived tweet keeps its external reply_to.
	src, err := st.GetDocumentSource(ctx, "doc_twitter_1002")
	if err != nil || src.ThreadID != "1001" || src.ReplyTo != "1001" || src.AuthorID != "@alice" {
		t.Fatalf("reply provenance wrong: %+v err=%v", src, err)
	}
	thread, err := st.ListThreadDocuments(ctx, "1001")
	if err != nil || len(thread) != 2 || thread[0].ExternalID != "1001" || thread[1].ExternalID != "1002" {
		t.Fatalf("thread recovery wrong: %+v err=%v", thread, err)
	}
	orphan, _ := st.GetDocumentSource(ctx, "doc_twitter_2001")
	if orphan.ThreadID != "2001" || orphan.ReplyTo != "555" {
		t.Fatalf("external reply provenance wrong: %+v", orphan)
	}

	// Hashtags became tags.
	tags, _ := st.ListDocumentTags(ctx, "doc_twitter_1001")
	if len(tags) != 1 || tags[0].Name != "searchengines" {
		t.Fatalf("hashtag tag missing: %+v", tags)
	}

	// Query language reaches tweets by author and notebook.
	res, err := st.Search(ctx, store.SearchRequest{Query: `authorid:@alice notebook:twitter root`, Limit: 10})
	if err != nil || len(res.Hits) != 1 {
		t.Fatalf("author+notebook search: %+v err=%v", res, err)
	}

	// Imported tweets are purge-protected in the trash.
	root, _ := st.GetDocument(ctx, "doc_twitter_1001")
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: root.ID, BaseRevisionID: root.CurrentRevisionID}); err != nil {
		t.Fatalf("trash tweet: %v", err)
	}
	if err := st.PurgeDocument(ctx, root.ID); !errors.Is(err, store.ErrProtected) {
		t.Fatalf("tweets must be purge-protected: %v", err)
	}

	// Re-running the import is idempotent for the unchanged notes and
	// leaves the trashed one alone (it is skipped as not-found... restored
	// imports would recreate provenance only).
	report2, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if report2.NotesUnchanged < 2 || report2.MediaImported != 0 {
		t.Fatalf("re-import not idempotent: %+v", report2)
	}
}

func TestImportTwitterDryRun(t *testing.T) {
	ctx := context.Background()
	dir := buildFixture(t)
	st := newTestStore(t)

	report, err := Import(ctx, st, dir, Options{CollectionID: "default", DryRun: true})
	if err != nil || report.NotesImported != 3 || !report.DryRun {
		t.Fatalf("dry run report: %+v err=%v", report, err)
	}
	if _, err := st.GetDocument(ctx, "doc_twitter_1001"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry run must not write: %v", err)
	}
	if _, err := st.GetNotebook(ctx, TwitterNotebookID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry run must not create notebooks: %v", err)
	}
}
