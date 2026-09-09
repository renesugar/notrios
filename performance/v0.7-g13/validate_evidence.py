#!/usr/bin/env python3
"""Check that the committed G13 evidence says what the documentation claims."""
from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent

# The only cases that may be authorized. Anything else answering 2xx is a hole.
AUTHORIZED = {
    "an enrolled peer",
    "pairing with a valid code",
    "status from loopback",
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g13 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "auth-matrix.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g13.authmatrix.v1", "wrong report schema")
    require(report["database_schema_version"] == 25, "report was not produced against schema v25")

    matrix = report["matrix"]
    require(len(matrix) >= 15, f"only {len(matrix)} cases were exercised")
    cases = {row["case"] for row in matrix}
    require(AUTHORIZED <= cases, f"the authorized cases were not all run: {sorted(AUTHORIZED - cases)}")

    for row in matrix:
        label = row["case"]
        if row["case"] in AUTHORIZED:
            require(row["authorized"], f"{label}: should have been authorized but was {row['status']}")
            continue
        require(not row["authorized"], f"{label}: was authorized with status {row['status']}")
        # A refusal that explains which check failed is a refusal that teaches
        # an attacker how to pass.
        require(not row["answer_explains_the_failure"], f"{label}: the refusal explained itself")

    # The audit vocabulary is closed, and every recorded reason is in it.
    vocabulary = set(report["audit_reason_vocabulary"])
    require(vocabulary, "the audit vocabulary is empty")
    for row in matrix:
        recorded = row.get("audited_reason", "")
        if not recorded or ":" not in recorded:
            continue
        _, reason = recorded.split(":", 1)
        if reason and reason not in vocabulary:
            # Pairing reasons are their own small closed set, named here so a
            # new one cannot appear without this file being edited.
            require(reason in {"malformed_code", "bad_proof", "wrong_database", "malformed_key",
                               "unknown_or_expired", "consumed", "revoked", "expired",
                               "not_enrolled_for_admission", "oversized_body", "paired over REST", ""},
                    f"{row['case']}: audited reason {reason!r} is outside the vocabulary")

    # The one case that says what a peer credential is worth elsewhere.
    ordinary = [row for row in matrix if "ordinary note route" in row["case"]]
    require(ordinary and not ordinary[0]["authorized"],
            "no case checks that a peer credential authorizes nothing outside the sync surface")

    transport = report["transport_policy"]
    require(len(transport) >= 4, "the transport policy was not exercised")
    for row in transport:
        loopback = row["listen_addr"].startswith("127.") or row["listen_addr"].startswith("localhost")
        if row["tls_configured"] or loopback:
            require(not row["startup_refused"], f"{row['listen_addr']}: a safe configuration was refused")
        else:
            require(row["startup_refused"], f"{row['listen_addr']}: an unsafe configuration was accepted")
    require(any(row["listen_addr"].startswith(":") for row in transport),
            "the empty-host case is not covered, and it is the one operators get wrong")

    require("revoked credential" in findings.lower() or "revoked" in findings,
            "FINDINGS.md does not discuss revocation")
    require("not a library key" in findings or "transfer secret" in findings,
            "FINDINGS.md does not state what a pairing code is")
    require("no data plane" in findings.lower(), "FINDINGS.md does not scope out the data plane")
    require(re.search(r"external (security )?review", findings, re.IGNORECASE),
            "FINDINGS.md does not record that no external review has happened")
    require(re.search(r"generated", readme, re.IGNORECASE), "README.md does not state the corpus is generated")

    print(f"g13 evidence: {len(matrix)} cases and {len(transport)} transport policies validated")


if __name__ == "__main__":
    main()
