package pkg

type t struct{}

func (t) fn1() {}
func (t) fn2() {}
func fn1()     {}
func fn2()     {}

func Fn() {
	var v t
	defer fn1()
	defer v.fn1()
	go fn2()
	go v.fn2()
}
