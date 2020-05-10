// <<<<< toggle,11,3,11,19,pass
package main

import "fmt"

func f() (int, float64,string) {
	return 1, 2.3,"hello"
}

func main() {
  var i, x, s = f()
  fmt.Println(i, x, s)
}
