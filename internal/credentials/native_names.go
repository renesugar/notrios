package credentials

import "runtime"

// availabilityService is the service name used only by the read-only
// availability probe, kept apart from any profile's real service name so a
// probe can never collide with stored material.
const availabilityService = "notrios-probe"

// nativeStoreNameFor names the store a person would recognise, so that a
// refusal says "GNOME Keyring did not respond" rather than naming a Go
// package. The switch is on runtime.GOOS rather than build-tagged files
// because a filename suffix of _linux.go also matches an Android build, and
// this package has one rule about not compiling into mobile targets.
func nativeStoreNameFor(goos string) string {
	switch goos {
	case "windows":
		return "Windows Credential Manager"
	case "darwin":
		return "macOS Keychain"
	default:
		return "Secret Service keyring"
	}
}

var nativeStoreName = nativeStoreNameFor(runtime.GOOS)
