# v1.0 J24: the usage guard checks the running agent's own quota

## The defect (found closing J23)

The J23 archive build was paused by `scripts/agent_usage_preflight.sh`.
- **The run was Claude's.** Claude's quota was above the 20% reserve: five-hour
  32% remaining, seven-day 92%.
- **The pause came from Codex's weekly bucket,** at 8% remaining. Codex is a
  separate product with its own quota.
- **The cause:** the preflight always passed `--agent all`, so the checker bound
  on the lowest bucket across every agent it could read.

The owner overrode the guard for the J23 and J7 archives, and each close commit
records why.

## Identifying the running agent

The first plan detected the agent from its environment variables. The owner
rejected that (2026-09-15): more than one coding agent can run at once, so the
run must be identified as its own. When J24 started, a Codex session
(`codex resume`) was running beside this Claude session. Neither of these can
tell the two apart:
- **A machine-wide scan.** The checker's existing `_claude_running` scans every
  process on the machine.
- **Inherited variables.** A shell started under one agent carries that agent's
  variables into whatever it launches.

**Process ancestry can.** `--agent self` walks from the checker's own process up
its parents and takes the **nearest** coding agent. Each process is classified
from its executable and `argv[0]` only, never from its other arguments:

| agent | what identifies it (read from the live processes on this machine) |
|---|---|
| Claude Code | `argv[0]` `claude`; the native build's executable is `.../claude/versions/<version>` |
| Codex | the vendored native `codex` executable, its `codex-*` helpers (`codex-code-mode-host`), or `node .../bin/codex` |

If no agent is among the parents, it falls back to an explicit
`NOTRIOS_AGENT_USAGE_AGENT` (`claude|codex|all`). This covers a detached run
whose launching shell has exited. If ancestry does find an agent, a differing
explicit value is ignored and reported.

With neither, it checks `all`, as before. Each result carries a `selection`
field saying which agent was checked and why. It holds no environment values
beyond that one variable.

`agent_usage_preflight.sh` now passes `--agent self`, so every caller gets the
selection with no change of its own:
- `package_release.sh`
- the `run_*_profile.sh` scripts
- the G14b/G14e harnesses, which read only the preflight's exit code

The `codex`, `claude` and `all` code paths are unchanged, and so is Claude's
status-line cache.

## J24-A and J24-B: the tests, failing first

`RunningAgentTests` (`scripts/test_check_agent_usage.py`) was written before the
change. Against the unchanged checker, **all 9 tests failed**: the selection
functions did not exist. They cover:

- **Classification.** Claude's native build and a plain `claude`, Codex's native
  binary, helper and node launcher. Not an agent: `bash -c claude`,
  `vim codex claude`, `node /srv/claude-notes/app.js`.
- **The nearest agent ancestor.** Claude under a shell under Codex resolves to
  Claude from above and to Codex from the shell between them. An orphan, a
  parent cycle and an empty table resolve to nothing.
- **Ancestry wins.** A Claude ancestor with `CODEX_THREAD_ID` and
  `NOTRIOS_AGENT_USAGE_AGENT=codex` set is Claude, and the ignored value is
  reported. A Codex ancestor with `CLAUDECODE=1` is Codex.
- **The fallbacks.** An explicit agent applies only without an agent ancestor.
  `CLAUDECODE=1` alone, or an invalid explicit value, checks `all`.
- **Only the running agent's quota gates.**
  - A Claude run at 91% is not paused by Codex at 8%, and is paused at 10% of
    its own.
  - A Codex run behaves the same way the other way round.
  - Each time, the other agent's probe is never called.
- **An unidentified run is still guarded by every agent,** and pauses on Codex
  at 8%.
- **The real process table.** `read_process` returns this process's true
  parent.

After the change, all of `scripts/` passes (66 tests). So do
`test_agent_usage_preflight.sh`, which now asserts `--agent self`,
`check_required_files.py` and `check_python_hygiene.py --sources`.

## J24-C: the callers get the selection unchanged, and the archive proves it

The J24 archive `notrios-v1.0-j24-b92f715.zip` was built from `b92f715` by
`package_release.sh`, unmodified. The usage guard stayed on: no override, the
first archive since J21 not to need one.

Its preflight recorded `"agent": "claude"`, with 90% remaining against the 20%
reserve, and `"selection": "process ancestry: pid 1616574 is claude"`. It passed
without consulting Codex, whose weekly bucket had paused J23's archive.
`check_release_zip.py` accepted it: 25,455,843 bytes, 2,604 entries.

The documentation for the guard names whose usage is checked and how to choose:
- `AGENTS.md`
- `ENVIRONMENT_SETUP.md`
- `skills/agent-usage-preflight/SKILL.md`
- `TESTING_POLICY.md`
- `CONTEXT_MAP.md`

## Live checks on this machine

These ran with the Codex session still running beside this one:

| case | checked | selection reported |
|---|---|---|
| from this Claude session | claude, 90% | `process ancestry: pid 1616574 is claude` |
| the same, with `CODEX_THREAD_ID` and `NOTRIOS_AGENT_USAGE_AGENT=codex` leaked in | claude | `...is claude; ignored NOTRIOS_AGENT_USAGE_AGENT=codex` |
| under a live process running as `codex`, with `CLAUDECODE=1` inherited | codex, 8% | `process ancestry: pid 2759566 is codex` |
| detached with `setsid`, launching shell exited | codex and claude | `no coding agent among this process's ancestors; checking all` |
| the same, with `NOTRIOS_AGENT_USAGE_AGENT=claude` | claude | `...; NOTRIOS_AGENT_USAGE_AGENT=claude` |
| **`agent_usage_preflight.sh package-release`, the call that paused J23** | **claude, 90%, exit 0** | `process ancestry: pid 1616574 is claude` |

**The simulated Codex parent.** The third row used a shell started as `codex`
(`exec -a codex`). A first attempt reported Claude. That was a flaw in the
simulation: `bash -c` replaces itself with its last command, so the `codex`-named
process became the checker before the walk reached it. The row shown kept that
process alive. No real Codex command was run, because that would spend Codex
quota.
