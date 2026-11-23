package bgpd

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"
)

func TestRunServesAPIAndStops(t *testing.T) {
	const addr = "127.0.0.1:50599"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, logger, addr)
	}()

	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("API never accepted connections on %s: %v", addr, err)
	}
	conn.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
