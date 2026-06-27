package renderer

import (
	"io"
	"strings"

	"github.com/olekukonko/ll"
	"github.com/olekukonko/ll/lh"
	"github.com/olekukonko/tablewriter/pkg/twwidth"

	"github.com/olekukonko/tablewriter/tw"
)

// ColorizedConfig holds configuration for the Colorized table renderer.
type ColorizedConfig struct {
	Borders   tw.Border   // Border visibility settings
	Settings  tw.Settings // Rendering behavior settings (e.g., separators, whitespace)
	Header    Tint        // Colors for header cells
	Column    Tint        // Colors for row cells
	Footer    Tint        // Colors for footer cells
	Border    Tint        // Colors for borders and lines
	Separator Tint        // Colors for column separators
	Symbols   tw.Symbols  // Symbols for table drawing (e.g., corners, lines)
}

// Colorized renders colored ASCII tables with customizable borders, colors, and alignments.
type Colorized struct {
	config       ColorizedConfig          // Renderer configuration
	trace        []string                 // Debug trace messages
	newLine      string                   // Newline character
	defaultAlign map[tw.Position]tw.Align // Default alignments for header, row, and footer
	logger       *ll.Logger               // Logger for debug messages
	w            io.Writer
}

// NewColorized creates a Colorized renderer with the specified configuration, falling back to defaults if none provided.
// Only the first config is used if multiple are passed.
func NewColorized(configs ...ColorizedConfig) *Colorized {
	// Initialize with default configuration
	baseCfg := defaultColorized()

	if len(configs) > 0 {
		userCfg := configs[0]

		// Override border settings if provided
		if userCfg.Borders.Left != 0 {
			baseCfg.Borders.Left = userCfg.Borders.Left
		}
		if userCfg.Borders.Right != 0 {
			baseCfg.Borders.Right = userCfg.Borders.Right
		}
		if userCfg.Borders.Top != 0 {
			baseCfg.Borders.Top = userCfg.Borders.Top
		}
		if userCfg.Borders.Bottom != 0 {
			baseCfg.Borders.Bottom = userCfg.Borders.Bottom
		}

		// Merge separator and line settings
		baseCfg.Settings.Separators = mergeSeparators(baseCfg.Settings.Separators, userCfg.Settings.Separators)
		baseCfg.Settings.Lines = mergeLines(baseCfg.Settings.Lines, userCfg.Settings.Lines)

		// Override compact mode if specified
		if userCfg.Settings.CompactMode != 0 {
			baseCfg.Settings.CompactMode = userCfg.Settings.CompactMode
		}

		// Override color settings for various table elements
		if len(userCfg.Header.FG) > 0 || len(userCfg.Header.BG) > 0 || userCfg.Header.Columns != nil {
			baseCfg.Header = userCfg.Header
		}
		if len(userCfg.Column.FG) > 0 || len(userCfg.Column.BG) > 0 || userCfg.Column.Columns != nil {
			baseCfg.Column = userCfg.Column
		}
		if len(userCfg.Footer.FG) > 0 || len(userCfg.Footer.BG) > 0 || userCfg.Footer.Columns != nil {
			baseCfg.Footer = userCfg.Footer
		}
		if len(userCfg.Border.FG) > 0 || len(userCfg.Border.BG) > 0 || userCfg.Border.Columns != nil {
			baseCfg.Border = userCfg.Border
		}
		if len(userCfg.Separator.FG) > 0 || len(userCfg.Separator.BG) > 0 || userCfg.Separator.Columns != nil {
			baseCfg.Separator = userCfg.Separator
		}

		// Override symbols if provided
		if userCfg.Symbols != nil {
			baseCfg.Symbols = userCfg.Symbols
		}
	}

	cfg := baseCfg
	// Ensure symbols are initialized
	if cfg.Symbols == nil {
		cfg.Symbols = tw.NewSymbols(tw.StyleLight)
	}

	// Initialize the Colorized renderer
	f := &Colorized{
		config:  cfg,
		newLine: tw.NewLine,
		defaultAlign: map[tw.Position]tw.Align{
			tw.Header: tw.AlignCenter,
			tw.Row:    tw.AlignLeft,
			tw.Footer: tw.AlignRight,
		},
		logger: ll.New("colorized", ll.WithHandler(lh.NewMemoryHandler())).Disable(),
	}
	// Log initialization details
	f.logger.Debugf("Initialized Colorized renderer with symbols: Center=%q, Row=%q, Column=%q", f.config.Symbols.Center(), f.config.Symbols.Row(), f.config.Symbols.Column())
	f.logger.Debugf("Final ColorizedConfig.Settings.Lines: %+v", f.config.Settings.Lines)
	f.logger.Debugf("Final ColorizedConfig.Borders: %+v", f.config.Borders)
	return f
}

