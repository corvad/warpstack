package netd

import (
	"fmt"
	"net/netip"

	"github.com/corvad/warpstack/internal/cortical/config"
	warpstackv1 "github.com/corvad/warpstack/proto/warpstack/v1"
)

type Peer struct {
	Address  netip.Addr
	ASN      uint32
	Multihop uint32
	Local    netip.Addr
	Password string
}

type Desired struct {
	ASN         uint32
	RouterID    netip.Addr
	Allowed     []netip.Prefix
	Peers       []Peer
	Paths       []netip.Prefix
	Communities config.Communities
}

type Observed struct {
	Started  bool
	ASN      uint32
	RouterID netip.Addr
	PolicyOK bool
	Peers    []Peer
	Paths    []netip.Prefix
	Tags     map[netip.Prefix]string
}

func Plan(n *warpstackv1.NodeConfig, password string, announce bool) (Desired, error) {
	routerID, err := netip.ParseAddr(n.GetRouterId())
	if err != nil {
		return Desired{}, fmt.Errorf("router_id %q: %w", n.GetRouterId(), err)
	}
	var v4, v6 netip.Addr
	if s := n.GetIpv4(); s != "" {
		if v4, err = netip.ParseAddr(s); err != nil {
			return Desired{}, fmt.Errorf("ipv4 %q: %w", s, err)
		}
	}
	if s := n.GetIpv6(); s != "" {
		if v6, err = netip.ParseAddr(s); err != nil {
			return Desired{}, fmt.Errorf("ipv6 %q: %w", s, err)
		}
	}
	if n.GetBgpPasswordSecret() == "" {
		password = ""
	}
	communities, err := config.ParseCommunities(n.GetCommunities())
	if err != nil {
		return Desired{}, err
	}
	d := Desired{ASN: n.GetCluster().GetAsn(), RouterID: routerID, Communities: communities}
	for _, s := range n.GetCluster().GetAnnounce() {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return Desired{}, fmt.Errorf("announce %q: %w", s, err)
		}
		d.Allowed = append(d.Allowed, p)
		if announce {
			d.Paths = append(d.Paths, p)
		}
	}
	for _, nb := range n.GetNeighbors() {
		addr, err := netip.ParseAddr(nb.GetAddress())
		if err != nil {
			return Desired{}, fmt.Errorf("neighbor %q: %w", nb.GetAddress(), err)
		}
		local := v4
		if addr.Is6() {
			local = v6
		}
		if !local.IsValid() {
			return Desired{}, fmt.Errorf("neighbor %q: no local address of the same family", nb.GetAddress())
		}
		d.Peers = append(d.Peers, Peer{
			Address:  addr,
			ASN:      nb.GetAsn(),
			Multihop: nb.GetMultihop(),
			Local:    local,
			Password: password,
		})
	}
	return d, nil
}
