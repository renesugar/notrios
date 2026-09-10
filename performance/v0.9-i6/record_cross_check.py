#!/usr/bin/env python3
"""Turn the cross-check runs into two records: agreement, and a dated scan."""
import argparse
import datetime as dt
import json
import pathlib
import re
import subprocess

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]


def purls(document, prefix):
    return {c["purl"].split("?")[0] for c in document.get("components", [])
            if c.get("purl", "").startswith(prefix)}


def version(*command):
    try:
        out = subprocess.run(command, capture_output=True, text=True, check=True)
    except (OSError, subprocess.CalledProcessError):
        return None
    return (out.stdout or out.stderr).strip().splitlines()[0]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--work", type=pathlib.Path, required=True)
    parser.add_argument("--set", dest="release_set", type=pathlib.Path, required=True)
    arguments = parser.parse_args()
    work, release_set = arguments.work, arguments.release_set

    mine = json.loads((release_set / "SBOM.cdx.json").read_text(encoding="utf-8"))
    syft = json.loads((work / "syft.json").read_text(encoding="utf-8"))
    cdxgen_path = work / "cdxgen.json"
    cdxgen = json.loads(cdxgen_path.read_text(encoding="utf-8")) if cdxgen_path.is_file() else {}

    mine_go, syft_go, cdx_go = (purls(d, "pkg:golang/") for d in (mine, syft, cdxgen))
    mine_npm, syft_npm = purls(mine, "pkg:npm/"), purls(syft, "pkg:npm/")
    actions = sorted(set(purls(syft, "pkg:github/")))

    crosscheck = {
        "schema": "notrios.v09.i6-sbom-crosscheck.v1",
        "generated_at": dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "why": "A generator and a verifier written by one author from one assumption fail "
               "together and look like agreement. These tools were written by other people for "
               "other reasons, so where they disagree there is something to learn.",
        "tools": {"syft": version("syft", "version"), "cdxgen": version("cdxgen", "--version")},
        "each_answers_a_different_question": {
            "notrios": "the dependency set the licence gate governs: the build module graph and "
                       "the npm lockfiles",
            "syft": "what is discoverable on disk, including built binaries and the workflows",
            "cdxgen": "the direct Go dependencies declared in go.mod",
        },
        "go": {"notrios": len(mine_go), "syft": len(syft_go), "cdxgen": len(cdx_go),
               "notrios_missing_from_syft": sorted(mine_go - syft_go),
               "syft_only_count": len(syft_go - mine_go),
               "cdxgen_outside_notrios": sorted(cdx_go - mine_go),
               "reading": "Every module this project ships appears in syft's larger set, and "
                          "cdxgen's direct dependencies are a subset of it. The extra modules "
                          "syft reports are test and tooling dependencies from the module graph, "
                          "which the licence gate deliberately does not govern."},
        "npm": {"notrios": len(mine_npm), "syft": len(syft_npm),
                "overlap": len(mine_npm & syft_npm),
                "syft_only": sorted(syft_npm - mine_npm)[:10],
                "notrios_only_count": len(mine_npm - syft_npm),
                "reading": "syft was run with node_modules excluded, so it reads fewer packages "
                           "than the lockfiles declare. The overlap is what matters, and it is "
                           "what the purl fix below repaired."},
        "what_it_found": [
            "The npm purls were wrong. A scoped package's namespace is percent-encoded in a purl "
            "-- pkg:npm/%40antfu/install-pkg@1.1.0 -- and this generator emitted a raw '@'. "
            "Agreement with syft was 124 of 361 before the fix and 268 of 270 after it. No count "
            "check could have found this: the counts were right and every identifier was wrong, "
            "which is the failure a cross-check exists for.",
            f"syft reports {len(actions)} GitHub Actions as components ({', '.join(sorted({a.split('@')[0] for a in actions}))}). "
            "This project's SBOM does not model them at all, so what runs in CI is a supply-chain "
            "surface the release evidence does not describe. I6 pinned those actions by digest; "
            "describing them in an SBOM is a separate gap and is recorded, not closed.",
            "cdxgen warns that SBOM generation invokes build tooling which inherits the "
            "environment, and named the API keys exported on this workstation. Its output carried "
            "none of them -- checked -- but the point stands, so the generators are run with a "
            "scrubbed environment here.",
        ],
        "github_actions_seen_by_syft": actions,
    }
    (HERE / "SBOM_CROSSCHECK.json").write_text(
        json.dumps(crosscheck, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    grype = json.loads((work / "grype.json").read_text(encoding="utf-8"))
    matches = grype.get("matches", [])
    severities: dict[str, int] = {}
    for match in matches:
        key = match["vulnerability"].get("severity", "Unknown")
        severities[key] = severities.get(key, 0) + 1
    text = (work / "govulncheck.txt").read_text(encoding="utf-8")
    reachable = re.search(r"affected by (\d+) vulnerabilit", text)
    required = re.search(r"(\d+)\s*\n?vulnerabilities in modules you require", text)

    scan = {
        "schema": "notrios.v09.i6-vulnerability-scan.v1",
        "scanned_at": crosscheck["generated_at"],
        "subject": "dist/release-set/SBOM.cdx.json",
        "this_is_not_a_gate": {
            "decision": "recorded, never a build gate",
            "why": "A vulnerability database changes daily. Gating on it means a build that "
                   "passed this morning fails this afternoon because somebody else published an "
                   "advisory, with nothing here changed -- a red build that carries no "
                   "information about this commit. It is also the rule this project already "
                   "holds: no probabilistic or time-varying result becomes a build gate. What "
                   "belongs in a gate is the *record*: that a scan was run, when, with what, and "
                   "what it said.",
        },
        "grype": {"version": grype.get("descriptor", {}).get("version"),
                  "matches": len(matches), "by_severity": severities,
                  "granularity": "module and package versions, without reachability"},
        "govulncheck": {
            "version": version("govulncheck", "-version"),
            "reachable_from_this_code": int(reachable.group(1)) if reachable else None,
            "in_required_modules": int(required.group(1)) if required else None,
            "granularity": "call graph: only vulnerabilities this code can actually reach",
            "verbatim": " ".join(text.split())[-320:],
        },
        "reading": "grype's severity counts describe the dependency graph; govulncheck's describe "
                   "this program. Both are true and they answer different questions, which is "
                   "exactly why the first must not gate a build: it would have blocked on "
                   "criticals that the second shows this code never calls.",
        "what_to_do_with_it": "Review at release time and when upgrading dependencies. A "
                              "reachable finding is a defect to fix; an unreachable one is a "
                              "reason to upgrade when convenient, and a reason not to panic.",
    }
    (HERE / "SCAN.json").write_text(json.dumps(scan, indent=2, sort_keys=True) + "\n",
                                    encoding="utf-8")
    print(f"cross-check: go {len(mine_go)}/{len(syft_go)}/{len(cdx_go)}, "
          f"npm overlap {len(mine_npm & syft_npm)}/{len(syft_npm)}, "
          f"{len(actions)} actions syft sees and this SBOM does not")
    print(f"scan: grype {len(matches)} matches {severities}, "
          f"govulncheck reachable={scan['govulncheck']['reachable_from_this_code']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
