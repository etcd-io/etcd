// Copyright 2010 The Freetype-Go Authors. All rights reserved.
// Use of this source code is governed by your choice of either the
// FreeType License or the GNU General Public License version 2 (or
// any later version), both of which can be found in the LICENSE file.

package raster

import (
	"fmt"
	"math"

	"golang.org/x/image/math/fixed"
)

// maxAbs returns the maximum of abs(a) and abs(b).
func maxAbs(a, b fixed.Int26_6) fixed.Int26_6 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if a < b {
		return b
	}
	return a
}

// pNeg returns the vector -p, or equivalently p rotated by 180 degrees.
func pNeg(p fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{-p.X, -p.Y}
}

// pDot returns the dot product p·q.
func pDot(p fixed.Point26_6, q fixed.Point26_6) fixed.Int52_12 {
	px, py := int64(p.X), int64(p.Y)
	qx, qy := int64(q.X), int64(q.Y)
	return fixed.Int52_12(px*qx + py*qy)
}

// pLen returns the length of the vector p.
func pLen(p fixed.Point26_6) fixed.Int26_6 {
	// TODO(nigeltao): use fixed point math.
	x := float64(p.X)
	y := float64(p.Y)
	return fixed.Int26_6(math.Sqrt(x*x + y*y))
}

// pNorm returns the vector p normalized to the given length, or zero if p is
// degenerate.
func pNorm(p fixed.Point26_6, length fixed.Int26_6) fixed.Point26_6 {
	d := pLen(p)
	if d == 0 {
		return fixed.Point26_6{}
	}
	s, t := int64(length), int64(d)
	x := int64(p.X) * s / t
	y := int64(p.Y) * s / t
	return fixed.Point26_6{fixed.Int26_6(x), fixed.Int26_6(y)}
}

// pRot45CW returns the vector p rotated clockwise by 45 degrees.
//
// Note that the Y-axis grows downwards, so {1, 0}.Rot45CW is {1/√2, 1/√2}.
func pRot45CW(p fixed.Point26_6) fixed.Point26_6 {
	// 181/256 is approximately 1/√2, or sin(π/4).
	px, py := int64(p.X), int64(p.Y)
	qx := (+px - py) * 181 / 256
	qy := (+px + py) * 181 / 256
	return fixed.Point26_6{fixed.Int26_6(qx), fixed.Int26_6(qy)}
}

// pRot90CW returns the vector p rotated clockwise by 90 degrees.
//
// Note that the Y-axis grows downwards, so {1, 0}.Rot90CW is {0, 1}.
func pRot90CW(p fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{-p.Y, p.X}
}

// pRot135CW returns the vector p rotated clockwise by 135 degrees.
//
// Note that the Y-axis grows downwards, so {1, 0}.Rot135CW is {-1/√2, 1/√2}.
func pRot135CW(p fixed.Point26_6) fixed.Point26_6 {
	// 181/256 is approximately 1/√2, or sin(π/4).
	px, py := int64(p.X), int64(p.Y)
	qx := (-px - py) * 181 / 256
	qy := (+px - py) * 181 / 256
	return fixed.Point26_6{fixed.Int26_6(qx), fixed.Int26_6(qy)}
}

// pRot45CCW returns the vector p rotated counter-clockwise by 45 degrees.
//
// Note that the Y-axis grows downwards, so {1, 0}.Rot45CCW is {1/√2, -1/√2}.
func pRot45CCW(p fixed.Point26_6) fixed.Point26_6 {
	// 181/256 is approximately 1/√2, or sin(π/4).
	px, py := int64(p.X), int64(p.Y)
	qx := (+px + py) * 181 / 256
	qy := (-px + py) * 181 / 256
	return fixed.Point26_6{fixed.Int26_6(qx), fixed.Int26_6(qy)}
}

// pRot90CCW returns the vector p rotated counter-clockwise by 90 degrees.
//
// Note that the Y-axis grows downwards, so {1, 0}.Rot90CCW is {0, -1}.
func pRot90CCW(p fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{p.Y, -p.X}
}

// pRot135CCW returns the vector p rotated counter-clockwise by 135 degrees.
//
// Note that the Y-axis grows downwards, so {1, 0}.Rot135CCW is {-1/√2, -1/√2}.
func pRot135CCW(p fixed.Point26_6) fixed.Point26_6 {
	// 181/256 is approximately 1/√2, or sin(π/4).
	px, py := int64(p.X), int64(p.Y)
	qx := (-px + py) * 181 / 256
	qy := (-px - py) * 181 / 256
	return fixed.Point26_6{fixed.Int26_6(qx), fixed.Int26_6(qy)}
}

