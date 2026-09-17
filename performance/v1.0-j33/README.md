# v1.0 J33: Ogg localizes; an SVG that opens with a comment does not

J30 measured Go's content sniffer over every media extension and found two
files the policy calls media but the fetcher refuses. Neither was fixed there,
because each needed an owner decision.

## The decisions (owner, 2026-09-17)

| case | decision |
|---|---|
| Ogg, sniffed `application/ogg` | **localizes**, as the audio and video class |
| SVG that opens with a comment, sniffed `text/html` | **stays refused** |

The second is a deliberate cost. The rule that a positive sniff beats the
header is what stops HTML claiming to be an image, and an exception to it is
the kind of narrow gap that gets used. A file that opens with `<!-- … -->`
before its `<svg>` is refused, and the policy now says so rather than leaving
it to be rediscovered.

## What changed

- `classFromMIME` maps `application/ogg` to the audio and video class, so an
  Ogg file's size cap is the video cap rather than "other", which refused it.
- `extensionForMIME` prefers `.ogg` for the quarantine file. The default from
  the MIME table is `.ogx`, which no one recognises.
- `.ogv` and `.opus` join the media extensions beside `.ogg` and `.oga`, so a
  plain link to either is scanned as media. Both are Ogg media, and an Opus
  file sniffs as `application/ogg` too.
- `SECURITY_AND_MEDIA_POLICY.md` records both decisions.

Nothing in J30's claimed-type table changed. A header *claiming*
`application/ogg` over an inconclusive sniff is still refused: Ogg is accepted
from its bytes, never from a claim.

## Proof

`internal/media/j33_ogg_test.go`:

| test | covers |
|---|---|
| `TestJ33OggLocalizesAsMedia` | an Ogg page quarantines as `application/ogg` with no header, a matching header, a wrong header (`text/plain`) and an `audio/ogg` header, and the quarantine file is named `.ogg` |
| `TestJ33OggExtensionsAreMedia` | `.ogg`, `.oga`, `.ogv` and `.opus` are the audio and video class, and a plain link to an `.opus` file is scanned |
| `TestJ33AClaimedOggHeaderDoesNotMakeBytesOgg` | random bytes served as `application/ogg` are refused |
| `TestJ33AnSVGThatOpensWithACommentStaysRefused` | it is refused for sniffing as `text/html`, nothing is left in quarantine, and an ordinary SVG still localizes |
| `TestJ33OggIsTheAudioAndVideoClass` | the class an Ogg file's size cap comes from |

The three Ogg tests fail on the code before this change: an Ogg file was
refused as "not localizable media", and `.ogv` and `.opus` were not media. The
comment-led SVG test passes before and after; it is the guard that the decision
to keep refusing is a decision and not a regression.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry. `validate-scaffold.sh` and
`make g18g-validate` pass.
