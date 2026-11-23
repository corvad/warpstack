package netd

import (
	"context"
	"net/netip"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/prototext"

	"github.com/corvad/warpstack/internal/cortical/config"
	warpstackv1 "github.com/corvad/warpstack/proto/warpstack/v1"
)

func TestReconcileCommunities(t *testing.T) {
	c, err := config.ParseCommunities([]string{"64600:4637"})
	if err != nil {
		t.Fatalf("ParseCommunities: %v", err)
	}
	d := desired()
	d.Communities = c

	untagged := converged()
	want := []string{"add-path 192.0.2.0/24", "add-path 2001:db8::/48"}
	if got := names(Reconcile(d, untagged)); !slices.Equal(got, want) {
		t.Errorf("untagged paths: got %v, want %v", got, want)
	}

	tagged := converged()
	tagged.Tags = map[netip.Prefix]string{}
	for _, p := range tagged.Paths {
		tagged.Tags[p] = c.Key()
	}
	if got := names(Reconcile(d, tagged)); len(got) != 0 {
		t.Errorf("tagged paths: got %v, want none", got)
	}
	if got := names(Reconcile(desired(), tagged)); !slices.Equal(got, want) {
		t.Errorf("communities removed: got %v, want %v", got, want)
	}
}

func TestPlanCommunities(t *testing.T) {
	n := &warpstackv1.NodeConfig{}
	if err := prototext.Unmarshal([]byte(node), n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	n.SetCommunities([]string{"64600:4637", "20473:6000:3257"})
	d, err := Plan(n, "secret", true)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if got, want := d.Communities.Key(), "64600:4637 20473:6000:3257"; got != want {
		t.Errorf("communities = %q, want %q", got, want)
	}
	n.SetCommunities([]string{"64600:99999"})
	if _, err := Plan(n, "secret", true); err == nil {
		t.Error("Plan succeeded with an invalid community")
	}
}

func TestSpeakerCommunities(t *testing.T) {
	g := startSpeaker(t, "127.0.0.1:50594")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := g.Start(ctx, 64500, netip.MustParseAddr("198.51.100.10")); err != nil {
		t.Fatalf("Start: %v", err)
	}
	allowed := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8::/48"),
	}
	if err := g.SetPolicy(ctx, allowed); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	for _, list := range [][]string{
		{"64600:4637", "20473:6000:3257"},
		{"64600:3257"},
		nil,
	} {
		c, err := config.ParseCommunities(list)
		if err != nil {
			t.Fatalf("ParseCommunities: %v", err)
		}
		for _, p := range allowed {
			if err := g.AddPath(ctx, p, c); err != nil {
				t.Fatalf("AddPath %s %v: %v", p, list, err)
			}
		}
		o, err := g.Observe(ctx, allowed)
		if err != nil {
			t.Fatalf("Observe: %v", err)
		}
		if got := sortedPrefixes(o.Paths); !slices.Equal(got, allowed) {
			t.Fatalf("%v: paths = %v, want %v", list, got, allowed)
		}
		for _, p := range allowed {
			if o.Tags[p] != c.Key() {
				t.Errorf("%v: tag for %s = %q, want %q", list, p, o.Tags[p], c.Key())
			}
		}
	}
}
