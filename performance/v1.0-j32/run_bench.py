#!/usr/bin/env python3
"""Measure imports for v1.0 J32: one import per process, alternating binaries.

    python3 performance/v1.0-j32/run_bench.py build <commit> <work-dir> <label>
    python3 performance/v1.0-j32/run_bench.py run <work-dir> <out.json> <case> \
        <label>[,<label>] [--runs N]

`build` compiles the harness test binary (cmd/notriosctl's TestJ32ImportBench)
at a commit, from a detached worktree so the checkout is never touched, into
<work-dir>/bin/<label>.test.

`run` measures one case -- a corpus shape and a scenario, such as
obsidian-10k/fresh or joplin-10k/reimport -- for one binary (a baseline) or two
(a baseline and a candidate). Every binary gets one unmeasured warm-up, then the
binaries alternate run by run so drift in the machine lands on both. Each run
is a fresh process of the test binary under /usr/bin/time, with HOME and every
XDG root inside the run's own directory, so no owner configuration, keychain or
shared temp directory is read or written.

Scenarios:

  fresh     import the corpus into an empty library
  reimport  import it again into the library a fresh import produced; that
            library is made once per binary and copied before every run, so
            each measured reimport starts from the same bytes

Before each run the one-minute load average must fall to 1.0 or below (J19
found concurrent work moved timings by 43%); the runner waits for it and
records it. The corpus manifest is checked before and after, so a run that
read different bytes, or wrote into the corpus, is refused.

The output holds every value of every metric, and per binary the median,
minimum, maximum and range. compare.py applies the J32 rule to it.
"""
from __future__ import annotations

import json
import os
import pathlib
import shutil
import statistics
import subprocess
import sys
import time

ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import make_corpus  # noqa: E402

LOAD_CEILING = 1.0
LOAD_WAIT_SECONDS = 1800
METRICS = ("wall_seconds", "process_wall_seconds", "user_seconds", "system_seconds",
           "peak_rss_kib", "total_alloc_bytes", "mallocs", "live_heap_bytes",
           "prepared_statements", "commits", "hashed_bytes", "hash_calls",
           "notes_per_second")


def build(commit: str, work: pathlib.Path, label: str) -> int:
    binary = work / "bin" / f"{label}.test"
    binary.parent.mkdir(parents=True, exist_ok=True)
    sha = subprocess.run(["git", "-C", str(ROOT), "rev-parse", "--verify", commit + "^{commit}"],
                         check=True, capture_output=True, text=True).stdout.strip()
    tree = work / "worktrees" / label
    if tree.exists():
        subprocess.run(["git", "-C", str(ROOT), "worktree", "remove", "--force", str(tree)], check=True)
    subprocess.run(["git", "-C", str(ROOT), "worktree", "add", "--detach", str(tree), sha],
                   check=True, capture_output=True)
    try:
        subprocess.run(["go", "test", "-c", "-o", str(binary), "./cmd/notriosctl"],
                       cwd=tree, check=True)
    finally:
        subprocess.run(["git", "-C", str(ROOT), "worktree", "remove", "--force", str(tree)], check=True)
    (work / "bin" / f"{label}.json").write_text(json.dumps(
        {"label": label, "commit": sha, "built_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
         "go": subprocess.run(["go", "version"], capture_output=True, text=True).stdout.strip()},
        indent=2) + "\n", encoding="utf-8")
    print(f"built {binary} at {sha}")
    return 0


def load_average() -> float:
    return float(pathlib.Path("/proc/loadavg").read_text().split()[0])


def wait_for_quiet() -> float:
    deadline = time.monotonic() + LOAD_WAIT_SECONDS
    while (load := load_average()) > LOAD_CEILING:
        if time.monotonic() > deadline:
            raise SystemExit(f"load average stayed above {LOAD_CEILING} for {LOAD_WAIT_SECONDS}s; "
                             "something else is running")
        time.sleep(15)
    return load


def isolated_environment(run: pathlib.Path) -> dict[str, str]:
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(("NOTRIOS_", "XDG_")) and key != "DBUS_SESSION_BUS_ADDRESS"}
    home = run / "home"
    for name in ("config", "data", "cache", "state"):
        (home / name).mkdir(parents=True, exist_ok=True)
    (run / "tmp").mkdir(exist_ok=True)
    env.update(HOME=str(home), XDG_CONFIG_HOME=str(home / "config"), XDG_DATA_HOME=str(home / "data"),
               XDG_CACHE_HOME=str(home / "cache"), XDG_STATE_HOME=str(home / "state"),
               TMPDIR=str(run / "tmp"), GOGC=os.environ.get("GOGC", "100"))
    env.pop("GOMEMLIMIT", None)
    return env


