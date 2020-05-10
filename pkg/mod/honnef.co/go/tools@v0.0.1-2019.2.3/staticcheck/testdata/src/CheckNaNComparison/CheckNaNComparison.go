package pkg

import "math"

func fn(f float64) {
	_ = f == math.NaN() // want `no value is equal to NaN`
	_ = f > math.NaN()  // want `no value is equal to NaN`
	_ = f != math.NaN() // want `no value is equal to NaN`
}
