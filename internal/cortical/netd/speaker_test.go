package netd

import (
	"context"
	"log/slog"
	"net/netip"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/corvad/warpstack/internal/cortical/bgpd"
	"github.com/corvad/warpstack/internal/cortical/config"
)

func startSpeaker(t *testing.T, addr string) *GoBGP {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	go func() {
		defer close(done)
		bgpd.Run(ctx, logger, addr)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	g, err := Dial(addr, -1)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	deadline := time.Now().Add(5 * time.Second)
	for {
		callCtx, callCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, _, _, err = g.Global(callCtx)
		callCancel()
		if err == nil {
			return g
		}
		if time.Now().After(deadline) {
			t.Fatalf("speaker API never became ready: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestSpeakerGlobalAndPeers(t *testing.T) {
	g := startSpeaker(t, "127.0.0.1:50598")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	started, _, _, err := g.Global(ctx)
	if err != nil {
		t.Fatalf("Global: %v", err)
	}
	if started {
		t.Fatal("fresh speaker reports started")
	}

	routerID := netip.MustParseAddr("198.51.100.10")
	if err := g.Start(ctx, 64500, routerID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	started, asn, gotID, err := g.Global(ctx)
	if err != nil {
		t.Fatalf("Global: %v", err)
	}
	if !started || asn != 64500 || gotID != routerID {
		t.Fatalf("after Start: started=%v asn=%d router_id=%v", started, asn, gotID)
	}

	want := []Peer{
		{Address: netip.MustParseAddr("169.254.169.254"), ASN: 64515, Multihop: 2},
		{Address: netip.MustParseAddr("2001:19f0:ffff::1"), ASN: 64515, Multihop: 2},
	}
	for _, p := range want {
		if err := g.AddPeer(ctx, p); err != nil {
			t.Fatalf("AddPeer: %v", err)
		}
	}
	got, err := g.Peers(ctx)
	if err != nil {
		t.Fatalf("Peers: %v", err)
	}
	got = sortedPeers(got)
	if len(got) != len(want) {
		t.Fatalf("peer count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !samePeer(got[i], want[i]) {
			t.Errorf("peer %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	if err := g.DeletePeer(ctx, want[0].Address); err != nil {
		t.Fatalf("DeletePeer: %v", err)
	}
	got, err = g.Peers(ctx)
	if err != nil {
		t.Fatalf("Peers: %v", err)
	}
	if len(got) != 1 || got[0].Address != want[1].Address {
		t.Errorf("after delete: %+v, want only %s", got, want[1].Address)
	}
}

func TestSpeakerPolicyAndPaths(t *testing.T) {
	g := startSpeaker(t, "127.0.0.1:50597")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := g.Start(ctx, 64500, netip.MustParseAddr("198.51.100.10")); err != nil {
		t.Fatalf("Start: %v", err)
	}

	allowed := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8::/48"),
	}
	ok, err := g.PolicyOK(ctx, allowed)
	if err != nil {
		t.Fatalf("PolicyOK: %v", err)
	}
	if ok {
		t.Fatal("fresh speaker reports policy in place")
	}
	for i := 0; i < 2; i++ {
		if err := g.SetPolicy(ctx, allowed); err != nil {
			t.Fatalf("SetPolicy pass %d: %v", i, err)
		}
		ok, err = g.PolicyOK(ctx, allowed)
		if err != nil {
			t.Fatalf("PolicyOK: %v", err)
		}
		if !ok {
			t.Fatalf("policy not in place after SetPolicy pass %d", i)
		}
	}

	wider := append(slices.Clone(allowed), netip.MustParsePrefix("198.51.100.0/24"))
	ok, err = g.PolicyOK(ctx, wider)
	if err != nil {
		t.Fatalf("PolicyOK: %v", err)
	}
	if ok {
		t.Error("PolicyOK true for a prefix list the speaker does not have")
	}

	for _, p := range allowed {
		if err := g.AddPath(ctx, p, config.Communities{}); err != nil {
			t.Fatalf("AddPath: %v", err)
		}
	}
	got, err := g.Paths(ctx)
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	if got = sortedPrefixes(got); !slices.Equal(got, allowed) {
		t.Fatalf("paths = %v, want %v", got, allowed)
	}

	if err := g.DeletePath(ctx, allowed[0]); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}
	got, err = g.Paths(ctx)
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	if !slices.Equal(got, allowed[1:]) {
		t.Errorf("after delete: paths = %v, want %v", got, allowed[1:])
	}

	o, err := g.Observe(ctx, allowed)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if !o.Started || !o.PolicyOK || len(o.Paths) != 1 || len(o.Peers) != 0 {
		t.Errorf("Observe = %+v", o)
	}
}
