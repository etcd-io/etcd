// Test of field usage detection

package pkg

type t15 struct{ f151 int }
type a2 [1]t15

type t16 struct{}
type a3 [1][1]t16

func foo() {
	_ = a2{0: {1}}
	_ = a3{{{}}}
}

func init() { foo() }
