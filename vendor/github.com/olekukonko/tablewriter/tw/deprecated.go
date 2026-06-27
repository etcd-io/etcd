package tw

// Deprecated: SymbolASCII is deprecated; use Glyphs with StyleASCII instead.
// this will be removed soon
type SymbolASCII struct{}

// SymbolASCII symbol methods
func (s *SymbolASCII) Name() string        { return StyleNameASCII.String() }
func (s *SymbolASCII) Center() string      { return "+" }
func (s *SymbolASCII) Row() string         { return "-" }
func (s *SymbolASCII) Column() string      { return "|" }
func (s *SymbolASCII) TopLeft() string     { return "+" }
func (s *SymbolASCII) TopMid() string      { return "+" }
func (s *SymbolASCII) TopRight() string    { return "+" }
func (s *SymbolASCII) MidLeft() string     { return "+" }
func (s *SymbolASCII) MidRight() string    { return "+" }
func (s *SymbolASCII) BottomLeft() string  { return "+" }
func (s *SymbolASCII) BottomMid() string   { return "+" }
func (s *SymbolASCII) BottomRight() string { return "+" }
func (s *SymbolASCII) HeaderLeft() string  { return "+" }
func (s *SymbolASCII) HeaderMid() string   { return "+" }
func (s *SymbolASCII) HeaderRight() string { return "+" }

// Deprecated: SymbolUnicode is deprecated; use Glyphs with appropriate styles (e.g., StyleLight, StyleHeavy) instead.
// this will be removed soon
type SymbolUnicode struct {
	row     string
	column  string
	center  string
	corners [9]string // [topLeft, topMid, topRight, midLeft, center, midRight, bottomLeft, bottomMid, bottomRight]
}

// SymbolUnicode symbol methods
func (s *SymbolUnicode) Name() string        { return "unicode" }
func (s *SymbolUnicode) Center() string      { return s.center }
func (s *SymbolUnicode) Row() string         { return s.row }
func (s *SymbolUnicode) Column() string      { return s.column }
func (s *SymbolUnicode) TopLeft() string     { return s.corners[0] }
func (s *SymbolUnicode) TopMid() string      { return s.corners[1] }
func (s *SymbolUnicode) TopRight() string    { return s.corners[2] }
func (s *SymbolUnicode) MidLeft() string     { return s.corners[3] }
func (s *SymbolUnicode) MidRight() string    { return s.corners[5] }
func (s *SymbolUnicode) BottomLeft() string  { return s.corners[6] }
func (s *SymbolUnicode) BottomMid() string   { return s.corners[7] }
func (s *SymbolUnicode) BottomRight() string { return s.corners[8] }
func (s *SymbolUnicode) HeaderLeft() string  { return s.MidLeft() }
func (s *SymbolUnicode) HeaderMid() string   { return s.Center() }
func (s *SymbolUnicode) HeaderRight() string { return s.MidRight() }

// Deprecated: SymbolMarkdown is deprecated; use Glyphs with StyleMarkdown instead.
// this will be removed soon
type SymbolMarkdown struct{}

// SymbolMarkdown symbol methods
func (s *SymbolMarkdown) Name() string        { return StyleNameMarkdown.String() }
func (s *SymbolMarkdown) Center() string      { return "|" }
func (s *SymbolMarkdown) Row() string         { return "-" }
func (s *SymbolMarkdown) Column() string      { return "|" }
func (s *SymbolMarkdown) TopLeft() string     { return "" }
func (s *SymbolMarkdown) TopMid() string      { return "" }
func (s *SymbolMarkdown) TopRight() string    { return "" }
func (s *SymbolMarkdown) MidLeft() string     { return "|" }
func (s *SymbolMarkdown) MidRight() string    { return "|" }
func (s *SymbolMarkdown) BottomLeft() string  { return "" }
func (s *SymbolMarkdown) BottomMid() string   { return "" }
func (s *SymbolMarkdown) BottomRight() string { return "" }
func (s *SymbolMarkdown) HeaderLeft() string  { return "|" }
func (s *SymbolMarkdown) HeaderMid() string   { return "|" }
func (s *SymbolMarkdown) HeaderRight() string { return "|" }

// Deprecated: SymbolNothing is deprecated; use Glyphs with StyleNone instead.
// this will be removed soon
type SymbolNothing struct{}

