package tablewriter

import (
	"database/sql"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/olekukonko/errors"
	"github.com/olekukonko/tablewriter/pkg/twwidth"
	"github.com/olekukonko/tablewriter/tw"
)

// applyHierarchicalMerges applies hierarchical merges to row content.
// Parameters ctx and mctx hold rendering and merge state.
// No return value.
func (t *Table) applyHierarchicalMerges(ctx *renderContext, mctx *mergeContext) {
	// First, ensure we should even run this logic.
	// Check both the new CellMerging struct and the deprecated Formatting field.
	mergeMode := t.config.Row.Merging.Mode
	if mergeMode == 0 {
		mergeMode = t.config.Row.Formatting.MergeMode
	}
	if !(mergeMode&tw.MergeHierarchical != 0) {
		return
	}

	mergeColumnMapper := t.config.Row.Merging.ByColumnIndex
	if mergeColumnMapper != nil {
		ctx.logger.Debugf("Applying hierarchical merges ONLY to specified columns: %v", mergeColumnMapper.Keys())
	} else {
		ctx.logger.Debug("Applying hierarchical merges (left-to-right vertical flow - snapshot comparison)")
	}

	if len(ctx.rowLines) <= 1 {
		ctx.logger.Debug("Skipping hierarchical merges - less than 2 rows")
		return
	}
	numCols := ctx.numCols

	originalRowLines := make([][][]string, len(ctx.rowLines))
	for i, row := range ctx.rowLines {
		originalRowLines[i] = make([][]string, len(row))
		for j, line := range row {
			originalRowLines[i][j] = make([]string, len(line))
			copy(originalRowLines[i][j], line)
		}
	}
	ctx.logger.Debug("Created snapshot of original row data for hierarchical merge comparison.")

	hMergeStartRow := make(map[int]int)

	for r := 1; r < len(ctx.rowLines); r++ {
		leftCellContinuedHierarchical := false

		for c := 0; c < numCols; c++ {
			// If a column map is specified, skip columns that are not in it.
			if mergeColumnMapper != nil && !mergeColumnMapper.Has(c) {
				leftCellContinuedHierarchical = false // Reset hierarchy tracking
				continue
			}

			if mctx.rowMerges[r] == nil {
				mctx.rowMerges[r] = make(map[int]tw.MergeState)
			}
			if mctx.rowMerges[r-1] == nil {
				mctx.rowMerges[r-1] = make(map[int]tw.MergeState)
			}

			canCompare := r > 0 &&
				len(originalRowLines[r]) > 0 &&
				len(originalRowLines[r-1]) > 0

			if !canCompare {
				currentState := mctx.rowMerges[r][c]
				currentState.Hierarchical = tw.MergeStateOption{}
				mctx.rowMerges[r][c] = currentState
				ctx.logger.Debugf("HCompare Skipped: r=%d, c=%d - Insufficient data in snapshot", r, c)
				leftCellContinuedHierarchical = false
				continue
			}

			// Join all lines of the cell for comparison
			var currentVal, aboveVal string
			for _, line := range originalRowLines[r] {
				if c < len(line) {
					currentVal += line[c]
				}
			}
			for _, line := range originalRowLines[r-1] {
				if c < len(line) {
					aboveVal += line[c]
				}
			}

			currentVal = t.Trimmer(currentVal)
			aboveVal = t.Trimmer(aboveVal)

			currentState := mctx.rowMerges[r][c]
			prevStateAbove := mctx.rowMerges[r-1][c]

			valuesMatch := currentVal == aboveVal && currentVal != "" && currentVal != "-"
			hierarchyAllowed := c == 0 || leftCellContinuedHierarchical
			shouldContinue := valuesMatch && hierarchyAllowed

			ctx.logger.Debugf("HCompare: r=%d, c=%d; current='%s', above='%s'; match=%v; leftCont=%v; shouldCont=%v",
				r, c, currentVal, aboveVal, valuesMatch, leftCellContinuedHierarchical, shouldContinue)

			if shouldContinue {
				currentState.Hierarchical.Present = true
				currentState.Hierarchical.Start = false

				if prevStateAbove.Hierarchical.Present && !prevStateAbove.Hierarchical.End {
					startRow, ok := hMergeStartRow[c]
					if !ok {
						ctx.logger.Debugf("Hierarchical merge WARNING: Recovering lost start row at r=%d, c=%d. Assuming r-1 was start.", r, c)
						startRow = r - 1
						hMergeStartRow[c] = startRow
						startState := mctx.rowMerges[startRow][c]
						startState.Hierarchical.Present = true
						startState.Hierarchical.Start = true
						startState.Hierarchical.End = false
						mctx.rowMerges[startRow][c] = startState
					}
					ctx.logger.Debugf("Hierarchical merge CONTINUED row %d, col %d. Block previously started row %d", r, c, startRow)
				} else {
					startRow := r - 1
					hMergeStartRow[c] = startRow
					startState := mctx.rowMerges[startRow][c]
					startState.Hierarchical.Present = true
					startState.Hierarchical.Start = true
					startState.Hierarchical.End = false
					mctx.rowMerges[startRow][c] = startState
					ctx.logger.Debugf("Hierarchical merge START detected for block ending at or after row %d, col %d (started at row %d)", r, c, startRow)
				}

				for lineIdx := range ctx.rowLines[r] {
					if c < len(ctx.rowLines[r][lineIdx]) {
						ctx.rowLines[r][lineIdx][c] = tw.Empty
					}
				}

				leftCellContinuedHierarchical = true
			} else {
				currentState.Hierarchical = tw.MergeStateOption{}

				if startRow, ok := hMergeStartRow[c]; ok {
					t.finalizeHierarchicalMergeBlock(ctx, mctx, c, startRow, r-1)
					delete(hMergeStartRow, c)
				}

				leftCellContinuedHierarchical = false
			}

			mctx.rowMerges[r][c] = currentState
		}
	}

	lastRowIdx := len(ctx.rowLines) - 1
	if lastRowIdx >= 0 {
		for c, startRow := range hMergeStartRow {
			t.finalizeHierarchicalMergeBlock(ctx, mctx, c, startRow, lastRowIdx)
		}
	}
	ctx.logger.Debug("Hierarchical merge processing completed")
}

// applyHorizontalMerges adjusts column widths for horizontal merges.
// Parameters include position, ctx for rendering, and mergeStates for merges.
// No return value.
func (t *Table) applyHorizontalMerges(position tw.Position, ctx *renderContext, mergeStates map[int]tw.MergeState) {
	if mergeStates == nil {
		t.logger.Debugf("applyHorizontalMerges: Skipping %s - no merge states", position)
		return
	}
	t.logger.Debugf("applyHorizontalMerges: Applying HMerge width recalc for %s", position)

	numCols := ctx.numCols
	targetWidthsMap := ctx.widths[position]
	originalNormalizedWidths := tw.NewMapper[int, int]()
	for i := 0; i < numCols; i++ {
		originalNormalizedWidths.Set(i, targetWidthsMap.Get(i))
	}

	separatorWidth := 0
	if t.renderer != nil {
		rendererConfig := t.renderer.Config()
		if rendererConfig.Settings.Separators.BetweenColumns.Enabled() {
			separatorWidth = twwidth.Width(rendererConfig.Symbols.Column())
		}
	}

	processedCols := make(map[int]bool)

	for col := 0; col < numCols; col++ {
		if processedCols[col] {
			continue
		}

		state, exists := mergeStates[col]
		if !exists {
			continue
		}

		if state.Horizontal.Present && state.Horizontal.Start {
			totalWidth := 0
			span := state.Horizontal.Span
			t.logger.Debugf("  -> HMerge detected: startCol=%d, span=%d, separatorWidth=%d", col, span, separatorWidth)

			for i := 0; i < span && (col+i) < numCols; i++ {
				currentColIndex := col + i
				normalizedWidth := originalNormalizedWidths.Get(currentColIndex)
				totalWidth += normalizedWidth
				t.logger.Debugf("      -> col %d: adding normalized width %d", currentColIndex, normalizedWidth)

				if i > 0 && separatorWidth > 0 {
					totalWidth += separatorWidth
					t.logger.Debugf("      -> col %d: adding separator width %d", currentColIndex, separatorWidth)
				}
			}

			targetWidthsMap.Set(col, totalWidth)
			t.logger.Debugf("  -> Set %s col %d width to %d (merged)", position, col, totalWidth)
			processedCols[col] = true

			for i := 1; i < span && (col+i) < numCols; i++ {
				targetWidthsMap.Set(col+i, 0)
				t.logger.Debugf("  -> Set %s col %d width to 0 (part of merge)", position, col+i)
				processedCols[col+i] = true
			}
		}
	}
	ctx.logger.Debugf("applyHorizontalMerges: Final widths for %s: %v", position, targetWidthsMap)
}

