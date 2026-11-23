package netd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"

	api "github.com/osrg/gobgp/v4/api"

	"github.com/corvad/warpstack/internal/cortical/config"
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

func pathOf(p netip.Prefix, c config.Communities) *api.Path {
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
	attrs := []*api.Attribute{origin, nextHop}
	if len(c.Standard) > 0 {
		attrs = append(attrs, &api.Attribute{Attr: &api.Attribute_Communities{
			Communities: &api.CommunitiesAttribute{Communities: c.Standard},
		}})
	}
	if len(c.Large) > 0 {
		large := make([]*api.LargeCommunity, 0, len(c.Large))
		for _, l := range c.Large {
			large = append(large, &api.LargeCommunity{GlobalAdmin: l.Global, LocalData1: l.Local1, LocalData2: l.Local2})
		}
		attrs = append(attrs, &api.Attribute{Attr: &api.Attribute_LargeCommunities{
			LargeCommunities: &api.LargeCommunitiesAttribute{Communities: large},
		}})
	}
	return &api.Path{
		Family: familyOf(v6),
		Nlri:   nlriOf(p),
		Pattrs: attrs,
	}
}

func communitiesOf(p *api.Path) config.Communities {
	var standard []uint32
	var large []config.Large
	for _, a := range p.GetPattrs() {
		standard = append(standard, a.GetCommunities().GetCommunities()...)
		for _, l := range a.GetLargeCommunities().GetCommunities() {
			large = append(large, config.Large{Global: l.GetGlobalAdmin(), Local1: l.GetLocalData1(), Local2: l.GetLocalData2()})
		}
	}
	return config.NewCommunities(standard, large)
}

func (g *GoBGP) AddPath(ctx context.Context, p netip.Prefix, c config.Communities) error {
	_, err := g.client.AddPath(ctx, &api.AddPathRequest{
		TableType: api.TableType_TABLE_TYPE_GLOBAL,
		Path:      pathOf(p, c),
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
		Path:      pathOf(p, config.Communities{}),
	})
	if err != nil {
		return fmt.Errorf("delete path %s: %w", p, err)
	}
	return nil
}

func (g *GoBGP) PathTags(ctx context.Context) ([]netip.Prefix, map[netip.Prefix]string, error) {
	var out []netip.Prefix
	tags := make(map[netip.Prefix]string)
	for _, v6 := range []bool{false, true} {
		stream, err := g.client.ListPath(ctx, &api.ListPathRequest{
			TableType: api.TableType_TABLE_TYPE_GLOBAL,
			Family:    familyOf(v6),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("list paths: %w", err)
		}
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, nil, fmt.Errorf("list paths: %w", err)
			}
			dest := resp.GetDestination()
			var local *api.Path
			for _, p := range dest.GetPaths() {
				if !p.GetIsFromExternal() {
					local = p
					break
				}
			}
			if local == nil {
				continue
			}
			prefix, err := netip.ParsePrefix(dest.GetPrefix())
			if err != nil {
				return nil, nil, fmt.Errorf("speaker prefix %q: %w", dest.GetPrefix(), err)
			}
			out = append(out, prefix)
			tags[prefix] = communitiesOf(local).Key()
		}
	}
	return out, tags, nil
}

func (g *GoBGP) Paths(ctx context.Context) ([]netip.Prefix, error) {
	out, _, err := g.PathTags(ctx)
	return out, err
}
