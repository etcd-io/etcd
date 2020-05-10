// Package pkg ...
package pkg

func fn(x int) {
	switch x {
	}
	switch x {
	case 1:
	}

	switch x {
	case 1:
	case 2:
	case 3:
	}

	switch x {
	default:
	}

	switch x {
	default:
	case 1:
	}

	switch x {
	case 1:
	default:
	}

	switch x {
	case 1:
	default: // want `default case should be first or last in switch statement`
	case 2:
	}
}
