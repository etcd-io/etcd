// Copyright ©2022 The gg Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gg

import (
	"math"

	"golang.org/x/image/math/fixed"
)

type Point struct {
	X, Y float64
}

func (a Point) Fixed() fixed.Point26_6 {
	return fixp(a.X, a.Y)
}

func (a Point) Distance(b Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func (a Point) Interpolate(b Point, t float64) Point {
	x := a.X + (b.X-a.X)*t
	y := a.Y + (b.Y-a.Y)*t
	return Point{x, y}
}
