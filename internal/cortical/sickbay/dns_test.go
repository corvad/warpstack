package sickbay

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/corvad/warpstack/internal/cortical/dnsd"
)

const zone = `$TTL 300
@ IN SOA ns1.example.org. hostmaster.example.org. 1 3600 600 604800 60
@ IN NS  ns1.example.org.
ns1 IN A 192.0.2.53
`

func waitFor(t *testing.T, want bool, d *DNS, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for d.Healthy() != want {
		if time.Now().After(deadline) {
			t.Fatalf("healthy never became %v", want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestDNSHealth(t *testing.T) {
	const addr = "127.0.0.1:53536"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "example.org.zone"), []byte(zone), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	checkCtx, checkCancel := context.WithCancel(context.Background())
	defer checkCancel()
	d := NewDNS(addr, 100*time.Millisecond, 3, logger)
	go d.Run(checkCtx)

	time.Sleep(300 * time.Millisecond)
	if d.Healthy() {
		t.Fatal("healthy before any server was running")
	}

	srvCtx, srvCancel := context.WithCancel(context.Background())
	srvDone := make(chan error, 1)
	go func() {
		srvDone <- dnsd.Run(srvCtx, logger, dnsd.Config{Addr: addr, ZoneDir: dir, NodeName: "test"})
	}()
	waitFor(t, true, d, 5*time.Second)

	srvCancel()
	<-srvDone
	waitFor(t, false, d, 10*time.Second)
}
