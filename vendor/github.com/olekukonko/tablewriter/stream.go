package tablewriter

import (
	"math"

	"github.com/olekukonko/errors"
	"github.com/olekukonko/tablewriter/pkg/twwidth"
	"github.com/olekukonko/tablewriter/tw"
)

// Close finalizes the table stream.
// It requires the stream to be started (by calling NewStreamTable).
// It calls the renderer's Close method to render final elements (like the bottom border) and close the stream.
func (t *Table) Close() error {
	t.logger.Debug("Close() called. Finalizing stream.")

	// Ensure stream was actually started and enabled
	if !t.config.Stream.Enable || !t.hasPrinted {
		t.logger.Warn("Close() called but streaming not enabled or not started. Ignoring Close() actions.")
		// If renderer has a Close method that should always be called, consider that.
		// For Blueprint, Close is a no-op, so returning early is fine.
		// If we always call renderer.Close(), ensure it's safe if renderer.Start() wasn't called.
		// Let's only call renderer.Close if stream was started.
		if t.hasPrinted && t.renderer != nil { // Check if renderer is not nil for safety
			t.renderer.Close() // Still call renderer's close for cleanup
		}
		t.hasPrinted = false // Reset flag
		return nil
	}

	// Render stored footer if any
	if len(t.streamFooterLines) > 0 {
		t.logger.Debug("Close(): Rendering stored footer.")
		if err := t.streamRenderFooter(t.streamFooterLines); err != nil {
			t.logger.Errorf("Close(): Failed to render stream footer: %v", err)
			// Continue to try and close renderer and render bottom border
		}
	}

	// Render the final table bottom border
	t.logger.Debug("Close(): Rendering stream bottom border.")
	if err := t.streamRenderBottomBorder(); err != nil {
		t.logger.Errorf("Close(): Failed to render stream bottom border: %v", err)
		// Continue to try and close renderer
	}

	// Call the underlying renderer's Close method
	err := t.renderer.Close()
	if err != nil {
		t.logger.Errorf("Renderer.Close() failed: %v", err)
	}

	// Reset streaming state
	t.hasPrinted = false
	t.headerRendered = false
	t.firstRowRendered = false
	t.lastRenderedLineContent = nil
	t.lastRenderedMergeState = nil
	t.lastRenderedPosition = ""
	t.streamFooterLines = nil
	// t.streamWidths should persist if we want to make multiple Start/Close calls on same config?
	// For now, let's assume Start re-evaluates. If widths are from StreamConfig, they'd be reused.
	// If derived, they'd be re-derived. Let's clear for true reset.
	t.streamWidths = tw.NewMapper[int, int]()
	t.streamNumCols = 0
	// t.streamRowCounter = 0 // Removed this field

	t.logger.Debug("Stream ended. hasPrinted = false.")
	return err // Return error from renderer.Close or other significant errors
}

// Start initializes the table stream.
// In this streaming model, renderer.Start() is primarily called in NewStreamTable.
// This method serves as a safeguard or point for adding pre-rendering logic.
// Start initializes the table stream.
// It is the entry point for streaming mode.
// Requires t.config.Stream.Enable to be true.
// Returns an error if streaming is disabled or the renderer does not support streaming,
// or if called multiple times on the same stream.
func (t *Table) Start() error {
	t.ensureInitialized() // Ensures basic setup like loggers

	if !t.config.Stream.Enable {
		// Start() should only be called when streaming is explicitly enabled.
		// Otherwise, the user should call Render() for batch mode.
		t.logger.Warn("Start() called but streaming is disabled. Call Render() instead for batch mode.")
		return errors.New("start() called but streaming is disabled")
	}

	if !t.renderer.Config().Streaming {
		// Check if the configured renderer actually supports streaming.
		t.logger.Error("Configured renderer does not support streaming.")
		return errors.Newf("renderer does not support streaming")
	}

	// t.renderer.Start(t.writer)
	// t.renderer.Logger(t.logger)

	if t.hasPrinted {
		// Prevent calling Start() multiple times on the same stream instance.
		t.logger.Warn("Start() called multiple times for the same table stream. Ignoring subsequent calls.")
		return nil
	}

	t.logger.Debug("Starting table stream.")

	// Initialize/reset streaming state flags and buffers
	t.headerRendered = false
	t.firstRowRendered = false
	t.lastRenderedLineContent = nil
	t.lastRenderedPosition = "" // Reset last rendered position
	t.streamFooterLines = nil   // Reset footer buffer
	t.streamNumCols = 0         // Reset derived column count

	// Calculate initial fixed widths if provided in StreamConfig.Widths
	// These widths will be used for all subsequent rendering in streaming mode.
	if t.config.Widths.PerColumn != nil && t.config.Widths.PerColumn.Len() > 0 {
		// Use per-column stream widths if set
		t.logger.Debugf("Using per-column stream widths from StreamConfig: %v", t.config.Widths.PerColumn)
		t.streamWidths = t.config.Widths.PerColumn.Clone()
		// Determine numCols from the highest index in PerColumn map
		maxColIdx := -1
		t.streamWidths.Each(func(col, width int) {
			if col > maxColIdx {
				maxColIdx = col
			}
			// Ensure configured widths are reasonable (>0 becomes >=1, <0 becomes 0)
			if width > 0 && width < 1 {
				t.streamWidths.Set(col, 1)
			} else if width < 0 {
				t.streamWidths.Set(col, 0) // Negative width means hide column
			}
		})
		if maxColIdx >= 0 {
			t.streamNumCols = maxColIdx + 1
			t.logger.Debugf("Derived streamNumCols from PerColumn widths: %d", t.streamNumCols)
		} else {
			// PerColumn map exists but is empty? Or all negative widths? Assume 0 columns for now.
			t.streamNumCols = 0
			t.logger.Debugf("PerColumn widths map is effectively empty or contains only negative values, streamNumCols = 0.")
		}

	} else if t.config.Widths.Global > 0 {
		// Global width is set, but we don't know the number of columns yet.
		// Defer applying global width until the first data (Header or first Row) arrives.
		// Store a placeholder or flag indicating global width should be used.
		// The simple way for now: Keep streamWidths empty, signal the global width preference.
		// The width calculation function called later will need to check StreamConfig.Widths.Global
		// if streamWidths is empty.
		t.logger.Debugf("Global stream width %d set in StreamConfig. Will derive numCols from first data.", t.config.Widths.Global)
		t.streamWidths = tw.NewMapper[int, int]() // Initialize as empty, will be populated later
		// Note: No need to store Global width value here, it's available in t.config.Stream.Widths.Global

	} else {
		// No explicit stream widths in config. They will be calculated from the first data (Header or first Row).
		t.logger.Debug("No explicit stream widths configured in StreamConfig. Will derive from first data.")
		t.streamWidths = tw.NewMapper[int, int]() // Initialize as empty, will be populated later
		t.streamNumCols = 0                       // NumCols will be determined by first data
	}

	// Log warnings if incompatible features are enabled in streaming config
	// Vertical/Hierarchical merges require processing all rows together.
	if t.config.Header.Formatting.MergeMode&(tw.MergeVertical|tw.MergeHierarchical) != 0 {
		t.logger.Warnf("Vertical or Hierarchical merge modes enabled on Header config (%d) but are unsupported in streaming mode. Only Horizontal merge will be considered.", t.config.Header.Formatting.MergeMode)
	}
	if t.config.Row.Formatting.MergeMode&(tw.MergeVertical|tw.MergeHierarchical) != 0 {
		t.logger.Warnf("Vertical or Hierarchical merge modes enabled on Row config (%d) but are unsupported in streaming mode. Only Horizontal merge will be considered.", t.config.Row.Formatting.MergeMode)
	}
	if t.config.Footer.Formatting.MergeMode&(tw.MergeVertical|tw.MergeHierarchical) != 0 {
		t.logger.Warnf("Vertical or Hierarchical merge modes enabled on Footer config (%d) but are unsupported in streaming mode. Only Horizontal merge will be considered.", t.config.Footer.Formatting.MergeMode)
	}
	// AutoHide requires processing all row data to find empty columns.
	if t.config.Behavior.AutoHide.Enabled() {
		t.logger.Warn("AutoHide is enabled in config but is ignored in streaming mode.")
	}

	// Call the renderer's start method for the stream.
	err := t.renderer.Start(t.writer)
	if err == nil {
		t.hasPrinted = true // Mark as started successfully only if renderer.Start works
		t.logger.Debug("Renderer.Start() succeeded. Table stream initiated.")
	} else {
		// Reset state if renderer.Start fails
		t.hasPrinted = false
		t.headerRendered = false
		t.firstRowRendered = false
		t.lastRenderedLineContent = nil
		t.lastRenderedPosition = ""
		t.streamFooterLines = nil
		t.streamWidths = tw.NewMapper[int, int]() // Clear any widths that might have been set
		t.streamNumCols = 0
		t.logger.Errorf("Renderer.Start() failed: %v. Streaming initialization failed.", err)
	}
	return err
}

