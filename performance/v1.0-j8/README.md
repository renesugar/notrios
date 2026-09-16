# v1.0 J8: security review of remote media and MCP

This is a review. It records what was examined, what was attempted, and what
happened. **Nothing here is a claim that these surfaces are secure**, and no
product code changed: by owner decision (2026-09-16) J8 records findings and
defers each fix to its own plan item.

The probes are in this directory and run with the ordinary test suite. Each one
states what it attempted and prints what the shipped code did; a probe that
finds a gap records it and continues, so the suite stays green while the gap
stays visible.

## What was examined

| | |
|---|---|
| **Remote media** | `internal/media` (policy evaluation, quarantine fetch), `internal/localize` (admission to the resource store and the note rewrite), and the store's attempt recording |
| **MCP** | `internal/httpapi/mcp*.go`: the tool registry, the scope ladder, scope enforcement, and the withheld operations |
| **Against** | `SECURITY_AND_MEDIA_POLICY.md` (the required pipeline and policy requirements) and `SECURITY_REVIEW.md`'s existing claims |

## What was not examined

- **TLS and certificate handling.** The fetcher uses Go's defaults; no probe
  attempted a bad chain, a downgrade, or a pinned-certificate case.
- **The store's own SQL layer**, publication, sync, and the REST surface beyond
  MCP. Those are other items' subjects and this review does not restate them.
- **Denial of service by volume**: many URLs, slow-loris servers, or a
  quarantine directory filled to exhaustion.
- **The GUI preview path.** The policy's preview rule says the server performs
  localization; whether the renderer honours that was read, not probed.
- **Perceptual-hash hooks**, which ship disabled.

## Findings

Four. Each is deferred to its own plan item, for the owner to approve; none is
fixed here. None is written up with an exploit recipe: each names the gap and
what closing it would take.

### J8-F1 — a trailing dot evades a blocked domain

**What happens.** `https://blocked.example.org./a.png` is evaluated as
`review`, not `block`, while `https://blocked.example.org/a.png` is blocked. In
DNS the two names are the same host; `matchDomain` compares the host string
literally, so the dotted form matches no pattern and falls through to the
policy default.

**Why it matters.** A URL the operator explicitly blocked becomes fetchable
under the default posture as soon as a reviewer opts in, and the audit trail
records it as a default-action decision rather than a blocked one. The allowed
list fails the safe way round (an allowed host with a trailing dot becomes
`review`, which is stricter), so only the blocked direction is a gap.

**What closing it takes.** Normalising the host before matching — one
lowercase, trailing-dot-stripped form used by both the pattern match and the
comparison — plus cases for the dotted form of every list.

**Probe.** `TestJ8HostFormsCannotDisguiseADestination`.

### J8-F2 — reserved address ranges the private check does not name

**What happens.** The static check refuses loopback, RFC 1918, link-local,
IPv6 unique-local, IPv4-mapped IPv6 forms of both loopback and private, the
unspecified address, `localhost`, and multicast. It does **not** refuse:

| literal | range |
|---|---|
| `100.64.0.1` | carrier-grade NAT, RFC 6598 |
| `198.18.0.1` | benchmarking, RFC 2544 |
| `192.0.0.1` | IETF protocol assignments, RFC 6890 |
| `240.0.0.1` | reserved for future use, RFC 1112 |
| `0.1.2.3` | "this network", RFC 1122 `0.0.0.0/8` |
| `255.255.255.255` | IPv4 broadcast |
| `64:ff9b::7f00:1` | NAT64 well-known prefix, RFC 6052 — this one maps to `127.0.0.1` |
| `fec0::1` | IPv6 site-local, deprecated, RFC 3879 |

**Why it matters.** The written policy says "private network and link-local",
and those are covered; these ranges are outside what it names, which is why
this is a finding about the *policy's* reach as much as the code's. The NAT64
prefix is the sharpest: on a host with NAT64 configured it is a route to
loopback that neither check refuses.

**What closing it takes.** Deciding which reserved ranges belong in the refusal
set, writing them into `SECURITY_AND_MEDIA_POLICY.md`, and applying the same
set at both the static check and the connect-time check, which today share
neither a list nor a helper.

**Probe.** `TestJ8AddressLiteralsThePrivateCheckRefuses`.

### J8-F3 — reference forms the scanner does not see

**What happens.** `ScanBody` finds Markdown embeds and links, and quoted HTML
`<img src="…">` including across newlines and in either case. It does not find
an **unquoted** `<img src=https://…>`, nor `<video src="…">`.

**Why it matters.** A reference the scan never returns is never evaluated by
any later stage: it is neither blocked, reviewed, nor localized, and it stays a
remote load in a rendered note. Whether `<video>` belongs in scope is a policy
question; the unquoted `<img>` is a form a renderer loads.

