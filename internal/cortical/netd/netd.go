package netd

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"
)

type Speaker interface {
	Observe(ctx context.Context, allowed []netip.Prefix) (Observed, error)
	Start(ctx context.Context, asn uint32, routerID netip.Addr) error
	Stop(ctx context.Context) error
	SetPolicy(ctx context.Context, allowed []netip.Prefix) error
	AddPeer(ctx context.Context, p Peer) error
	DeletePeer(ctx context.Context, address netip.Addr) error
	AddPath(ctx context.Context, p netip.Prefix) error
	DeletePath(ctx context.Context, p netip.Prefix) error
}

var _ Speaker = (*GoBGP)(nil)

type DesiredFunc func() (Desired, error)

type Netd struct {
	speaker  Speaker
	desired  DesiredFunc
	interval time.Duration
	logger   *slog.Logger
}

func New(speaker Speaker, desired DesiredFunc, interval time.Duration, logger *slog.Logger) *Netd {
	return &Netd{speaker: speaker, desired: desired, interval: interval, logger: logger}
}

func (n *Netd) Name() string {
	return "netd"
}

func (n *Netd) Init(ctx context.Context) error {
	return nil
}

func (n *Netd) Run(ctx context.Context) error {
	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()
	for {
		if err := n.Sync(ctx); err != nil && ctx.Err() == nil {
			n.logger.Warn("netd sync failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (n *Netd) Sync(ctx context.Context) error {
	d, err := n.desired()
	if err != nil {
		return fmt.Errorf("desired state: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	o, err := n.speaker.Observe(ctx, d.Allowed)
	if err != nil {
		return fmt.Errorf("observe: %w", err)
	}
	for _, a := range Reconcile(d, o) {
		n.logger.Info("netd applying", "action", a.String())
		if err := n.apply(ctx, d, a); err != nil {
			return fmt.Errorf("apply %s: %w", a, err)
		}
	}
	return nil
}

func (n *Netd) apply(ctx context.Context, d Desired, a Action) error {
	switch a.Kind {
	case ActionStop:
		return n.speaker.Stop(ctx)
	case ActionStart:
		return n.speaker.Start(ctx, d.ASN, d.RouterID)
	case ActionSetPolicy:
		return n.speaker.SetPolicy(ctx, d.Allowed)
	case ActionDeletePeer:
		return n.speaker.DeletePeer(ctx, a.Peer.Address)
	case ActionAddPeer:
		return n.speaker.AddPeer(ctx, a.Peer)
	case ActionDeletePath:
		return n.speaker.DeletePath(ctx, a.Prefix)
	case ActionAddPath:
		return n.speaker.AddPath(ctx, a.Prefix)
	}
	return fmt.Errorf("unknown action kind %d", a.Kind)
}
