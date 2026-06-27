package tw

import (
	"io"

	"github.com/olekukonko/ll"
)

// Renderer defines the interface for rendering tables to an io.Writer.
// Implementations must handle headers, rows, footers, and separator lines.
type Renderer interface {
	Start(w io.Writer) error
	Header(headers [][]string, ctx Formatting) // Renders table header
	Row(row []string, ctx Formatting)          // Renders a single row
	Footer(footers [][]string, ctx Formatting) // Renders table footer
	Line(ctx Formatting)                       // Renders separator line
	Config() Rendition                         // Returns renderer config
	Close() error                              // Gets Rendition form Blueprint
	Logger(logger *ll.Logger)                  // send logger to renderers
}

// Rendition holds the configuration for the default renderer.
type Rendition struct {
	Borders   Border   // Border visibility settings
	Symbols   Symbols  // Symbols used for table drawing
	Settings  Settings // Rendering behavior settings
	Streaming bool
}

// Renditioning  has a method to update its rendition.
// Let's define an optional interface for this.
type Renditioning interface {
	Rendition(r Rendition)
}

// Formatting encapsulates the complete formatting context for a table row.
// It provides all necessary information to render a row correctly within the table structure.
type Formatting struct {
	Row              RowContext // Detailed configuration for the row and its cells
	Level            Level      // Hierarchical level (Header, Body, Footer) affecting line drawing
	HasFooter        bool       // Indicates if the table includes a footer section
	IsSubRow         bool       // Marks this as a continuation or padding line in multi-line rows
	NormalizedWidths Mapper[int, int]
}

// CellContext defines the properties and formatting state of an individual table cell.
type CellContext struct {
	Data    string     // Content to be displayed in the cell, provided by the caller
	Align   Align      // Text alignment within the cell (Left, Right, Center, Skip)
	Padding Padding    // Padding characters surrounding the cell content
	Width   int        // Suggested width (often overridden by Row.Widths)
	Merge   MergeState // Details about cell spanning across rows or columns
}

// MergeState captures how a cell merges across different directions.
type MergeState struct {
	Vertical     MergeStateOption // Properties for vertical merging (across rows)
	Horizontal   MergeStateOption // Properties for horizontal merging (across columns)
	Hierarchical MergeStateOption // Properties for nested/hierarchical merging
}

// MergeStateOption represents common attributes for merging in a specific direction.
type MergeStateOption struct {
	Present bool // True if this merge direction is active
	Span    int  // Number of cells this merge spans
	Start   bool // True if this cell is the starting point of the merge
	End     bool // True if this cell is the ending point of the merge
}

// RowContext manages layout properties and relationships for a row and its columns.
// It maintains state about the current row and its neighbors for proper rendering.
type RowContext struct {
	Position     Position            // Section of the table (Header, Row, Footer)
	Location     Location            // Boundary position (First, Middle, End)
	Current      map[int]CellContext // Cells in this row, indexed by column
	Previous     map[int]CellContext // Cells from the row above; nil if none
	Next         map[int]CellContext // Cells from the row below; nil if none
	Widths       Mapper[int, int]    // Computed widths for each column
	ColMaxWidths CellWidth           // Maximum allowed width per column
}

func (r RowContext) GetCell(col int) CellContext {
	return r.Current[col]
}

// Separators controls the visibility of separators in the table.
type Separators struct {
	ShowHeader     State // Controls header separator visibility
	ShowFooter     State // Controls footer separator visibility
	BetweenRows    State // Determines if lines appear between rows
	BetweenColumns State // Determines if separators appear between columns
}

// Lines manages the visibility of table boundary lines.
type Lines struct {
	ShowTop        State // Top border visibility
	ShowBottom     State // Bottom border visibility
	ShowHeaderLine State // Header separator line visibility
	ShowFooterLine State // Footer separator line visibility
}

// Settings holds configuration preferences for rendering behavior.
type Settings struct {
	Separators  Separators // Separator visibility settings
	Lines       Lines      // Line visibility settings
	CompactMode State      // Reserved for future compact rendering (unused)
	// Cushion     State
}

// Border defines the visibility states of table borders.
type Border struct {
	Left      State // Left border visibility
	Right     State // Right border visibility
	Top       State // Top border visibility
	Bottom    State // Bottom border visibility
	Overwrite bool
}

type StreamConfig struct {
	Enable bool

	// StrictColumns, if true, causes Append() to return an error
	// in streaming mode if the number of cells in an appended row
	// does not match the established number of columns for the stream.
	// If false (default), rows with mismatched column counts will be
	// padded or truncated with a warning log.
	StrictColumns bool

	// Deprecated: Use top-level Config.Widths for streaming width control.
	// This field will be removed in a future version. It will be respected if
	// Config.Widths is not set and this field is.
	Widths CellWidth
}