// streamAppendRow processes and renders a single row in streaming mode.
// It calculates/uses fixed stream widths, processes content, renders separators and lines,
// and updates streaming state.
// It assumes Start() has already been called and t.hasPrinted is true.
func (t *Table) streamAppendRow(row interface{}) error {
	t.logger.Debugf("streamAppendRow called with row: %v (type: %T)", row, row)

	if !t.config.Stream.Enable {
		return errors.New("streaming mode is disabled")
	}

	rawCellsSlice, err := t.convertCellsToStrings(row, t.config.Row)
	if err != nil {
		t.logger.Errorf("streamAppendRow: Failed to convert row to strings: %v", err)
		return errors.Newf("failed to convert row to strings").Wrap(err)
	}

	if len(rawCellsSlice) == 0 {
		t.logger.Debug("streamAppendRow: No raw cells after conversion, skipping row rendering.")
		if !t.firstRowRendered {
			t.firstRowRendered = true
			t.logger.Debug("streamAppendRow: Marked first row rendered (empty content after processing).")
		}
		return nil
	}

	if err := t.ensureStreamWidthsCalculated(rawCellsSlice, t.config.Row); err != nil {
		return errors.New("failed to establish stream column count/widths").Wrap(err)
	}

	// Now, check for column mismatch if a column count has been established.
	if t.streamNumCols > 0 {
		if len(rawCellsSlice) != t.streamNumCols {
			if t.config.Stream.StrictColumns {
				err := errors.Newf("input row column count (%d) does not match established stream column count (%d) and StrictColumns is enabled", len(rawCellsSlice), t.streamNumCols)
				t.logger.Error(err.Error())
				return err
			}
			// If not strict, retain the old lenient behavior (warn and pad/truncate)
			t.logger.Warnf("streamAppendRow: Input row column count (%d) != stream column count (%d). Padding/Truncating (StrictColumns is false).", len(rawCellsSlice), t.streamNumCols)
			if len(rawCellsSlice) < t.streamNumCols {
				paddedCells := make([]string, t.streamNumCols)
				copy(paddedCells, rawCellsSlice)
				for i := len(rawCellsSlice); i < t.streamNumCols; i++ {
					paddedCells[i] = tw.Empty
				}
				rawCellsSlice = paddedCells
			} else {
				rawCellsSlice = rawCellsSlice[:t.streamNumCols]
			}
		}
	} else if len(rawCellsSlice) > 0 && t.config.Stream.StrictColumns {
		err := errors.Newf("failed to establish stream column count from first data row (%d cells) and StrictColumns is enabled", len(rawCellsSlice))
		t.logger.Error(err.Error())
		return err
	}

	if t.streamNumCols == 0 {
		t.logger.Warn("streamAppendRow: streamNumCols is 0. Cannot render row.")
		return errors.New("cannot render row, column count is zero and could not be determined")
	}

	_, rowMerges, _ := t.prepareWithMerges([][]string{rawCellsSlice}, t.config.Row, tw.Row)
	processedRowLines := t.prepareContent(rawCellsSlice, t.config.Row)
	t.logger.Debugf("streamAppendRow: Processed row lines: %d lines", len(processedRowLines))

	f := t.renderer
	cfg := t.renderer.Config()

	if !t.headerRendered && !t.firstRowRendered && t.lastRenderedPosition == "" {
		if cfg.Borders.Top.Enabled() && cfg.Settings.Lines.ShowTop.Enabled() {
			t.logger.Debug("streamAppendRow: Rendering table top border (first element is a row).")
			var nextCellsCtx map[int]tw.CellContext
			if len(processedRowLines) > 0 {
				firstRowLineResp := t.streamBuildCellContexts(
					tw.Row, 0, 0, processedRowLines, rowMerges, t.config.Row,
				)
				nextCellsCtx = firstRowLineResp.cells
			}
			f.Line(tw.Formatting{
				Row: tw.RowContext{
					Widths:       t.streamWidths,
					ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
					Next:         nextCellsCtx,
					Position:     tw.Row,
					Location:     tw.LocationFirst,
				},
				Level:    tw.LevelHeader,
				IsSubRow: false,

				NormalizedWidths: t.streamWidths,
			})
			t.logger.Debug("streamAppendRow: Top border rendered.")
		}
	}

	shouldDrawHeaderRowSeparator := t.headerRendered && !t.firstRowRendered && cfg.Settings.Lines.ShowHeaderLine.Enabled()
	shouldDrawRowRowSeparator := t.firstRowRendered && cfg.Settings.Separators.BetweenRows.Enabled()

	firstCellForLog := ""
	if len(rawCellsSlice) > 0 {
		firstCellForLog = rawCellsSlice[0]
	}
	t.logger.Debugf("streamAppendRow: Separator Pre-Check for row starting with '%s': headerRendered=%v, firstRowRendered=%v, ShowHeaderLine=%v, BetweenRows=%v, lastRenderedPos=%q",
		firstCellForLog, t.headerRendered, t.firstRowRendered, cfg.Settings.Lines.ShowHeaderLine.Enabled(),
		cfg.Settings.Separators.BetweenRows.Enabled(), t.lastRenderedPosition)
	t.logger.Debugf("streamAppendRow: Separator Decision Flags for row starting with '%s': shouldDrawHeaderRowSeparator=%v, shouldDrawRowRowSeparator=%v",
		firstCellForLog, shouldDrawHeaderRowSeparator, shouldDrawRowRowSeparator)

	if (shouldDrawHeaderRowSeparator || shouldDrawRowRowSeparator) && t.lastRenderedPosition != tw.Position("separator") {
		t.logger.Debugf("streamAppendRow: Rendering separator line for row starting with '%s'.", firstCellForLog)
		prevCellsCtx := t.streamRenderedMergeState(t.lastRenderedLineContent, t.lastRenderedMergeState)
		var nextCellsCtx map[int]tw.CellContext
		if len(processedRowLines) > 0 {
			firstRowLineResp := t.streamBuildCellContexts(tw.Row, 0, 0, processedRowLines, rowMerges, t.config.Row)
			nextCellsCtx = firstRowLineResp.cells
		}
		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      prevCellsCtx,
				Previous:     nil,
				Next:         nextCellsCtx,
				Position:     tw.Row,
				Location:     tw.LocationMiddle,
			},
			Level:    tw.LevelBody,
			IsSubRow: false,

			NormalizedWidths: t.streamWidths,
		})
		t.lastRenderedPosition = tw.Position("separator")
		t.lastRenderedLineContent = nil
		t.lastRenderedMergeState = nil
		t.logger.Debug("streamAppendRow: Separator line rendered. Updated lastRenderedPosition to 'separator'.")
	} else {
		details := ""
		if !shouldDrawHeaderRowSeparator && !shouldDrawRowRowSeparator {
			details = "neither header/row nor row/row separator was flagged true"
		} else if t.lastRenderedPosition == tw.Position("separator") {
			details = "lastRenderedPosition is already 'separator'"
		} else {
			details = "an unexpected combination of conditions"
		}
		t.logger.Debugf("streamAppendRow: Separator not drawn for row '%s' because %s.", firstCellForLog, details)
	}

	if len(processedRowLines) == 0 {
		t.logger.Debugf("streamAppendRow: No processed row lines to render for row starting with '%s'.", firstCellForLog)
		if !t.firstRowRendered {
			t.firstRowRendered = true
			t.logger.Debugf("streamAppendRow: Marked first row rendered (empty content after processing).")
		}
		return nil
	}

	totalRowLines := len(processedRowLines)
	for i := 0; i < totalRowLines; i++ {
		resp := t.streamBuildCellContexts(tw.Row, 0, i, processedRowLines, rowMerges, t.config.Row)
		t.logger.Debug("streamAppendRow: Rendering row line %d/%d with location %v for row starting with '%s'.", i, totalRowLines, resp.location, firstCellForLog)
		f.Row(resp.cellsContent, tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Next:         resp.nextCells,
				Position:     tw.Row,
				Location:     resp.location,
			},
			Level:    tw.LevelBody,
			IsSubRow: i > 0,

			NormalizedWidths: t.streamWidths,
			HasFooter:        len(t.streamFooterLines) > 0,
		})
		t.lastRenderedLineContent = resp.cellsContent
		t.lastRenderedMergeState = make(map[int]tw.MergeState)
		for colIdx, cellCtx := range resp.cells {
			t.lastRenderedMergeState[colIdx] = cellCtx.Merge
		}
		t.lastRenderedPosition = tw.Row
	}

	if !t.firstRowRendered {
		t.firstRowRendered = true
		t.logger.Debug("streamAppendRow: Marked first row rendered (after processing content).")
	}

	t.logger.Debug("streamAppendRow: Row processing completed for row starting with '%s'.", firstCellForLog)
	return nil
}

