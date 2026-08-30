# G18f evidence validator

`validate_evidence.py` is a strict, deterministic, model-free validator for
the committed generated-document and advisory evidence. Run it after the
deterministic report and advisory report have been supplied:

```sh
python3 performance/v0.7-g18f/validate_evidence.py
```

The deterministic freshness gate is run with:

```sh
go run ./cmd/docgen --user --api --check
```

Semantic `doccheck` is a maintainer-only command, run explicitly with the
local model and recorded inputs/outputs:

```sh
go run ./cmd/doccheck \
  --endpoint http://127.0.0.1:8081 \
  --triage performance/v0.7-g18f/TRIAGE.json \
  --output performance/v0.7-g18f/ADVISORY_REPORT.json \
  --repeats 2 --temperature 0.1
```

No source is uploaded to a hosted service; no note or database content is
used. The source-only explanation is generated before the claim is revealed.
Because the 1.5B model could not reliably make an overloaded three-way choice,
classification is decomposed into direct-conflict and whole-claim-support
questions. Their finite yes/no answers deterministically produce supported,
contradicted, or not-determinable and remain visible in every run.

The recorded Qwen 2.5 Coder 1.5B run classified 7 of 16 repeated calibration
decisions correctly and showed substantial variance. It is therefore useful
only as a disagreement generator. Eleven of 13 fragments received at least one
contradicted verdict; every one has a human disposition tied to deterministic
generation or a registered claim test. No model result rewrote prose or blocks
a release.

Actionability output is never executed directly. It must exactly match a
closed G18d example body or G18e journey identity; the recorded 26 attempts
matched none and were rejected. G18d and G18e validators are run separately
before any matched fixture result is accepted. Prompt/source hashes, model,
tokens, timing, variance, and zero local API cost are recorded without storing
raw prompts or duplicating source slices.
