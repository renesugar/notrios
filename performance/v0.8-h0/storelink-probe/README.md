# H0 real-store shared-link probe

This disposable probe complements the transport-independent ABI lifecycle
harness. It imports the real Notrios store, bootstraps schema v27 through a Go
`c-shared` library, reports the linked SQLite version without returning a Go
pointer, and checks that the library neither depends on a second dynamic
SQLite nor exports the embedded engine's symbols.

Run it only with the H0 checksum-pinned SQLite `pkg-config` boundary:

```sh
PKG_CONFIG_PATH=/tmp/notrios-h0-control \
  GOCACHE=/tmp/notrios-h0-storelink-cache \
  ./performance/v0.8-h0/storelink-probe/run.sh
```

This is not the ABI-major-1 implementation. H1 must connect the selected
application facade to the separately validated 12-symbol lifecycle contract.
