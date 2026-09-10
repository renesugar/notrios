#!/usr/bin/env python3
"""Validate the I5 signing-policy record, offline.

A policy nobody checks is a paragraph. This gates the parts that can go wrong
quietly: a platform that stops saying what blocks it, a claim level that drifts
away from the evidence that established it, a timestamping policy that stops
matching the one the verifier actually pins, and the separation between the
release key and the evidence key.

Two values are *derived* rather than restated, because this repository keeps
finding that a fact written in two places becomes two facts:

  * the RFC 3161 policy OID comes from `evidence/verify_evidence.py`, which is
    the code that will reject a timestamp that does not match it;
  * the Ubuntu amd64 claim level comes from I3's report, which is what actually
    installed and ran the package.

It also runs the secret-exposure check, so that `make validate` refuses a
workflow that could hand signing material to a pull request whether or not
anybody remembers this document exists.
"""
import json
import pathlib
import re
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class PolicyError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise PolicyError(message)


def pinned_policy_oid() -> str:
    text = (ROOT / "evidence" / "verify_evidence.py").read_text(encoding="utf-8")
    match = re.search(r'^TSA_POLICY_OID\s*=\s*"([^"]+)"', text, re.MULTILINE)
    require(match is not None, "evidence/verify_evidence.py no longer pins a TSA policy OID")
    return match.group(1)


def main() -> None:
    policy = json.loads((HERE / "POLICY.json").read_text(encoding="utf-8"))
    require(policy["schema"] == "notrios.v09.i5-signing-policy.v1", "wrong policy schema")

    platforms = {p["id"]: p for p in policy["platforms"]}
    require(platforms, "the policy names no platforms")
    for name, platform in platforms.items():
        require(platform["status"] in ("available", "blocked"), f"{name}: unknown status")
        if platform["status"] == "blocked":
            require(platform.get("blocked_on"),
                    f"{name} is blocked and no longer says on what; a blocker without a reason is "
                    f"indistinguishable from work nobody did")
        else:
            for field in ("produce", "verify"):
                require(platform.get(field),
                        f"{name} is available but has no {field} steps, so nobody else can follow it")
            require(any("gpg --verify" in step for step in platform["verify"]),
                    f"{name}: the verification steps no longer include checking the signature")

    # The claim level for the platform this project actually ships must match the
    # evidence that established it, not a number typed here.
    ubuntu = platforms.get("ubuntu-24.04-amd64")
    require(ubuntu is not None, "the policy no longer covers the platform this project ships")
    i3 = json.loads((ROOT / "performance" / "v0.9-i3" / "REPORT.json").read_text(encoding="utf-8"))
    require(ubuntu["claim_level"] == i3["level_reached"]["level"],
            f"Ubuntu amd64 claim level {ubuntu['claim_level']} disagrees with I3's evidence "
            f"({i3['level_reached']['level']})")

    stamping = policy["timestamping"]
    require(stamping["policy_oid"] == pinned_policy_oid(),
            f"the policy's timestamp OID {stamping['policy_oid']} is not the one the verifier "
            f"pins ({pinned_policy_oid()})")
    require(stamping.get("what_it_does_not_attest"),
            "the timestamping entry no longer says what a timestamp does not attest")

    custody = policy["key_custody"]
    require(custody["release_key_is_not_the_evidence_key"]["decision"] == "separate keys",
            "the release key and the evidence key are no longer recorded as separate")
    for field in ("where_it_lives", "never", "rotation"):
        require(custody.get(field), f"key custody no longer records {field}")
    require("environment" in custody["where_it_lives"],
            "key custody no longer requires an environment-scoped secret")

    require(policy["not_resolved_here"], "the policy no longer says what it did not resolve")

    exposure = subprocess.run([sys.executable, str(HERE / "check_secret_exposure.py")],
                              capture_output=True, text=True)
    if exposure.returncode != 0:
        raise PolicyError(exposure.stderr.strip() or "secret exposure check failed")

    blocked = sum(1 for p in platforms.values() if p["status"] == "blocked")
    print(f"I5 signing policy valid: {len(platforms)} platforms, {blocked} blocked with reasons, "
          f"timestamp OID matches the verifier, {exposure.stdout.strip()}")


if __name__ == "__main__":
    try:
        main()
    except (PolicyError, KeyError, ValueError, OSError) as error:
        print(f"I5 signing policy invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
