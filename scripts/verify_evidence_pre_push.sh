#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
reserve_root=${NOTRIOS_EVIDENCE_RESERVE:-/media/renes/SEAGATE2TB/notrios-evidence}
source_root=${NOTRIOS_EVIDENCE_SOURCE:-/home/renes/evidence/notrios}

python3 "$repo_root/evidence/verify_evidence.py" tracked --repo "$repo_root"
python3 "$repo_root/evidence/verify_evidence.py" source \
  --repo "$repo_root" --source-root "$source_root"
python3 "$repo_root/evidence/verify_evidence.py" reserve \
  --repo "$repo_root" --reserve-root "$reserve_root"

# Every volume's content commit must be in the history about to be pushed, not
# just the first one's. This read `json.load` on the catalog, which worked while
# the catalog held one entry and raised "Extra data" the moment volume-0002
# appended a second line -- a gate that stopped running rather than started
# failing, which is the worse of the two.
content_commits=$(python3 -c '
import json, sys
for line in open(sys.argv[1]):
    if line.strip():
        print(json.loads(line)["payload"]["content_commit"])
' "$repo_root/evidence/outer-iso-catalog.jsonl")
test -n "$content_commits"
for content_commit in $content_commits; do
  git -C "$repo_root" merge-base --is-ancestor "$content_commit" HEAD
done
catalog_commit=$(git -C "$repo_root" log -1 --format=%H -- evidence/outer-iso-catalog.jsonl)
test -n "$catalog_commit"
git -C "$repo_root" merge-base --is-ancestor "$catalog_commit" HEAD

printf 'Notrios evidence pre-push gate: complete coverage and reserve verified.\n'
