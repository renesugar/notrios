# Security and Media Policy

Remote-media localization must not be a blind downloader.

Implementation status (v0.3): the policy configuration (H1), the static
scan (H2), the quarantine fetch pipeline (H3, `internal/media.Fetcher`),
and localization (H4, `internal/localize`) are implemented — URL
validation, scheme/domain checks re-applied per redirect hop, connect-time
private-address blocking (post-DNS, so rebinding is caught), streaming
size caps, MIME sniffing that a lying header cannot override, exact
SHA-256, exact-hash rule checks (`media_hash_rules`) before admission to
the content-addressed store, Markdown rewriting to `resource://` in a new
revision with a base-revision precondition, and per-attempt records in
`media_policy_decisions` (quarantined → admitted/refused). One engine
serves REST, `notriosctl localize`, the MCP editor tool, the GUI inspector
action, and the importers' `--localize-media` flag. H5 adds exact-reference
reports and wires a perceptual-hash hook that is inert by default.

## Required pipeline

```text
remote URL in Markdown
    ↓
normalize and validate URL
    ↓
check scheme/domain stop list
    ↓
fetch into quarantine only
    ↓
apply size, timeout, redirect, MIME, and private-network rules
    ↓
compute exact hash
    ↓
compute perceptual hash where supported
    ↓
check local policy databases
    ↓
if allowed, admit to content-addressed store
    ↓
rewrite note and record provenance
```

## Policy requirements

- Block `file:`, `data:`, and other unsafe schemes by default.
- Refuse the reserved address ranges below by default, at the static URL check and at connect time.
- Apply domain policy to every redirect hop.
- Keep exact-hash and perceptual-hash policy hooks separate.
- Use exact hashes for deduplication.
- Use perceptual hashes for moderation signals and near-duplicate suggestions; do not silently collapse perceptual matches.
- Store provenance: original URL, final URL, retrieved timestamp, content type, hashes, and policy decision.

## What the scan covers

The static scan (`ScanBody`) finds the references a note can make a renderer
load, so that every one is evaluated before anything is fetched:

- Markdown embeds, and Markdown links whose extension is media;
- these HTML elements and attributes, in any case, with double-quoted,
  single-quoted or unquoted values, across lines, and with entities decoded
  (J29):

| element | attributes |
|---|---|
| `img` | `src`, `srcset` |
| `source` | `src`, `srcset` |
| `video` | `src`, `poster` |
| `audio` | `src` |
| `track` | `src` |
| `embed` | `src` |
| `object` | `data` |
| `image` (SVG) | `href`, `xlink:href` |

Each `srcset` candidate is a separate reference. `document://` and
`resource://` URIs are internal and not reported. **An inline image is not
remote** (owner direction, J29): a base64 `data:` URI of a PNG, JPEG, GIF, WebP
or SVG image — `data:image/(png|jpeg|jpg|gif|webp|svg+xml);base64,…` — is not
reported, and inline `<svg>` markup is not a reference. Any other `data:` URI,
and any `file:` URI, is reported and blocked.

**What the scan does not promise.** It is a pattern that finds covered start
tags and a small reader for their attributes, not an HTML parser, so:

- it does not follow CSS (`style` attributes, `<style>`, `url()`), documents
  (`iframe`, `frame`, `srcdoc`), `link` preloads and icons, `meta` refresh,
  `input type=image`, or sources a script or renderer extension builds;
- HTML a renderer would read differently from the text, such as a tag split by
  a comment, may be missed;
- HTML inside a code span or fence is reported although it does not render,
  which errs towards reporting;
- a URL written with entities is evaluated decoded, but localization rewrites
  only literal occurrences, so it is reported and not rewritten.

Keeping remote references from loading in the preview is the preview's job,
not the scan's (J34).

## Content type

The type recorded for a fetched resource comes from its bytes. The first 512
bytes are sniffed, and a positive identification always wins over the
server's `Content-Type` header. When the sniff is inconclusive (plain text,
generic XML, or unrecognised binary), the header is followed **only** for the
types below, and only when the bytes carry that format's signature (J30). Any
other claimed media type, or a payload without the signature, is refused as
`payload sniffed as … does not match the claimed content type …`, and nothing
is left in quarantine.

| claimed type | inconclusive sniff it may follow | signature |
|---|---|---|
| `image/svg+xml` | `text/xml`, `text/plain` | the first element, after an optional BOM, whitespace, XML declaration, processing instructions, comments and doctype, is `<svg` |
| `image/tiff` | `application/octet-stream` | `II*\0` or `MM\0*` |
| `image/avif` | `application/octet-stream` | an `ftyp` brand `avif` or `avis` |
| `image/heic`, `image/heif` | `application/octet-stream` | an `ftyp` brand in `heic heix heim heis hevc hevx mif1 msf1` |
| `video/quicktime` | `application/octet-stream` | an `ftyp` brand `qt  `, or a `moov`, `mdat` or `wide` atom first |
| `audio/mpeg` | `application/octet-stream` | an MPEG audio frame header (for files without an ID3 tag, which sniffs positively) |
| `audio/flac`, `audio/x-flac` | `application/octet-stream` | `fLaC` |

