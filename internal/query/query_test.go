package query

import (
	"testing"
	"time"
)

var now = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)

func TestParseOperatorsAndPhrases(t *testing.T) {
	q := Parse(`apples "exact phrase" title:architecture title:"multiple words" notebook:"My Work" tag:toys tag:"shopping mall" author:"Alice Smith" authorid:alice@example.social`, now)

	if len(q.Terms) != 2 || q.Terms[0].Text != "apples" || q.Terms[0].Phrase {
		t.Fatalf("terms wrong: %+v", q.Terms)
	}
	if !q.Terms[1].Phrase || q.Terms[1].Text != "exact phrase" {
		t.Fatalf("phrase term wrong: %+v", q.Terms[1])
	}
	if len(q.Title) != 2 || q.Title[1].Text != "multiple words" || !q.Title[1].Phrase {
		t.Fatalf("title terms wrong: %+v", q.Title)
	}
	if len(q.Notebooks) != 1 || q.Notebooks[0] != "My Work" {
		t.Fatalf("notebooks wrong: %+v", q.Notebooks)
	}
	if len(q.Tags) != 2 || q.Tags[1] != "shopping mall" {
		t.Fatalf("tags wrong: %+v", q.Tags)
	}
	if len(q.Authors) != 1 || q.Authors[0] != "Alice Smith" {
		t.Fatalf("authors wrong: %+v", q.Authors)
	}
	if len(q.AuthorIDs) != 1 || q.AuthorIDs[0] != "alice@example.social" {
		t.Fatalf("authorids wrong: %+v", q.AuthorIDs)
	}
}

func TestParseTimeBounds(t *testing.T) {
	q := Parse("since:2026-07-01 until:2026-07-31", now)
	wantSince := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix()
	wantUntil := time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC).Unix()
	if q.Since != wantSince {
		t.Fatalf("date-only since = start of day: got %d want %d", q.Since, wantSince)
	}
	if q.Until != wantUntil {
		t.Fatalf("date-only until = end of day: got %d want %d", q.Until, wantUntil)
	}

	q = Parse("since:2026-07-13T18:42:07Z", now)
	if q.Since != time.Date(2026, 7, 13, 18, 42, 7, 0, time.UTC).Unix() {
		t.Fatalf("RFC3339 since wrong: %d", q.Since)
	}

	// Zone-less timestamps and time-only values use the selected timezone/day.
	q = Parse("since:14:30:00", now)
	if q.Since != time.Date(2026, 7, 15, 14, 30, 0, 0, time.UTC).Unix() {
		t.Fatalf("time-only since must mean today at that time: %d", q.Since)
	}

	zone := time.FixedZone("UTC+2", 2*3600)
	localNow := time.Date(2026, 7, 15, 10, 0, 0, 0, zone)
	q = Parse("since:2026-07-15", localNow)
	if q.Since != time.Date(2026, 7, 15, 0, 0, 0, 0, zone).UTC().Unix() {
		t.Fatalf("date-only since must use the selected timezone: %d", q.Since)
	}
}

func TestParseFallbacksAndSpecials(t *testing.T) {
	q := Parse("is:trashed", now)
	if !q.Trashed {
		t.Fatal("is:trashed not recognized")
	}
	if Parse("", now).IsEmpty() != true {
		t.Fatal("empty query must be IsEmpty")
	}
	// Unknown operators and URL-ish tokens stay literal terms.
	q = Parse("re:invoice https://example.com/x", now)
	if len(q.Terms) != 2 {
		t.Fatalf("unknown operators must fall back to terms: %+v", q)
	}
	// Trailing colon is not an operator.
	q = Parse("note:", now)
	if len(q.Terms) != 1 || q.Terms[0].Text != "note:" {
		t.Fatalf("trailing colon handling wrong: %+v", q)
	}
}
