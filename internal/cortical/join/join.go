package join

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corvad/warpstack/internal/cortical/config"
	"github.com/corvad/warpstack/internal/cortical/dnsd"
	"github.com/corvad/warpstack/internal/cortical/install"
	"github.com/corvad/warpstack/internal/cortical/secrets"
)

type Options struct {
	Config   string
	Provider string
	Zones    string
}

func readConfig(ctx context.Context, source string) ([]byte, error) {
	if !strings.HasPrefix(source, "https://") {
		return os.ReadFile(source)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", source, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func copyZones(src, dst string) error {
	if _, err := dnsd.LoadDir(src); err != nil {
		return fmt.Errorf("zones in %s are not valid: %w", src, err)
	}
	old, err := filepath.Glob(filepath.Join(dst, "*.zone"))
	if err != nil {
		return err
	}
	for _, path := range old {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove old zone: %w", err)
		}
	}
	files, err := filepath.Glob(filepath.Join(src, "*.zone"))
	if err != nil {
		return err
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read zone: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dst, filepath.Base(path)), data, 0o644); err != nil {
			return fmt.Errorf("write zone: %w", err)
		}
	}
	return nil
}

func Run(ctx context.Context, opts Options, secretsIn io.Reader) error {
	if os.Geteuid() != 0 {
		return errors.New("join must run as root")
	}
	if opts.Config == "" || opts.Provider == "" || opts.Zones == "" {
		return errors.New("join needs --config, --provider and --zones")
	}

	data, err := readConfig(ctx, opts.Config)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	cluster, err := config.ParseConfig(data)
	if err != nil {
		return err
	}
	sec, err := secrets.Parse(secretsIn)
	if err != nil {
		return err
	}

	var exclude []netip.Prefix
	for _, s := range cluster.GetCluster().GetAnnounce() {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return fmt.Errorf("announce %q: %w", s, err)
		}
		exclude = append(exclude, p)
	}
	discovered, err := config.Discover(exclude)
	if err != nil {
		return err
	}
	node, err := config.FromCluster(cluster, opts.Provider, discovered)
	if err != nil {
		return err
	}
	if _, err := sec.Get(node.GetBgpPasswordSecret()); err != nil {
		return fmt.Errorf("provider %q: %w", opts.Provider, err)
	}

	var neighbors []netip.Addr
	for _, n := range node.GetNeighbors() {
		addr, err := netip.ParseAddr(n.GetAddress())
		if err != nil {
			return fmt.Errorf("neighbor %q: %w", n.GetAddress(), err)
		}
		neighbors = append(neighbors, addr)
	}

	if err := install.Dirs(); err != nil {
		return err
	}
	if err := config.Save(install.NodeConfigPath, node); err != nil {
		return err
	}
	if err := secrets.Save(install.SecretsPath, sec); err != nil {
		return err
	}
	if err := copyZones(opts.Zones, install.ZoneDir); err != nil {
		return err
	}
	if err := install.Binary(); err != nil {
		return err
	}
	if err := install.Firewall(neighbors); err != nil {
		return err
	}
	if err := install.Units(); err != nil {
		return err
	}
	fmt.Printf("joined as %s (%s): ipv4 %s ipv6 %s\n", node.GetName(), opts.Provider, node.GetIpv4(), node.GetIpv6())
	return nil
}
