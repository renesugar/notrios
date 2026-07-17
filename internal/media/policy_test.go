package media

import (
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

func testPolicy() *Policy {
	cfg := config.Default().RemoteMedia
	cfg.BlockedDomains = []string{"*.example-blocked.invalid", "tracker.example.com"}
	cfg.AllowedDomains = []string{"wikipedia.org", "*.wikimedia.org"}
	cfg.ReviewDomains = []string{"cdn.example.net"}
	return NewPolicy(cfg)
}

func TestEvaluateSchemes(t *testing.T) {
	p := testPolicy()
	cases := []struct {
		url    string
		action string
		reason string
	}{
		{"file:///etc/passwd", ActionBlock, "blocked by policy"},
		{"data:image/png;base64,AAAA", ActionBlock, "blocked by policy"},
		{"ftp://example.com/a.png", ActionBlock, "blocked by policy"},
		{"gopher://example.com/a.png", ActionBlock, "only http(s)"},
		{"a.png", ActionBlock, "missing URL scheme"},
		{"", ActionBlock, "empty URL"},
	}
	for _, tc := range cases {
		action, reason := p.Evaluate(tc.url)
		if action != tc.action || !strings.Contains(reason, tc.reason) {
			t.Errorf("Evaluate(%q) = %q, %q; want %q with reason containing %q", tc.url, action, reason, tc.action, tc.reason)
		}
	}
}

func TestEvaluatePrivateAddresses(t *testing.T) {
	p := testPolicy()
	for _, u := range []string{
		"http://127.0.0.1/x.png",
		"http://10.1.2.3/x.png",
		"http://192.168.0.5/x.png",
		"http://169.254.1.1/x.png",
		"http://[::1]/x.png",
		"http://localhost/x.png",
		"http://internal.localhost/x.png",
		"http://0.0.0.0/x.png",
	} {
		if action, reason := p.Evaluate(u); action != ActionBlock {
			t.Errorf("Evaluate(%q) = %q (%s); private addresses must be blocked", u, action, reason)
		}
	}

	permissive := config.Default().RemoteMedia
	permissive.AllowPrivateNetworks = true
	pp := NewPolicy(permissive)
	if action, _ := pp.Evaluate("http://127.0.0.1/x.png"); action == ActionBlock {
		t.Errorf("allow_private_networks=true must not block loopback")
	}
}

func TestEvaluateDomainLists(t *testing.T) {
	p := testPolicy()
	cases := []struct {
		url    string
		action string
	}{
		{"https://evil.example-blocked.invalid/a.png", ActionBlock},
		{"https://tracker.example.com/pixel.gif", ActionBlock},
		{"https://wikipedia.org/logo.png", ActionAllow},
		{"https://upload.wikimedia.org/a.jpg", ActionAllow},
		{"https://wikimedia.org/a.jpg", ActionReview}, // apex not matched by *.wikimedia.org
		{"https://cdn.example.net/b.png", ActionReview},
		{"https://unknown.example.org/c.png", ActionReview}, // default action
		{"https://WIKIPEDIA.ORG/upper.png", ActionAllow},    // case-insensitive
	}
	for _, tc := range cases {
		if action, reason := p.Evaluate(tc.url); action != tc.action {
			t.Errorf("Evaluate(%q) = %q (%s); want %q", tc.url, action, reason, tc.action)
		}
	}
}

func TestEvaluateBlockedListWinsOverAllowed(t *testing.T) {
	cfg := config.Default().RemoteMedia
	cfg.BlockedDomains = []string{"bad.example.org"}
	cfg.AllowedDomains = []string{"*.example.org"}
	p := NewPolicy(cfg)
	if action, _ := p.Evaluate("https://bad.example.org/a.png"); action != ActionBlock {
		t.Fatalf("blocked list must take precedence over allowed list")
	}
}

func TestNewPolicyNormalizesInvalidDefaultAction(t *testing.T) {
	cfg := config.Default().RemoteMedia
	cfg.DefaultAction = "yolo"
	p := NewPolicy(cfg)
	if action, _ := p.Evaluate("https://unknown.example.org/a.png"); action != ActionReview {
		t.Fatalf("invalid default action must normalize to review, got %q", action)
	}
}

func TestMediaClass(t *testing.T) {
	cases := map[string]string{
		"https://x.org/a.png":         "image",
		"https://x.org/a.JPEG?w=100":  "image",
		"https://x.org/v.mp4":         "video",
		"https://x.org/doc.pdf":       "pdf",
		"https://x.org/page.html":     "other",
		"https://x.org/no-extension":  "other",
		"data:image/png;base64,AAAA":  "image",
		"https://x.org/song.flac":     "video",
		"https://x.org/pic.webp#frag": "image",
	}
	for u, want := range cases {
		if got := MediaClass(u); got != want {
			t.Errorf("MediaClass(%q) = %q, want %q", u, got, want)
		}
	}
}

func TestScanBodyExtraction(t *testing.T) {
	p := testPolicy()
	body := `# Note

![ok](https://upload.wikimedia.org/a.png)
![blocked](https://tracker.example.com/pixel.gif)
![local scheme](file:///etc/passwd)
![internal](resource://default/resources/res_1)
[document link](document://default/documents/doc_1)
[a pdf](https://unknown.example.org/paper.pdf)
[a web page](https://unknown.example.org/page.html)
[relative](notes/other.md)
<img src="https://cdn.example.net/tag.png">
![dup](https://upload.wikimedia.org/a.png)
`
	decisions := p.ScanBody(body)
	byURL := map[string]Decision{}
	for _, d := range decisions {
		byURL[d.URL] = d
	}
	if len(decisions) != 5 {
		t.Fatalf("expected 5 decisions, got %d: %+v", len(decisions), decisions)
	}
	if d := byURL["https://upload.wikimedia.org/a.png"]; d.Action != ActionAllow || d.MediaClass != "image" || d.Line != 3 {
		t.Fatalf("wikimedia image: %+v", d)
	}
	if d := byURL["https://tracker.example.com/pixel.gif"]; d.Action != ActionBlock {
		t.Fatalf("tracker image must be blocked: %+v", d)
	}
	if d := byURL["file:///etc/passwd"]; d.Action != ActionBlock {
		t.Fatalf("file: embed must be surfaced and blocked: %+v", d)
	}
	if d := byURL["https://unknown.example.org/paper.pdf"]; d.Action != ActionReview || d.MediaClass != "pdf" {
		t.Fatalf("pdf link must be reviewed: %+v", d)
	}
	if d := byURL["https://cdn.example.net/tag.png"]; d.Action != ActionReview {
		t.Fatalf("html img must be scanned: %+v", d)
	}
	if _, ok := byURL["https://unknown.example.org/page.html"]; ok {
		t.Fatalf("plain non-media link must not be included")
	}
	if _, ok := byURL["document://default/documents/doc_1"]; ok {
		t.Fatalf("internal URIs must not be included")
	}
}

func TestEvaluateURLs(t *testing.T) {
	p := testPolicy()
	decisions := p.EvaluateURLs([]string{"https://wikipedia.org/a.png", "", "  ", "https://tracker.example.com/x.gif"})
	if len(decisions) != 2 {
		t.Fatalf("blank URLs must be skipped: %+v", decisions)
	}
	if decisions[0].Action != ActionAllow || decisions[1].Action != ActionBlock {
		t.Fatalf("unexpected decisions: %+v", decisions)
	}
}
