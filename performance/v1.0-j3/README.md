# v1.0 J3 — the safeguards, reachable from a packaged installation

```sh
bash performance/v1.0-j3/command_drills.sh     # needs `make build`
python3 performance/v1.0-j3/validate_evidence.py
```

`make purge` takes a verified backup before it deletes anything, refuses when
nothing can answer, never follows a symlink out of a profile, and keeps sync key
material out of the backup. All of it lived in `scripts/lifecycle.py`, and the
`.deb` ships no Python — so the user most in need of those safeguards was the
one who could not reach them. `notriosctl purge` closes that.

## The oracle exists twice, and the two are gated against each other

H3's oracle decides *may Notrios delete this path*, and was written before any
code could act on it so the rules could be argued about first. Porting it is a
real risk: two implementations of a deletion rule is the worst possible
duplication. So `internal/purge/oracle_test.go` drives **H3's own thirty
fixtures**, by name, reading the expected verdict and rule out of
`PURGE_ORACLE_FIXTURES.json` rather than restating them — and fails if a fixture
case exists that the port does not exercise.

Proved by breaking it: following a symlink instead of refusing, reclassifying
`cache` as irreplaceable, and dropping `/usr` from the forbidden roots are each
caught and named.

## One safeguard this adds

`--backup-dir` lets a user name a destination, and the obvious wrong answer is
somewhere inside the library they are about to delete. Every run now asserts
that the destination is *refused* by the oracle — H3 proved that property for
the default location, and this checks it rather than trusting the layout.

## Still owed

`scripts/lifecycle.py` keeps its own purge, so the **backup and deletion halves
exist twice** while only the oracle halves are gated. A divergence there is a
purge that deletes without the backup somebody was promised.

The fix is to have `make purge` delegate its data half to `notriosctl purge`,
and the ordering is the interesting part: lifecycle.py backs up, uninstalls,
then deletes — and delegation means deleting the data while the binary that does
it still exists, so the uninstall has to move after rather than before.
