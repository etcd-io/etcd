package pkg

func fn(b1, b2 bool) {
	if !!b1 { // want `negating a boolean twice`
		println()
	}

	if b1 && !!b2 { // want `negating a boolean twice`
		println()
	}

	if !(!b1) { // doesn't match, maybe it should
		println()
	}

	if !b1 {
		println()
	}

	if !b1 && !b2 {
		println()
	}
}
