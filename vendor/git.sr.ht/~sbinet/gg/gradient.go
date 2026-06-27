// Copyright ©2022 The gg Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gg

import (
	"image/color"
	"math"
	"sort"
)

type stop struct {
	pos   float64
	color color.Color
}

type stops []stop

// Len satisfies the Sort interface.
func (s stops) Len() int {
	return len(s)
}

// Less satisfies the Sort interface.
func (s stops) Less(i, j int) bool {
	return s[i].pos < s[j].pos
}

// Swap satisfies the Sort interface.
func (s stops) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type Gradient interface {
	Pattern
	AddColorStop(offset float64, color color.Color)
}

// Linear Gradient
type linearGradient struct {
	x0, y0, x1, y1 float64
	stops          stops
}

func (g *linearGradient) ColorAt(x, y int) color.Color {
	if len(g.stops) == 0 {
		return color.Transparent
	}

	fx, fy := float64(x), float64(y)
	x0, y0, x1, y1 := g.x0, g.y0, g.x1, g.y1
	dx, dy := x1-x0, y1-y0

	// Horizontal
	if dy == 0 && dx != 0 {
		return getColor((fx-x0)/dx, g.stops)
	}

	// Vertical
	if dx == 0 && dy != 0 {
		return getColor((fy-y0)/dy, g.stops)
	}

	// Dot product
	s0 := dx*(fx-x0) + dy*(fy-y0)
	if s0 < 0 {
		return g.stops[0].color
	}
	// Calculate distance to (x0,y0) alone (x0,y0)->(x1,y1)
	mag := math.Hypot(dx, dy)
	u := ((fx-x0)*-dy + (fy-y0)*dx) / (mag * mag)
	x2, y2 := x0+u*-dy, y0+u*dx
	d := math.Hypot(fx-x2, fy-y2) / mag
	return getColor(d, g.stops)
}

func (g *linearGradient) AddColorStop(offset float64, color color.Color) {
	g.stops = append(g.stops, stop{pos: offset, color: color})
	sort.Sort(g.stops)
}

func NewLinearGradient(x0, y0, x1, y1 float64) Gradient {
	g := &linearGradient{
		x0: x0, y0: y0,
		x1: x1, y1: y1,
	}
	return g
}

// Radial Gradient
type circle struct {
	x, y, r float64
}

type radialGradient struct {
	c0, c1, cd circle
	a, inva    float64
	mindr      float64
	stops      stops
}

func dot3(x0, y0, z0, x1, y1, z1 float64) float64 {
	return x0*x1 + y0*y1 + z0*z1
}

func (g *radialGradient) ColorAt(x, y int) color.Color {
	if len(g.stops) == 0 {
		return color.Transparent
	}

	// copy from pixman's pixman-radial-gradient.c

	dx, dy := float64(x)+0.5-g.c0.x, float64(y)+0.5-g.c0.y
	b := dot3(dx, dy, g.c0.r, g.cd.x, g.cd.y, g.cd.r)
	c := dot3(dx, dy, -g.c0.r, dx, dy, g.c0.r)

	if g.a == 0 {
		if b == 0 {
			return color.Transparent
		}
		t := 0.5 * c / b
		if t*g.cd.r >= g.mindr {
			return getColor(t, g.stops)
		}
		return color.Transparent
	}

	discr := dot3(b, g.a, 0, b, -c, 0)
	if discr >= 0 {
		sqrtdiscr := math.Sqrt(discr)
		t0 := (b + sqrtdiscr) * g.inva
		t1 := (b - sqrtdiscr) * g.inva

		if t0*g.cd.r >= g.mindr {
			return getColor(t0, g.stops)
		} else if t1*g.cd.r >= g.mindr {
			return getColor(t1, g.stops)
		}
	}

	return color.Transparent
}

func (g *radialGradient) AddColorStop(offset float64, color color.Color) {
	g.stops = append(g.stops, stop{pos: offset, color: color})
	sort.Sort(g.stops)
}

func NewRadialGradient(x0, y0, r0, x1, y1, r1 float64) Gradient {
	c0 := circle{x0, y0, r0}
	c1 := circle{x1, y1, r1}
	cd := circle{x1 - x0, y1 - y0, r1 - r0}
	a := dot3(cd.x, cd.y, -cd.r, cd.x, cd.y, cd.r)
	var inva float64
	if a != 0 {
		inva = 1.0 / a
	}
	mindr := -c0.r
	g := &radialGradient{
		c0:    c0,
		c1:    c1,
		cd:    cd,
		a:     a,
		inva:  inva,
		mindr: mindr,
	}
	return g
}

// Conic Gradient
type conicGradient struct {
	cx, cy   float64
	rotation float64
	stops    stops
}

func (g *conicGradient) ColorAt(x, y int) color.Color {
	if len(g.stops) == 0 {
		return color.Transparent
	}
	a := math.Atan2(float64(y)-g.cy, float64(x)-g.cx)
	t := norm(a, -math.Pi, math.Pi) - g.rotation
	if t < 0 {
		t += 1
	}
	return getColor(t, g.stops)
}

func (g *conicGradient) AddColorStop(offset float64, color color.Color) {
	g.stops = append(g.stops, stop{pos: offset, color: color})
	sort.Sort(g.stops)
}

func NewConicGradient(cx, cy, deg float64) Gradient {
	g := &conicGradient{
		cx:       cx,
		cy:       cy,
		rotation: normalizeAngle(deg) / 360,
	}
	return g
}

func normalizeAngle(t float64) float64 {
	t = math.Mod(t, 360)
	if t < 0 {
		t += 360
	}
	return t
}

// Map value which is in range [a..b] to range [0..1]
func norm(value, a, b float64) float64 {
	return (value - a) * (1.0 / (b - a))
}

func getColor(pos float64, stops stops) color.Color {
	if pos <= 0.0 || len(stops) == 1 {
		return stops[0].color
	}

	last := stops[len(stops)-1]

	if pos >= last.pos {
		return last.color
	}

	for i, stop := range stops[1:] {
		if pos < stop.pos {
			pos = (pos - stops[i].pos) / (stop.pos - stops[i].pos)
			return colorLerp(stops[i].color, stop.color, pos)
		}
	}

	return last.color
}

func colorLerp(c0, c1 color.Color, t float64) color.Color {
	r0, g0, b0, a0 := c0.RGBA()
	r1, g1, b1, a1 := c1.RGBA()

	return color.RGBA{
		lerp(r0, r1, t),
		lerp(g0, g1, t),
		lerp(b0, b1, t),
		lerp(a0, a1, t),
	}
}

func lerp(a, b uint32, t float64) uint8 {
	return uint8(int32(float64(a)*(1.0-t)+float64(b)*t) >> 8)
}
