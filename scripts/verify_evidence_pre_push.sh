#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
reserve_root=${NOTRIOS_EVIDENCE_RESERVE:-/media/renes/SEAGATE2TB/notrios-evidence}

python3 "$repo_root/evidence/verify_evidence.py" tracked --repo "$repo_root"
python3 "$repo_root/evidence/verify_evidence.py" reserve \
  --repo "$repo_root" --reserve-root "$reserve_root"

content_commit=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["payload"]["content_commit"])' \
  "$repo_root/evidence/outer-iso-catalog.jsonl")
git -C "$repo_root" merge-base --is-ancestor "$content_commit" HEAD

printf 'Notrios evidence pre-push gate: complete coverage and reserve verified.\n'