**What closing it takes.** Deciding which HTML forms the scanner is responsible
for, then matching them — with the caveat that a regular expression over HTML
will always have an edge it misses, which is itself worth recording in the
policy.

**Probe.** `TestJ8ScanBodyFindsWhatAPreviewWouldFetch`.

### J8-F4 — a lying header decides the type when the sniff is inconclusive

**What happens.** Bytes that sniff as `text/plain` — a shell script, a CSV, any
text — served with `Content-Type: image/jpeg` are quarantined as an image, with
`image/jpeg` recorded as the content type and a `.jpg` extension on the
quarantine file. The fallback exists for a real case (SVG sniffs as `text/xml`)
and only applies when the sniff is inconclusive; a positive sniff of
`text/html` still beats the header, which the probe confirms.

**Why it matters.** The policy's pipeline says "apply … MIME … rules", and
`SECURITY_REVIEW.md` says MIME is sniffed. For inconclusive payloads the header
decides instead, so a note can end up embedding a resource whose recorded type
is not what its bytes are. The blast radius is bounded: the bytes are stored
content-addressed under their hash, nothing executes them, and admission still
applies exact-hash rules.

**What closing it takes.** Narrowing the fallback to the cases it was added
for — an explicit list such as SVG, rather than any inconclusive sniff — or
validating the claimed type against the payload before accepting it.

**Probe.** `TestJ8AServerThatLiesAboutItsContent`.

## What held

These were attempted and refused, or checked and found sound.

**Remote media**

- Blocked schemes (`file:`, `data:`, `javascript:`, `ftp:`) and anything that
  is not http(s).
- Loopback, RFC 1918, link-local, IPv6 unique-local, IPv4-mapped IPv6 private
  and loopback forms, the unspecified address, multicast, and `localhost`.
- A host that carries an allowed name as **userinfo**
  (`https://images.example.org@evil.test/`) is evaluated as its real
  destination.
- A wildcard pattern does not match its own apex, and a suffix that is not a
  subdomain does not match.
- A **redirect into a blocked host is refused**, and an endless redirect chain
  is stopped at the configured hop limit.
- A body that contradicts its own `Content-Length` is refused, and nothing is
  left in quarantine. The size cap on an honest body is proven by
  `TestQuarantineEnforcesSizeCapWhileStreaming` in `internal/media`.
- Quarantine is `0700`, files are named `sha256-<hash><ext>` with no
  server-chosen name, and a refused fetch leaves nothing behind.
- Provenance is recorded for admissions **and** refusals: document, original
  URL, final URL, decision, status, content type, size, hash and quarantine
  path.
- Admission consults exact-hash rules before creating a resource, and removes
  the quarantine file either way.

**MCP**

- The scope ladder is `search-only → read-only → editor → organizer`, the
  default is **read-only**, and there is no administrator scope.
- `toolsInScope` is applied inside `mcpTools()`, so `tools/list` and the info
  endpoint return the filtered set — a candidate finding that they did not was
  **disproved** by reading the call site.
- `handleMCPToolCall` checks the scope before its dispatch switch, so the check
  covers every tool rather than the ones with a test.
- Shipped tests already prove: every tool is classified; each scope lists
  exactly its tools; a hidden tool called directly is refused and the refusal
  names the scope; an unknown tool is refused as unknown; the sync scope is
  orthogonal; a half-migrated configuration can only narrow; and no scope
  reaches lint, fix, garbage collection, archive export or restore,
  publication, tag rename, notebook deletion, document purge, graph report or
  export, or job start, cancel, import or export.

## The archive

Built from `0f0f956` with the usage guard on and no override:
- **Usage guard:** it checked only Claude, 70% remaining, identified as
  `process ancestry: pid 1616574 is claude` (J24). An earlier attempt was
  **paused by the owner** rather than overridden, when the five-hour window sat
  one point under the 20% reserve; the build ran after the window reset.
- **Package:** `package_release.sh` passed, including `go test ./...`,
  `validate-scaffold.sh` and every evidence validator.
- **ZIP:** `notrios-v1.0-j8-0f0f956.zip`, 25,556,784 bytes, 2,640 entries.
  `check_release_zip.py` accepts it.

The probes run with the ordinary suite, and the whole Go suite through
`scripts/check_temp_leaks.sh` left no `notrios-*` entry — which covers the
loopback servers and quarantine directories these probes create.

## Dispositions

| finding | disposition |
|---|---|
| J8-F1 trailing dot evades a blocked domain | **deferred** to its own plan item |
| J8-F2 reserved ranges not refused | **deferred** to its own plan item |
| J8-F3 reference forms the scanner misses | **deferred** to its own plan item |
| J8-F4 lying header decides an inconclusive type | **deferred** to its own plan item |

No finding is accepted-as-is, and none is fixed in J8.
