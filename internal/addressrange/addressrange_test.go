package addressrange

import (
	"net/netip"
	"strings"
	"testing"
)

// oneAddressPerDefaultRange is one address inside every default entry, with the
// entry it must be refused by.
var oneAddressPerDefaultRange = map[string]string{
	"0.1.2.3":                              "0.0.0.0/8",
	"0.0.0.0":                              "0.0.0.0/8",
	"10.1.2.3":                             "10.0.0.0/8",
	"100.64.0.1":                           "100.64.0.0/10",
	"127.0.0.1":                            "127.0.0.0/8",
	"169.254.169.254":                      "169.254.0.0/16",
	"172.16.0.1":                           "172.16.0.0/12",
	"192.0.0.1":                            "192.0.0.0/24",
	"192.0.2.1":                            "192.0.2.0/24",
	"192.88.99.1":                          "192.88.99.0/24",
	"192.168.1.1":                          "192.168.0.0/16",
	"198.18.0.1":                           "198.18.0.0/15",
	"198.51.100.1":                         "198.51.100.0/24",
	"203.0.113.1":                          "203.0.113.0/24",
	"224.0.0.2":                            "224.0.0.0/4",
	"239.1.1.1":                            "224.0.0.0/4",
	"233.252.0.1":                          "224.0.0.0/4",
	"240.0.0.1":                            "240.0.0.0/4",
	"255.255.255.255":                      "240.0.0.0/4",
	"::":                                   "::/128",
	"::1":                                  "::1/128",
	"64:ff9b:1::a00:1":                     "64:ff9b:1::/48",
	"100::1":                               "100::/64",
	"100:0:0:1::1":                         "100:0:0:1::/64",
	"2001:0:4136:e378:8000:63bf:80ff:fffe": "2001::/32",
	"2001:2::1":                            "2001:2::/48",
	"2001:10::1":                           "2001:10::/28",
	"2001:db8::1":                          "2001:db8::/32",
	"2002:7f00:1::1":                       "2002::/16",
	"3fff::1":                              "3fff::/20",
	"5f00::1":                              "5f00::/16",
	"fc00::1":                              "fc00::/7",
	"fd00:ec2::254":                        "fc00::/7",
	"fe80::1":                              "fe80::/10",
	"fec0::1":                              "fec0::/10",
	"ff02::1":                              "ff00::/8",
	"ff05::1":                              "ff00::/8",
	"ff0e::1":                              "ff00::/8",
}

func TestDefaultRefusesOneAddressInEveryRange(t *testing.T) {
	rules := MustDefault()
	covered := map[string]bool{}
	for text, want := range oneAddressPerDefaultRange {
		decision := rules.Check(netip.MustParseAddr(text))
		if !decision.Refused || decision.Range != want {
			t.Errorf("Check(%s) = %+v; want refused by %s", text, decision, want)
		}
		covered[want] = true
	}
	for _, entry := range Default {
		if !covered[entry] {
			t.Errorf("default entry %s has no case", entry)
		}
	}
	if len(Default) != 31 {
		t.Errorf("default set has %d entries; the policy names 15 IPv4 and 16 IPv6", len(Default))
	}
}

func TestBlocksLeftOutOfTheDefaultStayReachable(t *testing.T) {
	rules := MustDefault()
	for _, text := range []string{
		"8.8.8.8",          // public
		"2606:4700::1111",  // public
		"192.31.196.1",     // AS112-v4, globally reachable
		"192.52.193.1",     // AMT, globally reachable
		"192.175.48.1",     // AS112 direct delegation
		"2001:1::1",        // PCP anycast
		"2001:3::1",        // AMT
		"2001:4:112::1",    // AS112-v6
		"2001:20::1",       // ORCHIDv2, globally reachable
		"2001:30::1",       // drone remote ID, globally reachable
		"2620:4f:8000::1",  // AS112 direct delegation
		"::ffff:8.8.8.8",   // IPv4-mapped public
		"64:ff9b::808:808", // NAT64 of a public address
		"::808:808",        // IPv4-compatible public
	} {
		if decision := rules.Check(netip.MustParseAddr(text)); decision.Refused {
			t.Errorf("Check(%s) = %+v; the default set does not refuse it", text, decision)
		}
	}
}

func TestEmbeddedIPv4IsCheckedAgainstTheIPv4Set(t *testing.T) {
	rules := MustDefault()
	cases := map[string]string{
		"::ffff:127.0.0.1":   "127.0.0.0/8",
		"::ffff:10.0.0.1":    "10.0.0.0/8",
		"::ffff:100.64.0.1":  "100.64.0.0/10",
		"64:ff9b::7f00:1":    "127.0.0.0/8",
		"64:ff9b::a9fe:a9fe": "169.254.0.0/16",
		"::7f00:1":           "127.0.0.0/8",
		"::a00:1":            "10.0.0.0/8",
	}
	for text, want := range cases {
		if decision := rules.Check(netip.MustParseAddr(text)); !decision.Refused || decision.Range != want {
			t.Errorf("Check(%s) = %+v; want refused by %s through its embedded IPv4 address", text, decision, want)
		}
	}
	for _, text := range []string{"::", "::1"} {
		if decision := rules.Check(netip.MustParseAddr(text)); !decision.Refused || !strings.HasPrefix(decision.Range, text) {
			t.Errorf("Check(%s) = %+v; must stay refused as itself, by its IPv6 entry", text, decision)
		}
	}
	if _, ok := Embedded(netip.MustParseAddr("2002:7f00:1::1")); ok {
		t.Error("6to4 is refused whole, not unwrapped")
	}
}

