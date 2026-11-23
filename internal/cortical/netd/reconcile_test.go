package netd

import (
	"net/netip"
	"slices"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"

	warpstackv1 "github.com/corvad/warpstack/proto/warpstack/v1"
)

func desired() Desired {
	v4 := netip.MustParseAddr("198.51.100.10")
	v6 := netip.MustParseAddr("2001:db8:1::10")
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8::/48"),
	}
	return Desired{
		ASN:      64500,
		RouterID: v4,
		Allowed:  prefixes,
		Paths:    slices.Clone(prefixes),
		Peers: []Peer{
			{Address: netip.MustParseAddr("169.254.169.254"), ASN: 64515, Multihop: 2, Local: v4, Password: "x"},
			{Address: netip.MustParseAddr("2001:19f0:ffff::1"), ASN: 64515, Multihop: 2, Local: v6, Password: "x"},
		},
	}
}

func converged() Observed {
	d := desired()
	peers := slices.Clone(d.Peers)
	for i := range peers {
		peers[i].Password = ""
	}
	return Observed{
		Started:  true,
		ASN:      d.ASN,
		RouterID: d.RouterID,
		PolicyOK: true,
		Peers:    peers,
		Paths:    slices.Clone(d.Paths),
	}
}

func names(actions []Action) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = a.String()
	}
	return out
}

func TestReconcile(t *testing.T) {
	full := []string{
		"start",
		"set-policy",
		"add-peer 169.254.169.254",
		"add-peer 2001:19f0:ffff::1",
		"add-path 192.0.2.0/24",
		"add-path 2001:db8::/48",
	}

	wrongASN := converged()
	wrongASN.ASN = 64999

	noPolicy := converged()
	noPolicy.PolicyOK = false

	extras := converged()
	extras.Peers = append(extras.Peers, Peer{Address: netip.MustParseAddr("203.0.113.1"), ASN: 64999})
	extras.Paths = append(extras.Paths, netip.MustParsePrefix("203.0.113.0/24"))

	changedPeer := converged()
	changedPeer.Peers[0].Multihop = 5

	notAnnouncing := desired()
	notAnnouncing.Paths = nil

	tests := []struct {
		name string
		d    Desired
		o    Observed
		want []string
	}{
		{"fresh speaker", desired(), Observed{}, full},
		{"converged", desired(), converged(), nil},
		{"wrong asn restarts", desired(), wrongASN, append([]string{"stop"}, full...)},
		{"policy missing", desired(), noPolicy, []string{"set-policy"}},
		{"extra peer and path removed", desired(), extras, []string{"delete-peer 203.0.113.1", "delete-path 203.0.113.0/24"}},
		{"changed peer replaced", desired(), changedPeer, []string{"delete-peer 169.254.169.254", "add-peer 169.254.169.254"}},
		{"withdraw when not announcing", notAnnouncing, converged(), []string{"delete-path 192.0.2.0/24", "delete-path 2001:db8::/48"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := names(Reconcile(tt.d, tt.o))
			if !slices.Equal(got, tt.want) {
				t.Errorf("actions:\n got: %v\nwant: %v", got, tt.want)
			}
		})
	}
}

const node = `
name: "ewr"
provider: "vultr"
router_id: "198.51.100.10"
ipv4: "198.51.100.10"
ipv6: "2001:db8:1::10"
cluster {
  asn: 64500
  announce: "192.0.2.0/24"
  announce: "2001:db8::/48"
}
bgp_password_secret: "vultr_bgp"
neighbors { address: "169.254.169.254" asn: 64515 multihop: 2 }
neighbors { address: "2001:19f0:ffff::1" asn: 64515 multihop: 2 }
`

func TestPlan(t *testing.T) {
	n := &warpstackv1.NodeConfig{}
	if err := prototext.Unmarshal([]byte(node), n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	d, err := Plan(n, "secret", true)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if d.ASN != 64500 {
		t.Errorf("asn = %d, want 64500", d.ASN)
	}
	if len(d.Peers) != 2 || len(d.Paths) != 2 || len(d.Allowed) != 2 {
		t.Fatalf("peers=%d paths=%d allowed=%d, want 2 each", len(d.Peers), len(d.Paths), len(d.Allowed))
	}
	if got := d.Peers[0].Local.String(); got != "198.51.100.10" {
		t.Errorf("v4 peer local = %s, want 198.51.100.10", got)
	}
	if got := d.Peers[1].Local.String(); got != "2001:db8:1::10" {
		t.Errorf("v6 peer local = %s, want 2001:db8:1::10", got)
	}
	if d.Peers[0].Password != "secret" {
		t.Errorf("password not applied")
	}

	quiet, err := Plan(n, "secret", false)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(quiet.Paths) != 0 || len(quiet.Allowed) != 2 {
		t.Errorf("announce=false: paths=%d allowed=%d, want 0 and 2", len(quiet.Paths), len(quiet.Allowed))
	}

	n.SetIpv6("")
	if _, err := Plan(n, "secret", true); err == nil {
		t.Error("Plan succeeded with an IPv6 neighbor and no IPv6 address")
	}
}
