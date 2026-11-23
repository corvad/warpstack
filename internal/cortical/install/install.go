package install

import (
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	BinaryPath     = "/usr/local/bin/corticald"
	ConfigDir      = "/etc/warpstack"
	NodeConfigPath = ConfigDir + "/node.txtpb"
	SecretsPath    = ConfigDir + "/secrets"
	StateDir       = "/var/lib/warpstack"
	ZoneDir        = StateDir + "/zones"
	unitDir        = "/etc/systemd/system"
)

var unitOrder = []string{"warpstack-bgpd.service", "warpstack-dnsd.service", "corticald.service"}

var units = map[string]string{
	"warpstack-bgpd.service": `[Unit]
Description=warpstack BGP speaker
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/corticald bgpd
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
	"warpstack-dnsd.service": `[Unit]
Description=warpstack authoritative DNS
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/corticald dnsd
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
	"corticald.service": `[Unit]
Description=warpstack node agent
After=network-online.target warpstack-bgpd.service warpstack-dnsd.service
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/corticald run
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Dirs() error {
	if err := os.MkdirAll(ConfigDir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", ConfigDir, err)
	}
	if err := os.MkdirAll(ZoneDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", ZoneDir, err)
	}
	return nil
}

func Binary() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate running binary: %w", err)
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return fmt.Errorf("resolve running binary: %w", err)
	}
	if self == BinaryPath {
		return nil
	}
	src, err := os.Open(self)
	if err != nil {
		return fmt.Errorf("open running binary: %w", err)
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(BinaryPath), ".corticald-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close binary: %w", err)
	}
	if err := os.Rename(tmp.Name(), BinaryPath); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

func Firewall(neighbors []netip.Addr) error {
	if exec.Command("systemctl", "is-active", "--quiet", "firewalld").Run() != nil {
		return nil
	}
	if err := run("firewall-cmd", "--permanent", "--add-service=dns"); err != nil {
		return err
	}
	for _, n := range neighbors {
		family := "ipv4"
		if n.Is6() {
			family = "ipv6"
		}
		rule := fmt.Sprintf(`rule family="%s" source address="%s" port port="179" protocol="tcp" accept`, family, n)
		if err := run("firewall-cmd", "--permanent", "--add-rich-rule="+rule); err != nil {
			return err
		}
	}
	return run("firewall-cmd", "--reload")
}

func Units() error {
	for _, name := range unitOrder {
		if err := os.WriteFile(filepath.Join(unitDir, name), []byte(units[name]), 0o644); err != nil {
			return fmt.Errorf("write unit %s: %w", name, err)
		}
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	for _, name := range unitOrder {
		if err := run("systemctl", "enable", name); err != nil {
			return err
		}
		if err := run("systemctl", "restart", name); err != nil {
			return err
		}
	}
	return nil
}
