# v1.0 J30: stop a lying header deciding the type of an inconclusive payload

J8 found that bytes which sniff as `text/plain`, served with
`Content-Type: image/jpeg`, were quarantined as an image, with `image/jpeg`
recorded and a `.jpg` extension (finding J8-F4). The header fallback exists
for formats Go's sniffer does not know, but it applied to any claimed media
type whenever the sniff was inconclusive.

## What was measured first

`http.DetectContentType` on Go 1.27, one real header per extension the policy
treats as media:

| sniffed | formats |
|---|---|
| positive media type | PNG, JPEG, GIF, WebP, BMP, ICO, MP4, M4V, M4A, WebM, MKV, AVI, WAV, MP3 with ID3, PDF |
| `text/xml` | SVG with an XML declaration |
| `text/plain` | SVG without one; shell text |
| `application/octet-stream` | TIFF, AVIF, HEIC, QuickTime MOV, FLAC, MP3 starting with a frame |
| `text/html` (positive) | SVG that opens with a comment; HTML |
| `application/ogg` (positive, not media) | Ogg |

So seven real formats localized only through the header. A fix that simply
dropped the fallback would have stopped all seven, which the item's boundary
forbids. The last two rows are refused today although the policy lists them
as media. They are recorded as **J33** for an owner decision, not fixed here.

## What changed

`internal/media/claimed.go` names the seven formats, as nine claimed types
with their spellings, together with:
- the inconclusive sniffs each type may override (text for SVG, binary for
  the rest);
- the signature each must carry in the first 512 bytes.

In `fetchOne`, an inconclusive sniff with a claimed media type now either
matches that table and takes the claimed type, as before, or is refused as
`payload sniffed as "text/plain" does not match the claimed content type
"image/jpeg"`. A positive sniff still wins, and a payload with no claimed media
type is still refused as not localizable media.

`SECURITY_AND_MEDIA_POLICY.md` gains a "Content type" section with the table,
and `docs/troubleshooting.md` a row for the new refusal.

## Proof

`internal/media/j30_claimed_type_test.go`:

| test | covers |
|---|---|
| `TestJ30InconclusiveMediaStillLocalizeWithTheirSignature` | 14 real payloads localize with their claimed type recorded: SVG three ways (declaration, bare, BOM with doctype and processing instruction), TIFF both byte orders, AVIF by major and by compatible brand, HEIC, HEIF, MOV by `ftyp` and by `moov` atom, frame-led MP3, FLAC under both spellings |
| `TestJ30AClaimTheBytesContradictIsRefused` | 15 lies are refused with nothing left in quarantine: text as JPEG, PNG and MP4; text or other XML as SVG; an SVG inside HTML; binary as SVG; random bytes as TIFF, HEIC, MOV, MPEG audio and FLAC; an MP4 brand as AVIF; a reserved MPEG bitrate index; an unnamed image type |
| `TestJ30APositiveSniffStillWins` | HTML claiming PNG is refused as `text/html`; a real PNG labelled `text/plain` localizes; text with no header is refused as before |
| `TestJ30SVGRootIsTheFirstElement` | 12 cases for the root-element check, including `<svgfoo>`, unclosed declarations and comments, and case sensitivity |

**All 15 refusal cases fail on the code before this change**: each was
quarantined under the lying type. The 14 localize cases and the
positive-sniff cases pass both before and after, which is the proof that no
format regressed.

J8's probe `TestJ8AServerThatLiesAboutItsContent` needed no edit. Its J8-F4
case now logs `refused (payload sniffed as "text/plain" does not match the
claimed content type "image/jpeg")` and no longer records a finding.

**A limit, recorded rather than hidden.** A signature proves how a file
starts, not that the whole file is valid. A first draft of the tests included
`II*\0` followed by text, claimed as TIFF, and expected a refusal. It
localized: the NUL byte makes the sniff binary, and the bytes do begin with a
TIFF header. The case was removed because no check of the first bytes can
tell it from a TIFF. The resource is still stored under its hash, never
executed, and served with `nosniff`.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry. `validate-scaffold.sh` and
`make g18g-validate` pass. G18a's inventory was regenerated for the
troubleshooting row.

## The archive

Built from `0b47dd5` with the usage guard on and no override:
- **Usage guard:** it checked only Claude, 84% of the five-hour window
  remaining, identified by process ancestry (J24).
- **Package:** `package_release.sh` passed, including `go test ./...`,
  `validate-scaffold.sh`, `make g18g-validate` and every evidence validator.
- **ZIP:** `notrios-v1.0-j30-0b47dd5.zip`, 25,636,599 bytes, 2,660 entries.
  `check_release_zip.py` accepts it.
