package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	warpstackv1 "github.com/corvad/warpstack/proto/warpstack/v1"
)

type Discovered struct {
	Hostname string
	IPv4     netip.Addr
	IPv6     netip.Addr
}

func FromCluster(c *warpstackv1.ClusterConfig, provider string, d Discovered) (*warpstackv1.NodeConfig, error) {
	if err := Validate(c); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	c = proto.Clone(c).(*warpstackv1.ClusterConfig)
	var p *warpstackv1.Provider
	for _, candidate := range c.GetProviders() {
		if candidate.GetName() == provider {
			p = candidate
			break
		}
	}
	if p == nil {
		return nil, fmt.Errorf("provider %q is not defined", provider)
	}
	name, _, _ := strings.Cut(d.Hostname, ".")
	if name == "" {
		return nil, errors.New("hostname is empty")
	}
	if !d.IPv4.IsValid() {
		return nil, errors.New("no global IPv4 address found")
	}
	for _, n := range p.GetNeighbors() {
		addr, _ := netip.ParseAddr(n.GetAddress())
		if addr.Is6() && !d.IPv6.IsValid() {
			return nil, fmt.Errorf("provider %q has IPv6 neighbor %q but no global IPv6 address was found", provider, n.GetAddress())
		}
	}
	n := &warpstackv1.NodeConfig{}
	n.SetName(name)
	n.SetProvider(provider)
	n.SetRouterId(d.IPv4.String())
	n.SetIpv4(d.IPv4.String())
	if d.IPv6.IsValid() {
		n.SetIpv6(d.IPv6.String())
	}
	n.SetCluster(c.GetCluster())
	n.SetServices(c.GetServices())
	n.SetBgpPasswordSecret(p.GetBgpPasswordSecret())
	n.SetNeighbors(p.GetNeighbors())
	return n, nil
}

func Save(path string, n *warpstackv1.NodeConfig) error {
	data, err := prototext.MarshalOptions{Multiline: true}.Marshal(n)
	if err != nil {
		return fmt.Errorf("marshal node config: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".node-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write node config: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod node config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync node config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close node config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename node config: %w", err)
	}
	return nil
}

func Load(path string) (*warpstackv1.NodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read node config: %w", err)
	}
	n := &warpstackv1.NodeConfig{}
	if err := prototext.Unmarshal(data, n); err != nil {
		return nil, fmt.Errorf("parse node config: %w", err)
	}
	if n.GetName() == "" {
		return nil, errors.New("node config: name is missing")
	}
	if _, err := netip.ParseAddr(n.GetRouterId()); err != nil {
		return nil, fmt.Errorf("node config: router_id %q: %w", n.GetRouterId(), err)
	}
	return n, nil
}