// Close performs cleanup (no-op in this implementation).
func (c *Colorized) Close() error {
	c.logger.Debug("Colorized.Close() called (no-op).")
	return nil
}

// Config returns the renderer's configuration as a Rendition.
func (c *Colorized) Config() tw.Rendition {
	return tw.Rendition{
		Borders:   c.config.Borders,
		Settings:  c.config.Settings,
		Symbols:   c.config.Symbols,
		Streaming: true,
	}
}

// Debug returns the accumulated debug trace messages.
func (c *Colorized) Debug() []string {
	return c.trace
}

// Footer renders the table footer with configured colors and formatting.
func (c *Colorized) Footer(footers [][]string, ctx tw.Formatting) {
	c.logger.Debugf("Starting Footer render: IsSubRow=%v, Location=%v, Pos=%s",
		ctx.IsSubRow, ctx.Row.Location, ctx.Row.Position)

	// Check if there are footers to render
	if len(footers) == 0 || len(footers[0]) == 0 {
		c.logger.Debug("Footer: No footers to render")
		return
	}

	// Render the footer line
	c.renderLine(ctx, footers[0], c.config.Footer)
	c.logger.Debug("Completed Footer render")
}

// Header renders the table header with configured colors and formatting.
func (c *Colorized) Header(headers [][]string, ctx tw.Formatting) {
	c.logger.Debugf("Starting Header render: IsSubRow=%v, Location=%v, Pos=%s, lines=%d, widths=%v",
		ctx.IsSubRow, ctx.Row.Location, ctx.Row.Position, len(headers), ctx.Row.Widths)

	// Check if there are headers to render
	if len(headers) == 0 || len(headers[0]) == 0 {
		c.logger.Debug("Header: No headers to render")
		return
	}

	// Render the header line
	c.renderLine(ctx, headers[0], c.config.Header)
	c.logger.Debug("Completed Header render")
}

