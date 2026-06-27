package uv

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/xo/terminfo"
)

// buildKeysTable builds a table of key sequences and their corresponding key
// events based on the VT100/VT200, XTerm, and Urxvt terminal specs.
func buildKeysTable(flags LegacyKeyEncoding, term string, useTerminfo bool) map[string]Key {
	nul := Key{Code: KeySpace, Mod: ModCtrl} // ctrl+@ or ctrl+space
	if flags&flagCtrlAt != 0 {
		nul = Key{Code: '@', Mod: ModCtrl}
	}

	tab := Key{Code: KeyTab} // ctrl+i or tab
	if flags&flagCtrlI != 0 {
		tab = Key{Code: 'i', Mod: ModCtrl}
	}

	enter := Key{Code: KeyEnter} // ctrl+m or enter
	if flags&flagCtrlM != 0 {
		enter = Key{Code: 'm', Mod: ModCtrl}
	}

	esc := Key{Code: KeyEscape} // ctrl+[ or escape
	if flags&flagCtrlOpenBracket != 0 {
		esc = Key{Code: '[', Mod: ModCtrl} // ctrl+[ or escape
	}

	del := Key{Code: KeyBackspace}
	if flags&flagBackspace != 0 {
		del.Code = KeyDelete
	}

	find := Key{Code: KeyHome}
	if flags&flagFind != 0 {
		find.Code = KeyFind
	}

	sel := Key{Code: KeyEnd}
	if flags&flagSelect != 0 {
		sel.Code = KeySelect
	}

	// The following is a table of key sequences and their corresponding key
	// events based on the VT100/VT200 terminal specs.
	//
	// See: https://vt100.net/docs/vt100-ug/chapter3.html#S3.2
	// See: https://vt100.net/docs/vt220-rm/chapter3.html
	//
	// XXX: These keys may be overwritten by other options like XTerm or
	// Terminfo.
	table := map[string]Key{
		// C0 control characters
		string(byte(ansi.NUL)): nul,
		string(byte(ansi.SOH)): {Code: 'a', Mod: ModCtrl},
		string(byte(ansi.STX)): {Code: 'b', Mod: ModCtrl},
		string(byte(ansi.ETX)): {Code: 'c', Mod: ModCtrl},
		string(byte(ansi.EOT)): {Code: 'd', Mod: ModCtrl},
		string(byte(ansi.ENQ)): {Code: 'e', Mod: ModCtrl},
		string(byte(ansi.ACK)): {Code: 'f', Mod: ModCtrl},
		string(byte(ansi.BEL)): {Code: 'g', Mod: ModCtrl},
		string(byte(ansi.BS)):  {Code: 'h', Mod: ModCtrl},
		string(byte(ansi.HT)):  tab,
		string(byte(ansi.LF)):  {Code: 'j', Mod: ModCtrl},
		string(byte(ansi.VT)):  {Code: 'k', Mod: ModCtrl},
		string(byte(ansi.FF)):  {Code: 'l', Mod: ModCtrl},
		string(byte(ansi.CR)):  enter,
		string(byte(ansi.SO)):  {Code: 'n', Mod: ModCtrl},
		string(byte(ansi.SI)):  {Code: 'o', Mod: ModCtrl},
		string(byte(ansi.DLE)): {Code: 'p', Mod: ModCtrl},
		string(byte(ansi.DC1)): {Code: 'q', Mod: ModCtrl},
		string(byte(ansi.DC2)): {Code: 'r', Mod: ModCtrl},
		string(byte(ansi.DC3)): {Code: 's', Mod: ModCtrl},
		string(byte(ansi.DC4)): {Code: 't', Mod: ModCtrl},
		string(byte(ansi.NAK)): {Code: 'u', Mod: ModCtrl},
		string(byte(ansi.SYN)): {Code: 'v', Mod: ModCtrl},
		string(byte(ansi.ETB)): {Code: 'w', Mod: ModCtrl},
		string(byte(ansi.CAN)): {Code: 'x', Mod: ModCtrl},
		string(byte(ansi.EM)):  {Code: 'y', Mod: ModCtrl},
		string(byte(ansi.SUB)): {Code: 'z', Mod: ModCtrl},
		string(byte(ansi.ESC)): esc,
		string(byte(ansi.FS)):  {Code: '\\', Mod: ModCtrl},
		string(byte(ansi.GS)):  {Code: ']', Mod: ModCtrl},
		string(byte(ansi.RS)):  {Code: '^', Mod: ModCtrl},
		string(byte(ansi.US)):  {Code: '_', Mod: ModCtrl},

		// Special keys in G0
		string(byte(ansi.SP)):  {Code: KeySpace, Text: " "},
		string(byte(ansi.DEL)): del,

		// Special keys

		"\x1b[Z": {Code: KeyTab, Mod: ModShift},

		"\x1b[1~": find,
		"\x1b[2~": {Code: KeyInsert},
		"\x1b[3~": {Code: KeyDelete},
		"\x1b[4~": sel,
		"\x1b[5~": {Code: KeyPgUp},
		"\x1b[6~": {Code: KeyPgDown},
		"\x1b[7~": {Code: KeyHome},
		"\x1b[8~": {Code: KeyEnd},

		// Normal mode
		"\x1b[A": {Code: KeyUp},
		"\x1b[B": {Code: KeyDown},
		"\x1b[C": {Code: KeyRight},
		"\x1b[D": {Code: KeyLeft},
		"\x1b[E": {Code: KeyBegin},
		"\x1b[F": {Code: KeyEnd},
		"\x1b[H": {Code: KeyHome},
		"\x1b[P": {Code: KeyF1},
		"\x1b[Q": {Code: KeyF2},
		"\x1b[R": {Code: KeyF3},
		"\x1b[S": {Code: KeyF4},

		// Application Cursor Key Mode (DECCKM)
		"\x1bOA": {Code: KeyUp},
		"\x1bOB": {Code: KeyDown},
		"\x1bOC": {Code: KeyRight},
		"\x1bOD": {Code: KeyLeft},
		"\x1bOE": {Code: KeyBegin},
		"\x1bOF": {Code: KeyEnd},
		"\x1bOH": {Code: KeyHome},
		"\x1bOP": {Code: KeyF1},
		"\x1bOQ": {Code: KeyF2},
		"\x1bOR": {Code: KeyF3},
		"\x1bOS": {Code: KeyF4},

		// Keypad Application Mode (DECKPAM)

		"\x1bOM": {Code: KeyKpEnter},
		"\x1bOX": {Code: KeyKpEqual},
		"\x1bOj": {Code: KeyKpMultiply},
		"\x1bOk": {Code: KeyKpPlus},
		"\x1bOl": {Code: KeyKpComma},
		"\x1bOm": {Code: KeyKpMinus},
		"\x1bOn": {Code: KeyKpDecimal},
		"\x1bOo": {Code: KeyKpDivide},
		"\x1bOp": {Code: KeyKp0},
		"\x1bOq": {Code: KeyKp1},
		"\x1bOr": {Code: KeyKp2},
		"\x1bOs": {Code: KeyKp3},
		"\x1bOt": {Code: KeyKp4},
		"\x1bOu": {Code: KeyKp5},
		"\x1bOv": {Code: KeyKp6},
		"\x1bOw": {Code: KeyKp7},
		"\x1bOx": {Code: KeyKp8},
		"\x1bOy": {Code: KeyKp9},

		// Function keys

		"\x1b[11~": {Code: KeyF1},
		"\x1b[12~": {Code: KeyF2},
		"\x1b[13~": {Code: KeyF3},
		"\x1b[14~": {Code: KeyF4},
		"\x1b[15~": {Code: KeyF5},
		"\x1b[17~": {Code: KeyF6},
		"\x1b[18~": {Code: KeyF7},
		"\x1b[19~": {Code: KeyF8},
		"\x1b[20~": {Code: KeyF9},
		"\x1b[21~": {Code: KeyF10},
		"\x1b[23~": {Code: KeyF11},
		"\x1b[24~": {Code: KeyF12},
		"\x1b[25~": {Code: KeyF13},
		"\x1b[26~": {Code: KeyF14},
		"\x1b[28~": {Code: KeyF15},
		"\x1b[29~": {Code: KeyF16},
		"\x1b[31~": {Code: KeyF17},
		"\x1b[32~": {Code: KeyF18},
		"\x1b[33~": {Code: KeyF19},
		"\x1b[34~": {Code: KeyF20},
	}

	// CSI ~ sequence keys
	csiTildeKeys := map[string]Key{
		"1": find, "2": {Code: KeyInsert},
		"3": {Code: KeyDelete}, "4": sel,
		"5": {Code: KeyPgUp}, "6": {Code: KeyPgDown},
		"7": {Code: KeyHome}, "8": {Code: KeyEnd},
		// There are no 9 and 10 keys
		"11": {Code: KeyF1}, "12": {Code: KeyF2},
		"13": {Code: KeyF3}, "14": {Code: KeyF4},
		"15": {Code: KeyF5}, "17": {Code: KeyF6},
		"18": {Code: KeyF7}, "19": {Code: KeyF8},
		"20": {Code: KeyF9}, "21": {Code: KeyF10},
		"23": {Code: KeyF11}, "24": {Code: KeyF12},
		"25": {Code: KeyF13}, "26": {Code: KeyF14},
		"28": {Code: KeyF15}, "29": {Code: KeyF16},
		"31": {Code: KeyF17}, "32": {Code: KeyF18},
		"33": {Code: KeyF19}, "34": {Code: KeyF20},
	}

	// URxvt keys
	// See https://manpages.ubuntu.com/manpages/trusty/man7/urxvt.7.html#key%20codes
	table["\x1b[a"] = Key{Code: KeyUp, Mod: ModShift}
	table["\x1b[b"] = Key{Code: KeyDown, Mod: ModShift}
	table["\x1b[c"] = Key{Code: KeyRight, Mod: ModShift}
	table["\x1b[d"] = Key{Code: KeyLeft, Mod: ModShift}
	table["\x1bOa"] = Key{Code: KeyUp, Mod: ModCtrl}
	table["\x1bOb"] = Key{Code: KeyDown, Mod: ModCtrl}
	table["\x1bOc"] = Key{Code: KeyRight, Mod: ModCtrl}
	table["\x1bOd"] = Key{Code: KeyLeft, Mod: ModCtrl}
	//nolint:godox
	// TODO: invistigate if shift-ctrl arrow keys collide with DECCKM keys i.e.
	// "\x1bOA", "\x1bOB", "\x1bOC", "\x1bOD"

	// URxvt modifier CSI ~ keys
	for k, v := range csiTildeKeys {
		key := v
		// Normal (no modifier) already defined part of VT100/VT200
		// Shift modifier
		key.Mod = ModShift
		table["\x1b["+k+"$"] = key
		// Ctrl modifier
		key.Mod = ModCtrl
		table["\x1b["+k+"^"] = key
		// Shift-Ctrl modifier
		key.Mod = ModShift | ModCtrl
		table["\x1b["+k+"@"] = key
	}

	// URxvt F keys
	// Note: Shift + F1-F10 generates F11-F20.
	// This means Shift + F1 and Shift + F2 will generate F11 and F12, the same
	// applies to Ctrl + Shift F1 & F2.
	//
	// P.S. Don't like this? Blame URxvt, configure your terminal to use
	// different escapes like XTerm, or switch to a better terminal ¯\_(ツ)_/¯
	//
	// See https://manpages.ubuntu.com/manpages/trusty/man7/urxvt.7.html#key%20codes
	table["\x1b[23$"] = Key{Code: KeyF11, Mod: ModShift}
	table["\x1b[24$"] = Key{Code: KeyF12, Mod: ModShift}
	table["\x1b[25$"] = Key{Code: KeyF13, Mod: ModShift}
	table["\x1b[26$"] = Key{Code: KeyF14, Mod: ModShift}
	table["\x1b[28$"] = Key{Code: KeyF15, Mod: ModShift}
	table["\x1b[29$"] = Key{Code: KeyF16, Mod: ModShift}
	table["\x1b[31$"] = Key{Code: KeyF17, Mod: ModShift}
	table["\x1b[32$"] = Key{Code: KeyF18, Mod: ModShift}
	table["\x1b[33$"] = Key{Code: KeyF19, Mod: ModShift}
	table["\x1b[34$"] = Key{Code: KeyF20, Mod: ModShift}
	table["\x1b[11^"] = Key{Code: KeyF1, Mod: ModCtrl}
	table["\x1b[12^"] = Key{Code: KeyF2, Mod: ModCtrl}
	table["\x1b[13^"] = Key{Code: KeyF3, Mod: ModCtrl}
	table["\x1b[14^"] = Key{Code: KeyF4, Mod: ModCtrl}
	table["\x1b[15^"] = Key{Code: KeyF5, Mod: ModCtrl}
	table["\x1b[17^"] = Key{Code: KeyF6, Mod: ModCtrl}
	table["\x1b[18^"] = Key{Code: KeyF7, Mod: ModCtrl}
	table["\x1b[19^"] = Key{Code: KeyF8, Mod: ModCtrl}
	table["\x1b[20^"] = Key{Code: KeyF9, Mod: ModCtrl}
	table["\x1b[21^"] = Key{Code: KeyF10, Mod: ModCtrl}
	table["\x1b[23^"] = Key{Code: KeyF11, Mod: ModCtrl}
	table["\x1b[24^"] = Key{Code: KeyF12, Mod: ModCtrl}
	table["\x1b[25^"] = Key{Code: KeyF13, Mod: ModCtrl}
	table["\x1b[26^"] = Key{Code: KeyF14, Mod: ModCtrl}
	table["\x1b[28^"] = Key{Code: KeyF15, Mod: ModCtrl}
	table["\x1b[29^"] = Key{Code: KeyF16, Mod: ModCtrl}
	table["\x1b[31^"] = Key{Code: KeyF17, Mod: ModCtrl}
	table["\x1b[32^"] = Key{Code: KeyF18, Mod: ModCtrl}
	table["\x1b[33^"] = Key{Code: KeyF19, Mod: ModCtrl}
	table["\x1b[34^"] = Key{Code: KeyF20, Mod: ModCtrl}
	table["\x1b[23@"] = Key{Code: KeyF11, Mod: ModShift | ModCtrl}
	table["\x1b[24@"] = Key{Code: KeyF12, Mod: ModShift | ModCtrl}
	table["\x1b[25@"] = Key{Code: KeyF13, Mod: ModShift | ModCtrl}
	table["\x1b[26@"] = Key{Code: KeyF14, Mod: ModShift | ModCtrl}
	table["\x1b[28@"] = Key{Code: KeyF15, Mod: ModShift | ModCtrl}
	table["\x1b[29@"] = Key{Code: KeyF16, Mod: ModShift | ModCtrl}
	table["\x1b[31@"] = Key{Code: KeyF17, Mod: ModShift | ModCtrl}
	table["\x1b[32@"] = Key{Code: KeyF18, Mod: ModShift | ModCtrl}
	table["\x1b[33@"] = Key{Code: KeyF19, Mod: ModShift | ModCtrl}
	table["\x1b[34@"] = Key{Code: KeyF20, Mod: ModShift | ModCtrl}

	// Register Alt + <key> combinations
	// XXX: this must come after URxvt but before XTerm keys to register URxvt
	// keys with alt modifier
	tmap := map[string]Key{}
	for seq, key := range table {
		key := key
		key.Mod |= ModAlt
		key.Text = "" // Clear runes
		tmap["\x1b"+seq] = key
	}
	for seq, key := range tmap {
		table[seq] = key
	}

	// XTerm modifiers
	// These are offset by 1 to be compatible with our Mod type.
	// See https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-PC-Style-Function-Keys
	modifiers := []KeyMod{
		ModShift,                              // 1
		ModAlt,                                // 2
		ModShift | ModAlt,                     // 3
		ModCtrl,                               // 4
		ModShift | ModCtrl,                    // 5
		ModAlt | ModCtrl,                      // 6
		ModShift | ModAlt | ModCtrl,           // 7
		ModMeta,                               // 8
		ModMeta | ModShift,                    // 9
		ModMeta | ModAlt,                      // 10
		ModMeta | ModShift | ModAlt,           // 11
		ModMeta | ModCtrl,                     // 12
		ModMeta | ModShift | ModCtrl,          // 13
		ModMeta | ModAlt | ModCtrl,            // 14
		ModMeta | ModShift | ModAlt | ModCtrl, // 15
	}

	// SS3 keypad function keys
	ss3FuncKeys := map[string]Key{
		// These are defined in XTerm
		// Taken from Foot keymap.h and XTerm modifyOtherKeys
		// https://codeberg.org/dnkl/foot/src/branch/master/keymap.h
		"M": {Code: KeyKpEnter}, "X": {Code: KeyKpEqual},
		"j": {Code: KeyKpMultiply}, "k": {Code: KeyKpPlus},
		"l": {Code: KeyKpComma}, "m": {Code: KeyKpMinus},
		"n": {Code: KeyKpDecimal}, "o": {Code: KeyKpDivide},
		"p": {Code: KeyKp0}, "q": {Code: KeyKp1},
		"r": {Code: KeyKp2}, "s": {Code: KeyKp3},
		"t": {Code: KeyKp4}, "u": {Code: KeyKp5},
		"v": {Code: KeyKp6}, "w": {Code: KeyKp7},
		"x": {Code: KeyKp8}, "y": {Code: KeyKp9},
	}

	// XTerm keys
	csiFuncKeys := map[string]Key{
		"A": {Code: KeyUp}, "B": {Code: KeyDown},
		"C": {Code: KeyRight}, "D": {Code: KeyLeft},
		"E": {Code: KeyBegin}, "F": {Code: KeyEnd},
		"H": {Code: KeyHome}, "P": {Code: KeyF1},
		"Q": {Code: KeyF2}, "R": {Code: KeyF3},
		"S": {Code: KeyF4},
	}

	// CSI 27 ; <modifier> ; <code> ~ keys defined in XTerm modifyOtherKeys
	modifyOtherKeys := map[int]Key{
		ansi.BS:  {Code: KeyBackspace},
		ansi.HT:  {Code: KeyTab},
		ansi.CR:  {Code: KeyEnter},
		ansi.ESC: {Code: KeyEscape},
		ansi.DEL: {Code: KeyBackspace},
	}

	for _, m := range modifiers {
		// XTerm modifier offset +1
		xtermMod := strconv.Itoa(int(m) + 1)

		//  CSI 1 ; <modifier> <func>
		for k, v := range csiFuncKeys {
			// Functions always have a leading 1 param
			seq := "\x1b[1;" + xtermMod + k
			key := v
			key.Mod = m
			table[seq] = key
		}
		// SS3 <modifier> <func>
		for k, v := range ss3FuncKeys {
			seq := "\x1bO" + xtermMod + k
			key := v
			key.Mod = m
			table[seq] = key
		}
		//  CSI <number> ; <modifier> ~
		for k, v := range csiTildeKeys {
			seq := "\x1b[" + k + ";" + xtermMod + "~"
			key := v
			key.Mod = m
			table[seq] = key
		}
		// CSI 27 ; <modifier> ; <code> ~
		for k, v := range modifyOtherKeys {
			code := strconv.Itoa(k)
			seq := "\x1b[27;" + xtermMod + ";" + code + "~"
			key := v
			key.Mod = m
			table[seq] = key
		}
	}

	// Register terminfo keys
	// XXX: this might override keys already registered in table
	if useTerminfo {
		titable := buildTerminfoKeys(flags, term)
		for seq, key := range titable {
			table[seq] = key
		}
	}

	return table
}

