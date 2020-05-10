// <<<<< toggle,11,3,11,13,pass
package main

import "fmt"

func f() (float64, float64) {
	return 1, 2.3
}

func main() {
  i, x := f()
  fmt.Println(i, x)
}