// applyVerticalMerges applies vertical merges to row content.
// Parameters ctx and mctx hold rendering and merge state.
// No return value.
func (t *Table) applyVerticalMerges(ctx *renderContext, mctx *mergeContext) {
	// First, ensure we should even run this logic.
	// Check both the new CellMerging struct and the deprecated Formatting field.
	mergeMode := t.config.Row.Merging.Mode
	if mergeMode == 0 {
		mergeMode = t.config.Row.Formatting.MergeMode
	}
	if !(mergeMode&tw.MergeVertical != 0) {
		return
	}

	mergeColumnMapper := t.config.Row.Merging.ByColumnIndex
	if mergeColumnMapper != nil {
		ctx.logger.Debugf("Applying vertical merges ONLY to specified columns: %v", mergeColumnMapper.Keys())
	} else {
		ctx.logger.Debugf("Applying vertical merges across %d rows", len(ctx.rowLines))
	}

	numCols := ctx.numCols
	mergeStartRow := make(map[int]int)
	mergeStartContent := make(map[int]string)

	for i := 0; i < len(ctx.rowLines); i++ {
		if i >= len(mctx.rowMerges) {
			newRowMerges := make([]map[int]tw.MergeState, i+1)
			copy(newRowMerges, mctx.rowMerges)
			for k := len(mctx.rowMerges); k <= i; k++ {
				newRowMerges[k] = make(map[int]tw.MergeState)
			}
			mctx.rowMerges = newRowMerges
			ctx.logger.Debugf("Extended rowMerges to index %d", i)
		} else if mctx.rowMerges[i] == nil {
			mctx.rowMerges[i] = make(map[int]tw.MergeState)
		}

		if len(ctx.rowLines[i]) == 0 {
			continue
		}
		currentLineContent := ctx.rowLines[i]

		for col := 0; col < numCols; col++ {
			// If a column map is specified, skip columns that are not in it.
			if mergeColumnMapper != nil && !mergeColumnMapper.Has(col) {
				continue
			}

			// Join all lines of the cell to compare full content
			var currentVal strings.Builder
			for _, line := range currentLineContent {
				if col < len(line) {
					currentVal.WriteString(line[col])
				}
			}

			currentValStr := t.Trimmer(currentVal.String())

			startRow, ongoingMerge := mergeStartRow[col]
			startContent := mergeStartContent[col]
			mergeState := mctx.rowMerges[i][col]

			if ongoingMerge && currentValStr == startContent && currentValStr != "" {
				mergeState.Vertical = tw.MergeStateOption{
					Present: true,
					Span:    0,
					Start:   false,
					End:     false,
				}
				mctx.rowMerges[i][col] = mergeState
				for lineIdx := range ctx.rowLines[i] {
					if col < len(ctx.rowLines[i][lineIdx]) {
						ctx.rowLines[i][lineIdx][col] = tw.Empty
					}
				}
				ctx.logger.Debugf("Vertical merge continued at row %d, col %d", i, col)
			} else {
				if ongoingMerge {
					endedRow := i - 1
					if endedRow >= 0 && endedRow >= startRow {
						startState := mctx.rowMerges[startRow][col]
						startState.Vertical.Span = (endedRow - startRow) + 1
						startState.Vertical.End = startState.Vertical.Span == 1
						mctx.rowMerges[startRow][col] = startState

						endState := mctx.rowMerges[endedRow][col]
						endState.Vertical.End = true
						endState.Vertical.Span = startState.Vertical.Span
						mctx.rowMerges[endedRow][col] = endState
						ctx.logger.Debugf("Vertical merge ended at row %d, col %d, span %d", endedRow, col, startState.Vertical.Span)
					}
					delete(mergeStartRow, col)
					delete(mergeStartContent, col)
				}

				if currentValStr != "" {
					mergeState.Vertical = tw.MergeStateOption{
						Present: true,
						Span:    1,
						Start:   true,
						End:     false,
					}
					mctx.rowMerges[i][col] = mergeState
					mergeStartRow[col] = i
					mergeStartContent[col] = currentValStr
					ctx.logger.Debugf("Vertical merge started at row %d, col %d", i, col)
				} else if !mergeState.Horizontal.Present {
					mergeState.Vertical = tw.MergeStateOption{}
					mctx.rowMerges[i][col] = mergeState
				}
			}
		}
	}

	lastRowIdx := len(ctx.rowLines) - 1
	if lastRowIdx >= 0 {
		for col, startRow := range mergeStartRow {
			startState := mctx.rowMerges[startRow][col]
			finalSpan := (lastRowIdx - startRow) + 1
			startState.Vertical.Span = finalSpan
			startState.Vertical.End = finalSpan == 1
			mctx.rowMerges[startRow][col] = startState

			endState := mctx.rowMerges[lastRowIdx][col]
			endState.Vertical.Present = true
			endState.Vertical.End = true
			endState.Vertical.Span = finalSpan
			if startRow != lastRowIdx {
				endState.Vertical.Start = false
			}
			mctx.rowMerges[lastRowIdx][col] = endState
			ctx.logger.Debugf("Vertical merge finalized at row %d, col %d, span %d", lastRowIdx, col, finalSpan)
		}
	}
	ctx.logger.Debug("Vertical merges completed")
}

// buildAdjacentCells constructs cell contexts for adjacent lines.
// Parameters include ctx, mctx, hctx, and direction (-1 for prev, +1 for next).
// Returns a map of column indices to CellContext for the adjacent line.
func (t *Table) buildAdjacentCells(ctx *renderContext, mctx *mergeContext, hctx *helperContext, direction int) map[int]tw.CellContext {
	adjCells := make(map[int]tw.CellContext)
	var adjLine []string
	var adjMerges map[int]tw.MergeState
	found := false
	adjPosition := hctx.position // Assume adjacent line is in the same section initially

	switch hctx.position {
	case tw.Header:
		targetLineIdx := hctx.lineIdx + direction
		if direction < 0 { // Previous
			if targetLineIdx >= 0 && targetLineIdx < len(ctx.headerLines) {
				adjLine = ctx.headerLines[targetLineIdx]
				adjMerges = mctx.headerMerges
				found = true
			}
		} else { // Next
			if targetLineIdx < len(ctx.headerLines) {
				adjLine = ctx.headerLines[targetLineIdx]
				adjMerges = mctx.headerMerges
				found = true
			} else if len(ctx.rowLines) > 0 && len(ctx.rowLines[0]) > 0 && len(mctx.rowMerges) > 0 {
				adjLine = ctx.rowLines[0][0]
				adjMerges = mctx.rowMerges[0]
				adjPosition = tw.Row
				found = true
			} else if len(ctx.footerLines) > 0 {
				adjLine = ctx.footerLines[0]
				adjMerges = mctx.footerMerges
				adjPosition = tw.Footer
				found = true
			}
		}
	case tw.Row:
		targetLineIdx := hctx.lineIdx + direction
		if hctx.rowIdx < 0 || hctx.rowIdx >= len(ctx.rowLines) || hctx.rowIdx >= len(mctx.rowMerges) {
			t.logger.Debugf("Warning: Invalid row index %d in buildAdjacentCells", hctx.rowIdx)
			return nil
		}
		currentRowLines := ctx.rowLines[hctx.rowIdx]
		currentMerges := mctx.rowMerges[hctx.rowIdx]

		if direction < 0 { // Previous
			if targetLineIdx >= 0 && targetLineIdx < len(currentRowLines) {
				adjLine = currentRowLines[targetLineIdx]
				adjMerges = currentMerges
				found = true
			} else if targetLineIdx < 0 {
				targetRowIdx := hctx.rowIdx - 1
				if targetRowIdx >= 0 && targetRowIdx < len(ctx.rowLines) && targetRowIdx < len(mctx.rowMerges) {
					prevRowLines := ctx.rowLines[targetRowIdx]
					if len(prevRowLines) > 0 {
						adjLine = prevRowLines[len(prevRowLines)-1]
						adjMerges = mctx.rowMerges[targetRowIdx]
						found = true
					}
				} else if len(ctx.headerLines) > 0 {
					adjLine = ctx.headerLines[len(ctx.headerLines)-1]
					adjMerges = mctx.headerMerges
					adjPosition = tw.Header
					found = true
				}
			}
		} else { // Next
			if targetLineIdx >= 0 && targetLineIdx < len(currentRowLines) {
				adjLine = currentRowLines[targetLineIdx]
				adjMerges = currentMerges
				found = true
			} else if targetLineIdx >= len(currentRowLines) {
				targetRowIdx := hctx.rowIdx + 1
				if targetRowIdx < len(ctx.rowLines) && targetRowIdx < len(mctx.rowMerges) && len(ctx.rowLines[targetRowIdx]) > 0 {
					adjLine = ctx.rowLines[targetRowIdx][0]
					adjMerges = mctx.rowMerges[targetRowIdx]
					found = true
				} else if len(ctx.footerLines) > 0 {
					adjLine = ctx.footerLines[0]
					adjMerges = mctx.footerMerges
					adjPosition = tw.Footer
					found = true
				}
			}
		}
	case tw.Footer:
		targetLineIdx := hctx.lineIdx + direction
		if direction < 0 { // Previous
			if targetLineIdx >= 0 && targetLineIdx < len(ctx.footerLines) {
				adjLine = ctx.footerLines[targetLineIdx]
				adjMerges = mctx.footerMerges
				found = true
			} else if targetLineIdx < 0 {
				if len(ctx.rowLines) > 0 {
					lastRowIdx := len(ctx.rowLines) - 1
					if lastRowIdx < len(mctx.rowMerges) && len(ctx.rowLines[lastRowIdx]) > 0 {
						lastRowLines := ctx.rowLines[lastRowIdx]
						adjLine = lastRowLines[len(lastRowLines)-1]
						adjMerges = mctx.rowMerges[lastRowIdx]
						adjPosition = tw.Row
						found = true
					}
				} else if len(ctx.headerLines) > 0 {
					adjLine = ctx.headerLines[len(ctx.headerLines)-1]
					adjMerges = mctx.headerMerges
					adjPosition = tw.Header
					found = true
				}
			}
		} else { // Next
			if targetLineIdx >= 0 && targetLineIdx < len(ctx.footerLines) {
				adjLine = ctx.footerLines[targetLineIdx]
				adjMerges = mctx.footerMerges
				found = true
			}
		}
	}

	if !found {
		return nil
	}

	if adjMerges == nil {
		adjMerges = make(map[int]tw.MergeState)
		t.logger.Debugf("Warning: adjMerges was nil in buildAdjacentCells despite found=true")
	}

	paddedAdjLine := padLine(adjLine, ctx.numCols)

	for j := 0; j < ctx.numCols; j++ {
		mergeState := adjMerges[j]
		cellData := paddedAdjLine[j]
		finalAdjColWidth := ctx.widths[adjPosition].Get(j)

		adjCells[j] = tw.CellContext{
			Data:  cellData,
			Merge: mergeState,
			Width: finalAdjColWidth,
		}
	}
	return adjCells
}

