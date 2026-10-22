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
//
// 0.8.0 -> 1.0.0 in v1.0 J9, before the release rather than at it, so that the
// release documentation is executed against the version it describes. The
// schema is 28 since J18; that change is J18's, not this one's. Nothing compares
// this string -- it is a label in six places, and J9 checked each reader.
func TestVersionIsTheRelease(t *testing.T) {
	if Version != "1.0.0" {
		t.Fatalf("Version = %q, want 1.0.0", Version)
	}
}
