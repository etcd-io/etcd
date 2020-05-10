package pkg

type t1 struct {
	a int
	b int
}

type t2 struct {
	a int
	b int
}

type t3 t1

func fn() {
	v1 := t1{1, 2}
	v2 := t2{1, 2}
	_ = t2{v1.a, v1.b}       // want `should convert v1`
	_ = t2{a: v1.a, b: v1.b} // want `should convert v1`
	_ = t2{b: v1.b, a: v1.a} // want `should convert v1`
	_ = t3{v1.a, v1.b}       // want `should convert v1`

	_ = t3{v1.a, v2.b}

	_ = t2{v1.b, v1.a}
	_ = t2{a: v1.b, b: v1.a}
	_ = t2{a: v1.a}
	_ = t1{v1.a, v1.b}

	v := t1{1, 2}
	_ = &t2{v.a, v.b}
}
