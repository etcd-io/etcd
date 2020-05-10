package pkg

type t1 struct {
	a int
	b int
}

type t2 struct {
	a int
	b int
}

type t3 struct {
	a int `tag`
	b int `tag`
}

func fn() {
	v1 := t1{1, 2}
	_ = t2{v1.a, v1.b} // want `should convert v1`
	_ = t3{v1.a, v1.b} // want `should convert v1`
}
