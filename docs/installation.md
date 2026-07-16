# Installation and building

Notrios is currently **source-only**: there are no official prebuilt binaries, OS packages, or installers yet. You build it from a clone of the repository. The only tested platform is **Ubuntu Linux** (development and CI both run on Ubuntu; the GUI compile is checked on `ubuntu-latest`). Other Linux distributions with the same libraries will likely work but are untested; macOS and Windows are not currently built or tested, and no support is claimed for them.

## Prerequisites

| Requirement | Version | Needed for |
|---|---|---|
| Go | 1.25 or newer (`go.mod` says `go 1.25.0`; CI uses 1.25) | everything |
| C toolchain + `pkg-config` | Ubuntu `build-essential`, `pkg-config` | the SQLite store is a cgo wrapper over the system `libsqlite3` |
| `libsqlite3-dev` | Ubuntu package | service, CLI, GUI, tests |
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

All build targets live in the `Makefile` (`make help` lists them). Output paths are git-ignored.

| Command | Builds | Output |
|---|---|---|
| `make build` | headless service + CLI | `bin/notriosd`, `bin/notriosctl` |
| `make web` | production web assets (installs `web/node_modules` from the lockfile on first run) | `web/dist/` |
| `make gui` | desktop GUI binary (implies `make web`) | `bin/notrios` |
| `make docs` | documentation site with PageFind search (uses `npx`) | `_site/` |
| `make test` | all Go tests | — |
| `make clean` | removes all of the above outputs (never `data/`) | — |

Frontend dependencies install once into `web/node_modules` via `npm ci` (the lockfile-exact install); rerun `make deps` after pulling lockfile changes, and `make clobber` to remove them.

## Run

From the repository root:

```sh
# Headless service (REST + MCP + web UI at http://127.0.0.1:8080)
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

Two working-directory rules to know (both are current implementation behavior):

1. **The browser UI is loaded from `web/dist/` relative to the working directory.** Run `notriosd` (or the GUI) from the repository root after `make web`, or copy `web/dist/` into whatever directory you run from. If it is missing, `/` returns a `web_ui_not_built` error while the REST API, MCP endpoint, and importers keep working normally.
2. **Relative paths in the configuration resolve against the working directory.** The example config uses `./data/...`, so the database and asset store appear under wherever you launched the binary. Use absolute paths in your config file for anything you run outside the checkout. See the [service guide](service.md#configuration).

## Optional local installation

If you want the binaries on your `PATH`:

```sh
make build gui
install -m 0755 bin/notriosd bin/notriosctl bin/notrios ~/.local/bin/
```

Then run them with an explicit config that uses absolute paths, e.g. `notriosd -config ~/.config/notrios/config.yaml`. Because of working-directory rule 1 above, an installed `notriosd`/`notrios` only serves the browser/GUI interface when started from a directory containing `web/dist/` — the simplest arrangement today is to keep launching from the checkout.

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
