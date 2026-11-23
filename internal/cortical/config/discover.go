package config

import (
	"fmt"
	"net"
	"net/netip"
	"os"
)

func Discover(exclude []netip.Prefix) (Discovered, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return Discovered{}, fmt.Errorf("hostname: %w", err)
	}
	ifaddrs, err := net.InterfaceAddrs()
	if err != nil {
		return Discovered{}, fmt.Errorf("interface addresses: %w", err)
	}
	addrs := make([]netip.Addr, 0, len(ifaddrs))
	for _, a := range ifaddrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipnet.IP)
		if !ok {
			continue
		}
		addrs = append(addrs, addr.Unmap())
	}
	return pick(hostname, addrs, exclude), nil
}

func pick(hostname string, addrs []netip.Addr, exclude []netip.Prefix) Discovered {
	d := Discovered{Hostname: hostname}
	for _, addr := range addrs {
		if !addr.IsGlobalUnicast() || addr.IsPrivate() || containsAddr(exclude, addr) {
			continue
		}
		if addr.Is4() && !d.IPv4.IsValid() {
			d.IPv4 = addr
		}
		if addr.Is6() && !d.IPv6.IsValid() {
			d.IPv6 = addr
		}
	}
	return d
}
