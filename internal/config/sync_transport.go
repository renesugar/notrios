package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// ValidateSyncTransport decides whether the peer-authenticated sync surface may
// be served with this configuration.
//
// v0.7's whole security posture rests on one sentence: Notrios is a local
// application whose REST API has no general authentication. Enabling a sync
// surface is the first thing that can put any part of it on a network, so the
// checks here are refusals rather than warnings — a service that starts and
// then turns out to have been exposing plaintext is worse than one that does
// not start and says why.
func ValidateSyncTransport(cfg Config) error {
	if !cfg.Sync.REST.Enabled {
		return nil
	}
	certificate := strings.TrimSpace(cfg.Sync.REST.TLSCertFile)
	key := strings.TrimSpace(cfg.Sync.REST.TLSKeyFile)
	if (certificate == "") != (key == "") {
		return fmt.Errorf("sync.rest: a TLS certificate needs its key and vice versa")
	}
	if certificate != "" {
		for _, path := range []string{certificate, key} {
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("sync.rest: TLS material %s is not readable: %w", path, err)
			}
		}
		return nil
	}
	loopback, err := IsLoopbackListenAddr(cfg.Server.ListenAddr)
	if err != nil {
		return fmt.Errorf("sync.rest: %w", err)
	}
	if !loopback {
		return fmt.Errorf(
			"sync.rest: refusing to serve the sync surface on %s without TLS; "+
				"set sync.rest.tls_cert_file and sync.rest.tls_key_file, or bind to loopback",
			cfg.Server.ListenAddr)
	}
	if cfg.Sync.REST.RequireTLS {
		// Loopback plaintext is allowed even with require_tls set, because the
		// traffic never leaves the machine and this is how the surface is
		// tested. It is stated rather than silent.
		return nil
	}
	return nil
}

// IsLoopbackListenAddr reports whether a listen address binds only to the local
// machine. An empty host, or 0.0.0.0, binds everywhere and is not loopback —
// which is exactly the case an operator most often gets wrong.
func IsLoopbackListenAddr(listenAddr string) (bool, error) {
	host, _, err := net.SplitHostPort(strings.TrimSpace(listenAddr))
	if err != nil {
		return false, fmt.Errorf("unparsable listen address %q: %w", listenAddr, err)
	}
	if host == "" {
		return false, nil
	}
	if host == "localhost" {
		return true, nil
	}
	address := net.ParseIP(host)
	if address == nil {
		return false, nil
	}
	return address.IsLoopback(), nil
}