// streamBuildCellContexts creates CellContext objects for a given line in streaming mode.
// Parameters:
// - position: The section being processed (Header, Row, Footer).
// - rowIdx: The row index within its section (always 0 for Header/Footer, row number for Row).
// - lineIdx: The line index within the processed lines for this block.
// - processedLines: All multi-lines for the current row/header/footer block.
// - sectionMerges: Merge states for the section or row (map[int]tw.MergeState).
// - sectionConfig: The CellConfig for this section (Header, Row, Footer).
// Returns a renderMergeResponse with Current, Previous, Next cells, cellsContent, and the determined Location.
func (t *Table) streamBuildCellContexts(
	position tw.Position,
	rowIdx, lineIdx int,
	processedLines [][]string,
	sectionMerges map[int]tw.MergeState,
	sectionConfig tw.CellConfig,
) renderMergeResponse {
	t.logger.Debug("streamBuildCellContexts: Building contexts for position=%s, rowIdx=%d, lineIdx=%d", position, rowIdx, lineIdx)
	resp := renderMergeResponse{
		cells:        make(map[int]tw.CellContext),
		prevCells:    nil,
		nextCells:    nil,
		cellsContent: make([]string, t.streamNumCols),
		location:     tw.LocationMiddle,
	}

	if t.streamWidths == nil || t.streamWidths.Len() == 0 || t.streamNumCols == 0 {
		t.logger.Warn("streamBuildCellContexts: streamWidths is not set or streamNumCols is 0. Returning empty contexts.")
		return resp
	}

	currentLineContent := make([]string, t.streamNumCols)
	if lineIdx >= 0 && lineIdx < len(processedLines) {
		currentLineContent = padLine(processedLines[lineIdx], t.streamNumCols)
	} else {
		t.logger.Warnf("streamBuildCellContexts: lineIdx %d out of bounds for processedLines (len %d) at position %s, rowIdx %d. Using empty line.", lineIdx, len(processedLines), position, rowIdx)
		for j := range currentLineContent {
			currentLineContent[j] = tw.Empty
		}
	}
	resp.cellsContent = currentLineContent

	colAligns := t.buildAligns(sectionConfig)
	colPadding := t.buildPadding(sectionConfig.Padding)
	resp.cells = t.buildCoreCellContexts(currentLineContent, sectionMerges, t.streamWidths, colAligns, colPadding, t.streamNumCols)

	if t.lastRenderedLineContent != nil && t.lastRenderedPosition.Validate() == nil {
		resp.prevCells = t.streamRenderedMergeState(t.lastRenderedLineContent, t.lastRenderedMergeState)
	}

	totalLinesInBlock := len(processedLines)
	if lineIdx < totalLinesInBlock-1 {
		resp.nextCells = make(map[int]tw.CellContext)
		nextLineContent := padLine(processedLines[lineIdx+1], t.streamNumCols)
		nextCells := t.buildCoreCellContexts(nextLineContent, sectionMerges, t.streamWidths, colAligns, colPadding, t.streamNumCols)
		for j := 0; j < t.streamNumCols; j++ {
			resp.nextCells[j] = nextCells[j]
		}
	}

	isFirstLineOfBlock := (lineIdx == 0)
	if isFirstLineOfBlock && (t.lastRenderedLineContent == nil || t.lastRenderedPosition != position) {
		resp.location = tw.LocationFirst
	}

	t.logger.Debug("streamBuildCellContexts: Position %s, Row %d, Line %d/%d. Location: %v. Prev Pos: %v. Has Prev: %v.",
		position, rowIdx, lineIdx, totalLinesInBlock, resp.location, t.lastRenderedPosition, t.lastRenderedLineContent != nil)
	return resp
}

