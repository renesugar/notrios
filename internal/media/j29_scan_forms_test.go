package media

import (
	"sort"
	"strings"
	"testing"
)

// J29: the scanner finds every covered HTML media reference (J8 finding
// J8-F3), and does not report inline data: URIs, which are not remote.

func j29URLs(decisions []Decision) []string {
	urls := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		urls = append(urls, decision.URL)
	}
	sort.Strings(urls)
	return urls
}

func TestJ29EveryCoveredElementAndAttributeIsFound(t *testing.T) {
	cases := map[string]string{
		`<img src=https://a.example/unquoted.png>`:                            "https://a.example/unquoted.png",
		`<img src='https://a.example/single.png'>`:                            "https://a.example/single.png",
		`<IMG SRC="https://a.example/upper.png">`:                             "https://a.example/upper.png",
		`<img alt="a > b" src="https://a.example/after-gt.png">`:              "https://a.example/after-gt.png",
		"<img\n  alt=\"x\"\n  src=\"https://a.example/lines.png\"\n>":         "https://a.example/lines.png",
		`<img srcset="https://a.example/1x.png">`:                             "https://a.example/1x.png",
		`<picture><source srcset="https://a.example/picture.webp"></picture>`: "https://a.example/picture.webp",
		`<video><source src="https://a.example/source.mp4" type="video/mp4">`: "https://a.example/source.mp4",
		`<video src="https://a.example/video.mp4"></video>`:                   "https://a.example/video.mp4",
		`<video poster="https://a.example/poster.jpg"></video>`:               "https://a.example/poster.jpg",
		`<audio src="https://a.example/audio.mp3"></audio>`:                   "https://a.example/audio.mp3",
		`<video><track src="https://a.example/captions.vtt"></video>`:         "https://a.example/captions.vtt",
		`<embed src="https://a.example/embed.pdf">`:                           "https://a.example/embed.pdf",
		`<object data="https://a.example/object.pdf"></object>`:               "https://a.example/object.pdf",
		`<svg><image href="https://a.example/svg-image.png"/></svg>`:          "https://a.example/svg-image.png",
		`<svg><image xlink:href="https://a.example/xlink.png"/></svg>`:        "https://a.example/xlink.png",
		`<img src="https://a.example/q.png?a=1&amp;b=2">`:                     "https://a.example/q.png?a=1&b=2",
	}
	policy := testPolicy()
	for body, want := range cases {
		got := j29URLs(policy.ScanBody(body))
		if len(got) != 1 || got[0] != want {
			t.Errorf("ScanBody(%q) = %v; want [%s]", body, got, want)
		}
	}
}

