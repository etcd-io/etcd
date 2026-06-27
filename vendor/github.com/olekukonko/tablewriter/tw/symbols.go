package tw

import "fmt"

// Symbols defines the interface for table border symbols
type Symbols interface {
	// Name returns the style name
	Name() string

	// Basic component symbols
	Center() string // Junction symbol (where lines cross)
	Row() string    // Horizontal line symbol
	Column() string // Vertical line symbol

	// Corner and junction symbols
	TopLeft() string     // LevelHeader-left corner
	TopMid() string      // LevelHeader junction
	TopRight() string    // LevelHeader-right corner
	MidLeft() string     // Left junction
	MidRight() string    // Right junction
	BottomLeft() string  // LevelFooter-left corner
	BottomMid() string   // LevelFooter junction
	BottomRight() string // LevelFooter-right corner

	// Optional header-specific symbols
	HeaderLeft() string
	HeaderMid() string
	HeaderRight() string
}

// BorderStyle defines different border styling options
type BorderStyle int

// Border style constants
const (
	StyleNone BorderStyle = iota
	StyleASCII
	StyleLight
	StyleHeavy
	StyleDouble
	StyleDoubleLong
	StyleLightHeavy
	StyleHeavyLight
	StyleLightDouble
	StyleDoubleLight
	StyleRounded
	StyleMarkdown
	StyleGraphical
	StyleMerger
	StyleDefault
	StyleDotted
	StyleArrow
	StyleStarry
	StyleHearts
	StyleCircuit // Renamed from StyleTech
	StyleNature
	StyleArtistic
	Style8Bit
	StyleChaos
	StyleDots
	StyleBlocks
	StyleZen
	StyleVintage
	StyleSketch
	StyleArrowDouble
	StyleCelestial
	StyleCyber
	StyleRunic
	StyleIndustrial
	StyleInk
	StyleArcade
	StyleBlossom
	StyleFrosted
	StyleMosaic
	StyleUFO
	StyleSteampunk
	StyleGalaxy
	StyleJazz
	StylePuzzle
	StyleHypno
)

// StyleName defines names for border styles
type StyleName string

func (s StyleName) String() string {
	return string(s)
}

const (
	StyleNameNothing     StyleName = "nothing"
	StyleNameASCII       StyleName = "ascii"
	StyleNameLight       StyleName = "light"
	StyleNameHeavy       StyleName = "heavy"
	StyleNameDouble      StyleName = "double"
	StyleNameDoubleLong  StyleName = "doublelong"
	StyleNameLightHeavy  StyleName = "lightheavy"
	StyleNameHeavyLight  StyleName = "heavylight"
	StyleNameLightDouble StyleName = "lightdouble"
	StyleNameDoubleLight StyleName = "doublelight"
	StyleNameRounded     StyleName = "rounded"
	StyleNameMarkdown    StyleName = "markdown"
	StyleNameGraphical   StyleName = "graphical"
	StyleNameMerger      StyleName = "merger"
	StyleNameDotted      StyleName = "dotted"
	StyleNameArrow       StyleName = "arrow"
	StyleNameStarry      StyleName = "starry"
	StyleNameHearts      StyleName = "hearts"
	StyleNameCircuit     StyleName = "circuit" // Renamed from Tech
	StyleNameNature      StyleName = "nature"
	StyleNameArtistic    StyleName = "artistic"
	StyleName8Bit        StyleName = "8bit"
	StyleNameChaos       StyleName = "chaos"
	StyleNameDots        StyleName = "dots"
	StyleNameBlocks      StyleName = "blocks"
	StyleNameZen         StyleName = "zen"
	StyleNameVintage     StyleName = "vintage"
	StyleNameSketch      StyleName = "sketch"
	StyleNameArrowDouble StyleName = "arrowdouble"
	StyleNameCelestial   StyleName = "celestial"
	StyleNameCyber       StyleName = "cyber"
	StyleNameRunic       StyleName = "runic"
	StyleNameIndustrial  StyleName = "industrial"
	StyleNameInk         StyleName = "ink"
	StyleNameArcade      StyleName = "arcade"
	StyleNameBlossom     StyleName = "blossom"
	StyleNameFrosted     StyleName = "frosted"
	StyleNameMosaic      StyleName = "mosaic"
	StyleNameUFO         StyleName = "ufo"
	StyleNameSteampunk   StyleName = "steampunk"
	StyleNameGalaxy      StyleName = "galaxy"
	StyleNameJazz        StyleName = "jazz"
	StyleNamePuzzle      StyleName = "puzzle"
	StyleNameHypno       StyleName = "hypno"
)

