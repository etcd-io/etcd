package tw

// Operation Status Constants
// Used to indicate the success or failure of operations
const (
	Pending = 0  // Operation failed
	Fail    = -1 // Operation failed
	Success = 1  // Operation succeeded

	MinimumColumnWidth = 8

	DefaultCacheStringCapacity = 10 * 1024 // 10 KB
)

const (
	Empty   = ""
	Skip    = ""
	Space   = " "
	NewLine = "\n"
	Column  = ":"
	Dash    = "-"
)

// Feature State Constants
// Represents enabled/disabled states for features
const (
	Unknown State = Pending // Feature is enabled
	On      State = Success // Feature is enabled
	Off     State = Fail    // Feature is disabled
)

// Table Alignment Constants
// Defines text alignment options for table content
const (
	AlignNone    Align = "none"    // Center-aligned text
	AlignCenter  Align = "center"  // Center-aligned text
	AlignRight   Align = "right"   // Right-aligned text
	AlignLeft    Align = "left"    // Left-aligned text
	AlignDefault       = AlignLeft // Left-aligned text
)

const (
	Header Position = "header" // Table header section
	Row    Position = "row"    // Table row section
	Footer Position = "footer" // Table footer section
)

const (
	LevelHeader Level = iota // Topmost line position
	LevelBody                // LevelBody line position
	LevelFooter              // LevelFooter line position
)

const (
	LocationFirst  Location = "first"  // Topmost line position
	LocationMiddle Location = "middle" // LevelBody line position
	LocationEnd    Location = "end"    // LevelFooter line position
)

const (
	SectionHeader = "header"
	SectionRow    = "row"
	SectionFooter = "footer"
)

// Text Wrapping Constants
// Defines text wrapping behavior in table cells
const (
	WrapNone     = iota // No wrapping
	WrapNormal          // Standard word wrapping
	WrapTruncate        // Truncate text with ellipsis
	WrapBreak           // Break words to fit
)

// Cell Merge Constants
// Specifies cell merging behavior in tables

const (
	MergeNone         = iota // No merging
	MergeVertical            // Merge cells vertically
	MergeHorizontal          // Merge cells horizontally
	MergeBoth                // Merge both vertically and horizontally
	MergeHierarchical        // Hierarchical merging
)

// Special Character Constants
// Defines special characters used in formatting
const (
	CharEllipsis = "…" // Ellipsis character for truncation
	CharBreak    = "↩" // Break character for wrapping
)

type Spot int

const (
	SpotNone Spot = iota
	SpotTopLeft
	SpotTopCenter
	SpotTopRight
	SpotBottomLeft
	SpotBottomCenter // Default for legacy SetCaption
	SpotBottomRight
	SpotLeftTop
	SpotLeftCenter
	SpotLeftBottom
	SpotRightTop
	SpotRightCenter
	SpotRIghtBottom
)