// An Adder accumulates points on a curve.
type Adder interface {
	// Start starts a new curve at the given point.
	Start(a fixed.Point26_6)
	// Add1 adds a linear segment to the current curve.
	Add1(b fixed.Point26_6)
	// Add2 adds a quadratic segment to the current curve.
	Add2(b, c fixed.Point26_6)
	// Add3 adds a cubic segment to the current curve.
	Add3(b, c, d fixed.Point26_6)
}

// A Path is a sequence of curves, and a curve is a start point followed by a
// sequence of linear, quadratic or cubic segments.
type Path []fixed.Int26_6

// String returns a human-readable representation of a Path.
func (p Path) String() string {
	s := ""
	for i := 0; i < len(p); {
		if i != 0 {
			s += " "
		}
		switch p[i] {
		case 0:
			s += "S0" + fmt.Sprint([]fixed.Int26_6(p[i+1:i+3]))
			i += 4
		case 1:
			s += "A1" + fmt.Sprint([]fixed.Int26_6(p[i+1:i+3]))
			i += 4
		case 2:
			s += "A2" + fmt.Sprint([]fixed.Int26_6(p[i+1:i+5]))
			i += 6
		case 3:
			s += "A3" + fmt.Sprint([]fixed.Int26_6(p[i+1:i+7]))
			i += 8
		default:
			panic("freetype/raster: bad path")
		}
	}
	return s
}

// Clear cancels any previous calls to p.Start or p.AddXxx.
func (p *Path) Clear() {
	*p = (*p)[:0]
}

// Start starts a new curve at the given point.
func (p *Path) Start(a fixed.Point26_6) {
	*p = append(*p, 0, a.X, a.Y, 0)
}

// Add1 adds a linear segment to the current curve.
func (p *Path) Add1(b fixed.Point26_6) {
	*p = append(*p, 1, b.X, b.Y, 1)
}

// Add2 adds a quadratic segment to the current curve.
func (p *Path) Add2(b, c fixed.Point26_6) {
	*p = append(*p, 2, b.X, b.Y, c.X, c.Y, 2)
}

// Add3 adds a cubic segment to the current curve.
func (p *Path) Add3(b, c, d fixed.Point26_6) {
	*p = append(*p, 3, b.X, b.Y, c.X, c.Y, d.X, d.Y, 3)
}

// AddPath adds the Path q to p.
func (p *Path) AddPath(q Path) {
	*p = append(*p, q...)
}

// AddStroke adds a stroked Path.
func (p *Path) AddStroke(q Path, width fixed.Int26_6, cr Capper, jr Joiner) {
	Stroke(p, q, width, cr, jr)
}

// firstPoint returns the first point in a non-empty Path.
func (p Path) firstPoint() fixed.Point26_6 {
	return fixed.Point26_6{p[1], p[2]}
}

// lastPoint returns the last point in a non-empty Path.
func (p Path) lastPoint() fixed.Point26_6 {
	return fixed.Point26_6{p[len(p)-3], p[len(p)-2]}
}

// addPathReversed adds q reversed to p.
// For example, if q consists of a linear segment from A to B followed by a
// quadratic segment from B to C to D, then the values of q looks like:
// index: 01234567890123
// value: 0AA01BB12CCDD2
// So, when adding q backwards to p, we want to Add2(C, B) followed by Add1(A).
func addPathReversed(p Adder, q Path) {
	if len(q) == 0 {
		return
	}
	i := len(q) - 1
	for {
		switch q[i] {
		case 0:
			return
		case 1:
			i -= 4
			p.Add1(
				fixed.Point26_6{q[i-2], q[i-1]},
			)
		case 2:
			i -= 6
			p.Add2(
				fixed.Point26_6{q[i+2], q[i+3]},
				fixed.Point26_6{q[i-2], q[i-1]},
			)
		case 3:
			i -= 8
			p.Add3(
				fixed.Point26_6{q[i+4], q[i+5]},
				fixed.Point26_6{q[i+2], q[i+3]},
				fixed.Point26_6{q[i-2], q[i-1]},
			)
		default:
			panic("freetype/raster: bad path")
		}
	}
}
