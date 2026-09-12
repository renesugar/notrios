// Package purge decides whether Notrios may delete a path, and then does it.
//
// The decision procedure is a port of H3's reference oracle
// (performance/v0.8-h3/purge_oracle.py), which was written first precisely so
// the rules could be argued about before any code could act on them. Porting it
// rather than calling it is a deliberate trade: the packaged application ships
// no Python, and a user who installed the .deb had no way to reach these
// safeguards at all.
//
// Two implementations of a deletion rule is the worst possible duplication, so
// they are held together by the fixtures H3 already generated: oracle_test.go
// drives the same thirty cases and requires the same verdict and rule for each,
// reading the expected answers out of PURGE_ORACLE_FIXTURES.json. A rule that
// changes on one side without the other fails the build.
//
// The rules are closed by default. A target is refused unless it is positively
// identified as a path under a root Notrios owns; unknown, ambiguous, outside,
// unreadable, crossing a mount, or reached through a symlink all refuse. That
// asymmetry is the design: a wrongly refused deletion costs the user a manual
// `rm`, and a wrongly allowed one costs them their notes.
package purge

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Verdicts.
const (
	Allow       = "ALLOW"        // delete it
	AllowAbsent = "ALLOW_ABSENT" // nothing there; a repeated purge is a no-op, not an error
	Refuse      = "REFUSE"       // do not delete; the reason names what to do instead
)

// forbiddenRoots are paths no purge may ever name, whatever the configuration
// says. Kept in the same order as the reference so the two read alike.
var forbiddenRoots = []string{
	"/", "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib32", "/lib64",
	"/media", "/mnt", "/opt", "/proc", "/root", "/run", "/sbin", "/srv",
	"/sys", "/tmp", "/usr", "/var",
}

// Environment is what the oracle may consider Notrios's own territory.
type Environment struct {
	OwnedRoots []string
	Home       string
	// ExternalProfilePaths are paths named inside profiles that live outside
	// every owned root. They are enumerated and backed up, never deleted
	// automatically -- H3's recommended default.
	ExternalProfilePaths []string
	// DeviceOf is injectable so the mount-boundary rule can be exercised
	// without mounting a filesystem, which a test cannot do.
	DeviceOf func(path string) (uint64, error)
}

// Decision is a verdict, why it was reached, and which rule reached it.
type Decision struct {
	Verdict string
	Reason  string
	Rule    string
}

func (d Decision) String() string {
	return fmt.Sprintf("%s: %s [%s]", d.Verdict, d.Reason, d.Rule)
}

func isWithin(child, parent string) bool {
	parent = strings.TrimRight(parent, "/")
	return child == parent || strings.HasPrefix(child, parent+"/")
}

