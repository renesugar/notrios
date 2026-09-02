#!/usr/bin/env python3
"""Strict, model-free validation of the committed G18f evidence bundle."""

import hashlib
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
VERDICTS = ("supported", "contradicted", "not-determinable")
HEX64 = re.compile(r"^[0-9a-f]{64}$")
DIRECTIVE = re.compile(r"//notrios:doc\s+(user|api)\s+([a-z][a-z0-9]*(?:-[a-z0-9]+)*)")
MARKER = re.compile(r"<!-- notrios:generated:(user|api):([a-z0-9][a-z0-9-]*):(begin|end) -->")


class EvidenceError(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def load_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def validate_templates(root=ROOT):
    template = load_json(root / "docs/docgen/templates.json")
    require(template.get("schema") == "notrios.docgen.templates.v1", "wrong template schema")
    pages = template.get("pages")
    require(isinstance(pages, list) and pages, "template pages missing")
    page_paths, section_keys, slot_ids, slots = set(), set(), set(), []
    for page in pages:
        path = page.get("path")
        require(isinstance(path, str) and path.startswith("docs/") and ".." not in Path(path).parts, "unsafe template path")
        require(path not in page_paths and (root / path).is_file(), "duplicate or missing template page")
        page_paths.add(path)
        sections = page.get("sections")
        require(isinstance(sections, list) and sections, f"missing sections for {path}")
        for section in sections:
            slug = section.get("slug")
            key = (path, slug)
            require(isinstance(slug, str) and slug and key not in section_keys, "duplicate or empty page section")
            section_keys.add(key)
            for slot in section.get("slots", []):
                ident, audience = slot.get("id"), slot.get("audience")
                require(isinstance(ident, str) and ident and ident not in slot_ids, "duplicate or empty slot id")
                require(audience in ("user", "api"), "invalid slot audience")
                slot_ids.add(ident)
                slots.append((ident, audience, path, slug))
    require(len(slots) == 15, "expected 15 slots")
    require(sum(item[1] == "user" for item in slots) == 13 and sum(item[1] == "api" for item in slots) == 2, "expected 13 user/2 api slots")
    return template, slots


def source_fragments(root=ROOT):
    found = []
    for path in root.rglob("*.go"):
        if path.name.endswith("_test.go") or any(part in (".git", "node_modules", "dist") for part in path.parts):
            continue
        found.extend((match.group(2), match.group(1)) for match in DIRECTIVE.finditer(path.read_text(encoding="utf-8")))
    require(len(found) == 15 and len({item[0] for item in found}) == 15, "expected 15 unique production source fragments")
    return dict(found)


def section_body(markdown, slug):
    heading = re.compile(r"^(#{1,3})\s+(.+?)\s*$", re.MULTILINE)
    matches = list(heading.finditer(markdown))
    for index, match in enumerate(matches):
        title_slug = re.sub(r"[^a-z0-9]+", "-", match.group(2).lower()).strip("-")
        if title_slug == slug:
            end = matches[index + 1].start() if index + 1 < len(matches) else len(markdown)
            return markdown[match.end():end]
    raise EvidenceError(f"missing generated section {slug}")


def validate_generated(template, slots, root=ROOT):
    fragments = source_fragments(root)
    declared = {item[0]: item[1] for item in slots}
    require(fragments == declared, "template/source fragment identity or audience mismatch")
    grouped = {}
    for ident, audience, path, slug in slots:
        grouped.setdefault((path, slug, audience), []).append(ident)
    for page in template["pages"]:
        text = (root / page["path"]).read_text(encoding="utf-8")
        for section in page["sections"]:
            body = section_body(text, section["slug"])
            for audience in ("user", "api"):
                expected = grouped.get((page["path"], section["slug"], audience), [])
                begin = f"<!-- notrios:generated:{audience}:{section['slug']}:begin -->"
                end = f"<!-- notrios:generated:{audience}:{section['slug']}:end -->"
                count = 1 if expected else 0
                require(text.count(begin) == count and text.count(end) == count, f"marker mismatch {page['path']}/{section['slug']}/{audience}")
                if expected:
                    start, finish = body.find(begin), body.find(end)
                    require(0 <= start < finish, f"generated block outside declared section {page['path']}/{section['slug']}")
                    block = body[start + len(begin):finish]
                    require(len(MARKER.findall(block)) == 0, "nested generated marker")
                    require(block.count("<!-- source: ") == len(expected), f"source comment count mismatch {page['path']}/{section['slug']}/{audience}")


def validate_report(root=ROOT, here=HERE):
    report = load_json(here / "REPORT.json")
    require(report.get("schema") == "notrios.g18f.deterministic.v1", "wrong deterministic report schema")
    # 413 -> 419 in v0.8 H4 slice C: data.state_dir, data.cache_dir and
    # data.runtime_dir each add a row to the Config listing and a row to the
    # Default listing.
    expected_summary = {"fragments": 15, "user": 13, "api": 2, "generated_documents": 5, "enumerated_rows": 421}
    require(report.get("summary") == expected_summary, "wrong deterministic summary")
    expected_enumerations = {
        # 59 -> 62 and 49 -> 52 for the same three keys.
        "configuration_keys": 62, "configuration_defaults": 52, "cli_usage_forms": 58,
        "rest_openapi_operations": 109, "mcp_tools": 46, "mcp_sync_scope_assignments": 7,
        "mcp_scopes": 4, "mcp_tool_scope_assignments": 46, "gui_journeys": 37,
    }
    require(report.get("enumerations") == expected_enumerations, "wrong enumeration parity counts")
    documents = report.get("documents")
    require(isinstance(documents, list) and len(documents) == 5, "expected five deterministic documents")
    require(len({item.get("path") for item in documents}) == 5, "duplicate deterministic document")
    for item in documents:
        path = item.get("path", "")
        require(path.startswith("docs/") and ".." not in Path(path).parts and (root / path).is_file(), "invalid deterministic document path")
        require(item.get("sha256") == sha256((root / path).read_bytes()), f"stale generated hash {path}")
    freshness = report.get("freshness", {})
    require(freshness.get("test", "").endswith("#TestDocsAreCurrent") and freshness.get("mutation_test", "").endswith("#TestAudienceSeparationAndFreshness"), "freshness test anchors missing")
    return report


def fixture_ids(root=ROOT):
    registry = load_json(root / "docs/docaudit/registry.json")
    journeys = load_json(root / "performance/v0.7-g18e/JOURNEYS.json")
    return ({item["id"]: (item["state"], item.get("execution", {}).get("surface")) for item in registry["executables"]},
            {item["id"]: item["state"] for item in journeys["journeys"]})


def walk_keys(value):
    if isinstance(value, dict):
        for key, child in value.items():
            yield key
            yield from walk_keys(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk_keys(child)


def validate_run(run):
    require(run.get("verdict") in VERDICTS, "invalid verdict")
    require(run.get("conflict_answer") in ("yes", "no"), "invalid conflict answer")
    if run["conflict_answer"] == "yes":
        require(run["verdict"] == "contradicted" and "support_answer" not in run, "conflict mapping drift")
    else:
        require(run.get("support_answer") in ("yes", "no"), "missing support answer")
        expected = "supported" if run["support_answer"] == "yes" else "not-determinable"
        require(run["verdict"] == expected, "support mapping drift")
    require(isinstance(run.get("blind_explanation"), str) and run["blind_explanation"].strip(), "missing blind explanation")
    require(isinstance(run.get("model"), str) and run["model"].endswith(".gguf"), "unrecorded local model")
    for key in ("source_sha256", "explanation_prompt_sha256", "verdict_prompt_sha256"):
        require(HEX64.fullmatch(run.get(key, "")), f"invalid {key}")
    require(run.get("tokens_evaluated", 0) > 0 and run.get("tokens_predicted", 0) > 0, "missing token accounting")
    require(run.get("prompt_ms", 0) > 0 and run.get("predicted_ms", 0) > 0, "missing timing accounting")


def validate_advisory(slots, root=ROOT, here=HERE):
    report = load_json(here / "ADVISORY_REPORT.json")
    require(report.get("schema") == "notrios.doccheck.advisory.v1", "wrong advisory schema")
    expected_policy = {"endpoint_scope": "loopback-only", "source_scope": "repository-source-only; no notes or databases", "automatic_edit": False, "build_blocking": False, "cost_usd": 0}
    require(report.get("policy") == expected_policy, "advisory policy violation")
    require(not ({"prompt", "source", "source_text"} & set(walk_keys(report))), "raw prompt or source text present")

    calibration = report.get("calibration", {})
    cases = calibration.get("runs")
    require(isinstance(cases, list) and len(cases) == 8 and len({item.get("id") for item in cases}) == 8, "calibration must contain eight unique cases")
    expected_ids = {item["id"]: item["expected"] for item in load_json(root / "performance/v0.7-g18a/CALIBRATION.json")["cases"]}
    matrix = {expected: {actual: 0 for actual in VERDICTS} for expected in VERDICTS}
    total = correct = 0
    for case in cases:
        require(case.get("expected") == expected_ids.get(case.get("id")), "calibration label drift")
        runs = case.get("runs")
        require(isinstance(runs, list) and len(runs) >= 2, "calibration repeat missing")
        for run in runs:
            validate_run(run)
            matrix[case["expected"]][run["verdict"]] += 1
            total += 1
            correct += run["verdict"] == case["expected"]
    require(calibration.get("confusion_matrix") == matrix and calibration.get("total") == total and calibration.get("correct") == correct, "calibration matrix drift")
    require(set(calibration.get("negation_cases", [])) == {"recoll-negated-supported", "recoll-negation-mutation"}, "negation cases not separated")

    user_ids = {ident for ident, audience, _, _ in slots if audience == "user"}
    reviews = report.get("reviews")
    require(isinstance(reviews, list) and {item.get("id") for item in reviews} == user_ids and len(reviews) == 13, "user review coverage mismatch")
    example_states, journey_states = fixture_ids(root)
    verdict_counts = {key: 0 for key in VERDICTS}
    contradicted, accepted = [], 0
    for review in reviews:
        require(isinstance(review.get("claim"), str) and review["claim"].strip() and isinstance(review.get("anchor"), str), "incomplete triage identity")
        require(HEX64.fullmatch(review.get("evidence_sha256", "")), "invalid evidence hash")
        runs = review.get("runs")
        require(isinstance(runs, list) and len(runs) >= 2, "fragment repeat missing")
        for run in runs:
            validate_run(run)
            verdict_counts[run["verdict"]] += 1
        has_contradiction = any(run["verdict"] == "contradicted" for run in runs)
        if has_contradiction:
            contradicted.append(review["id"])
            disposition = review.get("disposition", {})
            require(disposition.get("id") == review["id"] and disposition.get("disposition", "").strip() and disposition.get("note", "").strip(), "contradiction lacks human disposition")
        actions = review.get("actionability_runs")
        require(isinstance(actions, list) and len(actions) >= 2, "actionability repeat missing")
        for action in actions:
            require(HEX64.fullmatch(action.get("prompt_sha256", "")) and isinstance(action.get("raw_attempt"), str), "invalid actionability evidence")
            fixture = action.get("fixture", {})
            if fixture.get("accepted"):
                accepted += 1
                if action.get("surface") == "gui":
                    require(journey_states.get(fixture.get("id")) == "executed" and fixture.get("state") == "executed", "accepted action is outside closed fixture catalog")
                else:
                    state, fixture_surface = example_states.get(fixture.get("id"), (None, None))
                    allowed = {"command": ("cli",), "configuration": ("config",), "api": ("rest", "mcp")}.get(action.get("surface"), ())
                    require(state == "executed" and fixture.get("state") == "executed" and fixture_surface in allowed, "accepted action is outside closed fixture catalog")
            else:
                require(not fixture.get("id") and fixture.get("reason", "").strip(), "rejected action carries an identity or no reason")
    summary = report.get("summary", {})
    require(summary.get("fragments") == 13 and summary.get("verdicts") == {key: value for key, value in verdict_counts.items() if value}, "advisory summary count drift")
    require((summary.get("contradicted") or []) == contradicted and not summary.get("missing_dispositions") and summary.get("accepted_fixture_attempts") == accepted, "advisory triage summary drift")
    return report


def main(root=ROOT, here=HERE):
    template, slots = validate_templates(root)
    validate_generated(template, slots, root)
    deterministic = validate_report(root, here)
    advisory = validate_advisory(slots, root, here)
    # Every number here is read back from what was just validated. Three of
    # them used to be literals, and the line printed "413 generated rows" while
    # validating 419 -- a success message that can disagree with its own
    # evidence is worse than no message.
    summary = deterministic["summary"]
    print(
        f"G18f evidence valid: {summary['fragments']} fragments, "
        f"{summary['enumerated_rows']} generated rows, "
        f"{advisory['calibration']['correct']}/{advisory['calibration']['total']} calibration decisions, "
        f"{advisory['summary']['fragments']} advisory reviews"
    )


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, OSError, KeyError, TypeError, json.JSONDecodeError) as error:
        print(f"G18f evidence invalid: {error}", file=sys.stderr)
        sys.exit(1)
