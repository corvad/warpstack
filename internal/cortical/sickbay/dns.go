package sickbay

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"codeberg.org/miekg/dns"
)

type DNS struct {
	addr      string
	interval  time.Duration
	threshold int
	logger    *slog.Logger
	healthy   atomic.Bool
}

func NewDNS(addr string, interval time.Duration, threshold int, logger *slog.Logger) *DNS {
	return &DNS{addr: addr, interval: interval, threshold: threshold, logger: logger}
}

func (d *DNS) Name() string {
	return "sickbay"
}

func (d *DNS) Init(ctx context.Context) error {
	return nil
}

func (d *DNS) Healthy() bool {
	return d.healthy.Load()
}

func (d *DNS) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	failures := 0
	for {
		if d.probe(ctx) {
			failures = 0
			if !d.healthy.Swap(true) {
				d.logger.Info("dns is answering", "addr", d.addr)
			}
		} else {
			failures++
			if failures >= d.threshold && d.healthy.Swap(false) {
				d.logger.Warn("dns stopped answering", "addr", d.addr, "failures", failures)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *DNS) probe(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	m := dns.NewMsg(".", dns.TypeSOA)
	m.RecursionDesired = false
	_, err := dns.Exchange(ctx, m, "udp", d.addr)
	return err == nil
}
