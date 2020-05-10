package pkg

type t struct {
	f int
}

func fn(v *t) {
	println(v.f)
}

func init() {
	var v t
	fn(&v)
}
