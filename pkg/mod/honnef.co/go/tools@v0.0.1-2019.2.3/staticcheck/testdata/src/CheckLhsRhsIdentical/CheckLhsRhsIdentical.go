package pkg

type Float float64

func fn(a int, s []int, f1 float64, f2 Float) {
	if 0 == 0 { // want `identical expressions`
		println()
	}
	if 1 == 1 { // want `identical expressions`
		println()
	}
	if a == a { // want `identical expressions`
		println()
	}
	if a != a { // want `identical expressions`
		println()
	}
	if s[0] == s[0] { // want `identical expressions`
		println()
	}
	if 1&1 == 1 { // want `identical expressions`
		println()
	}
	if (1 + 2 + 3) == (1 + 2 + 3) { // want `identical expressions`
		println()
	}
	if f1 == f1 {
		println()
	}
	if f1 != f1 {
		println()
	}
	if f1 > f1 { // want `identical expressions`
		println()
	}
	if f2 == f2 {
		println()
	}
}