These are the localizable formats Go's content sniffer does not recognise,
measured on Go 1.27 (`PLAN.md` J30). A signature proves the start of a file,
not the whole file; the bytes are stored under their hash, never executed, and
served with `nosniff`.

## Reserved address ranges

Remote media refuses the ranges below unless
`remote_media.allow_private_networks` is true (J28). One helper
(`internal/addressrange`) applies them in both places an address is checked:
an address literal in a URL, and the concrete address a host name resolves
to when a connection is made, on every redirect hop.

**The default set** was checked against IANA's IPv4 and IPv6 Special-Purpose
Address Registries (both last updated 2025-10-09) on 2026-09-16, with
RFC 5771 for IPv4 multicast, RFC 4291 for IPv6 multicast and IPv4-compatible
addresses, and RFC 3879 for site-local. Notrios does not fetch the registries
at run time.

| IPv4 range | purpose | source |
|---|---|---|
| `0.0.0.0/8` | "this network"; `0.0.0.0` reaches the local host | RFC 791, RFC 1122 |
| `10.0.0.0/8` | private use | RFC 1918 |
| `100.64.0.0/10` | shared address space, carrier-grade NAT | RFC 6598 |
| `127.0.0.0/8` | loopback | RFC 1122 |
| `169.254.0.0/16` | link-local, including cloud metadata | RFC 3927 |
| `172.16.0.0/12` | private use | RFC 1918 |
| `192.0.0.0/24` | IETF protocol assignments | RFC 6890 |
| `192.0.2.0/24` | documentation, TEST-NET-1 | RFC 5737 |
| `192.88.99.0/24` | deprecated 6to4 relay anycast | RFC 7526 |
| `192.168.0.0/16` | private use | RFC 1918 |
| `198.18.0.0/15` | benchmarking | RFC 2544 |
| `198.51.100.0/24` | documentation, TEST-NET-2 | RFC 5737 |
| `203.0.113.0/24` | documentation, TEST-NET-3 | RFC 5737 |
| `224.0.0.0/4` | multicast | RFC 5771 |
| `240.0.0.0/4` | reserved; contains `255.255.255.255` | RFC 1112, RFC 919 |

| IPv6 range | purpose | source |
|---|---|---|
| `::/128` | unspecified | RFC 4291 |
| `::1/128` | loopback | RFC 4291 |
| `64:ff9b:1::/48` | local-use NAT64 | RFC 8215 |
| `100::/64` | discard-only | RFC 6666 |
| `100:0:0:1::/64` | dummy prefix | RFC 9780 |
| `2001::/32` | Teredo | RFC 4380 |
| `2001:2::/48` | benchmarking | RFC 5180 |
| `2001:10::/28` | deprecated ORCHID | RFC 4843 |
| `2001:db8::/32` | documentation | RFC 3849 |
| `2002::/16` | 6to4 | RFC 3056 |
| `3fff::/20` | documentation | RFC 9637 |
| `5f00::/16` | SRv6 SIDs | RFC 9602 |
| `fc00::/7` | unique local, including cloud metadata | RFC 4193 |
| `fe80::/10` | link-local | RFC 4291 |
| `fec0::/10` | deprecated site-local | RFC 3879 |
| `ff00::/8` | multicast | RFC 4291 |

`localhost` and `*.localhost` names are refused at the static check as well
(RFC 6761).

**IPv4 carried inside IPv6.** An address in `::ffff:0:0/96` (IPv4-mapped),
`::/96` (deprecated IPv4-compatible) or `64:ff9b::/96` (the NAT64 well-known
prefix) is refused when its IPv6 form matches an IPv6 entry or its embedded
IPv4 address matches an IPv4 entry. The three prefixes are not entries: on an
IPv6-only network with DNS64, every IPv4-only media host resolves into
`64:ff9b::`, so refusing the prefix outright would stop localization there,
while checking the embedded address still refuses `64:ff9b::7f00:1`, which is
`127.0.0.1`. 6to4 and Teredo are refused whole.

**Deliberately not refused.** Blocks the registries mark globally reachable
are not reserved against the internet: PCP and TURN anycast (`192.0.0.9`,
`192.0.0.10`, `2001:1::1` to `2001:1::3`), AMT (`192.52.193.0/24`,
`2001:3::/32`), the AS112 blocks, ORCHIDv2 (`2001:20::/28`) and drone remote ID
(`2001:30::/28`). An owner who wants them refused lists them.

**Stating the set** (owner decisions D1–D9, recorded in `PLAN.md` J28):

- The block is `security.remote_media`, a top-level `security:` section whose
  subsection names the surface it governs. It governs remote-media fetches
  only; sync dials the peer the owner configured and is not subject to it.