// buildCellContexts creates CellContext objects for a given line in batch mode.
// Parameters include ctx, mctx, hctx, aligns, and padding for rendering.
// Returns a renderMergeResponse with current, previous, and next cell contexts.
func (t *Table) buildCellContexts(ctx *renderContext, mctx *mergeContext, hctx *helperContext, aligns map[int]tw.Align, padding map[int]tw.Padding) renderMergeResponse {
	t.logger.Debugf("buildCellContexts: Building contexts for position=%s, rowIdx=%d, lineIdx=%d", hctx.position, hctx.rowIdx, hctx.lineIdx)
	var merges map[int]tw.MergeState
	switch hctx.position {
	case tw.Header:
		merges = mctx.headerMerges
	case tw.Row:
		if hctx.rowIdx >= 0 && hctx.rowIdx < len(mctx.rowMerges) && mctx.rowMerges[hctx.rowIdx] != nil {
			merges = mctx.rowMerges[hctx.rowIdx]
		} else {
			merges = make(map[int]tw.MergeState)
			t.logger.Warnf("buildCellContexts: Invalid row index %d or nil merges for row", hctx.rowIdx)
		}
	case tw.Footer:
		merges = mctx.footerMerges
	default:
		merges = make(map[int]tw.MergeState)
		t.logger.Warnf("buildCellContexts: Invalid position '%s'", hctx.position)
	}

	cells := t.buildCoreCellContexts(hctx.line, merges, ctx.widths[hctx.position], aligns, padding, ctx.numCols)
	return renderMergeResponse{
		cells:     cells,
		prevCells: t.buildAdjacentCells(ctx, mctx, hctx, -1),
		nextCells: t.buildAdjacentCells(ctx, mctx, hctx, +1),
		location:  hctx.location,
	}
}

// buildCoreCellContexts constructs CellContext objects for a single line, shared between batch and streaming modes.
// Parameters:
// - line: The content of the current line (padded to numCols).
// - merges: Merge states for the line's columns (map[int]tw.MergeState).
// - widths: Column widths (tw.Mapper[int, int]).
// - aligns: Column alignments (map[int]tw.Align).
// - padding: Column padding settings (map[int]tw.Padding).
// - numCols: Number of columns to process.
// Returns a map of column indices to CellContext for the current line.
func (t *Table) buildCoreCellContexts(line []string, merges map[int]tw.MergeState, widths tw.Mapper[int, int], aligns map[int]tw.Align, padding map[int]tw.Padding, numCols int) map[int]tw.CellContext {
	cells := make(map[int]tw.CellContext)
	paddedLine := padLine(line, numCols)
	for j := 0; j < numCols; j++ {
		cellData := paddedLine[j]
		mergeState := tw.MergeState{}
		if merges != nil {
			if state, ok := merges[j]; ok {
				mergeState = state
			}
		}
		cells[j] = tw.CellContext{
			Data:    cellData,
			Align:   aligns[j],
			Padding: padding[j],
			Width:   widths.Get(j),
			Merge:   mergeState,
		}
	}
	t.logger.Debugf("buildCoreCellContexts: Built cell contexts for %d columns", numCols)
	return cells
}

// buildPaddingLineContents constructs a padding line for a given section, respecting column widths and horizontal merges.
// It generates a []string where each element is the padding content for a column, using the specified padChar.
func (t *Table) buildPaddingLineContents(padChar string, widths tw.Mapper[int, int], numCols int, merges map[int]tw.MergeState) []string {
	line := make([]string, numCols)
	padWidth := max(twwidth.Width(padChar), 1)
	for j := 0; j < numCols; j++ {
		mergeState := tw.MergeState{}
		if merges != nil {
			if state, ok := merges[j]; ok {
				mergeState = state
			}
		}
		if mergeState.Horizontal.Present && !mergeState.Horizontal.Start {
			line[j] = tw.Empty
			continue
		}
		colWd := widths.Get(j)
		repeatCount := 0
		if colWd > 0 && padWidth > 0 {
			repeatCount = colWd / padWidth
		}
		if colWd > 0 && repeatCount < 1 {
			repeatCount = 1
		}
		content := strings.Repeat(padChar, repeatCount)
		line[j] = content
	}
	if t.logger.Enabled() {
		t.logger.Debugf("Built padding line with char '%s' for %d columns", padChar, numCols)
	}
	return line
}

