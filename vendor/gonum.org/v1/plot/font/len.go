// Copyright ©2021 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package font

import (
	"strconv"
	"strings"
)

// Length is a unit-independent representation of length.
// Internally, the length is stored in postscript points.
type Length float64

// Dots returns the length in dots for the given resolution.
func (l Length) Dots(dpi float64) float64 {
	return float64(l) / Inch.Points() * dpi
}

// Points returns the length in postscript points.
func (l Length) Points() float64 {
	return float64(l)
}

// Common lengths.
const (
	Inch       Length = 72
	Centimeter        = Inch / 2.54
	Millimeter        = Centimeter / 10
)

// Points returns a length for the given number of points.
func Points(pt float64) Length {
	return Length(pt)
}

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
	var unit Length = 1
	switch {
	case strings.HasSuffix(value, "in"):
		value = value[:len(value)-len("in")]
		unit = Inch
	case strings.HasSuffix(value, "cm"):
		value = value[:len(value)-len("cm")]
		unit = Centimeter
	case strings.HasSuffix(value, "mm"):
		value = value[:len(value)-len("mm")]
		unit = Millimeter
	case strings.HasSuffix(value, "pt"):
		value = value[:len(value)-len("pt")]
		unit = 1
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return Length(v) * unit, nil
}
