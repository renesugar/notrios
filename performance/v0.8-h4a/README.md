# v0.8 H4a — distinct development and installed default ports

A source checkout and an installed Notrios can now run at the same time with no
flags and no edits. Slice D had already made their **libraries** separate; the
only thing left holding them apart by luck was the port, and both defaulted to
`127.0.0.1:8080`, so the second one to start simply failed to bind.

| Instance | Address | Where it comes from |
|---|---|---|
| installed, or any binary with no configuration file | `127.0.0.1:8080` | the compiled default, unchanged |
| a source checkout | `127.0.0.1:8099` | `server.listen_addr` in `config/config.example.yaml` |

The compiled default did not move, so nothing about an installed instance
changed. A checkout picks up the new address because `config.example.yaml` is
read **only** from a source checkout.

## Four functional changes, not one

The plan described this as a documentation project with a one-line code change.
Three further couplings turned up on contact:

- **`make serve` passed `-addr 127.0.0.1:8080` explicitly**, which would have
  overridden the example config straight back onto the installed address. The
  target that exists to run a checkout was the one place forcing the collision.
- **`web/vite.config.ts` proxies `/api` and `/healthz`** to the service running
  from this checkout. Left at 8080, `npm run dev` would have proxied a
  developer's requests to whatever else was on that port — possibly an installed
  Notrios, and so a different library.
- **The bind-failure message recommended `-addr 127.0.0.1:8099`.** Added in
  slice D, when 8099 was a free suggestion; after this change it is the port a
  checkout expects to own, so the advice would have moved an installed instance
  directly onto it. It now recommends 8081 and explains that the two defaults no
  longer collide.

## What guards it

`TestTheExampleConfigAndTheCompiledDefaultUseDifferentPorts` in
`internal/config` asserts the two addresses differ and that
`public_base_url` follows `listen_addr`. Nothing else would catch a regression:
both values are valid addresses and every other test passes with them equal —
the failure only appears when two instances run at once, which no unit test
does.

`validate_evidence.py` re-checks the same invariant from outside the Go tests,
and asserts every count in `PORT_MENTIONS.json` still matches the tree, so the
page-by-page reading below cannot quietly go stale.

## Which mentions changed, and which did not

`PORT_MENTIONS.json` records every page with its counts and the reason. The rule:

> A mention changes when the surrounding prose tells the reader to start or open
> a **source checkout**. It stays at 8080 when it states the compiled default,
> describes an installed or remote instance, or is a docexec substitution token
> inside an executed example.

That last case is why `docs/api/rest.md` keeps 55 literals. Each executed
example declares `http://127.0.0.1:8080` as a substitution token that docexec
replaces with the live fixture URL before running it, so the literal is a
placeholder rather than a claim about which instance the reader has. Rewriting
them would have churned about thirty registry entries — token and hash — for no
reader benefit. The page's *prose* did change: it told the reader to start from
the checkout's example config and then curl 8080, which after this change is two
different instances in consecutive sentences.

`docs/cli.md` keeps all three of its mentions by the plan's boundary: two are
`profile create --listen` synopses, and profile creation is explicitly out of
scope, and the third is sample `config show` output whose whole point is the
origin column reading `compiled`.

## Verification

Both instances were started together and each answered on its own address from
its own database:

```
notriosd listening on http://127.0.0.1:8099 using db .../devlib/notes.sqlite
notriosd listening on http://127.0.0.1:8081 using db .../installed/home/.local/share/notrios/notes.sqlite
curl 127.0.0.1:8099/healthz -> ok
curl 127.0.0.1:8081/healthz -> ok
```

**One limitation, recorded rather than papered over.** The installed instance
could not be exercised on its own default here: an unrelated process on this
machine already holds `*:8080`, and it was left alone. `8081` stands in for it
above. That the port was taken by something that is not Notrios at all is itself
the argument for the change, and it is why the bind-failure message now names a
non-Notrios program as a likely cause.

The defaults themselves were verified directly, with no flags in either case:

```
checkout:  server.listen_addr  127.0.0.1:8099   file      (config/config.example.yaml)
installed: server.listen_addr  127.0.0.1:8080   compiled  (built-in defaults)
```