- `refused_address_ranges`, when stated, **replaces** the default set. Service
  start and `notriosctl config show` warn, naming every default range the
  stated list omits; `[]` refuses nothing.
- `permitted_address_ranges` holds exceptions, empty by default. An exception
  reaches its IPv4-mapped, IPv4-compatible and NAT64 forms too. No exception
  may include loopback (`127.0.0.0/8`, `::1`) or "this host" (`0.0.0.0/8`,
  `::`), in any form: reaching them needs `allow_private_networks`, because a
  loopback exception would make the service fetch from its own REST, MCP and
  sync endpoints on a URL a note supplied.
- An entry that is not an address or a CIDR range, a range with host bits set,
  an IPv6 zone, or a list key with nothing under it fails configuration
  loading, naming the entry: dropping an entry from a refusal list would
  silently widen what may be fetched.
- `allow_private_networks: true` keeps its meaning and switches the refused
  set, its exceptions and the `localhost` check off at both checks.

## Preview rule

Preview components may detect remote images and offer actions, but the server performs localization. Browser image loading must not be treated as a safe content source.

The GUI preview never lets a note's HTML fetch remote media (J34). Every URL
that would load media — `img` `src`, `video` and `audio` `src` and `poster`,
`track` `src`, and `href`/`xlink:href` on SVG elements other than links — is
handled one way:
- a `resource://` source is resolved to its local content URL;
- an inline `data:image/…` source is kept, because inline images are not
  remote;
- an `http:` or `https:` source is moved to an inert `data-remote-…`
  attribute until localization rewrites it;
- anything else is removed.

`img` `srcset`, `source`, `embed` and `object` are removed. Inline SVG markup
renders. The UI's Content-Security-Policy also states `media-src 'self'`, so a
remote audio or video source is refused by the browser even if the sanitizer
missed one. `img-src` still admits remote origins; the sanitizer is what keeps
a note from using that.

## Stop lists and hash checks

Remote-media localization must support:

- configured blocked/allowed/review domain patterns, matched against one normalised host form (lowercase, one trailing dot removed) so a DNS-equivalent spelling meets the same rule, with a host that has an empty label refused as malformed;
- redirect-chain policy checks;
- blocked schemes and private-network protections;
- quarantine fetches before admission to the resource store;
- exact content hashes for deduplication and blocking;
- optional perceptual-hash hooks for moderated content and near-duplicate review.

Perceptual hashes must not be used to silently deduplicate bytes; exact hashes deduplicate blobs, perceptual hashes raise similarity/moderation signals.

## Perceptual-hash hook contract

Notrios defines `store.PerceptualHashHook` but ships no pHash, dHash,
blockhash, or other implementation. An embedding application may install one
on `SQLiteStore` before admitting resources:

- `Algorithm` returns a stable lowercase identifier distinct from `sha256`.
- `SupportsMIME` explicitly selects content the hook can parse.
- `Compute` receives a read-only blob stream and returns one canonical
  lowercase hash. Hook errors fail that resource admission rather than silently
  claiming the blob was checked.
- `SuggestNearDuplicates` receives stored hashes for that algorithm and returns
  candidate blob pairs plus an algorithm-specific non-negative distance.

The store computes at most once per exact blob/algorithm, persists the value in
`resource_hashes`, and reuses it for logical resources sharing those exact
bytes. It checks `media_hash_rules` during admission, but perceptual rules may
only yield `review`; `AddMediaHashRule` rejects perceptual `block` rules.
Returned candidate pairs are validated against the supplied blob set,
canonicalized, and deduplicated before appearing in the resource report.

The hook has no store mutation capability and no authority to admit, reject,
merge, rewrite, hide, delete, or garbage-collect data. Exact SHA-256 remains
the only deduplication identity. With no installed hook, admission behavior is
unchanged and the perceptual report is empty with `hook_enabled: false`.

Reports are available from
`GET /api/v1/resources/reports/reference` and
`notriosctl resources report`. The report is advisory and read-only; H6 owns
retention and deletion policy.

## Resource retention and collection

H6 garbage collection is conservative and dry-run first:

- retention begins when the final reference disappears, not when bytes were
  originally uploaded;
- unattached/detached resources and resources orphaned by permanent note purge
  have independently configured windows;
- Trash references remain real references and therefore block collection;
- every apply candidate is rechecked under an immediate SQLite transaction;
- a logical resource may be removed while a shared exact blob remains for
  another logical resource;
- physical bytes and stored perceptual hashes are removed only after the final
  logical resource for that exact blob is gone;
- filesystem paths loaded from the database are constrained beneath the asset
  root before unlinking.

`store.RetentionGate` separates time eligibility from future synchronization
safety. v0.3 uses the local gate; v0.7 must also require peer acknowledgement
watermarks before allowing replicated resources, blobs, or tombstones to be
collected.

REST exposes only the dry-run report. CLI apply requires the explicit
`--apply` flag. Existing immediate resource deletion and permanent note purge
require object-specific `X-Notrios-Confirmation` headers.
