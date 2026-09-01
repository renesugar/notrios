#!/usr/bin/env python3
"""Validate the v0.8 H2a Mermaid investigation evidence.

Offline and self-contained: it re-reads the recorded report and the raw probe
results and checks that the conclusions still follow from the measurements.
It does not re-run a browser; the point is that the archived numbers and the
recommendation cannot drift apart.
"""

from __future__ import annotations

import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


class EvidenceError(ValueError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise EvidenceError(message)


def load(name: str):
    with open(os.path.join(HERE, name), encoding="utf-8") as stream:
        return json.load(stream)


def validate() -> dict:
    report = load("REPORT.json")
    probe = load("probe-results.json")
    supplementary = load("probe2-results.json")

    require(report["schema"] == "notrios.v0.8.h2a.mermaid-investigation.v1",
            "unexpected report schema")
    require(report["baseline"]["mermaid_enabled"] is False,
            "the investigation must start from the disabled baseline")

    # The CSP claim is the load-bearing one: it is what makes enablement
    # possible without widening the policy.
    require(report["csp"]["violations"] == 0, "recorded CSP violations are not zero")
    require(len(probe["csp_violations"]) == report["csp"]["violations"],
            "report and raw probe disagree on CSP violations")
    require(report["csp"]["unsafe_eval_required"] is False,
            "unsafe-eval must not be required")
    require(report["csp"]["new_function_surviving_vite_bundle"] == 0,
            "dynamic code construction must not survive the bundle")

    # The containment recommendation rests on html labels being off.
    contained = report["containment"]["with_html_labels_disabled"]
    require(contained["cross_origin_requests"] == 0,
            "the recommended configuration must make zero cross-origin requests")
    require(contained["foreign_objects_emitted"] is False,
            "the recommended configuration must emit no foreignObject")
    require(contained["script_execution"] is False, "script execution was detected")
    require(probe["script_execution_detected"] is False,
            "raw probe recorded script execution")

    # The default configuration must still be recorded as unacceptable, so the
    # reason for the recommendation is not lost.
    default = report["containment"]["with_html_labels_default"]
    require(default["cross_origin_requests"] > 0,
            "the default configuration's remote fetch finding was lost")

    # A residual vector that is known must stay named.
    require("href" in contained["residual_vector"],
            "the surviving href vector must remain recorded")
    require(any("href" in step for step in report["recommendation"]["required_post_processing"]),
            "the recommendation must require href neutralisation")

    # The link-navigation finding is the reason the recommendation changed, and
    # it is counter-intuitive enough that losing it would invite the original
    # mistake again: stock strict mode keeps remote links and drops the
    # product's own note links.
    links = report["link_navigation"]
    require(links["measured_strict"]["notrios_scheme_href"] is None,
            "the finding that strict drops notrios:// links must stay recorded")
    require(links["measured_strict"]["https_href"],
            "the finding that strict keeps remote links must stay recorded")
    require(links["measured_loose"]["javascript_href"],
            "the finding that loose re-admits javascript: URLs must stay recorded")
    require(links["root_cause"]["component"] == "dompurify",
            "the root cause must remain attributed to the DOM sanitiser")
    require(links["root_cause"]["mermaid_own_sanitizer_is_not_the_cause"] is True,
            "the sanitize-url exoneration must stay recorded")
    require(links["html_anchor_label_method"]["anchors_emitted_with_html_labels_disabled"] == 0,
            "the html anchor label finding must stay recorded")
    require(any("notrios" in step for step in report["recommendation"]["required_post_processing"]),
            "the recommendation must allowlist the notrios scheme")
    require(any("remote" in step for step in report["recommendation"]["required_post_processing"]),
            "the recommendation must neutralise remote hrefs")
    require(report["recommendation"].get("superseded_note"),
            "the superseded strip-every-href draft must stay recorded")

    # The diagram link policy sends a reader to the note to reach a remote URL.
    # That is only coherent if a note link opens, which on the desktop shell it
    # currently does not, so the dependency must stay visible next to the
    # policy that relies on it.
    dependency = links["dependency_on_note_link_behaviour"]
    require(dependency["no_browseropenurl_call_in_repository"] is True,
            "the missing desktop link handling must stay recorded")
    require("desktop" in dependency["tracked_as"].lower() or dependency["tracked_as"],
            "the dependency must name where it is tracked")
    require("not confirmed" in dependency["confidence"],
            "the confidence caveat on the desktop finding must stay recorded")

    # Adversarial fixtures must be present and must have been exercised.
    fixture_dir = os.path.join(HERE, "fixtures")
    on_disk = {name[:-4] for name in os.listdir(fixture_dir) if name.endswith(".mmd")}
    for required in ("malformed_syntax", "html_script_handler",
                     "javascript_and_remote_url", "node_limit_plus_one",
                     "click_schemes", "html_anchor_in_label"):
        require(required in on_disk, f"missing fixture {required}")
        if required in ("click_schemes", "html_anchor_in_label"):
            require(required in links["raw_results"]["strict"],
                    f"fixture {required} was not exercised under strict")
            continue
        require(required in report["fixtures"], f"fixture {required} was not exercised")

    require(report["fixtures"]["malformed_syntax"]["rendered"] is False,
            "malformed source must not render")
    require(report["fixtures"]["node_limit_plus_one"]["rendered"] is False,
            "an over-limit graph must not render")

    # Licence blockers are the other half of the recommendation; losing them
    # would make enablement look cheaper than it is.
    require(len(report["license_gate_blockers"]) == 3,
            "expected three recorded licence-gate blockers")
    for blocker in report["license_gate_blockers"]:
        require(blocker.get("fix"), f"{blocker['package']} has no recorded fix")

    # Honesty check: the untested list must survive, including the deadline gap.
    require(any("deadline" in item for item in report["untested"]),
            "the untested render-deadline gap must stay recorded")
    require(any("Wails" in item for item in report["untested"]),
            "the untested Wails smoke must stay recorded")
    require(any("end to end" in item for item in report["untested"]),
            "the untested end-to-end notrios:// click must stay recorded")

    require(supplementary["narrow_390"]["overflowsViewport"] is False,
            "narrow-layout rendering overflowed")
    for theme in ("theme_default", "theme_dark"):
        require(supplementary[theme]["ok"] is True, f"{theme} did not render")

    return report


def main() -> int:
    try:
        report = validate()
    except (EvidenceError, KeyError, OSError) as error:
        print(f"h2a evidence: {error}", file=sys.stderr)
        return 1
    print(
        "h2a evidence: mermaid {version} validated; {violations} CSP violations, "
        "zero cross-origin requests under the recommended containment, "
        "{blockers} licence-gate blockers, {fixtures} fixtures".format(
            version=report["candidate"]["version"],
            violations=report["csp"]["violations"],
            blockers=len(report["license_gate_blockers"]),
            fixtures=len(report["fixtures"]),
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
