//<<<<<extract,11,2,13,2,Foo,fail
package main

import (
	"errors"
	"fmt"
	"math"
)

func MySqrt(f float64) (float64, error) {
	if f < 0 {
		return float64(math.NaN()), errors.New("Negative Number")
	}
	return math.Sqrt(f), nil
}

func main() {
	ret1, err1 := MySqrt(-1)
	if err1 != nil {
		fmt.Println("No Valid input", ret1, err1)
	} else {
		fmt.Println("RESULT", ret1, err1)
	}
	if ret2, err2 := MySqrt(5); err2 != nil {
		fmt.Println("No Valid input", ret2, err2)
	} else {
		fmt.Println("RESULT", ret2, err2)
	}
}
