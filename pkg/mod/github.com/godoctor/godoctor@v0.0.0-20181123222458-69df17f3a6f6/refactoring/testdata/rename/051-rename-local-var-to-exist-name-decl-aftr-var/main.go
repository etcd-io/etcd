																																																																																																																																																						package main

import "fmt"

func main() {
     a := 5     // <<<<< rename,6,6,6,6,b,fail
     fmt.Println(a)
     b := 6
     fmt.Println(a, b)
}
