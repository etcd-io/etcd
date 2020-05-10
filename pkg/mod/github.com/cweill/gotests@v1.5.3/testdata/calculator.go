package testdata

import "errors"

type Calculator struct{}

func (c *Calculator) Multiply(n, d int) int {
	return n * d
}

func (c *Calculator) Divide(n, d int) (int, error) {
	if d == 0 {
		return 0, errors.New("division by zero")
	}
	return n / d, nil
}
