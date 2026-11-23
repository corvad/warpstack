package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/corvad/warpstack/internal/cortical/addrs"
	"github.com/corvad/warpstack/internal/cortical/bgpd"
	"github.com/corvad/warpstack/internal/cortical/config"
	"github.com/corvad/warpstack/internal/cortical/dnsd"
	"github.com/corvad/warpstack/internal/cortical/install"
	"github.com/corvad/warpstack/internal/cortical/join"
	"github.com/corvad/warpstack/internal/cortical/netd"
	"github.com/corvad/warpstack/internal/cortical/secrets"
	"github.com/corvad/warpstack/internal/cortical/sickbay"
	"github.com/corvad/warpstack/internal/cortical/supervisor"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: corticald <run|bgpd|dnsd|join> [flags]")
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var err error
	switch os.Args[1] {
	case "run":
		err = runAgent(ctx, logger)
	case "bgpd":
		err = bgpd.Run(ctx, logger, bgpd.DefaultAPIAddress)
	case "dnsd":
		err = runDNS(ctx, logger)
	case "join":
		err = runJoin(ctx, os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		logger.Error("corticald failed", "command", os.Args[1], "error", err)
		stop()
		os.Exit(1)
	}
}

func runJoin(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("join", flag.ContinueOnError)
	var opts join.Options
	fs.StringVar(&opts.Config, "config", "", "cluster config: a file path or an https URL")
	fs.StringVar(&opts.Provider, "provider", "", "provider name from the cluster config")
	fs.StringVar(&opts.Zones, "zones", "", "directory of .zone files to install")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return join.Run(ctx, opts, os.Stdin)
}

func runDNS(ctx context.Context, logger *slog.Logger) error {
	node, err := config.Load(install.NodeConfigPath)
	if err != nil {
		return err
	}
	return dnsd.Run(ctx, logger, dnsd.Config{
		Addr:     ":53",
		ZoneDir:  install.ZoneDir,
		NodeName: node.GetName(),
	})
}

func runAgent(ctx context.Context, logger *slog.Logger) error {
	node, err := config.Load(install.NodeConfigPath)
	if err != nil {
		return err
	}
	sec, err := secrets.Load(install.SecretsPath)
	if err != nil {
		return err
	}
	password, err := sec.Get(node.GetBgpPasswordSecret())
	if err != nil {
		return err
	}

	var managed []netip.Prefix
	for _, s := range node.GetCluster().GetAnnounce() {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return fmt.Errorf("announce %q: %w", s, err)
		}
		managed = append(managed, p)
	}
	var service []netip.Addr
	for _, svc := range node.GetServices() {
		for _, s := range svc.GetAddresses() {
			a, err := netip.ParseAddr(s)
			if err != nil {
				return fmt.Errorf("service %q address %q: %w", svc.GetName(), s, err)
			}
			service = append(service, a)
		}
	}

	speaker, err := netd.Dial(bgpd.DefaultAPIAddress, 179)
	if err != nil {
		return err
	}
	defer speaker.Close()

	health := sickbay.NewDNS("127.0.0.1:53", 2*time.Second, 3, logger)
	addresses := addrs.New("lo", func() ([]netip.Addr, []netip.Prefix, error) {
		return service, managed, nil
	}, 15*time.Second, logger)
	bgp := netd.New(speaker, func() (netd.Desired, error) {
		return netd.Plan(node, password, health.Healthy())
	}, 5*time.Second, logger)

	opts := supervisor.Options{
		MinBackoff:        time.Second,
		MaxBackoff:        30 * time.Second,
		StableBackoff:     time.Minute,
		BackoffMultiplier: 2,
	}
	s, err := supervisor.New([]supervisor.Spec{
		{Subsystem: addresses, Options: opts},
		{Subsystem: health, Options: opts},
		{Subsystem: bgp, Options: opts},
	})
	if err != nil {
		return err
	}
	if err := s.Init(ctx); err != nil {
		return err
	}
	return s.Run(ctx)
}
