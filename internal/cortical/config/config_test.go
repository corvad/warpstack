package config

import (
	"strings"
	"testing"
)

const valid = `
cluster {
  asn: 64500
  announce: "192.0.2.0/24"
  announce: "2001:db8::/48"
}
services {
  name: "dns"
  addresses: "192.0.2.53"
  addresses: "2001:db8::53"
}
providers {
  name: "vultr"
  bgp_password_secret: "vultr_bgp"
  neighbors { address: "169.254.169.254" asn: 64515 multihop: 2 }
  neighbors { address: "2001:19f0:ffff::1" asn: 64515 multihop: 2 }
}
`

func TestParseConfigValid(t *testing.T) {
	c, err := ParseConfig([]byte(valid))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if got := c.GetCluster().GetAsn(); got != 64500 {
		t.Errorf("asn = %d, want 64500", got)
	}
	if got := len(c.GetCluster().GetAnnounce()); got != 2 {
		t.Errorf("announce count = %d, want 2", got)
	}
	if got := len(c.GetProviders()[0].GetNeighbors()); got != 2 {
		t.Errorf("neighbor count = %d, want 2", got)
	}
}

func TestParseConfigInvalid(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		want string
	}{
		{"zero asn", "asn: 64500", "asn: 0", "asn is missing or zero"},
		{"bad prefix", `"192.0.2.0/24"`, `"192.0.2.0/33"`, `announce "192.0.2.0/33"`},
		{"host bits", `"192.0.2.0/24"`, `"192.0.2.1/24"`, "host bits set"},
		{"duplicate prefix", `"2001:db8::/48"`, `"192.0.2.0/24"`, "duplicate"},
		{"service outside prefixes", `"192.0.2.53"`, `"198.51.100.53"`, "outside all announced prefixes"},
		{"bad service address", `"192.0.2.53"`, `"not-an-ip"`, `address "not-an-ip"`},
		{"missing service name", `name: "dns"`, `name: ""`, "service: name is missing"},
		{"missing provider name", `name: "vultr"`, `name: ""`, "provider: name is missing"},
		{"zero neighbor asn", `"169.254.169.254" asn: 64515`, `"169.254.169.254" asn: 0`, `neighbor "169.254.169.254": asn is missing or zero`},
		{"multihop too large", "multihop: 2", "multihop: 300", "exceeds 255"},
		{"duplicate neighbor", `"2001:19f0:ffff::1"`, `"169.254.169.254"`, "duplicate"},
		{"unknown field", "asn: 64500", "asn: 64500\n  bogus: 1", "parse config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.Replace(valid, tt.old, tt.new, 1)
			if in == valid {
				t.Fatalf("replacement %q did not apply", tt.old)
			}
			_, err := ParseConfig([]byte(in))
			if err == nil {
				t.Fatal("ParseConfig succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestParseConfigMissingSections(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no cluster", `providers { name: "vultr" neighbors { address: "169.254.169.254" asn: 64515 } }`, "cluster section is missing"},
		{"no providers", `cluster { asn: 64500 announce: "192.0.2.0/24" }`, "no providers defined"},
		{"no announce", `cluster { asn: 64500 }`, "announce is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.in))
			if err == nil {
				t.Fatal("ParseConfig succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
