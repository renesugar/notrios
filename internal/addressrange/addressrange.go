// Package addressrange decides whether remote media may reach an IP address
// (J28). It holds the default refusal set named in
// SECURITY_AND_MEDIA_POLICY.md, parses the security.remote_media
// configuration block, and applies one rule at both places an address is
// checked: the static URL check and the connect-time dial check.
//
// It is a leaf package so the configuration loader can reject malformed entries
// at load time; internal/media imports internal/config and cannot be imported
// by it.
package addressrange

import (
	"fmt"
	"net/netip"
	"strings"
)

// RegistryCheckedOn is the date the default set was checked against IANA's
// IPv4 and IPv6 Special-Purpose Address Registries (both last updated
// 2025-10-09). Notrios never fetches the registries at run time.
const RegistryCheckedOn = "2026-09-16"

// Default is the refusal set used when the configuration states none. Blocks
// the registries mark globally reachable are deliberately absent (PCP and TURN
// anycast outside 192.0.0.0/24, AMT, AS112, ORCHIDv2, drone remote ID).
//
// The IPv4-mapped (::ffff:0:0/96), IPv4-compatible (::/96) and NAT64
// (64:ff9b::/96) prefixes are not entries: an entry refuses its whole range,
// and D4 decided their embedded IPv4 address is checked instead.
var Default = []string{
	// IPv4
	"0.0.0.0/8",       // "this network"; 0.0.0.0 reaches the local host (RFC 791, RFC 1122)
	"10.0.0.0/8",      // private use (RFC 1918)
	"100.64.0.0/10",   // shared address space, carrier-grade NAT (RFC 6598)
	"127.0.0.0/8",     // loopback (RFC 1122)
	"169.254.0.0/16",  // link-local, including cloud metadata (RFC 3927)
	"172.16.0.0/12",   // private use (RFC 1918)
	"192.0.0.0/24",    // IETF protocol assignments (RFC 6890)
	"192.0.2.0/24",    // documentation, TEST-NET-1 (RFC 5737)
	"192.88.99.0/24",  // deprecated 6to4 relay anycast (RFC 7526)
	"192.168.0.0/16",  // private use (RFC 1918)
	"198.18.0.0/15",   // benchmarking (RFC 2544)
	"198.51.100.0/24", // documentation, TEST-NET-2 (RFC 5737)
	"203.0.113.0/24",  // documentation, TEST-NET-3 (RFC 5737)
	"224.0.0.0/4",     // multicast (RFC 5771)
	"240.0.0.0/4",     // reserved, including 255.255.255.255 (RFC 1112, RFC 919)
	// IPv6
	"::/128",         // unspecified (RFC 4291)
	"::1/128",        // loopback (RFC 4291)
	"64:ff9b:1::/48", // local-use NAT64 (RFC 8215)
	"100::/64",       // discard-only (RFC 6666)
	"100:0:0:1::/64", // dummy prefix (RFC 9780)
	"2001::/32",      // Teredo (RFC 4380)
	"2001:2::/48",    // benchmarking (RFC 5180)
	"2001:10::/28",   // deprecated ORCHID (RFC 4843)
	"2001:db8::/32",  // documentation (RFC 3849)
	"2002::/16",      // 6to4 (RFC 3056)
	"3fff::/20",      // documentation (RFC 9637)
	"5f00::/16",      // SRv6 SIDs (RFC 9602)
	"fc00::/7",       // unique local, including cloud metadata (RFC 4193)
	"fe80::/10",      // link-local (RFC 4291)
	"fec0::/10",      // deprecated site-local (RFC 3879)
	"ff00::/8",       // multicast (RFC 4291)
}

// neverPermitted are the addresses no exception may reach (D8): loopback and
// "this host". A loopback exception would make the service fetch from its own
// REST, MCP and sync endpoints on a URL a note supplied. Reaching them needs
// remote_media.allow_private_networks.
var neverPermitted = mustPrefixes("127.0.0.0/8", "0.0.0.0/8", "::1/128", "::/128")

// embeddings are the IPv6 prefixes whose last 32 bits carry an IPv4 address
// that is checked against the IPv4 entries (D4).
var embeddings = mustPrefixes("::ffff:0:0/96", "::/96", "64:ff9b::/96")

// neverPermittedEmbedded is neverPermitted's IPv4 half written in every
// embedding, so an IPv6 exception cannot reach loopback through one.
var neverPermittedEmbedded = func() []netip.Prefix {
	var out []netip.Prefix
	for _, embedding := range embeddings {
		base := embedding.Addr().As16()
		for _, guard := range neverPermitted {
			if !guard.Addr().Is4() {
				continue
			}
			v4 := guard.Addr().As4()
			bytes := base
			copy(bytes[12:], v4[:])
			out = append(out, netip.PrefixFrom(netip.AddrFrom16(bytes), 96+guard.Bits()))
		}
	}
	return out
}()

// Rules is a parsed refusal set and its exceptions.
type Rules struct {
	refused   []netip.Prefix
	permitted []netip.Prefix
}

// Decision is the verdict for one address.
type Decision struct {
	Refused bool
	// Range is the refused entry the address matched, in canonical form, when
	// it matched one: set whether or not an exception then permitted it.
	Range string
	// Permitted is the exception that permitted the address, if one did.
	Permitted string
}

