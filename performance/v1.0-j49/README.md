# v1.0 J49: everything a stranger verifies with, in the release set

J9-C ran its verification instructions against the rehearsal set from run
34697727452 and found two gaps. Checking provenance needed a GitHub login,
because the set carried no attestation bundle. And the C ABI tarball was
attested but not signed, outside the signed `SHA256SUMS`. J9-B added a third:
the release ran on `ubuntu-latest`. Reading before the work added a fourth:
the upload would have left out `SHA256SUMS.asc`. The plan is in `PLAN.md`
under J49.

```sh
python3 performance/v1.0-j49/test_release_set.py      # 20 tests, a throwaway key
python3 performance/v1.0-j49/validate_evidence.py     # the runner is a supported row
python3 performance/v1.0-j9/run_verification.py --set <set>   # the page, as published
```

All three run in validation except the last, which needs a release set.

## J49-A: the attestation bundles travel with the release

The workflow gives each attest step an id and passes its `bundle-path` output
to `build_release_set.py --attested --attestation deb=… --attestation abi=…`.
The set then carries `<artifact>.attestation.jsonl`, one sigstore bundle per
line, which is the form `gh attestation download` writes. The builder accepts
either shape the action may produce, and refuses an attested set that is short
of a bundle. The provenance records `notrios:attestation`, `attested` or
`unattested`, beside `notrios:signing`.

The verifier requires one bundle per subject in an attested set. It decodes
each bundle's DSSE payload and requires an SLSA provenance statement naming
that artifact at the SHA-256 in `SHA256SUMS`. A set recorded as unattested must
carry no bundle. Checking the bundle's sigstore signature is
`gh attestation verify --bundle`'s job, and the page tells the reader to run
it.

**Measured with the real bundle.** The rehearsal set's package bundle, fetched
with `gh attestation download` and named as the set now names it, verifies with
the page's command and no GitHub login. It still refuses the package with one
byte added. `--bundle` combines with `--signer-workflow`, so the logged-out
check is as narrow as the logged-in one.

## J49-B: the C ABI tarball is signed

`build_release_set.py --abi` carries the tarball as a second provenance subject.
The workflow's signing step signs and timestamps every artifact in one loop.
The verifier now requires a signature, a `.asc.tsq` and an `.asc.tsr` for
**every** subject. Before, it required a signature only over the first subject
and `SHA256SUMS`, so a set signing only the package would have passed. The
separate `notrios-c-abi` run artifact is gone: the set carries the tarball.

**`SHA256SUMS` is now taken over the directory.** The workflow wrote it from a
list of globs naming the package, its signature files, the SBOM and the
provenance. It would have left out the tarball and both bundles, and the
verifier would then have refused the set on the runner. It now hashes every
file but `SHA256SUMS*`, as `build_release_set.py` already did.

## J49-C: the instructions follow

The installation page:
- lists the tarball, a signature and a timestamp pair per artifact, and a
  bundle per artifact;
- verifies both artifacts' signatures and timestamps;
- checks both attestations with `--bundle`, saying it needs no login and that
  without `--bundle` the command needs one.

`run_verification.py` ran the page as published against the rehearsal set,
with the real package bundle added: 11 of 11 as expected (`VERIFY.json` in
J9's directory). The no-login attestation check is now expected to pass, where
J9 expected a refusal.

**Not run here, and why.** The rehearsal set has no C ABI tarball. It
predates J6's change to the workflow, and `main` still carries the older
workflow. The page's four commands for the tarball (its signature, its
timestamp, its time, its attestation) are recorded in `VERIFY.json` under
`not_run_here`, by name. They run when a workflow built from this commit
produces a set, which needs a merge to `main` and a dispatch of `release.yml`.
That signs with the release key and requests a timestamp, so it needs the
owner's authorization. The plan gives it to J10-A's rehearsal if it is not
given here.

## J49-D: the release is built on the supported row

`release.yml` names `ubuntu-24.04`. `validate_evidence.py` reads every
`runs-on` in the workflow and refuses one that is not an exact Ubuntu image or
not a supported row of J9's matrix. Its tests refuse `ubuntu-latest` (it can
move) and `ubuntu-26.04` (not a supported row), and pass the repository's own
workflow.

## J49-E: the upload carries the whole set

`draft_release.sh` took its assets from `SHA256SUMS`, plus `SHA256SUMS` itself.
That left out `SHA256SUMS.asc`, which is deliberately absent from the file it
signs. The script's own comment recorded the same mistake made once before, for
`SHA256SUMS`. It now uploads every file in the set directory. A new
`--list-assets` mode prints that list and nothing else, and the test checks it
equals the directory, `SHA256SUMS.asc` and the bundles included. Its draft
notes no longer say the set is unsigned and that no key exists, which J2 made
untrue.

## J2's proof, kept

J2 proved the verifier with a throwaway key, five refusals, and kept none of
it. `test_release_set.py` builds sets the way the workflow does, with two
ed25519 keys generated in GnuPG homes of the test's own. Their agents are
killed afterwards, and no keyring of the owner's is read. It checks:
- **J2's five:** another key, a signature that is not one, one artifact's
  signature swapped for another's, the checksums unsigned, and signed
  artifacts recorded as unsigned.
- **J49's:** the tarball unsigned, a subject without its timestamp reply, a
  missing bundle, a bundle for other bytes, another artifact's bundle, a bundle
  in an unattested set, checksums that leave out a bundle, a builder asked for
  an attested set short of a bundle, and the upload list.
- **What must still pass:** a complete set, and the workstation's unsigned,
  unattested, package-only set, which J49 leaves as it was.

Against the verifier as it was before J49, eight tests fail. Those are J49's
new refusals and the two that read its new report line. J2's five pass, as
they should.

The pinned-action count in v0.9 I6's record goes from 24 to 23. The upload
step for the separate tarball is gone, and the validator derives the count, so
it refused until the record was corrected.
