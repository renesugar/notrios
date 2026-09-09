#!/usr/bin/env python3
"""Validate G20's immutable release matrix without network or private data."""
import hashlib
import importlib.util
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
REPORT = ROOT / "performance/v0.7-g20/REPORT.json"

class EvidenceError(ValueError):
    pass

def fail(message):
    raise EvidenceError(message)

def load(path):
    try:
        return json.loads(path.read_text())
    except Exception as exc:
        fail(f"invalid JSON {path}: {exc}")

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

HARDENING_DIMENSIONS = {"security", "performance", "memory", "reliability", "operability", "migration"}
HARDENING_HEADINGS = {
    "Decision", "Executive Recommendation", "Evidence", "Current Design And Failure Mode",
    "Desired Invariants", "Constraints And Non-Goals", "Before Architecture", "Options",
    "Comparison", "Recommendation", "Evidence Coverage And Residual Risk",
    "Migration And Rollout", "Validation Plan", "Implementation Work Packages", "Open Questions",
}

def _safe_repo_path(root, base, value, label):
    if not isinstance(value, str) or not value or Path(value).is_absolute():
        fail(f"hardening {label} is not repository-relative")
    path = Path(value)
    if ".." in path.parts or path != Path(*path.parts):
        fail(f"hardening {label} traverses its root")
    resolved = (base / path).resolve()
    try:
        resolved.relative_to(root.resolve())
    except ValueError:
        fail(f"hardening {label} escapes repository")
    if not resolved.is_file():
        fail(f"hardening {label} is missing: {value}")
    return resolved

def validate_hardening(root, hardening):
    if not isinstance(hardening, dict) or not hardening.get("opportunities"):
        fail("hardening opportunities are missing")
    hardening_dir = root / "performance/v0.7-g20/hardening"
    for opportunity in hardening["opportunities"]:
        proposal = _safe_repo_path(root, hardening_dir, opportunity.get("proposalPath"), "proposalPath")
        headings = {match.group(1).strip() for match in re.finditer(r"^#{1,3} +(.+?)\s*$", proposal.read_text(), re.MULTILINE)}
        missing = HARDENING_HEADINGS - headings
        if missing:
            fail(f"proposal {proposal} misses headings: {sorted(missing)}")
        options = opportunity.get("options")
        if not isinstance(options, list) or not options:
            fail(f"hardening opportunity has no options: {opportunity.get('opportunityId')}")
        option_ids = {option.get("optionId") for option in options}
        recommended = opportunity.get("recommendedOptionId")
        if recommended not in option_ids:
            fail(f"recommended option does not resolve: {recommended}")
        for option in options:
            coverage = option.get("findingCoverage")
            if not isinstance(coverage, list) or not coverage:
                fail(f"option has empty findingCoverage: {option.get('optionId')}")
            tradeoffs = option.get("tradeoffs")
            dimensions = [item.get("dimension") for item in tradeoffs] if isinstance(tradeoffs, list) else []
            if set(dimensions) != HARDENING_DIMENSIONS or len(dimensions) != len(HARDENING_DIMENSIONS):
                fail(f"option tradeoff dimensions are incomplete: {option.get('optionId')}")
            diagrams = option.get("diagramPaths")
            if not isinstance(diagrams, dict):
                fail(f"option diagramPaths missing: {option.get('optionId')}")
            for side in ("before", "after"):
                _safe_repo_path(root, hardening_dir, diagrams.get(side), f"{option.get('optionId')} {side} diagram")

