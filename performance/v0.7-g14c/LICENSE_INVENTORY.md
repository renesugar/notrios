# G14c dependency and license inventory

G14c adds no module, compressor, native library, vendored source, or runtime
tool. The representation uses:

- the existing system SQLite library through the project's narrow cgo wrapper
  and SQLite Online Backup API;
- Go standard-library `archive/tar`, `crypto/sha256`, JSON, and filesystem APIs;
- existing Apache-2.0 Notrios packages.

`go list -m all` remains unchanged by this slice. Recoll is not read, linked,
vendored, or packed; its derived index remains outside the authoritative
snapshot.
