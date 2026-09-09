package version

import "testing"

// The product version is asserted rather than merely defined, because it is
// read by the REST status route, the MCP handshake, the About dialog, the CLI
// and every snapshot image: a version that drifts from what the release says it
// is drifts in six places at once.
//
// 0.7.0 -> 0.8.0 in v0.8 H13. The schema stays at 27: no v0.8 item added a
// migration, and incrementing one for packaging would claim a data change that
// did not happen.
func TestVersionIsReleaseCandidate(t *testing.T) {
	if Version != "0.8.0" {
		t.Fatalf("Version = %q, want 0.8.0", Version)
	}
}
