// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"math"
)

type point struct {
	X, Y float64
}

type line struct {
	p1, p2 point
}

func sect(h, p [5]float64, v1, v2 int) float64 {
	return (h[v2]*p[v1] - h[v1]*p[v2]) / (h[v2] - h[v1])
}

// conrecLine performs an operation with a line at a given height derived
// from data over the 2D box interval (i, j) to (i+1, j+1).
type conrecLine func(i, j int, l line, height float64)

// conrec is a Go translation of the C version of CONREC by Paul Bourke:
// http://paulbourke.net/papers/conrec/conrec.c
//
// conrec takes g, an m×n grid function, a sorted slice of contour heights
// and a conrecLine function.
//
// For full details of the algorithm, see the paper at
// http://paulbourke.net/papers/conrec/
func conrec(g GridXYZ, heights []float64, fn conrecLine) {
	var (
		p1, p2 point

		h      [5]float64
		sh     [5]int
		xh, yh [5]float64

		im = [4]int{0, 1, 1, 0}
		jm = [4]int{0, 0, 1, 1}

		// We differ from conrec.c in the assignment of a single value
		// in cases (castab in conrec.c). The value of castab[1][1][1] is
		// 3, but we set cases[1][1][1] to 0.
		//
		// axiom: When we have a section of the grid where all the
		// Z values are equal, and equal to a contour height we would
		// expect to have no internal segments to draw.
		//
		// This is covered by case g) in Paul Bourke's description of
		// the CONREC algorithm (a triangle with three vertices the lie
		// on the contour level). He says, "... case g above has no really
		// satisfactory solution and fortunately will occur rarely with
		// real arithmetic." and then goes on to show the following image:
		//
		// http://paulbourke.net/papers/conrec/conrec3.gif
		//
		// which shows case g) in the set where no edge is drawn, agreeing
		// with our axiom above.
		//
		// However, in the iteration over sh at conrec.c +44, a triangle
		// with all vertices on the plane is given sh = {0,0,0,0,0} and
		// then when the switch at conrec.c +93 happens, castab resolves
		// that to case 3 for all values of m.
		//
		// This is fixed by replacing castab/cases[1][1][1] with 0.
		cases = [3][3][3]int{
			{{0, 0, 8}, {0, 2, 5}, {7, 6, 9}},
			{{0, 3, 4}, {1, 0, 1}, {4, 3, 0}},
			{{9, 6, 7}, {5, 2, 0}, {8, 0, 0}},
		}
	)

	c, r := g.Dims()
	for i := 0; i < c-1; i++ {
		for j := 0; j < r-1; j++ {
			dmin := math.Min(
				math.Min(g.Z(i, j), g.Z(i, j+1)),
				math.Min(g.Z(i+1, j), g.Z(i+1, j+1)),
			)

			dmax := math.Max(
				math.Max(g.Z(i, j), g.Z(i, j+1)),
				math.Max(g.Z(i+1, j), g.Z(i+1, j+1)),
			)

			if dmax < heights[0] || heights[len(heights)-1] < dmin {
				continue
			}

			for k := 0; k < len(heights); k++ {
				if heights[k] < dmin || dmax < heights[k] {
					continue
				}
				for m := 4; m >= 0; m-- {
					if m > 0 {
						h[m] = g.Z(i+im[m-1], j+jm[m-1]) - heights[k]
						xh[m] = g.X(i + im[m-1])
						yh[m] = g.Y(j + jm[m-1])
					} else {
						h[0] = 0.25 * (h[1] + h[2] + h[3] + h[4])
						xh[0] = 0.50 * (g.X(i) + g.X(i+1))
						yh[0] = 0.50 * (g.Y(j) + g.Y(j+1))
					}
					switch {
					case h[m] > 0:
						sh[m] = 1
					case h[m] < 0:
						sh[m] = -1
					default:
						sh[m] = 0
					}
				}

				/*
				   Note: at this stage the relative heights of the corners and the
				   centre are in the h array, and the corresponding coordinates are
				   in the xh and yh arrays. The centre of the box is indexed by 0
				   and the 4 corners by 1 to 4 as shown below.
				   Each triangle is then indexed by the parameter m, and the 3
				   vertices of each triangle are indexed by parameters m1,m2,and m3.
				   It is assumed that the centre of the box is always vertex 2
				   though this isimportant only when all 3 vertices lie exactly on
				   the same contour level, in which case only the side of the box
				   is drawn.

				      vertex 4 +-------------------+ vertex 3
				               | \               / |
				               |   \    m-3    /   |
				               |     \       /     |
				               |       \   /       |
				               |  m=2    X   m=2   |       the centre is vertex 0
				               |       /   \       |
				               |     /       \     |
				               |   /    m=1    \   |
				               | /               \ |
				      vertex 1 +-------------------+ vertex 2
				*/

				// Scan each triangle in the box.
				for m := 1; m <= 4; m++ {
					m1 := m
					const m2 = 0
					var m3 int
					if m != 4 {
						m3 = m + 1
					} else {
						m3 = 1
					}
					switch cases[sh[m1]+1][sh[m2]+1][sh[m3]+1] {
					case 0:
						continue

					case 1: // Line between vertices 1 and 2
						p1 = point{X: xh[m1], Y: yh[m1]}
						p2 = point{X: xh[m2], Y: yh[m2]}

					case 2: // Line between vertices 2 and 3
						p1 = point{X: xh[m2], Y: yh[m2]}
						p2 = point{X: xh[m3], Y: yh[m3]}

					case 3: // Line between vertices 3 and 1
						p1 = point{X: xh[m3], Y: yh[m3]}
						p2 = point{X: xh[m1], Y: yh[m1]}

					case 4: // Line between vertex 1 and side 2-3
						p1 = point{X: xh[m1], Y: yh[m1]}
						p2 = point{X: sect(h, xh, m2, m3), Y: sect(h, yh, m2, m3)}

					case 5: // Line between vertex 2 and side 3-1
						p1 = point{X: xh[m2], Y: yh[m2]}
						p2 = point{X: sect(h, xh, m3, m1), Y: sect(h, yh, m3, m1)}

					case 6: // Line between vertex 3 and side 1-2
						p1 = point{X: xh[m3], Y: yh[m3]}
						p2 = point{X: sect(h, xh, m1, m2), Y: sect(h, yh, m1, m2)}

					case 7: // Line between sides 1-2 and 2-3
						p1 = point{X: sect(h, xh, m1, m2), Y: sect(h, yh, m1, m2)}
						p2 = point{X: sect(h, xh, m2, m3), Y: sect(h, yh, m2, m3)}

					case 8: // Line between sides 2-3 and 3-1
						p1 = point{X: sect(h, xh, m2, m3), Y: sect(h, yh, m2, m3)}
						p2 = point{X: sect(h, xh, m3, m1), Y: sect(h, yh, m3, m1)}

					case 9: // Line between sides 3-1 and 1-2
						p1 = point{X: sect(h, xh, m3, m1), Y: sect(h, yh, m3, m1)}
						p2 = point{X: sect(h, xh, m1, m2), Y: sect(h, yh, m1, m2)}

					default:
						panic("cannot reach")
					}

					fn(i, j, line{p1: p1, p2: p2}, heights[k])
				}
			}
		}
	}
}