// calculateAndNormalizeWidths computes and normalizes column widths.
// Parameter ctx holds rendering state with width maps.
// Returns an error if width calculation fails.
func (t *Table) calculateAndNormalizeWidths(ctx *renderContext) error {
	ctx.logger.Debugf("calculateAndNormalizeWidths: Computing and normalizing widths for %d columns. Compact: %v",
		ctx.numCols, t.config.Behavior.Compact.Merge.Enabled())

	// Compute content-based widths for each section
	for _, lines := range ctx.headerLines {
		t.updateWidths(lines, t.headerWidths, t.config.Header.Padding)
	}
	rowWidthCache := make([]tw.Mapper[int, int], len(ctx.rowLines))
	for i, row := range ctx.rowLines {
		rowWidthCache[i] = tw.NewMapper[int, int]()
		for _, line := range row {
			t.updateWidths(line, rowWidthCache[i], t.config.Row.Padding)
			for col, width := range rowWidthCache[i] {
				currentMax, _ := t.rowWidths.OK(col)
				if width > currentMax {
					t.rowWidths.Set(col, width)
				}
			}
		}
	}
	for _, lines := range ctx.footerLines {
		t.updateWidths(lines, t.footerWidths, t.config.Footer.Padding)
	}
	ctx.logger.Debugf("Content-based widths: header=%v, row=%v, footer=%v", t.headerWidths, t.rowWidths, t.footerWidths)

	// Analyze header merges for optimization
	var headerMergeSpans map[int]int
	if t.config.Header.Formatting.MergeMode&tw.MergeHorizontal != 0 && len(ctx.headerLines) > 0 {
		headerMergeSpans = make(map[int]int)
		visitedCols := make(map[int]bool)
		firstHeaderLine := ctx.headerLines[0]
		if len(firstHeaderLine) > 0 {
			for i := 0; i < len(firstHeaderLine); {
				if visitedCols[i] {
					i++
					continue
				}
				var currentLogicalCellContentBuilder strings.Builder
				for _, hLine := range ctx.headerLines {
					if i < len(hLine) {
						currentLogicalCellContentBuilder.WriteString(hLine[i])
					}
				}
				currentHeaderCellContent := t.Trimmer(currentLogicalCellContentBuilder.String())
				span := 1
				for j := i + 1; j < len(firstHeaderLine); j++ {
					var nextLogicalCellContentBuilder strings.Builder
					for _, hLine := range ctx.headerLines {
						if j < len(hLine) {
							nextLogicalCellContentBuilder.WriteString(hLine[j])
						}
					}
					nextHeaderCellContent := t.Trimmer(nextLogicalCellContentBuilder.String())
					if currentHeaderCellContent == nextHeaderCellContent && currentHeaderCellContent != "" && currentHeaderCellContent != "-" {
						span++
					} else {
						break
					}
				}
				if span > 1 {
					headerMergeSpans[i] = span
					for k := 0; k < span; k++ {
						visitedCols[i+k] = true
					}
				}
				i += span
			}
		}
		if len(headerMergeSpans) > 0 {
			ctx.logger.Debugf("Header merge spans: %v", headerMergeSpans)
		}
	}

	// Determine natural column widths
	naturalColumnWidths := tw.NewMapper[int, int]()
	for i := 0; i < ctx.numCols; i++ {
		width := 0
		if colWidth, ok := t.config.Widths.PerColumn.OK(i); ok && colWidth >= 0 {
			width = colWidth
			ctx.logger.Debugf("Col %d width from Config.Widths.PerColumn: %d", i, width)
		} else {
			maxRowFooterWidth := tw.Max(t.rowWidths.Get(i), t.footerWidths.Get(i))
			headerCellOriginalWidth := t.headerWidths.Get(i)
			if t.config.Behavior.Compact.Merge.Enabled() &&
				t.config.Header.Formatting.MergeMode&tw.MergeHorizontal != 0 &&
				headerMergeSpans != nil {
				isColInHeaderMerge := false
				for startCol, span := range headerMergeSpans {
					if i >= startCol && i < startCol+span {
						isColInHeaderMerge = true
						break
					}
				}
				if isColInHeaderMerge {
					width = maxRowFooterWidth
					if width == 0 && headerCellOriginalWidth > 0 {
						width = headerCellOriginalWidth
					}
					ctx.logger.Debugf("Col %d (in merge) width: %d (row/footer: %d, header: %d)", i, width, maxRowFooterWidth, headerCellOriginalWidth)
				} else {
					width = tw.Max(headerCellOriginalWidth, maxRowFooterWidth)
					ctx.logger.Debugf("Col %d (not in merge) width: %d", i, width)
				}
			} else {
				width = tw.Max(tw.Max(headerCellOriginalWidth, t.rowWidths.Get(i)), t.footerWidths.Get(i))
				ctx.logger.Debugf("Col %d width (no merge): %d", i, width)
			}
			if width == 0 && (headerCellOriginalWidth > 0 || t.rowWidths.Get(i) > 0 || t.footerWidths.Get(i) > 0) {
				width = tw.Max(tw.Max(headerCellOriginalWidth, t.rowWidths.Get(i)), t.footerWidths.Get(i))
			}
			if width == 0 {
				width = 1
			}
		}
		naturalColumnWidths.Set(i, width)
	}
	ctx.logger.Debugf("Natural column widths: %v", naturalColumnWidths)

	// Expand columns for merged header content if needed
	workingWidths := naturalColumnWidths.Clone()
	if t.config.Header.Formatting.MergeMode&tw.MergeHorizontal != 0 && headerMergeSpans != nil {
		if span, isOneBigMerge := headerMergeSpans[0]; isOneBigMerge && span == ctx.numCols && ctx.numCols > 0 {
			var firstHeaderCellLogicalContentBuilder strings.Builder
			for _, hLine := range ctx.headerLines {
				if 0 < len(hLine) {
					firstHeaderCellLogicalContentBuilder.WriteString(hLine[0])
				}
			}
			mergedContentString := t.Trimmer(firstHeaderCellLogicalContentBuilder.String())
			headerCellPadding := t.config.Header.Padding.Global
			if 0 < len(t.config.Header.Padding.PerColumn) && t.config.Header.Padding.PerColumn[0].Paddable() {
				headerCellPadding = t.config.Header.Padding.PerColumn[0]
			}
			actualMergedHeaderContentPhysicalWidth := twwidth.Width(mergedContentString) +
				twwidth.Width(headerCellPadding.Left) +
				twwidth.Width(headerCellPadding.Right)
			currentSumOfColumnWidths := 0
			workingWidths.Each(func(_, w int) { currentSumOfColumnWidths += w })
			numSeparatorsInFullSpan := 0
			if ctx.numCols > 1 {
				if t.renderer != nil && t.renderer.Config().Settings.Separators.BetweenColumns.Enabled() {
					numSeparatorsInFullSpan = (ctx.numCols - 1) * twwidth.Width(t.renderer.Config().Symbols.Column())
				}
			}
			totalCurrentSpanPhysicalWidth := currentSumOfColumnWidths + numSeparatorsInFullSpan
			if actualMergedHeaderContentPhysicalWidth > totalCurrentSpanPhysicalWidth {
				ctx.logger.Debugf("Merged header content '%s' (width %d) exceeds total width %d. Expanding.",
					mergedContentString, actualMergedHeaderContentPhysicalWidth, totalCurrentSpanPhysicalWidth)
				shortfall := actualMergedHeaderContentPhysicalWidth - totalCurrentSpanPhysicalWidth
				numNonZeroCols := 0
				workingWidths.Each(func(_, w int) {
					if w > 0 {
						numNonZeroCols++
					}
				})
				if numNonZeroCols == 0 && ctx.numCols > 0 {
					numNonZeroCols = ctx.numCols
				}
				if numNonZeroCols > 0 && shortfall > 0 {
					extraPerColumn := int(math.Ceil(float64(shortfall) / float64(numNonZeroCols)))
					finalSumAfterExpansion := 0
					workingWidths.Each(func(colIdx, currentW int) {
						if currentW > 0 || (numNonZeroCols == ctx.numCols && ctx.numCols > 0) {
							newWidth := currentW + extraPerColumn
							workingWidths.Set(colIdx, newWidth)
							finalSumAfterExpansion += newWidth
							ctx.logger.Debugf("Col %d expanded by %d to %d", colIdx, extraPerColumn, newWidth)
						} else {
							finalSumAfterExpansion += currentW
						}
					})
					overDistributed := (finalSumAfterExpansion + numSeparatorsInFullSpan) - actualMergedHeaderContentPhysicalWidth
					if overDistributed > 0 {
						ctx.logger.Debugf("Correcting over-distribution of %d", overDistributed)
						// Sort columns for deterministic reduction
						sortedCols := workingWidths.SortedKeys()
						for i := 0; i < overDistributed; i++ {
							// Reduce from highest-indexed column
							for j := len(sortedCols) - 1; j >= 0; j-- {
								col := sortedCols[j]
								if workingWidths.Get(col) > 1 && naturalColumnWidths.Get(col) < workingWidths.Get(col) {
									workingWidths.Set(col, workingWidths.Get(col)-1)
									ctx.logger.Debugf("Reduced col %d by 1 to %d", col, workingWidths.Get(col))
									break
								}
							}
						}
					}
				}
			}
		}
	}
	ctx.logger.Debugf("Widths after merged header expansion: %v", workingWidths)

	// Apply global width constraint
	finalWidths := workingWidths.Clone()
	if t.config.Widths.Global > 0 {
		ctx.logger.Debugf("Applying global width constraint: %d", t.config.Widths.Global)
		currentSumOfFinalColWidths := 0
		finalWidths.Each(func(_, w int) { currentSumOfFinalColWidths += w })
		numSeparators := 0
		if ctx.numCols > 1 && t.renderer != nil && t.renderer.Config().Settings.Separators.BetweenColumns.Enabled() {
			numSeparators = (ctx.numCols - 1) * twwidth.Width(t.renderer.Config().Symbols.Column())
		}
		totalCurrentTablePhysicalWidth := currentSumOfFinalColWidths + numSeparators
		if totalCurrentTablePhysicalWidth > t.config.Widths.Global {
			ctx.logger.Debugf("Table width %d exceeds global limit %d. Shrinking.", totalCurrentTablePhysicalWidth, t.config.Widths.Global)
			targetTotalColumnContentWidth := max(t.config.Widths.Global-numSeparators, 0)
			if ctx.numCols > 0 && targetTotalColumnContentWidth < ctx.numCols {
				targetTotalColumnContentWidth = ctx.numCols
			}
			hardMinimums := tw.NewMapper[int, int]()
			sumOfHardMinimums := 0
			isHeaderContentHardToWrap := t.config.Header.Formatting.AutoWrap != tw.WrapNormal && t.config.Header.Formatting.AutoWrap != tw.WrapBreak
			for i := 0; i < ctx.numCols; i++ {
				minW := 1
				if isHeaderContentHardToWrap && len(ctx.headerLines) > 0 {
					headerColNaturalWidthWithPadding := t.headerWidths.Get(i)
					if headerColNaturalWidthWithPadding > minW {
						minW = headerColNaturalWidthWithPadding
					}
				}
				hardMinimums.Set(i, minW)
				sumOfHardMinimums += minW
			}
			ctx.logger.Debugf("Hard minimums: %v (sum: %d)", hardMinimums, sumOfHardMinimums)
			if targetTotalColumnContentWidth < sumOfHardMinimums && sumOfHardMinimums > 0 {
				ctx.logger.Warnf("Target width %d below minimums %d. Scaling.", targetTotalColumnContentWidth, sumOfHardMinimums)
				scaleFactorMin := float64(targetTotalColumnContentWidth) / float64(sumOfHardMinimums)
				if scaleFactorMin < 0 {
					scaleFactorMin = 0
				}
				tempSum := 0
				scaledHardMinimums := tw.NewMapper[int, int]()
				hardMinimums.Each(func(colIdx, currentMinW int) {
					scaledMinW := int(math.Round(float64(currentMinW) * scaleFactorMin))
					if scaledMinW < 1 && targetTotalColumnContentWidth > 0 {
						scaledMinW = 1
					} else if scaledMinW < 0 {
						scaledMinW = 0
					}
					scaledHardMinimums.Set(colIdx, scaledMinW)
					tempSum += scaledMinW
				})
				errorDiffMin := targetTotalColumnContentWidth - tempSum
				if errorDiffMin != 0 && scaledHardMinimums.Len() > 0 {
					sortedKeys := scaledHardMinimums.SortedKeys()
					for i := 0; i < int(math.Abs(float64(errorDiffMin))); i++ {
						keyToAdjust := sortedKeys[i%len(sortedKeys)]
						val := scaledHardMinimums.Get(keyToAdjust)
						adj := 1
						if errorDiffMin < 0 {
							adj = -1
						}
						if val+adj >= 1 || (val+adj == 0 && targetTotalColumnContentWidth == 0) {
							scaledHardMinimums.Set(keyToAdjust, val+adj)
						} else if adj > 0 {
							scaledHardMinimums.Set(keyToAdjust, val+adj)
						}
					}
				}
				finalWidths = scaledHardMinimums.Clone()
				ctx.logger.Debugf("Scaled minimums: %v", finalWidths)
			} else {
				finalWidths = hardMinimums.Clone()
				widthAllocatedByMinimums := sumOfHardMinimums
				remainingWidthToDistribute := targetTotalColumnContentWidth - widthAllocatedByMinimums
				ctx.logger.Debugf("Target: %d, minimums: %d, remaining: %d", targetTotalColumnContentWidth, widthAllocatedByMinimums, remainingWidthToDistribute)
				if remainingWidthToDistribute > 0 {
					sumOfFlexiblePotentialBase := 0
					flexibleColsOriginalWidths := tw.NewMapper[int, int]()
					for i := 0; i < ctx.numCols; i++ {
						naturalW := workingWidths.Get(i)
						minW := hardMinimums.Get(i)
						if naturalW > minW {
							sumOfFlexiblePotentialBase += (naturalW - minW)
							flexibleColsOriginalWidths.Set(i, naturalW)
						}
					}
					ctx.logger.Debugf("Flexible potential: %d, flexible widths: %v", sumOfFlexiblePotentialBase, flexibleColsOriginalWidths)
					if sumOfFlexiblePotentialBase > 0 {
						distributedExtraSum := 0
						sortedFlexKeys := flexibleColsOriginalWidths.SortedKeys()
						for _, colIdx := range sortedFlexKeys {
							naturalWOfCol := flexibleColsOriginalWidths.Get(colIdx)
							hardMinOfCol := hardMinimums.Get(colIdx)
							flexiblePartOfCol := naturalWOfCol - hardMinOfCol
							proportion := 0.0
							if sumOfFlexiblePotentialBase > 0 {
								proportion = float64(flexiblePartOfCol) / float64(sumOfFlexiblePotentialBase)
							} else if len(sortedFlexKeys) > 0 {
								proportion = 1.0 / float64(len(sortedFlexKeys))
							}
							extraForThisCol := int(math.Round(float64(remainingWidthToDistribute) * proportion))
							currentAssignedW := finalWidths.Get(colIdx)
							finalWidths.Set(colIdx, currentAssignedW+extraForThisCol)
							distributedExtraSum += extraForThisCol
						}
						errorInDist := remainingWidthToDistribute - distributedExtraSum
						ctx.logger.Debugf("Distributed %d, error: %d", distributedExtraSum, errorInDist)
						if errorInDist != 0 && len(sortedFlexKeys) > 0 {
							for i := 0; i < int(math.Abs(float64(errorInDist))); i++ {
								colToAdjust := sortedFlexKeys[i%len(sortedFlexKeys)]
								w := finalWidths.Get(colToAdjust)
								adj := 1
								if errorInDist < 0 {
									adj = -1
								}
								if adj >= 0 || w+adj >= hardMinimums.Get(colToAdjust) {
									finalWidths.Set(colToAdjust, w+adj)
								} else if adj > 0 {
									finalWidths.Set(colToAdjust, w+adj)
								}
							}
						}
					} else if ctx.numCols > 0 {
						extraPerCol := remainingWidthToDistribute / ctx.numCols
						rem := remainingWidthToDistribute % ctx.numCols
						for i := 0; i < ctx.numCols; i++ {
							currentW := finalWidths.Get(i)
							add := extraPerCol
							if i < rem {
								add++
							}
							finalWidths.Set(i, currentW+add)
						}
					}
				}
			}
			finalSumCheck := 0
			finalWidths.Each(func(idx, w int) {
				if w < 1 && targetTotalColumnContentWidth > 0 {
					finalWidths.Set(idx, 1)
				} else if w < 0 {
					finalWidths.Set(idx, 0)
				}
				finalSumCheck += finalWidths.Get(idx)
			})
			ctx.logger.Debugf("Final widths after scaling: %v (sum: %d, target: %d)", finalWidths, finalSumCheck, targetTotalColumnContentWidth)
		}
	}

	// Assign final widths to context
	ctx.widths[tw.Header] = finalWidths.Clone()
	ctx.widths[tw.Row] = finalWidths.Clone()
	ctx.widths[tw.Footer] = finalWidths.Clone()
	ctx.logger.Debugf("Final normalized widths: header=%v, row=%v, footer=%v", ctx.widths[tw.Header], ctx.widths[tw.Row], ctx.widths[tw.Footer])
	return nil
}

