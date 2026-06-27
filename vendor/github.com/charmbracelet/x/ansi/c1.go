package ansi

// C1 control characters.
//
// These range from (0x80-0x9F) as defined in ISO 6429 (ECMA-48).
// See: https://en.wikipedia.org/wiki/C0_and_C1_control_codes
const (
	// PAD is the padding character.
	PAD = 0x80
	// HOP is the high octet preset character.
	HOP = 0x81
	// BPH is the break permitted here character.
	BPH = 0x82
	// NBH is the no break here character.
	NBH = 0x83
	// IND is the index character.
	IND = 0x84
	// NEL is the next line character.
	NEL = 0x85
	// SSA is the start of selected area character.
	SSA = 0x86
	// ESA is the end of selected area character.
	ESA = 0x87
	// HTS is the horizontal tab set character.
	HTS = 0x88
	// HTJ is the horizontal tab with justification character.
	HTJ = 0x89
	// VTS is the vertical tab set character.
	VTS = 0x8A
	// PLD is the partial line forward character.
	PLD = 0x8B
	// PLU is the partial line backward character.
	PLU = 0x8C
	// RI is the reverse index character.
	RI = 0x8D
	// SS2 is the single shift 2 character.
	SS2 = 0x8E
	// SS3 is the single shift 3 character.
	SS3 = 0x8F
	// DCS is the device control string character.
	DCS = 0x90
	// PU1 is the private use 1 character.
	PU1 = 0x91
	// PU2 is the private use 2 character.
	PU2 = 0x92
	// STS is the set transmit state character.
	STS = 0x93
	// CCH is the cancel character.
	CCH = 0x94
	// MW is the message waiting character.
	MW = 0x95
	// SPA is the start of guarded area character.
	SPA = 0x96
	// EPA is the end of guarded area character.
	EPA = 0x97
	// SOS is the start of string character.
	SOS = 0x98
	// SGCI is the single graphic character introducer character.
	SGCI = 0x99
	// SCI is the single character introducer character.
	SCI = 0x9A
	// CSI is the control sequence introducer character.
	CSI = 0x9B
	// ST is the string terminator character.
	ST = 0x9C
	// OSC is the operating system command character.
	OSC = 0x9D
	// PM is the privacy message character.
	PM = 0x9E
	// APC is the application program command character.
	APC = 0x9F
)
