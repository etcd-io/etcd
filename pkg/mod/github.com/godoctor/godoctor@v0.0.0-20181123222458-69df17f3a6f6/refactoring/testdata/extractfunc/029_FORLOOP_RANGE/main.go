//<<<<<extract,9,2,11,2,Foo,pass
package main

import "fmt"

func main() {
	xs := []float64{98, 93, 77, 82, 83}
	total := 0.0 + 0
	for _, v := range xs {
		total += v
	}
	fmt.Println(total / float64(len(xs)))
}
