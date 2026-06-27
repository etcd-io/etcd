package tw

// CellFormatting holds formatting options for table cells.
type CellFormatting struct {
	AutoWrap   int   // Wrapping behavior (e.g., WrapTruncate, WrapNormal)
	AutoFormat State // Enables automatic formatting (e.g., title case for headers)

	// Deprecated: Kept for backward compatibility. Use CellConfig.CellMerging.Mode instead.
	// This will be removed in a future version.
	MergeMode int

	// Deprecated: Kept for backward compatibility. Use CellConfig.Alignment instead.
	// This will be removed in a future version.
	Alignment Align
}

// CellMerging holds the configuration for how cells should be merged.
// This new struct replaces the deprecated MergeMode.
type CellMerging struct {
	// Mode is a bitmask specifying the type of merge (e.g., MergeHorizontal, MergeVertical).
	Mode int

	// ByColumnIndex specifies which column indices should be considered for merging.
	// If the mapper is nil or empty, merging applies to all columns (if Mode is set).
	// Otherwise, only columns with an index present as a key will be merged.
	ByColumnIndex Mapper[int, bool]

	// ByRowIndex is reserved for future features to specify merging on specific rows.
	ByRowIndex Mapper[int, bool]
}

// CellPadding defines padding settings for table cells.
type CellPadding struct {
	Global    Padding   // Default padding applied to all cells
	PerColumn []Padding // Column-specific padding overrides
}

// CellFilter defines filtering functions for cell content.
type CellFilter struct {
	Global    func([]string) []string // Processes the entire row
	PerColumn []func(string) string   // Processes individual cells by column
}

// CellCallbacks holds callback functions for cell processing.
// Note: These are currently placeholders and not fully implemented.
type CellCallbacks struct {
	Global    func()   // Global callback applied to all cells
	PerColumn []func() // Column-specific callbacks
}

// CellAlignment defines alignment settings for table cells.
type CellAlignment struct {
	Global    Align   // Default alignment applied to all cells
	PerColumn []Align // Column-specific alignment overrides
}

// CellConfig combines formatting, padding, and callback settings for a table section.
type CellConfig struct {
	Formatting   CellFormatting // Cell formatting options
	Padding      CellPadding    // Padding configuration
	Callbacks    CellCallbacks  // Callback functions (unused)
	Filter       CellFilter     // Function to filter cell content (renamed from Filter Filter)
	Alignment    CellAlignment  // Alignment configuration for cells
	ColMaxWidths CellWidth      // Per-column maximum width overrides
	Merging      CellMerging    // Merging holds all configuration related to cell merging.

	// Deprecated: use Alignment.PerColumn instead. Will be removed in a future version.
	// will be removed soon
	ColumnAligns []Align // Per-column alignment overrides
}

type CellWidth struct {
	Global    int
	PerColumn Mapper[int, int]
}

func (c CellWidth) Constrained() bool {
	return c.Global > 0 || c.PerColumn.Len() > 0
}
