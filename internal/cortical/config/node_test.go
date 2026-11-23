package config

import (
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

func discovered() Discovered {
	return Discovered{
		Hostname: "ewr.example.net",
		IPv4:     netip.MustParseAddr("198.51.100.10"),
		IPv6:     netip.MustParseAddr("2001:db8:1::10"),
	}
}

func TestFromCluster(t *testing.T) {
	c, err := ParseConfig([]byte(valid))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	n, err := FromCluster(c, "vultr", discovered())
	if err != nil {
		t.Fatalf("FromCluster: %v", err)
	}
	if got := n.GetName(); got != "ewr" {
		t.Errorf("name = %q, want %q", got, "ewr")
	}
	if got := n.GetRouterId(); got != "198.51.100.10" {
		t.Errorf("router_id = %q, want %q", got, "198.51.100.10")
	}
	if got := n.GetIpv6(); got != "2001:db8:1::10" {
		t.Errorf("ipv6 = %q, want %q", got, "2001:db8:1::10")
	}
	if got := n.GetCluster().GetAsn(); got != 64500 {
		t.Errorf("asn = %d, want 64500", got)
	}
	if got := len(n.GetNeighbors()); got != 2 {
		t.Errorf("neighbor count = %d, want 2", got)
	}
	if got := n.GetBgpPasswordSecret(); got != "vultr_bgp" {
		t.Errorf("bgp_password_secret = %q, want %q", got, "vultr_bgp")
	}
}

func TestFromClusterErrors(t *testing.T) {
	c, err := ParseConfig([]byte(valid))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	noV4 := discovered()
	noV4.IPv4 = netip.Addr{}
	noV6 := discovered()
	noV6.IPv6 = netip.Addr{}
	noHost := discovered()
	noHost.Hostname = ""
	tests := []struct {
		name     string
		provider string
		d        Discovered
		want     string
	}{
		{"unknown provider", "nope", discovered(), `provider "nope" is not defined`},
		{"no ipv4", "vultr", noV4, "no global IPv4 address"},
		{"no ipv6", "vultr", noV6, "no global IPv6 address"},
		{"no hostname", "vultr", noHost, "hostname is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromCluster(c, tt.provider, tt.d)
			if err == nil {
				t.Fatal("FromCluster succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	c, err := ParseConfig([]byte(valid))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	want, err := FromCluster(c, "vultr", discovered())
	if err != nil {
		t.Fatalf("FromCluster: %v", err)
	}
	path := filepath.Join(t.TempDir(), "node.txtpb")
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !proto.Equal(got, want) {
		t.Errorf("round trip mismatch:\n got: %v\nwant: %v", got, want)
	}
}
