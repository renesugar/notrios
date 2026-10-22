// Package twitter imports a Twitter/X archive into the canonical store
// (Notrios redesign task R9). It reads the archive as it is downloaded, a ZIP
// read in place, or its extracted folder (J25). It parses the `window.YTD.*`
// data files, recovers conversation threads by following in-reply-to chains
// among the archived tweets, records provenance (author, canonical handle,
// thread ID, reply-to, post URL, published time) in document_sources, imports
// tweet media as content-addressed resources, and places notes in a "Twitter"
// notebook. Reference format notes: github.com/doggy8088/x-archive-parser.
package twitter

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/importers/archivesource"
	"github.com/renesugar/notrios/internal/store"
)

// TwitterNotebookID is the deterministic notebook that holds imported tweets.
const TwitterNotebookID = "nb_twitter"

// Options controls one import run.
type Options struct {
	CollectionID string
	NotebookName string // defaults to "Twitter"
	DryRun       bool
}

// Report is the JSON-serializable import result.
type Report struct {
	TweetsSeen        int      `json:"tweets_seen"`
	NotesImported     int      `json:"notes_imported"`
	NotesUpdated      int      `json:"notes_updated"`
	NotesUnchanged    int      `json:"notes_unchanged"`
	ThreadsRecovered  int      `json:"threads_recovered"`
	MediaImported     int      `json:"media_imported"`
	MediaMissing      int      `json:"media_missing"`
	TagsApplied       int      `json:"tags_applied"`
	DryRun            bool     `json:"dry_run,omitempty"`
	AccountUsername   string   `json:"account_username,omitempty"`
	AccountDisplay    string   `json:"account_display_name,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
	NotebookID        string   `json:"notebook_id,omitempty"`
	AttachmentsLinked int      `json:"attachments_linked"`

	// SourceFormat is "zip" for a downloaded archive read in place, or
	// "directory" for an extracted one.
	SourceFormat string `json:"source_format"`
	// TweetFiles names the post files read, in part order.
	TweetFiles []string `json:"tweet_files"`
	// PostsInTweetFiles counts the posts in TweetFiles, before duplicates are
	// dropped. TweetHeaders is what tweet-headers.js lists, when the archive
	// has one; a difference between the two is reported as a warning.
	PostsInTweetFiles int `json:"posts_in_tweet_files"`
	TweetHeaders      int `json:"tweet_headers"`
	// CommunityPostsSeen counts posts from community-tweet.js, which are
	// imported. DeletedPostsSkipped counts deleted-tweets.js, which is not:
	// posts the user deleted do not come back silently (owner decision, J25).
	CommunityPostsSeen    int `json:"community_posts_seen"`
	DeletedPostsSkipped   int `json:"deleted_posts_skipped"`
	DuplicatePostsSkipped int `json:"duplicate_posts_skipped"`
	// ArchiveEntriesRejected counts ZIP entries refused as unsafe: a name that
	// escapes the archive, or an entry that is not a regular file.
	ArchiveEntriesRejected int `json:"archive_entries_rejected"`
	// MediaFilesInArchive counts the media directory's files. MediaUnmatched
	// counts those whose name names no imported post, so every media file is
	// either attached (imported or missing) or counted here.
	MediaFilesInArchive int `json:"media_files_in_archive"`
	MediaUnmatched      int `json:"media_unmatched"`
}

type tweet struct {
	ID          string
	Text        string
	CreatedAt   time.Time
	ReplyToID   string
	Hashtags    []string
	URLs        map[string]string // t.co -> expanded
	MediaByTCO  map[string]string // t.co media URL -> media external URL
	MediaFiles  []string          // archive-relative media filenames
	ScreenName  string
	ThreadID    string
	ExternalURL string
}

// tweetEntry is one entry of a post file, as the archive writes it.
type tweetEntry struct {
	Tweet struct {
		IDStr                string `json:"id_str"`
		FullText             string `json:"full_text"`
		Text                 string `json:"text"`
		CreatedAt            string `json:"created_at"`
		InReplyToStatusIDStr string `json:"in_reply_to_status_id_str"`
		Entities             struct {
			Hashtags []struct {
				Text string `json:"text"`
			} `json:"hashtags"`
			URLs []struct {
				URL         string `json:"url"`
				ExpandedURL string `json:"expanded_url"`
			} `json:"urls"`
		} `json:"entities"`
		ExtendedEntities struct {
			Media []struct {
				IDStr         string `json:"id_str"`
				URL           string `json:"url"`
				MediaURLHTTPS string `json:"media_url_https"`
			} `json:"media"`
		} `json:"extended_entities"`
	} `json:"tweet"`
}

type accountEntry struct {
	Account struct {
		Username           string `json:"username"`
		AccountID          string `json:"accountId"`
		AccountDisplayName string `json:"accountDisplayName"`
	} `json:"account"`
}

// Import reads a Twitter/X archive: the ZIP as downloaded, or its extracted
// folder (the folder containing `data/`, or `data/` itself).
func Import(ctx context.Context, st store.Store, sourcePath string, options Options) (Report, error) {
	report := Report{DryRun: options.DryRun}
	if strings.TrimSpace(options.CollectionID) == "" {
		options.CollectionID = "default"
	}
	if strings.TrimSpace(options.NotebookName) == "" {
		options.NotebookName = "Twitter"
	}

	archive, rejected, err := openArchive(sourcePath)
	if err != nil {
		return report, err
	}
	defer archive.Close()
	report.SourceFormat = archive.Kind()
	report.ArchiveEntriesRejected = rejected
	if rejected > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%d archive entries were refused: a name escaping the archive, or not a regular file", rejected))
	}

	username, display, err := parseAccount(archive)
	if err != nil {
		report.Warnings = append(report.Warnings, err.Error())
	}
	report.AccountUsername = username
	report.AccountDisplay = display

	names, err := archive.List("")
	if err != nil {
		return report, err
	}
	collector := &tweetCollector{seen: map[string]bool{}}
	for _, name := range tweetFiles(names) {
		posts, err := decodePosts(archive, name, username, collector.add)
		if err != nil {
			return report, err
		}
		report.TweetFiles = append(report.TweetFiles, name)
		report.PostsInTweetFiles += posts
	}
	if hasName(names, "community-tweet.js") {
		posts, err := decodePosts(archive, "community-tweet.js", username, collector.add)
		if err != nil {
			return report, err
		}
		report.CommunityPostsSeen = posts
	}
	if hasName(names, "deleted-tweets.js") {
		if report.DeletedPostsSkipped, err = countEntries(archive, "deleted-tweets.js"); err != nil {
			return report, err
		}
	}
	if hasName(names, "tweet-headers.js") {
		if report.TweetHeaders, err = countEntries(archive, "tweet-headers.js"); err != nil {
			return report, err
		}
		if report.TweetHeaders != report.PostsInTweetFiles {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"tweet-headers.js lists %d posts, but the tweets files hold %d", report.TweetHeaders, report.PostsInTweetFiles))
		}
	}
	tweets := collector.tweets
	report.DuplicatePostsSkipped = collector.duplicates
	report.TweetsSeen = len(tweets)

	mediaDir := ""
	for _, dir := range []string{"tweets_media", "tweet_media"} {
		if media, err := archive.List(dir); err == nil && len(media) > 0 {
			mediaDir = dir
			report.MediaFilesInArchive = len(media)
			report.MediaUnmatched = attachMedia(tweets, media)
			break
		}
	}
	report.ThreadsRecovered = recoverThreads(tweets)

	if options.DryRun {
		for range tweets {
			report.NotesImported++
		}
		return report, nil
	}

	notebookID, err := ensureNotebook(ctx, st, options.NotebookName)
	if err != nil {
		return report, err
	}
	report.NotebookID = notebookID

	for _, tw := range tweets {
		if err := importTweet(ctx, st, tw, archive, mediaDir, notebookID, options, &report, display, username); err != nil {
			return report, err
		}
	}
	return report, nil
}

// tweetCollector keeps the first copy of each post. A post in more than one
// file (a community post also in tweets.js, say) is imported once.
type tweetCollector struct {
	tweets     []*tweet
	seen       map[string]bool
	duplicates int
}

func (c *tweetCollector) add(tw *tweet) {
	if c.seen[tw.ID] {
		c.duplicates++
		return
	}
	c.seen[tw.ID] = true
	c.tweets = append(c.tweets, tw)
}

func parseAccount(archive archiveSource) (username, display string, err error) {
	file, err := archive.Open("account.js", archivesource.Limits.DataFileBytes)
	if err != nil {
		return "", "", fmt.Errorf("account.js not readable: %w", err)
	}
	defer file.Close()
	var first *accountEntry
	if _, err := decodeYTDArray(file, "account.js", func(entry accountEntry) error {
		if first == nil {
			first = &entry
		}
		return nil
	}); err != nil {
		return "", "", err
	}
	if first == nil {
		return "", "", errors.New("account.js: no account entry")
	}
	return first.Account.Username, first.Account.AccountDisplayName, nil
}

// decodePosts streams one post file, handing each post with an ID to add, and
// returns how many there were.
func decodePosts(archive archiveSource, name, username string, add func(*tweet)) (int, error) {
	file, err := archive.Open(name, archivesource.Limits.DataFileBytes)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	posts := 0
	_, err = decodeYTDArray(file, name, func(entry tweetEntry) error {
		if tw := newTweet(entry, username); tw != nil {
			posts++
			add(tw)
		}
		return nil
	})
	return posts, err
}

// countEntries streams a data file and counts its entries without keeping them.
func countEntries(archive archiveSource, name string) (int, error) {
	file, err := archive.Open(name, archivesource.Limits.DataFileBytes)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	return decodeYTDArray(file, name, func(struct{}) error { return nil })
}

func newTweet(entry tweetEntry, username string) *tweet {
	t := entry.Tweet
	if strings.TrimSpace(t.IDStr) == "" {
		return nil
	}
	created, _ := time.Parse(time.RubyDate, t.CreatedAt)
	tw := &tweet{
		ID:         t.IDStr,
		Text:       firstNonEmpty(t.FullText, t.Text),
		CreatedAt:  created,
		ReplyToID:  t.InReplyToStatusIDStr,
		URLs:       map[string]string{},
		MediaByTCO: map[string]string{},
		ScreenName: username,
	}
	for _, tag := range t.Entities.Hashtags {
		if strings.TrimSpace(tag.Text) != "" {
			tw.Hashtags = append(tw.Hashtags, tag.Text)
		}
	}
	for _, u := range t.Entities.URLs {
		if u.URL != "" && u.ExpandedURL != "" {
			tw.URLs[u.URL] = u.ExpandedURL
		}
	}
	for _, m := range t.ExtendedEntities.Media {
		if m.URL != "" && m.MediaURLHTTPS != "" {
			tw.MediaByTCO[m.URL] = m.MediaURLHTTPS
		}
	}
	handle := firstNonEmpty(username, "i")
	tw.ExternalURL = "https://twitter.com/" + handle + "/status/" + tw.ID
	return tw
}

// attachMedia associates archive media files (named "<tweetid>-<media>.<ext>")
// with their tweets, and returns how many files matched no tweet.
func attachMedia(tweets []*tweet, names []string) int {
	byID := map[string]*tweet{}
	for _, tw := range tweets {
		byID[tw.ID] = tw
	}
	unmatched := 0
	for _, name := range names {
		idx := strings.Index(name, "-")
		if idx <= 0 {
			unmatched++
			continue
		}
		if tw, ok := byID[name[:idx]]; ok {
			tw.MediaFiles = append(tw.MediaFiles, name)
		} else {
			unmatched++
		}
	}
	return unmatched
}

// recoverThreads assigns thread IDs by walking in-reply-to chains among the
// archived tweets: every tweet's thread root is its highest reachable
// ancestor inside the archive. Replies to other people's tweets (parent not
// archived) start their own thread but keep reply_to pointing at the external
// tweet ID. Returns the number of multi-tweet threads.
func recoverThreads(tweets []*tweet) int {
	byID := map[string]*tweet{}
	for _, tw := range tweets {
		byID[tw.ID] = tw
	}
	rootOf := func(tw *tweet) string {
		seen := map[string]bool{}
		current := tw
		for {
			if seen[current.ID] {
				break
			}
			seen[current.ID] = true
			parent, ok := byID[current.ReplyToID]
			if !ok {
				break
			}
			current = parent
		}
		return current.ID
	}
	counts := map[string]int{}
	for _, tw := range tweets {
		tw.ThreadID = rootOf(tw)
		counts[tw.ThreadID]++
	}
	threads := 0
	for _, n := range counts {
		if n > 1 {
			threads++
		}
	}
	return threads
}

func ensureNotebook(ctx context.Context, st store.Store, name string) (string, error) {
	nb, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: TwitterNotebookID, Name: name, IconEmoji: "🐦"})
	if err == nil {
		return nb.ID, nil
	}
	if errors.Is(err, store.ErrNameConflict) {
		// A notebook with this name already exists; find it.
		notebooks, listErr := st.ListNotebooks(ctx)
		if listErr != nil {
			return "", listErr
		}
		for _, existing := range notebooks {
			if strings.EqualFold(existing.Name, name) && existing.ParentID == "" {
				return existing.ID, nil
			}
		}
	}
	if existing, getErr := st.GetNotebook(ctx, TwitterNotebookID); getErr == nil {
		return existing.ID, nil
	}
	return "", err
}

var whitespaceRE = regexp.MustCompile(`\s+`)

func importTweet(ctx context.Context, st store.Store, tw *tweet, archive archiveSource, mediaDir, notebookID string, options Options, report *Report, display, username string) error {
	docID := "doc_twitter_" + tw.ID
	body, mediaResources, err := buildTweetBody(ctx, st, tw, archive, mediaDir, options.CollectionID, report)
	if err != nil {
		return err
	}
	title := tweetTitle(tw.Text)

	trashed := false
	existing, err := st.GetDocument(ctx, docID)
	switch {
	case err == nil:
		if existing.Title == title && existing.Body == body {
			report.NotesUnchanged++
		} else {
			if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
				ID: docID, Title: title, Body: body, BodyMIMEType: "text/markdown",
				BaseRevisionID: existing.CurrentRevisionID, Message: "import update from Twitter/X archive",
			}); err != nil {
				return err
			}
			report.NotesUpdated++
		}
	case errors.Is(err, store.ErrNotFound):
		// The note may exist but be in the user's Trash; never resurrect a
		// note the user deleted — just refresh its provenance below.
		if _, srcErr := st.FindDocumentBySource(ctx, "twitter", tw.ID); srcErr == nil {
			report.NotesUnchanged++
			trashed = true
			break
		} else if !errors.Is(srcErr, store.ErrNotFound) {
			return srcErr
		}
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: docID, CollectionID: options.CollectionID, NotebookID: notebookID,
			Title: title, Body: body, BodyMIMEType: "text/markdown", Message: "import from Twitter/X archive",
		}); err != nil {
			return err
		}
		report.NotesImported++
	default:
		return err
	}

	published := ""
	if !tw.CreatedAt.IsZero() {
		published = tw.CreatedAt.UTC().Format(time.RFC3339)
	}
	if _, err := st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{
		DocumentID:   docID,
		SourceSystem: "twitter",
		ExternalID:   tw.ID,
		Author:       firstNonEmpty(display, username),
		AuthorID:     "@" + username,
		ThreadID:     tw.ThreadID,
		ReplyTo:      tw.ReplyToID,
		SourceURL:    tw.ExternalURL,
		PublishedAt:  published,
	}); err != nil {
		return err
	}

	if trashed {
		// Tags and attachments require a live note; provenance above is enough.
		return nil
	}
	for _, tag := range tw.Hashtags {
		if _, err := st.AddDocumentTag(ctx, docID, tag); err != nil {
			return err
		}
		report.TagsApplied++
	}
	for _, resourceID := range mediaResources {
		_, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{
			DocumentID: docID, ResourceID: resourceID, RelationType: "embedded",
			AnchorJSON: fmt.Sprintf(`{"twitter_media":%q}`, tw.ID),
		})
		if err == nil {
			report.AttachmentsLinked++
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	return nil
}

// buildTweetBody renders the tweet as Markdown: expanded URLs, imported media
// as resource:// images, and a footer link to the original post.
func buildTweetBody(ctx context.Context, st store.Store, tw *tweet, archive archiveSource, mediaDir, collectionID string, report *Report) (string, []string, error) {
	text := tw.Text
	for tco, expanded := range tw.URLs {
		text = strings.ReplaceAll(text, tco, expanded)
	}
	mediaResources := []string{}
	mediaMarkdown := []string{}
	for _, filename := range tw.MediaFiles {
		resourceID, created, err := importMediaFile(ctx, st, archive, mediaDir, filename, collectionID)
		if err != nil {
			report.MediaMissing++
			report.Warnings = append(report.Warnings, fmt.Sprintf("media %s: %v", filename, err))
			continue
		}
		if created {
			report.MediaImported++
		}
		mediaResources = append(mediaResources, resourceID)
		mediaMarkdown = append(mediaMarkdown, fmt.Sprintf("![%s](%s)", filename, store.ResourceURI(collectionID, resourceID)))
	}
	// Media t.co links remain in the text; drop them since the media is
	// embedded below.
	for tco := range tw.MediaByTCO {
		text = strings.TrimSpace(strings.ReplaceAll(text, tco, ""))
	}

	var b strings.Builder
	b.WriteString(text)
	b.WriteString("\n")
	for _, line := range mediaMarkdown {
		b.WriteString("\n" + line + "\n")
	}
	b.WriteString("\n---\n\n[View on Twitter/X](" + tw.ExternalURL + ")\n")
	return b.String(), mediaResources, nil
}

// importMediaFile streams one media file from the archive into the asset
// store. From a ZIP it is decompressed as it is read; nothing is extracted.
func importMediaFile(ctx context.Context, st store.Store, archive archiveSource, mediaDir, filename, collectionID string) (string, bool, error) {
	resourceID := "res_twitter_" + sanitizeID(filename)
	if _, err := st.GetResource(ctx, resourceID); err == nil {
		return resourceID, false, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", false, err
	}
	file, err := archive.Open(mediaDir+"/"+filename, archivesource.Limits.MediaFileBytes)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	mimeType := firstNonEmpty(store.MIMETypeFromFilename(filename), "application/octet-stream")
	resource, err := st.CreateResource(ctx, store.CreateResourceRequest{
		PreferredID: resourceID, CollectionID: collectionID,
		Filename: filename, MIMEType: mimeType, Content: file,
	})
	if err != nil {
		return "", false, err
	}
	return resource.ID, true, nil
}

func sanitizeID(name string) string {
	name = strings.ToLower(name)
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, name)
	return strings.Trim(mapped, "_")
}

func tweetTitle(text string) string {
	line := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	line = whitespaceRE.ReplaceAllString(line, " ")
	if line == "" {
		return "Tweet"
	}
	runes := []rune(line)
	if len(runes) > 80 {
		return string(runes[:77]) + "…"
	}
	return line
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
