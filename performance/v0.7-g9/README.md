# v0.7 G9 — deterministic envelope codec, encryption, and signatures

G9 is production code. This directory holds the format specification, the
independent golden generator that keeps it honest, and the measurements of what
the canonical encoding and the crypto cost.

`FORMAT.md` is the specification. `generate_goldens.py` is a **second
implementation of it**, written from that document rather than from the Go
package, and `internal/syncwire`'s golden test compares the two byte for byte. A
round trip against the encoder that produced the bytes would pass even if the
format were wrong; this does not.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g9-gocache go test ./internal/syncwire/...

env GOCACHE=/tmp/notrios-g9-gocache \
  go test ./internal/syncwire -run '^$' -parallel=1 \
  -fuzz '^FuzzOpenNeverPanics$' -fuzztime=15s

PYTHONDONTWRITEBYTECODE=1 python3 performance/v0.7-g9/generate_goldens.py

env GOCACHE=/tmp/notrios-g9-gocache \
  go run ./performance/v0.7-g9/cmd/evidence -out performance/v0.7-g9/wire-results.json

PYTHONDONTWRITEBYTECODE=1 python3 performance/v0.7-g9/validate_evidence.py
```

## Evidence

- `FORMAT.md` — the specification;
- `golden-inputs.json` / `goldens.json` — shared inputs and the independently
  generated canonical bytes;
- `wire-results.json` — size and time against the JSON the journal stores today;
- `FINDINGS.md` — what the numbers say and what they do not cover.

## License inventory

G9 adds **no dependency**. AES-256-GCM, HKDF-SHA256, Ed25519, and HMAC-SHA256
all come from the Go standard library (BSD-3-Clause). G0's rule was to use a
maintained implementation rather than invent one; the standard library is the
most maintained option available and needs no version pin, no mobile-build
check, and no supply-chain review.

## Scope

Desktop measurements on one host. No carrier, no network, and no operating-system
secret store: the key and signer interfaces ship with in-memory test providers,
and binding them to a real secret store is v0.8's, as the plan item says.
