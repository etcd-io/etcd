// Copyright 2010 The Freetype-Go Authors. All rights reserved.
// Use of this source code is governed by your choice of either the
// FreeType License or the GNU General Public License version 2 (or
// any later version), both of which can be found in the LICENSE file.

package raster

import (
	"golang.org/x/image/math/fixed"
)

// Two points are considered practically equal if the square of the distance
// between them is less than one quarter (i.e. 1024 / 4096).
const epsilon = fixed.Int52_12(1024)

// A Capper signifies how to begin or end a stroked path.
type Capper interface {
	// Cap adds a cap to p given a pivot point and the normal vector of a
	// terminal segment. The normal's length is half of the stroke width.
	Cap(p Adder, halfWidth fixed.Int26_6, pivot, n1 fixed.Point26_6)
}

// The CapperFunc type adapts an ordinary function to be a Capper.
type CapperFunc func(Adder, fixed.Int26_6, fixed.Point26_6, fixed.Point26_6)

func (f CapperFunc) Cap(p Adder, halfWidth fixed.Int26_6, pivot, n1 fixed.Point26_6) {
	f(p, halfWidth, pivot, n1)
}

// A Joiner signifies how to join interior nodes of a stroked path.
type Joiner interface {
	// Join adds a join to the two sides of a stroked path given a pivot
	// point and the normal vectors of the trailing and leading segments.
	// Both normals have length equal to half of the stroke width.
	Join(lhs, rhs Adder, halfWidth fixed.Int26_6, pivot, n0, n1 fixed.Point26_6)
}

// The JoinerFunc type adapts an ordinary function to be a Joiner.
type JoinerFunc func(lhs, rhs Adder, halfWidth fixed.Int26_6, pivot, n0, n1 fixed.Point26_6)

func (f JoinerFunc) Join(lhs, rhs Adder, halfWidth fixed.Int26_6, pivot, n0, n1 fixed.Point26_6) {
	f(lhs, rhs, halfWidth, pivot, n0, n1)
}

// RoundCapper adds round caps to a stroked path.
var RoundCapper Capper = CapperFunc(roundCapper)

func roundCapper(p Adder, halfWidth fixed.Int26_6, pivot, n1 fixed.Point26_6) {
	// The cubic Bézier approximation to a circle involves the magic number
	// (√2 - 1) * 4/3, which is approximately 35/64.
	const k = 35
	e := pRot90CCW(n1)
	side := pivot.Add(e)
	start, end := pivot.Sub(n1), pivot.Add(n1)
	d, e := n1.Mul(k), e.Mul(k)
	p.Add3(start.Add(e), side.Sub(d), side)
	p.Add3(side.Add(d), end.Add(e), end)
}

// ButtCapper adds butt caps to a stroked path.
var ButtCapper Capper = CapperFunc(buttCapper)

func buttCapper(p Adder, halfWidth fixed.Int26_6, pivot, n1 fixed.Point26_6) {
	p.Add1(pivot.Add(n1))
}

// SquareCapper adds square caps to a stroked path.
var SquareCapper Capper = CapperFunc(squareCapper)

func squareCapper(p Adder, halfWidth fixed.Int26_6, pivot, n1 fixed.Point26_6) {
	e := pRot90CCW(n1)
	side := pivot.Add(e)
	p.Add1(side.Sub(n1))
	p.Add1(side.Add(n1))
	p.Add1(pivot.Add(n1))
}

// RoundJoiner adds round joins to a stroked path.
var RoundJoiner Joiner = JoinerFunc(roundJoiner)

func roundJoiner(lhs, rhs Adder, haflWidth fixed.Int26_6, pivot, n0, n1 fixed.Point26_6) {
	dot := pDot(pRot90CW(n0), n1)
	if dot >= 0 {
		addArc(lhs, pivot, n0, n1)
		rhs.Add1(pivot.Sub(n1))
	} else {
		lhs.Add1(pivot.Add(n1))
		addArc(rhs, pivot, pNeg(n0), pNeg(n1))
	}
}

// BevelJoiner adds bevel joins to a stroked path.
var BevelJoiner Joiner = JoinerFunc(bevelJoiner)

