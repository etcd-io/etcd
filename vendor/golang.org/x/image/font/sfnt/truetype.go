// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sfnt

import (
	"golang.org/x/image/math/fixed"
)

// Flags for simple (non-compound) glyphs.
//
// See https://www.microsoft.com/typography/OTSPEC/glyf.htm
const (
	flagOnCurve      = 1 << 0 // 0x0001
	flagXShortVector = 1 << 1 // 0x0002
	flagYShortVector = 1 << 2 // 0x0004
	flagRepeat       = 1 << 3 // 0x0008

	// The same flag bits are overloaded to have two meanings, dependent on the
	// value of the flag{X,Y}ShortVector bits.
	flagPositiveXShortVector = 1 << 4 // 0x0010
	flagThisXIsSame          = 1 << 4 // 0x0010
	flagPositiveYShortVector = 1 << 5 // 0x0020
	flagThisYIsSame          = 1 << 5 // 0x0020
)

// Flags for compound glyphs.
//
// See https://www.microsoft.com/typography/OTSPEC/glyf.htm
const (
	flagArg1And2AreWords        = 1 << 0  // 0x0001
	flagArgsAreXYValues         = 1 << 1  // 0x0002
	flagRoundXYToGrid           = 1 << 2  // 0x0004
	flagWeHaveAScale            = 1 << 3  // 0x0008
	flagReserved4               = 1 << 4  // 0x0010
	flagMoreComponents          = 1 << 5  // 0x0020
	flagWeHaveAnXAndYScale      = 1 << 6  // 0x0040
	flagWeHaveATwoByTwo         = 1 << 7  // 0x0080
	flagWeHaveInstructions      = 1 << 8  // 0x0100
	flagUseMyMetrics            = 1 << 9  // 0x0200
	flagOverlapCompound         = 1 << 10 // 0x0400
	flagScaledComponentOffset   = 1 << 11 // 0x0800
	flagUnscaledComponentOffset = 1 << 12 // 0x1000
)

func midPoint(p, q fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{
		X: (p.X + q.X) / 2,
		Y: (p.Y + q.Y) / 2,
	}
}

func parseLoca(src *source, loca table, glyfOffset uint32, indexToLocFormat bool, numGlyphs int32) (locations []uint32, err error) {
	if indexToLocFormat {
		if loca.length != 4*uint32(numGlyphs+1) {
			return nil, errInvalidLocaTable
		}
	} else {
		if loca.length != 2*uint32(numGlyphs+1) {
			return nil, errInvalidLocaTable
		}
	}

	locations = make([]uint32, numGlyphs+1)
	buf, err := src.view(nil, int(loca.offset), int(loca.length))
	if err != nil {
		return nil, err
	}

	if indexToLocFormat {
		for i := range locations {
			locations[i] = 1*uint32(u32(buf[4*i:])) + glyfOffset
		}
	} else {
		for i := range locations {
			locations[i] = 2*uint32(u16(buf[2*i:])) + glyfOffset
		}
	}
	return locations, nil
}

// https://www.microsoft.com/typography/OTSPEC/glyf.htm says that "Each
// glyph begins with the following [10 byte] header".
const glyfHeaderLen = 10

func loadGlyf(f *Font, b *Buffer, x GlyphIndex, stackBottom, recursionDepth uint32) error {
	data, _, _, err := f.viewGlyphData(b, x)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if len(data) < glyfHeaderLen {
		return errInvalidGlyphData
	}
	index := glyfHeaderLen

	numContours, numPoints := int16(u16(data)), 0
	switch {
	case numContours == -1:
		// We have a compound glyph. No-op.
	case numContours == 0:
		return nil
	case numContours > 0:
		// We have a simple (non-compound) glyph.
		index += 2 * int(numContours)
		if index > len(data) {
			return errInvalidGlyphData
		}
		// The +1 for numPoints is because the value in the file format is
		// inclusive, but Go's slice[:index] semantics are exclusive.
		numPoints = 1 + int(u16(data[index-2:]))
	default:
		return errInvalidGlyphData
	}

	if numContours < 0 {
		return loadCompoundGlyf(f, b, data[glyfHeaderLen:], stackBottom, recursionDepth)
	}

	// Skip the hinting instructions.
	index += 2
	if index > len(data) {
		return errInvalidGlyphData
	}
	hintsLength := int(u16(data[index-2:]))
	index += hintsLength
	if index > len(data) {
		return errInvalidGlyphData
	}

	// For simple (non-compound) glyphs, the remainder of the glyf data
	// consists of (flags, x, y) points: the Bézier curve segments. These are
	// stored in columns (all the flags first, then all the x coordinates, then
	// all the y coordinates), not rows, as it compresses better.
	//
	// Decoding those points in row order involves two passes. The first pass
	// determines the indexes (relative to the data slice) of where the flags,
	// the x coordinates and the y coordinates each start.
	flagIndex := int32(index)
	xIndex, yIndex, ok := findXYIndexes(data, index, numPoints)
	if !ok {
		return errInvalidGlyphData
	}

	// The second pass decodes each (flags, x, y) tuple in row order.
	g := glyfIter{
		data:      data,
		flagIndex: flagIndex,
		xIndex:    xIndex,
		yIndex:    yIndex,
		endIndex:  glyfHeaderLen,
		// The -1 on prevEnd and finalEnd are because the contour-end index in
		// the file format is inclusive, but Go's slice[:index] is exclusive.
		prevEnd:     -1,
		finalEnd:    int32(numPoints - 1),
		numContours: int32(numContours),
	}
	for g.nextContour() {
		for g.nextSegment() {
			b.segments = append(b.segments, g.seg)
		}
	}
	return g.err
}