def import_once(binary: pathlib.Path, kind: str, corpus: pathlib.Path, run: pathlib.Path) -> dict:
    """One import in one process. The library is run/library, which may already exist."""
    library = run / "library"
    library.mkdir(parents=True, exist_ok=True)
    env = isolated_environment(run)
    args = ["import", kind, "--db", str(library / "notes.sqlite"),
            "--asset-store", str(library / "assets"), str(corpus)]
    env.update(NOTRIOS_J32_BENCH_ARGS=json.dumps(args),
               NOTRIOS_J32_BENCH_STDOUT=str(run / "import.out"),
               NOTRIOS_J32_BENCH_REPORT=str(run / "measure.json"))
    started = time.monotonic()
    with open(run / "stderr.txt", "wb") as stderr:
        done = subprocess.run(["/usr/bin/time", "-f", "%M %e %U %S", "-o", str(run / "time.txt"),
                               str(binary), "-test.run", "^TestJ32ImportBench$", "-test.count", "1"],
                              cwd=run, env=env, stdout=subprocess.DEVNULL, stderr=stderr)
    elapsed = time.monotonic() - started
    if done.returncode != 0:
        tail = (run / "stderr.txt").read_text(errors="replace")[-2000:]
        raise SystemExit(f"import failed ({done.returncode}) in {run}:\n{tail}")
    peak, wall, user, system = (run / "time.txt").read_text().split()[-4:]
    measured = json.loads((run / "measure.json").read_text())
    output = (run / "import.out").read_text()
    result = json.loads(output[output.index("\n{") + 1:] if not output.startswith("{") else output)
    changed = sum(result.get(key, 0) for key in ("notes_imported", "notes_updated"))
    seen = result.get("markdown_seen") or result.get("notes_seen") or changed
    return {
        **measured,
        "process_wall_seconds": float(wall),
        "harness_seconds": round(elapsed, 3),
        "user_seconds": float(user),
        "system_seconds": float(system),
        "peak_rss_kib": int(peak),
        "notes_seen": seen,
        "notes_changed": changed,
        "notes_per_second": round(seen / measured["wall_seconds"], 3) if measured["wall_seconds"] else None,
        "import": {key: value for key, value in result.items() if key != "warnings"},
        "warnings": len(result.get("warnings") or []),
    }


def summarise(values: list[float]) -> dict:
    return {"values": values, "median": statistics.median(values), "min": min(values),
            "max": max(values), "range": max(values) - min(values)}


def run(work: pathlib.Path, out: pathlib.Path, case: str, labels: list[str], runs: int) -> int:
    shape, _, scenario = case.partition("/")
    if shape not in make_corpus.SHAPES or scenario not in ("fresh", "reimport"):
        print(f"case must be <shape>/<fresh|reimport>; shapes: {', '.join(make_corpus.SHAPES)}",
              file=sys.stderr)
        return 2
    manifest = make_corpus.build(work, shape)
    kind = manifest["kind"]
    corpus = work / "corpora" / shape
    binaries = {}
    for label in labels:
        binary = work / "bin" / f"{label}.test"
        if not binary.is_file():
            print(f"no binary {binary}; build it first", file=sys.stderr)
            return 2
        binaries[label] = (binary, json.loads((work / "bin" / f"{label}.json").read_text()))

    scratch = work / "runs" / f"{shape}-{scenario}"
    if scratch.exists():
        shutil.rmtree(scratch)
    scratch.mkdir(parents=True)

    seeds = {}
    if scenario == "reimport":
        for label, (binary, _) in binaries.items():
            seed = scratch / f"seed-{label}"
            import_once(binary, kind, corpus, seed)
            seeds[label] = seed / "library"

    def one(label: str, name: str) -> dict:
        target = scratch / name
        if target.exists():
            shutil.rmtree(target)
        target.mkdir()
        if label in seeds:
            shutil.copytree(seeds[label], target / "library")
        load = wait_for_quiet()
        measured = import_once(binaries[label][0], kind, corpus, target)
        measured["load_average_before"] = load
        shutil.rmtree(target / "library")
        return measured

    for label in labels:
        one(label, f"warmup-{label}")
        print(f"{case} {label}: warm-up done", file=sys.stderr)
    results: dict[str, list[dict]] = {label: [] for label in labels}
    for index in range(runs):
        order = labels if index % 2 == 0 else list(reversed(labels))
        for label in order:
            measured = one(label, f"run-{index}-{label}")
            results[label].append(measured)
            print(f"{case} {label} run {index + 1}/{runs}: {measured['wall_seconds']:.2f}s "
                  f"rss {measured['peak_rss_kib']} KiB, {measured['prepared_statements']} statements",
                  file=sys.stderr)

    if make_corpus.manifest_of(corpus) != manifest["content"]:
        raise SystemExit(f"{corpus} changed while it was being measured")
    for seed in seeds.values():
        shutil.rmtree(seed.parent)

    record = {
        "schema": "notrios.j32.bench.v1",
        "case": case,
        "corpus": manifest,
        "runs_per_binary": runs,
        "host": {"cpus": os.cpu_count(), "kernel": os.uname().release,
                 "work_filesystem": subprocess.run(["findmnt", "-no", "SOURCE,FSTYPE", "--target", str(work)],
                                                   capture_output=True, text=True).stdout.strip()},
        "binaries": {label: info for label, (_, info) in binaries.items()},
        "results": {
            label: {
                "summary": {metric: summarise([row[metric] for row in rows])
                            for metric in METRICS if all(row.get(metric) is not None for row in rows)},
                "runs": rows,
            }
            for label, rows in results.items()
        },
    }
    out.parent.mkdir(parents=True, exist_ok=True)
    # The work directory's absolute path names the machine and the account it
    # ran under; the record keeps only its place.
    out.write_text(json.dumps(record, indent=2).replace(str(work), "<work>") + "\n", encoding="utf-8")
    print(f"wrote {out}", file=sys.stderr)
    return 0


def main() -> int:
    arguments = sys.argv[1:]
    if len(arguments) == 4 and arguments[0] == "build":
        return build(arguments[1], pathlib.Path(arguments[2]).resolve(), arguments[3])
    if len(arguments) >= 5 and arguments[0] == "run":
        runs = 5
        if "--runs" in arguments:
            position = arguments.index("--runs")
            runs = int(arguments[position + 1])
            del arguments[position:position + 2]
        if runs < 5:
            print("J32 requires at least five measured runs", file=sys.stderr)
            return 2
        return run(pathlib.Path(arguments[1]).resolve(), pathlib.Path(arguments[2]).resolve(),
                   arguments[3], arguments[4].split(","), runs)
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    os.umask(0o077)
    raise SystemExit(main())