func bevelJoiner(lhs, rhs Adder, haflWidth fixed.Int26_6, pivot, n0, n1 fixed.Point26_6) {
	lhs.Add1(pivot.Add(n1))
	rhs.Add1(pivot.Sub(n1))
}

// addArc adds a circular arc from pivot+n0 to pivot+n1 to p. The shorter of
// the two possible arcs is taken, i.e. the one spanning <= 180 degrees. The
// two vectors n0 and n1 must be of equal length.
func addArc(p Adder, pivot, n0, n1 fixed.Point26_6) {
	// r2 is the square of the length of n0.
	r2 := pDot(n0, n0)
	if r2 < epsilon {
		// The arc radius is so small that we collapse to a straight line.
		p.Add1(pivot.Add(n1))
		return
	}
	// We approximate the arc by 0, 1, 2 or 3 45-degree quadratic segments plus
	// a final quadratic segment from s to n1. Each 45-degree segment has
	// control points {1, 0}, {1, tan(π/8)} and {1/√2, 1/√2} suitably scaled,
	// rotated and translated. tan(π/8) is approximately 27/64.
	const tpo8 = 27
	var s fixed.Point26_6
	// We determine which octant the angle between n0 and n1 is in via three
	// dot products. m0, m1 and m2 are n0 rotated clockwise by 45, 90 and 135
	// degrees.
	m0 := pRot45CW(n0)
	m1 := pRot90CW(n0)
	m2 := pRot90CW(m0)
	if pDot(m1, n1) >= 0 {
		if pDot(n0, n1) >= 0 {
			if pDot(m2, n1) <= 0 {
				// n1 is between 0 and 45 degrees clockwise of n0.
				s = n0
			} else {
				// n1 is between 45 and 90 degrees clockwise of n0.
				p.Add2(pivot.Add(n0).Add(m1.Mul(tpo8)), pivot.Add(m0))
				s = m0
			}
		} else {
			pm1, n0t := pivot.Add(m1), n0.Mul(tpo8)
			p.Add2(pivot.Add(n0).Add(m1.Mul(tpo8)), pivot.Add(m0))
			p.Add2(pm1.Add(n0t), pm1)
			if pDot(m0, n1) >= 0 {
				// n1 is between 90 and 135 degrees clockwise of n0.
				s = m1
			} else {
				// n1 is between 135 and 180 degrees clockwise of n0.
				p.Add2(pm1.Sub(n0t), pivot.Add(m2))
				s = m2
			}
		}
	} else {
		if pDot(n0, n1) >= 0 {
			if pDot(m0, n1) >= 0 {
				// n1 is between 0 and 45 degrees counter-clockwise of n0.
				s = n0
			} else {
				// n1 is between 45 and 90 degrees counter-clockwise of n0.
				p.Add2(pivot.Add(n0).Sub(m1.Mul(tpo8)), pivot.Sub(m2))
				s = pNeg(m2)
			}
		} else {
			pm1, n0t := pivot.Sub(m1), n0.Mul(tpo8)
			p.Add2(pivot.Add(n0).Sub(m1.Mul(tpo8)), pivot.Sub(m2))
			p.Add2(pm1.Add(n0t), pm1)
			if pDot(m2, n1) <= 0 {
				// n1 is between 90 and 135 degrees counter-clockwise of n0.
				s = pNeg(m1)
			} else {
				// n1 is between 135 and 180 degrees counter-clockwise of n0.
				p.Add2(pm1.Sub(n0t), pivot.Sub(m0))
				s = pNeg(m0)
			}
		}
	}
	// The final quadratic segment has two endpoints s and n1 and the middle
	// control point is a multiple of s.Add(n1), i.e. it is on the angle
	// bisector of those two points. The multiple ranges between 128/256 and
	// 150/256 as the angle between s and n1 ranges between 0 and 45 degrees.
	//
	// When the angle is 0 degrees (i.e. s and n1 are coincident) then
	// s.Add(n1) is twice s and so the middle control point of the degenerate
	// quadratic segment should be half s.Add(n1), and half = 128/256.
	//
	// When the angle is 45 degrees then 150/256 is the ratio of the lengths of
	// the two vectors {1, tan(π/8)} and {1 + 1/√2, 1/√2}.
	//
	// d is the normalized dot product between s and n1. Since the angle ranges
	// between 0 and 45 degrees then d ranges between 256/256 and 181/256.
	d := 256 * pDot(s, n1) / r2
	multiple := fixed.Int26_6(150-(150-128)*(d-181)/(256-181)) >> 2
	p.Add2(pivot.Add(s.Add(n1).Mul(multiple)), pivot.Add(n1))
}