// Styles maps BorderStyle to StyleName
var Styles = map[BorderStyle]StyleName{
	StyleNone:        StyleNameNothing,
	StyleASCII:       StyleNameASCII,
	StyleLight:       StyleNameLight,
	StyleHeavy:       StyleNameHeavy,
	StyleDouble:      StyleNameDouble,
	StyleDoubleLong:  StyleNameDoubleLong,
	StyleLightHeavy:  StyleNameLightHeavy,
	StyleHeavyLight:  StyleNameHeavyLight,
	StyleLightDouble: StyleNameLightDouble,
	StyleDoubleLight: StyleNameDoubleLight,
	StyleRounded:     StyleNameRounded,
	StyleMarkdown:    StyleNameMarkdown,
	StyleGraphical:   StyleNameGraphical,
	StyleMerger:      StyleNameMerger,
	StyleDefault:     StyleNameLight,
	StyleDotted:      StyleNameDotted,
	StyleArrow:       StyleNameArrow,
	StyleStarry:      StyleNameStarry,
	StyleHearts:      StyleNameHearts,
	StyleCircuit:     StyleNameCircuit,
	StyleNature:      StyleNameNature,
	StyleArtistic:    StyleNameArtistic,
	Style8Bit:        StyleName8Bit,
	StyleChaos:       StyleNameChaos,
	StyleDots:        StyleNameDots,
	StyleBlocks:      StyleNameBlocks,
	StyleZen:         StyleNameZen,
	StyleVintage:     StyleNameVintage,
	StyleSketch:      StyleNameSketch,
	StyleArrowDouble: StyleNameArrowDouble,
	StyleCelestial:   StyleNameCelestial,
	StyleCyber:       StyleNameCyber,
	StyleRunic:       StyleNameRunic,
	StyleIndustrial:  StyleNameIndustrial,
	StyleInk:         StyleNameInk,
	StyleArcade:      StyleNameArcade,
	StyleBlossom:     StyleNameBlossom,
	StyleFrosted:     StyleNameFrosted,
	StyleMosaic:      StyleNameMosaic,
	StyleUFO:         StyleNameUFO,
	StyleSteampunk:   StyleNameSteampunk,
	StyleGalaxy:      StyleNameGalaxy,
	StyleJazz:        StyleNameJazz,
	StylePuzzle:      StyleNamePuzzle,
	StyleHypno:       StyleNameHypno,
}

// String returns the string representation of a border style
func (s BorderStyle) String() string {
	return [...]string{
		"None",
		"ASCII",
		"Light",
		"Heavy",
		"Double",
		"DoubleLong",
		"LightHeavy",
		"HeavyLight",
		"LightDouble",
		"DoubleLight",
		"Rounded",
		"Markdown",
		"Graphical",
		"Merger",
		"Default",
		"Dotted",
		"Arrow",
		"Starry",
		"Hearts",
		"Circuit",
		"Nature",
		"Artistic",
		"8Bit",
		"Chaos",
		"Dots",
		"Blocks",
		"Zen",
		"Vintage",
		"Sketch",
		"ArrowDouble",
		"Celestial",
		"Cyber",
		"Runic",
		"Industrial",
		"Ink",
		"Arcade",
		"Blossom",
		"Frosted",
		"Mosaic",
		"UFO",
		"Steampunk",
		"Galaxy",
		"Jazz",
		"Puzzle",
		"Hypno",
	}[s]
}

