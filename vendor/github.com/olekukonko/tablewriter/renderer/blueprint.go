package renderer

import (
	"io"
	"strings"

	"github.com/olekukonko/ll"
	"github.com/olekukonko/tablewriter/pkg/twwidth"

	"github.com/olekukonko/tablewriter/tw"
)

// Blueprint implements a primary table rendering engine with customizable borders and alignments.
type Blueprint struct {
	config tw.Rendition // Rendering configuration for table borders and symbols
	logger *ll.Logger   // Logger for debug trace messages
	w      io.Writer
}

// NewBlueprint creates a new Blueprint instance with optional custom configurations.
func NewBlueprint(configs ...tw.Rendition) *Blueprint {
	// Initialize with default configuration
	cfg := defaultBlueprint()
	if len(configs) > 0 {
		userCfg := configs[0]
		// Override default borders if provided
		if userCfg.Borders.Left != 0 {
			cfg.Borders.Left = userCfg.Borders.Left
		}
		if userCfg.Borders.Right != 0 {
			cfg.Borders.Right = userCfg.Borders.Right
		}
		if userCfg.Borders.Top != 0 {
			cfg.Borders.Top = userCfg.Borders.Top
		}
		if userCfg.Borders.Bottom != 0 {
			cfg.Borders.Bottom = userCfg.Borders.Bottom
		}
		// Override symbols if provided
		if userCfg.Symbols != nil {
			cfg.Symbols = userCfg.Symbols
		}

		// Merge user settings with default settings
		cfg.Settings = mergeSettings(cfg.Settings, userCfg.Settings)
	}
	return &Blueprint{config: cfg, logger: ll.New("blueprint").Disable()}
}

// Close performs cleanup (no-op in this implementation).
func (f *Blueprint) Close() error {
	f.logger.Debug("Blueprint.Close() called (no-op).")
	return nil
}

// Config returns the renderer's current configuration.
func (f *Blueprint) Config() tw.Rendition {
	return f.config
}

// Footer renders the table footer section with configured formatting.
func (f *Blueprint) Footer(footers [][]string, ctx tw.Formatting) {
	f.logger.Debugf("Starting Footer render: IsSubRow=%v, Location=%v, Pos=%s", ctx.IsSubRow, ctx.Row.Location, ctx.Row.Position)
	// Render the footer line
	f.renderLine(ctx)
	f.logger.Debug("Completed Footer render")
}

// Header renders the table header section with configured formatting.
func (f *Blueprint) Header(headers [][]string, ctx tw.Formatting) {
	f.logger.Debugf("Starting Header render: IsSubRow=%v, Location=%v, Pos=%s, lines=%d, widths=%v",
		ctx.IsSubRow, ctx.Row.Location, ctx.Row.Position, len(ctx.Row.Current), ctx.Row.Widths)
	// Render the header line
	f.renderLine(ctx)
	f.logger.Debug("Completed Header render")
}

