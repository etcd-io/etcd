// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package palette

import (
	"image/color"
)

// Reverse reverses the direction of ColorMap c.
func Reverse(c ColorMap) ColorMap {
	return reverse{ColorMap: c}
}

// reverse is a ColorMap that reverses the direction of the ColorMap it
// contains.
type reverse struct {
	ColorMap
}

// At implements the ColorMap interface for a Reversed ColorMap.
func (r reverse) At(v float64) (color.Color, error) {
	return r.ColorMap.At(r.Max() - (v - r.Min()))
}

// Palette implements the ColorMap interface for a Reversed ColorMap.
func (r reverse) Palette(colors int) Palette {
	c := r.ColorMap.Palette(colors).Colors()
	c2 := make([]color.Color, len(c))
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c2[i], c2[j] = c[j], c[i]
	}
	return palette(c2)
}
