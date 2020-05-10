package pkg

type t1 struct {
	a int
	b int // want `b`
}

type t2 struct {
	a int // want `a`
	b int
}

func Fn() {
	x := t1{}
	y := t2{}
	println(x.a)
	println(y.b)
}
