# Installation and building

Notrios is currently **source-only**: there are no official prebuilt binaries, OS packages, or installers yet. You build it from a clone of the repository. The only tested platform is **Ubuntu Linux** (development and CI both run on Ubuntu; the GUI compile is checked on `ubuntu-latest`). Other Linux distributions with the same libraries will likely work but are untested; macOS and Windows are not currently built or tested, and no support is claimed for them.

## Prerequisites

| Requirement | Version | Needed for |
|---|---|---|
| Go | 1.25 or newer (`go.mod` says `go 1.25.0`; CI uses 1.25) | everything |
| C toolchain | Ubuntu `build-essential` | the SQLite store is a cgo wrapper over the vendored SQLite amalgamation, compiled from source into the binary |
| Node.js + npm | Node 22 (CI-tested); Node ≥ 20.19 may work | building the web UI and the documentation site |
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

## Optional local installation

If you want the binaries on your `PATH`:

```sh
make build gui
install -m 0755 bin/notriosd bin/notriosctl bin/notrios ~/.local/bin/
```

Copy the built interface somewhere the binaries will find it — the program-assets root, or beside the executable:

```sh
sudo mkdir -p /usr/local/share/notrios
sudo cp -r web/dist /usr/local/share/notrios/web
```

Without that step an installed binary has no interface to serve unless you pass `--web-dir`; the headless service still runs, and REST, MCP and the importers do not need one. Run `notriosctl paths` to see which roots the installed copy resolved.

Uninstall by deleting the copies:

```sh
rm -f ~/.local/bin/notriosd ~/.local/bin/notriosctl ~/.local/bin/notrios
```

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