// NewSymbols creates a new Symbols instance with the specified style
func NewSymbols(style BorderStyle) Symbols {
	switch style {
	case StyleASCII:
		return &Glyphs{
			name:   StyleNameASCII,
			row:    "-",
			column: "|",
			center: "+",
			corners: [9]string{
				"+", "+", "+",
				"+", "+", "+",
				"+", "+", "+",
			},
			headerLeft:  "+",
			headerMid:   "+",
			headerRight: "+",
		}
	case StyleLight, StyleDefault:
		return &Glyphs{
			name:   StyleNameLight,
			row:    "─",
			column: "│",
			center: "┼",
			corners: [9]string{
				"┌", "┬", "┐",
				"├", "┼", "┤",
				"└", "┴", "┘",
			},
			headerLeft:  "├",
			headerMid:   "┼",
			headerRight: "┤",
		}
	case StyleHeavy:
		return &Glyphs{
			name:   StyleNameHeavy,
			row:    "━",
			column: "┃",
			center: "╋",
			corners: [9]string{
				"┏", "┳", "┓",
				"┣", "╋", "┫",
				"┗", "┻", "┛",
			},
			headerLeft:  "┣",
			headerMid:   "╋",
			headerRight: "┫",
		}
	case StyleDouble:
		return &Glyphs{
			name:   StyleNameDouble,
			row:    "═",
			column: "║",
			center: "╬",
			corners: [9]string{
				"╔", "╦", "╗",
				"╠", "╬", "╣",
				"╚", "╩", "╝",
			},
			headerLeft:  "╠",
			headerMid:   "╬",
			headerRight: "╣",
		}
	case StyleDoubleLong:
		return &Glyphs{
			name:   StyleNameDoubleLong,
			row:    "═╡═",
			column: "╞",
			center: "╪",
			corners: [9]string{
				"╔═╡", "═╤═", "╡═╗",
				"╟ ", "╪ ", " ╢",
				"╚═╡", "═╧═", "╡═╝",
			},
			headerLeft:  "╟═╡",
			headerMid:   "╪═╡",
			headerRight: "╡═╢",
		}
	case StyleLightHeavy:
		return &Glyphs{
			name:   StyleNameLightHeavy,
			row:    "─",
			column: "┃",
			center: "╂",
			corners: [9]string{
				"┍", "┯", "┑",
				"┝", "╂", "┥",
				"┕", "┷", "┙",
			},
			headerLeft:  "┝",
			headerMid:   "╂",
			headerRight: "┥",
		}
	case StyleHeavyLight:
		return &Glyphs{
			name:   StyleNameHeavyLight,
			row:    "━",
			column: "│",
			center: "┿",
			corners: [9]string{
				"┎", "┰", "┒",
				"┠", "┿", "┨",
				"┖", "┸", "┚",
			},
			headerLeft:  "┠",
			headerMid:   "┿",
			headerRight: "┨",
		}
	case StyleLightDouble:
		return &Glyphs{
			name:   StyleNameLightDouble,
			row:    "─",
			column: "║",
			center: "╫",
			corners: [9]string{
				"╓", "╥", "╖",
				"╟", "╫", "╢",
				"╙", "╨", "╜",
			},
			headerLeft:  "╟",
			headerMid:   "╫",
			headerRight: "╢",
		}
	case StyleDoubleLight:
		return &Glyphs{
			name:   StyleNameDoubleLight,
			row:    "═",
			column: "│",
			center: "╪",
			corners: [9]string{
				"╒", "╤", "╕",
				"╞", "╪", "╡",
				"╘", "╧", "╛",
			},
			headerLeft:  "╞",
			headerMid:   "╪",
			headerRight: "╡",
		}
	case StyleRounded:
		return &Glyphs{
			name:   StyleNameRounded,
			row:    "─",
			column: "│",
			center: "┼",
			corners: [9]string{
				"╭", "┬", "╮",
				"├", "┼", "┤",
				"╰", "┴", "╯",
			},
			headerLeft:  "├",
			headerMid:   "┼",
			headerRight: "┤",
		}
	case StyleMarkdown:
		return &Glyphs{
			name:   StyleNameMarkdown,
			row:    "-",
			column: "|",
			center: "|",
			corners: [9]string{
				"", "", "",
				"|", "|", "|",
				"", "", "",
			},
			headerLeft:  "|",
			headerMid:   "|",
			headerRight: "|",
		}
	case StyleGraphical:
		return &Glyphs{
			name:   StyleNameGraphical,
			row:    "┄┄",
			column: "┆",
			center: "╂",
			corners: [9]string{
				"┌┄", "┄┄", "┄┐",
				"┆ ", "╂ ", " ┆",
				"└┄", "┄┄", "┄┘",
			},
			headerLeft:  "├┄",
			headerMid:   "╂┄",
			headerRight: "┄┤",
		}
	case StyleMerger:
		return &Glyphs{
			name:   StyleNameMerger,
			row:    "─",
			column: "│",
			center: "+",
			corners: [9]string{
				"┌", "┬", "┐",
				"├", "┼", "┤",
				"└", "┴", "┘",
			},
			headerLeft:  "├",
			headerMid:   "+",
			headerRight: "┤",
		}
	case StyleDotted:
		return &Glyphs{
			name:   StyleNameDotted,
			row:    "·",
			column: ":",
			center: "+",
			corners: [9]string{
				".", "·", ".",
				":", "+", ":",
				"'", "·", "'",
			},
			headerLeft:  ":",
			headerMid:   "+",
			headerRight: ":",
		}
	case StyleArrow:
		return &Glyphs{
			name:   StyleNameArrow,
			row:    "→",
			column: "↓",
			center: "↔",
			corners: [9]string{
				"↗", "↑", "↖",
				"→", "↔", "←",
				"↘", "↓", "↙",
			},
			headerLeft:  "→",
			headerMid:   "↔",
			headerRight: "←",
		}
	case StyleStarry:
		return &Glyphs{
			name:   StyleNameStarry,
			row:    "★",
			column: "☆",
			center: "✶",
			corners: [9]string{
				"✧", "✯", "✧",
				"✦", "✶", "✦",
				"✧", "✯", "✧",
			},
			headerLeft:  "✦",
			headerMid:   "✶",
			headerRight: "✦",
		}
	case StyleHearts:
		return &Glyphs{
			name:   StyleNameHearts,
			row:    "♥",
			column: "❤",
			center: "✚",
			corners: [9]string{
				"❥", "♡", "❥",
				"❣", "✚", "❣",
				"❦", "♡", "❦",
			},
			headerLeft:  "❣",
			headerMid:   "✚",
			headerRight: "❣",
		}
	case StyleCircuit:
		return &Glyphs{
			name:   StyleNameCircuit,
			row:    "=",
			column: "||",
			center: "<>",
			corners: [9]string{
				"/*", "##", "*/",
				"//", "<>", "\\",
				"\\*", "##", "*/",
			},
			headerLeft:  "//",
			headerMid:   "<>",
			headerRight: "\\",
		}
	case StyleNature:
		return &Glyphs{
			name:   StyleNameNature,
			row:    "~",
			column: "|",
			center: "❀",
			corners: [9]string{
				"🌱", "🌿", "🌱",
				"🍃", "❀", "🍃",
				"🌻", "🌾", "🌻",
			},
			headerLeft:  "🍃",
			headerMid:   "❀",
			headerRight: "🍃",
		}
	case StyleArtistic:
		return &Glyphs{
			name:   StyleNameArtistic,
			row:    "▬",
			column: "▐",
			center: "⬔",
			corners: [9]string{
				"◈", "◊", "◈",
				"◀", "⬔", "▶",
				"◭", "▣", "◮",
			},
			headerLeft:  "◀",
			headerMid:   "⬔",
			headerRight: "▶",
		}
	case Style8Bit:
		return &Glyphs{
			name:   StyleName8Bit,
			row:    "■",
			column: "█",
			center: "♦",
			corners: [9]string{
				"╔", "▲", "╗",
				"◄", "♦", "►",
				"╚", "▼", "╝",
			},
			headerLeft:  "◄",
			headerMid:   "♦",
			headerRight: "►",
		}
	case StyleChaos:
		return &Glyphs{
			name:   StyleNameChaos,
			row:    "≈",
			column: "§",
			center: "☯",
			corners: [9]string{
				"⌘", "∞", "⌥",
				"⚡", "☯", "♞",
				"⌂", "∆", "◊",
			},
			headerLeft:  "⚡",
			headerMid:   "☯",
			headerRight: "♞",
		}
	case StyleDots:
		return &Glyphs{
			name:   StyleNameDots,
			row:    "·",
			column: " ",
			center: "·",
			corners: [9]string{
				"·", "·", "·",
				" ", "·", " ",
				"·", "·", "·",
			},
			headerLeft:  " ",
			headerMid:   "·",
			headerRight: " ",
		}
	case StyleBlocks:
		return &Glyphs{
			name:   StyleNameBlocks,
			row:    "▀",
			column: "█",
			center: "█",
			corners: [9]string{
				"▛", "▀", "▜",
				"▌", "█", "▐",
				"▙", "▄", "▟",
			},
			headerLeft:  "▌",
			headerMid:   "█",
			headerRight: "▐",
		}
	case StyleZen:
		return &Glyphs{
			name:   StyleNameZen,
			row:    "~",
			column: " ",
			center: "☯",
			corners: [9]string{
				" ", "♨", " ",
				" ", "☯", " ",
				" ", "♨", " ",
			},
			headerLeft:  " ",
			headerMid:   "☯",
			headerRight: " ",
		}
	case StyleVintage:
		return &Glyphs{
			name:   StyleNameVintage,
			row:    "────",
			column: " ⁜ ",
			center: " ✠ ",
			corners: [9]string{
				"╔══", "══╤", "══╗",
				" ⁜ ", " ✠ ", " ⁜ ",
				"╚══", "══╧", "══╝",
			},
			headerLeft:  " ├─",
			headerMid:   "─✠─",
			headerRight: "─┤ ",
		}
	case StyleSketch:
		return &Glyphs{
			name:   StyleNameSketch,
			row:    "~~",
			column: "/",
			center: "+",
			corners: [9]string{
				" .", "~~", ". ",
				"/ ", "+ ", " \\",
				" '", "~~", "` ",
			},
			headerLeft:  "/~",
			headerMid:   "+~",
			headerRight: "~\\",
		}
	case StyleArrowDouble:
		return &Glyphs{
			name:   StyleNameArrowDouble,
			row:    "»»",
			column: "⫸",
			center: "✿",
			corners: [9]string{
				"⌜»", "»»", "»⌝",
				"⫸ ", "✿ ", " ⫷",
				"⌞»", "»»", "»⌟",
			},
			headerLeft:  "⫸»",
			headerMid:   "✿»",
			headerRight: "»⫷",
		}
	case StyleCelestial:
		return &Glyphs{
			name:   StyleNameCelestial,
			row:    "✦✧",
			column: "☽",
			center: "☀",
			corners: [9]string{
				"✧✦", "✦✧", "✦✧",
				"☽ ", "☀ ", " ☾",
				"✧✦", "✦✧", "✦✧",
			},
			headerLeft:  "☽✦",
			headerMid:   "☀✧",
			headerRight: "✦☾",
		}
	case StyleCyber:
		return &Glyphs{
			name:   StyleNameCyber,
			row:    "═╦═",
			column: "║",
			center: "╬",
			corners: [9]string{
				"╔╦═", "╦═╦", "═╦╗",
				"║ ", "╬ ", " ║",
				"╚╩═", "╩═╩", "═╩╝",
			},
			headerLeft:  "╠╦═",
			headerMid:   "╬═╦",
			headerRight: "═╦╣",
		}
	case StyleRunic:
		return &Glyphs{
			name:   StyleNameRunic,
			row:    "ᛖᛖᛖ",
			column: "ᛟ",
			center: "ᛞ",
			corners: [9]string{
				"ᛏᛖᛖ", "ᛖᛖᛖ", "ᛖᛖᛏ",
				"ᛟ ", "ᛞ ", " ᛟ",
				"ᛗᛖᛖ", "ᛖᛖᛖ", "ᛖᛖᛗ",
			},
			headerLeft:  "ᛟᛖᛖ",
			headerMid:   "ᛞᛖᛖ",
			headerRight: "ᛖᛖᛟ",
		}
	case StyleIndustrial:
		return &Glyphs{
			name:   StyleNameIndustrial,
			row:    "━╋━",
			column: "┃",
			center: "╋",
			corners: [9]string{
				"┏╋━", "╋━╋", "━╋┓",
				"┃ ", "╋ ", " ┃",
				"┗╋━", "╋━╋", "━╋┛",
			},
			headerLeft:  "┣╋━",
			headerMid:   "╋━╋",
			headerRight: "━╋┫",
		}
	case StyleInk:
		return &Glyphs{
			name:   StyleNameInk,
			row:    "﹌",
			column: "︱",
			center: "✒",
			corners: [9]string{
				"﹏", "﹌", "﹏",
				"︱ ", "✒ ", " ︱",
				"﹋", "﹌", "﹋",
			},
			headerLeft:  "︱﹌",
			headerMid:   "✒﹌",
			headerRight: "﹌︱",
		}
	case StyleArcade:
		return &Glyphs{
			name:   StyleNameArcade,
			row:    "■□",
			column: "▐",
			center: "◉",
			corners: [9]string{
				"▞■", "■□", "□▚",
				"▐ ", "◉ ", " ▐",
				"▚■", "■□", "□▞",
			},
			headerLeft:  "▐■",
			headerMid:   "◉□",
			headerRight: "■▐",
		}
	case StyleBlossom:
		return &Glyphs{
			name:   StyleNameBlossom,
			row:    "🌸",
			column: "🌿",
			center: "✿",
			corners: [9]string{
				"🌷", "🌸", "🌷",
				"🌿", "✿", "🌿",
				"🌱", "🌸", "🌱",
			},
			headerLeft:  "🌿🌸",
			headerMid:   "✿🌸",
			headerRight: "🌸🌿",
		}
	case StyleFrosted:
		return &Glyphs{
			name:   StyleNameFrosted,
			row:    "░▒░",
			column: "▓",
			center: "◍",
			corners: [9]string{
				"◌░▒", "░▒░", "▒░◌",
				"▓ ", "◍ ", " ▓",
				"◌░▒", "░▒░", "▒░◌",
			},
			headerLeft:  "▓░▒",
			headerMid:   "◍▒░",
			headerRight: "░▒▓",
		}
	case StyleMosaic:
		return &Glyphs{
			name:   StyleNameMosaic,
			row:    "▰▱",
			column: "⧉",
			center: "⬖",
			corners: [9]string{
				"⧠▰", "▰▱", "▱⧠",
				"⧉ ", "⬖ ", " ⧉",
				"⧅▰", "▰▱", "▱⧅",
			},
			headerLeft:  "⧉▰",
			headerMid:   "⬖▱",
			headerRight: "▰⧉",
		}
	case StyleUFO:
		return &Glyphs{
			name:   StyleNameUFO,
			row:    "⊚⊚",
			column: "☽",
			center: "☢",
			corners: [9]string{
				"⌖⊚", "⊚⊚", "⊚⌖",
				"☽ ", "☢ ", " ☽",
				"⌗⊚", "⊚⊚", "⊚⌗",
			},
			headerLeft:  "☽⊚",
			headerMid:   "☢⊚",
			headerRight: "⊚☽",
		}
	case StyleSteampunk:
		return &Glyphs{
			name:   StyleNameSteampunk,
			row:    "═⚙═",
			column: "⛓️",
			center: "⚔️",
			corners: [9]string{
				"🜂⚙═", "═⚙═", "═⚙🜂",
				"⛓️ ", "⚔️ ", " ⛓️",
				"🜄⚙═", "═⚙═", "═⚙🜄",
			},
			headerLeft:  "⛓️⚙═",
			headerMid:   "⚔️═⚙",
			headerRight: "═⚙⛓️",
		}
	case StyleGalaxy:
		return &Glyphs{
			name:   StyleNameGalaxy,
			row:    "≋≋",
			column: "♆",
			center: "☄️",
			corners: [9]string{
				"⌇≋", "≋≋", "≋⌇",
				"♆ ", "☄️ ", " ♆",
				"⌇≋", "≋≋", "≋⌇",
			},
			headerLeft:  "♆≋",
			headerMid:   "☄️≋",
			headerRight: "≋♆",
		}
	case StyleJazz:
		return &Glyphs{
			name:   StyleNameJazz,
			row:    "♬♬",
			column: "▷",
			center: "★",
			corners: [9]string{
				"♔♬", "♬♬", "♬♔",
				"▷ ", "★ ", " ◁",
				"♕♬", "♬♬", "♬♕",
			},
			headerLeft:  "▷♬",
			headerMid:   "★♬",
			headerRight: "♬◁",
		}
	case StylePuzzle:
		return &Glyphs{
			name:   StyleNamePuzzle,
			row:    "▣▣",
			column: "◫",
			center: "✚",
			corners: [9]string{
				"◩▣", "▣▣", "▣◪",
				"◫ ", "✚ ", " ◫",
				"◧▣", "▣▣", "▣◨",
			},
			headerLeft:  "◫▣",
			headerMid:   "✚▣",
			headerRight: "▣◫",
		}
	case StyleHypno:
		return &Glyphs{
			name:   StyleNameHypno,
			row:    "◜◝",
			column: "꩜",
			center: "⃰",
			corners: [9]string{
				"◟◜", "◜◝", "◝◞",
				"꩜ ", "⃰ ", " ꩜",
				"◟◜", "◜◝", "◝◞",
			},
			headerLeft:  "꩜◜",
			headerMid:   "⃰◝",
			headerRight: "◜꩜",
		}
	default:
		return &Glyphs{
			name:   StyleNameNothing,
			row:    "",
			column: "",
			center: "",
			corners: [9]string{
				"", "", "",
				"", "", "",
				"", "", "",
			},
			headerLeft:  "",
			headerMid:   "",
			headerRight: "",
		}
	}
}