// Line renders a horizontal row line with colored junctions and segments, skipping zero-width columns.
func (c *Colorized) Line(ctx tw.Formatting) {
	c.logger.Debugf("Line: Starting with Level=%v, Location=%v, IsSubRow=%v, Widths=%v", ctx.Level, ctx.Row.Location, ctx.IsSubRow, ctx.Row.Widths)

	// Initialize junction renderer
	jr := NewJunction(JunctionContext{
		Symbols:       c.config.Symbols,
		Ctx:           ctx,
		ColIdx:        0,
		BorderTint:    c.config.Border,
		SeparatorTint: c.config.Separator,
		Logger:        c.logger,
	})

	var line strings.Builder

	// Get sorted column indices and filter out zero-width columns
	allSortedKeys := ctx.Row.Widths.SortedKeys()
	effectiveKeys := []int{}
	keyWidthMap := make(map[int]int)

	for _, k := range allSortedKeys {
		width := ctx.Row.Widths.Get(k)
		keyWidthMap[k] = width
		if width > 0 {
			effectiveKeys = append(effectiveKeys, k)
		}
	}
	c.logger.Debugf("Line: All keys=%v, Effective keys (width>0)=%v", allSortedKeys, effectiveKeys)

	// Handle case with no effective columns
	if len(effectiveKeys) == 0 {
		prefix := tw.Empty
		suffix := tw.Empty
		if c.config.Borders.Left.Enabled() {
			prefix = jr.RenderLeft()
		}
		if c.config.Borders.Right.Enabled() {
			originalLastColIdx := -1
			if len(allSortedKeys) > 0 {
				originalLastColIdx = allSortedKeys[len(allSortedKeys)-1]
			}
			suffix = jr.RenderRight(originalLastColIdx)
		}
		if prefix != tw.Empty || suffix != tw.Empty {
			line.WriteString(prefix + suffix + tw.NewLine)
			c.w.Write([]byte(line.String()))
		}
		c.logger.Debug("Line: Handled empty row/widths case (no effective keys)")
		return
	}

	// Add left border if enabled
	if c.config.Borders.Left.Enabled() {
		line.WriteString(jr.RenderLeft())
	}

	// Render segments for each effective column
	for keyIndex, currentColIdx := range effectiveKeys {
		jr.colIdx = currentColIdx
		segment := jr.GetSegment()
		colWidth := keyWidthMap[currentColIdx]
		c.logger.Debugf("Line: Drawing segment for Effective colIdx=%d, segment='%s', width=%d", currentColIdx, segment, colWidth)

		if segment == tw.Empty {
			line.WriteString(strings.Repeat(tw.Space, colWidth))
		} else {
			// Calculate how many times to repeat the segment
			segmentWidth := twwidth.Width(segment)
			if segmentWidth <= 0 {
				segmentWidth = 1
			}
			repeat := 0
			if colWidth > 0 && segmentWidth > 0 {
				repeat = colWidth / segmentWidth
			}
			drawnSegment := strings.Repeat(segment, repeat)
			line.WriteString(drawnSegment)

			// Adjust for width discrepancies
			actualDrawnWidth := twwidth.Width(drawnSegment)
			if actualDrawnWidth < colWidth {
				missingWidth := colWidth - actualDrawnWidth
				spaces := strings.Repeat(tw.Space, missingWidth)
				if len(c.config.Border.BG) > 0 {
					line.WriteString(Tint{BG: c.config.Border.BG}.Apply(spaces))
				} else {
					line.WriteString(spaces)
				}
				c.logger.Debugf("Line: colIdx=%d corrected segment width, added %d spaces", currentColIdx, missingWidth)
			} else if actualDrawnWidth > colWidth {
				c.logger.Debugf("Line: WARNING colIdx=%d segment draw width %d > target %d", currentColIdx, actualDrawnWidth, colWidth)
			}
		}

		// Add junction between columns if not the last visible column
		isLastVisible := keyIndex == len(effectiveKeys)-1
		if !isLastVisible && c.config.Settings.Separators.BetweenColumns.Enabled() {
			nextVisibleColIdx := effectiveKeys[keyIndex+1]
			originalPrecedingCol := -1
			foundCurrent := false
			for _, k := range allSortedKeys {
				if k == currentColIdx {
					foundCurrent = true
				}
				if foundCurrent && k < nextVisibleColIdx {
					originalPrecedingCol = k
				}
				if k >= nextVisibleColIdx {
					break
				}
			}

			if originalPrecedingCol != -1 {
				jr.colIdx = originalPrecedingCol
				junction := jr.RenderJunction(originalPrecedingCol, nextVisibleColIdx)
				c.logger.Debugf("Line: Junction between visible %d (orig preceding %d) and next visible %d: '%s'", currentColIdx, originalPrecedingCol, nextVisibleColIdx, junction)
				line.WriteString(junction)
			} else {
				c.logger.Debugf("Line: Could not determine original preceding column for junction before visible %d", nextVisibleColIdx)
				line.WriteString(c.config.Separator.Apply(jr.sym.Center()))
			}
		}
	}

	// Add right border if enabled
	if c.config.Borders.Right.Enabled() {
		originalLastColIdx := -1
		if len(allSortedKeys) > 0 {
			originalLastColIdx = allSortedKeys[len(allSortedKeys)-1]
		}
		jr.colIdx = originalLastColIdx
		line.WriteString(jr.RenderRight(originalLastColIdx))
	}

	// Write the final line
	line.WriteString(c.newLine)
	c.w.Write([]byte(line.String()))
	c.logger.Debugf("Line rendered: %s", strings.TrimSuffix(line.String(), c.newLine))
}

