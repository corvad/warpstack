package netd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"

	api "github.com/osrg/gobgp/v4/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GoBGP struct {
	conn       *grpc.ClientConn
	client     api.GoBgpServiceClient
	listenPort int32
}

func Dial(address string, listenPort int32) (*GoBGP, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial speaker %s: %w", address, err)
	}
	return &GoBGP{
		conn:       conn,
		client:     api.NewGoBgpServiceClient(conn),
		listenPort: listenPort,
	}, nil
}

func (g *GoBGP) Close() error {
	return g.conn.Close()
}

func (g *GoBGP) Start(ctx context.Context, asn uint32, routerID netip.Addr) error {
	_, err := g.client.StartBgp(ctx, &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        asn,
			RouterId:   routerID.String(),
			ListenPort: g.listenPort,
		},
	})
	if err != nil {
		return fmt.Errorf("start bgp: %w", err)
	}
	return nil
}

func (g *GoBGP) Stop(ctx context.Context) error {
	if _, err := g.client.StopBgp(ctx, &api.StopBgpRequest{}); err != nil {
		return fmt.Errorf("stop bgp: %w", err)
	}
	return nil
}

func (g *GoBGP) Global(ctx context.Context) (started bool, asn uint32, routerID netip.Addr, err error) {
	resp, err := g.client.GetBgp(ctx, &api.GetBgpRequest{})
	if err != nil {
		return false, 0, netip.Addr{}, fmt.Errorf("get bgp: %w", err)
	}
	global := resp.GetGlobal()
	if global.GetAsn() == 0 {
		return false, 0, netip.Addr{}, nil
	}
	routerID, err = netip.ParseAddr(global.GetRouterId())
	if err != nil {
		return false, 0, netip.Addr{}, fmt.Errorf("speaker router id %q: %w", global.GetRouterId(), err)
	}
	return true, global.GetAsn(), routerID, nil
}

func (g *GoBGP) Peers(ctx context.Context) ([]Peer, error) {
	stream, err := g.client.ListPeer(ctx, &api.ListPeerRequest{})
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	var peers []Peer
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return peers, nil
		}
		if err != nil {
			return nil, fmt.Errorf("list peers: %w", err)
		}
		p := resp.GetPeer()
		addr, err := netip.ParseAddr(p.GetConf().GetNeighborAddress())
		if err != nil {
			return nil, fmt.Errorf("speaker peer address %q: %w", p.GetConf().GetNeighborAddress(), err)
		}
		peer := Peer{Address: addr, ASN: p.GetConf().GetPeerAsn()}
		if mh := p.GetEbgpMultihop(); mh.GetEnabled() {
			peer.Multihop = mh.GetMultihopTtl()
		}
		if local, err := netip.ParseAddr(p.GetTransport().GetLocalAddress()); err == nil && !local.IsUnspecified() {
			peer.Local = local
		}
		peers = append(peers, peer)
	}
}

func (g *GoBGP) AddPeer(ctx context.Context, p Peer) error {
	family := &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST}
	if p.Address.Is6() {
		family.Afi = api.Family_AFI_IP6
	}
	peer := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: p.Address.String(),
			PeerAsn:         p.ASN,
			AuthPassword:    p.Password,
		},
		AfiSafis: []*api.AfiSafi{{
			Config: &api.AfiSafiConfig{Family: family, Enabled: true},
		}},
	}
	if p.Local.IsValid() {
		peer.Transport = &api.Transport{LocalAddress: p.Local.String()}
	}
	if p.Multihop > 0 {
		peer.EbgpMultihop = &api.EbgpMultihop{Enabled: true, MultihopTtl: p.Multihop}
	}
	if _, err := g.client.AddPeer(ctx, &api.AddPeerRequest{Peer: peer}); err != nil {
		return fmt.Errorf("add peer %s: %w", p.Address, err)
	}
	return nil
}

func (g *GoBGP) DeletePeer(ctx context.Context, address netip.Addr) error {
	if _, err := g.client.DeletePeer(ctx, &api.DeletePeerRequest{Address: address.String()}); err != nil {
		return fmt.Errorf("delete peer %s: %w", address, err)
	}
	return nil
}
