// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotutil

import (
	"math"
	"sort"

	"gonum.org/v1/plot/plotter"
)

// ErrorPoints holds a set of x, y pairs along
// with their X and Y errors.
type ErrorPoints struct {
	plotter.XYs
	plotter.XErrors
	plotter.YErrors
}

// NewErrorPoints returns a new ErrorPoints where each
// point in the ErrorPoints is given by evaluating the
// center function on the Xs and Ys for the corresponding
// set of XY values in the pts parameter.  The XError
// and YError are computed likewise, using the err
// function.
//
// This function can be useful for summarizing sets of
// scatter points using a single point and error bars for
// each element of the scatter.
func NewErrorPoints(f func([]float64) (c, l, h float64), pts ...plotter.XYer) (*ErrorPoints, error) {

	c := &ErrorPoints{
		XYs:     make(plotter.XYs, len(pts)),
		XErrors: make(plotter.XErrors, len(pts)),
		YErrors: make(plotter.YErrors, len(pts)),
	}

	for i, xy := range pts {
		xs := make([]float64, xy.Len())
		ys := make([]float64, xy.Len())
		for j := 0; j < xy.Len(); j++ {
			xs[j], ys[j] = xy.XY(j)
			if err := plotter.CheckFloats(xs[j], ys[j]); err != nil {
				return nil, err
			}
		}
		c.XYs[i].X, c.XErrors[i].Low, c.XErrors[i].High = f(xs)
		if err := plotter.CheckFloats(c.XYs[i].X, c.XErrors[i].Low, c.XErrors[i].High); err != nil {
			return nil, err
		}
		c.XYs[i].Y, c.YErrors[i].Low, c.YErrors[i].High = f(ys)
		if err := plotter.CheckFloats(c.XYs[i].Y, c.YErrors[i].Low, c.YErrors[i].High); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// MeanAndConf95 returns the mean
// and the magnitude of the 95% confidence
// interval on the mean as low and high
// error values.
//
// MeanAndConf95 may be used as
// the f argument to NewErrorPoints.
func MeanAndConf95(vls []float64) (mean, lowerr, higherr float64) {
	n := float64(len(vls))

	sum := 0.0
	for _, v := range vls {
		sum += v
	}
	mean = sum / n

	sum = 0.0
	for _, v := range vls {
		diff := v - mean
		sum += diff * diff
	}
	stdev := math.Sqrt(sum / n)

	conf := 1.96 * stdev / math.Sqrt(n)
	return mean, conf, conf
}

// MedianAndMinMax returns the median
// value and error on the median given
// by the minimum and maximum data
// values.
//
// MedianAndMinMax may be used as
// the f argument to NewErrorPoints.
func MedianAndMinMax(vls []float64) (med, lowerr, higherr float64) {
	n := len(vls)
	if n == 0 {
		panic("plotutil: MedianAndMinMax: No values")
	}
	if n == 1 {
		return vls[0], 0, 0
	}
	sort.Float64s(vls)
	if n%2 == 0 {
		med = (vls[n/2]-vls[n/2-1])/2 + vls[n/2-1]
	} else {
		med = vls[n/2]
	}

	min := vls[0]
	max := vls[0]
	for _, v := range vls {
		min = math.Min(min, v)
		max = math.Max(max, v)
	}

	return med, med - min, max - med
}
