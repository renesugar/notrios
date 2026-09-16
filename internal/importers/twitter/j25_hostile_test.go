package twitter

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J25-B: an archive is untrusted input. These bound what an import reads and
// refuse what escapes it. All fixtures are synthetic.

func j25Lower(t *testing.T, target *int64, value int64) {
	t.Helper()
	previous := *target
	*target = value
	t.Cleanup(func() { *target = previous })
}

// j25WriteZipEntries writes entries in the order given, without cleaning names,
// so an archive can carry names a well-behaved tool would never write.
func j25WriteZipEntries(t *testing.T, entries [][2]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hostile.zip")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for _, entry := range entries {
		file, err := writer.CreateHeader(&zip.FileHeader{Name: entry[0], Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry[1])); err != nil {
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

func TestJ25RefusesAPostFileOverTheSizeLimit(t *testing.T) {
	j25Lower(t, &maxDataFileBytes, 64<<10)
	// A megabyte of one repeated character compresses to almost nothing: the
	// shape of a decompression bomb, at a size a test can afford.
	bomb := j25File("tweets", "part0", j25Post("3001", strings.Repeat("a", 1<<20), ""))
	files := map[string]string{"data/account.js": j25Archive()["data/account.js"], "data/tweets.js": bomb}
	for name, source := range map[string]string{"zip": j25WriteZip(t, files), "folder": j25WriteDir(t, files)} {
		if _, err := Import(context.Background(), newTestStore(t), source, Options{}); !errors.Is(err, errTooLarge) {
			t.Errorf("%s: a post file over the limit must be refused with errTooLarge, got %v", name, err)
		}
	}
}

func TestJ25StreamLimitCatchesAFileLargerThanItsRecordedSize(t *testing.T) {
	j25Lower(t, &maxDataFileBytes, 64<<10)
	body := strings.NewReader(strings.Repeat("b", 1<<20))
	limited := &limitedReadCloser{reader: body, closer: nil, limit: maxDataFileBytes, name: "tweets.js"}
	buffer := make([]byte, 32<<10)
	total := 0
	var err error
	for err == nil {
		var n int
		n, err = limited.Read(buffer)
		total += n
	}
	if !errors.Is(err, errTooLarge) || int64(total) > maxDataFileBytes {
		t.Fatalf("read %d bytes and stopped with %v; want errTooLarge at no more than %d bytes", total, err, maxDataFileBytes)
	}
}

func TestJ25RefusesAnArchiveWithTooManyEntries(t *testing.T) {
	previous := maxArchiveEntries
	maxArchiveEntries = 5
	t.Cleanup(func() { maxArchiveEntries = previous })
	_, err := Import(context.Background(), newTestStore(t), j25WriteZip(t, j25Archive()), Options{DryRun: true})
	if !errors.Is(err, errTooLarge) || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("an archive with more entries than the limit must be refused, got %v", err)
	}
}

func TestJ25RefusesEntryNamesThatEscapeTheArchive(t *testing.T) {
	archive := j25Archive()
	path := j25WriteZipEntries(t, [][2]string{
		{"data/account.js", archive["data/account.js"]},
		{"data/tweets.js", j25File("tweets", "part0", j25Post("3001", "Post with media", ""))},
		{"data/tweets_media/3001-kept.png", "PNGDATA"},
		{"data/tweets_media/../../3001-escaped.png", "ESCAPED"},
		{"/data/tweets_media/3001-absolute.png", "ABSOLUTE"},
		{`data\tweets_media\3001-backslash.png`, "BACKSLASH"},
	})
	ctx := context.Background()
	st := newTestStore(t)
	report, err := Import(ctx, st, path, Options{})
	if err != nil {
		t.Fatalf("an archive with some unsafe names still imports its safe entries: %v", err)
	}
	if report.ArchiveEntriesRejected != 3 {
		t.Errorf("archive_entries_rejected=%d, want 3", report.ArchiveEntriesRejected)
	}
	if report.MediaImported != 1 {
		t.Errorf("media_imported=%d, want only the safely named file", report.MediaImported)
	}
	document, err := st.GetDocument(ctx, "doc_twitter_3001")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"escaped", "absolute", "backslash"} {
		if strings.Contains(document.Body, name) {
			t.Errorf("the note embeds media from an unsafe entry name (%s)", name)
		}
	}
	for _, dir := range []string{filepath.Dir(path), filepath.Dir(filepath.Dir(path))} {
		if matches, _ := filepath.Glob(filepath.Join(dir, "*escaped*")); len(matches) > 0 {
			t.Errorf("an escaping entry was written to disk: %v", matches)
		}
	}
}

func TestJ25DirectoryEntriesWithOddNamesAreNotRefusals(t *testing.T) {
	archive := j25Archive()
	path := j25WriteZipEntries(t, [][2]string{
		{"./", ""},
		{"assets//", ""},
		{"assets/images//", ""},
		{"data/", ""},
		{"data/account.js", archive["data/account.js"]},
		{"data/tweets.js", archive["data/tweets.js"]},
	})
	report, err := Import(context.Background(), newTestStore(t), path, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.ArchiveEntriesRejected != 0 || len(report.Warnings) != 0 {
		t.Fatalf("directory entries hold no content and must not be reported as refused: rejected=%d warnings=%v",
			report.ArchiveEntriesRejected, report.Warnings)
	}
}

func TestJ25SkipsAMediaFileOverTheSizeLimit(t *testing.T) {
	j25Lower(t, &maxMediaFileBytes, 16)
	files := j25Archive()
	files["data/tweets_media/3101-a.png"] = strings.Repeat("P", 64)
	report, err := Import(context.Background(), newTestStore(t), j25WriteZip(t, files), Options{})
	if err != nil {
		t.Fatalf("one oversized media file must not fail the import: %v", err)
	}
	if report.MediaMissing != 1 || report.MediaImported != 0 {
		t.Fatalf("media_missing=%d media_imported=%d, want 1 and 0", report.MediaMissing, report.MediaImported)
	}
	found := false
	for _, warning := range report.Warnings {
		found = found || strings.Contains(warning, errTooLarge.Error())
	}
	if !found {
		t.Fatalf("the skipped media file must be reported with its reason, warnings: %v", report.Warnings)
	}
}

func TestJ25RefusesAFileThatIsNeitherAFolderNorAZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Import(context.Background(), newTestStore(t), path, Options{DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "neither a Twitter/X archive ZIP") {
		t.Fatalf("a file that is not a ZIP must be refused clearly, got %v", err)
	}
}

func TestJ25RefusesAZipWithNoPostFiles(t *testing.T) {
	path := j25WriteZip(t, map[string]string{"data/account.js": j25Archive()["data/account.js"]})
	_, err := Import(context.Background(), newTestStore(t), path, Options{DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "no tweets.js") {
		t.Fatalf("a ZIP without post files must be refused clearly, got %v", err)
	}
}
