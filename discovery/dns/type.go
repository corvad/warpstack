package dns

import "strconv"

type Type uint16

const (
	TypeA          Type = 1
	TypeNS         Type = 2
	TypeMD         Type = 3
	TypeMF         Type = 4
	TypeCNAME      Type = 5
	TypeSOA        Type = 6
	TypeMB         Type = 7
	TypeMG         Type = 8
	TypeMR         Type = 9
	TypeNULL       Type = 10
	TypeWKS        Type = 11
	TypePTR        Type = 12
	TypeHINFO      Type = 13
	TypeMINFO      Type = 14
	TypeMX         Type = 15
	TypeTXT        Type = 16
	TypeRP         Type = 17
	TypeAFSDB      Type = 18
	TypeX25        Type = 19
	TypeISDN       Type = 20
	TypeRT         Type = 21
	TypeNSAP       Type = 22
	TypeNSAPPTR    Type = 23
	TypeSIG        Type = 24
	TypeKEY        Type = 25
	TypePX         Type = 26
	TypeGPOS       Type = 27
	TypeAAAA       Type = 28
	TypeLOC        Type = 29
	TypeNXT        Type = 30
	TypeEID        Type = 31
	TypeNIMLOC     Type = 32
	TypeSRV        Type = 33
	TypeATMA       Type = 34
	TypeNAPTR      Type = 35
	TypeKX         Type = 36
	TypeCERT       Type = 37
	TypeA6         Type = 38
	TypeDNAME      Type = 39
	TypeSINK       Type = 40
	TypeOPT        Type = 41
	TypeAPL        Type = 42
	TypeDS         Type = 43
	TypeSSHFP      Type = 44
	TypeIPSECKEY   Type = 45
	TypeRRSIG      Type = 46
	TypeNSEC       Type = 47
	TypeDNSKEY     Type = 48
	TypeDHCID      Type = 49
	TypeNSEC3      Type = 50
	TypeNSEC3PARAM Type = 51
	TypeTLSA       Type = 52
	TypeSMIMEA     Type = 53

	TypeHIP        Type = 55
	TypeNINFO      Type = 56
	TypeRKEY       Type = 57
	TypeTALINK     Type = 58
	TypeCDS        Type = 59
	TypeCDNSKEY    Type = 60
	TypeOPENPGPKEY Type = 61
	TypeCSYNC      Type = 62
	TypeZONEMD     Type = 63
	TypeSVCB       Type = 64
	TypeHTTPS      Type = 65
	TypeDSYNC      Type = 66
	TypeHHIT       Type = 67
	TypeBRID       Type = 68
	TypeUNECE      Type = 69

	TypeSPF    Type = 99
	TypeUINFO  Type = 100
	TypeUID    Type = 101
	TypeGID    Type = 102
	TypeUNSPEC Type = 103
	TypeNID    Type = 104
	TypeL32    Type = 105
	TypeL64    Type = 106
	TypeLP     Type = 107
	TypeEUI48  Type = 108
	TypeEUI64  Type = 109

	TypeNXNAME Type = 128

	TypeTKEY     Type = 249
	TypeTSIG     Type = 250
	TypeIXFR     Type = 251
	TypeAXFR     Type = 252
	TypeMAILB    Type = 253
	TypeMAILA    Type = 254
	TypeALL      Type = 255
	TypeURI      Type = 256
	TypeCAA      Type = 257
	TypeAVC      Type = 258
	TypeDOA      Type = 259
	TypeAMTRELAY Type = 260
	TypeRESINFO  Type = 261
	TypeWALLET   Type = 262
	TypeCLA      Type = 263
	TypeIPN      Type = 264

	TypeTA  Type = 32768
	TypeDLV Type = 32769
)

