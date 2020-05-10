//<<<<<extract,20,2,25,2,foo,pass
package main

import "fmt"

type SliceOfints []int
type AgesByNames map[string]int

func (s SliceOfints) sum() int {
	sum := 0
	for _, value := range s {
		sum += value
	}
	return sum
}

func (people AgesByNames) older() string {
	a := 0
	n := ""
	for key, value := range people {
		if value > a {
			a = value
			n = key
		}
	}
	fmt.Println(a, n)
	return n
}

func main() {
	s := SliceOfints{1, 2, 3, 4, 5}
	folks := AgesByNames{
		"Bob":   36,
		"Mike":  44,
		"Jane":  30,
		"Popey": 100,
	}

	fmt.Println("The sum of ints in the slice s is: ", s.sum())
	fmt.Println("The older in the map folks is:", folks.older())
}
