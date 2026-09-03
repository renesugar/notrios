package service

import (
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/paths"
	"github.com/renesugar/notrios/internal/synckeys"
)

// resolveCredentialStore answers which store this profile uses, from the same
// inputs wherever it is asked. The two facts it gathers -- whether this is an
// installed profile and whether plaintext key material already exists -- are
// deliberately gathered here rather than passed in, so that the service, the
// CLI and doctor cannot disagree about them.
func resolveCredentialStore(cfg config.Config, keyFilePath string) config.CredentialStoreResolution {
	installed := false
	if resolution, err := paths.ForProcess(nil); err == nil {
		// A portable or explicitly-addressed installation is an installation.
		// Only a source checkout is not.
		installed = resolution.Mode != paths.ModeSource
	}
	return config.ResolveCredentialStore(trimmedCredentialStore(cfg), installed,
		synckeys.HasPlaintextMaterial(keyFilePath), synckeys.HasSealedMaterial(keyFilePath))
}

func trimmedCredentialStore(cfg config.Config) string {
	return strings.ToLower(strings.TrimSpace(cfg.Sync.REST.CredentialStore))
}
