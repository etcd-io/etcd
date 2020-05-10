package main

import "fmt"


// Test for renaming the switch variable
func main() {

	switch x := 5; { 		// <<<<< rename,9,9,9,9,renamed,pass
	 case x < 0:  fmt.Println(-x)
         case x==0 :  fmt.Println(x)
         case x > 0 :  fmt.Println(x)
	 default:  fmt.Println(x)
       }

 
}
