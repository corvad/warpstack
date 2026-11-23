package config

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type Large struct {
	Global uint32
	Local1 uint32
	Local2 uint32
}

type Communities struct {
	Standard []uint32
	Large    []Large
}

func NewCommunities(standard []uint32, large []Large) Communities {
	c := Communities{Standard: slices.Clone(standard), Large: slices.Clone(large)}
	slices.Sort(c.Standard)
	c.Standard = slices.Compact(c.Standard)
	slices.SortFunc(c.Large, func(a, b Large) int {
		return cmp.Or(
			cmp.Compare(a.Global, b.Global),
			cmp.Compare(a.Local1, b.Local1),
			cmp.Compare(a.Local2, b.Local2),
		)
	})
	c.Large = slices.Compact(c.Large)
	return c
}

func ParseCommunities(in []string) (Communities, error) {
	var standard []uint32
	var large []Large
	for _, s := range in {
		parts := strings.Split(s, ":")
		bits := 16
		if len(parts) == 3 {
			bits = 32
		} else if len(parts) != 2 {
			return Communities{}, fmt.Errorf("community %q: want a:b or a:b:c", s)
		}
		values := make([]uint32, len(parts))
		for i, part := range parts {
			v, err := strconv.ParseUint(part, 10, bits)
			if err != nil {
				return Communities{}, fmt.Errorf("community %q: %w", s, err)
			}
			values[i] = uint32(v)
		}
		if len(values) == 2 {
			standard = append(standard, values[0]<<16|values[1])
		} else {
			large = append(large, Large{Global: values[0], Local1: values[1], Local2: values[2]})
		}
	}
	return NewCommunities(standard, large), nil
}

func (c Communities) Empty() bool {
	return len(c.Standard) == 0 && len(c.Large) == 0
}

func (c Communities) Key() string {
	parts := make([]string, 0, len(c.Standard)+len(c.Large))
	for _, v := range c.Standard {
		parts = append(parts, fmt.Sprintf("%d:%d", v>>16, v&0xffff))
	}
	for _, l := range c.Large {
		parts = append(parts, fmt.Sprintf("%d:%d:%d", l.Global, l.Local1, l.Local2))
	}
	return strings.Join(parts, " ")
}