func TestJ29SrcsetCandidatesAreEachFound(t *testing.T) {
	body := `<img src="https://a.example/base.png" srcset="https://a.example/small.png 480w, https://a.example/large.png 1080w,https://a.example/2x.png 2x">`
	got := strings.Join(j29URLs(testPolicy().ScanBody(body)), " ")
	want := "https://a.example/2x.png https://a.example/base.png https://a.example/large.png https://a.example/small.png"
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestJ29ReferencesAreEvaluatedAtTheirTagsLine(t *testing.T) {
	body := "# Note\n\ntext\n<video\n  poster=\"https://tracker.example.com/p.gif\"\n  src=\"https://unknown.example.org/v.mp4\">\n"
	decisions := testPolicy().ScanBody(body)
	if len(decisions) != 2 {
		t.Fatalf("decisions = %+v", decisions)
	}
	for _, decision := range decisions {
		if decision.Line != 4 {
			t.Errorf("%s at line %d; want the tag's line 4", decision.URL, decision.Line)
		}
		if strings.Contains(decision.URL, "tracker.example.com") && decision.Action != ActionBlock {
			t.Errorf("a blocked poster must be evaluated like any reference: %+v", decision)
		}
	}
}

func TestJ29UncoveredAndNonMatchingFormsAreNotReported(t *testing.T) {
	for _, body := range []string{
		`<iframe src="https://a.example/page.html"></iframe>`,
		`<div style="background:url(https://a.example/bg.png)"></div>`,
		`<link rel="icon" href="https://a.example/favicon.ico">`,
		`<a href="https://a.example/page.html">link</a>`,
		`<imgx src="https://a.example/not-img.png">`,
		`<img data-src="https://a.example/lazy.png">`,
		`<img src="relative/path.png">`,
	} {
		if got := testPolicy().ScanBody(body); len(got) != 0 {
			t.Errorf("ScanBody(%q) = %+v; this form is not covered", body, got)
		}
	}
}

func TestJ29InlineDataURIsAreNotRemote(t *testing.T) {
	body := strings.Join([]string{
		"![inline](data:image/png;base64,iVBORw0KGgo=)",
		`<img src="data:image/png;base64,iVBORw0KGgo=">`,
		`<img src=data:image/gif;base64,R0lGODlhAQABAAAAACw=>`,
		`<svg viewBox="0 0 1 1"><rect width="1" height="1"/><image href="data:image/png;base64,AAAA"/></svg>`,
		`<video poster="data:image/png;base64,AAAA"></video>`,
		"![local](file:///etc/passwd)",
	}, "\n")
	decisions := testPolicy().ScanBody(body)
	if len(decisions) != 1 || decisions[0].URL != "file:///etc/passwd" || decisions[0].Action != ActionBlock {
		t.Fatalf("decisions = %+v; want only the file: reference, blocked", decisions)
	}
	for _, image := range []string{"png", "jpeg", "jpg", "gif", "webp", "svg+xml"} {
		if got := testPolicy().ScanBody("![x](data:image/" + image + ";base64,AAAA)"); len(got) != 0 {
			t.Errorf("a base64 %s image is inline: %+v", image, got)
		}
	}
	// Only the named base64 image form is inline; any other data: URI is still
	// reported, and blocked by the data scheme rule.
	for _, other := range []string{
		"data:text/html;base64,PGgxPg==",
		"data:image/bmp;base64,Qk0=",
		"data:image/svg+xml,<svg/>",
		"data:image/png;base64,not base64!",
	} {
		got := testPolicy().ScanBody(`<img src="` + other + `">`)
		if len(got) != 1 || got[0].Action != ActionBlock {
			t.Errorf("%s is not an inline image and must be reported blocked: %+v", other, got)
		}
	}
	if action, _ := testPolicy().Evaluate("data:image/png;base64,AAAA"); action != ActionBlock {
		t.Error("an explicitly checked data: URL is still refused: nothing fetches it")
	}
}

func TestJ29MalformedTagsNeitherPanicNorInvent(t *testing.T) {
	for body, want := range map[string]string{
		`<img`:                                  "",
		`<img src=`:                             "",
		`<img src="https://a.example/open.png`:  "https://a.example/open.png",
		`<img =x src=https://a.example/eq.png>`: "https://a.example/eq.png",
		`<img src="https://a.example/first.png" src="https://a.example/second.png">`: "https://a.example/first.png",
		`<img/src="https://a.example/slash.png">`:                                    "https://a.example/slash.png",
		`<img srcset=", ,, ">`:   "",
		`<video poster>`:         "",
		"<img src=\"\x00\xff\">": "",
	} {
		got := j29URLs(testPolicy().ScanBody(body))
		if (want == "" && len(got) != 0) || (want != "" && (len(got) != 1 || got[0] != want)) {
			t.Errorf("ScanBody(%q) = %v; want %q", body, got, want)
		}
	}
}

func FuzzJ29HTMLMediaReferences(f *testing.F) {
	for _, seed := range []string{`<img src=x>`, `<video poster="a" src='b'>`, `<img srcset="a 1x, b 2x">`, `<image xlink:href=c/>`, `<img`, `<source srcset=",">`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		for _, reference := range htmlMediaReferences(body) {
			if reference.offset < 0 || reference.offset >= len(body) {
				t.Fatalf("offset %d outside a %d-byte body", reference.offset, len(body))
			}
		}
	})
}
