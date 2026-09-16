# v1.0 J28: refuse the reserved address ranges remote media must not reach

J8 found that the remote-media private-address check refused what Go's
`IsLoopback`, `IsPrivate`, `IsLinkLocalUnicast`, `IsLinkLocalMulticast` and
`IsUnspecified` cover, and nothing else (finding J8-F2). The same expression
was written twice, once for URL literals and once at connect time. This item
replaces both with one named, dated, configurable set. The owner's decisions
D1–D9 are recorded in `PLAN.md` J28.

## Before

The expression, run over 44 addresses (one or more in every default range,
plus blocks deliberately left out), refused **15**. Of the 29 it let through,
27 belong to the default set; `::ffff:8.8.8.8` and ORCHIDv2 (`2001:20::1`)
should stay reachable.

Also measured: J8's README said "multicast" held. Its probe used `224.0.0.2`,
which is link-local multicast; `239.1.1.1`, `233.252.0.1`, `ff05::1` and
`ff0e::1` were not refused. The README now says so (see below).

## What changed

| where | change |
|---|---|
| `internal/addressrange` (new) | the default set (15 IPv4, 16 IPv6), entry parsing, the D8 guard, the embedded-IPv4 rule (D4, D9), `Check`, and `OmittedDefaults` (D2) |
| `internal/config/security.go` (new) | `security.remote_media.refused_address_ranges` and `permitted_address_ranges`; the block's reader; `RemoteMediaAddressWarnings` |
| `internal/config/config.go` | the loader hands the `security:` section to that reader and fails on a malformed block, naming the entry |
| `internal/media` | `WithAddressRanges`; the static check and `checkDialAddress` both call `Policy.checkAddress`, which calls `Rules.Check`; `localhost` names stay refused; ranges that do not parse refuse every URL, and `NewFetcher` returns an error |
| `internal/localize`, `internal/httpapi`, `cmd/notriosctl` | every production caller passes `WithAddressRanges(cfg.Security.RemoteMedia)` |
| `internal/service` | start fails on ranges it cannot apply, and logs each warning once |
| `internal/api`, `api/openapi.yaml` | `media_policy` gains `refused_address_ranges`, `permitted_address_ranges`, `address_ranges_origin` and `warnings` |
| `notriosctl config show` | rows for both keys with their origin, the refused list printed in full, `remote_media_addresses` and `warnings` in `--json`, warnings on standard error |

Refusal reasons now name the range: `address 100.64.0.1 is in refused range
100.64.0.0/10`, and at connect time `resolved address 127.0.0.1 is in refused
range 127.0.0.0/8`. `localhost` names give `localhost name localhost is
refused`.

### Where the implementation departs from the scope's wording

These were written into `PLAN.md` J28 ("Implementation design") before the code:

- **The helper is a leaf package**, not a function in `internal/media`. The
  loader has to reject malformed entries, and `internal/media` imports
  `internal/config`. `internal/media` keeps no range logic of its own.
- **Stricter malformed-entry rules than D3 names.** A prefix with host bits
  set, an IPv6 zone, and a list key with no value and no items also fail
  loading. A widened exception must not pass silently.
- **Unknown keys** under `security:` are ignored, as everywhere in the loader.
  A misspelled `refused_address_ranges` therefore leaves the default in force,
  which is the stricter outcome.
- **`localhost` names** are unaffected by a stated set and switched off only by
  `allow_private_networks`, as before.

## Proof

The new tests:

| package | tests | covers |
|---|---|---|
| `internal/addressrange` | 8 | every default entry has a case and refuses it; blocks left out stay reachable; embedded IPv4 in mapped, compatible and NAT64 forms, with `::` and `::1` refused as themselves and 6to4 not unwrapped; a stated set replaces the default and `OmittedDefaults` counts correctly, including a covering range; malformed entries in both lists named; exceptions and their embedded forms (D9); no exception reaches loopback or "this host" at parse or at `Check` (D8); zones |
| `internal/config` | 7 | absent block means default and warns about nothing; block and flow lists state the set with origin `file`; `[]`; eight malformed forms fail with the entry named; unknown keys and another surface's key are ignored; `allow_private_networks` warning (D5); a code-built invalid block |
| `internal/media` | 8 | the 44 measured addresses at **both** checks with the range named; both checks give one reason; embedded public addresses stay reachable; a stated set and its exceptions at both checks; `allow_private_networks` switches both off; ranges that do not parse refuse everything; `localhost` names; a source guard that every production caller passes `WithAddressRanges` |
| `internal/httpapi` | 2 | the default set in the REST view; a stated set reaches the check endpoint and the localizer, and its warnings are served |
| `internal/service` | 2 | start logs the omitted ranges and serves the stated set; start fails on a loopback exception |
| `cmd/notriosctl` | 1 | `config show`, plain and `--json`, and a malformed entry failing |

**The default-set test fails on the code before this change.** Run against the
previous `policy.go` and `fetch.go` in a scratch worktree, it failed for
exactly **27 addresses at the static check and the same 27 at the dial
check**: the 27 the plan predicted.

Existing tests whose assertions named the old wording were updated to the new
reason, with a stronger assertion:
- `TestQuarantineBlocksPrivateDialEndToEnd` now requires the range.
- `TestMediaPolicyEndpoints` does the same.

### J8's evidence

J8's archived probe `TestJ8AddressLiteralsThePrivateCheckRefuses` classified a
refusal by the words `private, loopback, or link-local`. Its classifier now
also recognises J28's wording. With that one change it records **21 of 21
refused and 0 not refused**; before J28 it recorded the J8-F2 gap. J8's README
gains a dated correction note and says "link-local multicast" where it said
"multicast". No other J8 file changed.

## Documentation

- `SECURITY_AND_MEDIA_POLICY.md` names the dated default set, the embedded-IPv4
  rule, what is deliberately left out and why, and how the set is stated.
- `docs/configuration.md` explains the block, the replace rule, the malformed
  forms and the version that first reads `security:`.
  - It gains a **tracked, executed example**
    (`configuration-remote-media-example-2`): a stated set and an exception,
    checked by `config show --json`.
  - The example needed `cmd/docexamples` to render one more level of nesting,
    and the documentation runner's `heredocLeafKeys` to read that level and
    compare list values. A throwaway check confirmed a list that differs from
    what `config show` reports fails the postcondition.
- The generated key table and key lists, `docs/service.md`'s table and
  status paragraph, `docs/troubleshooting.md` (the refusal reason and a
  malformed block), `SECURITY_REVIEW.md`, `CONTEXT_MAP.md` and
  `config/config.example.yaml` (the block commented out) are updated.

**Pinned evidence regenerated with its own tools:**
- the G18a inventory and G18f's hash of `docs/service.md`;
- J13's example classification (170 examples, 70 executed);
- the docaudit registry hash, set by `docexamples --write`;
- docgen's enumeration counts (`Config` 64 → 68, `Default` 54 → 56);
- docexec's coverage (169 → 170 entries, 69 → 70 executed), with G18d's
  validator pins and its `REPORT.json` regenerated from a real run;
- docaudit's coverage (170 executables, 77 → 78 executed, denominator
  485 → 486, unverified unchanged).
- J4's surface review, because `docs/service.md` now also documents
  `GET /api/v1/media-policy`.

Each count change carries a comment saying why.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry, and `validate-scaffold.sh` passes.

**Caught by the first archive attempt.** `package_release.sh` at `3a472ed`
stopped at `make g18g-validate`: the new paragraph in `docs/configuration.md`
linked `../SECURITY_AND_MEDIA_POLICY.md`, which the documentation site does not
publish. The local checks above had not built the site. The file is now named
in backticks, as the other published pages name it, and `make g18g-validate`
passes. The archive is built from the fix commit.

**The frozen configuration surface moved (I8).** `build_freeze.py` records 3
names added: `security`, `refused_address_ranges` and
`permitted_address_ranges`, taking the count from 61 to 64. The change is
**additive and compatible**: no key was removed or changed meaning, a
configuration without `security:` behaves as the default set, and
`allow_private_networks` keeps its meaning (D5). The behaviour change is that
more addresses are refused by default, which is the point of the item.