// midpoint returns the midpoint of two Points.
func midpoint(a, b fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{(a.X + b.X) / 2, (a.Y + b.Y) / 2}
}

// angleGreaterThan45 returns whether the angle between two vectors is more
// than 45 degrees.
func angleGreaterThan45(v0, v1 fixed.Point26_6) bool {
	v := pRot45CCW(v0)
	return pDot(v, v1) < 0 || pDot(pRot90CW(v), v1) < 0
}

// interpolate returns the point (1-t)*a + t*b.
func interpolate(a, b fixed.Point26_6, t fixed.Int52_12) fixed.Point26_6 {
	s := 1<<12 - t
	x := s*fixed.Int52_12(a.X) + t*fixed.Int52_12(b.X)
	y := s*fixed.Int52_12(a.Y) + t*fixed.Int52_12(b.Y)
	return fixed.Point26_6{fixed.Int26_6(x >> 12), fixed.Int26_6(y >> 12)}
}

// curviest2 returns the value of t for which the quadratic parametric curve
// (1-t)²*a + 2*t*(1-t).b + t²*c has maximum curvature.
//
// The curvature of the parametric curve f(t) = (x(t), y(t)) is
// |x′y″-y′x″| / (x′²+y′²)^(3/2).
//
// Let d = b-a and e = c-2*b+a, so that f′(t) = 2*d+2*e*t and f″(t) = 2*e.
// The curvature's numerator is (2*dx+2*ex*t)*(2*ey)-(2*dy+2*ey*t)*(2*ex),
// which simplifies to 4*dx*ey-4*dy*ex, which is constant with respect to t.
//
// Thus, curvature is extreme where the denominator is extreme, i.e. where
// (x′²+y′²) is extreme. The first order condition is that
// 2*x′*x″+2*y′*y″ = 0, or (dx+ex*t)*ex + (dy+ey*t)*ey = 0.
// Solving for t gives t = -(dx*ex+dy*ey) / (ex*ex+ey*ey).
func curviest2(a, b, c fixed.Point26_6) fixed.Int52_12 {
	dx := int64(b.X - a.X)
	dy := int64(b.Y - a.Y)
	ex := int64(c.X - 2*b.X + a.X)
	ey := int64(c.Y - 2*b.Y + a.Y)
	if ex == 0 && ey == 0 {
		return 2048
	}
	return fixed.Int52_12(-4096 * (dx*ex + dy*ey) / (ex*ex + ey*ey))
}

// A stroker holds state for stroking a path.
type stroker struct {
	// p is the destination that records the stroked path.
	p Adder
	// u is the half-width of the stroke.
	u fixed.Int26_6
	// cr and jr specify how to end and connect path segments.
	cr Capper
	jr Joiner
	// r is the reverse path. Stroking a path involves constructing two
	// parallel paths 2*u apart. The first path is added immediately to p,
	// the second path is accumulated in r and eventually added in reverse.
	r Path
	// a is the most recent segment point. anorm is the segment normal of
	// length u at that point.
	a, anorm fixed.Point26_6
}

