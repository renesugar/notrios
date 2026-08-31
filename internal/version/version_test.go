package version

import "testing"

func TestVersionIsReleaseCandidate(t *testing.T) {
	if Version != "0.7.0" {
		t.Fatalf("Version = %q, want 0.7.0", Version)
	}
}
