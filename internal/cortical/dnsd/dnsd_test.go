package dnsd

import (
	"context"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"codeberg.org/miekg/dns"
)

const testAddr = "127.0.0.1:53535"

const testZone = `$TTL 300
@   IN SOA ns1.example.org. hostmaster.example.org. 1 3600 600 604800 60
@   IN NS  ns1.example.org.
@   IN NS  ns2.example.org.
@   IN A   192.0.2.10
ns1 IN A   192.0.2.53
ns2 IN A   192.0.2.54
www IN CNAME example.org.
a.b IN TXT "deep"
`

func exchange(network, name string, qtype uint16, edit func(*dns.Msg)) (*dns.Msg, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	m := dns.NewMsg(name, qtype)
	if edit != nil {
		edit(m)
	}
	return dns.Exchange(ctx, m, network, testAddr)
}

func query(t *testing.T, network, name string, qtype uint16) *dns.Msg {
	t.Helper()
	r, err := exchange(network, name, qtype, nil)
	if err != nil {
		t.Fatalf("%s type %d over %s: %v", name, qtype, network, err)
	}
	return r
}

func TestServer(t *testing.T) {
	dir := t.TempDir()
	zonePath := filepath.Join(dir, "example.org.zone")
	if err := os.WriteFile(zonePath, []byte(testZone), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	go func() {
		done <- Run(ctx, logger, Config{Addr: testAddr, ZoneDir: dir, NodeName: "ewr"})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Run did not return after cancel")
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := exchange("udp", "example.org.", dns.TypeSOA, nil); err == nil {
			if _, err := exchange("tcp", "example.org.", dns.TypeSOA, nil); err == nil {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("server never answered on both transports")
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Run("positive answer", func(t *testing.T) {
		for _, network := range []string{"udp", "tcp"} {
			r := query(t, network, "example.org.", dns.TypeA)
			if r.Rcode != dns.RcodeSuccess || !r.Authoritative || len(r.Answer) != 1 {
				t.Errorf("%s: rcode=%d aa=%v answers=%d", network, r.Rcode, r.Authoritative, len(r.Answer))
			}
			if r.RecursionAvailable {
				t.Errorf("%s: recursion available is set", network)
			}
		}
	})

	t.Run("mixed case name", func(t *testing.T) {
		r := query(t, "udp", "ExAmPlE.oRg.", dns.TypeA)
		if r.Rcode != dns.RcodeSuccess || len(r.Answer) != 1 {
			t.Errorf("rcode=%d answers=%d", r.Rcode, len(r.Answer))
		}
	})

	t.Run("cname is followed", func(t *testing.T) {
		r := query(t, "udp", "www.example.org.", dns.TypeA)
		if r.Rcode != dns.RcodeSuccess || len(r.Answer) != 2 {
			t.Fatalf("rcode=%d answers=%d, want CNAME plus A", r.Rcode, len(r.Answer))
		}
		if dns.RRToType(r.Answer[0]) != dns.TypeCNAME || dns.RRToType(r.Answer[1]) != dns.TypeA {
			t.Errorf("answer types = %d, %d", dns.RRToType(r.Answer[0]), dns.RRToType(r.Answer[1]))
		}
	})

	t.Run("no data", func(t *testing.T) {
		r := query(t, "udp", "example.org.", dns.TypeAAAA)
		if r.Rcode != dns.RcodeSuccess || len(r.Answer) != 0 || len(r.Ns) != 1 {
			t.Fatalf("rcode=%d answers=%d authority=%d", r.Rcode, len(r.Answer), len(r.Ns))
		}
		if dns.RRToType(r.Ns[0]) != dns.TypeSOA || r.Ns[0].Header().TTL != 60 {
			t.Errorf("authority = %v, want SOA with TTL 60", r.Ns[0])
		}
	})

	t.Run("empty non-terminal", func(t *testing.T) {
		r := query(t, "udp", "b.example.org.", dns.TypeA)
		if r.Rcode != dns.RcodeSuccess || len(r.Ns) != 1 {
			t.Errorf("rcode=%d authority=%d, want success with SOA", r.Rcode, len(r.Ns))
		}
	})

	t.Run("name error", func(t *testing.T) {
		r := query(t, "udp", "nope.example.org.", dns.TypeA)
		if r.Rcode != dns.RcodeNameError || len(r.Ns) != 1 || !r.Authoritative {
			t.Errorf("rcode=%d authority=%d aa=%v", r.Rcode, len(r.Ns), r.Authoritative)
		}
	})

	t.Run("other zone refused", func(t *testing.T) {
		r := query(t, "udp", "example.com.", dns.TypeA)
		if r.Rcode != dns.RcodeRefused || r.Authoritative {
			t.Errorf("rcode=%d aa=%v", r.Rcode, r.Authoritative)
		}
	})

	t.Run("nsid", func(t *testing.T) {
		r, err := exchange("udp", "example.org.", dns.TypeA, func(m *dns.Msg) {
			m.UDPSize = 1232
			m.Pseudo = append(m.Pseudo, &dns.NSID{})
		})
		if err != nil {
			t.Fatal(err)
		}
		want := hex.EncodeToString([]byte("ewr"))
		found := false
		for _, rr := range r.Pseudo {
			if n, ok := rr.(*dns.NSID); ok && n.Nsid == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no NSID %q in reply pseudo section: %v", want, r.Pseudo)
		}
	})

	t.Run("bad reload keeps old zones", func(t *testing.T) {
		if err := os.WriteFile(zonePath, []byte("this is not a zone file\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		syscall.Kill(os.Getpid(), syscall.SIGHUP)
		time.Sleep(300 * time.Millisecond)
		r := query(t, "udp", "example.org.", dns.TypeA)
		if r.Rcode != dns.RcodeSuccess || len(r.Answer) != 1 {
			t.Errorf("after bad reload: rcode=%d answers=%d", r.Rcode, len(r.Answer))
		}
	})

	t.Run("good reload takes effect", func(t *testing.T) {
		updated := testZone + "new IN A 192.0.2.99\n"
		if err := os.WriteFile(zonePath, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
		syscall.Kill(os.Getpid(), syscall.SIGHUP)
		deadline := time.Now().Add(3 * time.Second)
		for {
			r := query(t, "udp", "new.example.org.", dns.TypeA)
			if r.Rcode == dns.RcodeSuccess && len(r.Answer) == 1 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("new record never appeared: rcode=%d", r.Rcode)
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}
