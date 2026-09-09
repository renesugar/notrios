# G19 archive-v2 compatibility evidence

This directory records the producer-side external compatibility evidence for
G19. The executable contract is under `contracts/archive-v2/`; focused Go tests
independently reproduce and fully verify the three positive goldens, compile
all Draft 2020-12 schemas, validate every published structural sample, compare
the registry with production constants, and exercise the current/previous
reader matrix plus CLI exit/JSON behavior.

`external-consumer-audit.json` records the bounded local MoveNotes inspection.
No `notrios2sql.py` or Notrios archive consumer exists in either checked
repository or locally reachable history, so no external repository was edited
and no cross-repository consumer result is claimed.

Run the durable gate with:

```sh
make g19-validate
```

All fixtures are invented and sanitized. The physical refusal directory has a
production-shaped manifest with fake hashes but intentionally contains no
SQLite image or pack. G9 sync-wire vectors are byte-copied for publication but
remain explicitly separate from archive-v2.