// SymbolCustom implements the Symbols interface with fully configurable symbols
type SymbolCustom struct {
	name        string
	center      string
	row         string
	column      string
	topLeft     string
	topMid      string
	topRight    string
	midLeft     string
	midRight    string
	bottomLeft  string
	bottomMid   string
	bottomRight string
	headerLeft  string
	headerMid   string
	headerRight string
}

// NewSymbolCustom creates a new customizable border style
func NewSymbolCustom(name string) *SymbolCustom {
	return &SymbolCustom{
		name:   name,
		center: "+",
		row:    "-",
		column: "|",
	}
}

// Implement all Symbols interface methods
func (c *SymbolCustom) Name() string        { return c.name }
func (c *SymbolCustom) Center() string      { return c.center }
func (c *SymbolCustom) Row() string         { return c.row }
func (c *SymbolCustom) Column() string      { return c.column }
func (c *SymbolCustom) TopLeft() string     { return c.topLeft }
func (c *SymbolCustom) TopMid() string      { return c.topMid }
func (c *SymbolCustom) TopRight() string    { return c.topRight }
func (c *SymbolCustom) MidLeft() string     { return c.midLeft }
func (c *SymbolCustom) MidRight() string    { return c.midRight }
func (c *SymbolCustom) BottomLeft() string  { return c.bottomLeft }
func (c *SymbolCustom) BottomMid() string   { return c.bottomMid }
func (c *SymbolCustom) BottomRight() string { return c.bottomRight }
func (c *SymbolCustom) HeaderLeft() string  { return c.headerLeft }
func (c *SymbolCustom) HeaderMid() string   { return c.headerMid }
func (c *SymbolCustom) HeaderRight() string { return c.headerRight }