func buildTerminfoKeys(flags LegacyKeyEncoding, term string) map[string]Key {
	table := make(map[string]Key)
	ti, _ := terminfo.Load(term)
	if ti == nil {
		return table
	}

	tiTable := defaultTerminfoKeys(flags)

	// Default keys
	for name, seq := range ti.StringCapsShort() {
		if !strings.HasPrefix(name, "k") || len(seq) == 0 {
			continue
		}

		if k, ok := tiTable[name]; ok {
			table[string(seq)] = k
		}
	}

	// Extended keys
	for name, seq := range ti.ExtStringCapsShort() {
		if !strings.HasPrefix(name, "k") || len(seq) == 0 {
			continue
		}

		if k, ok := tiTable[name]; ok {
			table[string(seq)] = k
		}
	}

	return table
}

// This returns a map of terminfo keys to key events. It's a mix of ncurses
// terminfo default and user-defined key capabilities.
// Upper-case caps that are defined in the default terminfo database are
//   - kNXT
//   - kPRV
//   - kHOM
//   - kEND
//   - kDC
//   - kIC
//   - kLFT
//   - kRIT
//
// See https://man7.org/linux/man-pages/man5/terminfo.5.html
// See https://github.com/mirror/ncurses/blob/master/include/Caps-ncurses
func defaultTerminfoKeys(flags LegacyKeyEncoding) map[string]Key {
	keys := map[string]Key{
		"kcuu1": {Code: KeyUp},
		"kUP":   {Code: KeyUp, Mod: ModShift},
		"kUP3":  {Code: KeyUp, Mod: ModAlt},
		"kUP4":  {Code: KeyUp, Mod: ModShift | ModAlt},
		"kUP5":  {Code: KeyUp, Mod: ModCtrl},
		"kUP6":  {Code: KeyUp, Mod: ModShift | ModCtrl},
		"kUP7":  {Code: KeyUp, Mod: ModAlt | ModCtrl},
		"kUP8":  {Code: KeyUp, Mod: ModShift | ModAlt | ModCtrl},
		"kcud1": {Code: KeyDown},
		"kDN":   {Code: KeyDown, Mod: ModShift},
		"kDN3":  {Code: KeyDown, Mod: ModAlt},
		"kDN4":  {Code: KeyDown, Mod: ModShift | ModAlt},
		"kDN5":  {Code: KeyDown, Mod: ModCtrl},
		"kDN7":  {Code: KeyDown, Mod: ModAlt | ModCtrl},
		"kDN6":  {Code: KeyDown, Mod: ModShift | ModCtrl},
		"kDN8":  {Code: KeyDown, Mod: ModShift | ModAlt | ModCtrl},
		"kcub1": {Code: KeyLeft},
		"kLFT":  {Code: KeyLeft, Mod: ModShift},
		"kLFT3": {Code: KeyLeft, Mod: ModAlt},
		"kLFT4": {Code: KeyLeft, Mod: ModShift | ModAlt},
		"kLFT5": {Code: KeyLeft, Mod: ModCtrl},
		"kLFT6": {Code: KeyLeft, Mod: ModShift | ModCtrl},
		"kLFT7": {Code: KeyLeft, Mod: ModAlt | ModCtrl},
		"kLFT8": {Code: KeyLeft, Mod: ModShift | ModAlt | ModCtrl},
		"kcuf1": {Code: KeyRight},
		"kRIT":  {Code: KeyRight, Mod: ModShift},
		"kRIT3": {Code: KeyRight, Mod: ModAlt},
		"kRIT4": {Code: KeyRight, Mod: ModShift | ModAlt},
		"kRIT5": {Code: KeyRight, Mod: ModCtrl},
		"kRIT6": {Code: KeyRight, Mod: ModShift | ModCtrl},
		"kRIT7": {Code: KeyRight, Mod: ModAlt | ModCtrl},
		"kRIT8": {Code: KeyRight, Mod: ModShift | ModAlt | ModCtrl},
		"kich1": {Code: KeyInsert},
		"kIC":   {Code: KeyInsert, Mod: ModShift},
		"kIC3":  {Code: KeyInsert, Mod: ModAlt},
		"kIC4":  {Code: KeyInsert, Mod: ModShift | ModAlt},
		"kIC5":  {Code: KeyInsert, Mod: ModCtrl},
		"kIC6":  {Code: KeyInsert, Mod: ModShift | ModCtrl},
		"kIC7":  {Code: KeyInsert, Mod: ModAlt | ModCtrl},
		"kIC8":  {Code: KeyInsert, Mod: ModShift | ModAlt | ModCtrl},
		"kdch1": {Code: KeyDelete},
		"kDC":   {Code: KeyDelete, Mod: ModShift},
		"kDC3":  {Code: KeyDelete, Mod: ModAlt},
		"kDC4":  {Code: KeyDelete, Mod: ModShift | ModAlt},
		"kDC5":  {Code: KeyDelete, Mod: ModCtrl},
		"kDC6":  {Code: KeyDelete, Mod: ModShift | ModCtrl},
		"kDC7":  {Code: KeyDelete, Mod: ModAlt | ModCtrl},
		"kDC8":  {Code: KeyDelete, Mod: ModShift | ModAlt | ModCtrl},
		"khome": {Code: KeyHome},
		"kHOM":  {Code: KeyHome, Mod: ModShift},
		"kHOM3": {Code: KeyHome, Mod: ModAlt},
		"kHOM4": {Code: KeyHome, Mod: ModShift | ModAlt},
		"kHOM5": {Code: KeyHome, Mod: ModCtrl},
		"kHOM6": {Code: KeyHome, Mod: ModShift | ModCtrl},
		"kHOM7": {Code: KeyHome, Mod: ModAlt | ModCtrl},
		"kHOM8": {Code: KeyHome, Mod: ModShift | ModAlt | ModCtrl},
		"kend":  {Code: KeyEnd},
		"kEND":  {Code: KeyEnd, Mod: ModShift},
		"kEND3": {Code: KeyEnd, Mod: ModAlt},
		"kEND4": {Code: KeyEnd, Mod: ModShift | ModAlt},
		"kEND5": {Code: KeyEnd, Mod: ModCtrl},
		"kEND6": {Code: KeyEnd, Mod: ModShift | ModCtrl},
		"kEND7": {Code: KeyEnd, Mod: ModAlt | ModCtrl},
		"kEND8": {Code: KeyEnd, Mod: ModShift | ModAlt | ModCtrl},
		"kpp":   {Code: KeyPgUp},
		"kprv":  {Code: KeyPgUp},
		"kPRV":  {Code: KeyPgUp, Mod: ModShift},
		"kPRV3": {Code: KeyPgUp, Mod: ModAlt},
		"kPRV4": {Code: KeyPgUp, Mod: ModShift | ModAlt},
		"kPRV5": {Code: KeyPgUp, Mod: ModCtrl},
		"kPRV6": {Code: KeyPgUp, Mod: ModShift | ModCtrl},
		"kPRV7": {Code: KeyPgUp, Mod: ModAlt | ModCtrl},
		"kPRV8": {Code: KeyPgUp, Mod: ModShift | ModAlt | ModCtrl},
		"knp":   {Code: KeyPgDown},
		"knxt":  {Code: KeyPgDown},
		"kNXT":  {Code: KeyPgDown, Mod: ModShift},
		"kNXT3": {Code: KeyPgDown, Mod: ModAlt},
		"kNXT4": {Code: KeyPgDown, Mod: ModShift | ModAlt},
		"kNXT5": {Code: KeyPgDown, Mod: ModCtrl},
		"kNXT6": {Code: KeyPgDown, Mod: ModShift | ModCtrl},
		"kNXT7": {Code: KeyPgDown, Mod: ModAlt | ModCtrl},
		"kNXT8": {Code: KeyPgDown, Mod: ModShift | ModAlt | ModCtrl},

		"kbs":  {Code: KeyBackspace},
		"kcbt": {Code: KeyTab, Mod: ModShift},

		// Function keys
		// This only includes the first 12 function keys. The rest are treated
		// as modifiers of the first 12.
		// Take a look at XTerm modifyFunctionKeys
		//
		// XXX: To use unambiguous function keys, use fixterms or kitty clipboard.
		//
		// See https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyFunctionKeys
		// See https://invisible-island.net/xterm/terminfo.html

		"kf1":  {Code: KeyF1},
		"kf2":  {Code: KeyF2},
		"kf3":  {Code: KeyF3},
		"kf4":  {Code: KeyF4},
		"kf5":  {Code: KeyF5},
		"kf6":  {Code: KeyF6},
		"kf7":  {Code: KeyF7},
		"kf8":  {Code: KeyF8},
		"kf9":  {Code: KeyF9},
		"kf10": {Code: KeyF10},
		"kf11": {Code: KeyF11},
		"kf12": {Code: KeyF12},
		"kf13": {Code: KeyF1, Mod: ModShift},
		"kf14": {Code: KeyF2, Mod: ModShift},
		"kf15": {Code: KeyF3, Mod: ModShift},
		"kf16": {Code: KeyF4, Mod: ModShift},
		"kf17": {Code: KeyF5, Mod: ModShift},
		"kf18": {Code: KeyF6, Mod: ModShift},
		"kf19": {Code: KeyF7, Mod: ModShift},
		"kf20": {Code: KeyF8, Mod: ModShift},
		"kf21": {Code: KeyF9, Mod: ModShift},
		"kf22": {Code: KeyF10, Mod: ModShift},
		"kf23": {Code: KeyF11, Mod: ModShift},
		"kf24": {Code: KeyF12, Mod: ModShift},
		"kf25": {Code: KeyF1, Mod: ModCtrl},
		"kf26": {Code: KeyF2, Mod: ModCtrl},
		"kf27": {Code: KeyF3, Mod: ModCtrl},
		"kf28": {Code: KeyF4, Mod: ModCtrl},
		"kf29": {Code: KeyF5, Mod: ModCtrl},
		"kf30": {Code: KeyF6, Mod: ModCtrl},
		"kf31": {Code: KeyF7, Mod: ModCtrl},
		"kf32": {Code: KeyF8, Mod: ModCtrl},
		"kf33": {Code: KeyF9, Mod: ModCtrl},
		"kf34": {Code: KeyF10, Mod: ModCtrl},
		"kf35": {Code: KeyF11, Mod: ModCtrl},
		"kf36": {Code: KeyF12, Mod: ModCtrl},
		"kf37": {Code: KeyF1, Mod: ModShift | ModCtrl},
		"kf38": {Code: KeyF2, Mod: ModShift | ModCtrl},
		"kf39": {Code: KeyF3, Mod: ModShift | ModCtrl},
		"kf40": {Code: KeyF4, Mod: ModShift | ModCtrl},
		"kf41": {Code: KeyF5, Mod: ModShift | ModCtrl},
		"kf42": {Code: KeyF6, Mod: ModShift | ModCtrl},
		"kf43": {Code: KeyF7, Mod: ModShift | ModCtrl},
		"kf44": {Code: KeyF8, Mod: ModShift | ModCtrl},
		"kf45": {Code: KeyF9, Mod: ModShift | ModCtrl},
		"kf46": {Code: KeyF10, Mod: ModShift | ModCtrl},
		"kf47": {Code: KeyF11, Mod: ModShift | ModCtrl},
		"kf48": {Code: KeyF12, Mod: ModShift | ModCtrl},
		"kf49": {Code: KeyF1, Mod: ModAlt},
		"kf50": {Code: KeyF2, Mod: ModAlt},
		"kf51": {Code: KeyF3, Mod: ModAlt},
		"kf52": {Code: KeyF4, Mod: ModAlt},
		"kf53": {Code: KeyF5, Mod: ModAlt},
		"kf54": {Code: KeyF6, Mod: ModAlt},
		"kf55": {Code: KeyF7, Mod: ModAlt},
		"kf56": {Code: KeyF8, Mod: ModAlt},
		"kf57": {Code: KeyF9, Mod: ModAlt},
		"kf58": {Code: KeyF10, Mod: ModAlt},
		"kf59": {Code: KeyF11, Mod: ModAlt},
		"kf60": {Code: KeyF12, Mod: ModAlt},
		"kf61": {Code: KeyF1, Mod: ModShift | ModAlt},
		"kf62": {Code: KeyF2, Mod: ModShift | ModAlt},
		"kf63": {Code: KeyF3, Mod: ModShift | ModAlt},
	}

	// Preserve F keys from F13 to F63 instead of using them for F-keys
	// modifiers.
	if flags&flagFKeys != 0 {
		keys["kf13"] = Key{Code: KeyF13}
		keys["kf14"] = Key{Code: KeyF14}
		keys["kf15"] = Key{Code: KeyF15}
		keys["kf16"] = Key{Code: KeyF16}
		keys["kf17"] = Key{Code: KeyF17}
		keys["kf18"] = Key{Code: KeyF18}
		keys["kf19"] = Key{Code: KeyF19}
		keys["kf20"] = Key{Code: KeyF20}
		keys["kf21"] = Key{Code: KeyF21}
		keys["kf22"] = Key{Code: KeyF22}
		keys["kf23"] = Key{Code: KeyF23}
		keys["kf24"] = Key{Code: KeyF24}
		keys["kf25"] = Key{Code: KeyF25}
		keys["kf26"] = Key{Code: KeyF26}
		keys["kf27"] = Key{Code: KeyF27}
		keys["kf28"] = Key{Code: KeyF28}
		keys["kf29"] = Key{Code: KeyF29}
		keys["kf30"] = Key{Code: KeyF30}
		keys["kf31"] = Key{Code: KeyF31}
		keys["kf32"] = Key{Code: KeyF32}
		keys["kf33"] = Key{Code: KeyF33}
		keys["kf34"] = Key{Code: KeyF34}
		keys["kf35"] = Key{Code: KeyF35}
		keys["kf36"] = Key{Code: KeyF36}
		keys["kf37"] = Key{Code: KeyF37}
		keys["kf38"] = Key{Code: KeyF38}
		keys["kf39"] = Key{Code: KeyF39}
		keys["kf40"] = Key{Code: KeyF40}
		keys["kf41"] = Key{Code: KeyF41}
		keys["kf42"] = Key{Code: KeyF42}
		keys["kf43"] = Key{Code: KeyF43}
		keys["kf44"] = Key{Code: KeyF44}
		keys["kf45"] = Key{Code: KeyF45}
		keys["kf46"] = Key{Code: KeyF46}
		keys["kf47"] = Key{Code: KeyF47}
		keys["kf48"] = Key{Code: KeyF48}
		keys["kf49"] = Key{Code: KeyF49}
		keys["kf50"] = Key{Code: KeyF50}
		keys["kf51"] = Key{Code: KeyF51}
		keys["kf52"] = Key{Code: KeyF52}
		keys["kf53"] = Key{Code: KeyF53}
		keys["kf54"] = Key{Code: KeyF54}
		keys["kf55"] = Key{Code: KeyF55}
		keys["kf56"] = Key{Code: KeyF56}
		keys["kf57"] = Key{Code: KeyF57}
		keys["kf58"] = Key{Code: KeyF58}
		keys["kf59"] = Key{Code: KeyF59}
		keys["kf60"] = Key{Code: KeyF60}
		keys["kf61"] = Key{Code: KeyF61}
		keys["kf62"] = Key{Code: KeyF62}
		keys["kf63"] = Key{Code: KeyF63}
	}

	return keys
}