func findXYIndexes(data []byte, index, numPoints int) (xIndex, yIndex int32, ok bool) {
	xDataLen := 0
	yDataLen := 0
	for i := 0; ; {
		if i > numPoints {
			return 0, 0, false
		}
		if i == numPoints {
			break
		}

		repeatCount := 1
		if index >= len(data) {
			return 0, 0, false
		}
		flag := data[index]
		index++
		if flag&flagRepeat != 0 {
			if index >= len(data) {
				return 0, 0, false
			}
			repeatCount += int(data[index])
			index++
		}

		xSize := 0
		if flag&flagXShortVector != 0 {
			xSize = 1
		} else if flag&flagThisXIsSame == 0 {
			xSize = 2
		}
		xDataLen += xSize * repeatCount

		ySize := 0
		if flag&flagYShortVector != 0 {
			ySize = 1
		} else if flag&flagThisYIsSame == 0 {
			ySize = 2
		}
		yDataLen += ySize * repeatCount

		i += repeatCount
	}
	if index+xDataLen+yDataLen > len(data) {
		return 0, 0, false
	}
	return int32(index), int32(index + xDataLen), true
}

func loadCompoundGlyf(f *Font, b *Buffer, data []byte, stackBottom, recursionDepth uint32) error {
	if recursionDepth++; recursionDepth == maxCompoundRecursionDepth {
		return errUnsupportedCompoundGlyph
	}

	// Read and process the compound glyph's components. They are two separate
	// for loops, since reading parses the elements of the data slice, and
	// processing can overwrite the backing array.

	stackTop := stackBottom
	for {
		if stackTop >= maxCompoundStackSize {
			return errUnsupportedCompoundGlyph
		}
		elem := &b.compoundStack[stackTop]
		stackTop++

		if len(data) < 4 {
			return errInvalidGlyphData
		}
		flags := u16(data)
		elem.glyphIndex = GlyphIndex(u16(data[2:]))
		if flags&flagArg1And2AreWords == 0 {
			if len(data) < 6 {
				return errInvalidGlyphData
			}
			elem.dx = int16(int8(data[4]))
			elem.dy = int16(int8(data[5]))
			data = data[6:]
		} else {
			if len(data) < 8 {
				return errInvalidGlyphData
			}
			elem.dx = int16(u16(data[4:]))
			elem.dy = int16(u16(data[6:]))
			data = data[8:]
		}

		if flags&flagArgsAreXYValues == 0 {
			return errUnsupportedCompoundGlyph
		}
		elem.hasTransform = flags&(flagWeHaveAScale|flagWeHaveAnXAndYScale|flagWeHaveATwoByTwo) != 0
		if elem.hasTransform {
			switch {
			case flags&flagWeHaveAScale != 0:
				if len(data) < 2 {
					return errInvalidGlyphData
				}
				elem.transformXX = int16(u16(data))
				elem.transformXY = 0
				elem.transformYX = 0
				elem.transformYY = elem.transformXX
				data = data[2:]
			case flags&flagWeHaveAnXAndYScale != 0:
				if len(data) < 4 {
					return errInvalidGlyphData
				}
				elem.transformXX = int16(u16(data[0:]))
				elem.transformXY = 0
				elem.transformYX = 0
				elem.transformYY = int16(u16(data[2:]))
				data = data[4:]
			case flags&flagWeHaveATwoByTwo != 0:
				if len(data) < 8 {
					return errInvalidGlyphData
				}
				elem.transformXX = int16(u16(data[0:]))
				elem.transformXY = int16(u16(data[2:]))
				elem.transformYX = int16(u16(data[4:]))
				elem.transformYY = int16(u16(data[6:]))
				data = data[8:]
			}
		}

		if flags&flagMoreComponents == 0 {
			break
		}
	}

	// To support hinting, we'd have to save the remaining bytes in data here
	// and interpret them after the for loop below, since that for loop's
	// loadGlyf calls can overwrite the backing array.

	for i := stackBottom; i < stackTop; i++ {
		elem := &b.compoundStack[i]
		base := len(b.segments)
		if err := loadGlyf(f, b, elem.glyphIndex, stackTop, recursionDepth); err != nil {
			return err
		}
		dx, dy := fixed.Int26_6(elem.dx), fixed.Int26_6(elem.dy)
		segments := b.segments[base:]
		if elem.hasTransform {
			txx := elem.transformXX
			txy := elem.transformXY
			tyx := elem.transformYX
			tyy := elem.transformYY
			for j := range segments {
				transformArgs(&segments[j].Args, txx, txy, tyx, tyy, dx, dy)
			}
		} else {
			for j := range segments {
				translateArgs(&segments[j].Args, dx, dy)
			}
		}
	}

	return nil
}

