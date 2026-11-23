package secrets

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Secrets map[string]string

func Parse(r io.Reader) (Secrets, error) {
	s := Secrets{}
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		name, value, ok := strings.Cut(text, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("secrets line %d: want name=value", line)
		}
		if _, dup := s[name]; dup {
			return nil, fmt.Errorf("secrets line %d: %q defined twice", line, name)
		}
		s[name] = value
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read secrets: %w", err)
	}
	return s, nil
}

func Load(path string) (Secrets, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open secrets: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat secrets: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("secrets file %s is readable by group or others (mode %o)", path, info.Mode().Perm())
	}
	return Parse(f)
}

func Save(path string, s Secrets) error {
	names := make([]string, 0, len(s))
	for name := range s {
		names = append(names, name)
	}
	slices.Sort(names)
	var b strings.Builder
	for _, name := range names {
		if strings.ContainsAny(name, "=\n") || strings.Contains(s[name], "\n") {
			return fmt.Errorf("secret %q contains a character that cannot be stored", name)
		}
		fmt.Fprintf(&b, "%s=%s\n", name, s[name])
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod secrets: %w", err)
	}
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return fmt.Errorf("write secrets: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync secrets: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close secrets: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename secrets: %w", err)
	}
	return nil
}

func (s Secrets) Get(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	value, ok := s[name]
	if !ok {
		return "", fmt.Errorf("secret %q is not defined", name)
	}
	return value, nil
}