var typeName = map[Type]string{
	TypeA:          "A",
	TypeNS:         "NS",
	TypeMD:         "MD",
	TypeMF:         "MF",
	TypeCNAME:      "CNAME",
	TypeSOA:        "SOA",
	TypeMB:         "MB",
	TypeMG:         "MG",
	TypeMR:         "MR",
	TypeNULL:       "NULL",
	TypeWKS:        "WKS",
	TypePTR:        "PTR",
	TypeHINFO:      "HINFO",
	TypeMINFO:      "MINFO",
	TypeMX:         "MX",
	TypeTXT:        "TXT",
	TypeRP:         "RP",
	TypeAFSDB:      "AFSDB",
	TypeX25:        "X25",
	TypeISDN:       "ISDN",
	TypeRT:         "RT",
	TypeNSAP:       "NSAP",
	TypeNSAPPTR:    "NSAPPTR",
	TypeSIG:        "SIG",
	TypeKEY:        "KEY",
	TypePX:         "PX",
	TypeGPOS:       "GPOS",
	TypeAAAA:       "AAAA",
	TypeLOC:        "LOC",
	TypeNXT:        "NXT",
	TypeEID:        "EID",
	TypeNIMLOC:     "NIMLOC",
	TypeSRV:        "SRV",
	TypeATMA:       "ATMA",
	TypeNAPTR:      "NAPTR",
	TypeKX:         "KX",
	TypeCERT:       "CERT",
	TypeA6:         "A6",
	TypeDNAME:      "DNAME",
	TypeSINK:       "SINK",
	TypeOPT:        "OPT",
	TypeAPL:        "APL",
	TypeDS:         "DS",
	TypeSSHFP:      "SSHFP",
	TypeIPSECKEY:   "IPSECKEY",
	TypeRRSIG:      "RRSIG",
	TypeNSEC:       "NSEC",
	TypeDNSKEY:     "DNSKEY",
	TypeDHCID:      "DHCID",
	TypeNSEC3:      "NSEC3",
	TypeNSEC3PARAM: "NSEC3PARAM",
	TypeTLSA:       "TLSA",
	TypeSMIMEA:     "SMIMEA",

	TypeHIP:        "HIP",
	TypeNINFO:      "NINFO",
	TypeRKEY:       "RKEY",
	TypeTALINK:     "TALINK",
	TypeCDS:        "CDS",
	TypeCDNSKEY:    "CDNSKEY",
	TypeOPENPGPKEY: "OPENPGPKEY",
	TypeCSYNC:      "CSYNC",
	TypeZONEMD:     "ZONEMD",
	TypeSVCB:       "SVCB",
	TypeHTTPS:      "HTTPS",
	TypeDSYNC:      "DSYNC",
	TypeHHIT:       "HHIT",
	TypeBRID:       "BRID",
	TypeUNECE:      "UNECE",

	TypeSPF:    "SPF",
	TypeUINFO:  "UINFO",
	TypeUID:    "UID",
	TypeGID:    "GID",
	TypeUNSPEC: "UNSPEC",
	TypeNID:    "NID",
	TypeL32:    "L32",
	TypeL64:    "L64",
	TypeLP:     "LP",
	TypeEUI48:  "EUI48",
	TypeEUI64:  "EUI64",
	TypeNXNAME: "NXNAME",

	TypeTKEY:     "TKEY",
	TypeTSIG:     "TSIG",
	TypeIXFR:     "IXFR",
	TypeAXFR:     "AXFR",
	TypeMAILB:    "MAILB",
	TypeMAILA:    "MAILA",
	TypeALL:      "ALL",
	TypeURI:      "URI",
	TypeCAA:      "CAA",
	TypeAVC:      "AVC",
	TypeDOA:      "DOA",
	TypeAMTRELAY: "AMTRELAY",
	TypeRESINFO:  "RESINFO",
	TypeWALLET:   "WALLET",
	TypeCLA:      "CLA",
	TypeIPN:      "IPN",

	TypeTA:  "TA",
	TypeDLV: "DLV",
}

func (t Type) String() string {
	if s, ok := typeName[t]; ok {
		return s
	}
	return "Type" + strconv.Itoa(int(t))
}