type glyfIter struct {
	data []byte
	err  error

	// Various indices into the data slice. See the "Decoding those points in
	// row order" comment above.
	flagIndex int32
	xIndex    int32
	yIndex    int32

	// endIndex points to the uint16 that is the inclusive point index of the
	// current contour's end. prevEnd is the previous contour's end. finalEnd
	// should match the final contour's end.
	endIndex int32
	prevEnd  int32
	finalEnd int32

	// c and p count the current contour and point, up to numContours and
	// numPoints.
	c, numContours int32
	p, nPoints     int32

	// The next two groups of fields track points and segments. Points are what
	// the underlying file format provides. Bézier curve segments are what the
	// rasterizer consumes.
	//
	// Points are either on-curve or off-curve. Two consecutive on-curve points
	// define a linear curve segment between them. N off-curve points between
	// on-curve points define N quadratic curve segments. The TrueType glyf
	// format does not use cubic curves. If N is greater than 1, some of these
	// segment end points are implicit, the midpoint of two off-curve points.
	// Given the points A, B1, B2, ..., BN, C, where A and C are on-curve and
	// all the Bs are off-curve, the segments are:
	//
	//	- A,                  B1, midpoint(B1, B2)
	//	- midpoint(B1, B2),   B2, midpoint(B2, B3)
	//	- midpoint(B2, B3),   B3, midpoint(B3, B4)
	//	- ...
	//	- midpoint(BN-1, BN), BN, C
	//
	// Note that the sequence of Bs may wrap around from the last point in the
	// glyf data to the first. A and C may also be the same point (the only
	// explicit on-curve point), or there may be no explicit on-curve points at
	// all (but still implicit ones between explicit off-curve points).

	// Points.
	x, y    int16
	on      bool
	flag    uint8
	repeats uint8

	// Segments.
	closing            bool
	closed             bool
	firstOnCurveValid  bool
	firstOffCurveValid bool
	lastOffCurveValid  bool
	firstOnCurve       fixed.Point26_6
	firstOffCurve      fixed.Point26_6
	lastOffCurve       fixed.Point26_6
	seg                Segment
}

func (g *glyfIter) nextContour() (ok bool) {
	if g.c == g.numContours {
		if g.prevEnd != g.finalEnd {
			g.err = errInvalidGlyphData
		}
		return false
	}
	g.c++

	end := int32(u16(g.data[g.endIndex:]))
	g.endIndex += 2
	if (end <= g.prevEnd) || (g.finalEnd < end) {
		g.err = errInvalidGlyphData
		return false
	}
	g.nPoints = end - g.prevEnd
	g.p = 0
	g.prevEnd = end

	g.closing = false
	g.closed = false
	g.firstOnCurveValid = false
	g.firstOffCurveValid = false
	g.lastOffCurveValid = false

	return true
}