// calculateContentMaxWidth computes the maximum content width for a column, accounting for padding and mode-specific constraints.
// Returns the effective content width (after subtracting padding) for the given column index.
func (t *Table) calculateContentMaxWidth(colIdx int, config tw.CellConfig, padLeftWidth, padRightWidth int, isStreaming bool) int {
	var effectiveContentMaxWidth int

	if isStreaming {
		// Existing streaming logic remains unchanged
		totalColumnWidthFromStream := max(t.streamWidths.Get(colIdx), 0)
		effectiveContentMaxWidth = totalColumnWidthFromStream - padLeftWidth - padRightWidth
		if effectiveContentMaxWidth < 1 && totalColumnWidthFromStream > (padLeftWidth+padRightWidth) {
			effectiveContentMaxWidth = 1
		} else if effectiveContentMaxWidth < 0 {
			effectiveContentMaxWidth = 0
		}
		if totalColumnWidthFromStream == 0 {
			effectiveContentMaxWidth = 0
		}
		t.logger.Debugf("calculateContentMaxWidth: Streaming col %d, TotalColWd=%d, PadL=%d, PadR=%d -> ContentMaxWd=%d", colIdx, totalColumnWidthFromStream, padLeftWidth, padRightWidth, effectiveContentMaxWidth)
	} else {
		// New priority-based width constraint checking
		constraintTotalCellWidth := 0
		hasConstraint := false

		// Check new Widths.PerColumn (highest priority)
		if t.config.Widths.Constrained() {

			if colWidth, ok := t.config.Widths.PerColumn.OK(colIdx); ok && colWidth > 0 {
				constraintTotalCellWidth = colWidth
				hasConstraint = true
				t.logger.Debugf("calculateContentMaxWidth: Using Widths.PerColumn[%d] = %d",
					colIdx, constraintTotalCellWidth)
			}

			// Check new Widths.Global
			if !hasConstraint && t.config.Widths.Global > 0 {
				constraintTotalCellWidth = t.config.Widths.Global
				hasConstraint = true
				t.logger.Debugf("calculateContentMaxWidth: Using Widths.Global = %d", constraintTotalCellWidth)
			}
		}

		// Fall back to legacy ColMaxWidths.PerColumn (backward compatibility)
		if !hasConstraint && config.ColMaxWidths.PerColumn != nil {
			if colMax, ok := config.ColMaxWidths.PerColumn.OK(colIdx); ok && colMax > 0 {
				constraintTotalCellWidth = colMax
				hasConstraint = true
				t.logger.Debugf("calculateContentMaxWidth: Using legacy ColMaxWidths.PerColumn[%d] = %d",
					colIdx, constraintTotalCellWidth)
			}
		}

		// Fall back to legacy ColMaxWidths.Global
		if !hasConstraint && config.ColMaxWidths.Global > 0 {
			constraintTotalCellWidth = config.ColMaxWidths.Global
			hasConstraint = true
			t.logger.Debugf("calculateContentMaxWidth: Using legacy ColMaxWidths.Global = %d",
				constraintTotalCellWidth)
		}

		// Fall back to table MaxWidth if auto-wrapping
		if !hasConstraint && t.config.MaxWidth > 0 && config.Formatting.AutoWrap != tw.WrapNone {
			constraintTotalCellWidth = t.config.MaxWidth
			hasConstraint = true
			t.logger.Debugf("calculateContentMaxWidth: Using table MaxWidth = %d (AutoWrap enabled)",
				constraintTotalCellWidth)
		}

		// Calculate effective width based on found constraint
		if hasConstraint {
			effectiveContentMaxWidth = constraintTotalCellWidth - padLeftWidth - padRightWidth
			if effectiveContentMaxWidth < 1 && constraintTotalCellWidth > (padLeftWidth+padRightWidth) {
				effectiveContentMaxWidth = 1
			} else if effectiveContentMaxWidth < 0 {
				effectiveContentMaxWidth = 0
			}
			t.logger.Debugf("calculateContentMaxWidth: ConstraintTotalCellWidth=%d, PadL=%d, PadR=%d -> EffectiveContentMaxWidth=%d",
				constraintTotalCellWidth, padLeftWidth, padRightWidth, effectiveContentMaxWidth)
		} else {
			effectiveContentMaxWidth = 0
			t.logger.Debugf("calculateContentMaxWidth: No width constraints found for column %d", colIdx)
		}
	}

	return effectiveContentMaxWidth
}

