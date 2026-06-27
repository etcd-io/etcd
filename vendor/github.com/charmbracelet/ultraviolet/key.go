package uv

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// KeyMod represents modifier keys.
type KeyMod int

// Modifier keys.
const (
	ModShift KeyMod = 1 << iota
	ModAlt
	ModCtrl
	ModMeta

	// These modifiers are used with the Kitty protocol.
	// XXX: Meta and Super are swapped in the Kitty protocol,
	// this is to preserve compatibility with XTerm modifiers.

	ModHyper
	ModSuper // Windows/Command keys

	// These are key lock states.

	ModCapsLock
	ModNumLock
	ModScrollLock // Defined in Windows API only
)

// Contains reports whether m contains the given modifiers.
//
// Example:
//
//	m := ModAlt | ModCtrl
//	m.Contains(ModCtrl) // true
//	m.Contains(ModAlt | ModCtrl) // true
//	m.Contains(ModAlt | ModCtrl | ModShift) // false
func (m KeyMod) Contains(mods KeyMod) bool {
	return m&mods == mods
}

const (
	// KeyExtended is a special key code used to signify that a key event
	// contains multiple runes.
	KeyExtended = unicode.MaxRune + 1
)

// Special key symbols.
const (

	// Special keys.

	KeyUp rune = KeyExtended + iota + 1
	KeyDown
	KeyRight
	KeyLeft
	KeyBegin
	KeyFind
	KeyInsert
	KeyDelete
	KeySelect
	KeyPgUp
	KeyPgDown
	KeyHome
	KeyEnd

	// Keypad keys.

	KeyKpEnter
	KeyKpEqual
	KeyKpMultiply
	KeyKpPlus
	KeyKpComma
	KeyKpMinus
	KeyKpDecimal
	KeyKpDivide
	KeyKp0
	KeyKp1
	KeyKp2
	KeyKp3
	KeyKp4
	KeyKp5
	KeyKp6
	KeyKp7
	KeyKp8
	KeyKp9

	//nolint:godox
	// The following are keys defined in the Kitty keyboard protocol.
	// TODO: Investigate the names of these keys.

	KeyKpSep
	KeyKpUp
	KeyKpDown
	KeyKpLeft
	KeyKpRight
	KeyKpPgUp
	KeyKpPgDown
	KeyKpHome
	KeyKpEnd
	KeyKpInsert
	KeyKpDelete
	KeyKpBegin

	// Function keys.

	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyF21
	KeyF22
	KeyF23
	KeyF24
	KeyF25
	KeyF26
	KeyF27
	KeyF28
	KeyF29
	KeyF30
	KeyF31
	KeyF32
	KeyF33
	KeyF34
	KeyF35
	KeyF36
	KeyF37
	KeyF38
	KeyF39
	KeyF40
	KeyF41
	KeyF42
	KeyF43
	KeyF44
	KeyF45
	KeyF46
	KeyF47
	KeyF48
	KeyF49
	KeyF50
	KeyF51
	KeyF52
	KeyF53
	KeyF54
	KeyF55
	KeyF56
	KeyF57
	KeyF58
	KeyF59
	KeyF60
	KeyF61
	KeyF62
	KeyF63

	//nolint:godox
	// The following are keys defined in the Kitty keyboard protocol.
	// TODO: Investigate the names of these keys.

	KeyCapsLock
	KeyScrollLock
	KeyNumLock
	KeyPrintScreen
	KeyPause
	KeyMenu

	KeyMediaPlay
	KeyMediaPause
	KeyMediaPlayPause
	KeyMediaReverse
	KeyMediaStop
	KeyMediaFastForward
	KeyMediaRewind
	KeyMediaNext
	KeyMediaPrev
	KeyMediaRecord

	KeyLowerVol
	KeyRaiseVol
	KeyMute

	KeyLeftShift
	KeyLeftAlt
	KeyLeftCtrl
	KeyLeftSuper
	KeyLeftHyper
	KeyLeftMeta
	KeyRightShift
	KeyRightAlt
	KeyRightCtrl
	KeyRightSuper
	KeyRightHyper
	KeyRightMeta
	KeyIsoLevel3Shift
	KeyIsoLevel5Shift

	// Special names in C0.

	KeyBackspace = rune(ansi.DEL)
	KeyTab       = rune(ansi.HT)
	KeyEnter     = rune(ansi.CR)
	KeyReturn    = KeyEnter
	KeyEscape    = rune(ansi.ESC)
	KeyEsc       = KeyEscape

	// Special names in G0.

	KeySpace = rune(ansi.SP)
)

