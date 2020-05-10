package pkg

import "sort"

type MyIntSlice []int

func (s MyIntSlice) Len() int           { return 0 }
func (s MyIntSlice) Less(i, j int) bool { return true }
func (s MyIntSlice) Swap(i, j int)      {}

func fn1() {
	var a []int
	sort.Sort(sort.IntSlice(a)) // want `sort\.Ints`
}

func fn2() {
	var b []float64
	sort.Sort(sort.Float64Slice(b)) // want `sort\.Float64s`
}

func fn3() {
	var c []string
	sort.Sort(sort.StringSlice(c)) // want `sort\.Strings`
}

func fn4() {
	var a []int
	sort.Sort(MyIntSlice(a))
}

func fn5() {
	var d MyIntSlice
	sort.Sort(d)
}

func fn6() {
	var e sort.Interface
	sort.Sort(e)
}

func fn7() {
	// Don't recommend sort.Ints when there was another legitimate
	// sort.Sort call already
	var a []int
	var e sort.Interface
	sort.Sort(e)
	sort.Sort(sort.IntSlice(a))
}

func fn8() {
	var a []int
	sort.Sort(sort.IntSlice(a)) // want `sort\.Ints`
	sort.Sort(sort.IntSlice(a)) // want `sort\.Ints`
}

func fn9() {
	func() {
		var a []int
		sort.Sort(sort.IntSlice(a)) // want `sort\.Ints`
	}()
}

func fn10() {
	var a MyIntSlice
	sort.Sort(sort.IntSlice(a)) // want `sort\.Ints`
}
