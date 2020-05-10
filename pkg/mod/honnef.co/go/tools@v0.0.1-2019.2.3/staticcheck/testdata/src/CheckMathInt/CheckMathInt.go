package pkg

import "math"

func fn(x int) {
	math.Ceil(float64(x))      // want `on a converted integer is pointless`
	math.Floor(float64(x * 2)) // want `on a converted integer is pointless`
}
