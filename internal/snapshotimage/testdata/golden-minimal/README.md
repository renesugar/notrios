# Independent physical snapshot fixture inputs

`object-a.txt`, `object-b.txt`, and `fixture.json` are reader-fixture inputs,
not output copied from the production snapshot writer. `golden_test.go` builds
a schema-v25 SQLite image with the store bootstrap, encodes its USTAR pack with
the standard-library tar writer, and assembles the manifest independently. The
production `Create` path is never called. This catches writer/reader agreement
bugs while keeping a generated SQLite binary out of the repository.
