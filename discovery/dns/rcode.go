package dns

import "strconv"

type RCODE uint16

const (
	RCODENoError    RCODE = 0
	RCODEFormErr    RCODE = 1
	RCODEServFail   RCODE = 2
	RCODENXDomain   RCODE = 3
	RCODENotImp     RCODE = 4
	RCODEServRef    RCODE = 5
	RCODEYXDomain   RCODE = 6
	RCODEYXRRSet    RCODE = 7
	RCODENXRRSet    RCODE = 8
	RCODENotAuth    RCODE = 9
	RCODENotZone    RCODE = 10
	RCODEDSOTYPENI  RCODE = 11
	RCODEBADVERSSIG RCODE = 16
	RCODEBADSIG     RCODE = 16
	RCODEBADKEY     RCODE = 17
	RCODEBADTIME    RCODE = 18
	RCODEBADMODE    RCODE = 19
	RCODEBADNAME    RCODE = 20
	RCODEBADALG     RCODE = 21
	RCODEBADTRUNC   RCODE = 22
	RCODEBADCOOKIE  RCODE = 23
)

var rcodeName = map[RCODE]string{
	RCODENoError:    "NoError",
	RCODEFormErr:    "FormErr",
	RCODEServFail:   "ServFail",
	RCODENXDomain:   "NXDomain",
	RCODENotImp:     "NotImp",
	RCODEServRef:    "ServRef",
	RCODEYXDomain:   "YXDomain",
	RCODEYXRRSet:    "YXRRSet",
	RCODENXRRSet:    "NXRRSet",
	RCODENotAuth:    "NotAuth",
	RCODENotZone:    "NotZone",
	RCODEDSOTYPENI:  "DSOTYPENI",
	RCODEBADVERSSIG: "BADVERS/SIG",
	RCODEBADKEY:     "BADKEY",
	RCODEBADTIME:    "BADTIME",
	RCODEBADMODE:    "BADMODE",
	RCODEBADNAME:    "BADNAME",
	RCODEBADALG:     "BADALG",
	RCODEBADTRUNC:   "BADTRUNC",
	RCODEBADCOOKIE:  "BADCOOKIE",
}

func (r RCODE) String() string {
	if s, ok := rcodeName[r]; ok {
		return s
	}
	return "RCODE" + strconv.Itoa(int(r))
}
