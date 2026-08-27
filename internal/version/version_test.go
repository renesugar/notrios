package version

import "testing"

func TestVersionIsReleaseCandidate(t *testing.T) {
	if Version != "0.6.0" {
		t.Fatalf("Version = %q, want 0.6.0", Version)
	}
}