// Logger sets the logger for the Colorized instance.
func (c *Colorized) Logger(logger *ll.Logger) {
	c.logger = logger.Namespace("colorized")
}

// Reset clears the renderer's internal state, including debug traces.
func (c *Colorized) Reset() {
	c.trace = nil
	c.logger.Debugf("Reset: Cleared debug trace")
}

// Row renders a table data row with configured colors and formatting.
func (c *Colorized) Row(row []string, ctx tw.Formatting) {
	c.logger.Debugf("Starting Row render: IsSubRow=%v, Location=%v, Pos=%s, hasFooter=%v",
		ctx.IsSubRow, ctx.Row.Location, ctx.Row.Position, ctx.HasFooter)

	// Check if there is data to render
	if len(row) == 0 {
		c.logger.Debugf("Row: No data to render")
		return
	}

	// Render the row line
	c.renderLine(ctx, row, c.config.Column)
	c.logger.Debugf("Completed Row render")
}

// Start initializes the rendering process (no-op in this implementation).
func (c *Colorized) Start(w io.Writer) error {
	c.w = w
	c.logger.Debugf("Colorized.Start() called (no-op).")
	return nil
}

// formatCell formats a cell's content with color, width, padding, and alignment, handling whitespace trimming and truncation.
func (c *Colorized) formatCell(content string, width int, padding tw.Padding, align tw.Align, tint Tint) string {
	c.logger.Debugf("Formatting cell: content='%s', width=%d, align=%s, paddingL='%s', paddingR='%s', tintFG=%v, tintBG=%v",
		content, width, align, padding.Left, padding.Right, tint.FG, tint.BG)

	// Return empty string if width is non-positive
	if width <= 0 {
		c.logger.Debugf("formatCell: width %d <= 0, returning empty string", width)
		return tw.Empty
	}

	// Calculate visual width of content
	contentVisualWidth := twwidth.Width(content)

	// Set padding characters
	padLeftCharStr := padding.Left
	padRightCharStr := padding.Right

	// Determine the character to use for alignment filling.
	// We default to the padding character defined for that side.
	// If the padding character is empty (e.g. Overwrite: true), we MUST fallback to Space
	// for the alignment calculation to prevent the content from shifting incorrectly.
	alignFillLeft := padLeftCharStr
	if alignFillLeft == tw.Empty {
		alignFillLeft = tw.Space
	}
	alignFillRight := padRightCharStr
	if alignFillRight == tw.Empty {
		alignFillRight = tw.Space
	}

	// Calculate padding widths
	definedPadLeftWidth := twwidth.Width(padLeftCharStr)
	definedPadRightWidth := twwidth.Width(padRightCharStr)

	// Calculate available width for content and alignment
	availableForContentAndAlign := max(width-definedPadLeftWidth-definedPadRightWidth, 0)

	// Truncate content if it exceeds available width
	if contentVisualWidth > availableForContentAndAlign {
		content = twwidth.Truncate(content, availableForContentAndAlign)
		contentVisualWidth = twwidth.Width(content)
		c.logger.Debugf("Truncated content to fit %d: '%s' (new width %d)", availableForContentAndAlign, content, contentVisualWidth)
	}

	// Calculate remaining space for alignment
	remainingSpaceForAlignment := max(availableForContentAndAlign-contentVisualWidth, 0)

	// Apply alignment padding
	// Note: We use tw.Pad* helpers here instead of strings.Repeat to handle multi-byte fill chars correctly.
	leftAlignmentPadSpaces := tw.Empty
	rightAlignmentPadSpaces := tw.Empty

	switch align {
	case tw.AlignLeft:
		rightAlignmentPadSpaces = tw.PadRight(tw.Empty, alignFillRight, remainingSpaceForAlignment)
	case tw.AlignRight:
		leftAlignmentPadSpaces = tw.PadLeft(tw.Empty, alignFillLeft, remainingSpaceForAlignment)
	case tw.AlignCenter:
		leftSpacesCount := remainingSpaceForAlignment / 2
		rightSpacesCount := remainingSpaceForAlignment - leftSpacesCount
		if leftSpacesCount > 0 {
			leftAlignmentPadSpaces = tw.PadLeft(tw.Empty, alignFillLeft, leftSpacesCount)
		}
		if rightSpacesCount > 0 {
			rightAlignmentPadSpaces = tw.PadRight(tw.Empty, alignFillRight, rightSpacesCount)
		}
	default:
		// Default to left alignment
		rightAlignmentPadSpaces = tw.PadRight(tw.Empty, alignFillRight, remainingSpaceForAlignment)
	}

	// Apply colors to content and padding
	coloredContent := tint.Apply(content)
	coloredPadLeft := padLeftCharStr
	coloredPadRight := padRightCharStr
	coloredAlignPadLeft := leftAlignmentPadSpaces
	coloredAlignPadRight := rightAlignmentPadSpaces

	if len(tint.BG) > 0 {
		bgTint := Tint{BG: tint.BG}
		// Apply foreground color to non-space padding if foreground is defined
		if len(tint.FG) > 0 && padLeftCharStr != tw.Space {
			coloredPadLeft = tint.Apply(padLeftCharStr)
		} else {
			coloredPadLeft = bgTint.Apply(padLeftCharStr)
		}
		if len(tint.FG) > 0 && padRightCharStr != tw.Space {
			coloredPadRight = tint.Apply(padRightCharStr)
		} else {
			coloredPadRight = bgTint.Apply(padRightCharStr)
		}
		// Apply background color to alignment padding
		if leftAlignmentPadSpaces != tw.Empty {
			coloredAlignPadLeft = bgTint.Apply(leftAlignmentPadSpaces)
		}
		if rightAlignmentPadSpaces != tw.Empty {
			coloredAlignPadRight = bgTint.Apply(rightAlignmentPadSpaces)
		}
	} else if len(tint.FG) > 0 {
		// Apply foreground color to non-space padding
		if padLeftCharStr != tw.Space {
			coloredPadLeft = tint.Apply(padLeftCharStr)
		}
		if padRightCharStr != tw.Space {
			coloredPadRight = tint.Apply(padRightCharStr)
		}
	}

	// Build final cell string
	var sb strings.Builder
	sb.WriteString(coloredPadLeft)
	sb.WriteString(coloredAlignPadLeft)
	sb.WriteString(coloredContent)
	sb.WriteString(coloredAlignPadRight)
	sb.WriteString(coloredPadRight)
	output := sb.String()

	// Adjust output width if necessary (safety check)
	currentVisualWidth := twwidth.Width(output)
	if currentVisualWidth != width {
		c.logger.Debugf("formatCell MISMATCH: content='%s', target_w=%d. Calculated parts width = %d. String: '%s'",
			content, width, currentVisualWidth, output)
		if currentVisualWidth > width {
			output = twwidth.Truncate(output, width)
		} else {
			paddingSpacesStr := strings.Repeat(tw.Space, width-currentVisualWidth)
			if len(tint.BG) > 0 {
				output += Tint{BG: tint.BG}.Apply(paddingSpacesStr)
			} else {
				output += paddingSpacesStr
			}
		}
		c.logger.Debugf("formatCell Post-Correction: Target %d, New Visual width %d. Output: '%s'", width, twwidth.Width(output), output)
	}

	c.logger.Debugf("Formatted cell final result: '%s' (target width %d, display width %d)", output, width, twwidth.Width(output))
	return output
}

