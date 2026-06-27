// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bezier implements 2D Bézier curve calculation.
package bezier // import "gonum.org/v1/plot/tools/bezier"

import "gonum.org/v1/plot/vg"

type point struct {
	Point, Control vg.Point
}

// Curve implements Bezier curve calculation according to the algorithm of Robert D. Miller.
//
// Graphics Gems 5, 'Quick and Simple Bézier Curve Drawing', pages 206-209.
type Curve []point

// NewCurve returns a Curve initialized with the control points in cp.
func New(cp ...vg.Point) Curve {
	if len(cp) == 0 {
		return nil
	}
	c := make(Curve, len(cp))
	for i, p := range cp {
		c[i].Point = p
	}

	var w vg.Length
	for i, p := range c {
		switch i {
		case 0:
			w = 1
		case 1:
			w = vg.Length(len(c)) - 1
		default:
			w *= vg.Length(len(c)-i) / vg.Length(i)
		}
		c[i].Control.X = p.Point.X * w
		c[i].Control.Y = p.Point.Y * w
	}

	return c
}

// Point returns the point at t along the curve, where 0 ≤ t ≤ 1.
func (c Curve) Point(t float64) vg.Point {
	c[0].Point = c[0].Control
	u := t
	for i, p := range c[1:] {
		c[i+1].Point = vg.Point{
			X: p.Control.X * vg.Length(u),
			Y: p.Control.Y * vg.Length(u),
		}
		u *= t
	}

	var (
		t1 = 1 - t
		tt = t1
	)
	p := c[len(c)-1].Point
	for i := len(c) - 2; i >= 0; i-- {
		p.X += c[i].Point.X * vg.Length(tt)
		p.Y += c[i].Point.Y * vg.Length(tt)
		tt *= t1
	}

	return p
}

// Curve returns a slice of vg.Point, p, filled with points along the Bézier curve described by c.
// If the length of p is less than 2, the curve points are undefined. The length of p is not
// altered by the call.
func (c Curve) Curve(p []vg.Point) []vg.Point {
	for i, nf := 0, float64(len(p)-1); i < len(p); i++ {
		p[i] = c.Point(float64(i) / nf)
	}
	return p
}
