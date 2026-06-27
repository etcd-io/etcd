// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tex

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v float64) float64 {
	const (
		min = -1000000000.
		max = +1000000000.
	)
	switch {
	case v < min:
		return min
	case v > max:
		return max
	}
	return v
}
