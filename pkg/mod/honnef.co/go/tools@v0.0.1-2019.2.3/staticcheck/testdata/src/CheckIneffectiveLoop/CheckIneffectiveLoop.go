package pkg

func fn() {
	for {
		if true {
			println()
		}
		break // want `the surrounding loop is unconditionally terminated`
	}
	for {
		if true {
			break
		} else {
			break
		}
	}
	for range ([]int)(nil) {
		if true {
			println()
		}
		break // want `the surrounding loop is unconditionally terminated`
	}
	for range (map[int]int)(nil) {
		if true {
			println()
		}
		break
	}
	for {
		if true {
			goto Label
		}
		break
	Label:
	}
	for {
		if true {
			continue
		}
		break
	}
	for {
		if true {
			continue
		}
		break
	}
}

var z = func() {
	for {
		if true {
			println()
		}
		break // want `the surrounding loop is unconditionally terminated`
	}
}
