package dnsd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
)

type Zone struct {
	Origin   string
	negative dns.RR
	sets     map[string]map[uint16][]dns.RR
	exists   map[string]bool
}

func (z *Zone) contains(name string) bool {
	return name == z.Origin || dnsutil.IsBelow(z.Origin, name)
}

func (z *Zone) lookup(name string) (map[uint16][]dns.RR, bool) {
	sets := z.sets[name]
	return sets, sets != nil || z.exists[name]
}

func parent(name string) string {
	i := strings.IndexByte(name, '.')
	if i < 0 || i+1 >= len(name) {
		return "."
	}
	return name[i+1:]
}

func LoadZone(origin string, r io.Reader, file string) (*Zone, error) {
	origin = dnsutil.Canonical(origin)
	z := &Zone{
		Origin: origin,
		sets:   make(map[string]map[uint16][]dns.RR),
		exists: make(map[string]bool),
	}
	var soa *dns.SOA
	zp := dns.NewZoneParser(r, origin, file)
	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		h := rr.Header()
		name := dnsutil.Canonical(h.Name)
		if !z.contains(name) {
			return nil, fmt.Errorf("%s: record %q is outside the zone", origin, h.Name)
		}
		if h.Class != dns.ClassINET {
			return nil, fmt.Errorf("%s: record %q has a class other than IN", origin, h.Name)
		}
		h.Name = name
		t := dns.RRToType(rr)
		if t == dns.TypeSOA {
			s, ok := rr.(*dns.SOA)
			if !ok || name != origin || soa != nil {
				return nil, fmt.Errorf("%s: SOA must appear exactly once, at the apex", origin)
			}
			soa = s
		}
		if z.sets[name] == nil {
			z.sets[name] = make(map[uint16][]dns.RR)
		}
		z.sets[name][t] = append(z.sets[name][t], rr)
		for n := name; ; n = parent(n) {
			z.exists[n] = true
			if n == origin || n == "." {
				break
			}
		}
	}
	if err := zp.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	if soa == nil {
		return nil, fmt.Errorf("%s: no SOA record", origin)
	}
	if len(z.sets[origin][dns.TypeNS]) == 0 {
		return nil, fmt.Errorf("%s: no NS records at the apex", origin)
	}
	for name, sets := range z.sets {
		if cnames := sets[dns.TypeCNAME]; len(cnames) > 0 && (len(cnames) > 1 || len(sets) > 1) {
			return nil, fmt.Errorf("%s: %s has a CNAME alongside other records", origin, name)
		}
	}
	negative, ok := soa.Clone().(*dns.SOA)
	if !ok {
		return nil, fmt.Errorf("%s: cannot clone SOA", origin)
	}
	if negative.Minttl < negative.Hdr.TTL {
		negative.Hdr.TTL = negative.Minttl
	}
	z.negative = negative
	return z, nil
}

type Store struct {
	zones []*Zone
}

func (s *Store) find(name string) *Zone {
	if s == nil {
		return nil
	}
	for _, z := range s.zones {
		if z.contains(name) {
			return z
		}
	}
	return nil
}

func LoadDir(dir string) (*Store, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read zone directory: %w", err)
	}
	s := &Store{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zone") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open zone: %w", err)
		}
		z, err := LoadZone(strings.TrimSuffix(e.Name(), ".zone"), f, path)
		f.Close()
		if err != nil {
			return nil, err
		}
		s.zones = append(s.zones, z)
	}
	if len(s.zones) == 0 {
		return nil, errors.New("no .zone files found in " + dir)
	}
	slices.SortFunc(s.zones, func(a, b *Zone) int {
		return dnsutil.Labels(b.Origin) - dnsutil.Labels(a.Origin)
	})
	return s, nil
}