// streamCalculateWidths determines the fixed column widths for streaming mode.
// It prioritizes widths from StreamConfig.Widths.PerColumn, then StreamConfig.Widths.Global,
// then derives from the provided sample data lines.
// It populates t.streamWidths and t.streamNumCols if they are currently empty.
// The sampleDataLines should be the *raw* input lines (e.g., []string for Header/Footer, or the first row's []string cells for Row).
// The paddingConfig should be the CellPadding config relevant to the sample data (Header/Row/Footer).
// Returns the determined number of columns.
// This function should only be called when t.streamWidths is currently empty.
func (t *Table) streamCalculateWidths(sampling []string, config tw.CellConfig) int {
	if t.streamWidths != nil && t.streamWidths.Len() > 0 {
		t.logger.Debug("streamCalculateWidths: Called when streaming widths are already set (%d columns). Reusing existing.", t.streamNumCols)
		return t.streamNumCols
	}

	t.logger.Debug("streamCalculateWidths: Calculating streaming widths. Sample data cells: %d. Using section config: %+v", len(sampling), config.Formatting)

	determinedNumCols := 0
	if t.config.Widths.PerColumn != nil && t.config.Widths.PerColumn.Len() > 0 {
		maxColIdx := -1
		t.config.Widths.PerColumn.Each(func(col, width int) {
			if col > maxColIdx {
				maxColIdx = col
			}
		})
		determinedNumCols = maxColIdx + 1
		t.logger.Debug("streamCalculateWidths: Determined numCols (%d) from StreamConfig.Widths.PerColumn", determinedNumCols)
	} else if len(sampling) > 0 {
		determinedNumCols = len(sampling)
		t.logger.Debug("streamCalculateWidths: Determined numCols (%d) from sample data length", determinedNumCols)
	} else {
		t.logger.Debug("streamCalculateWidths: Cannot determine numCols (no PerColumn config, no sample data)")
		t.streamNumCols = 0
		t.streamWidths = tw.NewMapper[int, int]()
		return 0
	}

	t.streamNumCols = determinedNumCols
	t.streamWidths = tw.NewMapper[int, int]()

	// Use padding and autowrap from the provided config
	paddingForWidthCalc := config.Padding
	autoWrapForWidthCalc := config.Formatting.AutoWrap

	if t.config.Widths.PerColumn != nil && t.config.Widths.PerColumn.Len() > 0 {
		t.logger.Debug("streamCalculateWidths: Using widths from StreamConfig.Widths.PerColumn")
		for i := 0; i < t.streamNumCols; i++ {
			width, ok := t.config.Widths.PerColumn.OK(i)
			if !ok {
				width = 0
			}
			if width > 0 && width < 1 {
				width = 1
			} else if width < 0 {
				width = 0
			}
			t.streamWidths.Set(i, width)
		}
	} else {
		// No PerColumn config, derive from sampling intelligently
		t.logger.Debug("streamCalculateWidths: Intelligently deriving widths from sample data content and padding.")
		tempRequiredWidths := tw.NewMapper[int, int]() // Widths from updateWidths (content + padding)
		if len(sampling) > 0 {
			// updateWidths calculates: DisplayWidth(content) + padLeft + padRight
			t.updateWidths(sampling, tempRequiredWidths, paddingForWidthCalc)
		}

		ellipsisWidthBuffer := 0
		if autoWrapForWidthCalc == tw.WrapTruncate {
			ellipsisWidthBuffer = twwidth.Width(tw.CharEllipsis)
		}
		varianceBuffer := 2 // Your suggested variance
		minTotalColWidth := tw.MinimumColumnWidth
		// Example: if t.config.Stream.MinAutoColumnWidth > 0 { minTotalColWidth = t.config.Stream.MinAutoColumnWidth }

		for i := 0; i < t.streamNumCols; i++ {
			// baseCellWidth (content_width + padding_width) comes from tempRequiredWidths.Get(i)
			// We need to deconstruct it to apply logic to content_width first.

			sampleContent := ""
			if i < len(sampling) {
				sampleContent = t.Trimmer(sampling[i])
			}
			sampleContentDisplayWidth := twwidth.Width(sampleContent)

			colPad := paddingForWidthCalc.Global
			if i < len(paddingForWidthCalc.PerColumn) && paddingForWidthCalc.PerColumn[i].Paddable() {
				colPad = paddingForWidthCalc.PerColumn[i]
			}
			currentPadLWidth := twwidth.Width(colPad.Left)
			currentPadRWidth := twwidth.Width(colPad.Right)
			currentTotalPaddingWidth := currentPadLWidth + currentPadRWidth

			// Start with the target content width logic
			targetContentWidth := sampleContentDisplayWidth
			if autoWrapForWidthCalc == tw.WrapTruncate {
				// If content is short, ensure it's at least wide enough for an ellipsis
				if targetContentWidth < ellipsisWidthBuffer {
					targetContentWidth = ellipsisWidthBuffer
				}
			}
			targetContentWidth += varianceBuffer // Add variance

			// Now calculate the total cell width based on this buffered content target + padding
			calculatedWidth := targetContentWidth + currentTotalPaddingWidth

			// Apply an absolute minimum total column width
			if calculatedWidth > 0 && calculatedWidth < minTotalColWidth {
				t.logger.Debug("streamCalculateWidths: Col %d, InitialCalcW=%d (ContentTarget=%d + Pad=%d) is less than MinTotalW=%d. Adjusting to MinTotalW.",
					i, calculatedWidth, targetContentWidth, currentTotalPaddingWidth, minTotalColWidth)
				calculatedWidth = minTotalColWidth
			} else if calculatedWidth <= 0 && sampleContentDisplayWidth > 0 { // If content exists but calc width is 0 (e.g. large negative variance)
				// Ensure at least min width or content + padding + buffers
				fallbackWidth := sampleContentDisplayWidth + currentTotalPaddingWidth
				if autoWrapForWidthCalc == tw.WrapTruncate {
					fallbackWidth += ellipsisWidthBuffer
				}
				fallbackWidth += varianceBuffer
				calculatedWidth = tw.Max(minTotalColWidth, fallbackWidth)
				if calculatedWidth <= 0 && (currentTotalPaddingWidth+1) > 0 { // last resort if all else is zero
					calculatedWidth = currentTotalPaddingWidth + 1
				} else if calculatedWidth <= 0 {
					calculatedWidth = 1 // absolute last resort
				}

				t.logger.Debug("streamCalculateWidths: Col %d, CalculatedW was <=0 despite content. Adjusted to %d.", i, calculatedWidth)
			} else if calculatedWidth <= 0 && sampleContentDisplayWidth == 0 {
				// Column is truly empty in sample and buffers didn't make it positive, or minTotalColWidth is 0.
				// Keep width 0 (it will be hidden by renderer if all content is empty for this col)
				// Or, if we want empty columns to have a minimum presence (even if just padding):
				// calculatedWidth = currentTotalPaddingWidth // This would make it just wide enough for padding
				// For now, let truly empty sample + no min width result in 0.
				calculatedWidth = 0 // Explicitly set to 0 if it ended up non-positive and no content
			}

			t.streamWidths.Set(i, calculatedWidth)
			t.logger.Debug("streamCalculateWidths: Col %d, SampleContentW=%d, PadW=%d, EllipsisBufIfTruncate=%d, VarianceBuf=%d -> FinalTotalColW=%d",
				i, sampleContentDisplayWidth, currentTotalPaddingWidth, ellipsisWidthBuffer, varianceBuffer, calculatedWidth)
		}
	}

	// Apply Global Constraint (if t.config.Stream.Widths.Global > 0)
	if t.config.Widths.Global > 0 && t.streamNumCols > 0 {
		t.logger.Debug("streamCalculateWidths: Applying global stream width constraint %d", t.config.Widths.Global)
		currentTotalColumnWidthsSum := 0
		t.streamWidths.Each(func(_, w int) {
			currentTotalColumnWidthsSum += w
		})

		separatorWidth := 0
		if t.renderer != nil {
			rendererConfig := t.renderer.Config()
			if rendererConfig.Settings.Separators.BetweenColumns.Enabled() {
				separatorWidth = twwidth.Width(rendererConfig.Symbols.Column())
			}
		} else {
			separatorWidth = 1 // Default if renderer not available yet
		}

		totalWidthIncludingSeparators := currentTotalColumnWidthsSum
		if t.streamNumCols > 1 {
			totalWidthIncludingSeparators += (t.streamNumCols - 1) * separatorWidth
		}

		if t.config.Widths.Global < totalWidthIncludingSeparators && totalWidthIncludingSeparators > 0 { // Added check for total > 0
			t.logger.Debug("streamCalculateWidths: Total calculated width (%d incl separators) exceeds global stream width (%d). Shrinking.", totalWidthIncludingSeparators, t.config.Widths.Global)

			// Target sum for column widths only (global limit - total separator width)
			targetSumForColumnWidths := t.config.Widths.Global
			if t.streamNumCols > 1 {
				targetSumForColumnWidths -= (t.streamNumCols - 1) * separatorWidth
			}
			if targetSumForColumnWidths < t.streamNumCols && t.streamNumCols > 0 { // Ensure at least 1 per column if possible
				targetSumForColumnWidths = t.streamNumCols
			} else if targetSumForColumnWidths < 0 {
				targetSumForColumnWidths = 0
			}

			scaleFactor := float64(targetSumForColumnWidths) / float64(currentTotalColumnWidthsSum)
			if currentTotalColumnWidthsSum <= 0 {
				scaleFactor = 0
			} // Avoid division by zero or negative scale

			adjustedSum := 0
			for i := 0; i < t.streamNumCols; i++ {
				originalColWidth := t.streamWidths.Get(i)
				if originalColWidth == 0 {
					continue
				} // Don't scale hidden columns

				scaledWidth := 0
				if scaleFactor > 0 {
					scaledWidth = int(math.Round(float64(originalColWidth) * scaleFactor))
				}

				if scaledWidth < 1 && originalColWidth > 0 { // Ensure at least 1 if original had width and scaling made it too small
					scaledWidth = 1
				} else if scaledWidth < 0 { // Should not happen with math.Round on positive*positive
					scaledWidth = 0
				}
				t.streamWidths.Set(i, scaledWidth)
				adjustedSum += scaledWidth
			}

			// Distribute rounding errors to meet targetSumForColumnWidths
			remainingSpace := targetSumForColumnWidths - adjustedSum
			t.logger.Debug("streamCalculateWidths: Scaling complete. TargetSum=%d, AchievedSum=%d, RemSpace=%d", targetSumForColumnWidths, adjustedSum, remainingSpace)
			// Distribute remainingSpace (positive or negative) among non-zero width columns
			if remainingSpace != 0 && t.streamNumCols > 0 {
				colsToAdjust := []int{}
				t.streamWidths.Each(func(col, w int) {
					if w > 0 { // Only consider columns that currently have width
						colsToAdjust = append(colsToAdjust, col)
					}
				})
				if len(colsToAdjust) > 0 {
					for i := 0; i < int(math.Abs(float64(remainingSpace))); i++ {
						colIdx := colsToAdjust[i%len(colsToAdjust)]
						currentColWidth := t.streamWidths.Get(colIdx)
						if remainingSpace > 0 {
							t.streamWidths.Set(colIdx, currentColWidth+1)
						} else if remainingSpace < 0 && currentColWidth > 1 { // Don't reduce below 1
							t.streamWidths.Set(colIdx, currentColWidth-1)
						}
					}
				}
			}
			t.logger.Debug("streamCalculateWidths: Widths after scaling and distribution: %v", t.streamWidths)
		} else {
			t.logger.Debug("streamCalculateWidths: Total calculated width (%d) fits global stream width (%d). No scaling needed.", totalWidthIncludingSeparators, t.config.Widths.Global)
		}
	}

	// Final sanitization
	t.streamWidths.Each(func(col, width int) {
		if width < 0 {
			t.streamWidths.Set(col, 0)
		}
	})

	t.logger.Debug("streamCalculateWidths: Final derived stream widths after all adjustments (%d columns): %v", t.streamNumCols, t.streamWidths)
	return t.streamNumCols
}

