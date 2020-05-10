// <<<<< toggle,9,2,9,23,pass
package main

import "fmt"

func main() {
	capitals:= map[string]string{"France": "Paris", "Italy": "Rome", "Japan": "Tokyo"}
	a, b := capitals["France"]
	c := capitals["Italy"]
	fmt.Println(a, b, c)
	for key2, val := range capitals {
		fmt.Println("Map item: Capital of", key2, "is", val)
	}
}
