// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vg

import "gonum.org/v1/plot/font"

// A Length is a unit-independent representation of length.
// Internally, the length is stored in postscript points.
type Length = font.Length

// Points returns a length for the given number of points.
func Points(pt float64) Length {
	return font.Points(pt)
}

// Common lengths.
const (
	Inch       = font.Inch
	Centimeter = font.Centimeter
	Millimeter = font.Millimeter
)

// ParseLength parses a Length string.
// A Length string is a possible signed floating number with a unit.
// e.g. "42cm" "2.4in" "66pt"
// If no unit was given, ParseLength assumes it was (postscript) points.
// Currently valid units are:
//
//   - mm (millimeter)
//   - cm (centimeter)
//   - in (inch)
//   - pt (point)
func ParseLength(value string) (Length, error) {
	return font.ParseLength(value)
}
