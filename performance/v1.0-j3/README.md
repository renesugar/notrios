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

## `make purge` runs this command rather than repeating it

The backup and deletion halves existed twice — Python for a checkout, Go for the
package — while only the oracle halves were gated against each other. A
divergence there is a purge that deletes without the backup somebody was
promised. So 181 lines of `scripts/lifecycle.py` went: it keeps the half this
command cannot see, which is what `make install` recorded writing, reads the
plan from `purge --dry-run --json`, shows both halves, asks one question about
both, and runs `purge --confirm --no-plan`.

That inverts the order — the binary that deletes the data is one of the files
uninstall removes — and the inversion broke something I had not predicted. The
install manifest lives *inside the data root*, so the data purge takes it, and
the uninstall that followed found no manifest and refused: notes gone, installed
files left behind. `scripts/test_lifecycle.py` caught it immediately. The
manifest is read once before the deletion now, and handed to the uninstall after
it.

The 27 tests that already passed could not tell whether the delegation happened
at all — they assert the data is gone, not who removed it, and they stayed green
with an `rmtree` put back in the Python. Two new ones replace the installed
command with one that plans truthfully and then does nothing: the data must
survive, and when the command fails the installed files must survive too. That
suite also stopped using a shell stub for `notriosctl`, which was fine while
purge only *asked* it where the roots were; it builds and installs the real one.

## Still owed

Nothing populates the oracle's `ExternalProfilePaths`. The rule is there, two of
H3's fixtures exercise it, and **both callers pass an empty list** — the Python
did too, before it delegated, so this is not something the port lost. A profile
whose data lives outside the six resolved roots is therefore never deleted,
which is right, and never enumerated, backed up or mentioned either, which is
not. The rule's own wording is "enumerated and backed up, never deleted
automatically", and only the second half of that is true today.

It matters because of what the user is shown. Purge prints what it will remove
and asks one question about it; an external library is silently absent from that
list, so somebody with several profiles cannot act on a warning nobody gives.
The fix is to read the profile registry in the command, resolve each profile's
database and asset store, pass the ones outside the owned roots, and then report
them as enumerated-not-deleted. I4's external-data-root drill is the one to run
against it.

Four of I4's nine drills are also not carried over, because they exercise the
installed-file half this command deliberately does not touch: they belong to
`make purge`, and `scripts/test_lifecycle.py` runs them. `DRILLS.jsonl` names
each one rather than running fewer quietly, and the validator fails if one stops
being named.
