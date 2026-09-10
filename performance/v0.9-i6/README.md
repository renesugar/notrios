# v0.9 I6 — the release evidence set, and CI's supply chain

```sh
make deb
python3 performance/v0.9-i6/build_release_set.py     # artifacts + SBOM + provenance + checksums
python3 performance/v0.9-i6/verify_release_set.py    # recompute everything, offline
bash    performance/v0.9-i6/draft_release.sh         # dry; --execute uploads
python3 performance/v0.9-i6/validate_evidence.py     # gates the record and the workflows
```

## No SBOM tool was added

None is installed and none was needed. H6a's rule was the smallest maintainable
toolchain, and the inputs are already here: G20's licence inventory, derived
from `go.mod` for its own reasons, and the two npm lockfiles. Generating from
those runs offline and produces a CycloneDX 1.5 document of 400 components.

## Cross-checked, because agreement between one author's two programs is not evidence

The SBOM is generated from the lockfiles and verified against G20's inventory —
39 Go modules, 353 + 8 npm packages, matching exactly. A generator and a
verifier written from the same assumption fail together and look like agreement;
these two were written for different purposes, so a disagreement is visible.

Licence expressions are parsed rather than matched: `(MPL-2.0 OR Apache-2.0)` is
a choice between two acceptable licences, and refusing it as "not a bare SPDX
id" would reject a dependency whose terms are fine twice over.

## Workflow hardening

All 17 `uses:` are pinned to commit digests, with the tag kept as a trailing
comment so a reader can still tell what version a digest is. A tag is a pointer
somebody else can move, and whoever controls it chooses what runs here with this
repository's token.

`ci.yml` declared no `permissions:` and inherited the repository default — which
can be read/write on every scope — for a workflow that only builds and tests. It
declares `contents: read` now.

## The same mistake, twice, caught once by running and once by a gate

The draft script builds its upload list from `SHA256SUMS`, and that file does
not list itself: the first version would have uploaded every artifact **except
the file a downloader checks the others against**. Running the dry flow showed
it. Then the report generator made the identical mistake, and
`validate_evidence.py` refused the report.

A dry run that only printed a command nobody read would have caught neither.

## What this does not establish

`REPORT.json` carries the list and the validator fails if it shrinks: nothing is
signed or timestamped (I5 created no key deliberately, and the verifier refuses
a set that claims otherwise), no draft was created so the upload path is
unexercised, the SBOM covers dependencies rather than the package's contents, no
vulnerability scanner ran, and the provenance is a statement this repository
wrote about its own build on a workstation — SLSA build level 1 at most, with no
builder identity anybody else can check.
