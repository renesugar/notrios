# G14c findings

The selected representation is now a production local-filesystem capability.
SQLite Online Backup produces a standalone sanitized image; deterministic
uncompressed USTAR packs close at 256 MiB payload or 65,536 entries, with a
single larger object explicitly marked oversized. Pack files publish through
private partials, fsync, a complete verification pass, and atomic rename. The
manifest is the sole completion marker and publishes last. A rerun reuses only
complete packs that still match the copied image and their content hashes; the
SQLite image always restarts.

Admission verifies strict JSON/capabilities, exact schema 25/application
bounds, manifest/database/pack hashes and lengths, SQLite integrity, database
identity, vector/floors, sanitized tables, deterministic tar headers, safe
content-addressed paths, every object byte, and exact database-to-pack
completeness. It produces install-ready staging only. Emergency backup,
atomic replacement, replica rotation, derived rebuild, and post-vector replay
remain G14d.

The generated 100k run created a 61,181,952-byte image in 18.212 seconds and
verified it in 8.231 seconds. Heap allocation at report time was 1,458,288
bytes and process peak RSS was 23,547,904 bytes, passing the 256 MiB generated
memory gate. The full-corpus G14b evidence remains the external-object scale
evidence; no private corpus was rerun or committed for G14c.

The reader matrix keeps loose and packed semantic archive-v2 behavior intact,
and neither reader silently interprets the other representation. No schema,
archive-v2 format, compressor, dependency, REST/MCP path, or catch-up cutover
changed.
