#!/usr/bin/env python3
import json
from collections import defaultdict
from pathlib import Path

log_path = Path('agent/ATTEMPT_LOG.jsonl')
counts = defaultdict(lambda: defaultdict(int))

if log_path.exists():
    for line in log_path.read_text(encoding='utf-8').splitlines():
        if not line.strip():
            continue
        row = json.loads(line)
        counts[row.get('task_id', 'unknown')][row.get('status', 'unknown')] += 1

for task_id, statuses in sorted(counts.items()):
    blocked = statuses.get('blocked', 0)
    total = sum(statuses.values())
    marker = ' LOOP-RISK' if blocked >= 3 else ''
    print(f'{task_id}: total={total} blocked={blocked}{marker}')
