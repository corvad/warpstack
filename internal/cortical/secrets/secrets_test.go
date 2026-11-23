package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	s, err := Parse(strings.NewReader("# comment\n\nvultr_bgp=p@ss=word\nother = spaced \n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := s["vultr_bgp"]; got != "p@ss=word" {
		t.Errorf("vultr_bgp = %q", got)
	}
	if _, ok := s["other"]; !ok {
		t.Errorf("other missing: %v", s)
	}
	for _, bad := range []string{"novalue\n", "=x\n", "a=1\na=2\n"} {
		if _, err := Parse(strings.NewReader(bad)); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", bad)
		}
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets")
	want := Secrets{"vultr_bgp": "hunter2", "b": "two"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(want) || got["vultr_bgp"] != "hunter2" || got["b"] != "two" {
		t.Errorf("round trip = %v, want %v", got, want)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted a world-readable secrets file")
	}

	if v, err := got.Get(""); err != nil || v != "" {
		t.Errorf("Get(\"\") = %q, %v", v, err)
	}
	if _, err := got.Get("missing"); err == nil {
		t.Error("Get(missing) succeeded")
	}
}