// streamRenderBottomBorder renders the bottom border of the table in streaming mode.
// It uses the fixed streamWidths and the last rendered content to create the border context.
// It assumes Start() has been called and t.hasPrinted is true.
// Returns an error if rendering fails.
func (t *Table) streamRenderBottomBorder() error {
	if t.streamWidths == nil || t.streamWidths.Len() == 0 {
		t.logger.Debug("streamRenderBottomBorder: No stream widths available, skipping bottom border.")
		return nil
	}

	cfg := t.renderer.Config()
	if !cfg.Borders.Bottom.Enabled() || !cfg.Settings.Lines.ShowBottom.Enabled() {
		t.logger.Debug("streamRenderBottomBorder: Bottom border disabled in config, skipping.")
		return nil
	}

	// The bottom border's "Current" context is the last rendered content line
	currentCells := make(map[int]tw.CellContext)
	if t.lastRenderedLineContent != nil {
		// Use a helper to convert last rendered state to cell contexts
		currentCells = t.streamRenderedMergeState(t.lastRenderedLineContent, t.lastRenderedMergeState)
	} else {
		// No content was ever rendered, but we might still want a bottom border if a top border was drawn.
		// Create empty cell contexts.
		for i := 0; i < t.streamNumCols; i++ {
			currentCells[i] = tw.CellContext{Width: t.streamWidths.Get(i)}
		}
		t.logger.Debug("streamRenderBottomBorder: No previous content line, creating empty context for bottom border.")
	}

	f := t.renderer
	f.Line(tw.Formatting{
		Row: tw.RowContext{
			Widths:       t.streamWidths,
			ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
			Current:      currentCells,           // Context of the line *above* the bottom border
			Previous:     nil,                    // No line before this, relative to the border itself (or use lastRendered's previous?)
			Next:         nil,                    // No line after the bottom border
			Position:     t.lastRenderedPosition, // Position of the content above the border (Row or Footer)
			Location:     tw.LocationEnd,         // This is the absolute end
		},
		Level:    tw.LevelFooter, // Bottom border is LevelFooter
		IsSubRow: false,

		NormalizedWidths: t.streamWidths,
	})
	t.logger.Debug("streamRenderBottomBorder: Bottom border rendered.")
	return nil
}

