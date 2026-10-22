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

## The rule that existed and could not fire

`ExternalProfilePaths` is an oracle rule with two H3 fixtures behind it, and
**both callers passed an empty list** — the Python did too, before it delegated,
so the Go port lost nothing. The rule was unreachable. A profile whose library
lives outside the six resolved roots was never deleted, which is right, and
never enumerated or mentioned, which is not: purge prints what it will remove
and asks one question about it, and an external library was silently missing
from that list. Nobody can act on a warning nobody gives.

`notriosctl purge` now reads the profile registry, resolves each profile's
database, asset store and config path, and passes the ones outside the owned
roots. They appear in the plan as `NOT DELETED`, with the profile that names
them and the field that named it, because "which of my profiles is this?" is the
first question that line provokes. The oracle gets the same list, so a root that
*contains* an external profile is now refused rather than deleted with the
profile inside it.

Registry failures produce a notice, not an error, and not silence. A registry
that cannot be located or parsed must not stop a user deleting their own data —
but "no external profiles" and "I could not tell" are different sentences and
only one is safe to act on, so the plan says which.

The seventh drill proves the listing rather than the deletion, and it was
confirmed by breaking it: with the enumeration removed the plan no longer names
the external library and the drill fails. It also asserts the library is still
*readable* afterwards — counted by hits, which is I7's lesson — because a file
that survived as bytes but cannot be opened is not a library anybody kept.

## The enumeration must not be able to block a purge

Handing the enumerated paths to the oracle looked obviously right and was
wrong, and only a run showed it. Every enumerated path is outside every owned
root — that is what made it external — so the rule can never fire to *protect* a
root. The single case it fires on is the reverse: a profile naming an **ancestor**
of the roots, which `profile register --asset-store ~/.local/share` produces by
accident. The data root was then `REFUSED`, the user's notes survived a
confirmed purge, and the line said the library had been "enumerated and backed
up" when nothing had been copied anywhere. A purge that silently keeps the
library is the worst outcome this item has.

External paths now inform the user and nothing else, and an eighth drill holds
it that way: a profile naming a parent of the roots refuses nothing, the library
is deleted, the named parent survives, and the plan says on that same line that
the roots inside it are still deleted — because "not deleted" is true of the
directory and badly misleading about its contents.

The owner decided the same day that an external library is reported and neither
deleted nor copied. `BackupPolicy("external")` is still named
`backup_never_delete`, and it stays named that: it is H3's word for H3's intent,
it lives in sealed evidence that fixtures gate against, and evidence is not
rewritten to agree with later code. The plan output says "neither deletes nor
copies it" so the reader gets the behaviour rather than the policy name, and
both call sites carry a comment saying why.

Four of I4's nine drills are also not carried over, because they exercise the
installed-file half this command deliberately does not touch: they belong to
`make purge`, and `scripts/test_lifecycle.py` runs them. `DRILLS.jsonl` names
each one rather than running fewer quietly, and the validator fails if one stops
being named.