// Builder methods for fluent configuration
func (c *SymbolCustom) WithCenter(s string) *SymbolCustom      { c.center = s; return c }
func (c *SymbolCustom) WithRow(s string) *SymbolCustom         { c.row = s; return c }
func (c *SymbolCustom) WithColumn(s string) *SymbolCustom      { c.column = s; return c }
func (c *SymbolCustom) WithTopLeft(s string) *SymbolCustom     { c.topLeft = s; return c }
func (c *SymbolCustom) WithTopMid(s string) *SymbolCustom      { c.topMid = s; return c }
func (c *SymbolCustom) WithTopRight(s string) *SymbolCustom    { c.topRight = s; return c }
func (c *SymbolCustom) WithMidLeft(s string) *SymbolCustom     { c.midLeft = s; return c }
func (c *SymbolCustom) WithMidRight(s string) *SymbolCustom    { c.midRight = s; return c }
func (c *SymbolCustom) WithBottomLeft(s string) *SymbolCustom  { c.bottomLeft = s; return c }
func (c *SymbolCustom) WithBottomMid(s string) *SymbolCustom   { c.bottomMid = s; return c }
func (c *SymbolCustom) WithBottomRight(s string) *SymbolCustom { c.bottomRight = s; return c }
func (c *SymbolCustom) WithHeaderLeft(s string) *SymbolCustom  { c.headerLeft = s; return c }
func (c *SymbolCustom) WithHeaderMid(s string) *SymbolCustom   { c.headerMid = s; return c }
func (c *SymbolCustom) WithHeaderRight(s string) *SymbolCustom { c.headerRight = s; return c }