// Key represents a Key press or release event. It contains information about
// the Key pressed, like the runes, the type of Key, and the modifiers pressed.
// There are a couple general patterns you could use to check for key presses
// or releases:
//
//	// Switch on the string representation of the key (shorter)
//	switch ev := ev.(type) {
//	case KeyPressEvent:
//	    switch ev.String() {
//	    case "enter":
//	        fmt.Println("you pressed enter!")
//	    case "a":
//	        fmt.Println("you pressed a!")
//	    }
//	}
//
//	// Switch on the key type (more foolproof)
//	switch ev := ev.(type) {
//	case KeyEvent:
//	    // catch both KeyPressEvent and KeyReleaseEvent
//	    switch key := ev.Key(); key.Code {
//	    case KeyEnter:
//	        fmt.Println("you pressed enter!")
//	    default:
//	        switch key.Text {
//	        case "a":
//	            fmt.Println("you pressed a!")
//	        }
//	    }
//	}
//
// Note that [Key.Text] will be empty for special keys like [KeyEnter],
// [KeyTab], and for keys that don't represent printable characters like key
// combos with modifier keys. In other words, [Key.Text] is populated only for
// keys that represent printable characters shifted or unshifted (like 'a',
// 'A', '1', '!', etc.).
type Key struct {
	// Text contains the actual characters received. This usually the same as
	// [Key.Code]. When [Key.Text] is non-empty, it indicates that the key
	// pressed represents printable character(s).
	Text string

	// Mod represents modifier keys, like [ModCtrl], [ModAlt], and so on.
	Mod KeyMod

	// Code represents the key pressed. This is usually a special key like
	// [KeyTab], [KeyEnter], [KeyF1], or a printable character like 'a'.
	Code rune

	// ShiftedCode is the actual, shifted key pressed by the user. For example,
	// if the user presses shift+a, or caps lock is on, [Key.ShiftedCode] will
	// be 'A' and [Key.Code] will be 'a'.
	//
	// In the case of non-latin keyboards, like Arabic, [Key.ShiftedCode] is the
	// unshifted key on the keyboard.
	//
	// This is only available with the Kitty Keyboard Protocol or the Windows
	// Console API.
	ShiftedCode rune

	// BaseCode is the key pressed according to the standard PC-101 key layout.
	// On international keyboards, this is the key that would be pressed if the
	// keyboard was set to US PC-101 layout.
	//
	// For example, if the user presses 'q' on a French AZERTY keyboard,
	// [Key.BaseCode] will be 'q'.
	//
	// This is only available with the Kitty Keyboard Protocol or the Windows
	// Console API.
	BaseCode rune

	// IsRepeat indicates whether the key is being held down and sending events
	// repeatedly.
	//
	// This is only available with the Kitty Keyboard Protocol or the Windows
	// Console API.
	IsRepeat bool
}

// MatchString returns true if the [Key] matches one of the given strings.
//
// A string can be a key name like "enter", "tab", "a", or a printable
// character like "1" or " ". It can also have combinations of modifiers like
// "ctrl+a", "shift+enter", "alt+tab", "ctrl+shift+enter", etc.
func (k Key) MatchString(s ...string) bool {
	for _, s := range s {
		if keyMatchString(k, s) {
			return true
		}
	}
	return false
}

func keyMatchString(k Key, s string) bool {
	var (
		mod  KeyMod
		code rune
		text string
	)
	parts := strings.Split(s, "+")
	for _, part := range parts {
		switch part {
		case "ctrl":
			mod |= ModCtrl
		case "alt":
			mod |= ModAlt
		case "shift":
			mod |= ModShift
		case "meta":
			mod |= ModMeta
		case "hyper":
			mod |= ModHyper
		case "super":
			mod |= ModSuper
		case "capslock":
			mod |= ModCapsLock
		case "scrolllock":
			mod |= ModScrollLock
		case "numlock":
			mod |= ModNumLock
		default:
			// Check if the part is a key name.
			if k, ok := stringKeyType[part]; ok {
				code = k
			} else {
				// Check if the part is a printable character.
				if utf8.RuneCountInString(part) == 1 {
					code, _ = utf8.DecodeRuneInString(part)
				} else {
					// Multi-rune key.
					code = KeyExtended
					text = part
				}
			}
		}
	}

	// Check if we have a printable character.
	smod := mod &^ (ModShift | ModCapsLock)
	if smod == 0 && text == "" && unicode.IsPrint(code) {
		if mod&ModShift != 0 || mod&ModCapsLock != 0 {
			// Shifted code we need to use uppercase.
			text = string(unicode.ToUpper(code))
		} else {
			// Otherwise, use the code as is.
			text = string(code)
		}
	}

	// Check if we have a match.
	return (k.Mod == mod && k.Code == code) ||
		(k.Text != "" && k.Text == text)
}

