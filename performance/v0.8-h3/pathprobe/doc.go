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
// Retired by H4 slice C, which made every mutable root owner-only:
//
//   - TestPrimaryDataRootsAreCreatedWorldReadable
//   - TestTheDataDirectoryGetsDifferentModesDependingOnWhoCreatesIt
//
// config.EnsureDirectories creates 0700, and internal/store creates the
// database's parent and the asset store 0700, so they now agree with
// publish.File.Save rather than racing it. The inverted assertions live in
// internal/config as TestEveryMutableRootIsCreatedOwnerOnly.
// Retired by H4 slice B, which routed both consumers through internal/paths:
//
//   - TestRelativeXDGConfigHomeIsAcceptedByProfilesAndRefusedByTheStandardLibrary
//   - TestProfileRegistryFallsBackToACurrentDirectoryRelativePath
//
// Both characterized the two config-root resolvers disagreeing. There is one
// resolver now, so there is nothing left to disagree. The behaviour they
// asserted is inverted and kept as regression tests in internal/profiles:
// TestDefaultPathIgnoresARelativeConfigHome and
// TestDefaultPathRefusesRatherThanInventingAPath, plus
// TestProfilesAndSyncKeysAgreeOnTheConfigRoot.
//
// Retired by H4 slice C:
//
//   - TestGeneratedProfileDataIsPlacedUnderTheConfigRoot
//
// A generated profile's database, assets, projections, index and quarantine no
// longer default inside ~/.config. The inverse is
// TestGeneratedProfileDataGoesUnderTheDataRoot in internal/profiles.
// Retired by H4 slice B:
//
//   - TestDefaultConfigurationIsReadFromTheCurrentWorkingDirectory
//
// config.LoadDefaultOrExample became config.LoadDefault, which reads a
// checkout's example only in source mode. Its inverses live in internal/config.
//
// Retired by H4 slice D:
//
//   - TestWebRootPrefersTheWorkingDirectoryOverTheExecutable
//   - TestExplicitWebRootIsTheOnlyCandidate
//
// The installed program-assets root now precedes the executable's tree, and the
// working directory is consulted only in a checkout. The inverses live in
// internal/httpapi as TestResolveWebRootUsesTheWorkingDirectoryInACheckout and
// TestResolveWebRootIgnoresTheWorkingDirectoryWhenInstalled, beside
// TestResolveWebRootExplicitPathDoesNotFallThrough which already covered the
// explicit case.
//
// The package holds only _test.go files, so it contributes nothing to a build.
package pathprobe
