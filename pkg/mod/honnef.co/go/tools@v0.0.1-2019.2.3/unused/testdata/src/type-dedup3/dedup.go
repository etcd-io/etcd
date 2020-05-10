package pkg

func fn1(t struct {
	a int
	b int
}) {
	fn2(t)
}

func fn2(t struct {
	a int
	b int
}) {
	println(t.a)
	println(t.b)
}

func Fn() {
	fn1(struct {
		a int
		b int
	}{1, 2})
}
