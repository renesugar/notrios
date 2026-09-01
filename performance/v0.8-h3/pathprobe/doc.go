// Package pathprobe is H3 evidence, not product code.
//
// H3 is an investigation: it changes no path default. Its conclusions are
// therefore only worth what its observations are worth, and a report that
// merely reads the source can be confidently wrong about what the source does.
// Every claim in ../REPORT.md that says "observed" is produced by a test here,
// so a reader can rerun it and a later change that invalidates a claim breaks a
// test rather than quietly outdating a document.
//
// These tests characterize behavior that H3 recommends changing. They are
// deliberately written to fail once H4 lands: a characterization test whose
// subject has been fixed should be deleted or inverted along with the fix, and
// the failure is the reminder to do it. Each such test names the H4 change that
// should break it.
//
// The package holds only _test.go files, so it contributes nothing to a build.
package pathprobe
