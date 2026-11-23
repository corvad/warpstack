package netd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"

	api "github.com/osrg/gobgp/v4/api"
)

func familyOf(v6 bool) *api.Family {
	if v6 {
		return &api.Family{Afi: api.Family_AFI_IP6, Safi: api.Family_SAFI_UNICAST}
	}
	return &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST}
}

func nlriOf(p netip.Prefix) *api.NLRI {
	return &api.NLRI{Nlri: &api.NLRI_Prefix{Prefix: &api.IPAddressPrefix{
		PrefixLen: uint32(p.Bits()),
		Prefix:    p.Addr().String(),
	}}}
}

func pathOf(p netip.Prefix) *api.Path {
	v6 := p.Addr().Is6()
	origin := &api.Attribute{Attr: &api.Attribute_Origin{Origin: &api.OriginAttribute{Origin: 0}}}
	var nextHop *api.Attribute
	if v6 {
		nextHop = &api.Attribute{Attr: &api.Attribute_MpReach{MpReach: &api.MpReachNLRIAttribute{
			Family:   familyOf(true),
			NextHops: []string{"::"},
			Nlris:    []*api.NLRI{nlriOf(p)},
		}}}
	} else {
		nextHop = &api.Attribute{Attr: &api.Attribute_NextHop{NextHop: &api.NextHopAttribute{NextHop: "0.0.0.0"}}}
	}
	return &api.Path{
		Family: familyOf(v6),
		Nlri:   nlriOf(p),
		Pattrs: []*api.Attribute{origin, nextHop},
	}
}

func (g *GoBGP) AddPath(ctx context.Context, p netip.Prefix) error {
	_, err := g.client.AddPath(ctx, &api.AddPathRequest{
		TableType: api.TableType_TABLE_TYPE_GLOBAL,
		Path:      pathOf(p),
	})
	if err != nil {
		return fmt.Errorf("add path %s: %w", p, err)
	}
	return nil
}

func (g *GoBGP) DeletePath(ctx context.Context, p netip.Prefix) error {
	_, err := g.client.DeletePath(ctx, &api.DeletePathRequest{
		TableType: api.TableType_TABLE_TYPE_GLOBAL,
		Family:    familyOf(p.Addr().Is6()),
		Path:      pathOf(p),
	})
	if err != nil {
		return fmt.Errorf("delete path %s: %w", p, err)
	}
	return nil
}

func (g *GoBGP) Paths(ctx context.Context) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, v6 := range []bool{false, true} {
		stream, err := g.client.ListPath(ctx, &api.ListPathRequest{
			TableType: api.TableType_TABLE_TYPE_GLOBAL,
			Family:    familyOf(v6),
		})
		if err != nil {
			return nil, fmt.Errorf("list paths: %w", err)
		}
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("list paths: %w", err)
			}
			dest := resp.GetDestination()
			local := false
			for _, p := range dest.GetPaths() {
				if !p.GetIsFromExternal() {
					local = true
					break
				}
			}
			if !local {
				continue
			}
			prefix, err := netip.ParsePrefix(dest.GetPrefix())
			if err != nil {
				return nil, fmt.Errorf("speaker prefix %q: %w", dest.GetPrefix(), err)
			}
			out = append(out, prefix)
		}
	}
	return out, nil
}
