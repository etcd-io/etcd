package pkg

func fn1() bool { return false }
func fn2() bool { return false }

func fn() {
	type T bool
	var x T
	const t T = false
	if x == t {
	}
	if fn1() == true { // want `simplified to fn1\(\)`
	}
	if fn1() != true { // want `simplified to !fn1\(\)`
	}
	if fn1() == false { // want `simplified to !fn1\(\)`
	}
	if fn1() != false { // want `simplified to fn1\(\)`
	}
	if fn1() && (fn1() || fn1()) || (fn1() && fn1()) == true { // want `simplified to \(fn1\(\) && fn1\(\)\)`
	}

	if (fn1() && fn2()) == false { // want `simplified to !\(fn1\(\) && fn2\(\)\)`
	}

	var y bool
	for y != true { // want `simplified to !y`
	}
	if !y == true { // want `simplified to !y`
	}
	if !y == false { // want `simplified to y`
	}
	if !y != true { // want `simplified to y`
	}
	if !y != false { // want `simplified to !y`
	}
	if !!y == false { // want `simplified to !y`
	}
	if !!!y == false { // want `simplified to y`
	}
	if !!y == true { // want `simplified to y`
	}
	if !!!y == true { // want `simplified to !y`
	}
	if !!y != true { // want `simplified to !y`
	}
	if !!!y != true { // want `simplified to y`
	}
	if !y == !false { // not matched because we expect true/false on one side, not !false
	}

	var z interface{}
	if z == true {
	}
}
