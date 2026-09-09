#!/usr/bin/env python3
"""Validate the v0.8 H11 Android-emulator acceptance record.

The run itself needs an emulator, an NDK and eight minutes, so it is not in
`make validate` for the same reason the GUI crawl is not: this has to stay
runnable offline. What is checked here is the committed record — that it
describes a run that happened, on a device it names, and that it claims exactly
what was executed and nothing adjacent to it.

The claim this exists to keep honest is the one that is easiest to overstate.
An emulator result is not a phone, x86_64 is not arm64, and a shared core with
no interface on top of it is not an Android application. Every one of those is
listed in the record as not claimed, and this refuses a record that quietly
stops listing one.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent

# The claims that must stay disclaimed. A record that drops one of these has
# started describing something nobody ran.
REQUIRED_DISCLAIMERS = {
    "physical Android device",
    "iOS",
    "any user interface",
    "an app-store artifact",
    "background or battery behaviour",
    "a production secure store on Android",
    "a Flutter client",
    "arm64 at runtime",
}

# The device the acceptance was taken on, pinned so that a re-run on a different
# image is a decision somebody makes rather than a number that moves. H0 chose
# it and recorded why; H11 executed against the same one.
EXPECTED_FINGERPRINT = (
    "google/sdk_gphone64_x86_64/emu64xa:15/AE3A.240806.043/12960925:userdebug/dev-keys")

# Every phase the acceptance runs. A record with fewer has skipped one, and a
# skipped phase is exactly what "it passed" would then be hiding: the kill and
# the reboot are the two that a desktop run cannot perform at all.
REQUIRED_PHASES = {
    "open", "work", "stream", "abandon+kill", "verify-after-kill",
    "reboot", "verify-after-reboot",
}

# 38 checks in the first complete run. Pinned like every other inventory here:
# a run that quietly asserts less is a run that says less while reporting the
# same word.
EXPECTED_CHECKS = 38


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.h11.android-acceptance.v1", "wrong acceptance schema")

    missing = REQUIRED_DISCLAIMERS - set(report.get("not_claimed", []))
    require(not missing, f"the record stopped disclaiming {sorted(missing)}")

    environment = report["environment"]
    require(environment["fingerprint"] == EXPECTED_FINGERPRINT,
            f"acceptance ran on {environment['fingerprint']}, not the pinned image")
    require(environment["device_abi"] == "x86_64",
            "the runtime ABI is x86_64; an arm64 runtime is a separate claim")
    require(environment["selinux"] == "Enforcing",
            "SELinux was not enforcing, so this says nothing about a real device's policy")

    abi = report["abi"]
    require(abi["exported_symbols"] == 12, "the Android library must export the frozen twelve symbols")
    require(abi["sqlite_symbols_exported"] == 0,
            "a SQLite symbol escaped the Android library, so a host could open the database behind it")

    runtime = report["runtime_x86_64"]
    require(runtime["status"] == "passed", "the runtime phase did not pass")
    require(set(runtime["phases"]) == REQUIRED_PHASES,
            f"phases {sorted(set(REQUIRED_PHASES) - set(runtime['phases']))} were not run")
    require(runtime["checks"] == EXPECTED_CHECKS,
            f"{runtime['checks']} checks ran, expected {EXPECTED_CHECKS}. If the host really "
            "gained or lost one, update EXPECTED_CHECKS and say which.")
    # A SIGKILL leaves a write-ahead log and the ownership marker the process
    # never released. The next open has to cope with both, and recording what
    # was actually left is what makes "it recovered" mean something.
    require(any(name.endswith(".owner") for name in runtime["leftovers_after_sigkill"]),
            "the killed process left no ownership marker, so recovery from one was not exercised")

    interchange = report["desktop_interchange"]
    require(interchange["integrity_check"] == "ok", "the returned database failed integrity_check")
    require(interchange["journal_mode"] == "wal", "the returned database is not in WAL mode")
    require(interchange["seed_sha256"] != interchange["returned_sha256"],
            "the file came back byte-identical, so Android wrote nothing to it")
    require(interchange["notes_android_wrote_that_the_desktop_reads"] >= 2,
            "the desktop could not read what Android wrote")

    arm = report["arm64_build_only"]
    require(arm["runtime_executed"] is False, "an arm64 runtime result is not claimable from this run")
    require(arm["machine"] == "AArch64", "the arm64 artifact is not AArch64")
    require("CANNOT LINK" in arm["refusal_on_this_device"],
            "the build-only claim is not evidenced by the device refusing to run it")

    print(f"H11 acceptance valid: {runtime['checks']} checks on {environment['fingerprint']}, "
          f"schema v{interchange['schema_version']}, arm64 built and not run")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError) as error:
        print(f"H11 acceptance invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
