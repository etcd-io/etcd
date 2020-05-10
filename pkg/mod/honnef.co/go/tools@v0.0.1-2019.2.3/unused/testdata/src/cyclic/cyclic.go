package pkg

func a() { // want `a`
	b()
}

func b() { // want `b`
	a()
}