// streamRenderFooter renders the stored footer lines in streaming mode.
// It's called by Close(). It renders the Row/Footer separator line first.
// It assumes Start() has been called and t.hasPrinted is true.
// Returns an error if rendering fails.
func (t *Table) streamRenderFooter(processedFooterLines [][]string) error {
	t.logger.Debug("streamRenderFooter: Rendering %d processed footer lines.", len(processedFooterLines))

	if t.streamWidths == nil || t.streamWidths.Len() == 0 || t.streamNumCols == 0 {
		t.logger.Warn("streamRenderFooter: No stream widths or columns defined. Cannot render footer.")
		return errors.New("cannot render stream footer without defined column widths")
	}

	if len(processedFooterLines) == 0 {
		t.logger.Debug("streamRenderFooter: No footer lines to render.")
		return nil
	}

	f := t.renderer
	cfg := t.renderer.Config()

	//  Render Row/Footer or Header/Footer Separator Line
	// This separator is drawn if ShowFooterLine is enabled AND there was content before the footer.
	// The last rendered position (t.lastRenderedPosition) should be Row or Header or "separator".
	if (t.lastRenderedPosition == tw.Row || t.lastRenderedPosition == tw.Header || t.lastRenderedPosition == tw.Position("separator")) &&
		cfg.Settings.Lines.ShowFooterLine.Enabled() {

		t.logger.Debug("streamRenderFooter: Rendering Row/Footer or Header/Footer separator line.")

		// Previous context is the last line rendered before this footer
		prevCells := t.streamRenderedMergeState(t.lastRenderedLineContent, t.lastRenderedMergeState)

		// Next context is the first line of this footer
		var nextCells map[int]tw.CellContext = nil
		if len(processedFooterLines) > 0 {
			// Need merge states for the footer section.
			// Since footer is processed once and stored, detect merges on its raw input once.
			// This requires access to the *original* raw footer strings passed to Footer().
			// For simplicity now, assume no complex horizontal merges in footer for this separator line context.
			// A better approach: streamStoreFooter should also calculate and store footerMerges.
			// For now, create nextCells without specific merge info for the separator line.
			// Or, call prepareWithMerges on the *stored processed* lines, which might be okay for simple cases.
			// Let's pass nil for sectionMerges to streamBuildCellContexts for this specific Next context.
			// It will result in default (no-merge) states.

			// For now, let's build nextCells manually for the separator line context
			nextCells = make(map[int]tw.CellContext)
			firstFooterLineContent := padLine(processedFooterLines[0], t.streamNumCols)
			// Footer merges should be calculated in streamStoreFooter and stored if needed.
			// For now, assume no merges for this 'Next' context.
			for j := 0; j < t.streamNumCols; j++ {
				nextCells[j] = tw.CellContext{Data: firstFooterLineContent[j], Width: t.streamWidths.Get(j)}
			}
		}

		separatorLevel := tw.LevelFooter // Line before footer section is LevelFooter
		separatorPosition := tw.Footer   // Positioned relative to the footer it precedes
		separatorLocation := tw.LocationMiddle

		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      prevCells, // Context of line above separator
				Previous:     nil,       // No line before Current in this specific context
				Next:         nextCells, // Context of line below separator (first footer line)
				Position:     separatorPosition,
				Location:     separatorLocation,
			},
			Level:    separatorLevel,
			IsSubRow: false,

			NormalizedWidths: t.streamWidths,
		})
		t.lastRenderedPosition = tw.Position("separator") // Update state
		t.lastRenderedLineContent = nil
		t.lastRenderedMergeState = nil
		t.logger.Debug("streamRenderFooter: Footer separator line rendered.")
	}
	//  End Render Separator Line

	// Detect horizontal merges for the footer section based on its (assumed stored) raw input.
	// This is tricky because streamStoreFooter gets []string, but prepareWithMerges expects [][]string.
	// For simplicity, if complex merges are needed in footer, streamStoreFooter should
	// have received raw data, called prepareWithMerges, and stored those merges.
	// For now, assume no complex horizontal merges in footer or pass nil for sectionMerges.
	// Let's assume footerMerges were calculated and stored as `t.streamFooterMerges map[int]tw.MergeState`
	// by `streamStoreFooter`. For this example, we'll pass nil, meaning no merges.
	var footerMerges map[int]tw.MergeState = nil // Placeholder

	totalFooterLines := len(processedFooterLines)
	for i := 0; i < totalFooterLines; i++ {
		resp := t.streamBuildCellContexts(
			tw.Footer,
			0, // Row index within Footer (always 0)
			i, // Line index
			processedFooterLines,
			footerMerges, // Pass footer-specific merges if calculated and stored
			t.config.Footer,
		)

		// Special Location logic for the *very last line* of the table if this footer line is it.
		// This is complex because bottom border might follow.
		// Let streamBuildCellContexts handle LocationFirst/Middle for now.
		// streamRenderBottomBorder will handle the final LocationEnd for its line.
		// If this footer line is the last content and no bottom border, *it* should be LocationEnd.

		// If this is the last line of the last content block (footer), and no bottom border will be drawn,
		// its Location should be End.
		isLastLineOfTableContent := (i == totalFooterLines-1) &&
			(!cfg.Borders.Bottom.Enabled() || !cfg.Settings.Lines.ShowBottom.Enabled())
		if isLastLineOfTableContent {
			resp.location = tw.LocationEnd
			t.logger.Debug("streamRenderFooter: Setting LocationEnd for last footer line as no bottom border will follow.")
		}

		f.Footer([][]string{resp.cellsContent}, tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Next:         resp.nextCells, // Next is nil if last line of footer block
				Position:     tw.Footer,
				Location:     resp.location,
			},
			Level:    tw.LevelFooter,
			IsSubRow: (i > 0),

			NormalizedWidths: t.streamWidths,
		})

		t.lastRenderedLineContent = resp.cellsContent
		t.lastRenderedMergeState = make(map[int]tw.MergeState)
		for colIdx, cellCtx := range resp.cells {
			t.lastRenderedMergeState[colIdx] = cellCtx.Merge
		}
		t.lastRenderedPosition = tw.Footer
	}

	t.logger.Debug("streamRenderFooter: Footer content rendering completed.")
	return nil
}

