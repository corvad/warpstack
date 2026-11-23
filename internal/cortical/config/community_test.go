package config

import (
	"net/netip"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"

	warpstackv1 "github.com/corvad/warpstack/proto/warpstack/v1"
)

func TestParseCommunities(t *testing.T) {
	c, err := ParseCommunities([]string{"64600:4637", "20473:6000:401144", "64600:3257", "64600:4637"})
	if err != nil {
		t.Fatalf("ParseCommunities: %v", err)
	}
	if got, want := c.Key(), "64600:3257 64600:4637 20473:6000:401144"; got != want {
		t.Errorf("Key = %q, want %q", got, want)
	}
	if c.Empty() {
		t.Error("Empty = true")
	}
	if !(Communities{}).Empty() || (Communities{}).Key() != "" {
		t.Error("zero value is not empty")
	}
	for _, bad := range []string{"", "64600", "64600:70000", "a:b", "1:2:3:4", "1:2:4294967296", "-1:2"} {
		if _, err := ParseCommunities([]string{bad}); err == nil {
			t.Errorf("ParseCommunities(%q) succeeded", bad)
		}
	}
}

const sited = `
cluster { asn: 64500 announce: "192.0.2.0/24" }
providers {
  name: "vultr"
  neighbors { address: "169.254.169.254" asn: 64515 multihop: 2 }
}
sites { name: "bom" communities: "64600:4637" }
sites { name: "dfw" communities: "64600:3257" communities: "20473:6000:3257" }
`

func TestSites(t *testing.T) {
	c, err := ParseConfig([]byte(sited))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	for host, want := range map[string]int{"bom.example.net": 1, "dfw.example.net": 2, "ewr.example.net": 0} {
		n, err := FromCluster(c, "vultr", Discovered{Hostname: host, IPv4: netip.MustParseAddr("198.51.100.10")})
		if err != nil {
			t.Fatalf("FromCluster %s: %v", host, err)
		}
		if got := len(n.GetCommunities()); got != want {
			t.Errorf("%s: %d communities, want %d", host, got, want)
		}
	}

	for name, extra := range map[string]string{
		"duplicate site": `sites { name: "bom" }`,
		"missing name":   `sites { communities: "64600:1" }`,
		"bad community":  `sites { name: "x" communities: "64600:99999" }`,
	} {
		bad := &warpstackv1.ClusterConfig{}
		if err := prototext.Unmarshal([]byte(sited+extra), bad); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if err := Validate(bad); err == nil {
			t.Errorf("%s: Validate succeeded", name)
		}
	}
}
