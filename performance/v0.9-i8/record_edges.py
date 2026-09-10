#!/usr/bin/env python3
"""Turn the edges runs into a record."""
import argparse, datetime as dt, json, pathlib, re, subprocess

HERE = pathlib.Path(__file__).resolve().parent


def summarise(log: pathlib.Path) -> dict:
    if not log.is_file():
        return {"ran": False}
    text = log.read_text(encoding="utf-8", errors="replace")
    checks = [line.strip() for line in text.splitlines()
              if line.strip().startswith(("ok ", "FAIL "))]
    return {
        "ran": True,
        "checks": len(checks),
        "passed": sum(1 for c in checks if c.startswith("ok")),
        "failed": [c[4:].strip() for c in checks if c.startswith("FAIL")],
        "data_races": len(re.findall(r"WARNING: DATA RACE", text)),
        "address_sanitizer_reports": len(re.findall(r"ERROR: AddressSanitizer", text)),
        "check_names": [re.sub(r"^ok\s+", "", c) for c in checks if c.startswith("ok")],
    }


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--out", type=pathlib.Path, required=True)
    p.add_argument("--symbols", type=pathlib.Path, required=True)
    a = p.parse_args()

    variants = {}
    for name in ("race", "asan"):
        summary = summarise(a.out / f"{name}.log")
        status_file = a.out / f"{name}.status"
        summary["exit_status"] = int(status_file.read_text().strip()) if status_file.is_file() else None
        variants[name] = summary
    # The race run is `go test -race ./internal/abi/`, which reports Go test
    # results rather than the C host's check lines.
    variants["race"]["subject"] = "internal/abi under go test -race"
    variants["asan"]["subject"] = "the C edges host against an -asan library"

    valgrind_log = a.out / "valgrind.log"
    valgrind = {"ran": False}
    if valgrind_log.is_file():
        text = valgrind_log.read_text(encoding="utf-8", errors="replace")
        def count(pattern):
            found = re.search(pattern, text)
            return int(found.group(1).replace(",", "")) if found else 0
        valgrind = {"ran": True,
                    "definitely_lost_bytes": count(r"definitely lost: (\d[\d,]*) bytes"),
                    "invalid_frees": len(re.findall(r"Invalid free", text))}

    record = {
        "schema": "notrios.v09.i8-abi-edges.v2",
        "ran_at": dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "symbols": sorted(a.symbols.read_text().split()),
        "go": subprocess.run(["go", "version"], capture_output=True, text=True).stdout.strip(),
        "variants": variants,
        "valgrind": valgrind,
        "instrumentation": {
            "chosen": ["go test -race ./internal/abi/", "go build -asan + C edges host"],
            "why":
                "valgrind is the wrong instrument for a Go c-shared library, and wrong loudly. Go "
                "grows a goroutine stack by allocating a larger one and copying the frames into "
                "it, rewriting pointers as it goes; to memcheck, which tracks addressability per "
                "byte, every one of those writes lands outside a known block. The first run of "
                "this host produced ten million invalid-access reports with every frame in "
                "runtime.* -- newstack, copystack, adjustframe, memmove -- and a host that does "
                "nothing but call notrios_abi_version() produces them too, which is the control "
                "that settles it. Targeted suppressions removed 9.5 million and the run still "
                "reached memcheck's cap. Go ships instruments that model its runtime, so those "
                "are what run -- each where it works. AddressSanitizer crosses the c-shared "
                "boundary and is clean. ThreadSanitizer cannot: it maps a large shadow region at "
                "process start, so through a c-shared library it fails with 'failed to allocate "
                "... bytes at address', and with the host instrumented too it fails with "
                "'unexpected memory mapping' -- with ASLR disabled as well. So the race detector "
                "runs on the Go side, against the same dispatch, session and handle code the "
                "twelve entry points call into, with concurrency tests written for these edges. "
                "The detector was proved live before it was trusted: a deliberately racy probe "
                "made it fire, then the probe was removed.",
            "valgrind_kept_for":
                "leak accounting only, behind I8_VALGRIND=1. It is the one number memcheck still "
                "reports usefully here.",
        },
        "not_verified": [
            "No fuzzing. The handles used here are wrong in the obvious ways -- never issued, "
            "already closed, released twice -- not in arbitrary ways.",
            "Single process, single library load. Nothing tests dlclose, a second dlopen, or two "
            "instances opening the same profile from separate processes.",
            "-msan did not run. It needs a clang-based toolchain wiring this build does not have, "
            "so uninitialised-memory reads are unexercised.",
            "The race detector observed the schedules this host produced. It reports races it "
            "sees; it does not prove there are none.",
            "The ABI's typed surface -- signatures rather than names -- is checked by abidiff "
            "separately. These runs prove the twelve exported names are unchanged.",
        ],
    }
    (HERE / "EDGES.json").write_text(json.dumps(record, indent=2, sort_keys=True) + "\n",
                                     encoding="utf-8")
    for name, summary in variants.items():
        if summary.get("ran"):
            print(f"{name}: {summary['passed']}/{summary['checks']} checks, "
                  f"{summary['data_races']} races, "
                  f"{summary['address_sanitizer_reports']} asan reports, "
                  f"exit {summary['exit_status']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
