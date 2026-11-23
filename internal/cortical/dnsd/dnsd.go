package dnsd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/miekg/dns"
)

type Config struct {
	Addr     string
	ZoneDir  string
	NodeName string
}

func Run(ctx context.Context, logger *slog.Logger, cfg Config) error {
	h, err := NewHandler(cfg.ZoneDir, cfg.NodeName)
	if err != nil {
		return fmt.Errorf("load zones: %w", err)
	}
	udp := &dns.Server{Addr: cfg.Addr, Net: "udp", Handler: h}
	tcp := &dns.Server{Addr: cfg.Addr, Net: "tcp", Handler: h}
	errc := make(chan error, 2)
	go func() { errc <- udp.ListenAndServe() }()
	go func() { errc <- tcp.ListenAndServe() }()

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	for {
		select {
		case <-ctx.Done():
			udp.Shutdown(context.Background())
			tcp.Shutdown(context.Background())
			return nil
		case err := <-errc:
			return fmt.Errorf("dns server stopped: %w", err)
		case <-hup:
			if err := h.Reload(); err != nil {
				logger.Error("zone reload failed, keeping previous zones", "error", err)
				continue
			}
			logger.Info("zones reloaded")
		}
	}
}
