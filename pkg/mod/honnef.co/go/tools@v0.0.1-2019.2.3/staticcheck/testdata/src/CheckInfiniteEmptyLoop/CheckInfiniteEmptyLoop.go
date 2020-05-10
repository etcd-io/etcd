package pkg

func fn2() bool { return true }

func fn() {
	for { // want `this loop will spin`
	}

	for fn2() {
	}

	for {
		break
	}

	for true { // want `loop condition never changes` `this loop will spin`
	}

	x := true
	for x { // want `loop condition never changes` `this loop will spin`
	}

	x = false
	for x { // want `loop condition never changes` `this loop will spin`
	}

	for false {
	}

	false := true
	for false { // want `loop condition never changes` `this loop will spin`
	}
}
