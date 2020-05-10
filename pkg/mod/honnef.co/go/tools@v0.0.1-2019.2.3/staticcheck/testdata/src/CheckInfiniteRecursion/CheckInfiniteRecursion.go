package pkg

func fn1(x int) bool {
	println(x)
	return fn1(x + 1) // want `infinite recursive call`
	return true
}

func fn2(x int) bool {
	println(x)
	if x > 10 {
		return true
	}
	return fn2(x + 1)
}

func fn3(x int) bool {
	println(x)
	if x > 10 {
		goto l1
	}
	return fn3(x + 1)
l1:
	println(x)
	return true
}

func fn4(p *int, n int) {
	if n == 0 {
		return
	}
	x := 0
	fn4(&x, n-1)
	if x != n {
		panic("stack is corrupted")
	}
}

func fn5(p *int, n int) {
	x := 0
	fn5(&x, n-1) // want `infinite recursive call`
	if x != n {
		panic("stack is corrupted")
	}
}

func fn6() {
	go fn6()
}

type T struct {
	n int
}

func (t T) Fn1() {
	t.Fn1() // want `infinite recursive call`
}

func (t T) Fn2() {
	x := T{}
	x.Fn2() // want `infinite recursive call`
}

func (t T) Fn3() {
	if t.n == 0 {
		return
	}
	t.Fn1()
}