// convertToStringer invokes the table's stringer function with optional caching.
func (t *Table) convertToStringer(input interface{}) ([]string, error) {
	// This function is now only called if t.stringer is non-nil.
	if t.stringer == nil {
		return nil, errors.New("internal error: convertToStringer called with nil t.stringer")
	}

	t.logger.Debugf("convertToString attempt %v using %v", input, t.stringer)

	inputType := reflect.TypeOf(input)

	// Cache lookup using twcache.LRU
	// This assumes t.stringerCache is *twcache.LRU[reflect.Type, reflect.Value]
	if t.stringerCache != nil {
		if cachedFunc, ok := t.stringerCache.Get(inputType); ok {
			t.logger.Debugf("convertToStringer: Cache hit for type %v", inputType)
			// We can proceed to call it immediately because it's already been validated/cached
			results := cachedFunc.Call([]reflect.Value{reflect.ValueOf(input)})
			if len(results) == 1 && results[0].Type() == reflect.TypeOf([]string{}) {
				return results[0].Interface().([]string), nil
			}
		}
	}

	stringerFuncVal := reflect.ValueOf(t.stringer)
	stringerFuncType := stringerFuncVal.Type()

	// Robust type checking for the stringer function
	validSignature := stringerFuncVal.Kind() == reflect.Func &&
		stringerFuncType.NumIn() == 1 &&
		stringerFuncType.NumOut() == 1 &&
		stringerFuncType.Out(0) == reflect.TypeOf([]string{})

	if !validSignature {
		return nil, errors.Newf("table stringer (type %T) does not have signature func(SomeType) []string", t.stringer)
	}

	// Check if input is assignable to stringer's parameter type
	paramType := stringerFuncType.In(0)
	assignable := false
	if inputType != nil { // input is not untyped nil
		if inputType.AssignableTo(paramType) {
			assignable = true
		} else if paramType.Kind() == reflect.Interface && inputType.Implements(paramType) {
			assignable = true
		} else if paramType.Kind() == reflect.Interface && paramType.NumMethod() == 0 { // stringer expects interface{}
			assignable = true
		}
	} else if paramType.Kind() == reflect.Interface || (paramType.Kind() == reflect.Ptr && paramType.Elem().Kind() != reflect.Interface) {
		// If input is nil, it can be assigned if stringer expects an interface or a pointer type
		assignable = true
	}

	if !assignable {
		return nil, errors.Newf("input type %T cannot be passed to table stringer expecting %s", input, paramType)
	}

	var callArgs []reflect.Value
	if input == nil {
		// If input is nil, we must pass a zero value of the stringer's parameter type
		// if that type is a pointer or interface.
		callArgs = []reflect.Value{reflect.Zero(paramType)}
	} else {
		callArgs = []reflect.Value{reflect.ValueOf(input)}
	}

	resultValues := stringerFuncVal.Call(callArgs)

	// Add to cache if enabled (not nil) and input type is valid
	if t.stringerCache != nil && inputType != nil {
		t.stringerCache.Add(inputType, stringerFuncVal)
	}

	return resultValues[0].Interface().([]string), nil
}

// convertToString converts a value to its string representation.
func (t *Table) convertToString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case tw.Formatter:
		return v.Format()
	case io.Reader:
		const maxReadSize = 512
		var buf strings.Builder
		_, err := io.CopyN(&buf, v, maxReadSize)
		if err != nil && err != io.EOF {
			return fmt.Sprintf("[reader error: %v]", err) // Keep fmt.Sprintf for rare error case
		}
		if buf.Len() == maxReadSize {
			buf.WriteString(tw.CharEllipsis)
		}
		return buf.String()
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return ""
	case sql.NullInt64:
		if v.Valid {
			return strconv.FormatInt(v.Int64, 10)
		}
		return ""
	case sql.NullFloat64:
		if v.Valid {
			return strconv.FormatFloat(v.Float64, 'f', -1, 64)
		}
		return ""
	case sql.NullBool:
		if v.Valid {
			return strconv.FormatBool(v.Bool)
		}
		return ""
	case sql.NullTime:
		if v.Valid {
			return v.Time.String()
		}
		return ""
	case []byte:
		return string(v)
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	case string:
		return v
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		t.logger.Debugf("convertToString: Falling back to fmt.Sprintf for type %T", value)
		return fmt.Sprintf("%v", value) // Fallback for rare types
	}
}

// convertItemToCells is responsible for converting a single input item (which could be
// a struct, a basic type, or an item implementing Stringer/Formatter) into a slice
// of strings, where each string represents a cell for the table row.
func (t *Table) convertItemToCells(item interface{}) ([]string, error) {
	t.logger.Debugf("convertItemToCells: Converting item of type %T", item)

	// User-defined table-wide stringer (t.stringer) takes highest precedence.
	if t.stringer != nil {
		res, err := t.convertToStringer(item)
		if err == nil {
			t.logger.Debugf("convertItemToCells: Used custom table stringer for type %T. Produced %d cells: %v", item, len(res), res)
			return res, nil
		}
		t.logger.Warnf("convertItemToCells: Custom table stringer was set but incompatible for type %T: %v. Will attempt other methods.", item, err)
	}

	// Handle untyped nil directly.
	if item == nil {
		t.logger.Debugf("convertItemToCells: Item is untyped nil. Returning single empty cell.")
		return []string{""}, nil
	}

	// Use the new unified struct parser. It handles pointers and embedding.
	// We only care about the values it returns.
	_, values := t.extractFieldsAndValuesFromStruct(item)
	if values != nil {
		t.logger.Debugf("convertItemToCells: Structs %T reflected into %d cells: %v", item, len(values), values)
		return values, nil
	}

	// Fallback for any other single item (e.g., basic types, or types that implement Stringer/Formatter).
	// This code path is now for non-struct types.
	if formatter, ok := item.(tw.Formatter); ok {
		t.logger.Debugf("convertItemToCells: Item (non-struct, type %T) is tw.Formatter. Using Format().", item)
		return []string{formatter.Format()}, nil
	}
	if stringer, ok := item.(fmt.Stringer); ok {
		t.logger.Debugf("convertItemToCells: Item (non-struct, type %T) is fmt.Stringer. Using String().", item)
		return []string{stringer.String()}, nil
	}

	t.logger.Debugf("convertItemToCells: Item (type %T) is a basic type. Treating as single cell via convertToString.", item)
	return []string{t.convertToString(item)}, nil
}

