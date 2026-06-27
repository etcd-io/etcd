package renderer

import (
	"github.com/olekukonko/ll"
	"github.com/olekukonko/tablewriter/tw"
)

// Junction handles rendering of table junction points (corners, intersections) with color support.
type Junction struct {
	sym           tw.Symbols    // Symbols used for rendering junctions and lines
	ctx           tw.Formatting // Current table formatting context
	colIdx        int           // Index of the column being processed
	debugging     bool          // Enables debug logging
	borderTint    Tint          // Colors for border symbols
	separatorTint Tint          // Colors for separator symbols
	logger        *ll.Logger
}

type JunctionContext struct {
	Symbols       tw.Symbols
	Ctx           tw.Formatting
	ColIdx        int
	Logger        *ll.Logger
	BorderTint    Tint
	SeparatorTint Tint
}

// NewJunction initializes a Junction with the given symbols, context, and tints.
// If debug is nil, a no-op debug function is used.
func NewJunction(ctx JunctionContext) *Junction {
	return &Junction{
		sym:           ctx.Symbols,
		ctx:           ctx.Ctx,
		colIdx:        ctx.ColIdx,
		logger:        ctx.Logger.Namespace("junction"),
		borderTint:    ctx.BorderTint,
		separatorTint: ctx.SeparatorTint,
	}
}

// getMergeState retrieves the merge state for a specific column in a row, returning an empty state if not found.
func (jr *Junction) getMergeState(row map[int]tw.CellContext, colIdx int) tw.MergeState {
	if row == nil || colIdx < 0 {
		return tw.MergeState{}
	}
	return row[colIdx].Merge
}

// GetSegment determines whether to render a colored horizontal line or an empty space based on merge states.
func (jr *Junction) GetSegment() string {
	currentMerge := jr.getMergeState(jr.ctx.Row.Current, jr.colIdx)
	nextMerge := jr.getMergeState(jr.ctx.Row.Next, jr.colIdx)

	vPassThruStrict := (currentMerge.Vertical.Present && nextMerge.Vertical.Present && !currentMerge.Vertical.End && !nextMerge.Vertical.Start) ||
		(currentMerge.Hierarchical.Present && nextMerge.Hierarchical.Present && !currentMerge.Hierarchical.End && !nextMerge.Hierarchical.Start)

	if vPassThruStrict {
		jr.logger.Debugf("GetSegment col %d: VPassThruStrict=%v -> Empty segment", jr.colIdx, vPassThruStrict)
		return tw.Empty
	}
	symbol := jr.sym.Row()
	coloredSymbol := jr.borderTint.Apply(symbol)
	jr.logger.Debugf("GetSegment col %d: VPassThruStrict=%v -> Colored row symbol '%s'", jr.colIdx, vPassThruStrict, coloredSymbol)
	return coloredSymbol
}

// RenderLeft selects and colors the leftmost junction symbol for the current row line based on position and merges.
func (jr *Junction) RenderLeft() string {
	mergeAbove := jr.getMergeState(jr.ctx.Row.Current, 0)
	mergeBelow := jr.getMergeState(jr.ctx.Row.Next, 0)

	jr.logger.Debugf("RenderLeft: Level=%v, Location=%v, Previous=%v", jr.ctx.Level, jr.ctx.Row.Location, jr.ctx.Row.Previous)

	isTopBorder := (jr.ctx.Level == tw.LevelHeader && jr.ctx.Row.Location == tw.LocationFirst) ||
		(jr.ctx.Level == tw.LevelBody && jr.ctx.Row.Location == tw.LocationFirst && jr.ctx.Row.Previous == nil)
	if isTopBorder {
		symbol := jr.sym.TopLeft()
		return jr.borderTint.Apply(symbol)
	}

	isBottom := jr.ctx.Level == tw.LevelBody && jr.ctx.Row.Location == tw.LocationEnd && !jr.ctx.HasFooter
	isFooter := jr.ctx.Level == tw.LevelFooter && jr.ctx.Row.Location == tw.LocationEnd
	if isBottom || isFooter {
		symbol := jr.sym.BottomLeft()
		return jr.borderTint.Apply(symbol)
	}

	isVPassThruStrict := (mergeAbove.Vertical.Present && mergeBelow.Vertical.Present && !mergeAbove.Vertical.End && !mergeBelow.Vertical.Start) ||
		(mergeAbove.Hierarchical.Present && mergeBelow.Hierarchical.Present && !mergeAbove.Hierarchical.End && !mergeBelow.Hierarchical.Start)
	if isVPassThruStrict {
		symbol := jr.sym.Column()
		return jr.separatorTint.Apply(symbol)
	}

	symbol := jr.sym.MidLeft()
	return jr.borderTint.Apply(symbol)
}

