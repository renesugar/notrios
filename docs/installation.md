# Installation and building

There are two ways to install Notrios, and this page is ordered for the first
one: **from the Ubuntu package** a release publishes, which needs no repository,
no Go, no Node and no compiler. Building from a clone is the second half of the
page.

**The supported platform is Ubuntu 24.04 on amd64.** The package has been
installed there in clean containers that hold no source tree, no Go and no
compiler, run as an unprivileged user, upgraded, rolled back, removed and
reinstalled. Every other platform is listed in
[what is supported](#what-is-supported) with what ran on it and what did not.
Windows and macOS are not built and not claimed.

## Installing from the package

Releases are published at
[github.com/renesugar/notrios/releases](https://github.com/renesugar/notrios/releases).
A release set is these files:

| File | What it is |
|---|---|
| `notrios_1.0.0-1_amd64.deb` | the package |
| `notrios_1.0.0-1_amd64.deb.asc` | a detached OpenPGP signature over the package |
| `notrios_1.0.0-1_amd64.deb.asc.tsq`, `.tsr` | an RFC 3161 timestamp request and reply over that signature |
| `SBOM.cdx.json` | the software bill of materials, in CycloneDX |
| `PROVENANCE.json` | an in-toto statement of how the package was built |
| `SHA256SUMS`, `SHA256SUMS.asc` | the checksum of every file above, and a signature over the checksums |

[Verify the files](#verifying-what-you-downloaded) before you install them. Then
`apt` resolves the dependencies:

```sh
sudo apt install ./notrios_1.0.0-1_amd64.deb
```

That installs `/usr/bin/notrios`, `/usr/bin/notriosd` and `/usr/bin/notriosctl`,
the built interface and help under `/usr/share/notrios`, the `notrios://`
desktop entry, and a user service that is **installed and not enabled**. Nothing
starts on its own.

Check the installation before trusting it:

<!-- notrios:generated:example:installation-installing-from-the-package-example-2:begin -->
```sh
notriosctl doctor
```
<!-- notrios:generated:example:installation-installing-from-the-package-example-2:end -->

`doctor` reports each check and exits non-zero if a required one failed. On a
machine with no keyring — a server, a container — it will tell you that no
credential store is reachable and that this is not a failure until you store
sync keys. Paths are printed with your home directory shown as `~`; pass
`--no-redact` when you need the literal path.

### Verifying what you downloaded

There are four checks, and each answers a different question. Run them in the
directory that holds the downloaded files.

**Is the download intact?**

```sh
sha256sum -c SHA256SUMS
```

Every line must end `OK`. A checksum says the files arrived as they were
published. It does not say who published them.

**Who made it?** The signature answers that, against the project's public key.
Take the key from the repository rather than from the release page: a key that
comes from the same place as the file it vouches for vouches for very little.

```sh
curl -fsSLO https://raw.githubusercontent.com/renesugar/notrios/main/keys/release-public.asc
gpg --show-keys --with-fingerprint release-public.asc
```

The primary key's fingerprint must be
`1234 C691 AC07 76A1 8524  D556 8702 7B1D D464 695E`. If it is, import it and
check both signatures:

```sh
gpg --import release-public.asc
gpg --verify SHA256SUMS.asc SHA256SUMS
gpg --verify notrios_1.0.0-1_amd64.deb.asc notrios_1.0.0-1_amd64.deb
```

Each must print `Good signature from "Rene Sugar <rene.sugar@gmail.com>"` and
the primary fingerprint above. The signature itself is made by the key's
signing subkey, `348B 6B87 3994 D1AF F924  D7FA F1BF 879D CE41 EA2C`, and gpg
prints both. gpg also warns `This key is not certified with a trusted
signature!` That means you have not certified the key yourself, and it is
expected. Comparing the fingerprint is the step you are relying on. `BAD
signature` means the file is not the one that was signed.

**When was it signed?** The timestamp is a third party's statement that the
signature existed at a given time. It keeps meaning something after the key
expires or is revoked. Check it against the authority's certificates, which are
in the repository too:

```sh
curl -fsSLO https://raw.githubusercontent.com/renesugar/notrios/main/evidence/current/TRUST/tsa-root.pem
curl -fsSLO https://raw.githubusercontent.com/renesugar/notrios/main/evidence/current/TRUST/tsa-untrusted.pem
openssl ts -verify -data notrios_1.0.0-1_amd64.deb.asc -in notrios_1.0.0-1_amd64.deb.asc.tsr \
  -CAfile tsa-root.pem -untrusted tsa-untrusted.pem
```

The verification must print `Verification: OK`. It also warns that the
certificate in `tsa-untrusted.pem` `is not a CA cert`. That is expected: the
file holds the responder's own certificate, not an authority's. A changed
signature prints `message imprint mismatch` and `Verification: FAILED`, and
exits non-zero.

The time it attests is in the reply:

```sh
openssl ts -reply -in notrios_1.0.0-1_amd64.deb.asc.tsr -text | grep 'Time stamp'
```

**Where was it built?** The attestation says the package came out of this
repository's release workflow, from a named commit, on GitHub's runner. It is
checked with the [GitHub CLI](https://cli.github.com/):

```sh
gh attestation verify notrios_1.0.0-1_amd64.deb --repo renesugar/notrios \
  --signer-workflow renesugar/notrios/.github/workflows/release.yml
```

It exits 0 when the attestation verifies, and it can print nothing when it
does. Add `--format json` to see what it established: the SLSA provenance
predicate, the workflow, the commit and the issuer. **It needs a GitHub login**
(`gh auth login`). Without one, it stops and asks for one.

The C library and header (`notrios-c-abi-1.0.0-linux-amd64.tar.gz`) come with
a checksum file of their own. They are checked with
`sha256sum -c notrios-c-abi-1.0.0-linux-amd64.tar.gz.sha256`. They are not
signed. See [the C ABI](../PACKAGING.md) for what the tarball holds.

### Upgrading

Install the newer package over the older one:

```sh
sudo apt install ./notrios_1.0.0-1_amd64.deb
```

The binaries are replaced and the notes stay where they are, in your home
directory rather than anywhere the package manager owns.

**Upgrading from 0.8 to 1.0 changes the library as well as the program.** 1.0
moves a library from schema 27 to 28 the first time it opens it, and 0.8 will
not open it afterwards. Your notes come through: an upgrade from the 0.8.0
package keeps every note, and the library takes new ones. Going back is then not
only a matter of reinstalling 0.8; see the next section.

**Upgrade every machine that syncs together.** 0.8 and 1.0 do not sync with each
other by any path. An older version refuses an invitation from 1.0. When the
older version invites, pairing completes and then nothing moves, because each
side skips what the other publishes. Replicas that were already syncing carry on
once all of them run 1.0.

**Do not open a 1.0 library with 0.7.0.** 0.7.0 predates the check that refuses
a library from a newer version. It opens a 1.0 library and rewrites its schema
version from 28 to 27. The notes are intact, and opening the library with 1.0
again repairs the version. 0.8.0 refuses a 1.0 library instead, and changes
nothing.

### Going back to an earlier version

`apt` refuses to go backwards unless you ask it to, which is deliberate, and a
refused downgrade leaves the newer package installed and your library
untouched. Between two builds of the same schema, asking is all it takes. This
is the rehearsal's pair, a 1.0.0 release candidate under 1.0.0:

```sh
sudo apt install --allow-downgrades ./notrios_1.0.0~rc1-1_amd64.deb
```

**From 1.0 back to 0.8, moving the program is not enough.** 0.8 refuses a
library 1.0 has opened, and says so:

```text
database schema is newer than this build supports: the database records schema version 28 and this build supports 27.
```

So take the notes with you in an archive. Do it with 1.0 before downgrading,
because 0.8 can no longer read the library:

<!-- notrios:generated:example:installation-going-back-to-an-earlier-version-example-2:begin -->
```sh
notriosctl export archive-v2 ~/notrios-backup
notriosctl verify archive-v2 ~/notrios-backup
```
<!-- notrios:generated:example:installation-going-back-to-an-earlier-version-example-2:end -->

Then downgrade, set the 1.0 library aside rather than deleting it, and restore
the archive into the new, empty library 0.8 creates:

<!-- notrios:generated:example:installation-going-back-to-an-earlier-version-example-3:begin -->
```sh
sudo apt install --allow-downgrades ./notrios_0.8.0-1_amd64.deb
mv ~/.local/share/notrios ~/.local/share/notrios.from-1.0
notriosctl restore archive-v2 --intent adopt ~/notrios-backup
```
<!-- notrios:generated:example:installation-going-back-to-an-earlier-version-example-3:end -->

`~/.local/share/notrios` is the data root `notriosctl paths --no-redact` prints
for a packaged install that uses the default locations. Use the path it prints
for you. 0.8 restores a 1.0 archive completely: an archive declares schema 28,
but nothing in it is beyond 0.8's reader, because schema 28 adds only a private
search index that archives do not carry.

### Backing up, and getting your notes back

The lossless backup is an archive-v2. It is verified before the command reports
success, and you can verify it again at any time without opening a library:

<!-- notrios:generated:example:installation-backing-up-and-getting-your-notes-back-example-1:begin -->
```sh
notriosctl export archive-v2 ~/notrios-backup
notriosctl verify archive-v2 ~/notrios-backup
```
<!-- notrios:generated:example:installation-backing-up-and-getting-your-notes-back-example-1:end -->

Keep it on another disk. A backup on the disk it protects is lost with it.

To recover a lost library, restore the archive into an empty one. `adopt` keeps
the archive's logical database identity and gives this copy a new replica
identity:

<!-- notrios:generated:example:installation-backing-up-and-getting-your-notes-back-example-2:begin -->
```sh
notriosctl restore archive-v2 --intent adopt ~/notrios-backup
```
<!-- notrios:generated:example:installation-backing-up-and-getting-your-notes-back-example-2:end -->

That is the recovery path, and it is worth rehearsing before you need it. It
has been rehearsed here at scale. A 382,206-note, 7.35 GB library was exported
in 55 minutes to a 1.25 GB archive, verified, and destroyed by a purge. It was
then restored in 28 minutes, and its contents and search results matched the
original exactly. The export never used more than 324 MiB of memory.

`notriosctl export archive` writes a different, human-readable archive meant
for moving notes between libraries. It is not a lossless backup.

`purge` also writes a verified backup before it deletes anything; see
[removing your data as well](#removing-your-data-as-well). That backup is a
`tar` file you can extract, and it is a second way back. But a backup you took on
purpose is better than one a deletion made for you.

### Removing Notrios when you installed the package

Removing the program and removing your notes are two different acts, and the
package only does the first:

```sh
sudo apt remove notrios
```

That deletes every file the package installed and **leaves your notes,
configuration, profiles, keys, state and cache exactly where they are**. Install
the package again and they are still there.

Deleting your data as well is `notriosctl purge`, which ships with the package.
It shows what it will delete, asks first, and writes a verified backup before it
deletes anything. See [removing your data as well](#removing-your-data-as-well).

## What is supported

Each row names what was executed at 1.0.0 to entitle it. A row that is not
supported says what it lacks. The rows are generated from
`performance/v1.0-j9/MATRIX.json`, which takes the libraries from the built
package rather than from a list somebody wrote.

<!-- notrios:generated:matrix:rows:begin -->
| Surface | Platform | Status | Needs at run time |
|---|---|---|---|
| command line and background service | Ubuntu 24.04 amd64 | installed and run at 1.0.0 | libc6 (>= 2.34); newest glibc symbol GLIBC_2.34 |
| C library and header (libnotrios.so.1) | Ubuntu 24.04 amd64 | built, compiled against and run at 1.0.0 | libc6 (>= 2.34); newest glibc symbol GLIBC_2.34 |
| web interface | Google Chrome 156.0.8078.47 on Ubuntu 26.04.1 amd64 | every control driven at 1.0.0 | a browser on the same machine as the service, which binds loopback |
| web interface | WebKit 26.5 (Playwright 1.61.1's build) | preview only, not claimed | — |
| desktop shell (Wails v2, GTK3 / webkit2gtk-4.1) | Ubuntu 24.04 amd64 | **compiled and packaged, never run** | libc6 (>= 2.34), libgdk-pixbuf-2.0-0 (>= 2.22.0), libglib2.0-0t64 (>= 2.16.0), libgtk-3-0t64 (>= 3.21.4), libjavascriptcoregtk-4.1-0, libsoup-3.0-0 (>= 2.4.0), libwebkit2gtk-4.1-0 (>= 2.39.91); newest glibc symbol GLIBC_2.34 |
| command line and background service | Ubuntu 26.04.1 amd64 | built and tested from source, package not installed | — |
| command line and background service | Ubuntu 24.04 arm64 | built, never run | — |
| all | Windows | not built, not claimed | — |
| all | macOS | not built, not claimed | — |
<!-- notrios:generated:matrix:rows:end -->

The desktop shell is the row worth reading twice. It is compiled, packaged and
checked to be present in every release package, and nothing in this project has
opened its window, so it is not claimed as supported.

The web interface is served by the same program on every platform. The browser
row names the browser in which every control was driven, not the only browser
that works. Another browser is simply untested here.

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

<!-- notrios:generated:example:installation-run-example-1:begin -->
```sh
# Headless service (REST + MCP + web UI at http://127.0.0.1:8099)
./bin/notriosd -config config/config.example.yaml

# Desktop GUI (contains the service; see the GUI guide for -no-gui / -gui-only)
./bin/notrios

# CLI
./bin/notriosctl doctor
```
<!-- notrios:generated:example:installation-run-example-1:end -->

You can also run everything from source without building binaries:

<!-- notrios:generated:example:installation-run-example-2:begin -->
```sh
go run ./cmd/notriosd -config config/config.example.yaml
go run ./cmd/notriosctl doctor
```
<!-- notrios:generated:example:installation-run-example-2:end -->

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

<!-- notrios:generated:example:installation-upgrading-from-before-08-example-1:begin -->
```sh
notriosctl migrate --dry-run   # show what would move
notriosctl migrate             # copy, verify, and retire the old directory
```
<!-- notrios:generated:example:installation-upgrading-from-before-08-example-1:end -->

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

### Building the package yourself

`make deb` builds the same package a release publishes, from your checkout. It is
built here rather than by the release workflow, so it carries no attestation and
no signature: it is for installing what you built, not for passing on.

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
written by hand, so `apt` installs what the program actually needs. Without the
GTK and webkit2gtk-4.1 development headers, `make deb` builds a package with no
desktop binary and says `no desktop GUI`. The release workflow installs the
headers and refuses a package that lacks it. Removing the
package with `apt remove` deletes every file it installed and **leaves your
notes, configuration, profiles, keys, state and cache alone** — deleting those
is `notriosctl purge`, which asks first and takes a verified backup before it
deletes anything. It removes your data and leaves the program to your package
manager.

### Removing your data as well

There are two ways in, and they do the same deleting.

`notriosctl purge` deletes this user's Notrios data. It ships with the program,
so it is the one to use if you installed a package:

<!-- notrios:generated:example:installation-removing-your-data-as-well-example-1:begin -->
```sh
notriosctl purge --dry-run   # print the whole plan and stop; asks nothing
notriosctl purge             # shows the plan, then asks
notriosctl purge --confirm   # skip the question, for headless automation
```
<!-- notrios:generated:example:installation-removing-your-data-as-well-example-1:end -->

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
being deleted: if you want a copy elsewhere, make one yourself. If purge cannot
read your profile registry it says so, rather than assuming you have no such
profile.

Listing it is all that happens. A registered path never stops a purge: if you
pointed a profile's asset store at a directory that *contains* these roots —
`~/.local/share`, say — purge still deletes the roots and says so on the same
line, and leaves only that outer directory alone.

`FORCE=1` skips the confirmation, never the backup. If you genuinely want
neither:

<!-- notrios:generated:example:installation-removing-your-data-as-well-example-3:begin -->
```sh
NO_BACKUP=1 make purge       # still asks, and warns in detail first
notriosctl purge --no-backup # the same, for a packaged install
```
<!-- notrios:generated:example:installation-removing-your-data-as-well-example-3:end -->

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

### Checking that a purge removed everything

`notriosctl paths --report` lists every directory and file this installation
occupies, every registered profile — including one whose library lives outside
these roots — and the program files the installer recorded. It is worth keeping
before a purge, because afterwards it is the only record of what was there:

<!-- notrios:generated:example:installation-checking-that-a-purge-removed-everything-example-1:begin -->
```sh
notriosctl paths --report                      # for reading
notriosctl paths --report --json               # for a script
notriosctl paths --report --paths > ~/notrios-before-purge.txt
```
<!-- notrios:generated:example:installation-checking-that-a-purge-removed-everything-example-1:end -->

Save that last one somewhere the purge does not reach. Afterwards:

```sh
bash scripts/check_purged.sh ~/notrios-before-purge.txt
```

It needs nothing but a shell, because by then the program that would have read
a manifest is gone. It reports two different failures: a path that should have
been deleted and is still there, and a library outside the roots that a purge
was supposed to keep and that is missing.

The first two forms redact your home directory to `~`, like `paths` and
`config show`. `--paths` does not, because a checker compares real paths.

## Ways to consume Notrios

- **Run from source** (`go run ./cmd/...`) — best for development; nothing to install, always current.
- **Local binaries** (`make build`, `make gui`) — fast startup, same behavior; outputs under `bin/`.
- **Source release ZIP** (`bash scripts/package_release.sh`, output `dist/notrios-src.zip`) — a reviewed source snapshot *including* prebuilt `web/dist/` assets, intended for archiving or importing into another repository host. It is **not** a binary distribution: recipients still build with Go on their machine.
- **The release package** — the Ubuntu 24.04 amd64 `.deb` a release publishes, built and attested by the release workflow and signed; see [installing from the package](#installing-from-the-package). It is the only prebuilt binary, and there is no installer for any other operating system.

## Next steps

- [Configure and run the service](service.md)
- [Use the desktop GUI](gui.md)
- [Import your notes](import-export.md) (Joplin, Obsidian, Twitter/X, ChatGPT, Claude)
- [Troubleshooting](troubleshooting.md)
