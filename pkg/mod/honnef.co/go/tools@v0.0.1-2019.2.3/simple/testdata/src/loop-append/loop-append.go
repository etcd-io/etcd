package pkg

type T struct {
	F string
}

func fn1() {
	var x []interface{}
	var y []int

	for _, v := range y {
		x = append(x, v)
	}

	var a, b []int
	for _, v := range a { // want `should replace loop`
		b = append(b, v)
	}

	var m map[string]int
	var c []int
	for _, v := range m {
		c = append(c, v)
	}

	var t []T
	var m2 map[string][]T

	for _, tt := range t {
		m2[tt.F] = append(m2[tt.F], tt)
	}

	var out []T
	for _, tt := range t {
		out = append(m2[tt.F], tt)
	}
	_ = out
}

func fn2() {
	var v struct {
		V int
	}
	var in []int
	var out []int

	for _, v.V = range in {
		out = append(out, v.V)
	}
}

func fn3() {
	var t []T
	var out [][]T
	var m2 map[string][]T

	for _, tt := range t {
		out = append(out, m2[tt.F])
	}
}

func fn4() {
	var a, b, c []int
	for _, v := range a {
		b = append(c, v)
	}
	_ = b
}

func fn5() {
	var t []T
	var m2 map[string][]T
	var out []T
	for _, tt := range t {
		out = append(m2[tt.F], tt)
	}
	_ = out
}
