#!/usr/bin/env python3
"""Can a reader act on this page? Ask a model, then run what it says.

The question every other documentation gate in this repository leaves alone is
whether a person could *do* anything with a page. This asks it directly: give a
model the prose for one task, take the command it produces, run it against a
disposable library, and look at what changed.

Three arms, and the second one is why this is a measurement rather than a demo.

    prose        the page's own words for the task
    no-prose     the task name alone
    misleading   the prose with one detail mutated

A page earns credit only when *prose* succeeds and *no-prose* fails. Without
that second arm a capable model produces a plausible `notriosctl` invocation
from familiarity with command-line conventions alone, and the score measures the
model while claiming to measure the documentation. The third arm catches a model
ignoring the text it was handed.

Exit status is not the oracle. A command can exit zero having done nothing, and
it can exit zero having done the opposite of what the page described, so each
task carries the postcondition the journey catalogue already defines and the
sandbox is inspected afterwards.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shlex
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
JOURNEYS = ROOT / "docs/docjourneys/CLI_JOURNEYS.json"

# A model's output is untrusted text. Only an argument vector that begins with
# notriosctl and contains nothing a shell would interpret is ever run, and it is
# run without a shell, with the sandbox's own binary and roots substituted in.
# Anything else is recorded as refused rather than executed.
SAFE_ARGUMENT = re.compile(r"^[A-Za-z0-9_.,:/@=+{}\[\]-]+$")


def extract_command(text: str) -> list[str] | None:
    """Take the first line that looks like a notriosctl invocation."""
    for line in text.splitlines():
        line = line.strip().strip("`").strip()
        if line.startswith("$ "):
            line = line[2:].strip()
        if not line.startswith("notriosctl "):
            continue
        try:
            argv = shlex.split(line)
        except ValueError:
            return None
        return argv[1:]
    return None


def refuse(argv: list[str]) -> str | None:
    """Why this argument vector must not be run, or None if it may be."""
    for argument in argv:
        if not SAFE_ARGUMENT.match(argument):
            return f"argument {argument!r} contains characters this harness will not execute"
    return None


def prompt_for(journey: dict, arm: str) -> str:
    goal = f"{journey['title']}: {journey['goal']}"
    if arm == "no-prose":
        return (
            "You are using a note application called Notrios, whose command-line tool is "
            "`notriosctl`.\n\n"
            f"Task: {goal}\n\n"
            "Reply with exactly one line: the single notriosctl command that does this. "
            "No explanation, no code fence."
        )
    steps = []
    for index, step in enumerate(journey["steps"], start=1):
        narrative = step["narrative"]
        if arm == "misleading" and index == 1:
            # One detail changed, not the whole passage: the arm is meant to
            # catch a model ignoring the text, not to make the text unusable.
            narrative = narrative.replace("notriosctl", "notriosctl --legacy")
            narrative += " (Note: this command was renamed and now requires --legacy.)"
        steps.append(f"{index}. {narrative}")
    body = "\n".join(steps)
    return (
        "You are using a note application called Notrios, whose command-line tool is "
        "`notriosctl`. Here is the documentation for one task.\n\n"
        f"Task: {goal}\n\nDocumentation:\n{body}\n\n"
        "Reply with exactly one line: the single notriosctl command that performs step 1. "
        "No explanation, no code fence."
    )


def ask(model: str, prompt: str, timeout: int) -> tuple[str, str | None]:
    """Return the model's text, and an error string when the call did not work.

    opencode exits zero on an upstream rate limit, so the exit status cannot be
    trusted and the text is inspected instead.
    """
    with tempfile.TemporaryDirectory() as scratch:
        try:
            result = subprocess.run(
                # --pure drops external plugins so the zg integration cannot
                # feed the model anything; --auto is required rather than
                # desirable. opencode's permission config asks before touching
                # an external directory, every sandbox is external, and a
                # non-interactive run cannot answer the prompt -- so without it
                # the call hangs until the timeout rather than failing. That is
                # recorded as a finding about the instrument: an agentic CLI is
                # an awkward shape for a single-completion measurement, and the
                # empty sandbox is what keeps it from reading anything.
                ["opencode", "run", "--pure", "--auto", "--model", model, prompt],
                capture_output=True, text=True, timeout=timeout, cwd=scratch,
            )
        except subprocess.TimeoutExpired:
            return "", "timeout"
    text = re.sub(r"\x1b\[[0-9;]*m", "", result.stdout + result.stderr)
    lowered = text.lower()
    for marker in ("rate-limited", "rate limited", "429", "quota", "error:"):
        if marker in lowered:
            return text, "unavailable"
    return text, None


def run_task(binary: str, journey: dict, argv: list[str]) -> dict:
    """Run one candidate command in its own library and check the postcondition."""
    with tempfile.TemporaryDirectory() as sandbox:
        values = {
            "db": os.path.join(sandbox, "notes.sqlite"),
            "assets": os.path.join(sandbox, "assets"),
            "out": os.path.join(sandbox, "out"),
            "out2": os.path.join(sandbox, "out2"),
            "keys": os.path.join(sandbox, "sync-keys.json"),
            "config": os.path.join(sandbox, "notrios.yaml"),
            "docs": str(ROOT / "docs"),
        }
        Path(values["config"]).write_text("sync:\n  rest:\n    credential_store: development-file\n")

        def expand(vector):
            out = []
            for argument in vector:
                for name, value in values.items():
                    argument = argument.replace("{" + name + "}", value)
                out.append(argument)
            return out

        # The model does not choose which library it touches: whatever paths it
        # names are replaced by the sandbox's own.
        cleaned, skip = [], False
        for argument in argv:
            if skip:
                skip = False
                continue
            if argument in ("--db", "--asset-store", "--keys", "--config"):
                skip = True
                continue
            cleaned.append(argument)
        vector = [binary, *cleaned, "--db", values["db"], "--asset-store", values["assets"]]
        try:
            attempt = subprocess.run(vector, capture_output=True, text=True, timeout=120)
        except subprocess.TimeoutExpired:
            return {"ran": False, "reason": "the generated command did not finish"}
        except OSError as error:
            return {"ran": False, "reason": str(error)}

        post = journey["postcondition"]
        if post.get("file_exists"):
            path = expand([post["file_exists"]])[0]
            return {"ran": True, "exit": attempt.returncode, "held": os.path.exists(path)}
        check = subprocess.run([binary, *expand(post["command"])],
                               capture_output=True, text=True, timeout=120)
        combined = check.stdout + check.stderr
        held = all(want in combined for want in post.get("contains", [])) and \
            not any(bad in combined for bad in post.get("absent", []))
        return {"ran": True, "exit": attempt.returncode, "held": held}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True, help="the notriosctl under test")
    parser.add_argument("--model", action="append", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--timeout", type=int, default=240)
    parser.add_argument("--journey", action="append", default=[])
    options = parser.parse_args()

    for model in options.model:
        if "free" not in model:
            print(f"refusing {model}: only slugs containing 'free' are used", file=sys.stderr)
            return 2

    catalogue = json.loads(JOURNEYS.read_text())
    journeys = [j for j in catalogue["journeys"]
                if not options.journey or j["id"] in options.journey]

    results = []
    for model in options.model:
        for journey in journeys:
            for arm in ("prose", "no-prose", "misleading"):
                prompt = prompt_for(journey, arm)
                started = time.time()
                text, failure = ask(model, prompt, options.timeout)
                record = {
                    "model": model,
                    "journey": journey["id"],
                    "arm": arm,
                    "prompt_sha256": hashlib.sha256(prompt.encode()).hexdigest(),
                    "seconds": round(time.time() - started, 1),
                }
                if failure:
                    record["outcome"] = failure
                    results.append(record)
                    print(json.dumps(record), flush=True)
                    continue
                argv = extract_command(text)
                if argv is None:
                    record["outcome"] = "no-command"
                elif (why := refuse(argv)) is not None:
                    record["outcome"] = "refused"
                    record["detail"] = why
                else:
                    record["command"] = " ".join(argv)
                    outcome = run_task(options.binary, journey, argv)
                    if not outcome["ran"]:
                        record["outcome"] = "did-not-run"
                        record["detail"] = outcome["reason"]
                    else:
                        record["exit"] = outcome["exit"]
                        record["outcome"] = "acted" if outcome["held"] else "no-change"
                results.append(record)
                print(json.dumps(record), flush=True)

    Path(options.out).write_text(json.dumps({
        "schema": "notrios.h14.actionability.v1",
        "runs": results,
    }, indent=2) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
