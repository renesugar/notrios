# v1.0 J10: publish the user-authorized release

The plan is in `PLAN.md` under J10. This directory records each step as it
happens, in the order the plan gives.

```sh
python3 performance/v1.0-j10/plan_volumes.py     # the seal's volumes, held to the plan
```

## J10-A: the seal, planned

v1.0's 49 archives, 1,267,409,548 bytes, need two volumes. `plan_volumes.py`
derives `VOLUME_PLAN.json` from `performance/v1.0/MANIFEST.json`, so it needs no
evidence directory and runs in validation:

| Volume | Items | Archives | Payload bytes | Under the budget |
| --- | --- | --- | --- | --- |
| `NTR-EV-0005` | J1 to J26 | 25 | 636,677,987 | 44,896,413 |
| `NTR-EV-0006` | J27 to J50 | 24 | 630,731,561 | 50,842,839 |

J10 is in neither: its archive is taken at its close, after the release. The
cut is the one in item order that makes the larger volume smallest. A greedy
fill would have put J27 in the first volume, too, leaving it 19 MB under the
budget rather than 45.

Before anything was sealed, the pre-push evidence gate passed against the
reserve, and the reserve verified four volumes, `NTR-EV-0001` to `NTR-EV-0004`.
It listed 55 artifacts as unsealed: the 49 above and the six superseded v0.8
archives.

## J10-A: the seal

**Rehearsed first, 2026-10-22.** Both volumes ran end to end against a throwaway
ed25519 key, in a GnuPG home of their own on a scratch reserve, with a scratch
copy of the catalog. The rehearsal chained onto the real volume-0004. Each build
took about three and a half minutes. Both ISOs were built twice and came out
byte-identical, and both were read back and verified. The scratch catalog grew
to six entries, with the real four-entry catalog a byte-exact prefix of it. No
rehearsal file entered the repository.

**Sealed, 2026-10-22, on the owner's authorization.** Both volumes are signed by
the production subkey `4ABEB98A…68264005` and timestamped by DigiCert under the
pinned policy `2.16.840.1.114412.7.1`:

| Volume | Archives | ISO bytes | Blocks | Timestamp | Content commit |
| --- | --- | --- | --- | --- | --- |
| `NTR-EV-0005` | 25 | 637,296,640 | 311,180 | 2026-10-22T14:31:20Z | `8706d77` |
| `NTR-EV-0006` | 24 | 631,343,104 | 308,273 | 2026-10-22T14:39:42Z | `3eb36a8` |

`NTR-EV-0005` chains onto `NTR-EV-0004` and `NTR-EV-0006` onto `NTR-EV-0005`,
each by the hash of the previous checkpoint document. The catalog is at six
entries, and each append left the earlier entries a byte-exact prefix.
`plan_volumes.py` finds each volume's committed signatures equal to the plan.

The volumes are on the reserve drive only. Burning them to disc is a separate
step that the owner has not asked for.

## J10-A: main, and the rehearsal J49 left owing

**Synchronized.** `develop` was pushed (`028a4f7..188e190`) and pull request
#11 opened into `main`. Its checks passed: go, gui-build, smoke and web, on
both CI runs. It merged with a merge commit, `2aab9a8`. The docs site deployed
from it (run 39806769137), with the 1.0 installation page ahead of the release,
as the owner accepted. Then `develop` was fast-forwarded to `2aab9a8`, the way
earlier milestones were synced, and pushed. The two branches are the same
commit.

**Dispatched by the owner.** Claude Code's auto-mode classifier refused the
agent's `gh workflow run release.yml --ref main` as a production deploy, so
the owner ran it. That was run
[39807652373](https://github.com/renesugar/notrios/actions/runs/39807652373),
on `ubuntu-24.04`, green in 4m55s:
- **The narrowed secret is now proven narrow.** The import step printed
  `signing key: 1234C691AC0776A18524D55687027B1DD464695E (primary stubbed,
  signing subkey present)`. That is J2's field-15 gate, running on `main` for
  the first time. It is the one thing J10's plan said was still owed for the
  narrowing.
- **Both artifacts were attested:** attestations 58703929 (the package) and
  58704349 (the C ABI tarball).
- **The verifier accepted the set on the runner:** `release set verified: 12
  artifacts, 3 signatures checked, 2 attestation bundles bound to their
  subjects, 404 SBOM components matching the licence inventory (1 without a
  declared licence), provenance bound to the bytes, state signed, attested`.

**Downloaded and checked here.** The set was downloaded into an empty directory
of its own, and the same verifier, run locally, printed the same line. Then
`run_verification.py --set <it>` ran the installation page's blocks as
published: 11 of 11 as expected, including the four refusals. `VERIFY.json`'s
`not_run_here` is now empty. The C ABI tarball's signature, timestamp, its
time and its attestation (logged in and with `--bundle`) each ran against the
real tarball. The runner gained `--source`, so the record names this run
rather than J9's.

What remains is J10-B, the tag and the release, and it needs the owner's
authorization.

## J10-B: prepared

`draft_release.sh` drafts the 1.0 release from the tag's own set, with
`RELEASE_NOTES.md` as the notes. Every link in the notes was checked against the
live page, and so was the backup command. 0.8.0 has `export archive-v2` too, so
the advice to back up before upgrading can be followed. Run dry against J10-A's
rehearsal set, the script verified the set, then refused it: `tag v1.0.0: not
on GitHub`.

## J10-B: tagged, built, drafted (2026-10-22)

**Tagged.** `develop` was pushed (`2aab9a8..1ad8afa`). An annotated tag `v1.0.0`
was created at `2aab9a8`, the reviewed `main` that the rehearsal built from, and
pushed.

**Built from the tag.** The tag started
[39812310020](https://github.com/renesugar/notrios/actions/runs/39812310020),
green, from head `2aab9a8`:
- **The tag is the product version:** the check printed `tag=1.0.0 product
  version=1.0.0`.
- **The narrowed key signed:** `signing key: 1234C691…D464695E (primary
  stubbed, signing subkey present)`.
- **The verifier accepted the set:** `release set verified: 12 artifacts, 3
  signatures checked, 2 attestation bundles bound to their subjects, …, state
  signed, attested`.

**Drafted.** The run's set was downloaded into an empty directory of its own.
`draft_release.sh` verified it and confirmed the provenance's commit is the
tag's. It then created the draft "Notrios 1.0.0" with all 14 files and
`RELEASE_NOTES.md`. Read back from GitHub, the draft's 14 assets match the
verified set by name, size and SHA-256. The draft is not published.

**A finding, recorded and not fixed.** The tag's set and the rehearsal's were
built from the same commit, yet the package and the C ABI tarball differ byte
for byte. The package is `e2a80a22…` against the rehearsal's `3f5fb3e6…`; the
SBOM is identical. Nothing published claims the package is reproducible, and
each set is consistent with its own signatures, attestations and provenance.
So it does not block the release. Making the builds reproducible is an item of
its own, after 1.0.
