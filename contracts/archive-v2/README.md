# Notrios archive-v2 consumer contract

This directory is the published, machine-readable portable semantic archive
contract. Start with `contract.json`, then validate individual JSON/JSONL
values with the strict Draft 2020-12 schemas under `schemas/`. JSON Schema is a
structural check only; consumers must also enforce bounded reads, duplicate-key
refusal, ordering, capabilities, hashes, paths, counts, references, and complete
filesystem verification.

`fixtures/loose-schema12`, `fixtures/packed-schema12`, and
`fixtures/sync-era-schema27-packed` are complete, sanitized, independently
generated archives. The other fixtures are narrow declaration/index refusal
or compatibility probes described by `fixtures/fixture-matrix.json`.
`sync-wire/` is a byte-for-byte publication of G9's separate deterministic
NCB1/NEV1 vectors and invented inputs; it is not an archive-v2 capability.

Run `notriosctl compatibility archive-v2 <archive-dir|manifest.json>` before
opening objects, then perform full verification before consuming content.
`notrios-sqlite-image` is a separate physical format and must be refused by a
portable archive-v2 consumer with its declared `notrios-archive-v2` fallback.