def validate(root=ROOT, report_path=REPORT):
    report = load(report_path)
    if report.get("schema") != "notrios.g20.release-acceptance.v1":
        fail("wrong report schema")
    release = report.get("release", {})
    if release != {"product_version": "0.7.0", "database_schema_version": 27,
                    "v07_schema_steps": list(range(19, 28)), "private_corpus_read": False}:
        fail("release identity or schema history drift")

    # v0.7's release identity, checked against the record rather than against
    # today's source.
    #
    # This read `internal/version/version.go` and required 0.7.0, which was a
    # true statement about the repository until the version moved and a false
    # one for ever after: v0.8 H13 bumped it to 0.8.0 and this record -- whose
    # subject is what v0.7 shipped -- started failing on a fact about v0.8. An
    # evidence record for a finished milestone must not re-derive its subject
    # from the present.
    #
    # What is still worth checking is that the version has not gone *backwards*
    # or sideways into something unparseable, because this record's schema
    # history claims steps 19-27 and a lower version would mean the tree is not
    # the one it describes.
    version_source = (root / "internal/version/version.go").read_text()
    match = re.search(r'const Version = "(\d+)\.(\d+)\.(\d+)"', version_source)
    if not match:
        fail("the Go product version is missing or unparseable")
    current = tuple(int(part) for part in match.groups())
    if current < (0, 7, 0):
        fail(f"the Go product version {'.'.join(map(str, current))} is older than the 0.7.0 this record describes")
    # The frontend's version tracks the Go one, so it gets the same treatment
    # for the same reason: this record describes v0.7 and must not fail because
    # a later milestone shipped.
    web_version = str(load(root / "web/package.json").get("version", ""))
    web_match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)", web_version)
    if not web_match:
        fail(f"the web product version {web_version!r} is missing or unparseable")
    if tuple(int(part) for part in web_match.groups()) < (0, 7, 0):
        fail(f"the web product version {web_version} is older than the 0.7.0 this record describes")
    store_source = (root / "internal/store/store.go").read_text()
    if not re.search(r"CurrentSchemaVersion\s*=\s*27\b", store_source):
        fail("canonical schema is not v27")
    for step in range(19, 28):
        if not list((root / "internal/store/migrations").glob(f"{step:04d}_*.sql")):
            fail(f"missing schema migration v{step}")

    aggregate = report.get("aggregate_evidence", {})
    for name in ("full_scale", "retention", "lazy_resources"):
        item = aggregate.get(name, {})
        path = root / item.get("path", "")
        if not path.is_file() or digest(path) != item.get("sha256"):
            fail(f"{name} evidence missing or hash drifted")
    full = load(root / aggregate["full_scale"]["path"])
    counts = full.get("corpora", {}).get("counts", {})
    expected = aggregate["full_scale"]
    for source_key, report_key in (("equivalent_joplin_documents", "equivalent_documents_per_view"),
                                   ("equivalent_obsidian_documents", "equivalent_documents_per_view"),
                                   ("attachment_documents", "attachment_documents"),
                                   ("attachment_resources", "attachment_resources"),
                                   ("attachment_blobs", "attachment_blobs")):
        if counts.get(source_key) != expected.get(report_key):
            fail(f"full-scale count drift: {source_key}")
    if full.get("claims", {}).get("aggregate_only") is not True:
        fail("full-scale evidence is not aggregate-only")
    retention = load(root / aggregate["retention"]["path"])
    if retention.get("generated_operations") != 382206 or retention.get("source_private_data_read") is not False:
        fail("retention evidence count/privacy drift")
    lazy = load(root / aggregate["lazy_resources"]["path"])
    sizes = lazy.get("sizes", [])
    if [row.get("object_bytes") for row in sizes] != aggregate["lazy_resources"]["sizes_bytes"]:
        fail("lazy-resource sizes drift")
    if any(row.get("attachments") != 8 or row.get("exact_reconstruction") is not True or
           row.get("note_readable_before_bytes") is not True for row in sizes):
        fail("lazy-resource exactness/readability drift")

    gates = report.get("gates", [])
    ids = [gate.get("id") for gate in gates]
    required = {"randomized-three-peer-convergence", "directory-and-rest-processes",
                "catchup-reset-retention", "lazy-attachment-carriers",
                "conflict-and-admission-faults", "security-abuse",
                "fresh-and-upgrade-schema", "aggregate-profiles",
                "documentation-and-contracts", "dependency-licenses", "release-package"}
    if len(ids) != len(set(ids)) or set(ids) != required:
        fail("release gate coverage is incomplete or duplicated")
    if any(not gate.get("command") or gate.get("status") not in {"required", "final-only"} for gate in gates):
        fail("release gate command/status is incomplete")

    advisory = report.get("advisory_review", {})
    advisory_path = root / advisory.get("path", "")
    if digest(advisory_path) != advisory.get("sha256"):
        fail("advisory report hash drift")
    advisory_data = load(advisory_path)
    if advisory_data.get("summary", {}).get("missing_dispositions") not in (None, []):
        fail("advisory findings lack disposition")
    for verdict in advisory_data.get("verdicts", []):
        if verdict.get("verdict") != "supported" and not verdict.get("disposition"):
            fail(f"advisory verdict lacks disposition: {verdict.get('id')}")

    security = report.get("security_scan", {})
    expected_security = {
        "scan_id": "a8529c1e-bc60-472a-ae36-de6067741fdd",
        "target_revision": "a3cea5a39f9c0bfe20e80c7615b503ac09967cce",
        "manifest_sha256": "061c5d52f17208dcd1ec2ebff9f0c2b0abb8048e9c47f5064fe265f71861f661",
        "findings_sha256": "0e84855ae4f18d528d23a2965d88643351a0748ecfa363b5d8f1b78136eaf7ba",
        "coverage_sha256": "f9ac37f7b978fd43e42234cf7de278ca06b9da10c52f64049e2249663ca9b840",
        "report_path": "performance/v0.7-g20/SECURITY_SCAN.md",
        "finding_count": 6,
        "remediations_verified": 6,
        "correction_cycle_completed": True,
        "hardening_analysis": "performance/v0.7-g20/hardening/hardening.json",
    }
    if security != expected_security:
        fail("security scan identity or remediation disposition drift")
    security_text = (root / security["report_path"]).read_text()
    for phrase in (security["scan_id"], security["manifest_sha256"], "Independent correction-cycle review", "six validated"):
        if phrase not in security_text:
            fail(f"security report omits {phrase!r}")
    hardening = load(root / security["hardening_analysis"])
    if hardening.get("analysisId") != "hardening_final" or hardening.get("sourceScan", {}).get("manifestSha256") != security["manifest_sha256"]:
        fail("hardening portfolio is not bound to the sealed scan")
    if hardening.get("assessment", {}).get("outcome") != "opportunities_identified" or len(hardening.get("opportunities", [])) != 2:
        fail("hardening opportunity portfolio is incomplete")
    validate_hardening(root, hardening)

    rollback = (root / "performance/v0.7-g20/UPGRADE_ROLLBACK.md").read_text()
    for phrase in ("no in-place v27-to-v18 downgrade", "verified", "stop all Notrios processes", "notriosctl doctor"):
        if phrase.lower() not in rollback.lower():
            fail(f"rollback contract omits {phrase!r}")

    spec = importlib.util.spec_from_file_location("g20_licenses", root / "performance/v0.7-g20/check_dependency_licenses.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    old_root, old_inventory = module.ROOT, module.INVENTORY
    try:
        module.ROOT = root
        module.INVENTORY = root / "performance/v0.7-g20/DEPENDENCY_LICENSES.json"
        module.validate()
    finally:
        module.ROOT, module.INVENTORY = old_root, old_inventory

    if set(report.get("operations_not_authorized", [])) != {"git-push", "git-tag", "public-release", "evidence-reserve-write", "physical-media-burn"}:
        fail("operational boundary drift")
    return len(gates)

if __name__ == "__main__":
    try:
        count = validate()
        print(f"G20 release evidence valid: 0.7.0/schema-v27, {count} closed gates")
    except EvidenceError as exc:
        print(f"G20 release evidence invalid: {exc}", file=sys.stderr)
        sys.exit(1)
