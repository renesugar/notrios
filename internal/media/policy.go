// Package media implements the remote-media policy engine
// (SECURITY_AND_MEDIA_POLICY.md). This file is the static half: URL policy
// evaluation and Markdown scanning, which never perform network I/O — not
// even DNS. Quarantine downloads and localization build on it in later
// v0.3 tasks, re-applying the same policy to every redirect hop and to
// resolved addresses at connect time.
package media

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/renesugar/notrios/internal/addressrange"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/markdownlinks"
)

// Actions a policy can produce for one URL.
const (
	ActionAllow  = "allow"
	ActionBlock  = "block"
	ActionReview = "review"
)

// Decision is the policy verdict for one remote-media URL in a note.
type Decision struct {
	URL        string `json:"url"`
	MediaClass string `json:"media_class"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	Line       int    `json:"line,omitempty"`
}

// Policy evaluates URLs against the configured remote-media rules.
type Policy struct {
	cfg config.RemoteMediaConfig
	// addresses is the refused-address set (J28), shared with the fetcher's
	// connect-time check. addressErr is set when the ranges given in code
	// did not parse; the policy then refuses every URL.
	addresses  *addressrange.Rules
	addressErr error
}

// Option configures a Policy or Fetcher.
type Option func(*options)

type options struct {
	security *config.RemoteMediaSecurityConfig
}

// WithAddressRanges applies the configuration's security.remote_media block.
// Every production caller passes it. Without it the default set applies, so
// an omission errs strict rather than open.
func WithAddressRanges(security config.RemoteMediaSecurityConfig) Option {
	return func(o *options) { o.security = &security }
}

func NewPolicy(cfg config.RemoteMediaConfig, opts ...Option) *Policy {
	if !config.MediaActions[cfg.DefaultAction] {
		cfg.DefaultAction = ActionReview
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	policy := &Policy{cfg: cfg}
	if o.security == nil {
		policy.addresses = addressrange.MustDefault()
	} else {
		policy.addresses, policy.addressErr = o.security.Rules()
	}
	return policy
}

// checkAddress applies the refused-address set to one address literal. It is
// the only address check: Evaluate uses it for URL literals and the fetcher
// for every address it is about to dial. It returns "" when the address may
// be reached.
func (p *Policy) checkAddress(addr netip.Addr) string {
	if p.cfg.AllowPrivateNetworks {
		return ""
	}
	if p.addressErr != nil {
		return "the remote-media address ranges are invalid: " + p.addressErr.Error()
	}
	decision := p.addresses.Check(addr)
	if !decision.Refused {
		return ""
	}
	return fmt.Sprintf("address %s is in refused range %s", addr, decision.Range)
}

// Evaluate returns the policy action and a human-readable reason for one URL.
// It is purely static: no DNS resolution, no network access. Address checks
// therefore cover literals only; resolved addresses are re-checked at fetch
// time by the quarantine pipeline.
func (p *Policy) Evaluate(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ActionBlock, "empty URL"
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ActionBlock, "unparseable URL"
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "" {
		return ActionBlock, "missing URL scheme"
	}
	for _, blocked := range p.cfg.BlockedSchemes {
		if scheme == strings.ToLower(blocked) {
			return ActionBlock, "scheme " + scheme + " is blocked by policy"
		}
	}
	if scheme != "http" && scheme != "https" {
		return ActionBlock, "only http(s) URLs can be localized"
	}
	if parsed.Hostname() == "" {
		return ActionBlock, "missing host"
	}
	host, ok := normalizeHost(parsed.Hostname())
	if !ok {
		return ActionBlock, "malformed host"
	}
	if p.addressErr != nil {
		return ActionBlock, "the remote-media address ranges are invalid: " + p.addressErr.Error()
	}
	if !p.cfg.AllowPrivateNetworks && isLocalhostName(host) {
		return ActionBlock, "localhost name " + host + " is refused"
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if reason := p.checkAddress(addr); reason != "" {
			return ActionBlock, reason
		}
	}
	if pattern, ok := matchDomain(host, p.cfg.BlockedDomains); ok {
		return ActionBlock, "domain matches blocked pattern " + pattern
	}
	if pattern, ok := matchDomain(host, p.cfg.AllowedDomains); ok {
		return ActionAllow, "domain matches allowed pattern " + pattern
	}
	if pattern, ok := matchDomain(host, p.cfg.ReviewDomains); ok {
		return ActionReview, "domain matches review pattern " + pattern
	}
	return p.cfg.DefaultAction, "no domain rule matched; policy default"
}

// isLocalhostName reports whether a normalized host is a localhost name
// (RFC 6761). Address literals are decided by the refused-address set;
// hostnames that merely resolve to a refused address cannot be caught here (no
// DNS by design), and the fetch pipeline checks resolved addresses at connect
// time.
func isLocalhostName(host string) bool {
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// normalizeHost returns the one form a host is checked in: lowercased, with a
// single trailing dot removed, since DNS treats "example.org." and
// "example.org" as the same name (J27). It reports false for a host with an
// empty label ("", "a..b", ".a", "a.."), which no resolver treats as a name and
// which must not fall through to the default action. Address literals pass
// through unchanged apart from the dot, so "127.0.0.1." is checked as
// 127.0.0.1. Unicode and IDNA forms are not mapped here.
func normalizeHost(host string) (string, bool) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return "", false
	}
	if net.ParseIP(host) != nil {
		return host, true
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" {
			return "", false
		}
	}
	return host, true
}

// matchDomain matches a normalized host against configured patterns:
// "example.org" matches exactly; "*.example.org" matches any subdomain (not
// the apex). Patterns are normalized like hosts, so "example.org." in a list
// means example.org; a pattern that is malformed once normalized matches
// nothing.
func matchDomain(host string, patterns []string) (string, bool) {
	for _, pattern := range patterns {
		p := strings.TrimSpace(pattern)
		wildcard := strings.HasPrefix(p, "*.")
		if wildcard {
			p = p[2:]
		}
		p, ok := normalizeHost(p)
		if !ok {
			continue
		}
		if wildcard {
			p = "*." + p
		}
		if strings.HasPrefix(p, "*.") {
			if strings.HasSuffix(host, p[1:]) {
				return pattern, true
			}
			continue
		}
		if host == p {
			return pattern, true
		}
	}
	return "", false
}

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".svg": true, ".avif": true, ".bmp": true, ".ico": true, ".tif": true, ".tiff": true,
}

var videoExtensions = map[string]bool{
	".mp4": true, ".webm": true, ".mov": true, ".mkv": true, ".avi": true, ".m4v": true,
	".mp3": true, ".ogg": true, ".oga": true, ".wav": true, ".m4a": true, ".flac": true,
}

// MediaClass maps a URL to the policy size-cap classes (image/video/pdf) or
// "other" when the target type cannot be inferred from the URL alone.
func MediaClass(raw string) string {
	if strings.HasPrefix(strings.ToLower(raw), "data:image/") {
		return "image"
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "other"
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	switch {
	case imageExtensions[ext]:
		return "image"
	case videoExtensions[ext]:
		return "video"
	case ext == ".pdf":
		return "pdf"
	}
	return "other"
}

// EvaluateURLs applies the policy to an explicit URL list (media class
// inferred per URL) — used when a client checks URLs it collected itself.
func (p *Policy) EvaluateURLs(urls []string) []Decision {
	decisions := make([]Decision, 0, len(urls))
	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		action, reason := p.Evaluate(raw)
		decisions = append(decisions, Decision{URL: raw, MediaClass: MediaClass(raw), Action: action, Reason: reason})
	}
	return decisions
}

// ScanBody extracts remote-media URLs from a Markdown body and evaluates
// each against the policy, without downloading anything. Included are:
// embeds/images with any URI scheme (so file:/data: images are flagged as
// blocked instead of silently ignored), plain links whose extension maps to
// a media class, and raw HTML <img src="..."> tags. document:// and
// resource:// URIs are internal and never included. Duplicate URLs are
// reported once, at their first occurrence.
func (p *Policy) ScanBody(body string) []Decision {
	decisions := []Decision{}
	seen := map[string]bool{}

	appendURL := func(raw string, embedded bool, line int) {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			return
		}
		lower := strings.ToLower(raw)
		if strings.HasPrefix(lower, "document://") || strings.HasPrefix(lower, "resource://") {
			return
		}
		hasScheme := strings.Contains(raw, "://") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "mailto:")
		if !hasScheme {
			return // relative path or wiki target, not remote media
		}
		class := MediaClass(raw)
		if !embedded && class == "other" {
			return // plain link to a non-media target
		}
		seen[raw] = true
		action, reason := p.Evaluate(raw)
		decisions = append(decisions, Decision{URL: raw, MediaClass: class, Action: action, Reason: reason, Line: line})
	}

	// markdownlinks splits "#fragment" off RawTarget; policy evaluation only
	// needs scheme/host/path, so the stripped form is sufficient.
	for _, candidate := range markdownlinks.Extract(body) {
		appendURL(candidate.RawTarget, candidate.RelationType == "embed", candidate.Line)
	}
	for _, match := range htmlImgRE.FindAllStringSubmatchIndex(body, -1) {
		src := body[match[2]:match[3]]
		appendURL(src, true, lineOf(body, match[0]))
	}
	return decisions
}

var htmlImgRE = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)

func lineOf(body string, offset int) int {
	if offset > len(body) {
		offset = len(body)
	}
	return 1 + strings.Count(body[:offset], "\n")
}
