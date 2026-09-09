# v0.7 G1 — representative divergence and revision-delta workload

G1 is an investigation, not synchronization runtime code. It measures
aggregate body/resource shapes from three available corpora, generates a
deterministic divergence workload for the approved offline intervals, compares
complete bodies with line, word, and byte-span transfer deltas, classifies
three-way merges at line/word/byte granularity, and probes one permissive Go
merge candidate without adding it to `go.mod`.

The 2026-08-11 G1a plan amendment preserves these measurements but supersedes
the external merge-candidate recommendation. G7 will own the bounded pure-Go
line/word merge implementation; G1a separately investigates binary-safe
xdelta/VCDIFF encoding in pure Go.

## Reproduce

The committed corpus output is aggregate-only. The profiler accepts source
paths but writes only caller-supplied labels, counts, byte distributions,
item-type counts, and parse-quality counters. It never writes paths, names,
titles, bodies, source-content hashes, MIME names, or database bytes.

```bash
python3 performance/v0.7-g1/corpus_profile.py \
  --corpus 'label:joplin:/path/to/joplin-export:.md' \
  --corpus 'label:plain:/path/to/markdown-corpus:.md' \
  --output /tmp/g1-corpus-profile.json

python3 performance/v0.7-g1/workload.py \
  --output /tmp/g1-workload-results.json

PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s performance/v0.7-g1 -p 'test_*.py' -v

python3 performance/v0.7-g1/validate_evidence.py
```

The full workload took about two minutes on the recorded host because every
method runs in a fresh process and each ordinary measurement repeats five
times. CPU timings are comparative evidence from that host, not protocol
budgets for other devices.

## Evidence files

- `corpus-profile.json` — aggregate-only complete-corpus size observations;
- `scenarios.json` — frozen divergence, metadata, interval, and rejection
  matrix;
- `workload-results.json` — ratios, CPU, peak RSS, classifications, and exact
  reconstruction hashes;
- `upstream-probe.json` — exact candidate commit, license, size, and probe
  result;
- `FINDINGS.md` — the selected G7 body operation and conflict model;
- `corpus_profile.py`, `workload.py`, and `test_workload.py` — reproducible
  generators and focused tests;
- `validate_evidence.py` — completeness/privacy/invariant validator.

## Privacy and interpretation limits

The Joplin, recipe, and Twitter inputs are private static corpora. Only the
aggregate output is committed. They describe current-object sizes; they do not
contain independent-device editing history, so they cannot estimate human
concurrency, conflict frequency, or a real revision-chain distribution. The
one-hour/day/week/30-day chains are deterministic synthetic workloads reported
separately. That limitation is a result, not missing evidence.

No production merge engine, synchronization code, schema migration, runtime
dependency, private corpus, database, or resource was added by G1.