// Line renders a full horizontal row line with junctions and segments.
func (f *Blueprint) Line(ctx tw.Formatting) {
	// Initialize junction renderer
	jr := NewJunction(JunctionContext{
		Symbols:       f.config.Symbols,
		Ctx:           ctx,
		ColIdx:        0,
		Logger:        f.logger,
		BorderTint:    Tint{},
		SeparatorTint: Tint{},
	})

	var line strings.Builder
	totalLineWidth := 0 // Track total display width
	// Get sorted column indices
	sortedKeys := ctx.Row.Widths.SortedKeys()
	numCols := 0
	if len(sortedKeys) > 0 {
		numCols = sortedKeys[len(sortedKeys)-1] + 1
	}

	// Handle empty row case
	if numCols == 0 {
		prefix := tw.Empty
		suffix := tw.Empty
		if f.config.Borders.Left.Enabled() {
			prefix = jr.RenderLeft()
		}
		if f.config.Borders.Right.Enabled() {
			suffix = jr.RenderRight(-1)
		}
		if prefix != tw.Empty || suffix != tw.Empty {
			line.WriteString(prefix + suffix + tw.NewLine)
			totalLineWidth = twwidth.Width(prefix) + twwidth.Width(suffix)
			f.w.Write([]byte(line.String()))
		}
		f.logger.Debugf("Line: Handled empty row/widths case (total width %d)", totalLineWidth)
		return
	}

	// Calculate target total width based on data rows
	targetTotalWidth := 0
	for _, colIdx := range sortedKeys {
		targetTotalWidth += ctx.Row.Widths.Get(colIdx)
	}
	if f.config.Borders.Left.Enabled() {
		targetTotalWidth += twwidth.Width(f.config.Symbols.Column())
	}
	if f.config.Borders.Right.Enabled() {
		targetTotalWidth += twwidth.Width(f.config.Symbols.Column())
	}
	if f.config.Settings.Separators.BetweenColumns.Enabled() && len(sortedKeys) > 1 {
		targetTotalWidth += twwidth.Width(f.config.Symbols.Column()) * (len(sortedKeys) - 1)
	}

	// Add left border if enabled
	leftBorderWidth := 0
	if f.config.Borders.Left.Enabled() {
		leftBorder := jr.RenderLeft()
		line.WriteString(leftBorder)
		leftBorderWidth = twwidth.Width(leftBorder)
		totalLineWidth += leftBorderWidth
		f.logger.Debugf("Line: Left border='%s' (f.width %d)", leftBorder, leftBorderWidth)
	}

	visibleColIndices := make([]int, 0)
	// Calculate visible columns
	for _, colIdx := range sortedKeys {
		colWidth := ctx.Row.Widths.Get(colIdx)
		if colWidth > 0 {
			visibleColIndices = append(visibleColIndices, colIdx)
		}
	}

	f.logger.Debugf("Line: sortedKeys=%v, Widths=%v, visibleColIndices=%v, targetTotalWidth=%d", sortedKeys, ctx.Row.Widths, visibleColIndices, targetTotalWidth)
	// Render each column segment
	for keyIndex, currentColIdx := range visibleColIndices {
		jr.colIdx = currentColIdx
		segment := jr.GetSegment()
		colWidth := ctx.Row.Widths.Get(currentColIdx)
		// Adjust colWidth to account for wider borders
		adjustedColWidth := colWidth
		if f.config.Borders.Left.Enabled() && keyIndex == 0 {
			adjustedColWidth -= leftBorderWidth - twwidth.Width(f.config.Symbols.Column())
		}
		if f.config.Borders.Right.Enabled() && keyIndex == len(visibleColIndices)-1 {
			rightBorderWidth := twwidth.Width(jr.RenderRight(currentColIdx))
			adjustedColWidth -= rightBorderWidth - twwidth.Width(f.config.Symbols.Column())
		}
		if adjustedColWidth < 0 {
			adjustedColWidth = 0
		}
		f.logger.Debugf("Line: colIdx=%d, segment='%s', adjusted colWidth=%d", currentColIdx, segment, adjustedColWidth)
		if segment == tw.Empty {
			spaces := strings.Repeat(tw.Space, adjustedColWidth)
			line.WriteString(spaces)
			totalLineWidth += adjustedColWidth
			f.logger.Debugf("Line: Rendered spaces='%s' (f.width %d) for col %d", spaces, adjustedColWidth, currentColIdx)
		} else {
			segmentWidth := twwidth.Width(segment)
			if segmentWidth == 0 {
				segmentWidth = 1 // Avoid division by zero
				f.logger.Warnf("Line: Segment='%s' has zero width, using 1", segment)
			}
			// Calculate how many full segments fit
			repeat := adjustedColWidth / segmentWidth
			if repeat < 1 && adjustedColWidth > 0 {
				repeat = 1
			}
			repeatedSegment := strings.Repeat(segment, repeat)
			actualWidth := twwidth.Width(repeatedSegment)
			if actualWidth > adjustedColWidth {
				// Truncate if too long
				repeatedSegment = twwidth.Truncate(repeatedSegment, adjustedColWidth)
				actualWidth = twwidth.Width(repeatedSegment)
				f.logger.Debugf("Line: Truncated segment='%s' to width %d", repeatedSegment, actualWidth)
			} else if actualWidth < adjustedColWidth {
				// Pad with segment character to match adjustedColWidth
				remainingWidth := adjustedColWidth - actualWidth
				for i := 0; i < remainingWidth/segmentWidth; i++ {
					repeatedSegment += segment
				}
				actualWidth = twwidth.Width(repeatedSegment)
				if actualWidth < adjustedColWidth {
					repeatedSegment = tw.PadRight(repeatedSegment, tw.Space, adjustedColWidth)
					actualWidth = adjustedColWidth
					f.logger.Debugf("Line: Padded segment with spaces='%s' to width %d", repeatedSegment, actualWidth)
				}
				f.logger.Debugf("Line: Padded segment='%s' to width %d", repeatedSegment, actualWidth)
			}
			line.WriteString(repeatedSegment)
			totalLineWidth += actualWidth
			f.logger.Debugf("Line: Rendered segment='%s' (f.width %d) for col %d", repeatedSegment, actualWidth, currentColIdx)
		}

		// Add junction between columns if not the last column
		isLast := keyIndex == len(visibleColIndices)-1
		if !isLast && f.config.Settings.Separators.BetweenColumns.Enabled() {
			nextColIdx := visibleColIndices[keyIndex+1]
			junction := jr.RenderJunction(currentColIdx, nextColIdx)
			// Use center symbol (❀) or column separator (|) to match data rows
			if twwidth.Width(junction) != twwidth.Width(f.config.Symbols.Column()) {
				junction = f.config.Symbols.Center()
				if twwidth.Width(junction) != twwidth.Width(f.config.Symbols.Column()) {
					junction = f.config.Symbols.Column()
				}
			}
			junctionWidth := twwidth.Width(junction)
			line.WriteString(junction)
			totalLineWidth += junctionWidth
			f.logger.Debugf("Line: Junction between %d and %d: '%s' (f.width %d)", currentColIdx, nextColIdx, junction, junctionWidth)
		}
	}

	// Add right border
	rightBorderWidth := 0
	if f.config.Borders.Right.Enabled() && len(visibleColIndices) > 0 {
		lastIdx := visibleColIndices[len(visibleColIndices)-1]
		rightBorder := jr.RenderRight(lastIdx)
		rightBorderWidth = twwidth.Width(rightBorder)
		line.WriteString(rightBorder)
		totalLineWidth += rightBorderWidth
		f.logger.Debugf("Line: Right border='%s' (f.width %d)", rightBorder, rightBorderWidth)
	}

	// Write the final line
	line.WriteString(tw.NewLine)
	f.w.Write([]byte(line.String()))
	f.logger.Debugf("Line rendered: '%s' (total width %d, target %d)", strings.TrimSuffix(line.String(), tw.NewLine), totalLineWidth, targetTotalWidth)
}

