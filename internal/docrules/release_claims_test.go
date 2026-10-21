package docrules_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// v1.0 J9-D: no sentence remains that J1, J2 or J3 made untrue.
//
// Each of these was true when it was written, and each was written to be
// honest: the installation page said nothing was signed because nothing was,
// and that a packaged install could not delete its data because it could not.
// J1 built the package in a workflow and attested it, J2 signed it, and J3
// shipped `notriosctl purge`. After that the same sentences warned readers off
// things that now work, and the page contradicted itself further down.
//
// A sentence that has gone false is not found by reading, so they are listed by
// phrase. Each phrase carries the sentence it was found in, and the test checks
// the phrase still matches that sentence: a phrase that no longer matches
// anything would pass for ever and guard nothing.
var untrueReleaseClaims = []struct {
	phrase   string
	madeBy   string
	original string
}{
	{`nothing is signed yet`, "J2",
		"**Nothing is signed yet.** The signing policy is decided"},
	{`no key has been created`, "J2",
		"and no key has been created, so there is no signature to check"},
	{`there is no packaged way to delete your data`, "J3",
		"**There is no packaged way to delete your data, and you should know that before you need it.**"},
	{`the package does not ship it`, "J3",
		"lives in the repository, and the package does not ship it."},
	{"deleting those is `make purge` and nothing else", "J3",
		"anything `make install` wrote: deleting those is `make purge` and nothing else."},
	{`there is no published download yet`, "J1",
		"**There is no published download yet.** The package is built from this repository"},
	{`has not been released anywhere`, "J1",
		"The package is built from this repository and has not been released anywhere"},
	{`it is \*\*not published anywhere\*\* and is not a supported release`, "J1",
		"There is an internal Ubuntu package. It is **not published anywhere** and is not a supported release;"},
	{`there are no official prebuilt binaries`, "J1",
		"There are no official prebuilt binaries yet; Notrios is built from source"},
}

// spaced folds a document's line breaks and runs of spaces into single spaces,
// so a phrase matches however the paragraph happened to wrap.
var spaced = regexp.MustCompile(`\s+`)

func untrueClaimPattern(phrase string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)` + strings.ReplaceAll(phrase, " ", `\s+`))
}

func TestEachUntrueClaimStillMatchesWhereItWasFound(t *testing.T) {
	for _, claim := range untrueReleaseClaims {
		if !untrueClaimPattern(claim.phrase).MatchString(claim.original) {
			t.Errorf("%q no longer matches the sentence it was found in: %q", claim.phrase, claim.original)
		}
	}
}

// TestNoSentenceTheReleaseMadeUntrue covers what a reader of the release reads:
// the published documentation, which the site and the package's built-in help
// are built from, and the README.
func TestNoSentenceTheReleaseMadeUntrue(t *testing.T) {
	root := filepath.Join("..", "..")
	files := []string{filepath.Join(root, "README.md")}
	err := filepath.Walk(filepath.Join(root, "docs"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking docs: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("only %d documents found; the walk has stopped seeing them", len(files))
	}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := spaced.ReplaceAllString(string(body), " ")
		relative, _ := filepath.Rel(root, path)
		for _, claim := range untrueReleaseClaims {
			if untrueClaimPattern(claim.phrase).MatchString(text) {
				t.Errorf("%s says %q, which %s made untrue", filepath.ToSlash(relative), claim.phrase, claim.madeBy)
			}
		}
	}
}