// Parse builds Rules. refused nil means the default set; a non-nil empty slice
// refuses nothing. Every entry must be an address or a CIDR prefix with no host
// bits set and no zone, and no exception may overlap loopback or "this host" in
// any form. The error names the list and the entry.
func Parse(refused, permitted []string) (*Rules, error) {
	if refused == nil {
		refused = Default
	}
	rules := &Rules{}
	var err error
	if rules.refused, err = parseList(refusedKey, refused); err != nil {
		return nil, err
	}
	if rules.permitted, err = parseList(permittedKey, permitted); err != nil {
		return nil, err
	}
	for i, prefix := range rules.permitted {
		for _, guard := range append(append([]netip.Prefix{}, neverPermitted...), neverPermittedEmbedded...) {
			if prefix.Overlaps(guard) {
				return nil, fmt.Errorf("%s: %q overlaps %s, which no exception may permit (loopback and \"this host\" need remote_media.allow_private_networks)",
					permittedKey, permitted[i], guard)
			}
		}
	}
	return rules, nil
}

// MustDefault returns the default Rules. The default set is a compiled
// constant, so a failure is a programming error.
func MustDefault() *Rules {
	rules, err := Parse(nil, nil)
	if err != nil {
		panic(err)
	}
	return rules
}

const (
	refusedKey   = "security.remote_media.refused_address_ranges"
	permittedKey = "security.remote_media.permitted_address_ranges"
)

func parseList(key string, entries []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		prefix, err := ParseEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out = append(out, prefix)
	}
	return out, nil
}

// ParseEntry parses one configured entry: an address (its /32 or /128) or a
// CIDR prefix. Host bits, zones and anything else are refused rather than
// corrected, because silently widening an exception widens what is fetched.
func ParseEntry(entry string) (netip.Prefix, error) {
	text := strings.TrimSpace(entry)
	if text == "" {
		return netip.Prefix{}, fmt.Errorf("%q is empty; remove it", entry)
	}
	if strings.Contains(text, "%") {
		return netip.Prefix{}, fmt.Errorf("%q has an IPv6 zone, which an address range cannot carry", entry)
	}
	if !strings.Contains(text, "/") {
		addr, err := netip.ParseAddr(text)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("%q is not a valid address or CIDR range", entry)
		}
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%q is not a valid address or CIDR range", entry)
	}
	if prefix != prefix.Masked() {
		return netip.Prefix{}, fmt.Errorf("%q has host bits set; the range it names is %s", entry, prefix.Masked())
	}
	return prefix, nil
}

// Embedded returns the IPv4 address carried by an IPv4-mapped, IPv4-compatible
// or NAT64 well-known-prefix IPv6 address (D4).
func Embedded(addr netip.Addr) (netip.Addr, bool) {
	addr = addr.WithZone("")
	if !addr.Is6() {
		return netip.Addr{}, false
	}
	for _, embedding := range embeddings {
		if embedding.Contains(addr) {
			bytes := addr.As16()
			return netip.AddrFrom4([4]byte{bytes[12], bytes[13], bytes[14], bytes[15]}), true
		}
	}
	return netip.Addr{}, false
}

// Check decides one address. An address is refused when it, or the IPv4
// address it carries, is in a refused entry, unless an exception covers it or
// its carried IPv4 address (D9). No exception permits loopback or "this host"
// in any form (D8).
func (r *Rules) Check(addr netip.Addr) Decision {
	addr = addr.WithZone("")
	forms := []netip.Addr{addr}
	if embedded, ok := Embedded(addr); ok {
		forms = append(forms, embedded)
	}
	matched := ""
	for _, form := range forms {
		if prefix, ok := firstContaining(r.refused, form); ok {
			matched = prefix.String()
			break
		}
	}
	if matched == "" {
		return Decision{}
	}
	for _, form := range forms {
		if _, ok := firstContaining(neverPermitted, form); ok {
			return Decision{Refused: true, Range: matched}
		}
	}
	for _, form := range forms {
		if prefix, ok := firstContaining(r.permitted, form); ok {
			return Decision{Range: matched, Permitted: prefix.String()}
		}
	}
	return Decision{Refused: true, Range: matched}
}

func firstContaining(prefixes []netip.Prefix, addr netip.Addr) (netip.Prefix, bool) {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return prefix, true
		}
	}
	return netip.Prefix{}, false
}

// Refused returns the refused entries in canonical form.
func (r *Rules) Refused() []string { return prefixStrings(r.refused) }

// Permitted returns the exceptions in canonical form.
func (r *Rules) Permitted() []string { return prefixStrings(r.permitted) }

// OmittedDefaults returns, in order, every default range that no refused entry
// fully covers: what a stated set lets remote media reach that the default
// would not (D2).
func (r *Rules) OmittedDefaults() []string {
	omitted := []string{}
	for _, def := range mustPrefixes(Default...) {
		covered := false
		for _, prefix := range r.refused {
			if prefix.Addr().BitLen() == def.Addr().BitLen() && prefix.Bits() <= def.Bits() && prefix.Contains(def.Addr()) {
				covered = true
				break
			}
		}
		if !covered {
			omitted = append(omitted, def.String())
		}
	}
	return omitted
}

func prefixStrings(prefixes []netip.Prefix) []string {
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		out = append(out, prefix.String())
	}
	return out
}

func mustPrefixes(entries ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		out = append(out, netip.MustParsePrefix(entry))
	}
	return out
}
