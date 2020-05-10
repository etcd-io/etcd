// Package pkg ...
package pkg

func fn(x string, y int) {
	if "" == x { // want `Yoda`
	}
	if 0 == y { // want `Yoda`
	}
	if 0 > y {
	}
	if "" == "" {
	}

	if "" == "" || 0 == y { // want `Yoda`
	}
}
