package pkg

type iface interface { // want `iface`
	foo()
}

type t1 struct{} // want `t1`
func (t1) foo()  {}

type t2 struct{}

func (t t2) bar(arg int) (ret int) { return 0 } // want `bar`

func init() {
	_ = t2{}
}

type t3 struct { // want `t3`
	a int
	b int
}

type T struct{}

func fn1() { // want `fn1`
	meh := func(arg T) {
	}
	meh(T{})
}

type localityList []int // want `localityList`

func (l *localityList) Fn1() {}
func (l *localityList) Fn2() {}