// convertCellsToStrings converts a row to its raw string representation using specified cell config for filters.
// 'rowInput' can be []string, []any, or a custom type if t.stringer is set.
func (t *Table) convertCellsToStrings(rowInput interface{}, cellCfg tw.CellConfig) ([]string, error) {
	t.logger.Debugf("convertCellsToStrings: Converting row: %v (type: %T)", rowInput, rowInput)

	var cells []string
	var err error

	switch v := rowInput.(type) {
	// Directly supported slice types
	case []string:
		cells = v
	case []interface{}: // Catches variadic simple types grouped by Append
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = t.convertToString(val)
		}
	case []int:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.Itoa(val)
		}
	case []int8:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatInt(int64(val), 10)
		}
	case []int16:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatInt(int64(val), 10)
		}
	case []int32: // Also rune
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = t.convertToString(val)
		} // Use convertToString for potential rune
	case []int64:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatInt(val, 10)
		}
	case []uint:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatUint(uint64(val), 10)
		}
	case []uint8: // Also byte
		cells = make([]string, len(v))
		// If it's truly []byte, convertToString will handle it as a string.
		// If it's a slice of small numbers, convertToString will handle them individually.
		for i, val := range v {
			cells[i] = t.convertToString(val)
		}
	case []uint16:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatUint(uint64(val), 10)
		}
	case []uint32:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatUint(uint64(val), 10)
		}
	case []uint64:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatUint(val, 10)
		}
	case []float32:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatFloat(float64(val), 'f', -1, 32)
		}
	case []float64:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatFloat(val, 'f', -1, 64)
		}
	case []bool:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = strconv.FormatBool(val)
		}
	case []tw.Formatter:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = val.Format()
		}
	case []fmt.Stringer:
		cells = make([]string, len(v))
		for i, val := range v {
			cells[i] = val.String()
		}

	// Cases for single items that are NOT slices
	// These are now dispatched to convertItemToCells by the default case.
	// Keeping direct tw.Formatter and fmt.Stringer here could be a micro-optimization
	// if `rowInput` is *exactly* that type (not a struct implementing it),
	// but for clarity, `convertItemToCells` can handle these too.
	// For this iteration, to match the described flow:
	case tw.Formatter: // This handles a single Formatter item
		t.logger.Debugf("convertCellsToStrings: Input is a single tw.Formatter. Using Format().")
		cells = []string{v.Format()}
	case fmt.Stringer: // This handles a single Stringer item
		t.logger.Debugf("convertCellsToStrings: Input is a single fmt.Stringer. Using String().")
		cells = []string{v.String()}

	default:
		// If rowInput is not one of the recognized slice types above,
		// or not a single Formatter/Stringer that was directly matched,
		// it's treated as a single item that needs to be converted into potentially multiple cells.
		// This is where structs (for field expansion) or other single values (for a single cell) are handled.
		t.logger.Debugf("convertCellsToStrings: Default case for type %T. Dispatching to convertItemToCells.", rowInput)
		cells, err = t.convertItemToCells(rowInput)
		if err != nil {
			t.logger.Errorf("convertCellsToStrings: Error from convertItemToCells for type %T: %v", rowInput, err)
			return nil, err
		}
	}

	// Apply filters (common logic for all successful conversions)
	if err == nil && cells != nil {
		if cellCfg.Filter.Global != nil {
			t.logger.Debugf("convertCellsToStrings: Applying global filter to cells: %v", cells)
			cells = cellCfg.Filter.Global(cells)
		}
		if len(cellCfg.Filter.PerColumn) > 0 {
			t.logger.Debugf("convertCellsToStrings: Applying per-column filters to %d cells", len(cells))
			for i := 0; i < len(cellCfg.Filter.PerColumn); i++ {
				if i < len(cells) && cellCfg.Filter.PerColumn[i] != nil {
					originalCell := cells[i]
					cells[i] = cellCfg.Filter.PerColumn[i](cells[i])
					if cells[i] != originalCell {
						t.logger.Debugf("  convertCellsToStrings: Col %d filter applied: '%s' -> '%s'", i, originalCell, cells[i])
					}
				} else if i >= len(cells) && cellCfg.Filter.PerColumn[i] != nil {
					t.logger.Warnf("  convertCellsToStrings: Per-column filter defined for col %d, but item only produced %d cells. Filter for this column skipped.", i, len(cells))
				}
			}
		}
	}

	if err != nil {
		t.logger.Debugf("convertCellsToStrings: Returning with error: %v", err)
		return nil, err
	}
	t.logger.Debugf("convertCellsToStrings: Conversion and filtering completed, raw cells: %v", cells)
	return cells, nil
}

// determineLocation determines the boundary location for a line.
// Parameters include lineIdx, totalLines, topPad, and bottomPad.
// Returns a tw.Location indicating First, Middle, or End.
func (t *Table) determineLocation(lineIdx, totalLines int, topPad, bottomPad string) tw.Location {
	if lineIdx == 0 && topPad == tw.Empty {
		return tw.LocationFirst
	}
	if lineIdx == totalLines-1 && bottomPad == tw.Empty {
		return tw.LocationEnd
	}
	return tw.LocationMiddle
}

// ensureStreamWidthsCalculated ensures that stream widths and column count are initialized for streaming mode.
// It uses sampleData and sectionConfig to calculate widths if not already set.
// Returns an error if the column count cannot be determined.
func (t *Table) ensureStreamWidthsCalculated(sampleData []string, sectionConfig tw.CellConfig) error {
	if t.streamWidths != nil && t.streamWidths.Len() > 0 {
		t.logger.Debugf("Stream widths already set: %v", t.streamWidths)
		return nil
	}
	t.streamCalculateWidths(sampleData, sectionConfig)
	if t.streamNumCols == 0 {
		t.logger.Warn("Failed to determine column count from sample data")
		return errors.New("failed to determine column count for streaming")
	}
	for i := 0; i < t.streamNumCols; i++ {
		if _, ok := t.streamWidths.OK(i); !ok {
			t.streamWidths.Set(i, 0)
		}
	}
	t.logger.Debugf("Initialized stream widths: %v", t.streamWidths)
	return nil
}

// getColMaxWidths retrieves maximum column widths for a section.
// Parameter position specifies the section (Header, Row, Footer).
// Returns a map of column indices to maximum widths.
func (t *Table) getColMaxWidths(position tw.Position) tw.CellWidth {
	switch position {
	case tw.Header:
		return t.config.Header.ColMaxWidths
	case tw.Row:
		return t.config.Row.ColMaxWidths
	case tw.Footer:
		return t.config.Footer.ColMaxWidths
	default:
		return tw.CellWidth{}
	}
}

// getEmptyColumnInfo identifies empty columns in row data.
// Parameter numOriginalCols specifies the total column count.
// Returns a boolean slice (true for empty) and visible column count.
func (t *Table) getEmptyColumnInfo(processedRows [][][]string, numOriginalCols int) (isEmpty []bool, visibleColCount int) {
	isEmpty = make([]bool, numOriginalCols)
	for i := range isEmpty {
		isEmpty[i] = true
	}

	if t.config.Behavior.AutoHide.Disabled() {
		t.logger.Debugf("getEmptyColumnInfo: AutoHide disabled, marking all %d columns as visible.", numOriginalCols)
		for i := range isEmpty {
			isEmpty[i] = false
		}
		visibleColCount = numOriginalCols
		return isEmpty, visibleColCount
	}

	t.logger.Debugf("getEmptyColumnInfo: Checking %d rows for %d columns...", len(processedRows), numOriginalCols)

	for rowIdx, logicalRow := range processedRows {
		for lineIdx, visualLine := range logicalRow {
			for colIdx, cellContent := range visualLine {
				if colIdx >= numOriginalCols {
					continue
				}
				if !isEmpty[colIdx] {
					continue
				}

				cellContent = t.Trimmer(cellContent)

				if cellContent != "" {
					isEmpty[colIdx] = false
					t.logger.Debugf("getEmptyColumnInfo: Found content in row %d, line %d, col %d ('%s'). Marked as not empty.", rowIdx, lineIdx, colIdx, cellContent)
				}
			}
		}
	}

	visibleColCount = 0
	for _, empty := range isEmpty {
		if !empty {
			visibleColCount++
		}
	}

	t.logger.Debugf("getEmptyColumnInfo: Detection complete. isEmpty: %v, visibleColCount: %d", isEmpty, visibleColCount)
	return isEmpty, visibleColCount
}

