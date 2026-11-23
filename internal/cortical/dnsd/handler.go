package dnsd

import (
	"context"
	"encoding/hex"
	"io"
	"sync/atomic"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
)

const (
	serverUDPSize = 1232
	plainUDPSize  = 512
	maxCNAMEHops  = 8
)

type Handler struct {
	dir      string
	nodeName string
	store    atomic.Pointer[Store]
}

func NewHandler(dir, nodeName string) (*Handler, error) {
	h := &Handler{dir: dir, nodeName: nodeName}
	if err := h.Reload(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Handler) Reload() error {
	s, err := LoadDir(h.dir)
	if err != nil {
		return err
	}
	h.store.Store(s)
	return nil
}

func (h *Handler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
	if err := r.Unpack(); err != nil {
		return
	}
	if len(r.Question) != 1 {
		return
	}
	qname, qtype := dnsutil.Question(r)
	edns := r.UDPSize > 0
	clientSize := int(r.UDPSize)
	wantNSID := false
	for _, rr := range r.Pseudo {
		if _, ok := rr.(*dns.NSID); ok {
			wantNSID = true
		}
	}

	dnsutil.SetReply(r)
	r.Answer, r.Ns, r.Extra, r.Pseudo = nil, nil, nil, nil
	r.RecursionAvailable = false
	r.AuthenticatedData = false
	r.Security = false
	r.UDPSize = 0
	if edns {
		r.UDPSize = serverUDPSize
		if wantNSID {
			r.Pseudo = []dns.RR{&dns.NSID{Nsid: hex.EncodeToString([]byte(h.nodeName))}}
		}
	}

	h.answer(r, dnsutil.Canonical(qname), qtype)

	if err := r.Pack(); err != nil {
		return
	}
	if w.RemoteAddr().Network() == "udp" {
		limit := plainUDPSize
		if edns {
			limit = max(plainUDPSize, min(clientSize, serverUDPSize))
		}
		if len(r.Data) > limit {
			r.Answer, r.Ns, r.Extra = nil, nil, nil
			r.Truncated = true
			if err := r.Pack(); err != nil {
				return
			}
		}
	}
	io.Copy(w, r)
}

func (h *Handler) answer(m *dns.Msg, name string, qtype uint16) {
	zone := h.store.Load().find(name)
	if zone == nil {
		m.Rcode = dns.RcodeRefused
		return
	}
	m.Authoritative = true
	for hop := 0; hop < maxCNAMEHops; hop++ {
		sets, exists := zone.lookup(name)
		if qtype == dns.TypeANY && len(sets) > 0 {
			for _, rrs := range sets {
				m.Answer = append(m.Answer, rrs...)
			}
			return
		}
		if rrs := sets[qtype]; len(rrs) > 0 {
			m.Answer = append(m.Answer, rrs...)
			return
		}
		if cnames := sets[dns.TypeCNAME]; len(cnames) > 0 {
			m.Answer = append(m.Answer, cnames...)
			cname, ok := cnames[0].(*dns.CNAME)
			if !ok {
				return
			}
			target := dnsutil.Canonical(cname.Target)
			if !zone.contains(target) {
				return
			}
			name = target
			continue
		}
		if len(m.Answer) > 0 {
			return
		}
		if !exists {
			m.Rcode = dns.RcodeNameError
		}
		m.Ns = []dns.RR{zone.negative}
		return
	}
}
