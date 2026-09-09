#!/usr/bin/env python3
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
CONTRACT = ROOT / "contracts" / "archive-v2"
EVIDENCE = ROOT / "performance" / "v0.7-g19"


def load(path):
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def require(condition, message):
    if not condition:
        raise SystemExit(f"g19 evidence invalid: {message}")


report = load(EVIDENCE / "REPORT.json")
require(report["schema"] == "notrios.g19.report.v1", "report schema")
require(report["status"] == "passed" and report["private_data"] is False, "report status/privacy")
require(report["published_schemas"] == {"draft":"2020-12","count":5,"compiled":5,"structural_samples_valid":True,"semantic_verification_separate":True}, "schema summary")
require(report["positive_goldens"]["verify_directory_passed"] == 3, "positive verification count")
require(report["site_publication"] == {"files":27,"byte_exact":True,"base_path":"/notrios/contracts/archive-v2/"}, "site publication summary")
require(report["physical_refusal"]["manifest_only"] is True, "physical fixture boundary")
require(report["external_consumer"]["status"] == "absent" and report["external_consumer"]["external_writes"] == 0, "external consumer claim")

registry = load(CONTRACT / "contract.json")
require((registry["format"], registry["version"], registry["minimum_schema_version"], registry["current_schema_version"]) == ("notrios-archive", 2, 12, 27), "portable version bounds")
require(set(registry["reader_profiles"]) == {"current-v2", "previous-loose-v2"}, "reader profiles")
require(registry["separate_formats"]["physical_snapshot"]["portable_consumer_decision"] == "refuse", "physical separation")
require(registry["separate_formats"]["sync_wire"]["archive_v2_capability"] is False, "sync separation")

schemas = sorted((CONTRACT / "schemas").glob("*.schema.json"))
require(len(schemas) == 5, "published schema count")
for path in schemas:
    load(path)

matrix = load(CONTRACT / "fixtures" / "fixture-matrix.json")
expected = {"loose-schema12", "packed-schema12", "sync-era-schema27-packed", "previous-explicit-zero-record-counts", "unknown-required-capability", "unknown-optional-capability", "physical-refusal"}
require({item["name"] for item in matrix["fixtures"]} == expected, "fixture matrix coverage")

for name in ("loose-schema12", "packed-schema12", "sync-era-schema27-packed"):
    fixture = CONTRACT / "fixtures" / name
    manifest = load(fixture / "manifest.json")
    require(manifest["format"] == "notrios-archive" and manifest["version"] == 2, f"{name} discriminator")
    require(any(path.is_file() for path in (fixture / "objects").rglob("*")), f"{name} object payload")

physical = CONTRACT / "fixtures" / "physical-refusal"
physical_manifest = load(physical / "manifest.json")
require(physical_manifest["format"] == "notrios-sqlite-image", "physical discriminator")
require(physical_manifest["compatibility"]["semantic_fallback_format"] == "notrios-archive-v2", "physical fallback")
require(not (physical / "notes.sqlite").exists(), "physical fixture contains SQLite")
require(not (physical / "packs").exists(), "physical fixture contains packs")

required = load(CONTRACT / "fixtures" / "unknown-required-capability" / "manifest.json")
optional = load(CONTRACT / "fixtures" / "unknown-optional-capability" / "manifest.json")
require("vendor.required.future.v1" in required["compatibility"]["required_capabilities"], "unknown required probe")
require(optional["compatibility"]["optional_capabilities"] == ["vendor.optional.future.v1"], "unknown optional probe")

for name in ("golden-inputs.json", "goldens.json"):
    published = (CONTRACT / "sync-wire" / name).read_bytes()
    source = (ROOT / "performance" / "v0.7-g9" / name).read_bytes()
    require(published == source, f"sync-wire publication drift: {name}")

audit = load(EVIDENCE / "external-consumer-audit.json")
require(audit["task"] == "v0.7-g19", "external audit task")
require(len(audit["paths"]) == 2 and all(item["notrios2sql_py"] is False and item["notrios_archive_reference"] is False for item in audit["paths"]), "external audit findings")
require("no external repository writes" in audit["method"].lower(), "external write boundary")

for path in CONTRACT.rglob("*.json"):
    load(path)

print("G19 evidence valid: 5 schemas, 3 complete goldens, 12 matrix cases, physical refusal, sync separation, external consumer absent")