func (g *glyfIter) close() {
	switch {
	case !g.firstOffCurveValid && !g.lastOffCurveValid:
		g.closed = true
		g.seg = Segment{
			Op:   SegmentOpLineTo,
			Args: [3]fixed.Point26_6{g.firstOnCurve},
		}
	case !g.firstOffCurveValid && g.lastOffCurveValid:
		g.closed = true
		g.seg = Segment{
			Op:   SegmentOpQuadTo,
			Args: [3]fixed.Point26_6{g.lastOffCurve, g.firstOnCurve},
		}
	case g.firstOffCurveValid && !g.lastOffCurveValid:
		g.closed = true
		g.seg = Segment{
			Op:   SegmentOpQuadTo,
			Args: [3]fixed.Point26_6{g.firstOffCurve, g.firstOnCurve},
		}
	case g.firstOffCurveValid && g.lastOffCurveValid:
		g.lastOffCurveValid = false
		g.seg = Segment{
			Op: SegmentOpQuadTo,
			Args: [3]fixed.Point26_6{
				g.lastOffCurve,
				midPoint(g.lastOffCurve, g.firstOffCurve),
			},
		}
	}
}

func (g *glyfIter) nextSegment() (ok bool) {
	for !g.closed {
		if g.closing || !g.nextPoint() {
			g.closing = true
			g.close()
			return true
		}

		// Convert the tuple (g.x, g.y) to a fixed.Point26_6, since the latter
		// is what's held in a Segment. The input (g.x, g.y) is a pair of int16
		// values, measured in font units, since that is what the underlying
		// format provides. The output is a pair of fixed.Int26_6 values. A
		// fixed.Int26_6 usually represents a 26.6 fixed number of pixels, but
		// this here is just a straight numerical conversion, with no scaling
		// factor. A later step scales the Segment.Args values by such a factor
		// to convert e.g. 1792 font units to 10.5 pixels at 2048 font units
		// per em and 12 ppem (pixels per em).
		p := fixed.Point26_6{
			X: fixed.Int26_6(g.x),
			Y: fixed.Int26_6(g.y),
		}

		if !g.firstOnCurveValid {
			if g.on {
				g.firstOnCurve = p
				g.firstOnCurveValid = true
				g.seg = Segment{
					Op:   SegmentOpMoveTo,
					Args: [3]fixed.Point26_6{p},
				}
				return true
			} else if !g.firstOffCurveValid {
				g.firstOffCurve = p
				g.firstOffCurveValid = true
				continue
			} else {
				g.firstOnCurve = midPoint(g.firstOffCurve, p)
				g.firstOnCurveValid = true
				g.lastOffCurve = p
				g.lastOffCurveValid = true
				g.seg = Segment{
					Op:   SegmentOpMoveTo,
					Args: [3]fixed.Point26_6{g.firstOnCurve},
				}
				return true
			}

		} else if !g.lastOffCurveValid {
			if !g.on {
				g.lastOffCurve = p
				g.lastOffCurveValid = true
				continue
			} else {
				g.seg = Segment{
					Op:   SegmentOpLineTo,
					Args: [3]fixed.Point26_6{p},
				}
				return true
			}

		} else {
			if !g.on {
				g.seg = Segment{
					Op: SegmentOpQuadTo,
					Args: [3]fixed.Point26_6{
						g.lastOffCurve,
						midPoint(g.lastOffCurve, p),
					},
				}
				g.lastOffCurve = p
				g.lastOffCurveValid = true
				return true
			} else {
				g.seg = Segment{
					Op:   SegmentOpQuadTo,
					Args: [3]fixed.Point26_6{g.lastOffCurve, p},
				}
				g.lastOffCurveValid = false
				return true
			}
		}
	}
	return false
}

func (g *glyfIter) nextPoint() (ok bool) {
	if g.p == g.nPoints {
		return false
	}
	g.p++

	if g.repeats > 0 {
		g.repeats--
	} else {
		g.flag = g.data[g.flagIndex]
		g.flagIndex++
		if g.flag&flagRepeat != 0 {
			g.repeats = g.data[g.flagIndex]
			g.flagIndex++
		}
	}

	if g.flag&flagXShortVector != 0 {
		if g.flag&flagPositiveXShortVector != 0 {
			g.x += int16(g.data[g.xIndex])
		} else {
			g.x -= int16(g.data[g.xIndex])
		}
		g.xIndex += 1
	} else if g.flag&flagThisXIsSame == 0 {
		g.x += int16(u16(g.data[g.xIndex:]))
		g.xIndex += 2
	}

	if g.flag&flagYShortVector != 0 {
		if g.flag&flagPositiveYShortVector != 0 {
			g.y += int16(g.data[g.yIndex])
		} else {
			g.y -= int16(g.data[g.yIndex])
		}
		g.yIndex += 1
	} else if g.flag&flagThisYIsSame == 0 {
		g.y += int16(u16(g.data[g.yIndex:]))
		g.yIndex += 2
	}

	g.on = g.flag&flagOnCurve != 0
	return true
}