// RenderRight selects and colors the rightmost junction symbol for the row line based on position, merges, and last column index.
func (jr *Junction) RenderRight(lastColIdx int) string {
	jr.logger.Debugf("RenderRight: lastColIdx=%d, Level=%v, Location=%v, Previous=%v", lastColIdx, jr.ctx.Level, jr.ctx.Row.Location, jr.ctx.Row.Previous)

	if lastColIdx < 0 {
		switch jr.ctx.Level {
		case tw.LevelHeader:
			symbol := jr.sym.TopRight()
			return jr.borderTint.Apply(symbol)
		case tw.LevelFooter:
			symbol := jr.sym.BottomRight()
			return jr.borderTint.Apply(symbol)
		default:
			if jr.ctx.Row.Location == tw.LocationFirst {
				symbol := jr.sym.TopRight()
				return jr.borderTint.Apply(symbol)
			}
			if jr.ctx.Row.Location == tw.LocationEnd {
				symbol := jr.sym.BottomRight()
				return jr.borderTint.Apply(symbol)
			}
			symbol := jr.sym.MidRight()
			return jr.borderTint.Apply(symbol)
		}
	}

	mergeAbove := jr.getMergeState(jr.ctx.Row.Current, lastColIdx)
	mergeBelow := jr.getMergeState(jr.ctx.Row.Next, lastColIdx)

	isTopBorder := (jr.ctx.Level == tw.LevelHeader && jr.ctx.Row.Location == tw.LocationFirst) ||
		(jr.ctx.Level == tw.LevelBody && jr.ctx.Row.Location == tw.LocationFirst && jr.ctx.Row.Previous == nil)
	if isTopBorder {
		symbol := jr.sym.TopRight()
		return jr.borderTint.Apply(symbol)
	}

	isBottom := jr.ctx.Level == tw.LevelBody && jr.ctx.Row.Location == tw.LocationEnd && !jr.ctx.HasFooter
	isFooter := jr.ctx.Level == tw.LevelFooter && jr.ctx.Row.Location == tw.LocationEnd
	if isBottom || isFooter {
		symbol := jr.sym.BottomRight()
		return jr.borderTint.Apply(symbol)
	}

	isVPassThruStrict := (mergeAbove.Vertical.Present && mergeBelow.Vertical.Present && !mergeAbove.Vertical.End && !mergeBelow.Vertical.Start) ||
		(mergeAbove.Hierarchical.Present && mergeBelow.Hierarchical.Present && !mergeAbove.Hierarchical.End && !mergeBelow.Hierarchical.Start)
	if isVPassThruStrict {
		symbol := jr.sym.Column()
		return jr.separatorTint.Apply(symbol)
	}

	symbol := jr.sym.MidRight()
	return jr.borderTint.Apply(symbol)
}