// Logger sets the logger for the Blueprint instance.
func (f *Blueprint) Logger(logger *ll.Logger) {
	f.logger = logger.Namespace("blueprint")
}

// Row renders a table data row with configured formatting.
func (f *Blueprint) Row(row []string, ctx tw.Formatting) {
	f.logger.Debugf("Starting Row render: IsSubRow=%v, Location=%v, Pos=%s, hasFooter=%v",
		ctx.IsSubRow, ctx.Row.Location, ctx.Row.Position, ctx.HasFooter)

	// Render the row line
	f.renderLine(ctx)
	f.logger.Debug("Completed Row render")
}

// Start initializes the rendering process (no-op in this implementation).
func (f *Blueprint) Start(w io.Writer) error {
	f.w = w
	f.logger.Debug("Blueprint.Start() called (no-op).")
	return nil
}

// formatCell formats a cell's content with specified width, padding, and alignment, returning an empty string if width is non-positive.
func (f *Blueprint) formatCell(content string, width int, padding tw.Padding, align tw.Align) string {
	if width <= 0 {
		return tw.Empty
	}

	f.logger.Debugf("Formatting cell: content='%s', width=%d, align=%s, padding={L:'%s' R:'%s'}",
		content, width, align, padding.Left, padding.Right)

	// Calculate display width of content
	runeWidth := twwidth.Width(content)

	// Set default padding characters
	leftPadChar := padding.Left
	rightPadChar := padding.Right

	// if f.config.Settings.Cushion.Enabled() || f.config.Settings.Cushion.Default() {
	//	if leftPadChar == tw.Empty {
	//		leftPadChar = tw.Space
	//	}
	//	if rightPadChar == tw.Empty {
	//		rightPadChar = tw.Space
	//	}
	//}

	// Calculate padding widths
	padLeftWidth := twwidth.Width(leftPadChar)
	padRightWidth := twwidth.Width(rightPadChar)

	// Calculate available width for content
	availableContentWidth := max(width-padLeftWidth-padRightWidth, 0)
	f.logger.Debugf("Available content width: %d", availableContentWidth)

	// Truncate content if it exceeds available width
	if runeWidth > availableContentWidth {
		content = twwidth.Truncate(content, availableContentWidth)
		runeWidth = twwidth.Width(content)
		f.logger.Debugf("Truncated content to fit %d: '%s' (new width %d)", availableContentWidth, content, runeWidth)
	}

	// Calculate total padding needed
	totalPaddingWidth := max(width-runeWidth, 0)
	f.logger.Debugf("Total padding width: %d", totalPaddingWidth)

	var result strings.Builder
	var leftPaddingWidth, rightPaddingWidth int

	// Apply alignment and padding
	switch align {
	case tw.AlignLeft:
		result.WriteString(leftPadChar)
		result.WriteString(content)
		rightPaddingWidth = totalPaddingWidth - padLeftWidth
		if rightPaddingWidth > 0 {
			padChar := rightPadChar
			if padChar == tw.Empty {
				padChar = tw.Space
			}
			result.WriteString(tw.PadRight(tw.Empty, padChar, rightPaddingWidth))
			f.logger.Debugf("Applied right padding: '%s' for %d width", padChar, rightPaddingWidth)
		}
	case tw.AlignRight:
		leftPaddingWidth = totalPaddingWidth - padRightWidth
		if leftPaddingWidth > 0 {
			padChar := leftPadChar
			if padChar == tw.Empty {
				padChar = tw.Space
			}
			result.WriteString(tw.PadLeft(tw.Empty, padChar, leftPaddingWidth))
			f.logger.Debugf("Applied left padding: '%s' for %d width", padChar, leftPaddingWidth)
		}
		result.WriteString(content)
		result.WriteString(rightPadChar)
	case tw.AlignCenter:
		leftPaddingWidth = (totalPaddingWidth-padLeftWidth-padRightWidth)/2 + padLeftWidth
		rightPaddingWidth = totalPaddingWidth - leftPaddingWidth
		if leftPaddingWidth > padLeftWidth {
			padChar := leftPadChar
			if padChar == tw.Empty {
				padChar = tw.Space
			}
			result.WriteString(tw.PadLeft(tw.Empty, padChar, leftPaddingWidth-padLeftWidth))
			f.logger.Debugf("Applied left centering padding: '%s' for %d width", padChar, leftPaddingWidth-padLeftWidth)
		}
		result.WriteString(leftPadChar)
		result.WriteString(content)
		result.WriteString(rightPadChar)
		if rightPaddingWidth > padRightWidth {
			padChar := rightPadChar
			if padChar == tw.Empty {
				padChar = tw.Space
			}
			result.WriteString(tw.PadRight(tw.Empty, padChar, rightPaddingWidth-padRightWidth))
			f.logger.Debugf("Applied right centering padding: '%s' for %d width", padChar, rightPaddingWidth-padRightWidth)
		}
	default:
		// Default to left alignment
		result.WriteString(leftPadChar)
		result.WriteString(content)
		rightPaddingWidth = totalPaddingWidth - padLeftWidth
		if rightPaddingWidth > 0 {
			padChar := rightPadChar
			if padChar == tw.Empty {
				padChar = tw.Space
			}
			result.WriteString(tw.PadRight(tw.Empty, padChar, rightPaddingWidth))
			f.logger.Debugf("Applied right padding: '%s' for %d width", padChar, rightPaddingWidth)
		}
	}

	output := result.String()
	finalWidth := twwidth.Width(output)
	// Adjust output to match target width
	if finalWidth > width {
		output = twwidth.Truncate(output, width)
		f.logger.Debugf("formatCell: Truncated output to width %d", width)
	} else if finalWidth < width {
		output = tw.PadRight(output, tw.Space, width)
		f.logger.Debugf("formatCell: Padded output to meet width %d", width)
	}

	// Log warning if final width doesn't match target
	if f.logger.Enabled() && twwidth.Width(output) != width {
		f.logger.Debugf("formatCell Warning: Final width %d does not match target %d for result '%s'",
			twwidth.Width(output), width, output)
	}

	f.logger.Debugf("Formatted cell final result: '%s' (target width %d)", output, width)
	return output
}

