package netd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"

	api "github.com/osrg/gobgp/v4/api"
)

const (
	globalTable = "global"
	ownPolicy   = "warpstack-own"
	setNameV4   = "warpstack-own-v4"
	setNameV6   = "warpstack-own-v6"
)

func splitFamilies(prefixes []netip.Prefix) (v4, v6 []netip.Prefix) {
	for _, p := range prefixes {
		if p.Addr().Is6() {
			v6 = append(v6, p)
		} else {
			v4 = append(v4, p)
		}
	}
	return v4, v6
}

func definedSet(name string, prefixes []netip.Prefix) *api.DefinedSet {
	set := &api.DefinedSet{DefinedType: api.DefinedType_DEFINED_TYPE_PREFIX, Name: name}
	for _, p := range sortedPrefixes(prefixes) {
		bits := uint32(p.Bits())
		set.Prefixes = append(set.Prefixes, &api.Prefix{
			IpPrefix:      p.String(),
			MaskLengthMin: bits,
			MaskLengthMax: bits,
		})
	}
	return set
}

func setKey(set *api.DefinedSet) []string {
	keys := make([]string, 0, len(set.GetPrefixes()))
	for _, p := range set.GetPrefixes() {
		keys = append(keys, fmt.Sprintf("%s %d %d", p.GetIpPrefix(), p.GetMaskLengthMin(), p.GetMaskLengthMax()))
	}
	slices.Sort(keys)
	return keys
}

func (g *GoBGP) definedSetMatches(ctx context.Context, name string, prefixes []netip.Prefix) (bool, error) {
	if len(prefixes) == 0 {
		return true, nil
	}
	stream, err := g.client.ListDefinedSet(ctx, &api.ListDefinedSetRequest{
		DefinedType: api.DefinedType_DEFINED_TYPE_PREFIX,
		Name:        name,
	})
	if err != nil {
		return false, fmt.Errorf("list defined set %s: %w", name, err)
	}
	found := false
	var got []string
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, fmt.Errorf("list defined set %s: %w", name, err)
		}
		if resp.GetDefinedSet().GetName() != name {
			continue
		}
		found = true
		got = setKey(resp.GetDefinedSet())
	}
	return found && slices.Equal(got, setKey(definedSet(name, prefixes))), nil
}

func (g *GoBGP) assignment(ctx context.Context, direction api.PolicyDirection) (*api.PolicyAssignment, error) {
	stream, err := g.client.ListPolicyAssignment(ctx, &api.ListPolicyAssignmentRequest{
		Name:      globalTable,
		Direction: direction,
	})
	if err != nil {
		return nil, fmt.Errorf("list policy assignment: %w", err)
	}
	var result *api.PolicyAssignment
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, fmt.Errorf("list policy assignment: %w", err)
		}
		if a := resp.GetAssignment(); a.GetDirection() == direction {
			result = a
		}
	}
}

func (g *GoBGP) assign(ctx context.Context, direction api.PolicyDirection, policies []*api.Policy) error {
	_, err := g.client.SetPolicyAssignment(ctx, &api.SetPolicyAssignmentRequest{
		Assignment: &api.PolicyAssignment{
			Name:          globalTable,
			Direction:     direction,
			Policies:      policies,
			DefaultAction: api.RouteAction_ROUTE_ACTION_REJECT,
		},
	})
	if err != nil {
		return fmt.Errorf("set policy assignment: %w", err)
	}
	return nil
}

func policyDirections() []api.PolicyDirection {
	return []api.PolicyDirection{
		api.PolicyDirection_POLICY_DIRECTION_EXPORT,
		api.PolicyDirection_POLICY_DIRECTION_IMPORT,
	}
}

func ownAssigned(a *api.PolicyAssignment) bool {
	return a.GetDefaultAction() == api.RouteAction_ROUTE_ACTION_REJECT &&
		len(a.GetPolicies()) == 1 &&
		a.GetPolicies()[0].GetName() == ownPolicy
}

func (g *GoBGP) assignmentsOK(ctx context.Context) (bool, error) {
	for _, direction := range policyDirections() {
		a, err := g.assignment(ctx, direction)
		if err != nil {
			return false, err
		}
		if !ownAssigned(a) {
			return false, nil
		}
	}
	return true, nil
}

func (g *GoBGP) PolicyOK(ctx context.Context, allowed []netip.Prefix) (bool, error) {
	ok, err := g.assignmentsOK(ctx)
	if err != nil || !ok {
		return false, err
	}
	v4, v6 := splitFamilies(allowed)
	for name, prefixes := range map[string][]netip.Prefix{setNameV4: v4, setNameV6: v6} {
		ok, err := g.definedSetMatches(ctx, name, prefixes)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func (g *GoBGP) SetPolicy(ctx context.Context, allowed []netip.Prefix) error {
	v4, v6 := splitFamilies(allowed)
	sets := []struct {
		name     string
		prefixes []netip.Prefix
	}{{setNameV4, v4}, {setNameV6, v6}}

	var statements []*api.Statement
	for _, s := range sets {
		if len(s.prefixes) == 0 {
			continue
		}
		_, err := g.client.AddDefinedSet(ctx, &api.AddDefinedSetRequest{
			DefinedSet: definedSet(s.name, s.prefixes),
			Replace:    true,
		})
		if err != nil {
			return fmt.Errorf("add defined set %s: %w", s.name, err)
		}
		statements = append(statements, &api.Statement{
			Name:       s.name,
			Conditions: &api.Conditions{PrefixSet: &api.MatchSet{Type: api.MatchSet_TYPE_ANY, Name: s.name}},
			Actions:    &api.Actions{RouteAction: api.RouteAction_ROUTE_ACTION_ACCEPT},
		})
	}

	ok, err := g.assignmentsOK(ctx)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	for _, direction := range policyDirections() {
		if err := g.assign(ctx, direction, nil); err != nil {
			return err
		}
	}
	_, _ = g.client.DeletePolicy(ctx, &api.DeletePolicyRequest{Policy: &api.Policy{Name: ownPolicy}})
	_, err = g.client.AddPolicy(ctx, &api.AddPolicyRequest{
		Policy: &api.Policy{Name: ownPolicy, Statements: statements},
	})
	if err != nil {
		return fmt.Errorf("add policy %s: %w", ownPolicy, err)
	}
	for _, direction := range policyDirections() {
		if err := g.assign(ctx, direction, []*api.Policy{{Name: ownPolicy}}); err != nil {
			return err
		}
	}
	return nil
}

func (g *GoBGP) Observe(ctx context.Context, allowed []netip.Prefix) (Observed, error) {
	started, asn, routerID, err := g.Global(ctx)
	if err != nil || !started {
		return Observed{}, err
	}
	o := Observed{Started: true, ASN: asn, RouterID: routerID}
	if o.PolicyOK, err = g.PolicyOK(ctx, allowed); err != nil {
		return Observed{}, err
	}
	if o.Peers, err = g.Peers(ctx); err != nil {
		return Observed{}, err
	}
	if o.Paths, o.Tags, err = g.PathTags(ctx); err != nil {
		return Observed{}, err
	}
	return o, nil
}
