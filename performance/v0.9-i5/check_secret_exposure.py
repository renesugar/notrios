#!/usr/bin/env python3
"""Refuse a workflow that could hand signing material to a pull request.

I5's boundary is that certificates, tokens and passphrases stay out of
pull-request jobs, logs, artifacts, backups and the repository. The first four
of those are a property of the workflow files, so they are checked here rather
than promised in a document. No workflow uses a secret today; this exists so
that the first one to do so has to do it deliberately.

Three rules, each with a reason:

  * **A workflow with a `pull_request` trigger may not use a secret.** On this
    repository a pull request comes from a branch of the repository itself, and
    same-repository pull requests *do* receive secrets. So a repository-level
    secret plus a `pull_request` trigger means every branch anyone pushes can
    read the signing key.
  * **`pull_request_target` is refused outright.** It runs with the base
    repository's secrets against the *pull request's* code, which is the
    standard way credentials leave a repository.
  * **A workflow that uses a secret must name an `environment`.** An environment
    can require a reviewer, and that is what makes the use of a signing key a
    deliberate act rather than a consequence of a push.

`GITHUB_TOKEN` is exempt: it is issued per run, scoped by `permissions:`, and is
not key material anybody has to protect.

It reads the files as text rather than parsing YAML, so `make validate` gains no
dependency. That makes the check per-file rather than per-job, which
over-approximates: a workflow whose *push* job uses a secret is refused if the
same file also builds on pull requests. That direction is deliberate. A security
gate that is too coarse produces an argument; one that is too clever produces a
leak.
"""
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github" / "workflows"
SECRET = re.compile(r"secrets\.([A-Za-z_][A-Za-z0-9_]*)")
EXEMPT = {"GITHUB_TOKEN"}


class ExposureError(AssertionError):
    pass


def triggers(text: str) -> set[str]:
    """The events a workflow runs on, read from its `on:` block."""
    found, inside = set(), False
    for line in text.splitlines():
        if re.match(r"^on:\s*$", line):
            inside = True
            continue
        if re.match(r"^on:\s*\S", line):  # inline form: on: [push, pull_request]
            found.update(re.findall(r"[a-z_]+", line.split(":", 1)[1]))
            continue
        if inside:
            if line and not line[0].isspace():
                inside = False
                continue
            match = re.match(r"^\s{1,4}([a-z_]+):", line)
            if match:
                found.add(match.group(1))
    return found


def main() -> int:
    problems: list[str] = []
    if not WORKFLOWS.is_dir():
        print("no workflows to check")
        return 0
    checked = 0
    for path in sorted(WORKFLOWS.glob("*.y*ml")):
        checked += 1
        text = path.read_text(encoding="utf-8")
        name = path.relative_to(ROOT).as_posix()
        events = triggers(text)
        used = {n for n in SECRET.findall(text) if n not in EXEMPT}

        if "pull_request_target" in events:
            problems.append(f"{name}: pull_request_target runs the pull request's code with this "
                            f"repository's secrets; use pull_request and do not give it secrets")
        if used and "pull_request" in events:
            problems.append(f"{name}: uses {', '.join(sorted(used))} and runs on pull_request; "
                            f"a same-repository pull request receives secrets, so every branch "
                            f"pushed here could read them")
        if used and "environment:" not in text:
            problems.append(f"{name}: uses {', '.join(sorted(used))} without an environment; "
                            f"an environment is what lets a reviewer gate the use of a key")

    if problems:
        for problem in problems:
            print(f"secret exposure: {problem}", file=sys.stderr)
        return 1
    print(f"secret exposure check passed: {checked} workflows, "
          f"no signing material reachable from a pull request")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
