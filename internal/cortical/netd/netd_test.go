package netd

import (
	"context"
	"log/slog"
	"net/netip"
	"os"
	"testing"
	"time"
)

func TestSyncConverges(t *testing.T) {
	g := startSpeaker(t, "127.0.0.1:50595")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d := desired()
	for i := range d.Peers {
		d.Peers[i].Local = netip.Addr{}
		d.Peers[i].Password = ""
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	n := New(g, func() (Desired, error) { return d, nil }, time.Hour, logger)

	check := func(step string) {
		t.Helper()
		if err := n.Sync(ctx); err != nil {
			t.Fatalf("%s: Sync: %v", step, err)
		}
		o, err := g.Observe(ctx, d.Allowed)
		if err != nil {
			t.Fatalf("%s: Observe: %v", step, err)
		}
		if left := Reconcile(d, o); len(left) != 0 {
			t.Fatalf("%s: not converged, remaining: %v", step, names(left))
		}
	}

	check("first sync")
	check("second sync")

	d.Paths = nil
	check("after withdraw")
	o, err := g.Observe(ctx, d.Allowed)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(o.Paths) != 0 {
		t.Errorf("paths after withdraw = %v, want none", o.Paths)
	}
}