func TestAStatedSetReplacesTheDefault(t *testing.T) {
	rules, err := Parse([]string{"10.0.0.0/8"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !rules.Check(netip.MustParseAddr("10.9.9.9")).Refused {
		t.Error("the stated range must be refused")
	}
	if rules.Check(netip.MustParseAddr("127.0.0.1")).Refused {
		t.Error("a stated set replaces the default; loopback is not in it")
	}
	omitted := rules.OmittedDefaults()
	if len(omitted) != 30 || strings.Contains(strings.Join(omitted, ","), "10.0.0.0/8") {
		t.Errorf("OmittedDefaults = %v; want the 30 defaults other than 10.0.0.0/8", omitted)
	}

	none, err := Parse([]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if none.Check(netip.MustParseAddr("127.0.0.1")).Refused || len(none.OmittedDefaults()) != 31 {
		t.Error("a stated empty set refuses nothing and omits every default")
	}

	wider, err := Parse([]string{"10.0.0.0/7", "0.0.0.0/0", "::/0"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if omitted := wider.OmittedDefaults(); len(omitted) != 0 {
		t.Errorf("a range covering a default does not omit it: %v", omitted)
	}
	if len(MustDefault().OmittedDefaults()) != 0 {
		t.Error("the default omits nothing")
	}
}

func TestMalformedEntriesAreRefusedByName(t *testing.T) {
	for _, entry := range []string{"", "not-an-address", "10.0.0.1/8", "10.0.0.0/33", "fe80::1%eth0", "fe80::%eth0/64", "10.0.0.0/8/8", "10.0.0"} {
		for _, lists := range [][2][]string{{{entry}, nil}, {nil, {entry}}} {
			_, err := Parse(lists[0], lists[1])
			if err == nil {
				t.Errorf("Parse(%q) accepted a malformed entry", entry)
				continue
			}
			if !strings.Contains(err.Error(), "security.remote_media.") || !strings.Contains(err.Error(), `"`+entry+`"`) {
				t.Errorf("error %q must name the key and the entry", err)
			}
		}
	}
	if _, err := Parse([]string{"10.0.0.1/8"}, nil); err == nil || !strings.Contains(err.Error(), "10.0.0.0/8") {
		t.Errorf("a host-bits entry should say the range it names: %v", err)
	}
	rules, err := Parse([]string{"192.0.2.7", "2001:db8::7"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rules.Refused(), ","); got != "192.0.2.7/32,2001:db8::7/128" {
		t.Errorf("a bare address means its /32 or /128, got %s", got)
	}
}

func TestExceptionsPermitTheirRangeAndItsEmbeddedForms(t *testing.T) {
	rules, err := Parse(nil, []string{"192.168.1.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"192.168.1.5", "::ffff:192.168.1.5", "64:ff9b::c0a8:105", "::c0a8:105"} {
		decision := rules.Check(netip.MustParseAddr(text))
		if decision.Refused || decision.Permitted != "192.168.1.0/24" || decision.Range != "192.168.0.0/16" {
			t.Errorf("Check(%s) = %+v; want permitted by 192.168.1.0/24 (D9)", text, decision)
		}
	}
	if !rules.Check(netip.MustParseAddr("192.168.2.5")).Refused {
		t.Error("an exception permits only its own range")
	}
	if got := rules.Check(netip.MustParseAddr("8.8.8.8")); got.Refused || got.Permitted != "" {
		t.Errorf("an address no entry refuses needs no exception: %+v", got)
	}
}

func TestNoExceptionReachesLoopbackOrThisHost(t *testing.T) {
	for _, entry := range []string{
		"127.0.0.1", "127.0.0.0/8", "126.0.0.0/7", "0.0.0.0/8", "0.0.0.5", "0.0.0.0/0",
		"::1", "::", "::/0", "::/96",
		"::ffff:127.0.0.1", "::ffff:127.0.0.0/104", "::ffff:0:0/96",
		"64:ff9b::7f00:1", "64:ff9b::/96", "::7f00:1",
	} {
		if _, err := Parse(nil, []string{entry}); err == nil || !strings.Contains(err.Error(), "no exception may permit") {
			t.Errorf("Parse(permitted %q) = %v; want refused (D8)", entry, err)
		}
	}
	if _, err := Parse(nil, []string{"10.0.0.0/8", "::ffff:10.0.0.0/104", "fd00::/8"}); err != nil {
		t.Errorf("exceptions clear of loopback and this host must load: %v", err)
	}

	// Defence in depth: even Rules built without Parse's guard never permit them.
	forced := &Rules{refused: MustDefault().refused, permitted: mustPrefixes("0.0.0.0/0", "::/0")}
	for _, text := range []string{"127.0.0.1", "0.0.0.0", "::1", "::", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "::7f00:1"} {
		if !forced.Check(netip.MustParseAddr(text)).Refused {
			t.Errorf("Check(%s) was permitted by an exception (D8)", text)
		}
	}
	if forced.Check(netip.MustParseAddr("10.0.0.1")).Refused {
		t.Error("the forced exceptions should still permit ordinary private addresses")
	}
}

func TestZonesDoNotChangeTheVerdict(t *testing.T) {
	if !MustDefault().Check(netip.MustParseAddr("fe80::1%eth0")).Refused {
		t.Error("a zoned link-local address must be refused")
	}
}
