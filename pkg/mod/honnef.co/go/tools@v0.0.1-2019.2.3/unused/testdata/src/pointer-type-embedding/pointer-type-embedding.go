package pkg

func init() {
	var p P
	_ = p.n
}

type T0 struct {
	m int // want `m`
	n int
}

type T1 struct {
	T0
}

type P *T1
