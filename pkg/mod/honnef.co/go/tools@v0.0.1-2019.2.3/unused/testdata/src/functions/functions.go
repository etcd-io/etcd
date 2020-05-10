package main

type state func() state

func a() state {
	return a
}

func main() {
	st := a
	_ = st()
}

type t1 struct{} // want `t1`
type t2 struct{}
type t3 struct{}

func fn1() t1     { return t1{} } // want `fn1`
func fn2() (x t2) { return }
func fn3() *t3    { return nil }

func fn4() {
	const x = 1
	const y = 2  // want `y`
	type foo int // want `foo`
	type bar int

	_ = x
	_ = bar(0)
}

func init() {
	fn2()
	fn3()
	fn4()
}