// getNumColsToUse determines the number of columns to use for rendering, based on streaming or batch mode.
// Returns the number of columns (streamNumCols for streaming, maxColumns for batch).
func (t *Table) getNumColsToUse() int {
	if t.config.Stream.Enable && t.hasPrinted {
		t.logger.Debugf("getNumColsToUse: Using streamNumCols: %d", t.streamNumCols)
		return t.streamNumCols
	}

	// For batch mode:
	if t.isBatchRenderNumColsSet {
		// If the flag is set, batchRenderNumCols holds the authoritative count
		// for the current Render() pass, even if that count is 0.
		t.logger.Debugf("getNumColsToUse (batch): Using cached t.batchRenderNumCols: %d (because isBatchRenderNumColsSet is true)", t.batchRenderNumCols)
		return t.batchRenderNumCols
	}

	// Fallback: If not streaming and cache flag is not set (e.g., called outside a Render pass)
	num := t.maxColumns()
	t.logger.Debugf("getNumColsToUse (batch): Cache not active, calculated via t.maxColumns(): %d", num)
	return num
}

// prepareTableSection prepares either headers or footers for the table
func (t *Table) prepareTableSection(elements []any, config tw.CellConfig, sectionName string) [][]string {
	actualCellsToProcess := t.processVariadic(elements)
	t.logger.Debugf("%s(): Effective cells to process: %v", sectionName, actualCellsToProcess)

	stringsResult, err := t.convertCellsToStrings(actualCellsToProcess, config)
	if err != nil {
		t.logger.Errorf("%s(): Failed to convert elements to strings: %v", sectionName, err)
		stringsResult = []string{}
	}

	prepared := t.prepareContent(stringsResult, config)
	numColsBatch := t.maxColumns()

	if len(prepared) > 0 {
		for i := range prepared {
			if len(prepared[i]) < numColsBatch {
				t.logger.Debugf("Padding %s line %d from %d to %d columns", sectionName, i, len(prepared[i]), numColsBatch)
				paddedLine := make([]string, numColsBatch)
				copy(paddedLine, prepared[i])
				for j := len(prepared[i]); j < numColsBatch; j++ {
					paddedLine[j] = tw.Empty
				}
				prepared[i] = paddedLine
			} else if len(prepared[i]) > numColsBatch {
				t.logger.Debugf("Truncating %s line %d from %d to %d columns", sectionName, i, len(prepared[i]), numColsBatch)
				prepared[i] = prepared[i][:numColsBatch]
			}
		}
	}

	return prepared
}

// processVariadic handles the common logic for processing variadic arguments
// that could be either individual elements or a slice of elements
func (t *Table) processVariadic(elements []any) []any {
	if len(elements) == 1 {
		switch v := elements[0].(type) {
		case []string:
			t.logger.Debugf("Detected single []string argument. Unpacking it (fast path).")
			out := make([]any, len(v))
			for i := range v {
				out[i] = v[i]
			}
			return out

		case []interface{}:
			t.logger.Debugf("Detected single []interface{} argument. Unpacking it (fast path).")
			out := make([]any, len(v))
			copy(out, v)
			return out
		}
	}

	t.logger.Debugf("Input has multiple elements or single non-slice. Using variadic elements as-is.")
	return elements
}

// updateWidths updates the width map based on cell content and padding.
// Parameters include row content, widths map, and padding configuration.
// No return value.
func (t *Table) updateWidths(row []string, widths tw.Mapper[int, int], padding tw.CellPadding) {
	t.logger.Debugf("Updating widths for row: %v", row)
	for i, cell := range row {
		colPad := padding.Global

		if i < len(padding.PerColumn) && padding.PerColumn[i].Paddable() {
			colPad = padding.PerColumn[i]
			t.logger.Debugf("  Col %d: Using per-column padding: L:'%s' R:'%s'", i, colPad.Left, colPad.Right)
		} else {
			t.logger.Debugf("  Col %d: Using global padding: L:'%s' R:'%s'", i, padding.Global.Left, padding.Global.Right)
		}

		padLeftWidth := twwidth.Width(colPad.Left)
		padRightWidth := twwidth.Width(colPad.Right)

		// Split cell into lines and find maximum content width
		lines := strings.Split(cell, tw.NewLine)
		contentWidth := 0
		for _, line := range lines {
			// Always measure the raw line width, because the renderer
			// will receive the raw line. Do not trim before measuring.
			lineWidth := twwidth.Width(line)
			if lineWidth > contentWidth {
				contentWidth = lineWidth
			}
		}

		totalWidth := contentWidth + padLeftWidth + padRightWidth
		minRequiredPaddingWidth := padLeftWidth + padRightWidth

		if contentWidth == 0 && totalWidth < minRequiredPaddingWidth {
			t.logger.Debugf("  Col %d: Empty content, ensuring width >= padding width (%d). Setting totalWidth to %d.", i, minRequiredPaddingWidth, minRequiredPaddingWidth)
			totalWidth = minRequiredPaddingWidth
		}

		if totalWidth < 1 {
			t.logger.Debugf("  Col %d: Calculated totalWidth is zero, setting minimum width to 1.", i)
			totalWidth = 1
		}

		currentMax, _ := widths.OK(i)
		if totalWidth > currentMax {
			widths.Set(i, totalWidth)
			t.logger.Debugf("  Col %d: Updated width from %d to %d (content:%d + padL:%d + padR:%d) for cell '%s'", i, currentMax, totalWidth, contentWidth, padLeftWidth, padRightWidth, cell)
		} else {
			t.logger.Debugf("  Col %d: Width %d not greater than current max %d for cell '%s'", i, totalWidth, currentMax, cell)
		}
	}
}

// extractHeadersFromStruct is now a thin wrapper around the new unified function.
// It only cares about the header names.
func (t *Table) extractHeadersFromStruct(sample interface{}) []string {
	headers, _ := t.extractFieldsAndValuesFromStruct(sample)
	return headers
}

// extractFieldsAndValuesFromStruct is the new single source of truth for struct reflection.
// It recursively processes a struct, handling pointers and embedded structs,
// and returns two slices: one for header names and one for string-converted values.
func (t *Table) extractFieldsAndValuesFromStruct(sample interface{}) ([]string, []string) {
	v := reflect.ValueOf(sample)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, nil
	}

	typ := v.Type()
	headers := make([]string, 0, typ.NumField())
	values := make([]string, 0, typ.NumField())

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Handle embedded structs recursively
		if field.Anonymous {
			h, val := t.extractFieldsAndValuesFromStruct(fieldValue.Interface())
			if h != nil {
				headers = append(headers, h...)
				values = append(values, val...)
			}
			continue
		}

		var tagName string
		skipField := false

		// Loop through the priority list of configured tags (e.g., ["json", "db"])
		for _, tagKey := range t.config.Behavior.Structs.Tags {
			tagValue := field.Tag.Get(tagKey)

			// If a tag is found...
			if tagValue != "" {
				// If the tag is "-", this field should be skipped entirely.
				if tagValue == "-" {
					skipField = true
					break // Stop processing tags for this field.
				}
				// Otherwise, we've found our highest-priority tag. Store it and stop.
				tagName = tagValue
				break // Stop processing tags for this field.
			}
		}

		// If the field was marked for skipping, continue to the next field.
		if skipField {
			continue
		}

		// Determine header name from the tag or fallback to the field name
		headerName := field.Name
		if tagName != "" {
			headerName = strings.Split(tagName, ",")[0]
		}
		headers = append(headers, tw.Title(headerName))

		// Determine value, respecting omitempty from the found tag
		value := ""
		if !strings.Contains(tagName, ",omitempty") || !fieldValue.IsZero() {
			value = t.convertToString(fieldValue.Interface())
		}
		values = append(values, value)
	}

	return headers, values
}