// String implements [fmt.Stringer] and is quite useful for matching key
// events. It will return the textual representation of the [Key] if there is
// one, otherwise, it will fallback to [Key.Keystroke].
//
// For example, you'll always get "?" and instead of "shift+/" on a US ANSI
// keyboard.
func (k Key) String() string {
	if len(k.Text) > 0 && k.Text != " " {
		return k.Text
	}
	return k.Keystroke()
}

// Keystroke returns the keystroke representation of the [Key]. While less type
// safe than looking at the individual fields, it will usually be more
// convenient and readable to use this method when matching against keys.
//
// Note that modifier keys are always printed in the following order:
//   - ctrl
//   - alt
//   - shift
//   - meta
//   - hyper
//   - super
//
// For example, you'll always see "ctrl+shift+alt+a" and never
// "shift+ctrl+alt+a".
func (k Key) Keystroke() string {
	var sb strings.Builder
	if k.Mod.Contains(ModCtrl) && k.Code != KeyLeftCtrl && k.Code != KeyRightCtrl {
		sb.WriteString("ctrl+")
	}
	if k.Mod.Contains(ModAlt) && k.Code != KeyLeftAlt && k.Code != KeyRightAlt {
		sb.WriteString("alt+")
	}
	if k.Mod.Contains(ModShift) && k.Code != KeyLeftShift && k.Code != KeyRightShift {
		sb.WriteString("shift+")
	}
	if k.Mod.Contains(ModMeta) && k.Code != KeyLeftMeta && k.Code != KeyRightMeta {
		sb.WriteString("meta+")
	}
	if k.Mod.Contains(ModHyper) && k.Code != KeyLeftHyper && k.Code != KeyRightHyper {
		sb.WriteString("hyper+")
	}
	if k.Mod.Contains(ModSuper) && k.Code != KeyLeftSuper && k.Code != KeyRightSuper {
		sb.WriteString("super+")
	}

	if kt, ok := keyTypeString[k.Code]; ok {
		sb.WriteString(kt)
	} else {
		code := k.Code
		if k.BaseCode != 0 {
			// If a [Key.BaseCode] is present, use it to represent a key using the standard
			// PC-101 key layout.
			code = k.BaseCode
		}

		switch code {
		case KeySpace:
			// Space is the only invisible printable character.
			sb.WriteString("space")
		case KeyExtended:
			// Write the actual text of the key when the key contains multiple
			// runes.
			sb.WriteString(k.Text)
		default:
			sb.WriteRune(code)
		}
	}

	return sb.String()
}