// Decide answers whether path may be deleted.
func Decide(path string, env Environment) Decision {
	raw := path
	if strings.TrimSpace(path) == "" {
		return Decision{Refuse, "the purge target is empty", "non-empty"}
	}
	path = strings.TrimSpace(path)

	if !filepath.IsAbs(path) {
		return Decision{Refuse,
			fmt.Sprintf("%q is relative; a purge target must be absolute", raw), "absolute"}
	}

	// Clean collapses "a/../b" textually, which is exactly what must NOT decide
	// containment -- it disagrees with the kernel when a component is a symlink.
	// It is used only to reject obvious nonsense; containment is decided on the
	// resolved path below.
	normalized := filepath.Clean(path)
	if normalized != strings.TrimRight(path, "/") && normalized != path {
		return Decision{Refuse,
			fmt.Sprintf("%q is not in normal form; state the path plainly rather than through '..' or '.'", raw),
			"normal-form"}
	}

	for _, forbidden := range forbiddenRoots {
		if normalized == forbidden {
			return Decision{Refuse,
				fmt.Sprintf("%s is a system root and is never a purge target", normalized),
				"system-root"}
		}
	}

	if env.Home != "" {
		home := filepath.Clean(env.Home)
		if normalized == home {
			return Decision{Refuse, "the purge target is the home directory itself", "home"}
		}
		if isWithin(home, normalized) {
			return Decision{Refuse,
				fmt.Sprintf("%s contains the home directory %s", normalized, home), "home-ancestor"}
		}
	}

	for _, external := range env.ExternalProfilePaths {
		externalN := filepath.Clean(external)
		// Both directions: a rule that only looked downward would delete an
		// external profile as collateral when its parent was named.
		if normalized == externalN || isWithin(normalized, externalN) || isWithin(externalN, normalized) {
			return Decision{Refuse,
				fmt.Sprintf("%s is or contains the external profile path %s; "+
					"it is enumerated and backed up, never deleted automatically", normalized, externalN),
				"external-profile-path"}
		}
	}

	if len(env.OwnedRoots) == 0 {
		return Decision{Refuse,
			"no owned roots were declared, so nothing can be identified as ours", "owned-root"}
	}

	resolved := realpath(normalized)
	owner := ""
	for _, root := range env.OwnedRoots {
		resolvedRoot := realpath(filepath.Clean(root))
		if isWithin(resolved, resolvedRoot) {
			owner = resolvedRoot
			break
		}
	}
	if owner == "" {
		return Decision{Refuse,
			fmt.Sprintf("%s resolves to %s, which is outside every root Notrios owns", normalized, resolved),
			"owned-root"}
	}

	if resolved == owner && normalized != owner {
		return Decision{Refuse,
			fmt.Sprintf("%s resolves onto the owned root %s itself rather than a path within it",
				normalized, owner),
			"root-aliasing"}
	}

	if !lexists(normalized) {
		return Decision{AllowAbsent,
			fmt.Sprintf("%s does not exist; nothing to remove", normalized), "absent"}
	}

	if isSymlink(normalized) {
		return Decision{Refuse,
			fmt.Sprintf("%s is a symlink; remove the link deliberately rather than following it", normalized),
			"symlink"}
	}

	deviceOf := env.DeviceOf
	if deviceOf == nil {
		deviceOf = deviceOfPath
	}
	targetDevice, err := deviceOf(normalized)
	if err != nil {
		return Decision{Refuse, fmt.Sprintf("%s could not be inspected: %v", normalized, err), "unreadable"}
	}
	ownerDevice, err := deviceOf(owner)
	if err != nil {
		return Decision{Refuse, fmt.Sprintf("%s could not be inspected: %v", normalized, err), "unreadable"}
	}
	if targetDevice != ownerDevice {
		return Decision{Refuse,
			fmt.Sprintf("%s is on a different filesystem from its root %s; "+
				"a purge does not cross a mount boundary", normalized, owner),
			"mount-boundary"}
	}

	return Decision{Allow, fmt.Sprintf("%s is inside %s and safe to remove", normalized, owner), "owned"}
}

// BackupPolicy says what purge must do with a category before removing it.
//
// Deciding whether a path may be deleted is half the contract; the other half
// is what must happen first. Categories holding something the user cannot
// reproduce are backed up and verified; categories derived from them are
// disposed of, because backing up a rebuildable index makes every purge slower
// and the backup larger while protecting nothing.
//
// quarantine is the interesting case and lives under state: it holds untrusted
// downloaded bytes, which sounds disposable, but it is also the evidence of what
// a note tried to fetch and may be the only copy of media a user approved and
// has not yet localized.
func BackupPolicy(category string) string {
	switch category {
	case "config", "data", "state":
		return "backup_and_verify"
	case "cache", "runtime":
		return "dispose"
	case "program_assets":
		return "uninstall_manifest_only"
	case "external":
		return "backup_never_delete"
	default:
		// An unclassified category is treated as irreplaceable. A new root
		// added without a policy must not be silently disposed of.
		return "backup_and_verify"
	}
}
