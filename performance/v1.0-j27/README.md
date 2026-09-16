# v1.0 J27: normalise a host before matching it against the domain lists

J8 found that `https://blocked.example.org./a.png` was evaluated as `review`
while `https://blocked.example.org/a.png` was blocked (finding J8-F1). DNS
treats the two as the same name; the policy compared the host string
literally. This item closes that gap.

## What was measured before the change

How Go's `net/url` reads the forms concerned:

| input host | `Hostname()` | note |
|---|---|---|
| `blocked.example.org.` | `blocked.example.org.` | dot kept |
| `BLOCKED.Example.ORG.:8443` | `BLOCKED.Example.ORG.` | case and dot kept |
| `127.0.0.1.` | `127.0.0.1.` | `net.ParseIP` returns nil |
| `localhost.` | `localhost.` | the literal `localhost` comparison misses it |
| `blocked.example.org..` | `blocked.example.org..` | parses; Go's resolver says `no such host` |
| `.` | `.` | parses |
| `blocked%2Eexample.org` | — | parse error: a percent-encoded dot cannot disguise a host |
| `blocked．example.org` (U+FF0E) | kept as written | Unicode; outside this item |

So the gap was wider than J8 recorded: the dotted form also escaped the static
private check. `http://127.0.0.1./a.png` and `http://localhost./a.png` were
`review`, not `block`. The connect-time check still refused what they resolved
to, so this was a wrong static verdict rather than a fetch of loopback.

## What changed

`internal/media/policy.go` gained `normalizeHost`. `Evaluate` applies it once,
before the private-address check and the blocked, allowed and review lists.

- The host is lowercased and **one** trailing dot is removed.
- A host that is empty afterwards, or has an empty label (`a..b`, `.a`, `a..`),
  is **blocked as `malformed host`**, not left to the default action.
- Address literals pass through apart from the dot, so `127.0.0.1.` is checked
  as `127.0.0.1`.
- Configured patterns are normalised the same way, so `example.org.` in a list
  means `example.org`. A pattern malformed once normalised matches nothing. The
  wildcard rule is unchanged: `*.example.org` matches any subdomain, not the
  apex.

The redirect check calls the same `Evaluate`, so every redirect hop gets the
same normalisation.

`docs/service.md`'s domain-pattern row, and the stop-list requirements in
`SECURITY_AND_MEDIA_POLICY.md`, say how hosts are matched.

**Pinned evidence refreshed, not hand-waved.** G18a's inventory was regenerated
and G18f's recorded hash of `docs/service.md` updated, both because the row
changed.

## Proof

`internal/media/j27_host_forms_test.go`, six tests:

| test | covers |
|---|---|
| `TestJ27ATrailingDotHostMeetsTheSameRules` | dotted forms against the blocked, allowed and review lists, exact and wildcard, with case, port and userinfo; action **and** reason must equal the undotted form's |
| `TestJ27ATrailingDotCannotDisguiseAPrivateHost` | `localhost.`, `*.localhost.`, `127.0.0.1.`, `10.1.2.3.`; `allow_private_networks: true` still lets a dotted literal through |
| `TestJ27AHostWithAnEmptyLabelIsBlockedAsMalformed` | `.`, a second trailing dot, an inner and a leading empty label |
| `TestJ27ADottedPatternMeansTheUndottedName` | dotted and mixed-case patterns |
| `TestJ27AWildcardKeepsItsMeaning` | the apex and a non-subdomain suffix stay unmatched, dotted or not |
| `TestJ27AFetchRefusesADottedBlockedHost` | through `Fetcher.Quarantine`, with review permitted: a dotted blocked note URL and a redirect hop to one are refused by the blocked pattern |

**Every test fails on the code before this change**, which was checked by
restoring the previous `policy.go`:
- all six tests failed, the five policy tests with 22 failing cases;
- in the fetch test, the note URL and the redirect hop both reached the network
  (`fetch failed: Get "http://tracker.example.com./…"`) instead of being blocked.

With the change, the whole Go suite passes through
`scripts/check_temp_leaks.sh`, which left no `notrios-*` entry, and
`validate-scaffold.sh` passes. J8's probe
(`TestJ8HostFormsCannotDisguiseADestination`) now logs both J8-F1 cases as
`as expected`. That probe is J8's archived evidence and was not edited; the
regression guard is this item's tests.

## Not in this item

- **Unicode and IDNA host forms.** A full-width dot, or a Unicode name, is left
  as written and matches no ASCII pattern. It is a separate question, as J27's
  boundary says.
- **What the patterns mean.** Only the form of the host they are matched
  against changed, apart from the new malformed-host refusal.
- **The reserved address ranges.** That is J28.