var keyTypeString = map[rune]string{
	KeyEnter:      "enter",
	KeyTab:        "tab",
	KeyBackspace:  "backspace",
	KeyEscape:     "esc",
	KeySpace:      "space",
	KeyUp:         "up",
	KeyDown:       "down",
	KeyLeft:       "left",
	KeyRight:      "right",
	KeyBegin:      "begin",
	KeyFind:       "find",
	KeyInsert:     "insert",
	KeyDelete:     "delete",
	KeySelect:     "select",
	KeyPgUp:       "pgup",
	KeyPgDown:     "pgdown",
	KeyHome:       "home",
	KeyEnd:        "end",
	KeyKpEnter:    "enter",
	KeyKpEqual:    "equal",
	KeyKpMultiply: "mul",
	KeyKpPlus:     "plus",
	KeyKpComma:    "comma",
	KeyKpMinus:    "minus",
	KeyKpDecimal:  "period",
	KeyKpDivide:   "div",
	KeyKp0:        "0",
	KeyKp1:        "1",
	KeyKp2:        "2",
	KeyKp3:        "3",
	KeyKp4:        "4",
	KeyKp5:        "5",
	KeyKp6:        "6",
	KeyKp7:        "7",
	KeyKp8:        "8",
	KeyKp9:        "9",

	// Kitty keyboard extension
	KeyKpSep:    "sep",
	KeyKpUp:     "up",
	KeyKpDown:   "down",
	KeyKpLeft:   "left",
	KeyKpRight:  "right",
	KeyKpPgUp:   "pgup",
	KeyKpPgDown: "pgdown",
	KeyKpHome:   "home",
	KeyKpEnd:    "end",
	KeyKpInsert: "insert",
	KeyKpDelete: "delete",
	KeyKpBegin:  "begin",

	KeyF1:  "f1",
	KeyF2:  "f2",
	KeyF3:  "f3",
	KeyF4:  "f4",
	KeyF5:  "f5",
	KeyF6:  "f6",
	KeyF7:  "f7",
	KeyF8:  "f8",
	KeyF9:  "f9",
	KeyF10: "f10",
	KeyF11: "f11",
	KeyF12: "f12",
	KeyF13: "f13",
	KeyF14: "f14",
	KeyF15: "f15",
	KeyF16: "f16",
	KeyF17: "f17",
	KeyF18: "f18",
	KeyF19: "f19",
	KeyF20: "f20",
	KeyF21: "f21",
	KeyF22: "f22",
	KeyF23: "f23",
	KeyF24: "f24",
	KeyF25: "f25",
	KeyF26: "f26",
	KeyF27: "f27",
	KeyF28: "f28",
	KeyF29: "f29",
	KeyF30: "f30",
	KeyF31: "f31",
	KeyF32: "f32",
	KeyF33: "f33",
	KeyF34: "f34",
	KeyF35: "f35",
	KeyF36: "f36",
	KeyF37: "f37",
	KeyF38: "f38",
	KeyF39: "f39",
	KeyF40: "f40",
	KeyF41: "f41",
	KeyF42: "f42",
	KeyF43: "f43",
	KeyF44: "f44",
	KeyF45: "f45",
	KeyF46: "f46",
	KeyF47: "f47",
	KeyF48: "f48",
	KeyF49: "f49",
	KeyF50: "f50",
	KeyF51: "f51",
	KeyF52: "f52",
	KeyF53: "f53",
	KeyF54: "f54",
	KeyF55: "f55",
	KeyF56: "f56",
	KeyF57: "f57",
	KeyF58: "f58",
	KeyF59: "f59",
	KeyF60: "f60",
	KeyF61: "f61",
	KeyF62: "f62",
	KeyF63: "f63",

	// Kitty keyboard extension
	KeyCapsLock:         "capslock",
	KeyScrollLock:       "scrolllock",
	KeyNumLock:          "numlock",
	KeyPrintScreen:      "printscreen",
	KeyPause:            "pause",
	KeyMenu:             "menu",
	KeyMediaPlay:        "mediaplay",
	KeyMediaPause:       "mediapause",
	KeyMediaPlayPause:   "mediaplaypause",
	KeyMediaReverse:     "mediareverse",
	KeyMediaStop:        "mediastop",
	KeyMediaFastForward: "mediafastforward",
	KeyMediaRewind:      "mediarewind",
	KeyMediaNext:        "medianext",
	KeyMediaPrev:        "mediaprev",
	KeyMediaRecord:      "mediarecord",
	KeyLowerVol:         "lowervol",
	KeyRaiseVol:         "raisevol",
	KeyMute:             "mute",
	KeyLeftShift:        "leftshift",
	KeyLeftAlt:          "leftalt",
	KeyLeftCtrl:         "leftctrl",
	KeyLeftSuper:        "leftsuper",
	KeyLeftHyper:        "lefthyper",
	KeyLeftMeta:         "leftmeta",
	KeyRightShift:       "rightshift",
	KeyRightAlt:         "rightalt",
	KeyRightCtrl:        "rightctrl",
	KeyRightSuper:       "rightsuper",
	KeyRightHyper:       "righthyper",
	KeyRightMeta:        "rightmeta",
	KeyIsoLevel3Shift:   "isolevel3shift",
	KeyIsoLevel5Shift:   "isolevel5shift",
}

