package bgpd

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	api "github.com/osrg/gobgp/v4/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func TestRunExitsWhenSpeakerStopped(t *testing.T) {
	const addr = "127.0.0.1:50594"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, logger, addr)
	}()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := api.NewGoBgpServiceClient(conn)

	deadline := time.Now().Add(5 * time.Second)
	for {
		callCtx, callCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		_, err = client.GetBgp(callCtx, &api.GetBgpRequest{})
		callCancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("API never became ready: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	_, err = client.StartBgp(ctx, &api.StartBgpRequest{
		Global: &api.Global{Asn: 64500, RouterId: "198.51.100.10", ListenPort: -1},
	})
	if err != nil {
		t.Fatalf("StartBgp: %v", err)
	}
	if _, err := client.StopBgp(ctx, &api.StopBgpRequest{}); err != nil {
		t.Fatalf("StopBgp: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Error("Run returned nil after the speaker stopped, want an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run kept running after the speaker stopped")
	}
}
