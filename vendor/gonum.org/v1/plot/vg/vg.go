// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vg defines an interface for drawing 2D vector graphics.
// This package is designed with the hope that many different
// vector graphics back-ends can conform to the interface.
package vg // import "gonum.org/v1/plot/vg"

import (
	"image"
	"image/color"
	"io"

	"gonum.org/v1/plot/font"
)

// A Canvas is the main drawing interface for 2D vector
// graphics.
// The origin is in the bottom left corner.
type Canvas interface {
	// SetLineWidth sets the width of stroked paths.
	// If the width is not positive then stroked lines
	// are not drawn.
	//
	// The initial line width is 1 point.
	SetLineWidth(Length)

	// SetLineDash sets the dash pattern for lines.
	// The pattern slice specifies the lengths of
	// alternating dashes and gaps, and the offset
	// specifies the distance into the dash pattern
	// to start the dash.
	//
	// The initial dash pattern is a solid line.
	SetLineDash(pattern []Length, offset Length)

	// SetColor sets the current drawing color.
	// Note that fill color and stroke color are
	// the same, so if you want different fill
	// and stroke colors then you must set a color,
	// draw fills, set a new color and then draw lines.
	//
	// The initial color is black.
	// If SetColor is called with a nil color then black is used.
	SetColor(color.Color)

	// Rotate applies a rotation transform to the context.
	// The parameter is specified in radians.
	Rotate(rad float64)

	// Translate applies a translational transform
	// to the context.
	Translate(pt Point)

	// Scale applies a scaling transform to the
	// context.
	Scale(x, y float64)

	// Push saves the current line width, the
	// current dash pattern, the current
	// transforms, and the current color
	// onto a stack so that the state can later
	// be restored by calling Pop().
	Push()

	// Pop restores the context saved by the
	// corresponding call to Push().
	Pop()

	// Stroke strokes the given path.
	Stroke(Path)

	// Fill fills the given path.
	Fill(Path)

	// FillString fills in text at the specified
	// location using the given font.
	// If the font size is zero, the text is not drawn.
	FillString(f font.Face, pt Point, text string)

	// DrawImage draws the image, scaled to fit
	// the destination rectangle.
	DrawImage(rect Rectangle, img image.Image)
}

// CanvasSizer is a Canvas with a defined size.
type CanvasSizer interface {
	Canvas
	Size() (x, y Length)
}

// CanvasWriterTo is a CanvasSizer with a WriteTo method.
type CanvasWriterTo interface {
	CanvasSizer
	io.WriterTo
}

// Initialize sets all of the canvas's values to their
// initial values.
func Initialize(c Canvas) {
	c.SetLineWidth(Points(1))
	c.SetLineDash([]Length{}, 0)
	c.SetColor(color.Black)
}

type Path []PathComp

// Move moves the current location of the path to
// the given point.
func (p *Path) Move(pt Point) {
	*p = append(*p, PathComp{Type: MoveComp, Pos: pt})
}

// Line draws a line from the current point to the
// given point.
func (p *Path) Line(pt Point) {
	*p = append(*p, PathComp{Type: LineComp, Pos: pt})
}

// Arc adds an arc to the path defined by the center
// point of the arc's circle, the radius of the circle
// and the start and sweep angles.
func (p *Path) Arc(pt Point, rad Length, s, a float64) {
	*p = append(*p, PathComp{
		Type:   ArcComp,
		Pos:    pt,
		Radius: rad,
		Start:  s,
		Angle:  a,
	})
}

// QuadTo adds a quadratic curve element to the path,
// given by the control point p1 and end point pt.
func (p *Path) QuadTo(p1, pt Point) {
	*p = append(*p, PathComp{Type: CurveComp, Pos: pt, Control: []Point{p1}})
}

// CubeTo adds a cubic curve element to the path,
// given by the control points p1 and p2 and the end point pt.
func (p *Path) CubeTo(p1, p2, pt Point) {
	*p = append(*p, PathComp{Type: CurveComp, Pos: pt, Control: []Point{p1, p2}})
}

// Close closes the path by connecting the current
// location to the start location with a line.
func (p *Path) Close() {
	*p = append(*p, PathComp{Type: CloseComp})
}

// Constants that tag the type of each path
// component.
const (
	MoveComp = iota
	LineComp
	ArcComp
	CurveComp
	CloseComp
)

// A PathComp is a component of a path structure.
type PathComp struct {
	// Type is the type of a particluar component.
	// Based on the type, each of the following
	// fields may have a different meaning or may
	// be meaningless.
	Type int

	// The Pos field is used as the destination
	// of a MoveComp or LineComp and is the center
	// point of an ArcComp.  It is not used in
	// the CloseComp.
	Pos Point

	// Control is one or two intermediate points
	// for a CurveComp used by QuadTo and CubeTo.
	Control []Point

	// Radius is only used for ArcComps, it is
	// the radius of the circle defining the arc.
	Radius Length

	// Start and Angle are only used for ArcComps.
	// They define the start angle and sweep angle of
	// the arc around the circle.  The units of the
	// angle are radians.
	Start, Angle float64
}