// addNonCurvy2 adds a quadratic segment to the stroker, where the segment
// defined by (k.a, b, c) achieves maximum curvature at either k.a or c.
func (k *stroker) addNonCurvy2(b, c fixed.Point26_6) {
	// We repeatedly divide the segment at its middle until it is straight
	// enough to approximate the stroke by just translating the control points.
	// ds and ps are stacks of depths and points. t is the top of the stack.
	const maxDepth = 5
	var (
		ds [maxDepth + 1]int
		ps [2*maxDepth + 3]fixed.Point26_6
		t  int
	)
	// Initially the ps stack has one quadratic segment of depth zero.
	ds[0] = 0
	ps[2] = k.a
	ps[1] = b
	ps[0] = c
	anorm := k.anorm
	var cnorm fixed.Point26_6

	for {
		depth := ds[t]
		a := ps[2*t+2]
		b := ps[2*t+1]
		c := ps[2*t+0]
		ab := b.Sub(a)
		bc := c.Sub(b)
		abIsSmall := pDot(ab, ab) < fixed.Int52_12(1<<12)
		bcIsSmall := pDot(bc, bc) < fixed.Int52_12(1<<12)
		if abIsSmall && bcIsSmall {
			// Approximate the segment by a circular arc.
			cnorm = pRot90CCW(pNorm(bc, k.u))
			mac := midpoint(a, c)
			addArc(k.p, mac, anorm, cnorm)
			addArc(&k.r, mac, pNeg(anorm), pNeg(cnorm))
		} else if depth < maxDepth && angleGreaterThan45(ab, bc) {
			// Divide the segment in two and push both halves on the stack.
			mab := midpoint(a, b)
			mbc := midpoint(b, c)
			t++
			ds[t+0] = depth + 1
			ds[t-1] = depth + 1
			ps[2*t+2] = a
			ps[2*t+1] = mab
			ps[2*t+0] = midpoint(mab, mbc)
			ps[2*t-1] = mbc
			continue
		} else {
			// Translate the control points.
			bnorm := pRot90CCW(pNorm(c.Sub(a), k.u))
			cnorm = pRot90CCW(pNorm(bc, k.u))
			k.p.Add2(b.Add(bnorm), c.Add(cnorm))
			k.r.Add2(b.Sub(bnorm), c.Sub(cnorm))
		}
		if t == 0 {
			k.a, k.anorm = c, cnorm
			return
		}
		t--
		anorm = cnorm
	}
	panic("unreachable")
}

// Add1 adds a linear segment to the stroker.
func (k *stroker) Add1(b fixed.Point26_6) {
	bnorm := pRot90CCW(pNorm(b.Sub(k.a), k.u))
	if len(k.r) == 0 {
		k.p.Start(k.a.Add(bnorm))
		k.r.Start(k.a.Sub(bnorm))
	} else {
		k.jr.Join(k.p, &k.r, k.u, k.a, k.anorm, bnorm)
	}
	k.p.Add1(b.Add(bnorm))
	k.r.Add1(b.Sub(bnorm))
	k.a, k.anorm = b, bnorm
}

// Add2 adds a quadratic segment to the stroker.
func (k *stroker) Add2(b, c fixed.Point26_6) {
	ab := b.Sub(k.a)
	bc := c.Sub(b)
	abnorm := pRot90CCW(pNorm(ab, k.u))
	if len(k.r) == 0 {
		k.p.Start(k.a.Add(abnorm))
		k.r.Start(k.a.Sub(abnorm))
	} else {
		k.jr.Join(k.p, &k.r, k.u, k.a, k.anorm, abnorm)
	}

	// Approximate nearly-degenerate quadratics by linear segments.
	abIsSmall := pDot(ab, ab) < epsilon
	bcIsSmall := pDot(bc, bc) < epsilon
	if abIsSmall || bcIsSmall {
		acnorm := pRot90CCW(pNorm(c.Sub(k.a), k.u))
		k.p.Add1(c.Add(acnorm))
		k.r.Add1(c.Sub(acnorm))
		k.a, k.anorm = c, acnorm
		return
	}

	// The quadratic segment (k.a, b, c) has a point of maximum curvature.
	// If this occurs at an end point, we process the segment as a whole.
	t := curviest2(k.a, b, c)
	if t <= 0 || 4096 <= t {
		k.addNonCurvy2(b, c)
		return
	}

	// Otherwise, we perform a de Casteljau decomposition at the point of
	// maximum curvature and process the two straighter parts.
	mab := interpolate(k.a, b, t)
	mbc := interpolate(b, c, t)
	mabc := interpolate(mab, mbc, t)

	// If the vectors ab and bc are close to being in opposite directions,
	// then the decomposition can become unstable, so we approximate the
	// quadratic segment by two linear segments joined by an arc.
	bcnorm := pRot90CCW(pNorm(bc, k.u))
	if pDot(abnorm, bcnorm) < -fixed.Int52_12(k.u)*fixed.Int52_12(k.u)*2047/2048 {
		pArc := pDot(abnorm, bc) < 0

		k.p.Add1(mabc.Add(abnorm))
		if pArc {
			z := pRot90CW(abnorm)
			addArc(k.p, mabc, abnorm, z)
			addArc(k.p, mabc, z, bcnorm)
		}
		k.p.Add1(mabc.Add(bcnorm))
		k.p.Add1(c.Add(bcnorm))

		k.r.Add1(mabc.Sub(abnorm))
		if !pArc {
			z := pRot90CW(abnorm)
			addArc(&k.r, mabc, pNeg(abnorm), z)
			addArc(&k.r, mabc, z, pNeg(bcnorm))
		}
		k.r.Add1(mabc.Sub(bcnorm))
		k.r.Add1(c.Sub(bcnorm))

		k.a, k.anorm = c, bcnorm
		return
	}

	// Process the decomposed parts.
	k.addNonCurvy2(mab, mabc)
	k.addNonCurvy2(mbc, c)
}

