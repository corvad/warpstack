package config

import (
	"net/netip"
	"testing"
)

func addrs(s ...string) []netip.Addr {
	out := make([]netip.Addr, len(s))
	for i, a := range s {
		out[i] = netip.MustParseAddr(a)
	}
	return out
}

func TestPick(t *testing.T) {
	anycast := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8::/48"),
	}
	var none netip.Addr
	tests := []struct {
		name    string
		addrs   []netip.Addr
		exclude []netip.Prefix
		wantV4  netip.Addr
		wantV6  netip.Addr
	}{
		{
			name:   "skips loopback link-local and private",
			addrs:  addrs("127.0.0.1", "::1", "169.254.1.1", "fe80::1", "10.0.0.5", "fd00::1", "198.51.100.10", "2001:db8:1::10"),
			wantV4: netip.MustParseAddr("198.51.100.10"),
			wantV6: netip.MustParseAddr("2001:db8:1::10"),
		},
		{
			name:    "skips addresses inside excluded prefixes",
			addrs:   addrs("192.0.2.53", "2001:db8::53", "198.51.100.10", "2001:db8:1::10"),
			exclude: anycast,
			wantV4:  netip.MustParseAddr("198.51.100.10"),
			wantV6:  netip.MustParseAddr("2001:db8:1::10"),
		},
		{
			name:   "first global address wins",
			addrs:  addrs("198.51.100.10", "198.51.100.20", "2001:db8:1::10", "2001:db8:1::20"),
			wantV4: netip.MustParseAddr("198.51.100.10"),
			wantV6: netip.MustParseAddr("2001:db8:1::10"),
		},
		{
			name:   "ipv4 only",
			addrs:  addrs("127.0.0.1", "198.51.100.10", "fe80::1"),
			wantV4: netip.MustParseAddr("198.51.100.10"),
			wantV6: none,
		},
		{
			name:   "nothing usable",
			addrs:  addrs("127.0.0.1", "10.0.0.5", "fe80::1"),
			wantV4: none,
			wantV6: none,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := pick("ewr.example.net", tt.addrs, tt.exclude)
			if d.Hostname != "ewr.example.net" {
				t.Errorf("hostname = %q, want %q", d.Hostname, "ewr.example.net")
			}
			if d.IPv4 != tt.wantV4 {
				t.Errorf("ipv4 = %v, want %v", d.IPv4, tt.wantV4)
			}
			if d.IPv6 != tt.wantV6 {
				t.Errorf("ipv6 = %v, want %v", d.IPv6, tt.wantV6)
			}
		})
	}
}

func TestDiscoverRuns(t *testing.T) {
	d, err := Discover(nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if d.Hostname == "" {
		t.Error("hostname is empty")
	}
}
