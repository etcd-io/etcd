package pkg

func fn() {
	var m map[string]int

	// with :=
	for x, _ := range m {
		_ = x
	}
	// with =
	var y string
	_ = y
	for y, _ = range m {
	}

	// all OK:
	for x := range m {
		_ = x
	}
	for x, y := range m {
		_, _ = x, y
	}
	for _, y := range m {
		_ = y
	}
	var x int
	_ = x
	for y = range m {
	}
	for y, x = range m {
	}
	for _, x = range m {
	}
}