// renderLine renders a single line (header, row, or footer) with borders, separators, and merge handling.
func (f *Blueprint) renderLine(ctx tw.Formatting) {
	// Get sorted column indices
	sortedKeys := ctx.Row.Widths.SortedKeys()
	numCols := 0
	if len(sortedKeys) > 0 {
		numCols = sortedKeys[len(sortedKeys)-1] + 1
	}

	// Set column separator and borders
	columnSeparator := f.config.Symbols.Column()
	prefix := tw.Empty
	if f.config.Borders.Left.Enabled() {
		prefix = columnSeparator
	}
	suffix := tw.Empty
	if f.config.Borders.Right.Enabled() {
		suffix = columnSeparator
	}

	var output strings.Builder
	totalLineWidth := 0 // Track total display width
	if prefix != tw.Empty {
		output.WriteString(prefix)
		totalLineWidth += twwidth.Width(prefix)
		f.logger.Debugf("renderLine: Prefix='%s' (f.width %d)", prefix, twwidth.Width(prefix))
	}

	colIndex := 0
	separatorDisplayWidth := 0
	if f.config.Settings.Separators.BetweenColumns.Enabled() {
		separatorDisplayWidth = twwidth.Width(columnSeparator)
	}

	// Process each column
	for colIndex < numCols {
		visualWidth := ctx.Row.Widths.Get(colIndex)
		cellCtx, ok := ctx.Row.Current[colIndex]
		isHMergeStart := ok && cellCtx.Merge.Horizontal.Present && cellCtx.Merge.Horizontal.Start
		if visualWidth == 0 && !isHMergeStart {
			f.logger.Debugf("renderLine: Skipping col %d (zero width, not HMerge start)", colIndex)
			colIndex++
			continue
		}

		// Determine if a separator is needed
		shouldAddSeparator := false
		if colIndex > 0 && f.config.Settings.Separators.BetweenColumns.Enabled() {
			prevWidth := ctx.Row.Widths.Get(colIndex - 1)
			prevCellCtx, prevOk := ctx.Row.Current[colIndex-1]
			prevIsHMergeEnd := prevOk && prevCellCtx.Merge.Horizontal.Present && prevCellCtx.Merge.Horizontal.End
			if (prevWidth > 0 || prevIsHMergeEnd) && (!ok || (!cellCtx.Merge.Horizontal.Present || cellCtx.Merge.Horizontal.Start)) {
				shouldAddSeparator = true
			}
		}
		if shouldAddSeparator {
			output.WriteString(columnSeparator)
			totalLineWidth += separatorDisplayWidth
			f.logger.Debugf("renderLine: Added separator '%s' before col %d (f.width %d)", columnSeparator, colIndex, separatorDisplayWidth)
		} else if colIndex > 0 {
			f.logger.Debugf("renderLine: Skipped separator before col %d due to zero-width prev col or HMerge continuation", colIndex)
		}

		// Handle merged cells
		span := 1
		if isHMergeStart {
			span = cellCtx.Merge.Horizontal.Span
			if ctx.Row.Position == tw.Row {
				dynamicTotalWidth := 0
				for k := 0; k < span && colIndex+k < numCols; k++ {
					normWidth := max(ctx.NormalizedWidths.Get(colIndex+k), 0)
					dynamicTotalWidth += normWidth
					if k > 0 && separatorDisplayWidth > 0 && ctx.NormalizedWidths.Get(colIndex+k) > 0 {
						dynamicTotalWidth += separatorDisplayWidth
					}
				}
				visualWidth = dynamicTotalWidth
				f.logger.Debugf("renderLine: Row HMerge col %d, span %d, dynamic visualWidth %d", colIndex, span, visualWidth)
			} else {
				visualWidth = ctx.Row.Widths.Get(colIndex)
				f.logger.Debugf("renderLine: H/F HMerge col %d, span %d, pre-adjusted visualWidth %d", colIndex, span, visualWidth)
			}
		} else {
			visualWidth = ctx.Row.Widths.Get(colIndex)
			f.logger.Debugf("renderLine: Regular col %d, visualWidth %d", colIndex, visualWidth)
		}
		if visualWidth < 0 {
			visualWidth = 0
		}

		// Skip processing for non-start merged cells
		if ok && cellCtx.Merge.Horizontal.Present && !cellCtx.Merge.Horizontal.Start {
			f.logger.Debugf("renderLine: Skipping col %d processing (part of HMerge)", colIndex)
			colIndex++
			continue
		}

		// Handle empty cell context
		if !ok {
			if visualWidth > 0 {
				spaces := strings.Repeat(tw.Space, visualWidth)
				output.WriteString(spaces)
				totalLineWidth += visualWidth
				f.logger.Debugf("renderLine: No cell context for col %d, writing %d spaces (f.width %d)", colIndex, visualWidth, visualWidth)
			} else {
				f.logger.Debugf("renderLine: No cell context for col %d, visualWidth is 0, writing nothing", colIndex)
			}
			colIndex += span
			continue
		}

		// Set cell padding and alignment
		padding := cellCtx.Padding
		align := cellCtx.Align
		switch align {
		case tw.AlignNone:
			switch ctx.Row.Position {
			case tw.Header:
				align = tw.AlignCenter
			case tw.Footer:
				align = tw.AlignRight
			default:
				align = tw.AlignLeft
			}
			f.logger.Debugf("renderLine: col %d (data: '%s') using renderer default align '%s' for position %s.", colIndex, cellCtx.Data, align, ctx.Row.Position)
		case tw.Skip:
			switch ctx.Row.Position {
			case tw.Header:
				align = tw.AlignCenter
			case tw.Footer:
				align = tw.AlignRight
			default:
				align = tw.AlignLeft
			}
			f.logger.Debugf("renderLine: col %d (data: '%s') cellCtx.Align was Skip/empty, falling back to basic default '%s'.", colIndex, cellCtx.Data, align)
		}

		isTotalPattern := false

		// Case-insensitive check for "total"
		if isHMergeStart && colIndex > 0 {
			if prevCellCtx, ok := ctx.Row.Current[colIndex-1]; ok {
				if strings.Contains(strings.ToLower(prevCellCtx.Data), "total") {
					isTotalPattern = true
					f.logger.Debugf("renderLine: total pattern in row in %d", colIndex)
				}
			}
		}

		// Get the alignment from the configuration
		align = cellCtx.Align

		// Override alignment for footer merged cells
		if (ctx.Row.Position == tw.Footer && isHMergeStart) || isTotalPattern {
			if align == tw.AlignNone {
				f.logger.Debugf("renderLine: Applying AlignRight HMerge/TOTAL override for Footer col %d. Original/default align was: %s", colIndex, align)
				align = tw.AlignRight
			}
		}

		// Handle vertical/hierarchical merges
		cellData := cellCtx.Data
		if (cellCtx.Merge.Vertical.Present && !cellCtx.Merge.Vertical.Start) ||
			(cellCtx.Merge.Hierarchical.Present && !cellCtx.Merge.Hierarchical.Start) {
			cellData = tw.Empty
			f.logger.Debugf("renderLine: Blanked data for col %d (non-start V/Hierarchical)", colIndex)
		}

		// Format and render the cell
		formattedCell := f.formatCell(cellData, visualWidth, padding, align)
		if len(formattedCell) > 0 {
			output.WriteString(formattedCell)
			cellWidth := twwidth.Width(formattedCell)
			totalLineWidth += cellWidth
			f.logger.Debugf("renderLine: Rendered col %d, formattedCell='%s' (f.width %d), totalLineWidth=%d", colIndex, formattedCell, cellWidth, totalLineWidth)
		}

		// Log rendering details
		if isHMergeStart {
			f.logger.Debugf("renderLine: Rendered HMerge START col %d (span %d, visualWidth %d, align %v): '%s'",
				colIndex, span, visualWidth, align, formattedCell)
		} else {
			f.logger.Debugf("renderLine: Rendered regular col %d (visualWidth %d, align %v): '%s'",
				colIndex, visualWidth, align, formattedCell)
		}
		colIndex += span
	}

	// Add suffix and adjust total width
	if output.Len() > len(prefix) || f.config.Borders.Right.Enabled() {
		output.WriteString(suffix)
		totalLineWidth += twwidth.Width(suffix)
		f.logger.Debugf("renderLine: Suffix='%s' (f.width %d)", suffix, twwidth.Width(suffix))
	}
	output.WriteString(tw.NewLine)
	f.w.Write([]byte(output.String()))
	f.logger.Debugf("renderLine: Final rendered line: '%s' (total width %d)", strings.TrimSuffix(output.String(), tw.NewLine), totalLineWidth)
}

// Rendition updates the Blueprint's configuration.
func (f *Blueprint) Rendition(config tw.Rendition) {
	f.config = mergeRendition(f.config, config)
	f.logger.Debugf("Blueprint.Rendition updated. New config: %+v", f.config)
}

// Ensure Blueprint implements tw.Renditioning
var _ tw.Renditioning = (*Blueprint)(nil)
