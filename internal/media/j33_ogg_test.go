package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J33: Ogg is media (owner decision, 2026-09-17), and an SVG that opens with a
// comment stays refused, because its bytes sniff as text/html.

// oggBytes is the start of an Ogg page, which sniffs as application/ogg.
var oggBytes = append([]byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00"), make([]byte, 64)...)

func TestJ33OggLocalizesAsMedia(t *testing.T) {
	for _, header := range []string{"", "audio/ogg", "application/ogg", "text/plain"} {
		result, quarantine := j30Fetch(t, header, oggBytes)
		if result.Status != StatusQuarantined || result.ContentType != "application/ogg" {
			t.Fatalf("header %q: %+v; want quarantined as application/ogg from its own bytes", header, result)
		}
		entries, err := os.ReadDir(quarantine)
		if err != nil || len(entries) != 1 {
			t.Fatalf("header %q: quarantine holds %v (%v)", header, entries, err)
		}
		if filepath.Ext(entries[0].Name()) != ".ogg" {
			t.Errorf("header %q: quarantined as %s; want a .ogg name", header, entries[0].Name())
		}
	}
}

func TestJ33OggExtensionsAreMedia(t *testing.T) {
	for _, path := range []string{"a.ogg", "a.oga", "a.ogv", "a.opus"} {
		if class := MediaClass("https://a.example/" + path); class != "video" {
			t.Errorf("MediaClass(%s) = %s; want the audio and video class", path, class)
		}
	}
	// A plain link to one is scanned as media, like any other media link.
	decisions := testPolicy().ScanBody("[podcast](https://unknown.example.org/ep.opus)")
	if len(decisions) != 1 || decisions[0].MediaClass != "video" {
		t.Fatalf("decisions = %+v", decisions)
	}
}

func TestJ33AnSVGThatOpensWithACommentStaysRefused(t *testing.T) {
	// Its bytes sniff as text/html, a positive identification, and that rule is
	// what stops HTML claiming to be an image. Refusing this file is the price,
	// and the owner decided to pay it (2026-09-17).
	body := []byte("<!-- drawn by hand -->\n<svg xmlns=\"http://www.w3.org/2000/svg\"/>")
	result, quarantine := j30Fetch(t, "image/svg+xml", body)
	if result.Status != StatusRefused || !strings.Contains(result.Reason, `"text/html"`) {
		t.Fatalf("%+v; want refused because the bytes sniff as text/html", result)
	}
	if entries, err := os.ReadDir(quarantine); err == nil && len(entries) != 0 {
		t.Fatalf("a refused fetch left %d entries in quarantine", len(entries))
	}
	// An SVG that does not open with a comment still localizes (J30).
	result, _ = j30Fetch(t, "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	if result.Status != StatusQuarantined {
		t.Fatalf("%+v; an ordinary SVG must still localize", result)
	}
}

func TestJ33AClaimedOggHeaderDoesNotMakeBytesOgg(t *testing.T) {
	// Ogg is accepted from its own bytes. A header claiming it over an
	// inconclusive sniff is not in J30's table, so it is refused.
	result, _ := j30Fetch(t, "application/ogg", []byte("\x00\x01\x02\x03 not an ogg page"))
	if result.Status != StatusRefused || !strings.Contains(result.Reason, "does not match the claimed") {
		t.Fatalf("%+v; want refused", result)
	}
}

func TestJ33OggIsTheAudioAndVideoClass(t *testing.T) {
	// The class decides the size cap, so an Ogg file must reach the audio and
	// video cap rather than "other", which refuses it.
	if got := classFromMIME("application/ogg"); got != "video" {
		t.Fatalf("classFromMIME(application/ogg) = %q; want the audio and video class", got)
	}
}
