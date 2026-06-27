package tls

import (
	"fmt"
)

type hashAlgID uint8

const (
	HashNone hashAlgID = iota
	HashMD5
	HashSHA1
	HashSHA224
	HashSHA256
	HashSHA384
	HashSHA512
)

func (h hashAlgID) String() string {
	switch h {
	case HashNone:
		return "None"
	case HashMD5:
		return "MD5"
	case HashSHA1:
		return "SHA1"
	case HashSHA224:
		return "SHA224"
	case HashSHA256:
		return "SHA256"
	case HashSHA384:
		return "SHA384"
	case HashSHA512:
		return "SHA512"
	default:
		return "Unknown"
	}
}

type sigAlgID uint8

// Signature algorithms for TLS 1.2 (See RFC 5246, section A.4.1)
const (
	SigAnon sigAlgID = iota
	SigRSA
	SigDSA
	SigECDSA
)

func (sig sigAlgID) String() string {
	switch sig {
	case SigAnon:
		return "Anon"
	case SigRSA:
		return "RSA"
	case SigDSA:
		return "DSA"
	case SigECDSA:
		return "ECDSA"
	default:
		return "Unknown"
	}
}

// SignatureAndHash mirrors the TLS 1.2, SignatureAndHashAlgorithm struct. See
// RFC 5246, section A.4.1.
type SignatureAndHash struct {
	h hashAlgID
	s sigAlgID
}

func (sigAlg SignatureAndHash) String() string {
	return fmt.Sprintf("{%s,%s}", sigAlg.s, sigAlg.h)
}

