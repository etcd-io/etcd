package pkg

const c1 = 1

const c2 = 1
const c3 = 1
const c4 = 1
const C5 = 1

const (
	c6 = 0
	c7
	c8

	c9  // want `c9`
	c10 // want `c10`
	c11 // want `c11`
)

var _ = []int{c3: 1}

type T1 struct {
	F1 [c1]int
}

func init() {
	_ = []int{c2: 1}
	var _ [c4]int

	_ = c7
}

func Fn() {
	const X = 1 // want `X`
}
