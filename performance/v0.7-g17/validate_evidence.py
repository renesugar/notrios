#!/usr/bin/env python3
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
data = json.loads((ROOT / "RETENTION_COST.json").read_text())

assert data["schema"] == "notrios.g17.retention-cost.v1"
assert data["schema_version"] == 27
assert data["published_corpus_documents"] == 382206
assert data["generated_operations"] == data["published_corpus_documents"]
assert data["full_corpus_churn_equivalent"] == 1
assert data["incremental_bytes"] == data["retained_database_bytes"] - data["baseline_database_bytes"]
assert 400 < data["bytes_per_operation"] < 500
assert data["ninety_day_projected_bytes"] > 0
assert data["source_private_data_read"] is False
assert (ROOT / "README.md").is_file()
print("g17 evidence: full-corpus retention cost and 90-day projection validated")