func (sigAlg SignatureAndHash) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"signature":"%s","hash":"%s"}`, sigAlg.s, sigAlg.h)), nil
}

func (sigAlg SignatureAndHash) internal() signatureAndHash {
	return signatureAndHash{uint8(sigAlg.h), uint8(sigAlg.s)}
}

// defaultSignatureAndHashAlgorithms contains the default signature and hash
// algorithm paris supported by `crypto/tls`
var defaultSignatureAndHashAlgorithms []signatureAndHash

// AllSignatureAndHashAlgorithms contains all possible signature and
// hash algorithm pairs that the can be advertised in a TLS 1.2 ClientHello.
var AllSignatureAndHashAlgorithms []SignatureAndHash

func init() {
	defaultSignatureAndHashAlgorithms = supportedSignatureAlgorithms
	for _, sighash := range supportedSignatureAlgorithms {
		AllSignatureAndHashAlgorithms = append(AllSignatureAndHashAlgorithms,
			SignatureAndHash{hashAlgID(sighash.hash), sigAlgID(sighash.signature)})
	}
}

// TLSVersions is a list of the current SSL/TLS Versions implemented by Go
var Versions = map[uint16]string{
	VersionSSL30: "SSL 3.0",
	VersionTLS10: "TLS 1.0",
	VersionTLS11: "TLS 1.1",
	VersionTLS12: "TLS 1.2",
}

// CipherSuite describes an individual cipher suite, with long and short names
// and security properties.
type CipherSuite struct {
	Name, ShortName string
	// ForwardSecret cipher suites negotiate ephemeral keys, allowing forward secrecy.
	ForwardSecret bool
	EllipticCurve bool
}

// Returns the (short) name of the cipher suite.
func (c CipherSuite) String() string {
	if c.ShortName != "" {
		return c.ShortName
	}
	return c.Name
}

// CipherSuites contains all values in the TLS Cipher Suite Registry
// https://www.iana.org/assignments/tls-parameters/tls-parameters.xhtml
var CipherSuites = map[uint16]CipherSuite{
	0x0000: {Name: "TLS_NULL_WITH_NULL_NULL"},
	0x0001: {Name: "TLS_RSA_WITH_NULL_MD5"},
	0x0002: {Name: "TLS_RSA_WITH_NULL_SHA"},
	0x0003: {Name: "TLS_RSA_EXPORT_WITH_RC4_40_MD5", ShortName: "EXP-RC4-MD5"},
	0x0004: {Name: "TLS_RSA_WITH_RC4_128_MD5", ShortName: "RC4-MD5"},
	0x0005: {Name: "TLS_RSA_WITH_RC4_128_SHA", ShortName: "RC4-SHA"},
	0x0006: {Name: "TLS_RSA_EXPORT_WITH_RC2_CBC_40_MD5", ShortName: "EXP-RC2-CBC-MD5"},
	0x0007: {Name: "TLS_RSA_WITH_IDEA_CBC_SHA", ShortName: "IDEA-CBC-SHA"},
	0x0008: {Name: "TLS_RSA_EXPORT_WITH_DES40_CBC_SHA", ShortName: "EXP-DES-CBC-SHA"},
	0x0009: {Name: "TLS_RSA_WITH_DES_CBC_SHA", ShortName: "DES-CBC-SHA"},
	0x000A: {Name: "TLS_RSA_WITH_3DES_EDE_CBC_SHA", ShortName: "DES-CBC3-SHA"},
	0x000B: {Name: "TLS_DH_DSS_EXPORT_WITH_DES40_CBC_SHA", ShortName: "EXP-DH-DSS-DES-CBC-SHA"},
	0x000C: {Name: "TLS_DH_DSS_WITH_DES_CBC_SHA", ShortName: "DH-DSS-DES-CBC-SHA"},
	0x000D: {Name: "TLS_DH_DSS_WITH_3DES_EDE_CBC_SHA", ShortName: "DH-DSS-DES-CBC3-SHA"},
	0x000E: {Name: "TLS_DH_RSA_EXPORT_WITH_DES40_CBC_SHA", ShortName: "EXP-DH-RSA-DES-CBC-SHA"},
	0x000F: {Name: "TLS_DH_RSA_WITH_DES_CBC_SHA", ShortName: "DH-RSA-DES-CBC-SHA"},
	0x0010: {Name: "TLS_DH_RSA_WITH_3DES_EDE_CBC_SHA", ShortName: "DH-RSA-DES-CBC3-SHA"},
	0x0011: {Name: "TLS_DHE_DSS_EXPORT_WITH_DES40_CBC_SHA", ShortName: "EXP-EDH-DSS-DES-CBC-SHA", ForwardSecret: true},
	0x0012: {Name: "TLS_DHE_DSS_WITH_DES_CBC_SHA", ShortName: "EDH-DSS-DES-CBC-SHA", ForwardSecret: true},
	0x0013: {Name: "TLS_DHE_DSS_WITH_3DES_EDE_CBC_SHA", ShortName: "EDH-DSS-DES-CBC3-SHA", ForwardSecret: true},
	0x0014: {Name: "TLS_DHE_RSA_EXPORT_WITH_DES40_CBC_SHA", ShortName: "EXP-EDH-RSA-DES-CBC-SHA", ForwardSecret: true},
	0x0015: {Name: "TLS_DHE_RSA_WITH_DES_CBC_SHA", ShortName: "EDH-RSA-DES-CBC-SHA", ForwardSecret: true},
	0x0016: {Name: "TLS_DHE_RSA_WITH_3DES_EDE_CBC_SHA", ShortName: "EDH-RSA-DES-CBC3-SHA", ForwardSecret: true},
	0x0017: {Name: "TLS_DH_anon_EXPORT_WITH_RC4_40_MD5"},
	0x0018: {Name: "TLS_DH_anon_WITH_RC4_128_MD5"},
	0x0019: {Name: "TLS_DH_anon_EXPORT_WITH_DES40_CBC_SHA"},
	0x001A: {Name: "TLS_DH_anon_WITH_DES_CBC_SHA"},
	0x001B: {Name: "TLS_DH_anon_WITH_3DES_EDE_CBC_SHA"},
	0x001E: {Name: "TLS_KRB5_WITH_DES_CBC_SHA"},
	0x001F: {Name: "TLS_KRB5_WITH_3DES_EDE_CBC_SHA"},
	0x0020: {Name: "TLS_KRB5_WITH_RC4_128_SHA"},
	0x0021: {Name: "TLS_KRB5_WITH_IDEA_CBC_SHA"},
	0x0022: {Name: "TLS_KRB5_WITH_DES_CBC_MD5"},
	0x0023: {Name: "TLS_KRB5_WITH_3DES_EDE_CBC_MD5"},
	0x0024: {Name: "TLS_KRB5_WITH_RC4_128_MD5"},
	0x0025: {Name: "TLS_KRB5_WITH_IDEA_CBC_MD5"},
	0x0026: {Name: "TLS_KRB5_EXPORT_WITH_DES_CBC_40_SHA"},
	0x0027: {Name: "TLS_KRB5_EXPORT_WITH_RC2_CBC_40_SHA"},
	0x0028: {Name: "TLS_KRB5_EXPORT_WITH_RC4_40_SHA"},
	0x0029: {Name: "TLS_KRB5_EXPORT_WITH_DES_CBC_40_MD5"},
	0x002A: {Name: "TLS_KRB5_EXPORT_WITH_RC2_CBC_40_MD5"},
	0x002B: {Name: "TLS_KRB5_EXPORT_WITH_RC4_40_MD5"},
	0x002C: {Name: "TLS_PSK_WITH_NULL_SHA"},
	0x002D: {Name: "TLS_DHE_PSK_WITH_NULL_SHA", ForwardSecret: true},
	0x002E: {Name: "TLS_RSA_PSK_WITH_NULL_SHA"},
	0x002F: {Name: "TLS_RSA_WITH_AES_128_CBC_SHA", ShortName: "AES128-SHA"},
	0x0030: {Name: "TLS_DH_DSS_WITH_AES_128_CBC_SHA", ShortName: "DH-DSS-AES128-SHA"},
	0x0031: {Name: "TLS_DH_RSA_WITH_AES_128_CBC_SHA", ShortName: "DH-RSA-AES128-SHA"},
	0x0032: {Name: "TLS_DHE_DSS_WITH_AES_128_CBC_SHA", ShortName: "DHE-DSS-AES128-SHA", ForwardSecret: true},
	0x0033: {Name: "TLS_DHE_RSA_WITH_AES_128_CBC_SHA", ShortName: "DHE-RSA-AES128-SHA", ForwardSecret: true},
	0x0034: {Name: "TLS_DH_anon_WITH_AES_128_CBC_SHA"},
	0x0035: {Name: "TLS_RSA_WITH_AES_256_CBC_SHA", ShortName: "AES256-SHA"},
	0x0036: {Name: "TLS_DH_DSS_WITH_AES_256_CBC_SHA", ShortName: "DH-DSS-AES256-SHA"},
	0x0037: {Name: "TLS_DH_RSA_WITH_AES_256_CBC_SHA", ShortName: "DH-RSA-AES256-SHA"},
	0x0038: {Name: "TLS_DHE_DSS_WITH_AES_256_CBC_SHA", ShortName: "DHE-DSS-AES256-SHA", ForwardSecret: true},
	0x0039: {Name: "TLS_DHE_RSA_WITH_AES_256_CBC_SHA", ShortName: "DHE-RSA-AES256-SHA", ForwardSecret: true},
	0x003A: {Name: "TLS_DH_anon_WITH_AES_256_CBC_SHA"},
	0x003B: {Name: "TLS_RSA_WITH_NULL_SHA256"},
	0x003C: {Name: "TLS_RSA_WITH_AES_128_CBC_SHA256", ShortName: "AES128-SHA256"},
	0x003D: {Name: "TLS_RSA_WITH_AES_256_CBC_SHA256", ShortName: "AES256-SHA256"},
	0x003E: {Name: "TLS_DH_DSS_WITH_AES_128_CBC_SHA256", ShortName: "DH-DSS-AES128-SHA256"},
	0x003F: {Name: "TLS_DH_RSA_WITH_AES_128_CBC_SHA256", ShortName: "DH-RSA-AES128-SHA256"},
	0x0040: {Name: "TLS_DHE_DSS_WITH_AES_128_CBC_SHA256", ShortName: "DHE-DSS-AES128-SHA256", ForwardSecret: true},
	0x0041: {Name: "TLS_RSA_WITH_CAMELLIA_128_CBC_SHA", ShortName: "CAMELLIA128-SHA"},
	0x0042: {Name: "TLS_DH_DSS_WITH_CAMELLIA_128_CBC_SHA", ShortName: "DH-DSS-CAMELLIA128-SHA"},
	0x0043: {Name: "TLS_DH_RSA_WITH_CAMELLIA_128_CBC_SHA", ShortName: "DH-RSA-CAMELLIA128-SHA"},
	0x0044: {Name: "TLS_DHE_DSS_WITH_CAMELLIA_128_CBC_SHA", ShortName: "DHE-DSS-CAMELLIA128-SHA", ForwardSecret: true},
	0x0045: {Name: "TLS_DHE_RSA_WITH_CAMELLIA_128_CBC_SHA", ShortName: "DHE-RSA-CAMELLIA128-SHA", ForwardSecret: true},
	0x0046: {Name: "TLS_DH_anon_WITH_CAMELLIA_128_CBC_SHA"},
	0x0067: {Name: "TLS_DHE_RSA_WITH_AES_128_CBC_SHA256", ShortName: "DHE-RSA-AES128-SHA256", ForwardSecret: true},
	0x0068: {Name: "TLS_DH_DSS_WITH_AES_256_CBC_SHA256", ShortName: "DH-DSS-AES256-SHA256"},
	0x0069: {Name: "TLS_DH_RSA_WITH_AES_256_CBC_SHA256", ShortName: "DH-RSA-AES256-SHA256"},
	0x006A: {Name: "TLS_DHE_DSS_WITH_AES_256_CBC_SHA256", ShortName: "DHE-DSS-AES256-SHA256", ForwardSecret: true},
	0x006B: {Name: "TLS_DHE_RSA_WITH_AES_256_CBC_SHA256", ShortName: "DHE-RSA-AES256-SHA256", ForwardSecret: true},
	0x006C: {Name: "TLS_DH_anon_WITH_AES_128_CBC_SHA256"},
	0x006D: {Name: "TLS_DH_anon_WITH_AES_256_CBC_SHA256"},
	0x0084: {Name: "TLS_RSA_WITH_CAMELLIA_256_CBC_SHA", ShortName: "CAMELLIA256-SHA"},
	0x0085: {Name: "TLS_DH_DSS_WITH_CAMELLIA_256_CBC_SHA", ShortName: "DH-DSS-CAMELLIA256-SHA"},
	0x0086: {Name: "TLS_DH_RSA_WITH_CAMELLIA_256_CBC_SHA", ShortName: "DH-RSA-CAMELLIA256-SHA"},
	0x0087: {Name: "TLS_DHE_DSS_WITH_CAMELLIA_256_CBC_SHA", ShortName: "DHE-DSS-CAMELLIA256-SHA", ForwardSecret: true},
	0x0088: {Name: "TLS_DHE_RSA_WITH_CAMELLIA_256_CBC_SHA", ShortName: "DHE-RSA-CAMELLIA256-SHA", ForwardSecret: true},
	0x0089: {Name: "TLS_DH_anon_WITH_CAMELLIA_256_CBC_SHA"},
	0x008A: {Name: "TLS_PSK_WITH_RC4_128_SHA", ShortName: "PSK-RC4-SHA"},
	0x008B: {Name: "TLS_PSK_WITH_3DES_EDE_CBC_SHA", ShortName: "PSK-3DES-EDE-CBC-SHA"},
	0x008C: {Name: "TLS_PSK_WITH_AES_128_CBC_SHA", ShortName: "PSK-AES128-CBC-SHA"},
	0x008D: {Name: "TLS_PSK_WITH_AES_256_CBC_SHA", ShortName: "PSK-AES256-CBC-SHA"},
	0x008E: {Name: "TLS_DHE_PSK_WITH_RC4_128_SHA", ForwardSecret: true},
	0x008F: {Name: "TLS_DHE_PSK_WITH_3DES_EDE_CBC_SHA", ForwardSecret: true},
	0x0090: {Name: "TLS_DHE_PSK_WITH_AES_128_CBC_SHA", ForwardSecret: true},
	0x0091: {Name: "TLS_DHE_PSK_WITH_AES_256_CBC_SHA", ForwardSecret: true},
	0x0092: {Name: "TLS_RSA_PSK_WITH_RC4_128_SHA"},
	0x0093: {Name: "TLS_RSA_PSK_WITH_3DES_EDE_CBC_SHA"},
	0x0094: {Name: "TLS_RSA_PSK_WITH_AES_128_CBC_SHA"},
	0x0095: {Name: "TLS_RSA_PSK_WITH_AES_256_CBC_SHA"},
	0x0096: {Name: "TLS_RSA_WITH_SEED_CBC_SHA", ShortName: "SEED-SHA"},
	0x0097: {Name: "TLS_DH_DSS_WITH_SEED_CBC_SHA", ShortName: "DH-DSS-SEED-SHA"},
	0x0098: {Name: "TLS_DH_RSA_WITH_SEED_CBC_SHA", ShortName: "DH-RSA-SEED-SHA"},
	0x0099: {Name: "TLS_DHE_DSS_WITH_SEED_CBC_SHA", ShortName: "DHE-DSS-SEED-SHA", ForwardSecret: true},
	0x009A: {Name: "TLS_DHE_RSA_WITH_SEED_CBC_SHA", ShortName: "DHE-RSA-SEED-SHA", ForwardSecret: true},
	0x009B: {Name: "TLS_DH_anon_WITH_SEED_CBC_SHA"},
	0x009C: {Name: "TLS_RSA_WITH_AES_128_GCM_SHA256", ShortName: "AES128-GCM-SHA256"},
	0x009D: {Name: "TLS_RSA_WITH_AES_256_GCM_SHA384", ShortName: "AES256-GCM-SHA384"},
	0x009E: {Name: "TLS_DHE_RSA_WITH_AES_128_GCM_SHA256", ShortName: "DHE-RSA-AES128-GCM-SHA256", ForwardSecret: true},
	0x009F: {Name: "TLS_DHE_RSA_WITH_AES_256_GCM_SHA384", ShortName: "DHE-RSA-AES256-GCM-SHA384", ForwardSecret: true},
	0x00A0: {Name: "TLS_DH_RSA_WITH_AES_128_GCM_SHA256", ShortName: "DH-RSA-AES128-GCM-SHA256"},
	0x00A1: {Name: "TLS_DH_RSA_WITH_AES_256_GCM_SHA384", ShortName: "DH-RSA-AES256-GCM-SHA384"},
	0x00A2: {Name: "TLS_DHE_DSS_WITH_AES_128_GCM_SHA256", ShortName: "DHE-DSS-AES128-GCM-SHA256", ForwardSecret: true},
	0x00A3: {Name: "TLS_DHE_DSS_WITH_AES_256_GCM_SHA384", ShortName: "DHE-DSS-AES256-GCM-SHA384", ForwardSecret: true},
	0x00A4: {Name: "TLS_DH_DSS_WITH_AES_128_GCM_SHA256", ShortName: "DH-DSS-AES128-GCM-SHA256"},
	0x00A5: {Name: "TLS_DH_DSS_WITH_AES_256_GCM_SHA384", ShortName: "DH-DSS-AES256-GCM-SHA384"},
	0x00A6: {Name: "TLS_DH_anon_WITH_AES_128_GCM_SHA256"},
	0x00A7: {Name: "TLS_DH_anon_WITH_AES_256_GCM_SHA384"},
	0x00A8: {Name: "TLS_PSK_WITH_AES_128_GCM_SHA256"},
	0x00A9: {Name: "TLS_PSK_WITH_AES_256_GCM_SHA384"},
	0x00AA: {Name: "TLS_DHE_PSK_WITH_AES_128_GCM_SHA256", ForwardSecret: true},
	0x00AB: {Name: "TLS_DHE_PSK_WITH_AES_256_GCM_SHA384", ForwardSecret: true},
	0x00AC: {Name: "TLS_RSA_PSK_WITH_AES_128_GCM_SHA256"},
	0x00AD: {Name: "TLS_RSA_PSK_WITH_AES_256_GCM_SHA384"},
	0x00AE: {Name: "TLS_PSK_WITH_AES_128_CBC_SHA256"},
	0x00AF: {Name: "TLS_PSK_WITH_AES_256_CBC_SHA384"},
	0x00B0: {Name: "TLS_PSK_WITH_NULL_SHA256"},
	0x00B1: {Name: "TLS_PSK_WITH_NULL_SHA384"},
	0x00B2: {Name: "TLS_DHE_PSK_WITH_AES_128_CBC_SHA256", ForwardSecret: true},
	0x00B3: {Name: "TLS_DHE_PSK_WITH_AES_256_CBC_SHA384", ForwardSecret: true},
	0x00B4: {Name: "TLS_DHE_PSK_WITH_NULL_SHA256", ForwardSecret: true},
	0x00B5: {Name: "TLS_DHE_PSK_WITH_NULL_SHA384", ForwardSecret: true},
	0x00B6: {Name: "TLS_RSA_PSK_WITH_AES_128_CBC_SHA256"},
	0x00B7: {Name: "TLS_RSA_PSK_WITH_AES_256_CBC_SHA384"},
	0x00B8: {Name: "TLS_RSA_PSK_WITH_NULL_SHA256"},
	0x00B9: {Name: "TLS_RSA_PSK_WITH_NULL_SHA384"},
	0x00BA: {Name: "TLS_RSA_WITH_CAMELLIA_128_CBC_SHA256"},
	0x00BB: {Name: "TLS_DH_DSS_WITH_CAMELLIA_128_CBC_SHA256"},
	0x00BC: {Name: "TLS_DH_RSA_WITH_CAMELLIA_128_CBC_SHA256"},
	0x00BD: {Name: "TLS_DHE_DSS_WITH_CAMELLIA_128_CBC_SHA256", ForwardSecret: true},
	0x00BE: {Name: "TLS_DHE_RSA_WITH_CAMELLIA_128_CBC_SHA256", ForwardSecret: true},
	0x00BF: {Name: "TLS_DH_anon_WITH_CAMELLIA_128_CBC_SHA256"},
	0x00C0: {Name: "TLS_RSA_WITH_CAMELLIA_256_CBC_SHA256"},
	0x00C1: {Name: "TLS_DH_DSS_WITH_CAMELLIA_256_CBC_SHA256"},
	0x00C2: {Name: "TLS_DH_RSA_WITH_CAMELLIA_256_CBC_SHA256"},
	0x00C3: {Name: "TLS_DHE_DSS_WITH_CAMELLIA_256_CBC_SHA256", ForwardSecret: true},
	0x00C4: {Name: "TLS_DHE_RSA_WITH_CAMELLIA_256_CBC_SHA256", ForwardSecret: true},
	0x00C5: {Name: "TLS_DH_anon_WITH_CAMELLIA_256_CBC_SHA256"},
	0x00FF: {Name: "TLS_EMPTY_RENEGOTIATION_INFO_SCSV"},
	0xC001: {Name: "TLS_ECDH_ECDSA_WITH_NULL_SHA", EllipticCurve: true},
	0xC002: {Name: "TLS_ECDH_ECDSA_WITH_RC4_128_SHA", ShortName: "ECDH-ECDSA-RC4-SHA", EllipticCurve: true},
	0xC003: {Name: "TLS_ECDH_ECDSA_WITH_3DES_EDE_CBC_SHA", ShortName: "ECDH-ECDSA-DES-CBC3-SHA", EllipticCurve: true},
	0xC004: {Name: "TLS_ECDH_ECDSA_WITH_AES_128_CBC_SHA", ShortName: "ECDH-ECDSA-AES128-SHA", EllipticCurve: true},
	0xC005: {Name: "TLS_ECDH_ECDSA_WITH_AES_256_CBC_SHA", ShortName: "ECDH-ECDSA-AES256-SHA", EllipticCurve: true},
	0xC006: {Name: "TLS_ECDHE_ECDSA_WITH_NULL_SHA", ForwardSecret: true, EllipticCurve: true},
	0xC007: {Name: "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA", ShortName: "ECDHE-ECDSA-RC4-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC008: {Name: "TLS_ECDHE_ECDSA_WITH_3DES_EDE_CBC_SHA", ShortName: "ECDHE-ECDSA-DES-CBC3-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC009: {Name: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA", ShortName: "ECDHE-ECDSA-AES128-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC00A: {Name: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA", ShortName: "ECDHE-ECDSA-AES256-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC00B: {Name: "TLS_ECDH_RSA_WITH_NULL_SHA", EllipticCurve: true},
	0xC00C: {Name: "TLS_ECDH_RSA_WITH_RC4_128_SHA", ShortName: "ECDH-RSA-RC4-SHA", EllipticCurve: true},
	0xC00D: {Name: "TLS_ECDH_RSA_WITH_3DES_EDE_CBC_SHA", ShortName: "ECDH-RSA-DES-CBC3-SHA", EllipticCurve: true},
	0xC00E: {Name: "TLS_ECDH_RSA_WITH_AES_128_CBC_SHA", ShortName: "ECDH-RSA-AES128-SHA", EllipticCurve: true},
	0xC00F: {Name: "TLS_ECDH_RSA_WITH_AES_256_CBC_SHA", ShortName: "ECDH-RSA-AES256-SHA", EllipticCurve: true},
	0xC010: {Name: "TLS_ECDHE_RSA_WITH_NULL_SHA", ForwardSecret: true, EllipticCurve: true},
	0xC011: {Name: "TLS_ECDHE_RSA_WITH_RC4_128_SHA", ShortName: "ECDHE-RSA-RC4-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC012: {Name: "TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", ShortName: "ECDHE-RSA-DES-CBC3-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC013: {Name: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA", ShortName: "ECDHE-RSA-AES128-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC014: {Name: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA", ShortName: "ECDHE-RSA-AES256-SHA", ForwardSecret: true, EllipticCurve: true},
	0xC015: {Name: "TLS_ECDH_anon_WITH_NULL_SHA", EllipticCurve: true},
	0xC016: {Name: "TLS_ECDH_anon_WITH_RC4_128_SHA", EllipticCurve: true},
	0xC017: {Name: "TLS_ECDH_anon_WITH_3DES_EDE_CBC_SHA", EllipticCurve: true},
	0xC018: {Name: "TLS_ECDH_anon_WITH_AES_128_CBC_SHA", EllipticCurve: true},
	0xC019: {Name: "TLS_ECDH_anon_WITH_AES_256_CBC_SHA", EllipticCurve: true},
	0xC01A: {Name: "TLS_SRP_SHA_WITH_3DES_EDE_CBC_SHA", ShortName: "SRP-3DES-EDE-CBC-SHA"},
	0xC01B: {Name: "TLS_SRP_SHA_RSA_WITH_3DES_EDE_CBC_SHA", ShortName: "SRP-RSA-3DES-EDE-CBC-SHA"},
	0xC01C: {Name: "TLS_SRP_SHA_DSS_WITH_3DES_EDE_CBC_SHA", ShortName: "SRP-DSS-3DES-EDE-CBC-SHA"},
	0xC01D: {Name: "TLS_SRP_SHA_WITH_AES_128_CBC_SHA", ShortName: "SRP-AES-128-CBC-SHA"},
	0xC01E: {Name: "TLS_SRP_SHA_RSA_WITH_AES_128_CBC_SHA", ShortName: "SRP-RSA-AES-128-CBC-SHA"},
	0xC01F: {Name: "TLS_SRP_SHA_DSS_WITH_AES_128_CBC_SHA", ShortName: "SRP-DSS-AES-128-CBC-SHA"},
	0xC020: {Name: "TLS_SRP_SHA_WITH_AES_256_CBC_SHA", ShortName: "SRP-AES-256-CBC-SHA"},
	0xC021: {Name: "TLS_SRP_SHA_RSA_WITH_AES_256_CBC_SHA", ShortName: "SRP-RSA-AES-256-CBC-SHA"},
	0xC022: {Name: "TLS_SRP_SHA_DSS_WITH_AES_256_CBC_SHA", ShortName: "SRP-DSS-AES-256-CBC-SHA"},
	0xC023: {Name: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256", ShortName: "ECDHE-ECDSA-AES128-SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC024: {Name: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA384", ShortName: "ECDHE-ECDSA-AES256-SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC025: {Name: "TLS_ECDH_ECDSA_WITH_AES_128_CBC_SHA256", ShortName: "ECDH-ECDSA-AES128-SHA256", EllipticCurve: true},
	0xC026: {Name: "TLS_ECDH_ECDSA_WITH_AES_256_CBC_SHA384", ShortName: "ECDH-ECDSA-AES256-SHA384", EllipticCurve: true},
	0xC027: {Name: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256", ShortName: "ECDHE-RSA-AES128-SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC028: {Name: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA384", ShortName: "ECDHE-RSA-AES256-SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC029: {Name: "TLS_ECDH_RSA_WITH_AES_128_CBC_SHA256", ShortName: "ECDH-RSA-AES128-SHA256", EllipticCurve: true},
	0xC02A: {Name: "TLS_ECDH_RSA_WITH_AES_256_CBC_SHA384", ShortName: "ECDH-RSA-AES256-SHA384", EllipticCurve: true},
	0xC02B: {Name: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", ShortName: "ECDHE-ECDSA-AES128-GCM-SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC02C: {Name: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", ShortName: "ECDHE-ECDSA-AES256-GCM-SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC02D: {Name: "TLS_ECDH_ECDSA_WITH_AES_128_GCM_SHA256", ShortName: "ECDH-ECDSA-AES128-GCM-SHA256", EllipticCurve: true},
	0xC02E: {Name: "TLS_ECDH_ECDSA_WITH_AES_256_GCM_SHA384", ShortName: "ECDH-ECDSA-AES256-GCM-SHA384", EllipticCurve: true},
	0xC02F: {Name: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", ShortName: "ECDHE-RSA-AES128-GCM-SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC030: {Name: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", ShortName: "ECDHE-RSA-AES256-GCM-SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC031: {Name: "TLS_ECDH_RSA_WITH_AES_128_GCM_SHA256", ShortName: "ECDH-RSA-AES128-GCM-SHA256", EllipticCurve: true},
	0xC032: {Name: "TLS_ECDH_RSA_WITH_AES_256_GCM_SHA384", ShortName: "ECDH-RSA-AES256-GCM-SHA384", EllipticCurve: true},
	0xC033: {Name: "TLS_ECDHE_PSK_WITH_RC4_128_SHA", ForwardSecret: true, EllipticCurve: true},
	0xC034: {Name: "TLS_ECDHE_PSK_WITH_3DES_EDE_CBC_SHA", ForwardSecret: true, EllipticCurve: true},
	0xC035: {Name: "TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA", ForwardSecret: true, EllipticCurve: true},
	0xC036: {Name: "TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA", ForwardSecret: true, EllipticCurve: true},
	0xC037: {Name: "TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC038: {Name: "TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC039: {Name: "TLS_ECDHE_PSK_WITH_NULL_SHA", ForwardSecret: true, EllipticCurve: true},
	0xC03A: {Name: "TLS_ECDHE_PSK_WITH_NULL_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC03B: {Name: "TLS_ECDHE_PSK_WITH_NULL_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC03C: {Name: "TLS_RSA_WITH_ARIA_128_CBC_SHA256"},
	0xC03D: {Name: "TLS_RSA_WITH_ARIA_256_CBC_SHA384"},
	0xC03E: {Name: "TLS_DH_DSS_WITH_ARIA_128_CBC_SHA256"},
	0xC03F: {Name: "TLS_DH_DSS_WITH_ARIA_256_CBC_SHA384"},
	0xC040: {Name: "TLS_DH_RSA_WITH_ARIA_128_CBC_SHA256"},
	0xC041: {Name: "TLS_DH_RSA_WITH_ARIA_256_CBC_SHA384"},
	0xC042: {Name: "TLS_DHE_DSS_WITH_ARIA_128_CBC_SHA256", ForwardSecret: true},
	0xC043: {Name: "TLS_DHE_DSS_WITH_ARIA_256_CBC_SHA384", ForwardSecret: true},
	0xC044: {Name: "TLS_DHE_RSA_WITH_ARIA_128_CBC_SHA256", ForwardSecret: true},
	0xC045: {Name: "TLS_DHE_RSA_WITH_ARIA_256_CBC_SHA384", ForwardSecret: true},
	0xC046: {Name: "TLS_DH_anon_WITH_ARIA_128_CBC_SHA256"},
	0xC047: {Name: "TLS_DH_anon_WITH_ARIA_256_CBC_SHA384"},
	0xC048: {Name: "TLS_ECDHE_ECDSA_WITH_ARIA_128_CBC_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC049: {Name: "TLS_ECDHE_ECDSA_WITH_ARIA_256_CBC_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC04A: {Name: "TLS_ECDH_ECDSA_WITH_ARIA_128_CBC_SHA256", EllipticCurve: true},
	0xC04B: {Name: "TLS_ECDH_ECDSA_WITH_ARIA_256_CBC_SHA384", EllipticCurve: true},
	0xC04C: {Name: "TLS_ECDHE_RSA_WITH_ARIA_128_CBC_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC04D: {Name: "TLS_ECDHE_RSA_WITH_ARIA_256_CBC_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC04E: {Name: "TLS_ECDH_RSA_WITH_ARIA_128_CBC_SHA256", EllipticCurve: true},
	0xC04F: {Name: "TLS_ECDH_RSA_WITH_ARIA_256_CBC_SHA384", EllipticCurve: true},
	0xC050: {Name: "TLS_RSA_WITH_ARIA_128_GCM_SHA256"},
	0xC051: {Name: "TLS_RSA_WITH_ARIA_256_GCM_SHA384"},
	0xC052: {Name: "TLS_DHE_RSA_WITH_ARIA_128_GCM_SHA256", ForwardSecret: true},
	0xC053: {Name: "TLS_DHE_RSA_WITH_ARIA_256_GCM_SHA384", ForwardSecret: true},
	0xC054: {Name: "TLS_DH_RSA_WITH_ARIA_128_GCM_SHA256"},
	0xC055: {Name: "TLS_DH_RSA_WITH_ARIA_256_GCM_SHA384"},
	0xC056: {Name: "TLS_DHE_DSS_WITH_ARIA_128_GCM_SHA256", ForwardSecret: true},
	0xC057: {Name: "TLS_DHE_DSS_WITH_ARIA_256_GCM_SHA384", ForwardSecret: true},
	0xC058: {Name: "TLS_DH_DSS_WITH_ARIA_128_GCM_SHA256"},
	0xC059: {Name: "TLS_DH_DSS_WITH_ARIA_256_GCM_SHA384"},
	0xC05A: {Name: "TLS_DH_anon_WITH_ARIA_128_GCM_SHA256"},
	0xC05B: {Name: "TLS_DH_anon_WITH_ARIA_256_GCM_SHA384"},
	0xC05C: {Name: "TLS_ECDHE_ECDSA_WITH_ARIA_128_GCM_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC05D: {Name: "TLS_ECDHE_ECDSA_WITH_ARIA_256_GCM_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC05E: {Name: "TLS_ECDH_ECDSA_WITH_ARIA_128_GCM_SHA256", EllipticCurve: true},
	0xC05F: {Name: "TLS_ECDH_ECDSA_WITH_ARIA_256_GCM_SHA384", EllipticCurve: true},
	0xC060: {Name: "TLS_ECDHE_RSA_WITH_ARIA_128_GCM_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC061: {Name: "TLS_ECDHE_RSA_WITH_ARIA_256_GCM_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC062: {Name: "TLS_ECDH_RSA_WITH_ARIA_128_GCM_SHA256", EllipticCurve: true},
	0xC063: {Name: "TLS_ECDH_RSA_WITH_ARIA_256_GCM_SHA384", EllipticCurve: true},
	0xC064: {Name: "TLS_PSK_WITH_ARIA_128_CBC_SHA256"},
	0xC065: {Name: "TLS_PSK_WITH_ARIA_256_CBC_SHA384"},
	0xC066: {Name: "TLS_DHE_PSK_WITH_ARIA_128_CBC_SHA256", ForwardSecret: true},
	0xC067: {Name: "TLS_DHE_PSK_WITH_ARIA_256_CBC_SHA384", ForwardSecret: true},
	0xC068: {Name: "TLS_RSA_PSK_WITH_ARIA_128_CBC_SHA256"},
	0xC069: {Name: "TLS_RSA_PSK_WITH_ARIA_256_CBC_SHA384"},
	0xC06A: {Name: "TLS_PSK_WITH_ARIA_128_GCM_SHA256"},
	0xC06B: {Name: "TLS_PSK_WITH_ARIA_256_GCM_SHA384"},
	0xC06C: {Name: "TLS_DHE_PSK_WITH_ARIA_128_GCM_SHA256", ForwardSecret: true},
	0xC06D: {Name: "TLS_DHE_PSK_WITH_ARIA_256_GCM_SHA384", ForwardSecret: true},
	0xC06E: {Name: "TLS_RSA_PSK_WITH_ARIA_128_GCM_SHA256"},
	0xC06F: {Name: "TLS_RSA_PSK_WITH_ARIA_256_GCM_SHA384"},
	0xC070: {Name: "TLS_ECDHE_PSK_WITH_ARIA_128_CBC_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC071: {Name: "TLS_ECDHE_PSK_WITH_ARIA_256_CBC_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC072: {Name: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_128_CBC_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC073: {Name: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_256_CBC_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC074: {Name: "TLS_ECDH_ECDSA_WITH_CAMELLIA_128_CBC_SHA256", EllipticCurve: true},
	0xC075: {Name: "TLS_ECDH_ECDSA_WITH_CAMELLIA_256_CBC_SHA384", EllipticCurve: true},
	0xC076: {Name: "TLS_ECDHE_RSA_WITH_CAMELLIA_128_CBC_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC077: {Name: "TLS_ECDHE_RSA_WITH_CAMELLIA_256_CBC_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC078: {Name: "TLS_ECDH_RSA_WITH_CAMELLIA_128_CBC_SHA256", EllipticCurve: true},
	0xC079: {Name: "TLS_ECDH_RSA_WITH_CAMELLIA_256_CBC_SHA384", EllipticCurve: true},
	0xC07A: {Name: "TLS_RSA_WITH_CAMELLIA_128_GCM_SHA256"},
	0xC07B: {Name: "TLS_RSA_WITH_CAMELLIA_256_GCM_SHA384"},
	0xC07C: {Name: "TLS_DHE_RSA_WITH_CAMELLIA_128_GCM_SHA256", ForwardSecret: true},
	0xC07D: {Name: "TLS_DHE_RSA_WITH_CAMELLIA_256_GCM_SHA384", ForwardSecret: true},
	0xC07E: {Name: "TLS_DH_RSA_WITH_CAMELLIA_128_GCM_SHA256"},
	0xC07F: {Name: "TLS_DH_RSA_WITH_CAMELLIA_256_GCM_SHA384"},
	0xC080: {Name: "TLS_DHE_DSS_WITH_CAMELLIA_128_GCM_SHA256", ForwardSecret: true},
	0xC081: {Name: "TLS_DHE_DSS_WITH_CAMELLIA_256_GCM_SHA384", ForwardSecret: true},
	0xC082: {Name: "TLS_DH_DSS_WITH_CAMELLIA_128_GCM_SHA256"},
	0xC083: {Name: "TLS_DH_DSS_WITH_CAMELLIA_256_GCM_SHA384"},
	0xC084: {Name: "TLS_DH_anon_WITH_CAMELLIA_128_GCM_SHA256"},
	0xC085: {Name: "TLS_DH_anon_WITH_CAMELLIA_256_GCM_SHA384"},
	0xC086: {Name: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_128_GCM_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC087: {Name: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_256_GCM_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC088: {Name: "TLS_ECDH_ECDSA_WITH_CAMELLIA_128_GCM_SHA256", EllipticCurve: true},
	0xC089: {Name: "TLS_ECDH_ECDSA_WITH_CAMELLIA_256_GCM_SHA384", EllipticCurve: true},
	0xC08A: {Name: "TLS_ECDHE_RSA_WITH_CAMELLIA_128_GCM_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC08B: {Name: "TLS_ECDHE_RSA_WITH_CAMELLIA_256_GCM_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC08C: {Name: "TLS_ECDH_RSA_WITH_CAMELLIA_128_GCM_SHA256", EllipticCurve: true},
	0xC08D: {Name: "TLS_ECDH_RSA_WITH_CAMELLIA_256_GCM_SHA384", EllipticCurve: true},
	0xC08E: {Name: "TLS_PSK_WITH_CAMELLIA_128_GCM_SHA256"},
	0xC08F: {Name: "TLS_PSK_WITH_CAMELLIA_256_GCM_SHA384"},
	0xC090: {Name: "TLS_DHE_PSK_WITH_CAMELLIA_128_GCM_SHA256", ForwardSecret: true},
	0xC091: {Name: "TLS_DHE_PSK_WITH_CAMELLIA_256_GCM_SHA384", ForwardSecret: true},
	0xC092: {Name: "TLS_RSA_PSK_WITH_CAMELLIA_128_GCM_SHA256"},
	0xC093: {Name: "TLS_RSA_PSK_WITH_CAMELLIA_256_GCM_SHA384"},
	0xC094: {Name: "TLS_PSK_WITH_CAMELLIA_128_CBC_SHA256"},
	0xC095: {Name: "TLS_PSK_WITH_CAMELLIA_256_CBC_SHA384"},
	0xC096: {Name: "TLS_DHE_PSK_WITH_CAMELLIA_128_CBC_SHA256", ForwardSecret: true},
	0xC097: {Name: "TLS_DHE_PSK_WITH_CAMELLIA_256_CBC_SHA384", ForwardSecret: true},
	0xC098: {Name: "TLS_RSA_PSK_WITH_CAMELLIA_128_CBC_SHA256"},
	0xC099: {Name: "TLS_RSA_PSK_WITH_CAMELLIA_256_CBC_SHA384"},
	0xC09A: {Name: "TLS_ECDHE_PSK_WITH_CAMELLIA_128_CBC_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xC09B: {Name: "TLS_ECDHE_PSK_WITH_CAMELLIA_256_CBC_SHA384", ForwardSecret: true, EllipticCurve: true},
	0xC09C: {Name: "TLS_RSA_WITH_AES_128_CCM"},
	0xC09D: {Name: "TLS_RSA_WITH_AES_256_CCM"},
	0xC09E: {Name: "TLS_DHE_RSA_WITH_AES_128_CCM", ForwardSecret: true},
	0xC09F: {Name: "TLS_DHE_RSA_WITH_AES_256_CCM", ForwardSecret: true},
	0xC0A0: {Name: "TLS_RSA_WITH_AES_128_CCM_8"},
	0xC0A1: {Name: "TLS_RSA_WITH_AES_256_CCM_8"},
	0xC0A2: {Name: "TLS_DHE_RSA_WITH_AES_128_CCM_8", ForwardSecret: true},
	0xC0A3: {Name: "TLS_DHE_RSA_WITH_AES_256_CCM_8", ForwardSecret: true},
	0xC0A4: {Name: "TLS_PSK_WITH_AES_128_CCM"},
	0xC0A5: {Name: "TLS_PSK_WITH_AES_256_CCM"},
	0xC0A6: {Name: "TLS_DHE_PSK_WITH_AES_128_CCM", ForwardSecret: true},
	0xC0A7: {Name: "TLS_DHE_PSK_WITH_AES_256_CCM", ForwardSecret: true},
	0xC0A8: {Name: "TLS_PSK_WITH_AES_128_CCM_8"},
	0xC0A9: {Name: "TLS_PSK_WITH_AES_256_CCM_8"},
	0xC0AA: {Name: "TLS_PSK_DHE_WITH_AES_128_CCM_8", ForwardSecret: true},
	0xC0AB: {Name: "TLS_PSK_DHE_WITH_AES_256_CCM_8", ForwardSecret: true},
	0xC0AC: {Name: "TLS_ECDHE_ECDSA_WITH_AES_128_CCM", ForwardSecret: true, EllipticCurve: true},
	0xC0AD: {Name: "TLS_ECDHE_ECDSA_WITH_AES_256_CCM", ForwardSecret: true, EllipticCurve: true},
	0xC0AE: {Name: "TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8", ForwardSecret: true, EllipticCurve: true},
	0xC0AF: {Name: "TLS_ECDHE_ECDSA_WITH_AES_256_CCM_8", ForwardSecret: true, EllipticCurve: true},
	// Non-IANA standardized cipher suites:
	// ChaCha20, Poly1305 cipher suites are defined in
	// https://tools.ietf.org/html/draft-agl-tls-chacha20poly1305-04
	0xCC13: {Name: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xCC14: {Name: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256", ForwardSecret: true, EllipticCurve: true},
	0xCC15: {Name: "TLS_DHE_RSA_WITH_CHACHA20_POLY1305_SHA256", ForwardSecret: true, EllipticCurve: true},
}

var Curves = map[CurveID]string{
	0:     "Unassigned",
	1:     "sect163k1",
	2:     "sect163r1",
	3:     "sect163r2",
	4:     "sect193r1",
	5:     "sect193r2",
	6:     "sect233k1",
	7:     "sect233r1",
	8:     "sect239k1",
	9:     "sect283k1",
	10:    "sect283r1",
	11:    "sect409k1",
	12:    "sect409r1",
	13:    "sect571k1",
	14:    "sect571r1",
	15:    "secp160k1",
	16:    "secp160r1",
	17:    "secp160r2",
	18:    "secp192k1",
	19:    "secp192r1",
	20:    "secp224k1",
	21:    "secp224r1",
	22:    "secp256k1",
	23:    "secp256r1",
	24:    "secp384r1",
	25:    "secp521r1",
	26:    "brainpoolP256r1",
	27:    "brainpoolP384r1",
	28:    "brainpoolP512r1",
	65281: "arbitrary_explicit_prime_curves",
	65282: "arbitrary_explicit_char2_curves",
}
