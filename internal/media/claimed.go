package media

import (
	"bytes"
	"encoding/binary"
	"strings"
)

// J30: a Content-Type header is evidence about a payload, not a verdict. When
// the sniff is inconclusive, the fetcher follows a claimed media type only if
// the type is named here and the payload's first bytes carry that format's own
// signature (J8 finding J8-F4). Each type is here because Go's
// http.DetectContentType does not recognise it: measured on Go 1.27 in
// PLAN.md J30.

// claimedType is one media type the fallback may follow.
type claimedType struct {
	// sniffs are the inconclusive sniff results the claim may override.
	sniffs map[string]bool
	// signature reports whether the payload's first bytes are this format.
	signature func(head []byte) bool
}

var (
	binarySniff = map[string]bool{"application/octet-stream": true}
	textSniff   = map[string]bool{"text/xml": true, "text/plain": true}
)

var claimedTypes = map[string]claimedType{
	"image/svg+xml":   {textSniff, svgRoot},
	"image/tiff":      {binarySniff, tiffHeader},
	"image/avif":      {binarySniff, ftypBrand("avif", "avis")},
	"image/heic":      {binarySniff, ftypBrand("heic", "heix", "heim", "heis", "hevc", "hevx", "mif1", "msf1")},
	"image/heif":      {binarySniff, ftypBrand("heic", "heix", "heim", "heis", "hevc", "hevx", "mif1", "msf1")},
	"video/quicktime": {binarySniff, quickTime},
	"audio/mpeg":      {binarySniff, mpegAudioFrame},
	"audio/flac":      {binarySniff, flacHeader},
	"audio/x-flac":    {binarySniff, flacHeader},
}

// claimMatchesPayload reports whether a claimed media type may stand in for an
// inconclusive sniff of head.
func claimMatchesPayload(claimed, sniffed string, head []byte) bool {
	entry, ok := claimedTypes[strings.ToLower(claimed)]
	return ok && entry.sniffs[sniffed] && entry.signature(head)
}

func tiffHeader(head []byte) bool {
	return bytes.HasPrefix(head, []byte("II*\x00")) || bytes.HasPrefix(head, []byte("MM\x00*"))
}

func flacHeader(head []byte) bool {
	return bytes.HasPrefix(head, []byte("fLaC"))
}

// ftypBrand matches an ISO base media file whose ftyp box names one of brands
// as its major brand or among its compatible brands.
func ftypBrand(brands ...string) func([]byte) bool {
	return func(head []byte) bool {
		for _, brand := range ftypBrands(head) {
			for _, want := range brands {
				if brand == want {
					return true
				}
			}
		}
		return false
	}
}

// ftypBrands returns the major and compatible brands of a leading ftyp box, or
// nil when the payload does not start with one.
func ftypBrands(head []byte) []string {
	if len(head) < 16 || string(head[4:8]) != "ftyp" {
		return nil
	}
	size := int(binary.BigEndian.Uint32(head[:4]))
	if size < 16 || size%4 != 0 {
		return nil
	}
	if size > len(head) {
		size = len(head) - len(head)%4
	}
	brands := []string{string(head[8:12])}
	for offset := 16; offset+4 <= size; offset += 4 {
		brands = append(brands, string(head[offset:offset+4]))
	}
	return brands
}

// quickTime matches a QuickTime movie: an ftyp box with the "qt  " brand, or an
// older file that opens directly with a moov, mdat or wide atom.
func quickTime(head []byte) bool {
	if ftypBrand("qt  ")(head) {
		return true
	}
	if len(head) < 8 {
		return false
	}
	switch string(head[4:8]) {
	case "moov", "mdat", "wide":
		return true
	}
	return false
}

// mpegAudioFrame matches an MPEG audio frame header with no ID3 tag in front
// (an ID3 tag already sniffs positively): eleven sync bits, a defined version
// and layer, and a bitrate index that is not the reserved 15.
func mpegAudioFrame(head []byte) bool {
	if len(head) < 4 || head[0] != 0xFF || head[1]&0xE0 != 0xE0 {
		return false
	}
	version := (head[1] >> 3) & 0x03
	layer := (head[1] >> 1) & 0x03
	bitrate := head[2] >> 4
	sampleRate := (head[2] >> 2) & 0x03
	return version != 1 && layer != 0 && bitrate != 0x0F && sampleRate != 0x03
}

// svgRoot matches an SVG document: after an optional byte-order mark,
// whitespace, the XML declaration, processing instructions, comments and a
// doctype, the first element is <svg. Anything else as the first element,
// including HTML that contains an svg, is not an SVG document.
func svgRoot(head []byte) bool {
	rest := bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
	for {
		rest = bytes.TrimLeft(rest, " \t\r\n")
		switch {
		case bytes.HasPrefix(rest, []byte("<?")):
			end := bytes.Index(rest, []byte("?>"))
			if end < 0 {
				return false
			}
			rest = rest[end+2:]
		case bytes.HasPrefix(rest, []byte("<!--")):
			end := bytes.Index(rest, []byte("-->"))
			if end < 0 {
				return false
			}
			rest = rest[end+3:]
		case bytes.HasPrefix(rest, []byte("<!DOCTYPE")), bytes.HasPrefix(rest, []byte("<!doctype")):
			end := bytes.IndexByte(rest, '>')
			if end < 0 {
				return false
			}
			rest = rest[end+1:]
		default:
			if !bytes.HasPrefix(rest, []byte("<svg")) {
				return false
			}
			if len(rest) == 4 {
				return true // the head ended right after the element name
			}
			switch rest[4] {
			case ' ', '\t', '\r', '\n', '>', '/':
				return true
			}
			return false // <svgfoo is another element
		}
	}
}
