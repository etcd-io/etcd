package pkg

func fn(x int) {
	switch x {
	case 1:
		println()
		break // want `redundant break`
	case 2:
		println()
	case 3:
		break // don't flag cases only consisting of break
	case 4:
		println()
		break
		println()
	case 5:
		println()
		if true {
			break // we don't currently detect this
		}
	case 6:
		println()
		if true {
			break
		}
		println()
	}

label:
	for {
		switch x {
		case 1:
			println()
			break label
		}
	}
}