// RenderJunction selects and colors the junction symbol between two adjacent columns based on merge states and table position.
func (jr *Junction) RenderJunction(leftColIdx, rightColIdx int) string {
	mergeCurrentL := jr.getMergeState(jr.ctx.Row.Current, leftColIdx)
	mergeCurrentR := jr.getMergeState(jr.ctx.Row.Current, rightColIdx)
	mergeNextL := jr.getMergeState(jr.ctx.Row.Next, leftColIdx)
	mergeNextR := jr.getMergeState(jr.ctx.Row.Next, rightColIdx)

	isSpannedCurrent := mergeCurrentL.Horizontal.Present && !mergeCurrentL.Horizontal.End
	isSpannedNext := mergeNextL.Horizontal.Present && !mergeNextL.Horizontal.End

	vPassThruLStrict := (mergeCurrentL.Vertical.Present && mergeNextL.Vertical.Present && !mergeCurrentL.Vertical.End && !mergeNextL.Vertical.Start) ||
		(mergeCurrentL.Hierarchical.Present && mergeNextL.Hierarchical.Present && !mergeCurrentL.Hierarchical.End && !mergeNextL.Hierarchical.Start)
	vPassThruRStrict := (mergeCurrentR.Vertical.Present && mergeNextR.Vertical.Present && !mergeCurrentR.Vertical.End && !mergeNextR.Vertical.Start) ||
		(mergeCurrentR.Hierarchical.Present && mergeNextR.Hierarchical.Present && !mergeCurrentR.Hierarchical.End && !mergeNextR.Hierarchical.Start)

	isTop := (jr.ctx.Level == tw.LevelHeader && jr.ctx.Row.Location == tw.LocationFirst) ||
		(jr.ctx.Level == tw.LevelBody && jr.ctx.Row.Location == tw.LocationFirst && len(jr.ctx.Row.Previous) == 0)
	isBottom := (jr.ctx.Level == tw.LevelFooter && jr.ctx.Row.Location == tw.LocationEnd) ||
		(jr.ctx.Level == tw.LevelBody && jr.ctx.Row.Location == tw.LocationEnd && !jr.ctx.HasFooter)
	isPreFooter := jr.ctx.Level == tw.LevelFooter && (jr.ctx.Row.Position == tw.Row || jr.ctx.Row.Position == tw.Header)

	if isTop {
		if isSpannedNext {
			symbol := jr.sym.Row()
			return jr.borderTint.Apply(symbol)
		}
		symbol := jr.sym.TopMid()
		return jr.borderTint.Apply(symbol)
	}

	if isBottom {
		if vPassThruLStrict && vPassThruRStrict {
			symbol := jr.sym.Column()
			return jr.separatorTint.Apply(symbol)
		}
		if vPassThruLStrict {
			symbol := jr.sym.MidLeft()
			return jr.borderTint.Apply(symbol)
		}
		if vPassThruRStrict {
			symbol := jr.sym.MidRight()
			return jr.borderTint.Apply(symbol)
		}
		if isSpannedCurrent {
			symbol := jr.sym.Row()
			return jr.borderTint.Apply(symbol)
		}
		symbol := jr.sym.BottomMid()
		return jr.borderTint.Apply(symbol)
	}

	if isPreFooter {
		if vPassThruLStrict && vPassThruRStrict {
			symbol := jr.sym.Column()
			return jr.separatorTint.Apply(symbol)
		}
		if vPassThruLStrict {
			symbol := jr.sym.MidLeft()
			return jr.borderTint.Apply(symbol)
		}
		if vPassThruRStrict {
			symbol := jr.sym.MidRight()
			return jr.borderTint.Apply(symbol)
		}
		if mergeCurrentL.Horizontal.Present {
			if !mergeCurrentL.Horizontal.End && mergeCurrentR.Horizontal.Present && !mergeCurrentR.Horizontal.End {
				jr.logger.Debugf("Footer separator: H-merge continues from col %d to %d (mid-span), using BottomMid", leftColIdx, rightColIdx)
				symbol := jr.sym.BottomMid()
				return jr.borderTint.Apply(symbol)
			}
			if !mergeCurrentL.Horizontal.End && mergeCurrentR.Horizontal.Present && mergeCurrentR.Horizontal.End {
				jr.logger.Debugf("Footer separator: H-merge ends at col %d, using BottomMid", rightColIdx)
				symbol := jr.sym.BottomMid()
				return jr.borderTint.Apply(symbol)
			}
			if mergeCurrentL.Horizontal.End && !mergeCurrentR.Horizontal.Present {
				jr.logger.Debugf("Footer separator: H-merge ends at col %d, next col %d not merged, using Center", leftColIdx, rightColIdx)
				symbol := jr.sym.Center()
				return jr.borderTint.Apply(symbol)
			}
		}
		if isSpannedNext {
			symbol := jr.sym.BottomMid()
			return jr.borderTint.Apply(symbol)
		}
		if isSpannedCurrent {
			symbol := jr.sym.TopMid()
			return jr.borderTint.Apply(symbol)
		}
		symbol := jr.sym.Center()
		return jr.borderTint.Apply(symbol)
	}

	if vPassThruLStrict && vPassThruRStrict {
		symbol := jr.sym.Column()
		return jr.separatorTint.Apply(symbol)
	}
	if vPassThruLStrict {
		symbol := jr.sym.MidLeft()
		return jr.borderTint.Apply(symbol)
	}
	if vPassThruRStrict {
		symbol := jr.sym.MidRight()
		return jr.borderTint.Apply(symbol)
	}
	if isSpannedCurrent && isSpannedNext {
		symbol := jr.sym.Row()
		return jr.borderTint.Apply(symbol)
	}
	if isSpannedCurrent {
		symbol := jr.sym.TopMid()
		return jr.borderTint.Apply(symbol)
	}
	if isSpannedNext {
		symbol := jr.sym.BottomMid()
		return jr.borderTint.Apply(symbol)
	}

	symbol := jr.sym.Center()
	return jr.borderTint.Apply(symbol)
}