// renderLine renders a single line (header, row, or footer) with colors, handling merges and separators.
func (c *Colorized) renderLine(ctx tw.Formatting, line []string, tint Tint) {
	// Determine number of columns
	numCols := 0
	if len(ctx.Row.Current) > 0 {
		maxKey := -1
		for k := range ctx.Row.Current {
			if k > maxKey {
				maxKey = k
			}
		}
		numCols = maxKey + 1
	} else {
		maxKey := -1
		for k := range ctx.Row.Widths {
			if k > maxKey {
				maxKey = k
			}
		}
		numCols = maxKey + 1
	}

	var output strings.Builder

	// Add left border if enabled
	prefix := tw.Empty
	if c.config.Borders.Left.Enabled() {
		prefix = c.config.Border.Apply(c.config.Symbols.Column())
	}
	output.WriteString(prefix)

	// Set up separator
	separatorDisplayWidth := 0
	separatorString := tw.Empty
	if c.config.Settings.Separators.BetweenColumns.Enabled() {
		separatorString = c.config.Separator.Apply(c.config.Symbols.Column())
		separatorDisplayWidth = twwidth.Width(c.config.Symbols.Column())
	}

	// Process each column
	for i := 0; i < numCols; {
		// Determine if a separator is needed
		shouldAddSeparator := false
		if i > 0 && c.config.Settings.Separators.BetweenColumns.Enabled() {
			cellCtx, ok := ctx.Row.Current[i]
			if !ok || (!cellCtx.Merge.Horizontal.Present || cellCtx.Merge.Horizontal.Start) {
				shouldAddSeparator = true
			}
		}
		if shouldAddSeparator {
			output.WriteString(separatorString)
			c.logger.Debugf("renderLine: Added separator '%s' before col %d", separatorString, i)
		} else if i > 0 {
			c.logger.Debugf("renderLine: Skipped separator before col %d due to HMerge continuation", i)
		}

		// Get cell context, use default if not present
		cellCtx, ok := ctx.Row.Current[i]
		if !ok {
			cellCtx = tw.CellContext{
				Data:    tw.Empty,
				Align:   c.defaultAlign[ctx.Row.Position],
				Padding: tw.Padding{Left: tw.Space, Right: tw.Space},
				Width:   ctx.Row.Widths.Get(i),
				Merge:   tw.MergeState{},
			}
		}

		// Handle merged cells
		visualWidth := 0
		span := 1
		isHMergeStart := ok && cellCtx.Merge.Horizontal.Present && cellCtx.Merge.Horizontal.Start

		if isHMergeStart {
			span = cellCtx.Merge.Horizontal.Span
			if ctx.Row.Position == tw.Row {
				// Calculate dynamic width for row merges
				dynamicTotalWidth := 0
				for k := 0; k < span && i+k < numCols; k++ {
					colToSum := i + k
					normWidth := max(ctx.NormalizedWidths.Get(colToSum), 0)
					dynamicTotalWidth += normWidth
					if k > 0 && separatorDisplayWidth > 0 {
						dynamicTotalWidth += separatorDisplayWidth
					}
				}
				visualWidth = dynamicTotalWidth
				c.logger.Debugf("renderLine: Row HMerge col %d, span %d, dynamic visualWidth %d", i, span, visualWidth)
			} else {
				visualWidth = ctx.Row.Widths.Get(i)
				c.logger.Debugf("renderLine: H/F HMerge col %d, span %d, pre-adjusted visualWidth %d", i, span, visualWidth)
			}
		} else {
			visualWidth = ctx.Row.Widths.Get(i)
			c.logger.Debugf("renderLine: Regular col %d, visualWidth %d", i, visualWidth)
		}
		if visualWidth < 0 {
			visualWidth = 0
		}

		// Skip processing for non-start merged cells
		if ok && cellCtx.Merge.Horizontal.Present && !cellCtx.Merge.Horizontal.Start {
			c.logger.Debugf("renderLine: Skipping col %d processing (part of HMerge)", i)
			i++
			continue
		}

		// Handle empty cell context with non-zero width
		if !ok && visualWidth > 0 {
			spaces := strings.Repeat(tw.Space, visualWidth)
			if len(tint.BG) > 0 {
				output.WriteString(Tint{BG: tint.BG}.Apply(spaces))
			} else {
				output.WriteString(spaces)
			}
			c.logger.Debugf("renderLine: No cell context for col %d, writing %d spaces", i, visualWidth)
			i += span
			continue
		}

		// Set cell alignment
		padding := cellCtx.Padding
		align := cellCtx.Align
		if align == tw.AlignNone {
			align = c.defaultAlign[ctx.Row.Position]
			c.logger.Debugf("renderLine: col %d using default renderer align '%s' for position %s because cellCtx.Align was AlignNone", i, align, ctx.Row.Position)
		}

		// Detect and handle TOTAL pattern
		isTotalPattern := false
		if i == 0 && isHMergeStart && cellCtx.Merge.Horizontal.Span >= 3 && strings.TrimSpace(cellCtx.Data) == "TOTAL" {
			isTotalPattern = true
			c.logger.Debugf("renderLine: Detected 'TOTAL' HMerge pattern at col 0")
		}
		// Override alignment for footer merges or TOTAL pattern
		if (ctx.Row.Position == tw.Footer && isHMergeStart) || isTotalPattern {
			if align == tw.AlignNone {
				c.logger.Debugf("renderLine: Applying AlignRight override for Footer HMerge/TOTAL pattern at col %d. Original/default align was: %s", i, align)
				align = tw.AlignRight
			}
		}

		// Handle vertical/hierarchical merges
		content := cellCtx.Data
		if (cellCtx.Merge.Vertical.Present && !cellCtx.Merge.Vertical.Start) ||
			(cellCtx.Merge.Hierarchical.Present && !cellCtx.Merge.Hierarchical.Start) {
			content = tw.Empty
			c.logger.Debugf("renderLine: Blanked data for col %d (non-start V/Hierarchical)", i)
		}

		// Apply per-column tint if available
		cellTint := tint
		if i < len(tint.Columns) {
			columnTint := tint.Columns[i]
			if len(columnTint.FG) > 0 || len(columnTint.BG) > 0 {
				cellTint = columnTint
			}
		}

		// Format and render the cell
		formattedCell := c.formatCell(content, visualWidth, padding, align, cellTint)
		if len(formattedCell) > 0 {
			output.WriteString(formattedCell)
		} else if visualWidth == 0 && isHMergeStart {
			c.logger.Debugf("renderLine: Rendered HMerge START col %d resulted in 0 visual width, wrote nothing.", i)
		} else if visualWidth == 0 {
			c.logger.Debugf("renderLine: Rendered regular col %d resulted in 0 visual width, wrote nothing.", i)
		}

		// Log rendering details
		if isHMergeStart {
			c.logger.Debugf("renderLine: Rendered HMerge START col %d (span %d, visualWidth %d, align %s): '%s'",
				i, span, visualWidth, align, formattedCell)
		} else {
			c.logger.Debugf("renderLine: Rendered regular col %d (visualWidth %d, align %s): '%s'",
				i, visualWidth, align, formattedCell)
		}

		i += span
	}

	// Add right border if enabled
	suffix := tw.Empty
	if c.config.Borders.Right.Enabled() {
		suffix = c.config.Border.Apply(c.config.Symbols.Column())
	}
	output.WriteString(suffix)

	// Write the final line
	output.WriteString(c.newLine)
	c.w.Write([]byte(output.String()))
	c.logger.Debugf("renderLine: Final rendered line: %s", strings.TrimSuffix(output.String(), c.newLine))
}

