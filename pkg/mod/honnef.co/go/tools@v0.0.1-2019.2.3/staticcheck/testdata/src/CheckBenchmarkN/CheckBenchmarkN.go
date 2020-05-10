package foo

import "testing"

func foo() {
	var b *testing.B
	b.N = 1 // want `should not assign to b\.N`
	_ = b
}
