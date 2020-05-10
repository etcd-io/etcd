package pkg

func fn() {
	var m map[string]int

	// with :=
	for x, _ := range m { // want `should omit value from range`
		_ = x
	}
	// with =
	var y string
	_ = y
	for y, _ = range m { // want `should omit value from range`
	}

	for _ = range m { // want `should omit values.*range.*equivalent.*for range`
	}

	for _, _ = range m { // want `should omit values.*range.*equivalent.*for range`
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
