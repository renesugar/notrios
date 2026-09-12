# Installation and building

There are two ways to install Notrios, and this page is ordered for the first
one: **from the Ubuntu package**, which needs no repository, no Go, no Node and
no compiler. Building from a clone is the second half of the page.

**There is no published download yet.** The package is built from this
repository and has not been released anywhere, so today you either receive one
or build one. What has been established is that it *works* once you have it: it
installs on a clean Ubuntu 24.04 with only its declared dependencies, runs as an
unprivileged user, survives upgrade, removal and reinstallation, and keeps your
notes when the program is removed. That was measured rather than assumed — see
[what is supported](#what-is-supported).

The only tested platform is **Ubuntu 24.04 on amd64**. arm64 is built and never
run; Windows and macOS are neither built nor tested here, and no support is
claimed for them.

## Installing from the package

You need the `.deb` and nothing else. `apt` resolves the dependencies:

```sh
sudo apt install ./notrios_0.8.0-1_amd64.deb
```

That installs `/usr/bin/notrios`, `/usr/bin/notriosd` and `/usr/bin/notriosctl`,
the built interface and help under `/usr/share/notrios`, the `notrios://`
desktop entry, and a user service that is **installed and not enabled**. Nothing
starts on its own.

Check the installation before trusting it:

```sh
notriosctl doctor
```

`doctor` reports each check and exits non-zero if a required one failed. On a
machine with no keyring — a server, a container — it will tell you that no
credential store is reachable and that this is not a failure until you store
sync keys. Paths are printed with your home directory shown as `~`; pass
`--no-redact` when you need the literal path.

### Verifying what you downloaded

A release set carries `SHA256SUMS` beside the artifacts:

```sh
sha256sum -c SHA256SUMS
```

**Nothing is signed yet.** The signing policy is decided — a detached OpenPGP
signature over each artifact and an RFC 3161 timestamp over it — and no key has
been created, so there is no signature to check and you should not believe a
file that claims otherwise. A checksum tells you the download is intact; it does
not tell you who made it.

### Upgrading

Install the newer package over the older one. Your library is not touched:

```sh
sudo apt install ./notrios_0.8.0-1_amd64.deb
```

The binaries are replaced and the notes stay where they are, in your home
directory rather than anywhere the package manager owns.

### Going back to an earlier version

`apt` refuses to go backwards unless you ask it to, which is deliberate:

```sh
sudo apt install --allow-downgrades ./notrios_0.8.0~rc1-1_amd64.deb
```

Without the flag the refusal leaves the newer package installed and your library
untouched. With it, the older program is installed and your library is still
untouched — a rollback moves the program, never the notes. If the older version
predates a schema migration, see [upgrading from before 0.8](#upgrading-from-before-08).

### Backing up, and getting your notes back

Export writes a portable archive you can copy anywhere:

```sh
notriosctl export archive ~/notrios-backup
```

To restore into an empty or replacement library:

```sh
notriosctl import archive ~/notrios-backup
```

That is the recovery path for a lost library, and it is the one to rehearse
before you need it. `purge` also writes a verified backup before it deletes
anything — see [removing your data as well](#removing-your-data-as-well) — but a
backup you took on purpose is better than one a deletion made for you.

### Removing Notrios when you installed the package

Removing the program and removing your notes are two different acts, and the
package only does the first:

```sh
sudo apt remove notrios
```

That deletes every file the package installed and **leaves your notes,
configuration, profiles, keys, state and cache exactly where they are**. Install
the package again and they are still there. That was measured, not assumed:
`performance/v0.9-i3` removes the package, checks the library on disk, reinstalls
and finds the note again.

**There is no packaged way to delete your data, and you should know that before
you need it.** `make purge` — which takes a verified backup first, refuses
without `FORCE=1` when nothing can answer a prompt, and excludes sync key
material from the backup — lives in the repository, and the package does not
ship it. With only the package installed, deleting your library is something you
do yourself. Find out where it is first:

```sh
notriosctl paths --no-redact
```

Delete the `data`, `config`, `state` and `cache` roots it prints, and understand
that nothing takes a backup for you when you do it that way. Export first if the
notes matter:

```sh
notriosctl export archive ~/notrios-backup
```

## What is supported

| Surface | Platform | Status |
|---|---|---|
| command line and service | Ubuntu 24.04 amd64 | installed and run |
| web interface | browsers driven by the journey suite | exercised |
| desktop shell | Ubuntu 24.04 amd64 | **compiled, never run here** |
| command line and service | Ubuntu 24.04 arm64 | built, never run |
| anything | Windows, macOS | not built, not claimed |

The desktop shell is the row worth reading twice: continuous integration
compiles it, and nothing in this project has launched its window, so it is not
claimed as supported.

## Prerequisites for building from source

| Requirement | Version | Needed for |
|---|---|---|
| Go | 1.25 or newer (`go.mod` says `go 1.25.0`; CI uses 1.25) | everything |
| C toolchain | Ubuntu `build-essential` | the SQLite store is a cgo wrapper over the vendored SQLite amalgamation, compiled from source into the binary |
| Node.js + npm | the version in `.nvmrc`, which CI reads too | building the web UI and the documentation site |
| `libgtk-3-dev`, `libwebkit2gtk-4.1-dev` | Ubuntu packages | **GUI builds only** (`make gui`) |
| Python 3 | Ubuntu `python3` | repository validation scripts only (not needed at runtime) |
| `zip` | Ubuntu package | release archives only (`scripts/package_release.sh`) |
| Recoll (`recollindex`, `recollq`) | optional | the optional [search sidecar](service.md#search-sidecar); everything works without it |

Ubuntu setup:

```sh
# Service + CLI + tests
sudo apt update
sudo apt install -y build-essential pkg-config libsqlite3-dev git

# Web UI / docs site (if Ubuntu's Node is older than 20.19, install Node 22
# from nodesource or a version manager instead)
sudo apt install -y nodejs npm

# Desktop GUI builds only
sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev

# Optional extras
sudo apt install -y python3 zip recoll
```

Install Go 1.25+ from [go.dev/dl](https://go.dev/dl/) if your distribution's Go is older.

## Clone

```sh
git clone https://github.com/renesugar/notrios.git
cd notrios
```

## Build

Every target lives in the `Makefile`; `make help` prints the same list. Output
paths are git-ignored. **Network** marks the targets that reach the network.

| Command | What it does | Output | Network |
|---|---|---|---|
| `make help` | print this list | — | |
| `make deps` | install frontend dependencies from the lockfile (`npm ci`) | `web/node_modules/` | ✅ |
| `make build` | headless service + CLI | `bin/notriosd`, `bin/notriosctl` | |
| `make build-service` | just the service | `bin/notriosd` | |
| `make build-cli` | just the CLI | `bin/notriosctl` | |
| `make web` | production web assets (runs `make deps` on first build) | `web/dist/` | first run |
| `make gui` | desktop GUI binary (implies `make web`) | `bin/notrios` | first run |
| `make docs` | documentation site with PageFind search (uses `npx`) | `_site/` | ✅ |
| `make test` | all Go tests | — | |
| `make validate` | tests plus scaffold and script checks | — | |
| `make smoke` | end-to-end REST/MCP smoke test on a loopback port | — | |
| `make serve` | **run the service from source** on `127.0.0.1:8099`, for opening the UI in a browser | — | |
| `make doctor` | check the configuration and environment (`notriosctl doctor`) | — | |
| `make seed-help` | mirror `docs/` into the built-in Help notebook of the default database | — | |
| `make install` | install to an end-user location (`prefix=$HOME/.local` by default) | `~/.local/...` | |
| `make uninstall` | remove exactly what `install` recorded installing | — | |
| `make purge` | uninstall **and** delete this user's data, after a verified backup | — | |
| `make deb` | build the internal Ubuntu package (needs `dpkg-dev`) | `dist/deb/` | |
| `make clean` | remove build/test/docs/release output — **never** `data/` and **never** `web/node_modules/` | — | |
| `make clobber` | `clean` plus remove `web/node_modules/` | — | |
| `make precheck` | fail if the tree has uncommitted changes or tracked ignored files | — | |

Frontend dependencies install once into `web/node_modules` via `npm ci` (the
lockfile-exact install). Rerun `make deps` after pulling lockfile changes.

**`make clean` does not remove `web/node_modules`** — that is deliberate, so a
clean never forces a network reinstall. Use `make clobber` when you want the
dependencies gone too.

## Run

From the repository root:

```sh
# Headless service (REST + MCP + web UI at http://127.0.0.1:8099)
./bin/notriosd -config config/config.example.yaml

# Desktop GUI (contains the service; see the GUI guide for -no-gui / -gui-only)
./bin/notrios

# CLI
./bin/notriosctl doctor
```

You can also run everything from source without building binaries:

```sh
go run ./cmd/notriosd -config config/config.example.yaml
go run ./cmd/notriosctl doctor
```

### The two ways to open the interface

**As a desktop window.** `./bin/notrios` starts the service and opens the GUI in
one process. Build it with `make gui` first.

**In a browser.** Start the service and visit it — `make serve` runs it from
source on `http://127.0.0.1:8099` without building any binaries, which is the
quickest loop while developing. `./bin/notriosd` does the same from a built
binary. Both serve the identical interface the desktop window renders; the
desktop binary is a webview around it.

### Where the interface files have to be

The built interface is `web/dist/`, produced by `make web` (and by `make gui`,
which implies it). A binary looks for it in this order and uses the first hit:

1. the path given by `--web-dir`, or `server.web_dir` in the configuration file
   — when either is set it is the **only** candidate, because an explicit answer
   that is wrong should fail loudly rather than fall through to a directory that
   happens to work;
2. `web/` under the program-assets root (`/usr/local/share/notrios/web` on
   Linux) — installed layouts only;
3. `web/dist` under the **executable's own directory**;
4. `web/dist` under the executable's **parent** directory;
5. `web/dist` under the **working directory** — *source checkouts only*.

Items 3 and 4 are what make `bin/notrios` work whether you run it as
`./bin/notrios` from the repository root or as `./notrios` from inside `bin/`.

The working directory comes **last and only in a checkout**. It used to come
first, unconditionally, which is right for a developer standing in a checkout
and wrong once the binary is installed: the HTML, CSS and JavaScript loaded into
the application's own window were taken from whichever `web/dist` sat beside
wherever the user happened to be.

If you move a binary somewhere else, copy `web/dist/` alongside it or pass
`--web-dir`:

```sh
./notrios --web-dir /opt/notrios/web/dist
```

The GUI **refuses to start** when it cannot find the interface, and prints every
directory it tried. The headless service starts anyway and says so in its log —
REST, MCP, and the importers do not need an interface — and `/` then answers
`web_ui_not_found` with the same list.

`-gui-only` needs no local interface at all: it renders whatever the remote
service serves.

### Where your files go

Run `notriosctl paths` to see exactly where this instance keeps things, and
which layout it selected.

An installed binary uses the native per-OS locations: on Linux
`$XDG_DATA_HOME/notrios` for the library, with separate config, state, cache and
runtime roots. A binary run from a checkout is a *separate instance* — every
root is checkout-local, under `./data`, so a development build cannot read or
write the library of an installed one.

Paths you state yourself are used exactly as written and never relocated. The
checkout's example config uses relative `./data/...` paths, which resolve
against the directory you launched from; use absolute paths in any config you
run outside a checkout. See the [service guide](service.md#configuration).

### Upgrading from before 0.8

Before 0.8 the built-in defaults were relative to the working directory: a
binary run with **no configuration file** kept its library in `./data`, under
whichever directory you launched from. An 0.8 binary resolves the native roots
instead, so it will find them empty and open a new, empty library. Your notes
are not gone — they are still in that directory.

`notriosctl paths` notices a pre-0.8 library in the directory you are standing
in and names it. Moving it is a single explicit command:

```sh
notriosctl migrate --dry-run   # show what would move
notriosctl migrate             # copy, verify, and retire the old directory
```

Nothing is moved for you, the original is copied rather than moved, every file
is verified with SHA-256, and the old directory is renamed rather than deleted.
See [`notriosctl migrate`](cli.md#migrate) for the full contract.

**You do not need this** if your configuration file states its paths — those are
used exactly as written and were never relocated — or if you run from a
checkout, whose roots are already `./data`.

## Installing to an end-user location

`make install` puts Notrios where an end user would have it — outside the
checkout — so you can run it the way they will:

```sh
make install
```

It installs to `$HOME/.local` by default, which needs no `sudo`: the binaries go
to `~/.local/bin`, the built interface and the help docs to
`~/.local/share/notrios`, and the `notrios://` desktop entry to
`~/.local/share/applications`. Add `~/.local/bin` to your `PATH` if it is not
there already. Nothing is written to any config, data, state, cache or runtime
root — an installed Notrios creates those itself, on first use.

Use the GNU directory variables to put it elsewhere, and `DESTDIR` to stage it
for packaging:

```sh
make install prefix=/usr/local            # system-wide; needs write access
make install DESTDIR=/tmp/stage           # stage for a package, touching no real root
make install-dry-run                      # print every path, write nothing
```

`DESTDIR` is prepended to installed files and to nothing else. It never creates
a user's roots, because a package built on one machine must not ship that
machine's idea of a home directory.

### Uninstalling

```sh
make uninstall            # remove what install recorded installing
make uninstall-dry-run    # list what that would be, remove nothing
```

Install records every file it wrote — path, SHA-256, mode and size — in
`~/.local/share/notrios/MANIFEST.json`, and uninstall removes **only** what that
manifest lists. Anything else is left alone:

- a file you edited is kept and reported, because its hash no longer matches;
- a path that has become a symbolic link is kept and never followed, because
  deleting through it would remove whatever it points at;
- anything resolving outside the manifest's own install roots is refused.

**Your notes, configuration, profiles, sync keys, state and cache are never
touched.** Uninstall is about the program; the library outlives it, and
reinstalling picks it straight back up.

### Installing from a package

There is an internal Ubuntu package. It is **not published anywhere** and is not
a supported release; it exists so the application can be installed and used the
way an end user would, without a checkout.

```sh
make deb
sudo apt install ./dist/deb/notrios_*.deb
```

It installs `/usr/bin/{notrios,notriosd,notriosctl}`, the built interface and
help under `/usr/share/notrios`, the `notrios://` desktop entry, and a **user
service that is installed and not enabled**. Nothing starts on its own; turn it
on yourself if you want it:

```sh
systemctl --user enable --now notrios
```

The service binds loopback, exactly as it does everywhere else.

Dependencies are computed from the binaries with `dpkg-shlibdeps` rather than
written by hand, so `apt` installs what the program actually needs. Removing the
package with `apt remove` deletes every file it installed and **leaves your
notes, configuration, profiles, keys, state and cache alone** — deleting those
is `notriosctl purge`, which asks first and takes a verified backup before it
deletes anything. It removes your data and leaves the program to your package
manager.

### Removing your data as well

There are two ways in, and they do the same deleting.

`notriosctl purge` deletes this user's Notrios data. It ships with the program,
so it is the one to use if you installed a package:

```sh
notriosctl purge --dry-run   # print the whole plan and stop; asks nothing
notriosctl purge             # shows the plan, then asks
notriosctl purge --confirm   # skip the question, for headless automation
```

`make purge` is that **plus** removing what `make install` recorded installing,
and needs a checkout:

```sh
make purge                   # shows the plan, then asks; type PURGE to confirm
DRYRUN=1 make purge          # print the whole plan and stop; asks nothing
FORCE=1 make purge           # skip the question, for headless automation
```

`make purge` runs `notriosctl purge` to do the deleting rather than deleting by
itself, so everything below is true of both. It deletes your data first and
removes the program second, because the program is what deletes the data.

Before deleting anything it copies your config, data and state roots into an
owner-only archive beside your state root
(`~/.local/state/notrios-purge-backups/<timestamp>/`), verifies that archive
against a per-file SHA-256 manifest, and only then removes anything. **If the
backup cannot be verified, nothing is deleted** and the message names the
partial archive. The backup is outside every root purge removes, so it survives
the purge that wrote it, and nothing deletes it for you afterwards.

The cache and runtime roots are deleted without a backup: they are rebuilt from
the library and hold nothing you wrote.

**A profile whose library lives somewhere else is listed and kept.** If you
registered a profile with a database outside the roots above — on another
volume, say — purge names it as `NOT DELETED`, says which profile names it, and
leaves it alone. It is not copied into the backup either, because it is not
being deleted: if you want it somewhere else as well, copy it yourself. Purge
never deletes a path outside the roots it owns, and if it cannot read your
profile registry it says so rather than assuming you have no such profile.

`FORCE=1` skips the confirmation, never the backup. If you genuinely want
neither:

```sh
NO_BACKUP=1 make purge       # still asks, and warns in detail first
notriosctl purge --no-backup # the same, for a packaged install
```

`NO_BACKUP=1` prints exactly what is about to be destroyed with no copy — your
notes database, attachments, configuration, profile registry, sync key material,
sync spools and backups, the catch-up inbox and the quarantine — and still asks
unless `FORCE=1` is also set.

These flags accept `1` or nothing at all. `NO_BACKUP=0`, `FORCE=no` and
`DRYRUN=true` are refused rather than interpreted: each reads to a person as
something specific, and guessing wrong here deletes a library. A purge that
cannot ask — no terminal, no `FORCE=1` — stops rather than proceeding or
hanging.

**`clean` and `clobber` never touch any of this**, and `install`, `uninstall`
and `purge` never touch the checkout. They are separate concerns with separate
targets.

## Ways to consume Notrios

- **Run from source** (`go run ./cmd/...`) — best for development; nothing to install, always current.
- **Local binaries** (`make build`, `make gui`) — fast startup, same behavior; outputs under `bin/`.
- **Source release ZIP** (`bash scripts/package_release.sh`, output `dist/notrios-src.zip`) — a reviewed source snapshot *including* prebuilt `web/dist/` assets, intended for archiving or importing into another repository host. It is **not** a binary distribution: recipients still build with Go on their machine.

There are no official prebuilt binaries or OS installers at this time.

## Next steps

- [Configure and run the service](service.md)
- [Use the desktop GUI](gui.md)
- [Import your notes](import-export.md) (Joplin, Obsidian, Twitter/X, ChatGPT, Claude)
- [Troubleshooting](troubleshooting.md)
