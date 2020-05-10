// <<<<< toggle,9,3,9,15,pass
package main

import "fmt"
import "reflect"

// Test for changing var i int = 3 to a short assignment  declaration
func main() {
  var i int = 3
  fmt.Println("type of i is", reflect.TypeOf(i))
  fmt.Println("value of i is", i)
}
