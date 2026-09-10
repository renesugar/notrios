#!/usr/bin/env python3
"""Run the installed service under continuous REST traffic and watch it grow.

    python3 performance/v0.9-i7/soak.py --minutes 10

A soak is not a load test. Nothing here is trying to find the throughput limit;
it is trying to find the things that only appear with time -- a file descriptor
that is opened per request and never closed, memory that climbs and never
returns, a database handle that accumulates. Those are invisible in a test that
runs for a second and obvious in one that runs for ten minutes, which is why the
measurement is the *slope* rather than the peak.

The duration is recorded rather than described. "Long-lived" means whatever it
actually ran for, and a run that was cut short says so: v0.9's boundary is that
a soak not observed to completion is reported as incomplete rather than
extrapolated.
"""
import argparse
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]


def sample(pid: int) -> dict:
    """Resident memory, open descriptors and thread count, from /proc."""
    status = pathlib.Path(f"/proc/{pid}/status").read_text(encoding="utf-8")
    rss = threads = 0
    for line in status.splitlines():
        if line.startswith("VmRSS:"):
            rss = int(line.split()[1])
        elif line.startswith("Threads:"):
            threads = int(line.split()[1])
    return {"rss_kb": rss, "threads": threads,
            "open_fds": len(list(pathlib.Path(f"/proc/{pid}/fd").iterdir()))}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--minutes", type=float, default=10.0)
    parser.add_argument("--out", type=pathlib.Path,
                        default=ROOT / "performance/v0.9-i7/SOAK.json")
    arguments = parser.parse_args()

    daemon = ROOT / "bin" / "notriosd"
    if not daemon.is_file():
        print("bin/notriosd is missing; run `make build`", file=sys.stderr)
        return 1

    work = pathlib.Path(tempfile.mkdtemp(prefix="notrios-soak-"))
    database = work / "notes.sqlite"
    address = "127.0.0.1:18571"
    process = subprocess.Popen(
        [str(daemon), "-addr", address, "-db", str(database)],
        cwd=work, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
        env=dict(os.environ, HOME=str(work)))

    status_url = f"http://{address}/api/v1/status"
    deadline = time.time() + 60
    while time.time() < deadline:
        try:
            urllib.request.urlopen(status_url, timeout=2).read()
            break
        except (urllib.error.URLError, OSError):
            time.sleep(0.2)
    else:
        process.kill()
        print("the service never became ready", file=sys.stderr)
        return 1

    started = time.time()
    first = sample(process.pid)
    requests = errors = 0
    samples = [{"at_seconds": 0, **first}]
    interrupted = None
    try:
        end = started + arguments.minutes * 60
        next_sample = started + 30
        while time.time() < end:
            try:
                with urllib.request.urlopen(status_url, timeout=5) as response:
                    response.read()
                requests += 1
            except (urllib.error.URLError, OSError):
                errors += 1
            if process.poll() is not None:
                interrupted = "the service exited during the soak"
                break
            if time.time() >= next_sample:
                samples.append({"at_seconds": round(time.time() - started), **sample(process.pid)})
                next_sample += 30
    except KeyboardInterrupt:
        interrupted = "interrupted by the operator"
    finally:
        ran_for = round(time.time() - started, 1)
        if process.poll() is None:
            samples.append({"at_seconds": round(ran_for), **sample(process.pid)})
            process.terminate()
            try:
                process.wait(timeout=20)
            except subprocess.TimeoutExpired:
                process.kill()
        stderr = process.stderr.read().decode("utf-8", "replace") if process.stderr else ""
        shutil.rmtree(work, ignore_errors=True)

    complete = interrupted is None and ran_for >= arguments.minutes * 60 - 5
    record = {
        "schema": "notrios.v09.i7-soak.v1",
        "target": "installed service over REST, loopback only",
        "requested_minutes": arguments.minutes,
        "ran_for_seconds": ran_for,
        "complete": complete,
        "incomplete_because": interrupted,
        "requests": requests, "errors": errors,
        "samples": samples,
        "growth": {
            "rss_kb": samples[-1]["rss_kb"] - samples[0]["rss_kb"],
            "open_fds": samples[-1]["open_fds"] - samples[0]["open_fds"],
            "threads": samples[-1]["threads"] - samples[0]["threads"],
        },
        "what_this_measures":
            "The slope, not the peak. A descriptor opened per request and never closed, or memory "
            "that climbs and never returns, is invisible in a one-second test and plain over "
            "minutes. Throughput is deliberately not measured: this is not a load test.",
        "daemon_stderr_tail": stderr.strip().splitlines()[-5:],
    }
    arguments.out.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"soak {'complete' if complete else 'INCOMPLETE'}: {ran_for}s, {requests} requests, "
          f"{errors} errors, rss {record['growth']['rss_kb']:+} kB, "
          f"fds {record['growth']['open_fds']:+}, threads {record['growth']['threads']:+}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
