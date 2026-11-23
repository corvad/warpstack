package config

import (
	"errors"
	"fmt"
	"net/netip"

	"google.golang.org/protobuf/encoding/prototext"

	warpstackv1 "github.com/corvad/warpstack/proto/warpstack/v1"
)

func ParseConfig(data []byte) (*warpstackv1.ClusterConfig, error) {
	c := &warpstackv1.ClusterConfig{}
	if err := prototext.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := Validate(c); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return c, nil
}

func Validate(c *warpstackv1.ClusterConfig) error {
	if c == nil {
		return errors.New("config is nil")
	}
	if !c.HasCluster() {
		return errors.New("cluster section is missing")
	}
	prefixes, err := validateCluster(c.GetCluster())
	if err != nil {
		return fmt.Errorf("cluster: %w", err)
	}
	if err := validateServices(c.GetServices(), prefixes); err != nil {
		return err
	}
	if err := validateProviders(c.GetProviders()); err != nil {
		return err
	}
	return validateSites(c.GetSites())
}

func validateCluster(c *warpstackv1.Cluster) ([]netip.Prefix, error) {
	if c.GetAsn() == 0 {
		return nil, errors.New("asn is missing or zero")
	}
	announce := c.GetAnnounce()
	if len(announce) == 0 {
		return nil, errors.New("announce is empty")
	}
	prefixes := make([]netip.Prefix, 0, len(announce))
	seen := make(map[netip.Prefix]bool, len(announce))
	for _, s := range announce {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("announce %q: %w", s, err)
		}
		if p != p.Masked() {
			return nil, fmt.Errorf("announce %q: host bits set, want %s", s, p.Masked())
		}
		if seen[p] {
			return nil, fmt.Errorf("announce %q: duplicate", s)
		}
		seen[p] = true
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}

func validateServices(services []*warpstackv1.Service, prefixes []netip.Prefix) error {
	names := make(map[string]bool, len(services))
	used := make(map[netip.Addr]string)
	for _, s := range services {
		name := s.GetName()
		if name == "" {
			return errors.New("service: name is missing")
		}
		if names[name] {
			return fmt.Errorf("service %q: duplicate name", name)
		}
		names[name] = true
		if len(s.GetAddresses()) == 0 {
			return fmt.Errorf("service %q: no addresses", name)
		}
		for _, a := range s.GetAddresses() {
			addr, err := netip.ParseAddr(a)
			if err != nil {
				return fmt.Errorf("service %q: address %q: %w", name, a, err)
			}
			if !containsAddr(prefixes, addr) {
				return fmt.Errorf("service %q: address %q is outside all announced prefixes", name, a)
			}
			if other, ok := used[addr]; ok {
				return fmt.Errorf("service %q: address %q already used by service %q", name, a, other)
			}
			used[addr] = name
		}
	}
	return nil
}

func containsAddr(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func validateProviders(providers []*warpstackv1.Provider) error {
	if len(providers) == 0 {
		return errors.New("no providers defined")
	}
	names := make(map[string]bool, len(providers))
	for _, p := range providers {
		name := p.GetName()
		if name == "" {
			return errors.New("provider: name is missing")
		}
		if names[name] {
			return fmt.Errorf("provider %q: duplicate name", name)
		}
		names[name] = true
		if len(p.GetNeighbors()) == 0 {
			return fmt.Errorf("provider %q: no neighbors", name)
		}
		seen := make(map[netip.Addr]bool, len(p.GetNeighbors()))
		for _, n := range p.GetNeighbors() {
			addr, err := netip.ParseAddr(n.GetAddress())
			if err != nil {
				return fmt.Errorf("provider %q: neighbor %q: %w", name, n.GetAddress(), err)
			}
			if seen[addr] {
				return fmt.Errorf("provider %q: neighbor %q: duplicate", name, n.GetAddress())
			}
			seen[addr] = true
			if n.GetAsn() == 0 {
				return fmt.Errorf("provider %q: neighbor %q: asn is missing or zero", name, n.GetAddress())
			}
			if n.GetMultihop() > 255 {
				return fmt.Errorf("provider %q: neighbor %q: multihop %d exceeds 255", name, n.GetAddress(), n.GetMultihop())
			}
		}
	}
	return nil
}

func validateSites(sites []*warpstackv1.Site) error {
	names := make(map[string]bool, len(sites))
	for _, s := range sites {
		name := s.GetName()
		if name == "" {
			return errors.New("site: name is missing")
		}
		if names[name] {
			return fmt.Errorf("site %q: duplicate name", name)
		}
		names[name] = true
		if _, err := ParseCommunities(s.GetCommunities()); err != nil {
			return fmt.Errorf("site %q: %w", name, err)
		}
	}
	return nil
}
