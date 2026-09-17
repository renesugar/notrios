package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

// J30: when the sniff is inconclusive, a claimed media type is followed only
// for named types whose signature is in the payload (J8 finding J8-F4).

func j30ftyp(major string, compatible ...string) []byte {
	box := []byte{0, 0, 0, byte(16 + 4*len(compatible)), 'f', 't', 'y', 'p'}
	box = append(box, major...)
	box = append(box, 0, 0, 0, 0)
	for _, brand := range compatible {
		box = append(box, brand...)
	}
	return append(box, make([]byte, 64)...)
}

func j30Fetch(t *testing.T, header string, body []byte) (FetchResult, string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if header != "" {
			w.Header().Set("Content-Type", header)
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()
	quarantine := t.TempDir()
	fetcher := testFetcher(t, server.URL, nil, func(cfg *config.RemoteMediaConfig) { cfg.QuarantineDir = quarantine })
	result := fetcher.Quarantine(context.Background(), QuarantineRequest{URLs: []string{server.URL + "/media"}})[0]
	return result, quarantine
}

func TestJ30InconclusiveMediaStillLocalizeWithTheirSignature(t *testing.T) {
	cases := []struct {
		name, header string
		body         []byte
	}{
		{"svg with xml declaration", "image/svg+xml", []byte(`<?xml version="1.0"?>` + "\n" + `<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		{"svg without declaration", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1"/>`)},
		{"svg with doctype and BOM", "image/svg+xml; charset=utf-8", []byte("\xef\xbb\xbf<?xml version=\"1.0\"?>\n<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"x\">\n<?xml-stylesheet href=\"a.css\"?>\n<svg/>")},
		{"tiff little-endian", "image/tiff", []byte("II*\x00\x08\x00\x00\x00\x00\x00\x00\x00")},
		{"tiff big-endian", "image/tiff", []byte("MM\x00*\x00\x00\x00\x08\x00\x00\x00\x00")},
		{"avif", "image/avif", j30ftyp("avif", "avif", "mif1", "miaf")},
		{"avif by compatible brand", "image/avif", j30ftyp("mif1", "mif1", "avif")},
		{"heic", "image/heic", j30ftyp("heic", "mif1", "heic")},
		{"heif", "image/heif", j30ftyp("mif1", "mif1")},
		{"quicktime ftyp", "video/quicktime", j30ftyp("qt  ", "qt  ")},
		{"quicktime moov atom", "video/quicktime", append([]byte("\x00\x00\x00\x6cmoov"), make([]byte, 64)...)},
		{"mp3 frame", "audio/mpeg", []byte("\xff\xfb\x90\x64\x00\x00\x00\x00\x00\x00")},
		{"flac", "audio/flac", []byte("fLaC\x00\x00\x00\x22\x10\x00")},
		{"flac, x- spelling", "audio/x-flac", []byte("fLaC\x00\x00\x00\x22\x10\x00")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, _ := j30Fetch(t, c.header, c.body)
			want := strings.TrimSpace(strings.SplitN(c.header, ";", 2)[0])
			if result.Status != StatusQuarantined || result.ContentType != want {
				t.Fatalf("%+v; want quarantined as %s", result, want)
			}
		})
	}
}

func TestJ30AClaimTheBytesContradictIsRefused(t *testing.T) {
	cases := []struct {
		name, header string
		body         []byte
	}{
		{"shell text claiming jpeg", "image/jpeg", []byte("#!/bin/sh\nrm -rf /\n")},
		{"csv claiming png", "image/png", []byte("a,b,c\n1,2,3\n")},
		{"text claiming svg without an svg root", "image/svg+xml", []byte("just some text, not an svg\n")},
		{"xml claiming svg with another root", "image/svg+xml", []byte(`<?xml version="1.0"?><rss version="2.0"/>`)},
		{"svg root appearing only later", "image/svg+xml", []byte(`<?xml version="1.0"?><html><svg/></html>`)},
		{"binary claiming svg", "image/svg+xml", []byte("\x00\x01\x02\x03<svg/>")},
		{"random bytes claiming tiff", "image/tiff", []byte("\x00\x01\x02\x03\x04\x05\x06\x07")},
		{"mp4 box claiming avif", "image/avif", j30ftyp("isoz", "isoz")},
		{"random bytes claiming heic", "image/heic", []byte("\x00\x01\x02\x03\x04\x05\x06\x07")},
		{"random bytes claiming quicktime", "video/quicktime", []byte("\x00\x01\x02\x03\x04\x05\x06\x07")},
		{"random bytes claiming mpeg audio", "audio/mpeg", []byte("\x00\x01\x02\x03\x04\x05\x06\x07")},
		{"bad bitrate index claiming mpeg audio", "audio/mpeg", []byte("\xff\xfb\xf0\x64\x00\x00\x00\x00")},
		{"random bytes claiming flac", "audio/flac", []byte("\x00\x01\x02\x03\x04\x05\x06\x07")},
		{"random bytes claiming an unnamed image type", "image/x-canon-cr2", []byte("\x00\x01\x02\x03\x04\x05\x06\x07")},
		{"text claiming video", "video/mp4", []byte("hello, world\n")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, quarantine := j30Fetch(t, c.header, c.body)
			if result.Status != StatusRefused || !strings.Contains(result.Reason, "does not match the claimed") {
				t.Fatalf("%+v; want refused because the bytes contradict %s", result, c.header)
			}
			if entries, err := os.ReadDir(quarantine); err == nil && len(entries) != 0 {
				t.Fatalf("a refused fetch left %d entries in quarantine", len(entries))
			}
		})
	}
}

func TestJ30APositiveSniffStillWins(t *testing.T) {
	result, _ := j30Fetch(t, "image/png", []byte("<html><body>not an image</body></html>"))
	if result.Status != StatusRefused || !strings.Contains(result.Reason, `"text/html"`) {
		t.Fatalf("%+v; a positive text/html sniff must still be refused", result)
	}
	result, _ = j30Fetch(t, "text/plain", pngBytes)
	if result.Status != StatusQuarantined || result.ContentType != "image/png" {
		t.Fatalf("%+v; a positive image sniff wins over a text header", result)
	}
	result, _ = j30Fetch(t, "", []byte("plain text with no header"))
	if result.Status != StatusRefused || !strings.Contains(result.Reason, "is not localizable media") {
		t.Fatalf("%+v; text with no claimed type is refused as before", result)
	}
}

func TestJ30SVGRootIsTheFirstElement(t *testing.T) {
	for text, want := range map[string]bool{
		`<svg/>`:                      true,
		"  \n<svg\n width='1'/>":      true,
		`<svg>`:                       true,
		`<!-- c --><svg/>`:            true,
		`<svgfoo/>`:                   false,
		`<html><svg/></html>`:         false,
		`<?xml version="1.0"`:         false, // declaration never closes
		`<!-- never closes <svg/>`:    false,
		"<!DOCTYPE svg>\n<svg/>":      true,
		"\xef\xbb\xbf<svg/>":          true,
		`text <svg/>`:                 false,
		`<?xml version="1.0"?><SVG/>`: false, // XML names are case-sensitive
	} {
		if got := svgRoot([]byte(text)); got != want {
			t.Errorf("svgRoot(%q) = %t, want %t", text, got, want)
		}
	}
}