// Rendition updates the parts of ColorizedConfig that correspond to tw.Rendition
// by merging the provided newRendition. Color-specific Tints are not modified.
func (c *Colorized) Rendition(newRendition tw.Rendition) { // Method name matches interface
	c.logger.Debug("Colorized.Rendition called. Current B/Sym/Set: B:%+v, Sym:%T, S:%+v. Override: %+v", c.config.Borders, c.config.Symbols, c.config.Settings, newRendition)

	currentRenditionPart := tw.Rendition{
		Borders:  c.config.Borders,
		Symbols:  c.config.Symbols,
		Settings: c.config.Settings,
	}

	mergedRenditionPart := mergeRendition(currentRenditionPart, newRendition)

	c.config.Borders = mergedRenditionPart.Borders
	c.config.Symbols = mergedRenditionPart.Symbols
	if c.config.Symbols == nil {
		c.config.Symbols = tw.NewSymbols(tw.StyleLight)
	}
	c.config.Settings = mergedRenditionPart.Settings

	c.logger.Debugf("Colorized.Rendition updated. New B/Sym/Set: B:%+v, Sym:%T, S:%+v",
		c.config.Borders, c.config.Symbols, c.config.Settings)
}

// Ensure Colorized implements tw.Renditioning
var _ tw.Renditioning = (*Colorized)(nil)