// SymbolNothing symbol methods
func (s *SymbolNothing) Name() string        { return StyleNameNothing.String() }
func (s *SymbolNothing) Center() string      { return "" }
func (s *SymbolNothing) Row() string         { return "" }
func (s *SymbolNothing) Column() string      { return "" }
func (s *SymbolNothing) TopLeft() string     { return "" }
func (s *SymbolNothing) TopMid() string      { return "" }
func (s *SymbolNothing) TopRight() string    { return "" }
func (s *SymbolNothing) MidLeft() string     { return "" }
func (s *SymbolNothing) MidRight() string    { return "" }
func (s *SymbolNothing) BottomLeft() string  { return "" }
func (s *SymbolNothing) BottomMid() string   { return "" }
func (s *SymbolNothing) BottomRight() string { return "" }
func (s *SymbolNothing) HeaderLeft() string  { return "" }
func (s *SymbolNothing) HeaderMid() string   { return "" }
func (s *SymbolNothing) HeaderRight() string { return "" }

// Deprecated: SymbolGraphical is deprecated; use Glyphs with StyleGraphical instead.
// this will be removed soon
type SymbolGraphical struct{}

// SymbolGraphical symbol methods
func (s *SymbolGraphical) Name() string        { return StyleNameGraphical.String() }
func (s *SymbolGraphical) Center() string      { return "🟧" } // Orange square (matches mid junctions)
func (s *SymbolGraphical) Row() string         { return "🟥" } // Red square (matches corners)
func (s *SymbolGraphical) Column() string      { return "🟦" } // Blue square (vertical line)
func (s *SymbolGraphical) TopLeft() string     { return "🟥" } // Top-left corner
func (s *SymbolGraphical) TopMid() string      { return "🔳" } // Top junction
func (s *SymbolGraphical) TopRight() string    { return "🟥" } // Top-right corner
func (s *SymbolGraphical) MidLeft() string     { return "🟧" } // Left junction
func (s *SymbolGraphical) MidRight() string    { return "🟧" } // Right junction
func (s *SymbolGraphical) BottomLeft() string  { return "🟥" } // Bottom-left corner
func (s *SymbolGraphical) BottomMid() string   { return "🔳" } // Bottom junction
func (s *SymbolGraphical) BottomRight() string { return "🟥" } // Bottom-right corner
func (s *SymbolGraphical) HeaderLeft() string  { return "🟧" } // Header left (matches mid junctions)
func (s *SymbolGraphical) HeaderMid() string   { return "🟧" } // Header middle (matches mid junctions)
func (s *SymbolGraphical) HeaderRight() string { return "🟧" } // Header right (matches mid junctions)

// Deprecated: SymbolMerger is deprecated; use Glyphs with StyleMerger instead.
// this will be removed soon
type SymbolMerger struct {
	row     string
	column  string
	center  string
	corners [9]string // [TL, TM, TR, ML, CenterIdx(unused), MR, BL, BM, BR]
}

// SymbolMerger symbol methods
func (s *SymbolMerger) Name() string        { return StyleNameMerger.String() }
func (s *SymbolMerger) Center() string      { return s.center } // Main crossing symbol
func (s *SymbolMerger) Row() string         { return s.row }
func (s *SymbolMerger) Column() string      { return s.column }
func (s *SymbolMerger) TopLeft() string     { return s.corners[0] }
func (s *SymbolMerger) TopMid() string      { return s.corners[1] } // LevelHeader junction
func (s *SymbolMerger) TopRight() string    { return s.corners[2] }
func (s *SymbolMerger) MidLeft() string     { return s.corners[3] } // Left junction
func (s *SymbolMerger) MidRight() string    { return s.corners[5] } // Right junction
func (s *SymbolMerger) BottomLeft() string  { return s.corners[6] }
func (s *SymbolMerger) BottomMid() string   { return s.corners[7] } // LevelFooter junction
func (s *SymbolMerger) BottomRight() string { return s.corners[8] }
func (s *SymbolMerger) HeaderLeft() string  { return s.MidLeft() }
func (s *SymbolMerger) HeaderMid() string   { return s.Center() }
func (s *SymbolMerger) HeaderRight() string { return s.MidRight() }