// Preview renders a small sample table to visualize the border style
func (s *SymbolCustom) Preview() string {
	return fmt.Sprintf(
		"%s%s%s\n%s %s %s\n%s%s%s",
		s.TopLeft(), s.Row(), s.TopRight(),
		s.Column(), s.Center(), s.Column(),
		s.BottomLeft(), s.Row(), s.BottomRight(),
	)
}

// Glyphs provides fully independent border symbols with a corners array
type Glyphs struct {
	name        StyleName
	row         string
	column      string
	center      string
	corners     [9]string // [TopLeft, TopMid, TopRight, MidLeft, Center, MidRight, BottomLeft, BottomMid, BottomRight]
	headerLeft  string
	headerMid   string
	headerRight string
}

// Glyphs symbol methods
func (s *Glyphs) Name() string        { return s.name.String() }
func (s *Glyphs) Center() string      { return s.center }
func (s *Glyphs) Row() string         { return s.row }
func (s *Glyphs) Column() string      { return s.column }
func (s *Glyphs) TopLeft() string     { return s.corners[0] }
func (s *Glyphs) TopMid() string      { return s.corners[1] }
func (s *Glyphs) TopRight() string    { return s.corners[2] }
func (s *Glyphs) MidLeft() string     { return s.corners[3] }
func (s *Glyphs) MidRight() string    { return s.corners[5] }
func (s *Glyphs) BottomLeft() string  { return s.corners[6] }
func (s *Glyphs) BottomMid() string   { return s.corners[7] }
func (s *Glyphs) BottomRight() string { return s.corners[8] }
func (s *Glyphs) HeaderLeft() string  { return s.headerLeft }
func (s *Glyphs) HeaderMid() string   { return s.headerMid }
func (s *Glyphs) HeaderRight() string { return s.headerRight }

// Preview renders a small sample table to visualize the border style
func (s *Glyphs) Preview() string {
	return fmt.Sprintf(
		"%s%s%s\n%s %s %s\n%s%s%s",
		s.TopLeft(), s.Row(), s.TopRight(),
		s.Column(), s.Center(), s.Column(),
		s.BottomLeft(), s.Row(), s.BottomRight(),
	)
}
