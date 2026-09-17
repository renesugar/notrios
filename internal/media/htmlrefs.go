package media

import (
	"html"
	"regexp"
	"strings"
)

// J29: the HTML media references the scanner is responsible for. This is not
// an HTML parser. A pattern finds where a covered element's start tag begins,
// and a small reader follows HTML's attribute rules for that one tag. What the
// scan does not promise is written in SECURITY_AND_MEDIA_POLICY.md.

// htmlMediaAttributes names, per covered element, the attributes whose value
// is a URL a renderer loads as media, and whether the value is a srcset list.
var htmlMediaAttributes = map[string]map[string]bool{
	"img":    {"src": false, "srcset": true},
	"source": {"src": false, "srcset": true},
	"video":  {"src": false, "poster": false},
	"audio":  {"src": false},
	"track":  {"src": false},
	"embed":  {"src": false},
	"object": {"data": false},
	"image":  {"href": false, "xlink:href": false},
}

// htmlMediaTagRE finds the start of a covered element's start tag: the name
// must end at whitespace, "/" or ">", so <imgx> and <images> are not <img>.
var htmlMediaTagRE = regexp.MustCompile(`(?i)<(img|source|video|audio|track|embed|object|image)[\s/>]`)

// htmlReference is one URL found in a covered attribute, with the byte offset
// of the tag that holds it.
type htmlReference struct {
	url    string
	offset int
}

// htmlMediaReferences returns every URL in a covered attribute of a covered
// element, in document order. Entities in values are decoded, and a srcset
// yields each candidate URL.
func htmlMediaReferences(body string) []htmlReference {
	var references []htmlReference
	for _, match := range htmlMediaTagRE.FindAllStringSubmatchIndex(body, -1) {
		element := strings.ToLower(body[match[2]:match[3]])
		covered := htmlMediaAttributes[element]
		for _, attribute := range readAttributes(body[match[3]:]) {
			isSrcset, ok := covered[attribute.name]
			if !ok {
				continue
			}
			value := html.UnescapeString(attribute.value)
			if isSrcset {
				for _, candidate := range srcsetURLs(value) {
					references = append(references, htmlReference{url: candidate, offset: match[0]})
				}
				continue
			}
			references = append(references, htmlReference{url: value, offset: match[0]})
		}
	}
	return references
}

// inlineImageRE is the inline image form the owner named for J29, as a whole
// URI: a base64 data: URI of a PNG, JPEG, GIF, WebP or SVG image. It is not a
// remote reference, so the scanner does not report it. Any other data: URI is
// still reported, and blocked by the data scheme rule.
var inlineImageRE = regexp.MustCompile(`(?i)^data:image/(?:png|jpeg|jpg|gif|webp|svg\+xml);base64,[A-Za-z0-9+/=]+$`)

func isInlineImage(uri string) bool {
	return inlineImageRE.MatchString(uri)
}

type htmlAttribute struct{ name, value string }

// readAttributes reads the attributes of one start tag, beginning just after
// the element name and stopping at the tag's closing ">" (or the end of the
// body). It follows HTML's tokenizer for attributes: names end at whitespace,
// "/", ">" or "="; values are double-quoted, single-quoted, or unquoted up to
// whitespace or ">"; a ">" inside quotes does not end the tag. Names are
// lowercased; a repeated attribute keeps its first value, as HTML does.
func readAttributes(rest string) []htmlAttribute {
	var attributes []htmlAttribute
	seen := map[string]bool{}
	i := 0
	skipSpace := func() {
		for i < len(rest) && isHTMLSpace(rest[i]) {
			i++
		}
	}
	for i < len(rest) {
		skipSpace()
		if i >= len(rest) || rest[i] == '>' {
			break
		}
		if rest[i] == '/' {
			i++
			continue
		}
		start := i
		for i < len(rest) && !isHTMLSpace(rest[i]) && rest[i] != '/' && rest[i] != '>' && rest[i] != '=' {
			i++
		}
		if i == start { // a lone "=" where a name should be; HTML treats it as part of a name
			i++
		}
		name := strings.ToLower(rest[start:i])
		skipSpace()
		value := ""
		if i < len(rest) && rest[i] == '=' {
			i++
			skipSpace()
			if i < len(rest) && (rest[i] == '"' || rest[i] == '\'') {
				quote := rest[i]
				i++
				end := strings.IndexByte(rest[i:], quote)
				if end < 0 {
					value, i = rest[i:], len(rest)
				} else {
					value, i = rest[i:i+end], i+end+1
				}
			} else {
				start := i
				for i < len(rest) && !isHTMLSpace(rest[i]) && rest[i] != '>' {
					i++
				}
				value = rest[start:i]
			}
		}
		if !seen[name] {
			seen[name] = true
			attributes = append(attributes, htmlAttribute{name: name, value: value})
		}
	}
	return attributes
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// srcsetURLs returns the candidate URLs of a srcset value: each candidate is a
// URL up to whitespace, optionally followed by descriptors, separated by
// commas. A URL's own trailing commas separate it from the next candidate.
func srcsetURLs(value string) []string {
	var urls []string
	i := 0
	for i < len(value) {
		for i < len(value) && (isHTMLSpace(value[i]) || value[i] == ',') {
			i++
		}
		start := i
		for i < len(value) && !isHTMLSpace(value[i]) {
			i++
		}
		candidate := value[start:i]
		trimmed := strings.TrimRight(candidate, ",")
		if trimmed != "" {
			urls = append(urls, trimmed)
		}
		if trimmed != candidate {
			continue // the comma ended this candidate; no descriptors follow
		}
		for i < len(value) && value[i] != ',' {
			i++ // descriptors such as "480w" or "2x"
		}
	}
	return urls
}