// streamRenderHeader processes and renders the header section in streaming mode.
// It calculates/uses fixed stream widths, processes content, renders borders/lines,
// and updates streaming state.
// It assumes Start() has already been called and t.hasPrinted is true.
func (t *Table) streamRenderHeader(headers []string) error {
	t.logger.Debug("streamRenderHeader called with headers: %v", headers)

	if !t.config.Stream.Enable {
		return errors.New("streaming mode is disabled")
	}

	if t.headerRendered {
		t.logger.Warn("streamRenderHeader called but header already rendered. Ignoring.")
		return nil
	}

	if err := t.ensureStreamWidthsCalculated(headers, t.config.Header); err != nil {
		return err
	}

	_, headerMerges, _ := t.prepareWithMerges([][]string{headers}, t.config.Header, tw.Header)
	processedHeaderLines := t.prepareContent(headers, t.config.Header)
	t.logger.Debug("streamRenderHeader: Processed header lines: %d", len(processedHeaderLines))

	if t.streamNumCols > 0 {
		t.headerRendered = true
	}
	if len(processedHeaderLines) == 0 && t.streamNumCols == 0 {
		t.logger.Debug("streamRenderHeader: No header content and no columns determined.")
		return nil
	}

	f := t.renderer
	cfg := t.renderer.Config()

	if t.lastRenderedPosition == "" && cfg.Borders.Top.Enabled() && cfg.Settings.Lines.ShowTop.Enabled() {
		t.logger.Debug("streamRenderHeader: Rendering table top border.")
		var nextCellsCtx map[int]tw.CellContext
		if len(processedHeaderLines) > 0 {
			firstHeaderLineResp := t.streamBuildCellContexts(
				tw.Header, 0, 0, processedHeaderLines, headerMerges, t.config.Header,
			)
			nextCellsCtx = firstHeaderLineResp.cells
		}
		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Next:         nextCellsCtx,
				Position:     tw.Header,
				Location:     tw.LocationFirst,
			},
			Level:    tw.LevelHeader,
			IsSubRow: false,

			NormalizedWidths: t.streamWidths,
		})
		t.logger.Debug("streamRenderHeader: Top border rendered.")
	}

	hasTopPadding := t.config.Header.Padding.Global.Top != tw.Empty
	if hasTopPadding {
		resp := t.streamBuildCellContexts(tw.Header, 0, -1, nil, headerMerges, t.config.Header)
		resp.cellsContent = t.buildPaddingLineContents(t.config.Header.Padding.Global.Top, t.streamWidths, t.streamNumCols, headerMerges)
		resp.location = tw.LocationFirst
		t.logger.Debug("streamRenderHeader: Rendering header top padding line: %v (loc: %v)", resp.cellsContent, resp.location)
		f.Header([][]string{resp.cellsContent}, tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Next:         resp.nextCells,
				Position:     tw.Header,
				Location:     resp.location,
			},
			Level:    tw.LevelHeader,
			IsSubRow: true,

			NormalizedWidths: t.streamWidths,
		})
		t.lastRenderedLineContent = resp.cellsContent
		t.lastRenderedMergeState = make(map[int]tw.MergeState)
		for colIdx, cellCtx := range resp.cells {
			t.lastRenderedMergeState[colIdx] = cellCtx.Merge
		}
		t.lastRenderedPosition = tw.Header
	}

	totalHeaderLines := len(processedHeaderLines)
	for i := 0; i < totalHeaderLines; i++ {
		resp := t.streamBuildCellContexts(tw.Header, 0, i, processedHeaderLines, headerMerges, t.config.Header)
		t.logger.Debug("streamRenderHeader: Rendering header content line %d/%d with location %v", i, totalHeaderLines, resp.location)
		f.Header([][]string{resp.cellsContent}, tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Next:         resp.nextCells,
				Position:     tw.Header,
				Location:     resp.location,
			},
			Level:    tw.LevelHeader,
			IsSubRow: i > 0,

			NormalizedWidths: t.streamWidths,
		})
		t.lastRenderedLineContent = resp.cellsContent
		t.lastRenderedMergeState = make(map[int]tw.MergeState)
		for colIdx, cellCtx := range resp.cells {
			t.lastRenderedMergeState[colIdx] = cellCtx.Merge
		}
		t.lastRenderedPosition = tw.Header
	}

	hasBottomPadding := t.config.Header.Padding.Global.Bottom != tw.Empty
	if hasBottomPadding {
		resp := t.streamBuildCellContexts(tw.Header, 0, totalHeaderLines, nil, headerMerges, t.config.Header)
		resp.cellsContent = t.buildPaddingLineContents(t.config.Header.Padding.Global.Bottom, t.streamWidths, t.streamNumCols, headerMerges)
		resp.location = tw.LocationEnd
		t.logger.Debug("streamRenderHeader: Rendering header bottom padding line: %v (loc: %v)", resp.cellsContent, resp.location)
		f.Header([][]string{resp.cellsContent}, tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Next:         resp.nextCells,
				Position:     tw.Header,
				Location:     resp.location,
			},
			Level:    tw.LevelHeader,
			IsSubRow: true,

			NormalizedWidths: t.streamWidths,
		})
		t.lastRenderedLineContent = resp.cellsContent
		t.lastRenderedMergeState = make(map[int]tw.MergeState)
		for colIdx, cellCtx := range resp.cells {
			t.lastRenderedMergeState[colIdx] = cellCtx.Merge
		}
		t.lastRenderedPosition = tw.Header
	}

	if cfg.Settings.Lines.ShowHeaderLine.Enabled() && (t.firstRowRendered || len(t.streamFooterLines) > 0) {
		t.logger.Debug("streamRenderHeader: Rendering header separator line.")
		resp := t.streamBuildCellContexts(tw.Header, 0, totalHeaderLines-1, processedHeaderLines, headerMerges, t.config.Header)
		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:       t.streamWidths,
				ColMaxWidths: tw.CellWidth{PerColumn: t.streamWidths},
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Next:         nil,
				Position:     tw.Header,
				Location:     tw.LocationMiddle,
			},
			Level:    tw.LevelBody,
			IsSubRow: false,

			NormalizedWidths: t.streamWidths,
		})
		t.lastRenderedPosition = tw.Position("separator")
		t.lastRenderedLineContent = nil
		t.lastRenderedMergeState = nil
	}

	t.logger.Debug("streamRenderHeader: Header content rendering completed.")
	return nil
}

