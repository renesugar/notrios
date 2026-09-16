package twitter

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J25: a large Twitter/X archive splits its posts across tweets.js and
// tweets-part1.js, tweets-part2.js, ..., and a user downloads it as a ZIP. The
// importer read only tweets.js, and only from an extracted folder: on the
// owner's 3.3 GB archive that is 55,339 of 176,423 posts.
//
// Every fixture here is synthetic. No real archive content is in the repository.

// j25Post is one synthetic post entry.
func j25Post(id, text, replyTo string) string {
	reply := ""
	if replyTo != "" {
		reply = fmt.Sprintf(`, "in_reply_to_status_id_str": %q`, replyTo)
	}
	return fmt.Sprintf(`{"tweet": {"id_str": %q, "full_text": %q, "created_at": "Mon Jul 13 12:00:00 +0000 2026"%s, "entities": {"hashtags": [], "urls": []}}}`, id, text, reply)
}

func j25File(variable, part string, posts ...string) string {
	return fmt.Sprintf("window.YTD.%s.%s = [\n  %s\n]", variable, part, strings.Join(posts, ",\n  "))
}

func j25Header(ids ...string) string {
	entries := make([]string, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, fmt.Sprintf(`{"tweet": {"tweet_id": %q, "user_id": "12345", "created_at": "Mon Jul 13 12:00:00 +0000 2026"}}`, id))
	}
	return fmt.Sprintf("window.YTD.tweet_headers.part0 = [\n  %s\n]", strings.Join(entries, ",\n  "))
}

// j25Archive is the file set of a split archive: three post parts, a community
// post, a deleted post, headers for the posts in the parts, and one media file.
func j25Archive() map[string]string {
	return map[string]string{
		"data/account.js": `window.YTD.account.part0 = [
  {"account": {"username": "alice", "accountId": "12345", "accountDisplayName": "Alice Smith"}}
]`,
		"data/tweets.js":               j25File("tweets", "part0", j25Post("3001", "Part zero root", ""), j25Post("3002", "Part zero second", "3001")),
		"data/tweets-part1.js":         j25File("tweets", "part1", j25Post("3101", "Part one post", ""), j25Post("3102", "Part one reply to part zero", "3002")),
		"data/tweets-part2.js":         j25File("tweets", "part2", j25Post("3201", "Part two post", "")),
		"data/community-tweet.js":      j25File("community_tweet", "part0", j25Post("3301", "Community post", "")),
		"data/deleted-tweets.js":       j25File("deleted_tweets", "part0", j25Post("3401", "Deleted post", "")),
		"data/tweet-headers.js":        j25Header("3001", "3002", "3101", "3102", "3201"),
		"data/tweets_media/3101-a.png": "PNGDATA",
	}
}

func j25WriteDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(name)), content)
	}
	return dir
}

func j25WriteZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "twitter-2026-08-18-synthetic.zip")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

var j25ImportedIDs = []string{"3001", "3002", "3101", "3102", "3201", "3301"}

func TestJ25ImportsEveryTweetsPartFromAnExtractedFolder(t *testing.T) {
	j25CheckArchive(t, j25WriteDir(t, j25Archive()))
}

func TestJ25ImportsTheDownloadedZipInPlace(t *testing.T) {
	j25CheckArchive(t, j25WriteZip(t, j25Archive()))
}

func j25CheckArchive(t *testing.T, source string) {
	t.Helper()
	ctx := context.Background()
	st := newTestStore(t)
	report, err := Import(ctx, st, source, Options{})
	if err != nil {
		t.Fatalf("Import(%s): %v", filepath.Base(source), err)
	}
	for _, id := range j25ImportedIDs {
		if _, err := st.GetDocument(ctx, "doc_twitter_"+id); err != nil {
			t.Errorf("post %s was not imported: %v", id, err)
		}
	}
	if _, err := st.GetDocument(ctx, "doc_twitter_3401"); err == nil {
		t.Error("a deleted post was imported; the owner decided deleted posts are skipped")
	}
	if report.TweetsSeen != len(j25ImportedIDs) || report.NotesImported != len(j25ImportedIDs) {
		t.Errorf("tweets_seen=%d notes_imported=%d, want %d each", report.TweetsSeen, report.NotesImported, len(j25ImportedIDs))
	}
	if report.CommunityPostsSeen != 1 || report.DeletedPostsSkipped != 1 {
		t.Errorf("community_posts_seen=%d deleted_posts_skipped=%d, want 1 and 1", report.CommunityPostsSeen, report.DeletedPostsSkipped)
	}
	if report.TweetHeaders != 5 || report.PostsInTweetFiles != 5 {
		t.Errorf("tweet_headers=%d posts_in_tweet_files=%d, want 5 and 5", report.TweetHeaders, report.PostsInTweetFiles)
	}
	if want := []string{"tweets.js", "tweets-part1.js", "tweets-part2.js"}; strings.Join(report.TweetFiles, ",") != strings.Join(want, ",") {
		t.Errorf("tweet_files=%v, want %v in part order", report.TweetFiles, want)
	}
	if report.MediaImported != 1 || report.MediaMissing != 0 {
		t.Errorf("media_imported=%d media_missing=%d, want 1 and 0", report.MediaImported, report.MediaMissing)
	}
	if report.MediaFilesInArchive != 1 || report.MediaUnmatched != 0 {
		t.Errorf("media_files_in_archive=%d media_unmatched=%d, want 1 and 0", report.MediaFilesInArchive, report.MediaUnmatched)
	}
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "tweet-headers") {
			t.Errorf("complete archive reported a header mismatch: %s", warning)
		}
	}
	// A reply in part one to a post in part zero joins that thread, which it
	// could not do when part one was never read.
	provenance, err := st.GetDocumentSource(ctx, "doc_twitter_3102")
	if err != nil {
		t.Fatal(err)
	}
	if provenance.ThreadID != "3001" {
		t.Fatalf("a reply across parts must join its thread: thread_id=%q, want 3001", provenance.ThreadID)
	}
}

func TestJ25ReportsAHeaderCountThatDoesNotMatch(t *testing.T) {
	files := j25Archive()
	files["data/tweet-headers.js"] = j25Header("3001", "3002", "3101", "3102", "3201", "3999")
	report, err := Import(context.Background(), newTestStore(t), j25WriteZip(t, files), Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, warning := range report.Warnings {
		found = found || strings.Contains(warning, "tweet-headers.js lists 6 posts")
	}
	if !found {
		t.Fatalf("a header count that differs from the posts found must be reported, warnings: %v", report.Warnings)
	}
}
