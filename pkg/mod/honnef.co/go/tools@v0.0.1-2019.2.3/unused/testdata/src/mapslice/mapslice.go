package pkg

type M map[int]int

func Fn() {
	var n M
	_ = []M{n}
}
