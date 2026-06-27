// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright ©2013 The bíogo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Palette type comments ©2002 Cynthia Brewer.

// Package brewer provides Brewer Palettes for informative graphics.
//
// The colors defined here are from http://www.ColorBrewer.org/ by Cynthia A. Brewer,
// Geography, Pennsylvania State University.
//
// For more information see:
// http://www.personal.psu.edu/cab38/ColorBrewer/ColorBrewer_learnMore.html
package brewer // import "gonum.org/v1/plot/palette/brewer"

import (
	"errors"
	"fmt"
	"image/color"

	"gonum.org/v1/plot/palette"
)

// Color represents a Brewer Palette color.
type Color struct {
	Letter byte
	color.Color
}

// Usability describes the usability of a palette for particular use cases.
type Usability byte

const (
	NotAvalailable Usability = iota
	Bad
	Unsure
	Good
)

// Palette represents a color scheme.
type Palette struct {
	ID   string
	Name string

	Laptop     Usability
	CRT        Usability
	ColorBlind Usability
	Copy       Usability
	Projector  Usability

	Color []color.Color
}

// DivergingPalette represents a diverging color scheme.
type DivergingPalette Palette

// Colors returns the palette's color collection.
func (d DivergingPalette) Colors() []color.Color { return d.Color }

// CriticalIndex returns the indices of the lightest (median) color or colors in the DivergingPalette.
// The low and high index values will be equal when there is a single median color.
func (d DivergingPalette) CriticalIndex() (low, high int) {
	l := len(d.Color)
	return (l - 1) / 2, l / 2
}

// NonDivergingPalette represents sequential or qualitative color schemes.
type NonDivergingPalette Palette

// Colors returns the palette's color collection.
func (d NonDivergingPalette) Colors() []color.Color { return d.Color }

// Diverging schemes put equal emphasis on mid-range critical values and extremes
// at both ends of the data range. The critical class or break in the middle of the
// legend is emphasized with light colors and low and high extremes are emphasized
// with dark colors that have contrasting hues.
type Diverging map[int]DivergingPalette

// Qualitative schemes do not imply magnitude differences between legend classes,
// and hues are used to create the primary visual differences between classes.
// Qualitative schemes are best suited to representing nominal or categorical data.
type Qualitative map[int]NonDivergingPalette

// Sequential schemes are suited to ordered data that progress from low to high.
// Lightness steps dominate the look of these schemes, with light colors for low
// data values to dark colors for high data values.
type Sequential map[int]NonDivergingPalette

var (
	// DivergingPalettes is a string-based map look-up table of diverging palettes.
	DivergingPalettes map[string]Diverging = diverging

	// QualitativePalettes is a string-based map look-up table of qualitative palettes.
	QualitativePalettes map[string]Qualitative = qualitative

	// SequentialPalettes is a string-based map look-up table of sequential palettes.
	SequentialPalettes map[string]Sequential = sequential
)

// PaletteType indicates palette type for a GetPalette request.
type PaletteType int

const (
	TypeAny PaletteType = iota
	TypeDiverging
	TypeQualitative
	TypeSequential
)

// GetPalette returns a Palette based on palette type and name, and the number of colors
// required. An error is returned if the palette name or type is not known or the requested
// palette does not support the required number of colors.
func GetPalette(typ PaletteType, name string, colors int) (palette.Palette, error) {
	if colors < 3 {
		return nil, errors.New("brewer: number of colors must be 3 or greater")
	}
	var (
		p palette.Palette

		nameOk, colorsOk bool
	)
	switch typ {
	case TypeAny:
		var pt interface{}
		pt, nameOk = all[name]
		if !nameOk {
			break
		}
		switch pt := pt.(type) {
		case Diverging:
			p, colorsOk = pt[colors]
		case Qualitative:
			p, colorsOk = pt[colors]
		case Sequential:
			p, colorsOk = pt[colors]
		default:
			panic("brewer: unexpected type")
		}
	case TypeDiverging:
		var pt Diverging
		pt, nameOk = diverging[name]
		if !nameOk {
			break
		}
		p, colorsOk = pt[colors]
	case TypeQualitative:
		var pt Qualitative
		pt, nameOk = qualitative[name]
		if !nameOk {
			break
		}
		p, colorsOk = pt[colors]
	case TypeSequential:
		var pt Sequential
		pt, nameOk = sequential[name]
		if !nameOk {
			break
		}
		p, colorsOk = pt[colors]
	default:
		return nil, fmt.Errorf("brewer: palette type not known: %v", typ)
	}
	if !nameOk {
		return nil, fmt.Errorf("brewer: palette %q not known", name)
	}
	if !colorsOk {
		return nil, fmt.Errorf("brewer: palette %q does not support %d colors", name, colors)
	}
	return p, nil
}
