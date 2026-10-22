#!/usr/bin/env python3
"""The release-set verifier, proven with a throwaway key (v1.0 J49).

    python3 performance/v1.0-j49/test_release_set.py

J2 proved `verify_release_set.py` this way, five refusals with a throwaway key
before the real key signed anything, and kept none of it. J49 changes the
verifier -- a second subject, attestation bundles, timestamps per subject -- so
the proof is a test now, and it runs in validation.

Each set is built the way the release workflow builds one: `build_release_set.py`
with the package, the C ABI tarball and a bundle per subject, then a detached
signature and a timestamp pair per subject, then SHA256SUMS over the directory
and its signature. The key is generated in a GnuPG home of its own, so the
owner's keyring is never read or written. The bundles are synthetic: their
sigstore signature is `gh attestation verify`'s to check, and what the verifier
checks is that each one names its artifact's bytes. The timestamp files are
placeholders for the same reason -- the workflow verifies each reply against
the pinned authority, and the verifier checks the reply is there.
"""
import base64
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "performance/v1.0-j49"))
import validate_evidence  # noqa: E402
BUILD = ROOT / "performance/v0.9-i6/build_release_set.py"
VERIFY = ROOT / "performance/v0.9-i6/verify_release_set.py"
DEB = "notrios_9.9.9-1_amd64.deb"
ABI = "notrios-c-abi-9.9.9-linux-amd64.tar.gz"


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def bundle_for(name, digest):
    statement = {"_type": "https://in-toto.io/Statement/v1",
                 "subject": [{"name": name, "digest": {"sha256": digest}}],
                 "predicateType": "https://slsa.dev/provenance/v1", "predicate": {}}
    return {"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json",
            "verificationMaterial": {},
            "dsseEnvelope": {"payloadType": "application/vnd.in-toto+json",
                             "payload": base64.b64encode(json.dumps(statement).encode()).decode(),
                             "signatures": []}}


class Keys:
    """Two throwaway signing keys, each in a GnuPG home of its own."""

    def __init__(self, work):
        self.homes = {}
        for label in ("release", "other"):
            home = work / f"gnupg-{label}"
            home.mkdir(mode=0o700)
            self.homes[label] = home
            self.gpg(label, "--quick-generate-key", f"Throwaway {label} <{label}@example.invalid>",
                     "ed25519", "sign", "1d")
            exported = self.gpg(label, "--armor", "--export").stdout
            (work / f"{label}-public.asc").write_bytes(exported)
        self.public = {label: work / f"{label}-public.asc" for label in self.homes}

    def gpg(self, label, *args):
        environment = dict(os.environ, GNUPGHOME=str(self.homes[label]))
        # The agent is allowed to start: generating and signing need it. It
        # starts inside this throwaway home and is killed in close().
        return subprocess.run(["gpg", "--batch", "--quiet", "--passphrase", "",
                               "--pinentry-mode", "loopback", *args],
                              env=environment, capture_output=True, check=True)

    def close(self):
        for home in self.homes.values():
            subprocess.run(["gpgconf", "--kill", "all"], env=dict(os.environ, GNUPGHOME=str(home)),
                           capture_output=True)

    def sign(self, label, path):
        signature = path.with_name(path.name + ".asc")
        signature.unlink(missing_ok=True)
        self.gpg(label, "--armor", "--detach-sign", "--output", str(signature), str(path))


def checksums(directory):
    """SHA256SUMS over the directory, as the workflow writes it."""
    lines = [f"{sha256(path)}  {path.name}\n" for path in sorted(directory.iterdir())
             if path.is_file() and not path.name.startswith("SHA256SUMS")]
    (directory / "SHA256SUMS").write_text("".join(lines), encoding="ascii")


class ReleaseSetTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.work = pathlib.Path(tempfile.mkdtemp(prefix="notrios-j49-"))
        cls.keys = Keys(cls.work)
        inputs = cls.work / "inputs"
        (inputs / "pkg/DEBIAN").mkdir(parents=True)
        (inputs / "pkg/DEBIAN/control").write_text(
            "Package: notrios\nVersion: 9.9.9-1\nArchitecture: amd64\n"
            "Maintainer: Throwaway <release@example.invalid>\nDescription: fixture\n")
        subprocess.run(["dpkg-deb", "--root-owner-group", "--build", str(inputs / "pkg"),
                        str(inputs / DEB)], check=True, capture_output=True)
        (inputs / "abi").mkdir()
        (inputs / "abi/README.md").write_text("fixture\n")
        subprocess.run(["tar", "-czf", str(inputs / ABI), "-C", str(inputs), "abi"], check=True)
        for name in (DEB, ABI):
            (inputs / f"{name}.bundle.json").write_text(
                json.dumps(bundle_for(name, sha256(inputs / name))))
        cls.inputs = inputs

    @classmethod
    def tearDownClass(cls):
        cls.keys.close()
        shutil.rmtree(cls.work, ignore_errors=True)

    def build(self, attested=True, abi=True):
        out = pathlib.Path(tempfile.mkdtemp(dir=self.work, prefix="set-"))
        command = [sys.executable, str(BUILD), "--deb", str(self.inputs / DEB), "--out", str(out), "--signed"]
        if abi:
            command += ["--abi", str(self.inputs / ABI)]
        if attested:
            command += ["--attested", "--attestation", f"deb={self.inputs / DEB}.bundle.json"]
            if abi:
                command += ["--attestation", f"abi={self.inputs / ABI}.bundle.json"]
        subprocess.run(command, check=True, capture_output=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
        for name in (DEB, ABI) if abi else (DEB,):
            self.keys.sign("release", out / name)
            for suffix in (".asc.tsq", ".asc.tsr"):
                (out / f"{name}{suffix}").write_bytes(b"placeholder")
        self.seal(out)
        return out

    def seal(self, out, signer="release"):
        checksums(out)
        self.keys.sign(signer, out / "SHA256SUMS")

    def verify(self, out, key="release"):
        return subprocess.run([sys.executable, str(VERIFY), "--set", str(out),
                               "--public-key", str(self.keys.public[key])],
                              capture_output=True, text=True,
                              env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))

    def assertRefused(self, out, reason, key="release"):
        result = self.verify(out, key)
        self.assertNotEqual(result.returncode, 0, f"accepted, wanted a refusal mentioning {reason!r}")
        self.assertIn(reason, result.stderr)

    def test_a_complete_set_verifies(self):
        result = self.verify(self.build())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("2 attestation bundles bound to their subjects", result.stdout)
        self.assertIn("3 signatures checked", result.stdout)
        self.assertIn("state signed, attested", result.stdout)

    # J2's five, kept.
    def test_refused_against_another_key(self):
        self.assertRefused(self.build(), "is not a good signature", key="other")

    def test_refused_a_signature_that_is_not_one(self):
        out = self.build()
        (out / f"{DEB}.asc").write_text("-----BEGIN PGP SIGNATURE-----\nnot one\n-----END PGP SIGNATURE-----\n")
        self.seal(out)
        self.assertRefused(out, f"{DEB}.asc is not a good signature")

    def test_refused_one_signature_swapped_for_another(self):
        out = self.build()
        shutil.copy2(out / f"{ABI}.asc", out / f"{DEB}.asc")
        self.seal(out)
        self.assertRefused(out, f"{DEB}.asc is not a good signature")

    def test_refused_the_checksums_unsigned(self):
        out = self.build()
        (out / "SHA256SUMS.asc").unlink()
        self.assertRefused(out, "must carry a signature over SHA256SUMS")

    def test_refused_signed_artifacts_recorded_as_unsigned(self):
        out = self.build()
        provenance = json.loads((out / "PROVENANCE.json").read_text())
        provenance["notrios:signing"]["state"] = "unsigned"
        (out / "PROVENANCE.json").write_text(json.dumps(provenance))
        self.seal(out)
        self.assertRefused(out, "records itself as unsigned")

    # J49's.
    def test_refused_the_abi_tarball_unsigned(self):
        out = self.build()
        (out / f"{ABI}.asc").unlink()
        self.seal(out)
        self.assertRefused(out, f"must carry a signature over {ABI}")

    def test_refused_a_subject_without_its_timestamp(self):
        out = self.build()
        (out / f"{ABI}.asc.tsr").unlink()
        self.seal(out)
        self.assertRefused(out, f"must carry {ABI}.asc.tsr")

    def test_refused_a_missing_bundle(self):
        out = self.build()
        (out / f"{ABI}.attestation.jsonl").unlink()
        self.seal(out)
        self.assertRefused(out, "one bundle per subject")

    def test_refused_a_bundle_for_other_bytes(self):
        out = self.build()
        (out / f"{ABI}.attestation.jsonl").write_text(
            json.dumps(bundle_for(ABI, "0" * 64)) + "\n")
        self.seal(out)
        self.assertRefused(out, f"not {ABI} at")

    def test_refused_another_artifact_s_bundle(self):
        out = self.build()
        shutil.copy2(out / f"{DEB}.attestation.jsonl", out / f"{ABI}.attestation.jsonl")
        self.seal(out)
        self.assertRefused(out, f"not {ABI} at")

    def test_refused_a_bundle_in_a_set_recorded_as_unattested(self):
        out = self.build(attested=False)
        shutil.copy2(self.inputs / f"{DEB}.bundle.json", out / f"{DEB}.attestation.jsonl")
        self.seal(out)
        self.assertRefused(out, "records itself as unattested")

    def test_refused_checksums_that_leave_out_a_bundle(self):
        out = self.build()
        sums = out / "SHA256SUMS"
        sums.write_text("".join(line for line in sums.read_text().splitlines(True)
                                if not line.rstrip().endswith(".attestation.jsonl")))
        self.keys.sign("release", sums)
        self.assertRefused(out, "does not cover the set exactly")

    def test_the_builder_refuses_an_attested_set_short_of_a_bundle(self):
        result = subprocess.run([sys.executable, str(BUILD), "--deb", str(self.inputs / DEB),
                                 "--abi", str(self.inputs / ABI),
                                 "--out", str(self.work / "short"), "--signed", "--attested",
                                 "--attestation", f"deb={self.inputs / DEB}.bundle.json"],
                                capture_output=True, text=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("needs a bundle for every subject", result.stderr)

    def test_an_unsigned_unattested_package_only_set_still_verifies(self):
        """The workstation's set, as v0.9 I6 builds it, is unchanged by J49."""
        out = pathlib.Path(tempfile.mkdtemp(dir=self.work, prefix="plain-"))
        subprocess.run([sys.executable, str(BUILD), "--deb", str(self.inputs / DEB), "--out", str(out)],
                       check=True, capture_output=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
        result = self.verify(out)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("state unsigned, unattested", result.stdout)


    # J49-E.
    def test_the_draft_uploads_every_file_in_the_set(self):
        out = self.build()
        listed = subprocess.run(["bash", str(ROOT / "performance/v0.9-i6/draft_release.sh"), "--list-assets"],
                                capture_output=True, text=True, check=True,
                                env=dict(os.environ, RELEASE_SET=str(out))).stdout.split()
        self.assertEqual(listed, sorted(path.name for path in out.iterdir()))
        self.assertIn("SHA256SUMS.asc", listed)
        self.assertIn(f"{ABI}.attestation.jsonl", listed)


class RunnerTest(unittest.TestCase):
    """J49-D: the release is built on a row the matrix supports."""
    matrix = {"surfaces": [
        {"surface": "command line and background service", "platform": "Ubuntu 24.04 amd64", "supported": True},
        {"surface": "command line and background service", "platform": "Ubuntu 26.04.1 amd64", "supported": False}]}

    def problems(self, label):
        return validate_evidence.runner_problems(f"jobs:\n  package:\n    runs-on: {label}\n", self.matrix)[1]

    def test_the_supported_image_passes(self):
        self.assertEqual(self.problems("ubuntu-24.04"), [])

    def test_a_moving_label_is_refused(self):
        self.assertIn("can move", self.problems("ubuntu-latest")[0])

    def test_an_image_the_matrix_does_not_support_is_refused(self):
        self.assertIn("does not support", self.problems("ubuntu-26.04")[0])

    def test_the_repository_s_workflow_passes(self):
        self.assertEqual(validate_evidence.main(), 0)


if __name__ == "__main__":
    unittest.main()
