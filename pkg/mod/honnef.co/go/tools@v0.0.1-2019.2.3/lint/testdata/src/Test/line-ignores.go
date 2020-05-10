package pkg

// the line directive should not affect the line ignores

//line random-file:1
func fn1() {} // want `test problem`

//lint:ignore TEST1000 This should be ignored, because ...
//lint:ignore XXX1000 Testing that multiple linter directives work correctly
func fn2() {}

//lint:ignore TEST1000 // want `malformed linter directive`
func fn3() {} // want `test problem`

//lint:ignore TEST1000 ignore
func fn4() {
	//lint:ignore TEST1000 ignore // want `this linter directive didn't match anything`
	var _ int
}