var stringKeyType = map[string]rune{
	"enter":     KeyEnter,
	"tab":       KeyTab,
	"backspace": KeyBackspace,
	"escape":    KeyEscape,
	"esc":       KeyEscape,
	"space":     KeySpace,
	"up":        KeyUp,
	"down":      KeyDown,
	"left":      KeyLeft,
	"right":     KeyRight,
	"begin":     KeyBegin,
	"find":      KeyFind,
	"insert":    KeyInsert,
	"delete":    KeyDelete,
	"select":    KeySelect,
	"pgup":      KeyPgUp,
	"pgdown":    KeyPgDown,
	"home":      KeyHome,
	"end":       KeyEnd,
	"kpenter":   KeyKpEnter,
	"kpequal":   KeyKpEqual,
	"kpmul":     KeyKpMultiply,
	"kpplus":    KeyKpPlus,
	"kpcomma":   KeyKpComma,
	"kpminus":   KeyKpMinus,
	"kpperiod":  KeyKpDecimal,
	"kpdiv":     KeyKpDivide,
	"kp0":       KeyKp0,
	"kp1":       KeyKp1,
	"kp2":       KeyKp2,
	"kp3":       KeyKp3,
	"kp4":       KeyKp4,
	"kp5":       KeyKp5,
	"kp6":       KeyKp6,
	"kp7":       KeyKp7,
	"kp8":       KeyKp8,
	"kp9":       KeyKp9,

	// Kitty keyboard extension
	"kpsep":    KeyKpSep,
	"kpup":     KeyKpUp,
	"kpdown":   KeyKpDown,
	"kpleft":   KeyKpLeft,
	"kpright":  KeyKpRight,
	"kppgup":   KeyKpPgUp,
	"kppgdown": KeyKpPgDown,
	"kphome":   KeyKpHome,
	"kpend":    KeyKpEnd,
	"kpinsert": KeyKpInsert,
	"kpdelete": KeyKpDelete,
	"kpbegin":  KeyKpBegin,

	"f1":  KeyF1,
	"f2":  KeyF2,
	"f3":  KeyF3,
	"f4":  KeyF4,
	"f5":  KeyF5,
	"f6":  KeyF6,
	"f7":  KeyF7,
	"f8":  KeyF8,
	"f9":  KeyF9,
	"f10": KeyF10,
	"f11": KeyF11,
	"f12": KeyF12,
	"f13": KeyF13,
	"f14": KeyF14,
	"f15": KeyF15,
	"f16": KeyF16,
	"f17": KeyF17,
	"f18": KeyF18,
	"f19": KeyF19,
	"f20": KeyF20,
	"f21": KeyF21,
	"f22": KeyF22,
	"f23": KeyF23,
	"f24": KeyF24,
	"f25": KeyF25,
	"f26": KeyF26,
	"f27": KeyF27,
	"f28": KeyF28,
	"f29": KeyF29,
	"f30": KeyF30,
	"f31": KeyF31,
	"f32": KeyF32,
	"f33": KeyF33,
	"f34": KeyF34,
	"f35": KeyF35,
	"f36": KeyF36,
	"f37": KeyF37,
	"f38": KeyF38,
	"f39": KeyF39,
	"f40": KeyF40,
	"f41": KeyF41,
	"f42": KeyF42,
	"f43": KeyF43,
	"f44": KeyF44,
	"f45": KeyF45,
	"f46": KeyF46,
	"f47": KeyF47,
	"f48": KeyF48,
	"f49": KeyF49,
	"f50": KeyF50,
	"f51": KeyF51,
	"f52": KeyF52,
	"f53": KeyF53,
	"f54": KeyF54,
	"f55": KeyF55,
	"f56": KeyF56,
	"f57": KeyF57,
	"f58": KeyF58,
	"f59": KeyF59,
	"f60": KeyF60,
	"f61": KeyF61,
	"f62": KeyF62,
	"f63": KeyF63,

	// Kitty keyboard extension
	"capslock":         KeyCapsLock,
	"scrolllock":       KeyScrollLock,
	"numlock":          KeyNumLock,
	"printscreen":      KeyPrintScreen,
	"pause":            KeyPause,
	"menu":             KeyMenu,
	"mediaplay":        KeyMediaPlay,
	"mediapause":       KeyMediaPause,
	"mediaplaypause":   KeyMediaPlayPause,
	"mediareverse":     KeyMediaReverse,
	"mediastop":        KeyMediaStop,
	"mediafastforward": KeyMediaFastForward,
	"mediarewind":      KeyMediaRewind,
	"medianext":        KeyMediaNext,
	"mediaprev":        KeyMediaPrev,
	"mediarecord":      KeyMediaRecord,
	"lowervol":         KeyLowerVol,
	"raisevol":         KeyRaiseVol,
	"mute":             KeyMute,
	"leftshift":        KeyLeftShift,
	"leftalt":          KeyLeftAlt,
	"leftctrl":         KeyLeftCtrl,
	"leftsuper":        KeyLeftSuper,
	"lefthyper":        KeyLeftHyper,
	"leftmeta":         KeyLeftMeta,
	"rightshift":       KeyRightShift,
	"rightalt":         KeyRightAlt,
	"rightctrl":        KeyRightCtrl,
	"rightsuper":       KeyRightSuper,
	"righthyper":       KeyRightHyper,
	"rightmeta":        KeyRightMeta,
	"isolevel3shift":   KeyIsoLevel3Shift,
	"isolevel5shift":   KeyIsoLevel5Shift,
}