// Add3 adds a cubic segment to the stroker.
func (k *stroker) Add3(b, c, d fixed.Point26_6) {
	panic("freetype/raster: stroke unimplemented for cubic segments")
}

// stroke adds the stroked Path q to p, where q consists of exactly one curve.
func (k *stroker) stroke(q Path) {
	// Stroking is implemented by deriving two paths each k.u apart from q.
	// The left-hand-side path is added immediately to k.p; the right-hand-side
	// path is accumulated in k.r. Once we've finished adding the LHS to k.p,
	// we add the RHS in reverse order.
	k.r = make(Path, 0, len(q))
	k.a = fixed.Point26_6{q[1], q[2]}
	for i := 4; i < len(q); {
		switch q[i] {
		case 1:
			k.Add1(
				fixed.Point26_6{q[i+1], q[i+2]},
			)
			i += 4
		case 2:
			k.Add2(
				fixed.Point26_6{q[i+1], q[i+2]},
				fixed.Point26_6{q[i+3], q[i+4]},
			)
			i += 6
		case 3:
			k.Add3(
				fixed.Point26_6{q[i+1], q[i+2]},
				fixed.Point26_6{q[i+3], q[i+4]},
				fixed.Point26_6{q[i+5], q[i+6]},
			)
			i += 8
		default:
			panic("freetype/raster: bad path")
		}
	}
	if len(k.r) == 0 {
		return
	}
	// TODO(nigeltao): if q is a closed curve then we should join the first and
	// last segments instead of capping them.
	k.cr.Cap(k.p, k.u, q.lastPoint(), pNeg(k.anorm))
	addPathReversed(k.p, k.r)
	pivot := q.firstPoint()
	k.cr.Cap(k.p, k.u, pivot, pivot.Sub(fixed.Point26_6{k.r[1], k.r[2]}))
}

// Stroke adds q stroked with the given width to p. The result is typically
// self-intersecting and should be rasterized with UseNonZeroWinding.
// cr and jr may be nil, which defaults to a RoundCapper or RoundJoiner.
func Stroke(p Adder, q Path, width fixed.Int26_6, cr Capper, jr Joiner) {
	if len(q) == 0 {
		return
	}
	if cr == nil {
		cr = RoundCapper
	}
	if jr == nil {
		jr = RoundJoiner
	}
	if q[0] != 0 {
		panic("freetype/raster: bad path")
	}
	s := stroker{p: p, u: width / 2, cr: cr, jr: jr}
	i := 0
	for j := 4; j < len(q); {
		switch q[j] {
		case 0:
			s.stroke(q[i:j])
			i, j = j, j+4
		case 1:
			j += 4
		case 2:
			j += 6
		case 3:
			j += 8
		default:
			panic("freetype/raster: bad path")
		}
	}
	s.stroke(q[i:])
}
