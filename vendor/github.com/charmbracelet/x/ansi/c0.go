package ansi

// C0 control characters.
//
// These range from (0x00-0x1F) as defined in ISO 646 (ASCII).
// See: https://en.wikipedia.org/wiki/C0_and_C1_control_codes
const (
	// NUL is the null character (Caret: ^@, Char: \0).
	NUL = 0x00
	// SOH is the start of heading character (Caret: ^A).
	SOH = 0x01
	// STX is the start of text character (Caret: ^B).
	STX = 0x02
	// ETX is the end of text character (Caret: ^C).
	ETX = 0x03
	// EOT is the end of transmission character (Caret: ^D).
	EOT = 0x04
	// ENQ is the enquiry character (Caret: ^E).
	ENQ = 0x05
	// ACK is the acknowledge character (Caret: ^F).
	ACK = 0x06
	// BEL is the bell character (Caret: ^G, Char: \a).
	BEL = 0x07
	// BS is the backspace character (Caret: ^H, Char: \b).
	BS = 0x08
	// HT is the horizontal tab character (Caret: ^I, Char: \t).
	HT = 0x09
	// LF is the line feed character (Caret: ^J, Char: \n).
	LF = 0x0A
	// VT is the vertical tab character (Caret: ^K, Char: \v).
	VT = 0x0B
	// FF is the form feed character (Caret: ^L, Char: \f).
	FF = 0x0C
	// CR is the carriage return character (Caret: ^M, Char: \r).
	CR = 0x0D
	// SO is the shift out character (Caret: ^N).
	SO = 0x0E
	// SI is the shift in character (Caret: ^O).
	SI = 0x0F
	// DLE is the data link escape character (Caret: ^P).
	DLE = 0x10
	// DC1 is the device control 1 character (Caret: ^Q).
	DC1 = 0x11
	// DC2 is the device control 2 character (Caret: ^R).
	DC2 = 0x12
	// DC3 is the device control 3 character (Caret: ^S).
	DC3 = 0x13
	// DC4 is the device control 4 character (Caret: ^T).
	DC4 = 0x14
	// NAK is the negative acknowledge character (Caret: ^U).
	NAK = 0x15
	// SYN is the synchronous idle character (Caret: ^V).
	SYN = 0x16
	// ETB is the end of transmission block character (Caret: ^W).
	ETB = 0x17
	// CAN is the cancel character (Caret: ^X).
	CAN = 0x18
	// EM is the end of medium character (Caret: ^Y).
	EM = 0x19
	// SUB is the substitute character (Caret: ^Z).
	SUB = 0x1A
	// ESC is the escape character (Caret: ^[, Char: \e).
	ESC = 0x1B
	// FS is the file separator character (Caret: ^\).
	FS = 0x1C
	// GS is the group separator character (Caret: ^]).
	GS = 0x1D
	// RS is the record separator character (Caret: ^^).
	RS = 0x1E
	// US is the unit separator character (Caret: ^_).
	US = 0x1F

	// LS0 is the locking shift 0 character.
	// This is an alias for [SI].
	LS0 = SI
	// LS1 is the locking shift 1 character.
	// This is an alias for [SO].
	LS1 = SO
)
