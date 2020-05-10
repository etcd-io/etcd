//<<<<<extract,12,2,12,14,Foo,fail
package main

import "fmt"

func sum(nums ...int) int {
	fmt.Print(nums, " ")
	total := 0
	for _, num := range nums {
		total += num
	}
	return total
}
func main() {
	nums := []int{1, 2, 3, 4}
	total := sum(nums...)
	fmt.Println(total)
}
