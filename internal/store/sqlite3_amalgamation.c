/*
** Translation unit for the vendored SQLite amalgamation.
**
** cgo compiles the C files that sit in a package directory, but not those in
** a subdirectory. Including the amalgamation from here lets the unmodified
** upstream sources stay in csqlite/, where their provenance is checkable and
** nothing in review has to scroll past 9.5 MB of third-party code.
**
** Compile options live in sqlite_cgo.go so there is exactly one place that
** decides how this library is built. See csqlite/PROVENANCE.json.
*/

#include "csqlite/sqlite3.c"
