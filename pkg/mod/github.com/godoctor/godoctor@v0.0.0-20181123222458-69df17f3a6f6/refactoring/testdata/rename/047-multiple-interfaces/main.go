package main

type if1 interface {
	m1()
	m2() int
}

type if2 interface {
	m1()
	m2() float32
	m3() float32
}

type if3 interface {
	m1()
	m2() int
	m3() float64
}

type t1 int // implements if1 only
type t2 struct { t1 } // implements if1
type t3 int // implements if3 and if1
type t4 int // implements if2 only

func (t1) m1()     {}
func (t1) m2() int { return 0 }
func (t1) m3() int { return 0 }

func (t3) m1()         {}
func (t3) m2() int     { return 0 }
func (t3) m3() float64 { return 0 }

func (t4) m1()         {}
func (t4) m2() float32 { return 0 }
func (t4) m3() float32 { return 0 }

func main() {
	var v1 t1
	var v2 t2
	var v3 t3
	var v4 t4
	v1.m1() // <<<<<rename,42,5,42,5,renamed,pass
	v1.m2()
	v1.m3()
	v2.m1()
	v2.m2()
	v2.m3()
	v3.m1()
	v3.m2()
	v3.m3()
	v4.m1()
	v4.m2()
	v4.m3()
}

func m1() {}
