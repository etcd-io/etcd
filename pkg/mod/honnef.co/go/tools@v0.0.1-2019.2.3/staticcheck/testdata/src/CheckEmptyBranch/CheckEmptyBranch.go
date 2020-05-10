package pkg

func fn1() {
	if true { // want `empty branch`
	}
	if true { // want `empty branch`
	} else { // want `empty branch`
	}
	if true {
		println()
	}

	if true {
		println()
	} else { // want `empty branch`
	}

	if true { // want `empty branch`
		// TODO handle error
	}

	if true {
	} else {
		println()
	}

	if true {
	} else if false { // want `empty branch`
	}
}
