package pkg

import "fmt"

func fn1() {
	var s []int
	s = append(s, 1) // want `this result of append is never used`
	s = append(s, 1) // want `this result of append is never used`
}

func fn2() (named []int) {
	named = append(named, 1)
	return
}

func fn3() {
	s := make([]int, 0)
	s = append(s, 1) // want `this result of append is never used`
}

func fn4() []int {
	var s []int
	s = append(s, 1)
	return s
}

func fn5() {
	var s []int
	s = append(s, 1)
	fn6(s)
}

func fn6([]int) {}

func fn7() {
	var s []int
	fn8(&s)
	s = append(s, 1)
}

func fn8(*[]int) {}

func fn9() {
	var s []int
	s = append(s, 1)
	fmt.Println(s)
	s = append(s, 1) // want `this result of append is never used`
}

func fn10() {
	var s []int
	return
	s = append(s, 1)
}

func fn11() {
	var s []int
	for x := 0; x < 10; x++ {
		s = append(s, 1) // want `this result of append is never used`
	}
}
