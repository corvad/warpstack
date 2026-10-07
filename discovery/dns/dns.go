package dns

type TTL uint32

type Record struct {
	Name  string
	Type  Type
	Class Class
	TTL   TTL
	Data  []byte
}

type Query struct {
	Name  string
	Type  Type
	Class Class
}

type OpCode uint8

const (
	OpCodeQuery  OpCode = 0
	OpCodeIQuery OpCode = 1
	OpCodeStatus OpCode = 2
	OpCodeNotify OpCode = 4
	OpCodeUpdate OpCode = 5
	OpCodeDSO    OpCode = 6
)

type Flags uint16

const (
	FlagQR     Flags = 1 << 15
	FlagOPCODE Flags = 0b0111100000000000
	FlagAA     Flags = 1 << 10
	FlagTC     Flags = 1 << 9
	FlagRD     Flags = 1 << 8
	FlagRA     Flags = 1 << 7
	FlagZ      Flags = 1 << 6
	FlagAD     Flags = 1 << 5
	FlagCD     Flags = 1 << 4
	FlagRCODE  Flags = 0x000F
)

var lowercaseLUT = func() (t [256]byte) {
	for i := 0; i < 256; i++ {
		c := byte(i)
		if 'A' <= c && c <= 'Z' {
			t[i] = c + 'a' - 'A'
		} else {
			t[i] = c
		}
	}
	return t
}()
