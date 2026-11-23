package addrs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

type DesiredFunc func() (want []netip.Addr, managed []netip.Prefix, err error)

type Manager struct {
	device   string
	desired  DesiredFunc
	interval time.Duration
	logger   *slog.Logger
}

func New(device string, desired DesiredFunc, interval time.Duration, logger *slog.Logger) *Manager {
	return &Manager{device: device, desired: desired, interval: interval, logger: logger}
}

func (m *Manager) Name() string {
	return "addrs"
}

func (m *Manager) Init(ctx context.Context) error {
	return nil
}

func (m *Manager) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		if err := m.Sync(ctx); err != nil && ctx.Err() == nil {
			m.logger.Warn("addrs sync failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Manager) Sync(ctx context.Context) error {
	want, managed, err := m.desired()
	if err != nil {
		return fmt.Errorf("desired addresses: %w", err)
	}
	out, err := exec.CommandContext(ctx, "ip", "-j", "addr", "show", "dev", m.device).Output()
	if err != nil {
		return fmt.Errorf("list addresses on %s: %w", m.device, err)
	}
	have, err := ParseAddrs(out)
	if err != nil {
		return err
	}
	add, del := Diff(want, have, managed)
	for _, a := range del {
		if err := m.ip(ctx, "del", a); err != nil {
			return err
		}
	}
	for _, a := range add {
		if err := m.ip(ctx, "add", a); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) ip(ctx context.Context, verb string, a netip.Addr) error {
	cidr := a.String() + "/" + strconv.Itoa(a.BitLen())
	out, err := exec.CommandContext(ctx, "ip", "addr", verb, cidr, "dev", m.device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip addr %s %s dev %s: %w: %s", verb, cidr, m.device, err, strings.TrimSpace(string(out)))
	}
	m.logger.Info("addrs applied", "action", verb, "address", cidr, "device", m.device)
	return nil
}

func ParseAddrs(data []byte) ([]netip.Addr, error) {
	var links []struct {
		AddrInfo []struct {
			Local string `json:"local"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(data, &links); err != nil {
		return nil, fmt.Errorf("parse ip output: %w", err)
	}
	var out []netip.Addr
	for _, l := range links {
		for _, info := range l.AddrInfo {
			if a, err := netip.ParseAddr(info.Local); err == nil {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

func Diff(want, have []netip.Addr, managed []netip.Prefix) (add, del []netip.Addr) {
	wantSet := make(map[netip.Addr]bool, len(want))
	for _, a := range want {
		wantSet[a] = true
	}
	haveSet := make(map[netip.Addr]bool, len(have))
	for _, a := range have {
		haveSet[a] = true
		if wantSet[a] {
			continue
		}
		for _, p := range managed {
			if p.Contains(a) {
				del = append(del, a)
				break
			}
		}
	}
	for _, a := range want {
		if !haveSet[a] {
			add = append(add, a)
		}
	}
	compare := func(a, b netip.Addr) int { return a.Compare(b) }
	slices.SortFunc(add, compare)
	slices.SortFunc(del, compare)
	return add, del
}
