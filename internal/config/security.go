package config

import (
	"fmt"
	"strings"

	"github.com/renesugar/notrios/internal/addressrange"
)

// SecurityConfig is the explicit security block (J28). Each subsection names
// the surface it governs; nothing here applies to any other surface.
type SecurityConfig struct {
	RemoteMedia RemoteMediaSecurityConfig `json:"remote_media"`
}

// RemoteMediaSecurityConfig states which addresses remote-media fetches may
// not reach. It governs remote media only: sync dials a peer the owner
// configured, and is not subject to it. A nil RefusedAddressRanges means the
// file did not state one, which means the default set.
type RemoteMediaSecurityConfig struct {
	// RefusedAddressRanges are the address ranges remote media may not reach,
	// as CIDR ranges or addresses. Left out, the default set in
	// SECURITY_AND_MEDIA_POLICY.md applies; stated, this list replaces it, []
	// refuses nothing, and service start warns about each default range it
	// omits.
	RefusedAddressRanges []string `json:"refused_address_ranges"`
	// PermittedAddressRanges are exceptions to the refused ranges, empty by
	// default. An exception also covers its IPv4-mapped and NAT64 forms, and
	// none may include loopback or 0.0.0.0/8, which only
	// remote_media.allow_private_networks reaches.
	PermittedAddressRanges []string `json:"permitted_address_ranges"`
}

// Configuration keys of the block, as written in a file and as origins.
const (
	RefusedAddressRangesKey   = "security.remote_media.refused_address_ranges"
	PermittedAddressRangesKey = "security.remote_media.permitted_address_ranges"
)

// Rules parses the block. The loader has already refused a malformed block,
// so an error here means a Config built in code.
func (c RemoteMediaSecurityConfig) Rules() (*addressrange.Rules, error) {
	return addressrange.Parse(c.RefusedAddressRanges, c.PermittedAddressRanges)
}

// RefusedStated reports whether the refused set was stated rather than
// defaulted.
func (c RemoteMediaSecurityConfig) RefusedStated() bool {
	return c.RefusedAddressRanges != nil
}

// RemoteMediaAddressWarnings are the sentences service start logs and
// `config show` prints about the address block: a switched-off set, the
// default ranges a stated set omits (D2), and the exceptions in force.
func (c Config) RemoteMediaAddressWarnings() []string {
	if c.RemoteMedia.AllowPrivateNetworks {
		return []string{"remote_media.allow_private_networks is true: the refused address ranges, their exceptions and the localhost-name check are not applied to remote media"}
	}
	rules, err := c.Security.RemoteMedia.Rules()
	if err != nil {
		return []string{fmt.Sprintf("the remote-media address ranges are invalid, so every remote-media URL is refused: %v", err)}
	}
	var warnings []string
	if c.Security.RemoteMedia.RefusedStated() {
		if omitted := rules.OmittedDefaults(); len(omitted) > 0 {
			warnings = append(warnings, fmt.Sprintf("%s omits %d default range(s), which remote media may now reach: %s",
				RefusedAddressRangesKey, len(omitted), strings.Join(omitted, ", ")))
		}
	}
	if permitted := rules.Permitted(); len(permitted) > 0 {
		warnings = append(warnings, fmt.Sprintf("%s lets remote media reach: %s",
			PermittedAddressRangesKey, strings.Join(permitted, ", ")))
	}
	return warnings
}

// securityBlock reads the security: section. The general loader reads lists
// only one level down; this block's lists are two levels down
// (security.remote_media.<list>), and a malformed entry must fail loading
// rather than be ignored (D3), so it has its own small reader.
type securityBlock struct {
	group     string    // the subsection at indent 2
	list      string    // the list key whose dash items follow, if any
	target    *[]string // where those items go
	listItems int
}

func (b *securityBlock) listTarget(cfg *Config, key string) *[]string {
	if b.group != "remote_media" {
		return nil
	}
	switch key {
	case "refused_address_ranges":
		return &cfg.Security.RemoteMedia.RefusedAddressRanges
	case "permitted_address_ranges":
		return &cfg.Security.RemoteMedia.PermittedAddressRanges
	}
	return nil
}

// line consumes one non-blank line inside the security section.
func (b *securityBlock) line(cfg *Config, indent int, trimmed string, provided map[string]bool) error {
	if strings.HasPrefix(trimmed, "-") {
		if b.target != nil {
			*b.target = append(*b.target, parseScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))))
			b.listItems++
		}
		return nil
	}
	if err := b.close(); err != nil {
		return err
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid line %q", trimmed)
	}
	key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if indent <= 2 {
		b.group = ""
		if value == "" {
			b.group = key
		}
		return nil
	}
	target := b.listTarget(cfg, key)
	if target == nil {
		return nil // unknown keys are ignored, as everywhere in the loader
	}
	name := "security." + b.group + "." + key
	if provided != nil {
		provided[name] = true
	}
	if value == "" {
		*target = []string{}
		b.list, b.target, b.listItems = name, target, 0
		return nil
	}
	items, ok := parseInlineList(value)
	if !ok {
		return fmt.Errorf("%s must be a list, such as [\"10.0.0.0/8\"] or [], not %q", name, value)
	}
	if items == nil {
		items = []string{}
	}
	*target = items
	return nil
}

// close ends a block-style list. A list key with no items and no value states
// nothing, and silently reading it as "empty" or as "default" would each be a
// guess about which the owner meant.
func (b *securityBlock) close() error {
	defer func() { b.list, b.target, b.listItems = "", nil, 0 }()
	if b.target != nil && b.listItems == 0 {
		return fmt.Errorf("%s has no value and no items; write [] to refuse nothing, or remove the key to use the default", b.list)
	}
	return nil
}