// streamRenderedMergeState converts the stored last rendered line content
// and its merge states into a map of CellContext, suitable for providing
// context (e.g., "Current" or "Previous") to the renderer.
// It uses the fixed streamWidths.
func (t *Table) streamRenderedMergeState(
	lineContent []string,
	lineMergeStates map[int]tw.MergeState,
) map[int]tw.CellContext {
	cells := make(map[int]tw.CellContext)
	if t.streamWidths == nil || t.streamWidths.Len() == 0 || t.streamNumCols == 0 {
		t.logger.Warn("streamRenderedMergeState: streamWidths not set or streamNumCols is 0. Returning empty cell contexts.")
		return cells
	}

	// Ensure lineContent is padded to streamNumCols if it's not nil
	var paddedLineContent []string
	if lineContent != nil {
		paddedLineContent = padLine(lineContent, t.streamNumCols)
	} else {
		// If lineContent is nil (e.g. after a separator), create an empty padded line
		paddedLineContent = make([]string, t.streamNumCols)
		for i := range paddedLineContent {
			paddedLineContent[i] = tw.Empty
		}
	}

	for j := 0; j < t.streamNumCols; j++ {
		cellData := paddedLineContent[j]
		colWidth := t.streamWidths.Get(j)
		mergeState := tw.MergeState{} // Default to no merge

		if lineMergeStates != nil {
			if state, ok := lineMergeStates[j]; ok {
				mergeState = state
			}
		}

		// For context purposes (like Previous or Current for a border line),
		// Align and Padding are often less critical than Data, Width, and Merge.
		// We can use default/empty Align and Padding here.
		cells[j] = tw.CellContext{
			Data:    cellData,
			Align:   tw.AlignDefault, // Or tw.AlignNone if preferred for context-only cells
			Padding: tw.Padding{},    // Empty padding
			Width:   colWidth,
			Merge:   mergeState,
		}
	}
	return cells
}

// streamStoreFooter processes the footer content and stores it for later rendering by Close()
// in streaming mode. It ensures stream widths are calculated if not already set.
func (t *Table) streamStoreFooter(footers []string) error {
	t.logger.Debug("streamStoreFooter called with footers: %v", footers)

	if !t.config.Stream.Enable {
		return errors.New("streaming mode is disabled")
	}

	if len(footers) == 0 {
		t.logger.Debug("streamStoreFooter: Empty footer cells, storing empty footer lines.")
		t.streamFooterLines = [][]string{}
		return nil
	}

	if err := t.ensureStreamWidthsCalculated(footers, t.config.Footer); err != nil {
		t.logger.Warnf("streamStoreFooter: Failed to determine column count from footer data: %v", err)
		t.streamFooterLines = [][]string{}
		return nil
	}

	if t.streamNumCols > 0 && len(footers) != t.streamNumCols {
		t.logger.Warnf("streamStoreFooter: Input footer column count (%d) does not match fixed stream column count (%d). Padding/Truncating input footers.", len(footers), t.streamNumCols)
		if len(footers) < t.streamNumCols {
			paddedFooters := make([]string, t.streamNumCols)
			copy(paddedFooters, footers)
			for i := len(footers); i < t.streamNumCols; i++ {
				paddedFooters[i] = tw.Empty
			}
			footers = paddedFooters
		} else {
			footers = footers[:t.streamNumCols]
		}
	}

	if t.streamNumCols == 0 {
		t.logger.Warn("streamStoreFooter: streamNumCols is 0, cannot process/store footer lines meaningfully.")
		t.streamFooterLines = [][]string{}
		return nil
	}

	t.streamFooterLines = t.prepareContent(footers, t.config.Footer)
	t.logger.Debug("streamStoreFooter: Processed and stored footer lines: %d lines. Content: %v", len(t.streamFooterLines), t.streamFooterLines)

	return nil
}
