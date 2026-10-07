package dns

import "strconv"

type Class uint16

const (
	ClassIN   Class = 1
	ClassCH   Class = 3
	ClassHS   Class = 4
	ClassNONE Class = 254
	ClassANY  Class = 255
)

var className = map[Class]string{
	ClassIN:   "IN",
	ClassCH:   "CH",
	ClassHS:   "HS",
	ClassNONE: "NONE",
	ClassANY:  "ANY",
}

func (c Class) String() string {
	if s, ok := className[c]; ok {
		return s
	}
	return "Class" + strconv.Itoa(int(c))
}
