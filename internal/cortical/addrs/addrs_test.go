package addrs

import (
	"net/netip"
	"slices"
	"testing"
)

const ipJSON = `[{"ifname":"lo","addr_info":[
{"family":"inet","local":"127.0.0.1","prefixlen":8},
{"family":"inet","local":"192.0.2.53","prefixlen":32},
{"family":"inet","local":"192.0.2.99","prefixlen":32},
{"family":"inet6","local":"::1","prefixlen":128}]}]`

func TestParseAndDiff(t *testing.T) {
	have, err := ParseAddrs([]byte(ipJSON))
	if err != nil {
		t.Fatalf("ParseAddrs: %v", err)
	}
	if len(have) != 4 {
		t.Fatalf("parsed %d addresses, want 4: %v", len(have), have)
	}
	want := []netip.Addr{netip.MustParseAddr("192.0.2.53"), netip.MustParseAddr("2001:db8::53")}
	managed := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("2001:db8::/48")}
	add, del := Diff(want, have, managed)
	if !slices.Equal(add, []netip.Addr{netip.MustParseAddr("2001:db8::53")}) {
		t.Errorf("add = %v", add)
	}
	if !slices.Equal(del, []netip.Addr{netip.MustParseAddr("192.0.2.99")}) {
		t.Errorf("del = %v, want only the stale managed address", del)
	}
}
