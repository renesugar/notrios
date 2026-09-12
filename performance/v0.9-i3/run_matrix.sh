#!/usr/bin/env bash
# I3 — promote the Ubuntu installer from "structurally inspected" to
# "natively installed and executed".
#
# H6a built the package and read it: level 2 on its own claim ladder, and it
# said so, listing "whether the .deb installs or runs" among the things it had
# not verified. Nothing has installed it since. This does, in clean Ubuntu
# containers with no source tree, no Go, no Node and no compiler -- which is the
# only environment in which the answer means anything, because a machine that
# built the package can hide every missing dependency.
#
# Each scenario gets a fresh container. The first one deliberately starts from a
# pristine ubuntu:24.04 so that dependency resolution is part of what is tested;
# the rest start from an image with the runtime dependencies already present,
# because re-resolving 200 MB of webkit for every scenario would buy nothing and
# the results record which image each ran on.
#
# The application is run as an unprivileged user rather than as root. An end
# user installs with sudo and then runs as themselves, and a rehearsal that only
# ever ran as root would miss every permission mistake.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/performance/v0.9-i3
PKGS=${I3_PACKAGES:-$ROOT/dist/i3-packages}
RESULTS=${I3_RESULTS:-$HERE/RESULTS.jsonl}
BASE_IMAGE=ubuntu:24.04
DEPS_IMAGE=notrios-i3-deps

RELEASE=$(ls "$PKGS"/notrios_0.8.0-1_amd64.deb)
PRERELEASE=$(ls "$PKGS"/notrios_0.8.0~rc1-1_amd64.deb)

# The unprivileged user every scenario runs the application as.
USER_SETUP='useradd -m -s /bin/bash tester'

prepare_deps_image() {
  if docker image inspect "$DEPS_IMAGE" >/dev/null 2>&1; then return; fi
  echo "== preparing $DEPS_IMAGE (runtime dependencies only, no notrios)" >&2
  local deps
  deps=$(dpkg-deb -f "$RELEASE" Depends | tr ',' '\n' | sed 's/(.*)//' | tr -d ' ' | grep -v '^$' | tr '\n' ' ')
  docker build -q -t "$DEPS_IMAGE" - >/dev/null <<DOCKERFILE
FROM $BASE_IMAGE
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends $deps \
    && rm -rf /var/lib/apt/lists/*
RUN $USER_SETUP
DOCKERFILE
}

# run_scenario <id> <image> <script-file>
run_scenario() {
  local id=$1 image=$2 script=$3
  local log=$HERE/.logs/$id.log
  mkdir -p "$HERE/.logs"
  echo "== $id (on $image)" >&2
  local started ended status
  started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if docker run --rm --network bridge \
      -v "$PKGS:/pkg:ro" -v "$script:/scenario.sh:ro" \
      "$image" bash /scenario.sh > "$log" 2>&1; then
    status=pass
  else
    status=fail
  fi
  ended=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  python3 - "$id" "$image" "$status" "$started" "$ended" "$log" >> "$RESULTS" <<'PY'
import json, sys
scenario, image, status, started, ended, log = sys.argv[1:7]
text = open(log, encoding="utf-8", errors="replace").read()
# Scenarios report their own observations as OBSERVE key=value lines, so the
# record holds what was seen and not only whether the exit code was zero.
observations = {}
for line in text.splitlines():
    if line.startswith("OBSERVE "):
        key, _, value = line[len("OBSERVE "):].partition("=")
        observations[key.strip()] = value.strip()
print(json.dumps({"scenario": scenario, "image": image, "status": status,
                  "started_at": started, "ended_at": ended,
                  "observations": observations,
                  "output_tail": text.strip().splitlines()[-12:]}, sort_keys=True))
PY
  [[ $status == pass ]] || echo "   FAILED -- see $log" >&2
}

: > "$RESULTS"
prepare_deps_image
for script in "$HERE"/scenarios/*.sh; do
  id=$(basename "$script" .sh)
  image=$DEPS_IMAGE
  # The fresh install starts from a pristine image on purpose: whether the
  # declared dependencies are sufficient is exactly what it is measuring.
  [[ $id == 01-fresh-install ]] && image=$BASE_IMAGE
  run_scenario "$id" "$image" "$script"
done

python3 - "$RESULTS" <<'PY'
import json, sys
rows = [json.loads(line) for line in open(sys.argv[1]) if line.strip()]
failed = [r["scenario"] for r in rows if r["status"] != "pass"]
print(f"{len(rows)} scenarios, {len(rows) - len(failed)} passed")
for r in rows:
    print(f"  {r['status']:4}  {r['scenario']}")
sys.exit(1 if failed else 0)
PY
