package tw

// BorderNone defines a border configuration with all sides disabled.
var (
	// PaddingNone represents explicitly empty padding (no spacing on any side)
	// Equivalent to Padding{Overwrite: true}
	PaddingNone    = Padding{Left: Empty, Right: Empty, Top: Empty, Bottom: Empty, Overwrite: true}
	BorderNone     = Border{Left: Off, Right: Off, Top: Off, Bottom: Off}
	LinesNone      = Lines{ShowTop: Off, ShowBottom: Off, ShowHeaderLine: Off, ShowFooterLine: Off}
	SeparatorsNone = Separators{ShowHeader: Off, ShowFooter: Off, BetweenRows: Off, BetweenColumns: Off}
)

// PaddingDefault represents standard single-space padding on left/right
// Equivalent to Padding{Left: " ", Right: " ", Overwrite: true}
var PaddingDefault = Padding{Left: " ", Right: " ", Overwrite: true}
