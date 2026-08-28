# G18d executed documentation evidence

G18d turns every detected non-GUI executable fence into a strict bidirectional
manifest entry. The checked report records 131 identities: 63 execute against
fresh scratch state and 68 remain visible with one of six finite reviewed
reasons. No provisional reason is accepted.

The execution harness builds the real CLI and daemon, opens real SQLite stores,
binds a loopback REST/MCP service, validates REST traffic against
`api/openapi.yaml`, parses configuration fragments, and checks semantic state
rather than status alone. It never uses a private corpus, user database, shared
test state, or external network service.

Run the complete evidence check with:

```bash
python3 performance/v0.7-g18d/validate_evidence.py
```

The runtime fields in `REPORT.json` are one checked local observation. The
validator reruns the entire manifest and compares every deterministic field;
runtime must be positive but is expected to vary by host.

`MUTATION_MATRIX.json` names the negative checks that independently break a
fence identity, a manifest contract, OpenAPI requests/responses, and semantic
implementation results. The Go tests are the executable authority for those
cases.
