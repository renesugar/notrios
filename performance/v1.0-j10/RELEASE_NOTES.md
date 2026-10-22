Notrios 1.0.0: local-first notes, search, import and publishing for very large
Markdown and document collections.

## What is in this release

| File | What it is |
|---|---|
| `notrios_1.0.0-1_amd64.deb` | the package: `notrios`, `notriosd`, `notriosctl`, the web interface and the desktop shell |
| `notrios-c-abi-1.0.0-linux-amd64.tar.gz` | the C library (`libnotrios.so.1`) and its header |
| `<artifact>.asc`, `.asc.tsq`, `.asc.tsr` | each artifact's OpenPGP signature, and an RFC 3161 timestamp over it |
| `<artifact>.attestation.jsonl` | each artifact's build attestation, as a sigstore bundle |
| `SBOM.cdx.json`, `PROVENANCE.json` | the software bill of materials, and how the two were built |
| `SHA256SUMS`, `SHA256SUMS.asc` | the checksum of every file, and a signature over the checksums |

The set is signed by the release key whose fingerprint is
`1234 C691 AC07 76A1 8524 D556 8702 7B1D D464 695E`, published in the
repository at `keys/release-public.asc`. Verify it before installing, as
[Verifying what you downloaded](https://notrios.com/installation.html#verifying-what-you-downloaded)
describes. Checking the attestations needs no GitHub login: pass `--bundle`.

```sh
sudo apt install ./notrios_1.0.0-1_amd64.deb
notriosctl doctor
```

## What is supported

- **Supported:** the command line and the background service, and the C
  library, on Ubuntu 24.04 amd64. These were installed and run at 1.0.0.
- **Supported:** the web interface in Google Chrome 156. Every control was
  driven in it at 1.0.0.
- **Packaged, not claimed:** the desktop shell is compiled and packaged, but
  nothing in this project has opened its window.

[The support matrix](https://notrios.com/installation.html#what-is-supported)
gives every row, and what each one lacks.

## Upgrading from 0.8

**Back up first:** `notriosctl export archive-v2 ~/notrios-backup`, as
[Backing up](https://notrios.com/installation.html#backing-up-and-getting-your-notes-back)
describes.

**The library format changes.** 1.0 moves a library from schema 27 to 28 the
first time it opens it. After that, 0.8 will not open the library. Every note
comes through.

**Upgrade every machine that syncs together.** 0.8 and 1.0 do not sync with
each other.

**Do not open a 1.0 library with 0.7.0.** It rewrites the schema version.

To go back to 0.8, restore the archive-v2 backup. Reinstalling the older package
is not enough; see
[Going back to an earlier version](https://notrios.com/installation.html#going-back-to-an-earlier-version).

## The record behind it

Every v1.0 item closed with an archive built from its closing commit. All 49
archives were sealed before this release, in evidence reserve volumes
`NTR-EV-0005` and `NTR-EV-0006`, signed and RFC 3161 timestamped on 2026-10-22.
`PLAN.md` says what each item proved and what it left owed.
