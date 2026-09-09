# H0 disposable C ABI probe

This directory is an investigation-only, production-independent probe for the
twelve symbols frozen in `performance/v0.7-g18/ABI_CONTRACT.json`. It does not
import Notrios packages or alter the module. `run.sh` builds both `c-shared`
and `c-archive`, copies the generated cgo header, checks all twelve symbols,
and compiles/runs `host.c`.

The probe exercises borrowed-input copying, generation-bearing opaque handles,
two isolated instances, bounded requests and reads, asynchronous polling and
cancellation, event polling, partial stream reads and EOF, exact-once
library-buffer release, wrong/stale/zero handles, idempotent close, and
shutdown refusal. Outputs are allocated with C malloc and tracked by owner
instance, so double release and cross-instance release return typed errors.

The ABI is intentionally a mechanical lifecycle harness, not a proposed
application facade: request bytes are echoed and the event is synthetic. It
does not claim semantic REST parity, persistence, authentication, secret
handling, or production thread-safety policy. The host uses polling and has no
callbacks from Go threads. The generated header is refreshed by `run.sh` and
is checked in as a reproducibility aid; build outputs remain in a temporary
directory.

Run from the repository root:

```sh
GOCACHE=/tmp/notrios-h0-gocache performance/v0.8-h0/abi-probe/run.sh
```
