#!/usr/bin/env python3
"""Run the installation page's verification instructions as published.

    python3 performance/v1.0-j9/run_verification.py --set <release-set-dir> [--out VERIFY.json]

The blocks are read out of docs/installation.md's "Verifying what you
downloaded" section, not retyped here, so what runs is what a reader is told to
run. The only change is the file name: the page names the 1.0.0 package, and
the set this runs against is the rehearsal set from run 34697727452, which is
0.8.0 -- J10-C runs the same blocks against the published 1.0.0 set.

Each block runs in a copy of the set with a GnuPG home of its own, so the
owner's keyring is never read or written. Then each check is made to fail once,
because a verification that accepts everything proves nothing:

- one byte appended to the package: the checksum, the signature and the
  attestation must refuse it;
- one byte appended to the package's signature: the timestamp must refuse it;
- no GitHub login: the attestation check must stop and say so, which is what
  the page tells the reader.

The curl blocks fetch the key and the timestamp authority's certificates from
the repository's main branch, as a reader would. The attestation check uses
the GitHub CLI's existing login for the logged-in run.
"""
import argparse
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/installation.md"
SECTION = "### Verifying what you downloaded"
PUBLISHED_DEB = "notrios_1.0.0-1_amd64.deb"
PRIMARY = "1234 C691 AC07 76A1 8524  D556 8702 7B1D D464 695E"
SUBKEY = "348B 6B87 3994 D1AF F924  D7FA F1BF 879D CE41 EA2C"


def published_blocks():
    text = DOC.read_text()
    start = text.index(SECTION)
    end = text.index("\n### ", start + len(SECTION))
    return re.findall(r"```sh\n(.*?)```", text[start:end], re.S)


def run(block, cwd, env):
    # -e: every line of a block must succeed, so a block whose last line
    # succeeds cannot hide an earlier failure behind its exit status.
    done = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", block], cwd=cwd, env=env,
                          capture_output=True, text=True)
    return done.returncode, done.stdout + done.stderr


def tail(output, lines=12):
    # gpg and openssl print a few lines about local configuration; the record
    # keeps the end of the output, where the verdict is.
    return [line for line in output.strip().splitlines() if "conf_def.c" not in line][-lines:]


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--set", required=True, type=pathlib.Path)
    parser.add_argument("--out", type=pathlib.Path, default=ROOT / "performance/v1.0-j9/VERIFY.json")
    args = parser.parse_args()

    debs = sorted(args.set.glob("notrios_*_amd64.deb"))
    if len(debs) != 1:
        sys.exit(f"want one package in {args.set}, found {[d.name for d in debs]}")
    deb = debs[0].name
    blocks = [block.replace(PUBLISHED_DEB, deb) for block in published_blocks()]
    if len(blocks) != 6:
        sys.exit(f"the section has {len(blocks)} sh blocks; this runner expects the six it checks")
    checksum, fetch_key, signatures, timestamp, timestamp_time, attestation = blocks

    results, failures = [], []

    def record(name, block, code, output, passed, expect):
        results.append({"check": name, "exit": code, "passed": passed, "expect": expect,
                        "block": block.strip().splitlines(), "output_tail": tail(output)})
        if not passed:
            failures.append(name)
        print(f"{'pass' if passed else 'FAIL'}  {name}")

    with tempfile.TemporaryDirectory() as work:
        work = pathlib.Path(work)
        good, altered = work / "set", work / "altered"
        shutil.copytree(args.set, good)
        shutil.copytree(args.set, altered)
        with open(altered / deb, "ab") as stream:
            stream.write(b"x")
        gnupg, ghcfg = work / "gnupg", work / "gh-logged-out"
        gnupg.mkdir(mode=0o700)
        ghcfg.mkdir()
        env = {**os.environ, "GNUPGHOME": str(gnupg)}

        code, out = run(checksum, good, env)
        record("checksums", checksum, code, out, code == 0 and "FAILED" not in out and out.count(": OK") >= 6,
               "every line OK")
        code, out = run(fetch_key, good, env)
        record("fetch the key and show its fingerprint", fetch_key, code, out,
               code == 0 and PRIMARY in out, f"primary fingerprint {PRIMARY}")
        code, out = run(signatures, good, env)
        record("signatures", signatures, code, out,
               code == 0 and out.count("Good signature from") == 2 and PRIMARY in out and SUBKEY in out,
               "two good signatures, primary and subkey fingerprints printed")
        code, out = run(timestamp, good, env)
        record("timestamp", timestamp, code, out, code == 0 and "Verification: OK" in out, "Verification: OK")
        code, out = run(timestamp_time, good, env)
        record("the time the timestamp attests", timestamp_time, code, out,
               code == 0 and "Time stamp:" in out, "a Time stamp line")
        code, out = run(attestation, good, env)
        record("attestation, logged in", attestation, code, out, code == 0, "exit 0")
        logged_out = {k: v for k, v in env.items() if k not in ("GH_TOKEN", "GITHUB_TOKEN")}
        logged_out["GH_CONFIG_DIR"] = str(ghcfg)
        code, out = run(attestation, good, logged_out)
        record("attestation, no login", attestation, code, out,
               code != 0 and "gh auth login" in out, "refused, asking for a login")

        # The refusals. The altered set has the key already: the same GnuPG home.
        shutil.copy2(good / "release-public.asc", altered)
        for name in ("tsa-root.pem", "tsa-untrusted.pem"):
            shutil.copy2(good / name, altered)
        code, out = run(checksum, altered, env)
        record("refusal: checksum of an altered package", checksum, code, out,
               code != 0 and f"{deb}: FAILED" in out, "the package line FAILED")
        code, out = run(signatures, altered, env)
        record("refusal: signature over an altered package", signatures, code, out,
               code != 0 and "BAD signature" in out, "BAD signature")
        code, out = run(attestation, altered, env)
        record("refusal: attestation of an altered package", attestation, code, out, code != 0, "non-zero exit")
        with open(altered / f"{deb}.asc", "ab") as stream:
            stream.write(b" ")
        code, out = run(timestamp, altered, env)
        record("refusal: timestamp over an altered signature", timestamp, code, out,
               code != 0 and "Verification: FAILED" in out and "imprint mismatch" in out,
               "message imprint mismatch, Verification: FAILED")

    tools = {}
    for tool, flag in (("gpg", "--version"), ("openssl", "version"), ("gh", "--version"), ("sha256sum", "--version")):
        found = subprocess.run([tool, flag], capture_output=True, text=True)
        tools[tool] = found.stdout.strip().splitlines()[0] if found.stdout else ""
    args.out.write_text(json.dumps({
        "schema": "notrios.v10.j9-verification-run.v1",
        "what_this_is": "The installation page's verification blocks, run as published against a release set, then made to fail once each.",
        "set": {"package": deb, "source": "release workflow run 34697727452 (rehearsal, 0.8.0)",
                "published_name_replaced": PUBLISHED_DEB},
        "tools": tools,
        "checks": results,
        "passed": len(results) - len(failures),
        "failed": failures,
    }, indent=2) + "\n")
    print(f"{len(results) - len(failures)}/{len(results)} checks as expected; written {args.out.relative_to(ROOT)}")
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
